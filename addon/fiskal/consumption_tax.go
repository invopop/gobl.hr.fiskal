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

// removeIncludedPP takes the consumption tax (PP) out of the price of each
// line whose PP has ExtKeyPP set to included-in-price, and sets it to
// excluded-from-price, so a second calculation changes nothing. A menu price includes VAT and PP
// on one base, the price without VAT (Local Taxes Act, Art. 23(2)), but
// prices_include names one category only. With prices_include VAT, the
// price keeps its VAT, which GOBL takes out as usual: a 30.00 menu price
// becomes 30.00 × 1.25 / 1.28.
func removeIncludedPP(inv *bill.Invoice) {
	for _, l := range inv.Lines {
		pp := l.Taxes.Get(hr.TaxCategoryPP)
		if pp == nil || pp.Percent == nil || pp.Ext.Get(ExtKeyPP) != PPIncludedInPrice {
			continue
		}
		included := num.MakePercentage(0, 2)
		if inv.Tax.PricesInclude == tax.CategoryVAT {
			vat, ok := vatPercent(inv, l.Taxes.Get(tax.CategoryVAT))
			if !ok {
				continue // GOBL refuses a VAT rate it can't find
			}
			included = vat
		}
		keep := included.Factor()
		with := keep.RescaleUp(pp.Percent.Exp()).Add(pp.Percent.Base())
		removePP(l, func(a num.Amount) num.Amount {
			return a.Upscale(ppRemovalAccuracy).Multiply(keep).Divide(with)
		})
		pp.Ext = pp.Ext.Set(ExtKeyPP, PPExcludedFromPrice)
	}
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
func removePP(l *bill.Line, remove func(num.Amount) num.Amount) {
	removeItemPP(l.Item, remove)
	removeLineDiscountsPP(l.Discounts, remove)
	removeLineChargesPP(l.Charges, remove)
	for _, sl := range slices.Concat(l.Breakdown, l.Substituted) {
		removeItemPP(sl.Item, remove)
		removeLineDiscountsPP(sl.Discounts, remove)
		removeLineChargesPP(sl.Charges, remove)
	}
}

func removeItemPP(item *org.Item, remove func(num.Amount) num.Amount) {
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

func removeLineDiscountsPP(ds []*bill.LineDiscount, remove func(num.Amount) num.Amount) {
	for _, d := range ds {
		if d.Percent == nil {
			d.Amount = remove(d.Amount)
		}
	}
}

func removeLineChargesPP(cs []*bill.LineCharge, remove func(num.Amount) num.Amount) {
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

// ppStatesPrice reports whether the PP of a line's taxes, if any, says if
// the price includes it.
func ppStatesPrice(val any) bool {
	taxes, ok := val.(tax.Set)
	if !ok {
		return true
	}
	pp := taxes.Get(hr.TaxCategoryPP)
	return pp == nil || pp.Ext.Has(ExtKeyPP)
}
