package fiskal

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/tax"
)

// Extension keys. All are set on the invoice tax (bill/tax), except
// ExtKeyPP, which is set on a tax combo, and ExtKeyVATStatus, which is set
// on the supplier. Each definition's Usage says so.
const (
	// ExtKeyPremises is the business premises label registered in
	// ePorezna (OznPosPr).
	ExtKeyPremises cbc.Key = "hr-fiskal-premises"

	// ExtKeyDevice is the label of the device, such as a till, that issued
	// the invoice (OznNapUr).
	ExtKeyDevice cbc.Key = "hr-fiskal-device"

	// ExtKeySequenceScope says what the invoice numbers count within
	// (OznSlijed).
	ExtKeySequenceScope cbc.Key = "hr-fiskal-sequence-scope"

	// ExtKeyPP says whether the price of a line includes its consumption
	// tax (PP).
	ExtKeyPP cbc.Key = "hr-fiskal-pp"

	// ExtKeyVATStatus says whether the supplier is registered for VAT
	// (USustPdv).
	//
	// The hr-eracun-v1 addon needs the same fact. Each addon defines its own
	// key until both show what they share; then it moves to the hr regime,
	// and this key becomes an alias that a normalizer migrates.
	ExtKeyVATStatus cbc.Key = "hr-fiskal-vat-status"
)

// Whether the supplier is registered for VAT (ExtKeyVATStatus)
const (
	// VATStatusRegistered marks a supplier registered for VAT, which charges
	// VAT and reports its VAT breakdown.
	VATStatusRegistered cbc.Code = "registered"
	// VATStatusNotRegistered marks a supplier not registered for VAT, such as
	// a small taxpayer exempt under Art. 90(1) of the VAT Act.
	VATStatusNotRegistered cbc.Code = "not-registered"
)

// Whether a line's price includes its consumption tax (ExtKeyPP)
const (
	// PPIncludedInPrice marks a price that includes PP, as a menu price
	// does. The addon takes PP out of the price and sets PPExcludedFromPrice.
	PPIncludedInPrice cbc.Code = "included-in-price"
	// PPExcludedFromPrice marks a price without PP, which is added on top.
	PPExcludedFromPrice cbc.Code = "excluded-from-price"
)

// Sequence scopes (OznSlijed)
const (
	// SequencePremises numbers invoices per business premises.
	SequencePremises cbc.Code = "P"
	// SequenceDevice numbers invoices per device.
	SequenceDevice cbc.Code = "N"
)

// usageIn is the usage of an extension of this addon set on schema.
func usageIn(schema string) *cbc.Usage {
	return &cbc.Usage{Keys: []cbc.Key{V1}, Schemas: []string{schema}}
}

var extensions = []*cbc.Definition{
	{
		Key:   ExtKeyPremises,
		Usage: usageIn(bill.ShortSchemaTax),
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Business Premises",
			i18n.HR: "Hrvatska fiskalizacija: Oznaka poslovnog prostora",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Label of the business premises the invoice is issued in, as
				registered in ePorezna: up to 20 letters and digits (OznPosPr).
			`),
		},
		Pattern: `^[0-9a-zA-Z]{1,20}$`,
	},
	{
		Key:   ExtKeyDevice,
		Usage: usageIn(bill.ShortSchemaTax),
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Device",
			i18n.HR: "Hrvatska fiskalizacija: Oznaka naplatnog uređaja",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Label of the device, such as a till, that issued the invoice: up to
				20 digits with no leading zero, unique within the business premises
				(OznNapUr).
			`),
		},
		Pattern: `^[1-9][0-9]{0,19}$`,
	},
	{
		Key:   ExtKeySequenceScope,
		Usage: usageIn(bill.ShortSchemaTax),
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Numbering Scope",
			i18n.HR: "Hrvatska fiskalizacija: Oznaka slijednosti",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				What the invoice numbers count within (OznSlijed): the business
				premises (P, the default) or the device (N).
			`),
		},
		Values: []*cbc.Definition{
			{Code: SequencePremises, Name: i18n.String{
				i18n.EN: "Per business premises",
				i18n.HR: "Na razini poslovnog prostora",
			}},
			{Code: SequenceDevice, Name: i18n.String{
				i18n.EN: "Per device",
				i18n.HR: "Na razini naplatnog uređaja",
			}},
		},
	},
	{
		Key:   ExtKeyPP,
		Usage: usageIn(tax.ShortSchemaCombo),
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Consumption Tax in the Price",
			i18n.HR: "Hrvatska fiskalizacija: Porez na potrošnju u cijeni",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Whether the price of the line, or the amount of the discount or
				charge, includes its consumption tax (PP), set on its PP tax. A
				menu price includes both VAT and PP, which share one base, the
				price without VAT. prices_include names VAT only, so an amount
				that also includes PP says so here (included-in-price), and the
				addon takes PP out of it and sets excluded-from-price. With
				prices_include VAT, each PP must say which.
			`),
		},
		Values: []*cbc.Definition{
			{Code: PPIncludedInPrice, Name: i18n.String{
				i18n.EN: "The price includes PP",
				i18n.HR: "Cijena uključuje porez na potrošnju",
			}},
			{Code: PPExcludedFromPrice, Name: i18n.String{
				i18n.EN: "The price does not include PP",
				i18n.HR: "Cijena ne uključuje porez na potrošnju",
			}},
		},
	},
	{
		Key:   ExtKeyVATStatus,
		Usage: usageIn(org.ShortSchemaParty),
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: VAT Status",
			i18n.HR: "Hrvatska fiskalizacija: U sustavu PDV-a",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Whether the supplier is registered for VAT (USustPdv), set on the
				supplier. Every Croatian supplier has an OIB in its tax ID, so the
				tax ID can't say it. A supplier not registered for VAT, such as a
				small taxpayer, must not charge VAT, and CIS receives only the
				total of its invoices.
			`),
		},
		Values: []*cbc.Definition{
			{Code: VATStatusRegistered, Name: i18n.String{
				i18n.EN: "Registered for VAT",
				i18n.HR: "U sustavu PDV-a",
			}},
			{Code: VATStatusNotRegistered, Name: i18n.String{
				i18n.EN: "Not registered for VAT",
				i18n.HR: "Nije u sustavu PDV-a",
			}},
		},
	},
}
