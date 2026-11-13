package fiskal

import (
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/tax"
)

// A supplier in the VAT system has a tax ID with its Croatian VAT number, HR
// followed by its OIB. A supplier outside the VAT system, such as a small
// taxpayer, has no VAT number: its tax ID has only the country HR, which
// keeps the regime, and its OIB is an identity of type OIB. So the invoice
// says whether the supplier is in the VAT system (USustPdv) without a flag
// of its own.
//
// This follows the model gobl.fr.ctc uses for French suppliers: the tax ID
// holds only a VAT number, and an identity says who the supplier is. The
// hr-eracun-v1 addon takes the same party, and PartyHasVATNumber, PartyOIB,
// NormalizeOIB and partyOutsideVAT are the same in both modules.

// PartyHasVATNumber says whether the party's tax ID has a VAT number. A
// Croatian supplier without one is outside the VAT system.
func PartyHasVATNumber(p *org.Party) bool {
	return p != nil && p.TaxID != nil && p.TaxID.Code != cbc.CodeEmpty
}

// PartyOIB provides the OIB of a party: the code of its Croatian VAT number,
// or else the code of its OIB identity. It is empty when the party has
// neither.
func PartyOIB(p *org.Party) cbc.Code {
	if p == nil {
		return cbc.CodeEmpty
	}
	if p.TaxID != nil && p.TaxID.Country == l10n.HR.Tax() {
		tID := *p.TaxID
		tax.NormalizeIdentity(&tID)
		if tID.Code != cbc.CodeEmpty {
			return tID.Code
		}
	}
	if id := org.IdentityForType(p.Identities, hr.IdentityTypeOIB); id != nil {
		return NormalizeOIB(id.Code.String())
	}
	return cbc.CodeEmpty
}

// NormalizeOIB removes the separators and the HR prefix of a VAT number from
// an OIB, as the hr regime does with an OIB identity.
func NormalizeOIB(code string) cbc.Code {
	c := cbc.NormalizeAlphanumericalCode(cbc.Code(code))
	return cbc.Code(strings.TrimPrefix(c.String(), l10n.HR.String()))
}

// partyOutsideVAT says whether the party is a Croatian supplier outside the
// VAT system: a tax ID with only the country HR. A party with no tax ID
// says nothing about it.
func partyOutsideVAT(p *org.Party) bool {
	return p != nil && p.TaxID != nil && p.TaxID.Country == l10n.HR.Tax() && p.TaxID.Code == cbc.CodeEmpty
}
