package fiskal_test

import (
	"testing"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// menuLine is a line at a menu price, with VAT and a 3 % PP whose place in
// the price is given by ppPrice (empty for none).
func menuLine(name string, qty int64, price string, rate cbc.Key, ppPrice cbc.Code) *bill.Line {
	taxes := tax.Set{{Category: tax.CategoryVAT, Rate: rate}}
	if ppPrice != cbc.CodeEmpty {
		taxes = append(taxes, &tax.Combo{
			Category: hr.TaxCategoryPP,
			Percent:  num.NewPercentage(3, 2),
			Ext:      tax.ExtensionsOf(cbc.CodeMap{fiskal.ExtKeyPPPrice: ppPrice}),
		})
	}
	return &bill.Line{
		Quantity: num.MakeAmount(qty, 0),
		Item:     &org.Item{Name: name, Price: amountPtr(price)},
		Taxes:    taxes,
	}
}

func amountPtr(s string) *num.Amount {
	a, err := num.AmountFromString(s)
	if err != nil {
		panic(err)
	}
	return &a
}

func menuInvoice(lines ...*bill.Line) *bill.Invoice {
	inv := testInvoice()
	inv.Tax.PricesInclude = tax.CategoryVAT
	inv.Lines = lines
	return inv
}

// categoryTotal is the base and amount of the first rate of a tax category.
func categoryTotal(t *testing.T, inv *bill.Invoice, cat cbc.Code) (string, string) {
	t.Helper()
	ct := inv.Totals.Taxes.Category(cat)
	require.NotNil(t, ct, "no %s total", cat)
	return ct.Rates[0].Base.String(), ct.Rates[0].Amount.String()
}

// A Trogir bar receipt (Feb 2025): 2 × 4.00 at 25 % VAT and 3 % PP.
func TestPPIncludedInMenuPrice(t *testing.T) {
	inv := menuInvoice(menuLine("Pelinkovac 0,03 l", 2, "4.00", tax.RateGeneral, fiskal.PPPriceIncluded))
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))

	assert.Equal(t, "3.906250", inv.Lines[0].Item.Price.String(), "4.00 × 1.25 / 1.28: the price keeps its VAT")
	assert.Equal(t, fiskal.PPPriceExcluded, inv.Lines[0].Taxes.Get(hr.TaxCategoryPP).Ext.Get(fiskal.ExtKeyPPPrice))
	base, vat := categoryTotal(t, inv, tax.CategoryVAT)
	assert.Equal(t, []string{"6.25", "1.56"}, []string{base, vat})
	base, pp := categoryTotal(t, inv, hr.TaxCategoryPP)
	assert.Equal(t, []string{"6.25", "0.19"}, []string{base, pp})
	assert.Equal(t, "8.00", inv.Totals.Payable.String())

	t.Run("a second calculation changes nothing", func(t *testing.T) {
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "3.906250", inv.Lines[0].Item.Price.String())
		assert.Equal(t, "8.00", inv.Totals.Payable.String())
	})
}

// A Split restaurant receipt (Jul 2025): drinks with PP, food at 13 % VAT
// and a cover charge at 25 % VAT only. The menu prices add up to the total.
func TestPPIncludedWithOtherLines(t *testing.T) {
	inv := menuInvoice(
		menuLine("Somersby 0,33 l", 1, "4.20", tax.RateGeneral, fiskal.PPPriceIncluded),
		menuLine("Točeno pivo 0,5 l", 1, "4.60", tax.RateGeneral, fiskal.PPPriceIncluded),
		menuLine("BBQ burger", 1, "12.90", tax.RateReduced, ""),
		menuLine("Pašteta", 1, "6.10", tax.RateReduced, ""),
		menuLine("Kruh s lukom", 1, "4.20", tax.RateReduced, ""),
		menuLine("Postava stola", 2, "0.50", tax.RateGeneral, ""),
		menuLine("Pizza tuna", 1, "12.00", tax.RateReduced, ""),
		menuLine("Bijelo vino 1 l", 1, "8.00", tax.RateGeneral, fiskal.PPPriceIncluded),
	)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, "6.10", inv.Lines[3].Item.Price.String(), "a line without PP keeps its price")
	_, pp := categoryTotal(t, inv, hr.TaxCategoryPP)
	assert.Equal(t, "0.39", pp)
	assert.Equal(t, "53.00", inv.Totals.Payable.String())
}

// A fixed discount on a drink includes PP too, so PP comes out of it.
func TestPPIncludedWithLineDiscount(t *testing.T) {
	l := menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPPriceIncluded)
	l.Discounts = []*bill.LineDiscount{{Amount: num.MakeAmount(100, 2), Reason: "Happy hour"}}
	inv := menuInvoice(l)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, "7.00", inv.Totals.Payable.String())
}

// A business outside the VAT system charges PP on its price, with no VAT.
func TestPPIncludedOutsideVATSystem(t *testing.T) {
	inv := testInvoice()
	outsideVAT(inv)
	inv.Lines = []*bill.Line{menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPPriceIncluded)}
	inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	base, pp := categoryTotal(t, inv, hr.TaxCategoryPP)
	assert.Equal(t, []string{"7.77", "0.23"}, []string{base, pp})
	assert.Equal(t, "8.00", inv.Totals.Payable.String())
}

func TestPPPriceRule(t *testing.T) {
	t.Run("refuses PP without the extension when prices include VAT", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPPriceIncluded))
		inv.Lines[0].Taxes[1].Ext = tax.Extensions{}
		assertFault(t, inv, faultCode("28"))
	})
	t.Run("adds PP on top of a price that excludes it", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPPriceExcluded))
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, "4.00", inv.Lines[0].Item.Price.String())
		assert.Equal(t, "8.19", inv.Totals.Payable.String())
	})
	t.Run("needs no extension with prices without VAT", func(t *testing.T) {
		inv := testInvoice()
		inv.Lines = []*bill.Line{menuLine("Pivo", 2, "3.125", tax.RateGeneral, "")}
		inv.Lines[0].Taxes = append(inv.Lines[0].Taxes, &tax.Combo{Category: hr.TaxCategoryPP, Percent: num.NewPercentage(3, 2)})
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, "8.00", inv.Totals.Payable.String())
	})
}
