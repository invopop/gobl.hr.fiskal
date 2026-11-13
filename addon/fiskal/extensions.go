package fiskal

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// Extension keys. All are set on the invoice tax (bill/tax).
//
// TODO: declare each extension's usage (cbc.Usage), as gobl.fr.ctc does, once
// the hr regime branch is rebased on a GOBL release that has it.
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

	// ExtKeyPaperNumber is the number of an invoice from the bound paper
	// invoice book, entered after the till broke down (ParagonBrRac).
	ExtKeyPaperNumber cbc.Key = "hr-fiskal-paper-number"
)

// Sequence scopes (OznSlijed)
const (
	// SequencePremises numbers invoices per business premises.
	SequencePremises cbc.Code = "P"
	// SequenceDevice numbers invoices per device.
	SequenceDevice cbc.Code = "N"
)

var extensions = []*cbc.Definition{
	{
		Key: ExtKeyPremises,
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
		Key: ExtKeyDevice,
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
		Key: ExtKeySequenceScope,
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
		Key: ExtKeyPaperNumber,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Paper Invoice Number",
			i18n.HR: "Hrvatska fiskalizacija: Broj računa iz uvezane knjige računa",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Number of an invoice issued from the bound paper invoice book while
				the till was broken, when it is entered and fiscalized later (up to
				100 characters, ParagonBrRac). Such an invoice is always sent late.
			`),
		},
		Pattern: `^.{1,100}$`,
	},
}
