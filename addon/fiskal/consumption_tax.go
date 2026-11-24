package fiskal

import (
	"slices"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/tax"
)

// ppRemovalAccuracy is the number of decimal places added to a price when PP
// is taken out of it, so that the invoice's total stays the menu price.
const ppRemovalAccuracy = 4

// removeIncludedPP takes the consumption tax (PP) out of every amount whose
// PP has ExtKeyPP set to included-in-price, and sets it to
// excluded-from-price, so a second calculation changes nothing: the price of
// a line, and the amount of a discount or charge on a line or on the whole
// invoice. A menu price includes VAT and PP on one base, the price without
// VAT (Local Taxes Act, Art. 23(2)), but prices_include names one category
// only. With prices_include VAT, the amount keeps its VAT, which GOBL takes
// out as usual: a 30.00 menu price becomes 30.00 × 1.25 / 1.28.
func removeIncludedPP(inv *bill.Invoice) {
	for _, l := range inv.Lines {
		withIncludedPP(inv, l.Taxes, func(remove ppRemoval) { removePP(l, remove) })
	}
	for _, d := range inv.Discounts {
		withIncludedPP(inv, d.Taxes, func(remove ppRemoval) {
			d.Base, d.Amount = removeOrderPP(d.Percent, d.Base, d.Amount, remove)
		})
	}
	for _, c := range inv.Charges {
		withIncludedPP(inv, c.Taxes, func(remove ppRemoval) {
			c.Base, c.Amount = removeOrderPP(c.Percent, c.Base, c.Amount, remove)
		})
	}
}

// ppRemoval takes PP out of one amount that includes it.
type ppRemoval func(num.Amount) num.Amount

// withIncludedPP calls apply with the removal for taxes whose PP is included
// in the amount, then marks that PP excluded-from-price. It does nothing for
// taxes without such a PP.
func withIncludedPP(inv *bill.Invoice, taxes tax.Set, apply func(ppRemoval)) {
	pp := taxes.Get(hr.TaxCategoryPP)
	if pp == nil || pp.Percent == nil || pp.Ext.Get(ExtKeyPP) != PPIncludedInPrice {
		return
	}
	included := num.MakePercentage(0, 2)
	if inv.Tax.PricesInclude == tax.CategoryVAT {
		vat, ok := vatPercent(inv, taxes.Get(tax.CategoryVAT))
		if !ok {
			return // GOBL refuses a VAT rate it can't find
		}
		included = vat
	}
	keep := included.Factor()
	with := keep.RescaleUp(pp.Percent.Exp()).Add(pp.Percent.Base())
	apply(func(a num.Amount) num.Amount {
		return a.Upscale(ppRemovalAccuracy).Multiply(keep).Divide(with)
	})
	pp.Ext = pp.Ext.Set(ExtKeyPP, PPExcludedFromPrice)
}

// removeOrderPP takes PP out of a discount or charge on the whole invoice:
// its fixed amount, or the base it gives for its percent. A percent with no
// base of its own applies to the line totals, which have no PP left.
func removeOrderPP(percent *num.Percentage, base *num.Amount, amount num.Amount, remove ppRemoval) (*num.Amount, num.Amount) {
	switch {
	case percent == nil:
		return base, remove(amount)
	case base != nil:
		b := remove(*base)
		return &b, amount
	}
	return base, amount
}

// vatPercent is the VAT percent of a line: its own, or the regime's for its
// rate on the issue date. GOBL fills in a combo's percent only after the
// normalizers run. No VAT, or an exempt key, has 0 %.
func vatPercent(inv *bill.Invoice, vat *tax.Combo) (num.Percentage, bool) {
	zero := num.MakePercentage(0, 2)
	if vat == nil {
		return zero, true
	}
	if vat.Percent != nil {
		return *vat.Percent, true
	}
	if vat.Rate.IsEmpty() {
		return zero, true
	}
	r := inv.RegimeDef()
	if r == nil {
		return zero, false
	}
	rate := r.CategoryDef(tax.CategoryVAT).RateDef(vat.Key, vat.Rate)
	if rate == nil {
		return zero, false
	}
	date := inv.IssueDate
	if date.IsZero() {
		date = cal.TodayIn(r.TimeLocation())
	}
	value := rate.Value(date, vat.Ext)
	if value == nil {
		return zero, false
	}
	return value.Percent, true
}

// removePP applies remove to every amount of the line that includes PP:
// the item's prices, and the fixed amounts of its discounts and charges. A
// discount or charge given as a percent follows the price.
func removePP(l *bill.Line, remove ppRemoval) {
	removeItemPP(l.Item, remove)
	removeLineDiscountsPP(l.Discounts, remove)
	removeLineChargesPP(l.Charges, remove)
	for _, sl := range slices.Concat(l.Breakdown, l.Substituted) {
		removeItemPP(sl.Item, remove)
		removeLineDiscountsPP(sl.Discounts, remove)
		removeLineChargesPP(sl.Charges, remove)
	}
}

func removeItemPP(item *org.Item, remove ppRemoval) {
	if item == nil {
		return
	}
	for _, p := range []**num.Amount{&item.Price, &item.List, &item.Discount} {
		if *p != nil {
			v := remove(**p)
			*p = &v
		}
	}
}

func removeLineDiscountsPP(ds []*bill.LineDiscount, remove ppRemoval) {
	for _, d := range ds {
		if d.Percent == nil {
			d.Amount = remove(d.Amount)
		}
	}
}

func removeLineChargesPP(cs []*bill.LineCharge, remove ppRemoval) {
	for _, c := range cs {
		switch {
		case c.Percent != nil:
		case c.Rate != nil:
			v := remove(*c.Rate)
			c.Rate = &v
		default:
			c.Amount = remove(c.Amount)
		}
	}
}

func pricesIncludeVAT(val any) bool {
	inv, ok := val.(*bill.Invoice)
	return ok && inv != nil && inv.Tax != nil && inv.Tax.PricesInclude == tax.CategoryVAT
}

// ppStatesPrice reports whether the PP of the taxes of a line, discount or
// charge, if any, says if the price or amount includes it.
func ppStatesPrice(val any) bool {
	taxes, ok := val.(tax.Set)
	if !ok {
		return true
	}
	pp := taxes.Get(hr.TaxCategoryPP)
	return pp == nil || pp.Ext.Has(ExtKeyPP)
}
