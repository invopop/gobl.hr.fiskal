// Converting a GOBL invoice into the Invoice CIS fiscalizes (Racun).
//
// The GOBL invoice must declare the hr-fiskal-b2c-v1 addon (package
// addon/fiskal) and be valid: the addon's rules check the fields the Invoice
// takes, so the conversion does not check their format again. The Invoice
// takes the issuer's OIB, the issue date and time, the number with the
// business premises and device labels, the numbering scope, the VAT system,
// the VAT breakdown, the total, the payment method, the operator's OIB, the
// paper invoice number and the buyer's OIB from the invoice. Only whether
// the Invoice is sent late comes from Options. An invoice in another
// currency is converted to euros first, with its exchange rate.

package racun

import (
	"crypto"
	"errors"
	"fmt"
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/tax"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
)

// Errors the conversion returns. The conversion does not check the
// invoice's content: the addon rules do, when the invoice is validated.
// It only refuses an input that breaks its contract in a way the rules
// can't see, and reports technical failures.
var (
	// ErrNotInvoice is an envelope that holds no invoice.
	ErrNotInvoice = errors.New("envelope does not hold an invoice")
	// ErrMissing is an invoice without the addon or the code the
	// conversion requires.
	ErrMissing = errors.New("missing what fiscalization requires")
	// ErrInvalid is a value fiscalization can't take, such as an amount
	// that can't be converted to euros.
	ErrInvalid = errors.New("has a value fiscalization can't take")
)

// Options are the Invoice values that are not in the GOBL invoice, as they
// depend on when it is sent.
type Options struct {
	// LateDelivery is true when the invoice was issued without a JIR and
	// is sent again (NakDost). An invoice with a paper invoice number is
	// always late.
	LateDelivery bool
}

// NewInvoice builds the Invoice of the GOBL invoice in the envelope, with its ZKI
// computed with key, the issuer's private key. The envelope must be
// calculated and valid.
func NewInvoice(env *gobl.Envelope, opts *Options, key crypto.Signer) (*Invoice, error) {
	if env == nil {
		return nil, ErrNotInvoice
	}
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrNotInvoice
	}
	return FromInvoice(inv, opts, key)
}

// FromInvoice builds the Invoice of a calculated and valid GOBL invoice with
// the hr-fiskal-b2c-v1 addon and a code, with its ZKI computed with key, the
// issuer's private key. The ZKI goes on the printed invoice and does not
// change when the Invoice is sent again.
//
// The addon rules check the invoice's content, so the conversion does not
// check it again: an invoice that breaks them is outside its contract.
func FromInvoice(inv *bill.Invoice, opts *Options, key crypto.Signer) (*Invoice, error) {
	if opts == nil {
		opts = new(Options)
	}
	r := new(Invoice)
	if err := checkInvoice(inv); err != nil {
		return nil, err
	}
	inv, err := inEuros(inv)
	if err != nil {
		return nil, err
	}

	r.IssuerOIB = inv.Supplier.TaxID.Code.String()
	issuedAt := issueTime(inv)
	r.IssuedAt = formatDateTime(issuedAt, "T")
	ext := inv.Tax.Ext
	r.Number = number(inv)
	r.SequenceScope = SequencePremises
	if ext.Get(fiskal.ExtKeySequenceScope) == fiskal.SequenceDevice {
		r.SequenceScope = SequenceDevice
	}
	r.InVATSystem = inv.Supplier.Ext.Get(fiskal.ExtKeyVATSystem) != fiskal.VATSystemOut
	addTaxes(r, inv)
	if !r.InVATSystem {
		dropVATBreakdown(r)
	}
	r.Total = amount(inv.Totals.Payable)
	r.PaymentMethod = paymentMethod(inv.Payment)
	r.OperatorOIB = operatorOIB(inv)
	r.HandwrittenNumber = ext.Get(fiskal.ExtKeyPaperNumber).String()
	r.LateDelivery = opts.LateDelivery || r.HandwrittenNumber != ""
	// CIS refuses a buyer's OIB on an invoice paid by bank transfer (error
	// 181, §2.1.5).
	if r.PaymentMethod != PaymentTransfer {
		r.BuyerOIB = buyerOIB(inv)
	}

	r.ZKI, err = ZKI(key, &ZKIInput{
		IssuerOIB: r.IssuerOIB,
		IssuedAt:  issuedAt,
		Number:    r.Number,
		Total:     r.Total,
	})
	if err != nil {
		return nil, fmt.Errorf("computing the ZKI: %w", err)
	}
	return r, nil
}

// checkInvoice refuses an input that breaks the contract of the conversion
// in a way the addon rules can't see: an invoice without the addon, whose
// rules then never ran, or without a code, which the rules allow so that a
// draft can be validated before it is numbered.
func checkInvoice(inv *bill.Invoice) error {
	switch {
	case inv == nil:
		return ErrNotInvoice
	case !fiskal.V1.In(inv.GetAddons()...):
		return fmt.Errorf("addon %s %w", fiskal.V1, ErrMissing)
	case inv.Code == cbc.CodeEmpty:
		return fmt.Errorf("invoice code %w: number the invoice first", ErrMissing)
	}
	return nil
}

// inEuros provides the invoice with its amounts in euros, which CIS
// receives (spec v2.7, p. 10). An invoice in another currency is converted
// with its exchange rate to EUR: GOBL converts the line prices, charges and
// discounts, and then calculates the totals again. The euro amounts printed
// on the invoice (Art. 81 of the VAT Act) must come from this conversion, as
// the ZKI and the QR code do.
func inEuros(inv *bill.Invoice) (*bill.Invoice, error) {
	if inv.Currency == currency.EUR {
		return inv, nil
	}
	// ConvertInto calculates its receiver again, so it gets a copy. The
	// invoice is calculated already, so the copy's shared parts do not
	// change.
	c := *inv
	eur, err := c.ConvertInto(currency.EUR)
	if err != nil {
		return nil, fmt.Errorf("currency %q %w: CIS receives euros: %s", inv.Currency, ErrInvalid, err)
	}
	return eur, nil
}

// operatorOIB is the OIB of the supplier person with the operator key.
func operatorOIB(inv *bill.Invoice) string {
	op := fiskal.Operator(inv.Supplier.People)
	return org.IdentityForType(op.Identities, hr.IdentityTypeOIB).Code.String()
}

// issueTime is the issue date and time of the invoice. GOBL dates and times
// are local to the regime and have no zone, and so do the Invoice's, so they
// are copied as they are: a time.Time would move a time that does not exist
// in Croatia, such as 02:30 on the day clocks go forward. 00:00:00 is
// midnight, a valid issue time.
func issueTime(inv *bill.Invoice) cal.DateTime {
	d, t := inv.IssueDate, inv.IssueTime
	return cal.MakeDateTime(d.Year, d.Month, d.Day, t.Hour, t.Minute, t.Second)
}

// number is the invoice number: the GOBL invoice code, which the addon
// requires to be the bare sequential number, and the business premises and
// device labels.
func number(inv *bill.Invoice) *Number {
	return &Number{
		Sequential: inv.Code.String(),
		Premises:   inv.Tax.Ext.Get(fiskal.ExtKeyPremises).String(),
		Device:     inv.Tax.Ext.Get(fiskal.ExtKeyDevice).String(),
	}
}

// addTaxes adds the VAT breakdown: one entry per rate with a percentage, and
// the exempt and outside-scope bases as totals. The addon rules allow only
// VAT, and no reverse charge, so every other rate has a percentage.
func addTaxes(r *Invoice, inv *bill.Invoice) {
	vat := inv.Totals.Taxes.Category(tax.CategoryVAT)
	if vat == nil {
		return
	}
	// The totals start with two decimals: Add keeps the decimals of its
	// receiver, so a zero Amount would round every base to whole euros.
	exempt, outside := num.MakeAmount(0, 2), num.MakeAmount(0, 2)
	var hasExempt, hasOutside bool
	for _, rate := range vat.Rates {
		switch rate.Key {
		case tax.KeyExempt, tax.KeyExport:
			exempt = exempt.Add(rate.Base)
			hasExempt = true
		case tax.KeyOutsideScope:
			outside = outside.Add(rate.Base)
			hasOutside = true
		default:
			r.VAT = append(r.VAT, &Tax{
				Rate:   percent(*rate.Percent),
				Base:   amount(rate.Base),
				Amount: amount(rate.Amount),
			})
		}
	}
	if hasExempt {
		r.VATExemptAmount = amount(exempt)
	}
	if hasOutside {
		r.NotSubjectAmount = amount(outside)
	}
}

// dropVATBreakdown leaves out the VAT breakdown of an issuer outside the VAT
// system, such as a small taxpayer: the VAT entries (Pdv), the exempt
// amount (IznosOslobPdv) and the amount not subject to tax
// (IznosNePodlOpor). The Invoice then has only its total.
//
// No source says which of these fields holds a small taxpayer's sales, so
// the Invoice makes no claim about it. CIS checks the breakdown against the
// total only for an issuer in the VAT system (checks v160, v161 and v166,
// spec v2.7, p. 212), and the spec's examples with USustPdv false carry no
// breakdown. Till software does the same: "Kod obveznika koji nisu u
// sustavu pdv-a ne smije biti nikakvih poreza u fiskalnoj XML datoteci"
// (IPOS, Fiskalizacija 1.0 help). Open question: regulations-kb#55.
func dropVATBreakdown(r *Invoice) {
	r.VAT = nil
	r.VATExemptAmount = ""
	r.NotSubjectAmount = ""
}

// amount writes an amount with two decimals, as IznosType asks.
func amount(a num.Amount) string {
	return a.Rescale(2).String()
}

// percent writes a percentage as a number with two decimals, such as
// "25.00" for 25 %, as StopaType asks.
func percent(p num.Percentage) string {
	return p.Amount().Rescale(2).String()
}

// paymentMethod maps how the invoice was paid to NacinPlac. An invoice paid
// at the till records its payment as advances; an invoice without them
// falls back to the payment instructions. Several methods, or one without
// a code of its own, are O (other), as the schema says.
func paymentMethod(p *bill.PaymentDetails) PaymentMethod {
	var keys []cbc.Key
	if p != nil {
		for _, a := range p.Advances {
			keys = append(keys, a.Key)
		}
		if len(keys) == 0 && p.Instructions != nil {
			keys = append(keys, p.Instructions.Key)
		}
	}
	method := PaymentOther
	for i, k := range keys {
		m := meansMethod(k)
		if i > 0 && m != method {
			return PaymentOther
		}
		method = m
	}
	return method
}

func meansMethod(k cbc.Key) PaymentMethod {
	base := cbc.Key(strings.SplitN(k.String(), "+", 2)[0])
	switch base {
	case pay.MeansKeyCash:
		return PaymentCash
	case pay.MeansKeyCard:
		return PaymentCard
	case pay.MeansKeyCreditTransfer, pay.MeansKeyDebitTransfer, pay.MeansKeyDirectDebit:
		return PaymentTransfer
	}
	return PaymentOther
}

// buyerOIB is the customer's OIB, for an invoice to a Croatian business. It
// is mandatory for one paid in cash or by card (§2.1.1.1).
func buyerOIB(inv *bill.Invoice) string {
	c := inv.Customer
	if c == nil || c.TaxID == nil || c.TaxID.Country != l10n.HR.Tax() {
		return ""
	}
	return c.TaxID.Code.String()
}
