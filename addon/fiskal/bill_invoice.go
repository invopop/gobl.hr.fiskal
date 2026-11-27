package fiskal

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// sequentialPattern is the sequential number of an invoice, its code: digits
// with no leading zero (BrOznRac).
var sequentialPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// PersonKeyOperator marks the supplier person who issued the invoice, the
// operator. Exactly one supplier person has it. The hr-eracun-v1 addon
// uses the same key.
const PersonKeyOperator cbc.Key = "operator"

// Operator provides the person who issued the invoice: the supplier person
// with the operator key, when exactly one has it. No other person is taken
// in their place; the rules require exactly one, with an OIB identity.
func Operator(people []*org.Person) *org.Person {
	var op *org.Person
	for _, p := range people {
		if !isOperator(p) {
			continue
		}
		if op != nil {
			return nil
		}
		op = p
	}
	return op
}

// -- Normalization --------------------------------------------------------

func normalizeInvoice(inv *bill.Invoice) {
	if inv == nil {
		return
	}
	inv.Code = normalizeCode(inv.Code)
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	if !inv.Tax.Ext.Has(ExtKeySequenceScope) {
		inv.Tax.Ext = inv.Tax.Ext.Set(ExtKeySequenceScope, SequencePremises)
	}
	if inv.Series == cbc.CodeEmpty {
		inv.Series = Series(inv)
	}
	removeIncludedPP(inv)
}

// normalizeCode removes the leading zeros of a code made only of digits,
// so "000042" becomes "42". CIS refuses leading zeros in the sequential
// number (BrOznRac), and Invopop's sequence step pads its numbers to six
// digits unless set otherwise. The invoice is numbered before it is
// validated, so a padded code would use up a number that is never issued.
// A code with other characters, or with only zeros, is left for rule 01
// to refuse.
func normalizeCode(code cbc.Code) cbc.Code {
	s := code.String()
	for _, c := range s {
		if c < '0' || c > '9' {
			return code
		}
	}
	if t := strings.TrimLeft(s, "0"); t != "" {
		return cbc.Code(t)
	}
	return code
}

// -- Rule set -------------------------------------------------------------

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// The code is not required: a draft has none until Invopop numbers it,
		// and GOBL signs an invoice only with a code.
		rules.Field("code",
			rules.AssertIfPresent("01", "invoice code must be the sequential number: digits with no leading zero (BrOznRac)",
				is.MatchesRegexp(sequentialPattern),
			),
		),
		rules.Assert("17", "invoice series must match the numbering scope: the premises label for scope P, or <premises>/<device> for scope N, so that each counter numbers one premises or one device (OznSlijed)",
			is.Func("series matches the numbering scope", seriesMatchesScope),
		),
		// The addon does not fill in a missing issue time with the current
		// time: the invoice must arrive with it. A normalizer runs on every
		// calculation, so it would take the time a draft is first
		// calculated, not the time the invoice is issued.
		rules.Field("issue_time",
			rules.Assert("02", "invoice issue time is required: it goes into the ZKI (DatVrijeme)", is.Present),
		),
		// CIS receives the amounts in euros (spec v2.7, p. 10), but the VAT Act
		// (Art. 81) lets an invoice show them in another currency too. The
		// racun conversion converts the invoice with its exchange rate.
		rules.Assert("03", "invoice must be in EUR or provide an exchange rate to EUR: CIS receives the amounts in euros",
			currency.CanConvertTo(currency.EUR),
		),
		// The payment method is an element of the invoice, reported and
		// printed (Fiscalization Act, Art. 2(7) and Art. 7(1)(3)). The till
		// knows it, so the addon does not fill it in: CIS uses O (other) for
		// several methods or one not on its list, not for an unknown one.
		rules.Assert("34", "invoice payment requires the payment method: an advance, for a sale paid at the till, or payment instructions (NacinPlac)",
			is.Func("has a payment method", hasPaymentMethod),
		),
		rules.Field("payment",
			rules.Field("advances",
				rules.Each(
					rules.Field("key",
						rules.Assert("35", "invoice payment advance requires the payment means key, such as cash or card; other for a method CIS does not list (NacinPlac)", is.Present),
					),
				),
			),
		),
		// The normalizer always sets the invoice tax, so it needs no rule.
		rules.Field("tax",
			rules.Field("ext",
				rules.Assert("08", fmt.Sprintf("invoice tax ext %s must be P or N (OznSlijed)", ExtKeySequenceScope),
					tax.ExtensionHasValidCode(ExtKeySequenceScope),
				),
				rules.Assert("10", fmt.Sprintf("invoice tax ext %s must be up to 100 characters (ParagonBrRac)", ExtKeyPaperNumber),
					tax.ExtensionHasValidCode(ExtKeyPaperNumber),
				),
			),
		),
		rules.Field("supplier",
			rules.Assert("05", fmt.Sprintf("invoice supplier requires exactly one identity with the key %s and one with the key %s: the labels of the business premises and the device that issued the invoice (BrRac)", IdentityKeyPremises, IdentityKeyDevice),
				is.Func("has one premises and one device", hasOnePremisesAndOneDevice),
			),
			rules.Field("identities",
				rules.Each(
					rules.When(org.IdentityKeyIn(IdentityKeyPremises),
						rules.Field("code",
							rules.Assert("06", fmt.Sprintf("invoice supplier identity %s must be up to 20 letters and digits (OznPosPr)", IdentityKeyPremises),
								is.MatchesRegexp(premisesRegexp),
							),
						),
					),
					rules.When(org.IdentityKeyIn(IdentityKeyDevice),
						rules.Field("code",
							rules.Assert("07", fmt.Sprintf("invoice supplier identity %s must be up to 20 digits with no leading zero (OznNapUr)", IdentityKeyDevice),
								is.MatchesRegexp(deviceRegexp),
							),
						),
					),
				),
			),
			rules.Field("tax_id",
				rules.Assert("11", "invoice supplier tax ID is required, with country HR and the supplier's OIB (Oib)", is.Present),
				rules.Field("country",
					rules.Assert("12", "invoice supplier tax ID must be Croatian (Oib)", is.In(l10n.HR.Tax())),
				),
				rules.Field("code",
					rules.Assert("13", "invoice supplier tax ID code is required: the supplier's OIB, also when it is not registered for VAT (Oib)", is.Present),
				),
			),
			// No default: a wrong status reaches CIS with no error, and only
			// the supplier knows it.
			rules.Field("ext",
				rules.Assert("30", fmt.Sprintf("invoice supplier ext requires %s: whether the supplier is registered for VAT (USustPdv)", ExtKeyVATStatus),
					tax.ExtensionsRequire(ExtKeyVATStatus),
				),
				rules.Assert("31", fmt.Sprintf("invoice supplier ext %s must be %s or %s (USustPdv)", ExtKeyVATStatus, VATStatusRegistered, VATStatusNotRegistered),
					tax.ExtensionHasValidCode(ExtKeyVATStatus),
				),
			),
			rules.Field("people",
				rules.Assert("14", "invoice supplier requires exactly one person with the operator key: the person at the till, or the supplier for a self-service device (OibOper)",
					is.Func("has one operator", peopleHaveOneOperator),
				),
				rules.Each(
					rules.When(is.Func("operator", isOperator),
						rules.Field("identities",
							rules.Assert("15", "invoice supplier operator requires an identity of type OIB (OibOper)",
								is.Func("has OIB", identitiesHaveOIB),
							),
						),
					),
				),
			),
		),
		// A customer with a Croatian tax ID is a Croatian business, whose OIB
		// the invoice must report when paid in cash or by card (spec v2.7,
		// p. 10). CIS can't tell a business without its OIB from a private
		// person, so it would accept the invoice: a tax ID with no code is a
		// mistake that only this rule catches.
		rules.Field("customer",
			rules.When(is.Func("has a Croatian tax ID", partyHasCroatianTaxID),
				rules.Field("tax_id",
					rules.Field("code",
						rules.Assert("29", "invoice customer with a Croatian tax ID requires its OIB as the tax ID code (OibPrimateljaRacuna)", is.Present),
					),
				),
			),
		),
		// CIS receives only the total from an issuer not registered for VAT,
		// so VAT charged on its invoice would not be reported. Such an issuer
		// must not show VAT anyway (Art. 90.g of the VAT Act).
		rules.When(is.Func("supplier not registered for VAT", supplierNotRegisteredForVAT),
			rules.Field("totals",
				rules.Assert("16", "invoice from a supplier not registered for VAT must not charge VAT: CIS receives only its total (Art. 90.g of the VAT Act)",
					is.Func("charges no VAT", totalsChargeNoVAT),
				),
			),
		),
		// Rules 18 to 23 refuse what the racun conversion can't convert.
		// racun does not check them again: these rules are the only check,
		// and a valid invoice must always convert. Invopop numbers it before
		// it is fiscalized, and a numbered invoice that is never issued
		// leaves a gap in the sequence, which Art. 9(2) of the
		// Fiscalization Act forbids.
		//
		// Rules 18 and 19 are cases racun does not support yet: corrections
		// (EU3-29). Drop a rule when racun supports its case. Rule 21, a
		// charge without taxes, went when racun began to report fees.
		//
		// Rules 20, 22 and 23 stay: the invoice request has no element for a
		// rounding amount, a discount without taxes or a line without taxes.
		// Each changes the total but no tax breakdown, and CIS checks that
		// the total is the sum of the breakdowns (check v166, spec v2.7,
		// p. 212). A line must say how VAT applies to it, even when it is
		// exempt or outside its scope, for racun to put its amount in the
		// right breakdown.
		rules.Field("type",
			rules.Assert("18", "invoice type must be standard: corrections are not supported yet", is.In(bill.InvoiceTypeStandard)),
		),
		rules.Field("totals",
			rules.Assert("19", "invoice total must not be negative: corrections are not supported yet",
				is.Func("payable not negative", totalsPayableNotNegative),
			),
			rules.Assert("20", "invoice rounding is not supported: the invoice request has no element for it",
				is.Func("no rounding", totalsHaveNoRounding),
			),
		),
		// With prices that include VAT, a till must say whether a drink's
		// price also includes PP: GOBL would otherwise add PP on top of a
		// menu price that already holds it, and charge more than the menu.
		// The same goes for a discount or charge on the whole invoice.
		rules.When(is.Func("prices include VAT", pricesIncludeVAT),
			rules.Field("lines",
				rules.Each(
					rules.Field("taxes",
						rules.Assert("28", fmt.Sprintf("invoice line with PP needs %s when prices include VAT: say whether the price includes PP", ExtKeyPP),
							is.Func("PP says whether the price includes it", ppStatesPrice),
						),
					),
				),
			),
			rules.Field("discounts",
				rules.Each(
					rules.Field("taxes",
						rules.Assert("32", fmt.Sprintf("invoice discount with PP needs %s when prices include VAT: say whether the amount includes PP", ExtKeyPP),
							is.Func("PP says whether the amount includes it", ppStatesPrice),
						),
					),
				),
			),
			rules.Field("charges",
				rules.Each(
					rules.Field("taxes",
						rules.Assert("33", fmt.Sprintf("invoice charge with PP needs %s when prices include VAT: say whether the amount includes PP", ExtKeyPP),
							is.Func("PP says whether the amount includes it", ppStatesPrice),
						),
					),
				),
			),
		),
		// A charge without taxes is a fee, such as a bottle deposit, which
		// racun reports by its name and amount (Naknada). The name is the
		// charge's reason, up to the 100 characters NazivN takes. A charge
		// with the key tax, such as the motor vehicle tax (PPMV), is a tax,
		// not a fee: it belongs in OstaliPor, which racun does not support
		// yet (EU3-34).
		rules.Field("charges",
			rules.Each(
				rules.When(is.Func("fee", chargeIsFee),
					rules.Field("reason",
						rules.Assert("26", "invoice charge without taxes is a fee and needs a reason of up to 100 characters: CIS takes it as the fee's name (NazivN)",
							is.Present, is.RuneLength(1, 100),
						),
					),
					rules.Field("key",
						rules.Assert("27", "invoice charge without taxes must not have the key tax: other taxes (OstaliPor) are not supported yet",
							is.Func("not a tax", keyNotTax),
						),
					),
				),
			),
		),
		rules.Field("discounts",
			rules.Each(
				rules.Field("taxes",
					rules.Assert("22", "invoice discount requires VAT: the invoice request has no element for a discount without VAT",
						is.Present, is.Func("has VAT", taxesHaveVAT),
					),
				),
			),
		),
		rules.Field("lines",
			rules.Each(
				rules.Field("taxes",
					rules.Assert("23", "invoice line requires VAT, with the key exempt or outside-scope when no VAT applies: the invoice request has no element for a line without VAT",
						is.Present, is.Func("has VAT", taxesHaveVAT),
					),
				),
			),
		),
	)
}

// allowedVATPercents are the VAT rates CIS accepts: 0, 5, 13 and 25 %
// (error 182, spec v2.7, p. 213).
var allowedVATPercents = []num.Percentage{
	num.MakePercentage(0, 2),
	num.MakePercentage(5, 2),
	num.MakePercentage(13, 2),
	num.MakePercentage(25, 2),
}

// maxPPPercent is the highest consumption tax rate: a municipality sets its
// rate up to 3 % (Local Taxes Act, Art. 24), and CIS refuses a higher one
// (error 184, spec v2.7, p. 22).
var maxPPPercent = num.MakePercentage(3, 2)

// taxComboRules refuse the taxes that the racun conversion does not support
// yet, for the same reason as rules 18 to 23 of the invoice: taxes other
// than VAT and the consumption tax (PP), and reverse charge, whose report to
// CIS is still to confirm. Rules 03 and 04 refuse a rate that CIS refuses
// (errors 182 to 184), as the invoice would be numbered and then never
// fiscalized.
//
// PP goes on each line of a drink, next to its VAT: both taxes have the same
// base, the price without VAT (Local Taxes Act, Art. 23(2)). The regime
// gives PP no rates, as each municipality sets its own, so the line gives
// the percent.
func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		rules.Field("cat",
			rules.Assert("01", "tax category must be VAT or PP: other taxes are not supported yet",
				is.In(tax.CategoryVAT, hr.TaxCategoryPP),
			),
		),
		rules.Field("key",
			rules.Assert("02", "VAT key must not be reverse-charge or intra-community: not supported yet",
				is.Func("not reverse charge", keyNotReverseCharge),
			),
		),
		rules.When(is.Func("category is VAT", taxComboIsVAT),
			rules.Field("percent",
				rules.AssertIfPresent("03", "VAT percent must be 0, 5, 13 or 25 %: CIS refuses other rates (error 182)",
					is.Func("allowed VAT percent", percentAllowed),
				),
			),
		),
		rules.When(is.Func("category is PP", taxComboIsPP),
			rules.Field("percent",
				rules.Assert("04", "consumption tax (PP) needs a percent from 0 to 3 %: CIS refuses other rates (errors 183 and 184)",
					is.Func("allowed PP percent", ppPercentAllowed),
				),
			),
			// The normalizer takes PP out of an amount only for
			// included-in-price, so any other value would add PP on top of a
			// menu price that already holds it.
			rules.Field("ext",
				rules.Assert("05", fmt.Sprintf("consumption tax (PP) ext %s must be %s or %s", ExtKeyPP, PPIncludedInPrice, PPExcludedFromPrice),
					tax.ExtensionHasValidCode(ExtKeyPP),
				),
			),
		),
	)
}

// -- Predicates -----------------------------------------------------------

func totalsPayableNotNegative(val any) bool {
	t, ok := val.(*bill.Totals)
	return !ok || t == nil || !t.Payable.IsNegative()
}

func totalsHaveNoRounding(val any) bool {
	t, ok := val.(*bill.Totals)
	return !ok || t == nil || t.Rounding == nil || t.Rounding.IsZero()
}

func percentAllowed(val any) bool {
	p, ok := val.(*num.Percentage)
	if !ok || p == nil {
		return true
	}
	for _, a := range allowedVATPercents {
		if p.Equals(a) {
			return true
		}
	}
	return false
}

// IsFee reports whether a charge is a fee (Naknada): a charge without taxes
// that adds to the total. An informative charge adds nothing, so it is not
// one.
func IsFee(c *bill.Charge) bool {
	return c != nil && !c.Informative && len(c.Taxes) == 0
}

func keyNotTax(val any) bool {
	k, ok := val.(cbc.Key)
	return !ok || k != bill.ChargeKeyTax
}

func chargeIsFee(val any) bool {
	c, ok := val.(*bill.Charge)
	return ok && IsFee(c)
}

func ppPercentAllowed(val any) bool {
	p, ok := val.(*num.Percentage)
	return ok && p != nil && !p.IsNegative() && p.Compare(maxPPPercent) <= 0
}

// taxesHaveVAT is true when the set has a VAT combo. PP alone is not
// enough: with no VAT, the amount is in the total but in no VAT breakdown,
// and CIS refuses the invoice (check v166).
func taxesHaveVAT(val any) bool {
	ts, ok := val.(tax.Set)
	return ok && ts.Get(tax.CategoryVAT) != nil
}

func taxComboIsVAT(val any) bool {
	tc, ok := val.(*tax.Combo)
	return ok && tc != nil && tc.Category == tax.CategoryVAT
}

func taxComboIsPP(val any) bool {
	tc, ok := val.(*tax.Combo)
	return ok && tc != nil && tc.Category == hr.TaxCategoryPP
}

func keyNotReverseCharge(val any) bool {
	k, ok := val.(cbc.Key)
	return !ok || !k.In(tax.KeyReverseCharge, tax.KeyIntraCommunity)
}

func peopleHaveOneOperator(val any) bool {
	people, _ := val.([]*org.Person)
	return Operator(people) != nil
}

func isOperator(val any) bool {
	p, ok := val.(*org.Person)
	return ok && p != nil && p.Key == PersonKeyOperator
}

func identitiesHaveOIB(val any) bool {
	ids, _ := val.([]*org.Identity)
	return org.IdentityForType(ids, hr.IdentityTypeOIB) != nil
}

func partyHasCroatianTaxID(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && p.TaxID != nil && p.TaxID.Country == l10n.HR.Tax()
}

func seriesMatchesScope(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Series == cbc.CodeEmpty {
		return true
	}
	return inv.Series == Series(inv)
}

func hasPaymentMethod(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	p := inv.Payment
	return p != nil && (len(p.Advances) > 0 || p.Instructions != nil)
}

func supplierNotRegisteredForVAT(val any) bool {
	inv, ok := val.(*bill.Invoice)
	return ok && inv != nil && notRegisteredForVAT(inv.Supplier)
}

// totalsChargeNoVAT is true when no VAT rate in the totals has a percentage
// above zero. Exempt and outside-scope amounts are allowed.
func totalsChargeNoVAT(val any) bool {
	t, ok := val.(*bill.Totals)
	if !ok || t == nil || t.Taxes == nil {
		return true
	}
	ct := t.Taxes.Category(tax.CategoryVAT)
	if ct == nil {
		return true
	}
	for _, r := range ct.Rates {
		if r.Percent != nil && !r.Percent.IsZero() {
			return false
		}
	}
	return true
}
