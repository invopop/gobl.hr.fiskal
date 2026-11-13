package fiskal

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

// NoteSmallTaxpayer is the note an invoice from a small taxpayer must carry
// (Art. 79(3) of the VAT Act): its supplies are exempt from VAT under
// Art. 90(1), which since 1 January 2025 holds the small-business
// exemption.
const NoteSmallTaxpayer = "Oslobođeno PDV-a temeljem čl. 90. st. 1. Zakona o porezu na dodanu vrijednost"

var invoiceScenarios = &tax.ScenarioSet{
	Schema: bill.ShortSchemaInvoice,
	List: []*tax.Scenario{
		// Supplier outside the VAT system with exempt supplies: a small
		// taxpayer. A supply outside the scope of VAT is not exempt, so it
		// gets no note.
		{
			Filter:     func(doc any) bool { return outsideVATSystem(doc) && hasExemptVAT(doc) },
			Categories: []cbc.Code{tax.CategoryVAT},
			Note: &tax.Note{
				Category: tax.CategoryVAT,
				Key:      tax.KeyExempt,
				Text:     NoteSmallTaxpayer,
			},
		},
	},
}

// hasExemptVAT is true when a line, charge or discount of the invoice has
// VAT with the exempt key.
func hasExemptVAT(doc any) bool {
	inv, ok := doc.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	for _, l := range inv.Lines {
		if l != nil && exemptVAT(l.Taxes) {
			return true
		}
	}
	for _, c := range inv.Charges {
		if c != nil && exemptVAT(c.Taxes) {
			return true
		}
	}
	for _, d := range inv.Discounts {
		if d != nil && exemptVAT(d.Taxes) {
			return true
		}
	}
	return false
}

func exemptVAT(ts tax.Set) bool {
	c := ts.Get(tax.CategoryVAT)
	return c != nil && c.Key == tax.KeyExempt
}
