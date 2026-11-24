package fiskal_test

import (
	"testing"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/untdid"
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
			Ext:      tax.ExtensionsOf(cbc.CodeMap{fiskal.ExtKeyPP: ppPrice}),
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
	inv := menuInvoice(menuLine("Pelinkovac 0,03 l", 2, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice))
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))

	assert.Equal(t, "3.906250", inv.Lines[0].Item.Price.String(), "4.00 × 1.25 / 1.28: the price keeps its VAT")
	pp := inv.Lines[0].Taxes.Get(hr.TaxCategoryPP)
	assert.Equal(t, fiskal.PPExcludedFromPrice, pp.Ext.Get(fiskal.ExtKeyPP))
	assert.False(t, pp.Ext.Has(untdid.ExtKeyTaxCategory), "PP is not VAT: it has no VAT category")
	assert.Equal(t, en16931.TaxCategoryStandard, inv.Lines[0].Taxes.Get(tax.CategoryVAT).Ext.Get(untdid.ExtKeyTaxCategory))
	base, vat := categoryTotal(t, inv, tax.CategoryVAT)
	assert.Equal(t, []string{"6.25", "1.56"}, []string{base, vat})
	base, ppAmount := categoryTotal(t, inv, hr.TaxCategoryPP)
	assert.Equal(t, []string{"6.25", "0.19"}, []string{base, ppAmount})
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
		menuLine("Somersby 0,33 l", 1, "4.20", tax.RateGeneral, fiskal.PPIncludedInPrice),
		menuLine("Točeno pivo 0,5 l", 1, "4.60", tax.RateGeneral, fiskal.PPIncludedInPrice),
		menuLine("BBQ burger", 1, "12.90", tax.RateReduced, ""),
		menuLine("Pašteta", 1, "6.10", tax.RateReduced, ""),
		menuLine("Kruh s lukom", 1, "4.20", tax.RateReduced, ""),
		menuLine("Postava stola", 2, "0.50", tax.RateGeneral, ""),
		menuLine("Pizza tuna", 1, "12.00", tax.RateReduced, ""),
		menuLine("Bijelo vino 1 l", 1, "8.00", tax.RateGeneral, fiskal.PPIncludedInPrice),
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
	l := menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice)
	l.Discounts = []*bill.LineDiscount{{Amount: num.MakeAmount(100, 2), Reason: "Happy hour"}}
	inv := menuInvoice(l)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, "7.00", inv.Totals.Payable.String())
}

// ppTaxes are the taxes of a drink: 25 % VAT and 3 % PP, which says whether
// the amount includes it unless ppPrice is empty.
func ppTaxes(ppPrice cbc.Code) tax.Set {
	pp := &tax.Combo{Category: hr.TaxCategoryPP, Percent: num.NewPercentage(3, 2)}
	if ppPrice != cbc.CodeEmpty {
		pp.Ext = tax.ExtensionsOf(cbc.CodeMap{fiskal.ExtKeyPP: ppPrice})
	}
	return tax.Set{{Category: tax.CategoryVAT, Rate: tax.RateGeneral}, pp}
}

// A discount or charge on the whole bill that includes PP loses it as a
// line's price does, so the total is the till's: 4.00 - 1.00, or 4.00 + 1.00.
func TestPPIncludedInOrderDiscountsAndCharges(t *testing.T) {
	beer := func() *bill.Line { return menuLine("Pivo", 1, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice) }
	tests := []struct {
		name    string
		edit    func(inv *bill.Invoice)
		payable string
	}{
		{"fixed discount", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Amount: num.MakeAmount(100, 2), Reason: "Popust", Taxes: ppTaxes(fiskal.PPIncludedInPrice)}}
		}, "3.00"},
		{"percent discount", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Percent: num.NewPercentage(25, 2), Reason: "Popust", Taxes: ppTaxes(fiskal.PPIncludedInPrice)}}
		}, "3.00"},
		{"percent discount of a given base", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Percent: num.NewPercentage(50, 2), Base: num.NewAmount(200, 2), Reason: "Popust", Taxes: ppTaxes(fiskal.PPIncludedInPrice)}}
		}, "3.00"},
		{"fixed charge", func(inv *bill.Invoice) {
			inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(100, 2), Reason: "Glazba uživo", Taxes: ppTaxes(fiskal.PPIncludedInPrice)}}
		}, "5.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := menuInvoice(beer())
			tt.edit(inv)
			require.NoError(t, inv.Calculate())
			require.NoError(t, rules.Validate(inv))
			assert.Equal(t, tt.payable, inv.Totals.Payable.String())
			assert.Len(t, inv.Totals.Taxes.Category(hr.TaxCategoryPP).Rates, 1, "one PP rate for CIS, as every PP is excluded-from-price")

			require.NoError(t, inv.Calculate())
			assert.Equal(t, tt.payable, inv.Totals.Payable.String(), "a second calculation changes nothing")
		})
	}
}

// With prices that include VAT, a discount or charge on the whole bill must
// say whether its amount includes PP, as a line must.
func TestPPOrderDiscountsAndChargesStatePrice(t *testing.T) {
	t.Run("discount", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 1, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice))
		inv.Discounts = []*bill.Discount{{Amount: num.MakeAmount(100, 2), Reason: "Popust", Taxes: ppTaxes("")}}
		assertFault(t, inv, faultCode("32"))
	})
	t.Run("charge", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 1, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice))
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(100, 2), Reason: "Glazba uživo", Taxes: ppTaxes("")}}
		assertFault(t, inv, faultCode("33"))
	})
}

// A business not registered for VAT charges PP on its price, with no VAT.
func TestPPIncludedNotRegisteredForVAT(t *testing.T) {
	inv := testInvoice()
	notRegistered(inv)
	inv.Lines = []*bill.Line{menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice)}
	inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	base, pp := categoryTotal(t, inv, hr.TaxCategoryPP)
	assert.Equal(t, []string{"7.77", "0.23"}, []string{base, pp})
	assert.Equal(t, "8.00", inv.Totals.Payable.String())
}

func TestPPRule(t *testing.T) {
	t.Run("refuses PP without the extension when prices include VAT", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice))
		inv.Lines[0].Taxes[1].Ext = tax.Extensions{}
		assertFault(t, inv, faultCode("28"))
	})
	t.Run("refuses an unknown value of the extension", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 2, "4.00", tax.RateGeneral, "included"))
		require.NoError(t, inv.Calculate())
		faults := rules.Validate(inv)
		require.Error(t, faults)
		assert.True(t, faults.HasCode(rules.GOBL.Add(fiskal.Namespace).Add("TAX-COMBO").Add("05")), "got %s", faults)
	})
	t.Run("adds PP on top of a price that excludes it", func(t *testing.T) {
		inv := menuInvoice(menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPExcludedFromPrice))
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

// Every amount on a drink line includes PP, so PP comes out of each one.
// Each invoice is 2 × 4.00 on the menu, with VAT and PP in the price, and is
// calculated twice: GOBL sets an item's price from its list price each
// time, so the list price must lose its PP too.
func TestPPIncludedInOtherLineAmounts(t *testing.T) {
	drink := func() *bill.Line {
		return menuLine("Pivo", 2, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice)
	}
	tests := []struct {
		name    string
		edit    func(*bill.Line)
		payable string
	}{
		{"a fixed line charge", func(l *bill.Line) {
			l.Charges = []*bill.LineCharge{{Amount: num.MakeAmount(50, 2), Reason: "Ice"}}
		}, "8.50"},
		{"a line charge by quantity and rate", func(l *bill.Line) {
			l.Charges = []*bill.LineCharge{{Quantity: num.NewAmount(2, 0), Rate: num.NewAmount(25, 2), Reason: "Ice"}}
		}, "8.50"},
		{"a line charge by percent", func(l *bill.Line) {
			l.Charges = []*bill.LineCharge{{Percent: num.NewPercentage(10, 2), Reason: "Service"}}
		}, "8.80"},
		{"a list price with a price discount", func(l *bill.Line) {
			l.Item.Price = nil
			l.Item.List = amountPtr("5.00")
			l.Item.Discount = amountPtr("1.00")
		}, "8.00"},
		{"a VAT line with its own percent", func(l *bill.Line) {
			l.Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Percent: num.NewPercentage(25, 2)}
		}, "8.00"},
		{"an exempt VAT line", func(l *bill.Line) {
			l.Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
		}, "8.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := drink()
			tt.edit(l)
			inv := menuInvoice(l)
			require.NoError(t, inv.Calculate())
			assert.Equal(t, tt.payable, inv.Totals.Payable.String())
			require.NoError(t, inv.Calculate())
			assert.Equal(t, tt.payable, inv.Totals.Payable.String(), "after a second calculation")
		})
	}

	t.Run("sub-lines", func(t *testing.T) {
		l := menuLine("Pivo", 1, "4.00", tax.RateGeneral, fiskal.PPIncludedInPrice)
		l.Item.Price = nil
		l.Breakdown = []*bill.SubLine{
			{Quantity: num.MakeAmount(1, 0), Item: &org.Item{Name: "Pivo", Price: amountPtr("3.00")}},
			{Quantity: num.MakeAmount(1, 0), Item: &org.Item{Name: "Limun", Price: amountPtr("1.00")}},
		}
		inv := menuInvoice(l)
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "4.00", inv.Totals.Payable.String())
	})

	t.Run("a draft without an issue date uses today's VAT rate", func(t *testing.T) {
		inv := menuInvoice(drink())
		inv.IssueDate = cal.Date{}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "8.00", inv.Totals.Payable.String())
	})

	t.Run("an unknown VAT rate is left for GOBL to refuse", func(t *testing.T) {
		l := drink()
		l.Taxes[0].Rate = "unknown"
		assert.Error(t, menuInvoice(l).Calculate())
		assert.Equal(t, fiskal.PPIncludedInPrice, l.Taxes[1].Ext.Get(fiskal.ExtKeyPP))
	})
}
