package fiskal

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Every Croatian citizen, resident and company has an OIB (OIB Act, NN
// 60/08), whether or not it is registered for VAT, and a Croatian VAT number
// is HR followed by the OIB. So a Croatian party's tax ID always holds its
// OIB, and the tax ID can't say whether the party is registered for VAT: the
// supplier says it in ExtKeyVATStatus (USustPdv).
//
// Invopop's sequence step keeps one counter for each supplier tax ID and
// series, so a tax ID with only the country HR would put every supplier
// without a code on one counter.

// PartyOIB provides the OIB of a party: the code of its Croatian tax ID. It
// is empty for a party with no Croatian tax ID.
func PartyOIB(p *org.Party) cbc.Code {
	if p == nil || p.TaxID == nil || p.TaxID.Country != l10n.HR.Tax() {
		return cbc.CodeEmpty
	}
	tID := *p.TaxID
	tax.NormalizeIdentity(&tID)
	return tID.Code
}

// RegisteredForVAT says whether the supplier is registered for VAT
// (USustPdv), as its ExtKeyVATStatus says.
func RegisteredForVAT(p *org.Party) bool {
	return p != nil && p.Ext.Get(ExtKeyVATStatus) == VATStatusRegistered
}

func notRegisteredForVAT(p *org.Party) bool {
	return p != nil && p.Ext.Get(ExtKeyVATStatus) == VATStatusNotRegistered
}
