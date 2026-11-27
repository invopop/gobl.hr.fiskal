package fiskal

import (
	"regexp"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pkg/here"
)

// Identity keys of the supplier. The business premises and the device that
// issue an invoice identify where the supplier sells, as a SIRET identifies
// an establishment of a French company, so each is an identity of the
// supplier, not an extension of the invoice. The keys are namespaced, as
// the de-tax-number and it-fiscal-code identity keys of GOBL's regimes are.
const (
	// IdentityKeyPremises is the identity of the business premises the
	// invoice is issued in: its label registered in ePorezna (OznPosPr).
	IdentityKeyPremises cbc.Key = "hr-fiskal-premises"

	// IdentityKeyDevice is the identity of the device, such as a till, that
	// issued the invoice: its label, unique within the premises (OznNapUr).
	IdentityKeyDevice cbc.Key = "hr-fiskal-device"
)

const (
	premisesPattern = `^[0-9a-zA-Z]{1,20}$`
	devicePattern   = `^[1-9][0-9]{0,19}$`
)

var (
	premisesRegexp = regexp.MustCompile(premisesPattern)
	deviceRegexp   = regexp.MustCompile(devicePattern)
)

var identities = []*cbc.Definition{
	{
		Key: IdentityKeyPremises,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Business Premises",
			i18n.HR: "Hrvatska fiskalizacija: Oznaka poslovnog prostora",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Label of the business premises the invoice is issued in, as
				registered in ePorezna: up to 20 letters and digits, unique for
				the supplier's OIB (OznPosPr). An identity of the supplier.
			`),
		},
		Pattern: premisesPattern,
	},
	{
		Key: IdentityKeyDevice,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: Device",
			i18n.HR: "Hrvatska fiskalizacija: Oznaka naplatnog uređaja",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Label of the device, such as a till, that issued the invoice: up
				to 20 digits with no leading zero, unique within the business
				premises (OznNapUr). For a self-service device, the number the
				Tax Administration gave it. An identity of the supplier.
			`),
		},
		Pattern: devicePattern,
	},
}

// Premises provides the label of the business premises that issued the
// invoice: the code of the supplier's one identity with IdentityKeyPremises.
// It is empty when the supplier has none, or more than one: the rules then
// refuse the invoice, and no identity is taken in place of another.
func Premises(supplier *org.Party) cbc.Code {
	return onlyIdentityCode(supplier, IdentityKeyPremises)
}

// Device provides the label of the device that issued the invoice: the code
// of the supplier's one identity with IdentityKeyDevice, or empty, as
// Premises.
func Device(supplier *org.Party) cbc.Code {
	return onlyIdentityCode(supplier, IdentityKeyDevice)
}

func onlyIdentityCode(p *org.Party, key cbc.Key) cbc.Code {
	if p == nil {
		return cbc.CodeEmpty
	}
	var found *org.Identity
	for _, id := range p.Identities {
		if id == nil || id.Key != key {
			continue
		}
		if found != nil {
			return cbc.CodeEmpty
		}
		found = id
	}
	if found == nil {
		return cbc.CodeEmpty
	}
	return found.Code
}

// Series provides the invoice series that matches the numbering scope: the
// business premises label for scope P, such as "SHOP1", or the premises and
// device labels for scope N, such as "SHOP1/1". It is empty without the
// labels.
//
// An invoice counter belongs to one series. When Invopop numbers the
// invoices, its sequence service keeps one counter for each supplier tax ID
// and series, so this series gives one counter for each business premises,
// or for each device, as the numbering scope requires. A till that numbers
// its own invoices can leave the series empty.
func Series(inv *bill.Invoice) cbc.Code {
	if inv == nil {
		return cbc.CodeEmpty
	}
	premises := Premises(inv.Supplier)
	if premises == cbc.CodeEmpty {
		return cbc.CodeEmpty
	}
	if inv.Tax == nil || inv.Tax.Ext.Get(ExtKeySequenceScope) != SequenceDevice {
		return premises
	}
	device := Device(inv.Supplier)
	if device == cbc.CodeEmpty {
		return cbc.CodeEmpty
	}
	return premises + "/" + device
}

func hasOnePremisesAndOneDevice(val any) bool {
	p, ok := val.(*org.Party)
	return ok && Premises(p) != cbc.CodeEmpty && Device(p) != cbc.CodeEmpty
}
