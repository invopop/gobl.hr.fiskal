package fiskal_test

import (
	"testing"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/hr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const supplierOIB = "80432461483"

func testInvoice() *bill.Invoice {
	return &bill.Invoice{
		Regime:    tax.WithRegime("HR"),
		Addons:    tax.WithAddons(fiskal.V1),
		Code:      "1001",
		Currency:  "EUR",
		IssueDate: cal.MakeDate(2026, 11, 6),
		IssueTime: cal.NewTime(13, 58, 10),
		Tax: &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				fiskal.ExtKeyPremises: "POS1",
				fiskal.ExtKeyDevice:   "1",
			}),
		},
		Supplier: &org.Party{
			Name:  "Invopop d.o.o.",
			TaxID: &tax.Identity{Country: "HR", Code: supplierOIB},
			People: []*org.Person{
				{
					Key:        fiskal.PersonKeyOperator,
					Name:       &org.Name{Given: "Operater"},
					Identities: []*org.Identity{{Type: hr.IdentityTypeOIB, Code: supplierOIB}},
				},
			},
		},
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(2, 0),
				Item:     &org.Item{Name: "Kava", Price: num.NewAmount(250, 2)},
				Taxes:    tax.Set{{Category: tax.CategoryVAT, Rate: tax.RateGeneral}},
			},
		},
		Payment: &bill.PaymentDetails{
			Advances: []*pay.Record{{Key: pay.MeansKeyCash, Description: "Gotovina", Percent: num.NewPercentage(100, 2)}},
		},
	}
}

func assertFault(t *testing.T, inv *bill.Invoice, code rules.Code) {
	t.Helper()
	require.NoError(t, inv.Calculate())
	faults := rules.Validate(inv)
	require.Error(t, faults)
	assert.True(t, faults.HasCode(code), "want %s, got %s", code, faults)
}

func faultCode(n string) rules.Code {
	return rules.GOBL.Add(fiskal.Namespace).Add("BILL-INVOICE").Add(rules.Code(n))
}

func TestAddon(t *testing.T) {
	ad := tax.AddonForKey(fiskal.V1)
	require.NotNil(t, ad)
	assert.Equal(t, fiskal.V1, ad.Key)
}

func TestInvoiceValid(t *testing.T) {
	inv := testInvoice()
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, fiskal.SequencePremises, inv.Tax.Ext.Get(fiskal.ExtKeySequenceScope))
	assert.True(t, fiskal.PartyHasVATNumber(inv.Supplier))
	assert.Equal(t, cbc.Code(supplierOIB), fiskal.PartyOIB(inv.Supplier))
	assert.Empty(t, inv.Supplier.Identities, "the OIB stays in the VAT number")
	assert.Empty(t, inv.Supplier.Ext)
	assert.Empty(t, inv.Tax.Notes)
}

func TestNormalizeKeepsExtensions(t *testing.T) {
	inv := testInvoice()
	inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeySequenceScope, fiskal.SequenceDevice)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, fiskal.SequenceDevice, inv.Tax.Ext.Get(fiskal.ExtKeySequenceScope))
}

// smallTaxpayer is testInvoice from a supplier outside the VAT system, with
// its line exempt.
func smallTaxpayer() *bill.Invoice {
	inv := testInvoice()
	outsideVAT(inv)
	inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
	return inv
}

func TestSmallTaxpayer(t *testing.T) {
	t.Run("an exempt line, with the note", func(t *testing.T) {
		inv := smallTaxpayer()
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.False(t, fiskal.PartyHasVATNumber(inv.Supplier))
		assert.Equal(t, cbc.Code(supplierOIB), fiskal.PartyOIB(inv.Supplier))
		assert.Equal(t, "HR", inv.GetRegime().String(), "a tax ID with only the country keeps the regime")
		require.Len(t, inv.Tax.Notes, 1)
		assert.Equal(t, tax.KeyExempt, inv.Tax.Notes[0].Key)
		assert.Equal(t, fiskal.NoteSmallTaxpayer, inv.Tax.Notes[0].Text)
	})
	t.Run("an exempt note of its own", func(t *testing.T) {
		inv := smallTaxpayer()
		inv.Tax.Notes = []*tax.Note{{Category: tax.CategoryVAT, Key: tax.KeyExempt, Text: "Oslobođeno PDV-a"}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Tax.Notes, 1)
		assert.Equal(t, "Oslobođeno PDV-a", inv.Tax.Notes[0].Text)
	})
	t.Run("an outside-scope line, without the note", func(t *testing.T) {
		inv := smallTaxpayer()
		inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyOutsideScope}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Empty(t, inv.Tax.Notes)
	})
	t.Run("an exempt charge, with the note", func(t *testing.T) {
		inv := smallTaxpayer()
		inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyOutsideScope}
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(100, 2), Reason: "Dostava", Taxes: tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Tax.Notes, 1)
		assert.Equal(t, fiskal.NoteSmallTaxpayer, inv.Tax.Notes[0].Text)
	})
	t.Run("a line with VAT", func(t *testing.T) {
		inv := smallTaxpayer()
		inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Rate: tax.RateGeneral}
		assertFault(t, inv, faultCode("16"))
	})
}

func TestIssueTimeOptional(t *testing.T) {
	t.Run("dated today without a time", func(t *testing.T) {
		inv := testInvoice()
		inv.IssueDate = cal.TodayIn(tax.RegimeDefFor(l10n.HR).TimeLocation())
		inv.IssueTime = nil
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Nil(t, inv.IssueTime, "the addon must not fill in the time")
	})

	t.Run("not dated and without a time", func(t *testing.T) {
		inv := testInvoice()
		inv.IssueDate = cal.Date{}
		inv.IssueTime = nil
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Nil(t, inv.IssueTime, "the addon must not fill in the time")
	})

	t.Run("at midnight", func(t *testing.T) {
		inv := testInvoice()
		inv.IssueTime = cal.NewTime(0, 0, 0)
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
}

func TestInvoiceRules(t *testing.T) {
	tests := []struct {
		name string
		edit func(*bill.Invoice)
		code string
	}{
		{"code with a series letter", func(inv *bill.Invoice) { inv.Code = "A1" }, "01"},
		{"code with only zeros", func(inv *bill.Invoice) { inv.Code = "000" }, "01"},
		{"padded code with a series letter", func(inv *bill.Invoice) { inv.Code = "A0001" }, "01"},
		{"no premises", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Delete(fiskal.ExtKeyPremises) }, "05"},
		{"no device", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Delete(fiskal.ExtKeyDevice) }, "05"},
		{"premises with a dash", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeyPremises, "POS-1") }, "06"},
		{"device with letters", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeyDevice, "T1") }, "07"},
		{"device with a leading zero", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeyDevice, "01") }, "07"},
		{"another currency without an exchange rate", func(inv *bill.Invoice) { inv.Currency = "USD" }, "03"},
		{"unknown sequence scope", func(inv *bill.Invoice) { inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeySequenceScope, "X") }, "08"},
		{"foreign supplier", func(inv *bill.Invoice) {
			inv.Supplier.TaxID = &tax.Identity{Country: "SI", Code: "92950663"}
		}, "12"},
		{"supplier with no VAT number and no OIB identity", func(inv *bill.Invoice) { inv.Supplier.TaxID.Code = "" }, "13"},
		{"supplier with no tax ID", func(inv *bill.Invoice) { outsideVAT(inv); inv.Supplier.TaxID = nil }, "11"},
		{"no operator", func(inv *bill.Invoice) { inv.Supplier.People = nil }, "14"},
		{"two operators", func(inv *bill.Invoice) {
			inv.Supplier.People = append(inv.Supplier.People, inv.Supplier.People[0])
		}, "14"},
		{"operator without an OIB", func(inv *bill.Invoice) { inv.Supplier.People[0].Identities = nil }, "15"},
		{"a credit note", func(inv *bill.Invoice) { inv.Type = bill.InvoiceTypeCreditNote }, "18"},
		{"a negative total", func(inv *bill.Invoice) { inv.Lines[0].Item.Price = num.NewAmount(-250, 2) }, "19"},
		{"a charge without taxes", func(inv *bill.Invoice) {
			inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(10, 2), Reason: "Povratna naknada"}}
		}, "21"},
		{"a discount without taxes", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Amount: num.MakeAmount(10, 2), Reason: "Promo"}}
		}, "22"},
		{"a line without taxes", func(inv *bill.Invoice) { inv.Lines[0].Taxes = nil }, "23"},
		{"VAT outside the VAT system", func(inv *bill.Invoice) { outsideVAT(inv) }, "16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := testInvoice()
			tt.edit(inv)
			assertFault(t, inv, faultCode(tt.code))
		})
	}

	t.Run("operator OIB with a wrong control digit", func(t *testing.T) {
		inv := testInvoice()
		inv.Supplier.People[0].Identities[0].Code = "80432461484"
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checksum")
	})

	t.Run("another currency with an exchange rate to EUR", func(t *testing.T) {
		inv := testInvoice()
		inv.Currency = "USD"
		inv.ExchangeRates = []*currency.ExchangeRate{{From: "USD", To: "EUR", Amount: num.MakeAmount(875, 3)}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("paper number", func(t *testing.T) {
		inv := testInvoice()
		inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeyPaperNumber, "123/458/5")
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
}

// TestNumbering covers both ways an invoice gets its number: a till that
// numbers its own invoices sends the code, and Invopop numbers a draft
// without one.
func TestNumbering(t *testing.T) {
	t.Run("a draft without a code", func(t *testing.T) {
		inv := testInvoice()
		inv.Code = ""
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, cbc.Code("POS1"), inv.Series)
	})
	t.Run("series per device", func(t *testing.T) {
		inv := testInvoice()
		inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeySequenceScope, fiskal.SequenceDevice)
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, cbc.Code("POS1/1"), inv.Series)
	})
	t.Run("a series that matches the labels", func(t *testing.T) {
		inv := testInvoice()
		inv.Series = "POS1"
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
	t.Run("a code padded by the sequence step", func(t *testing.T) {
		inv := testInvoice()
		inv.Code = "000042"
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, cbc.Code("42"), inv.Code)
	})
	t.Run("a series of another premises", func(t *testing.T) {
		inv := testInvoice()
		inv.Series = "POS2"
		assertFault(t, inv, faultCode("17"))
	})
	t.Run("a series per device with scope P", func(t *testing.T) {
		inv := testInvoice()
		inv.Series = "POS1/1"
		assertFault(t, inv, faultCode("17"))
	})
}

func TestSeries(t *testing.T) {
	ext := func(m cbc.CodeMap) tax.Extensions { return tax.ExtensionsOf(m) }
	assert.Equal(t, cbc.Code("POS1"), fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyPremises: "POS1", fiskal.ExtKeyDevice: "2"})))
	assert.Equal(t, cbc.Code("POS1/2"), fiskal.Series(ext(cbc.CodeMap{
		fiskal.ExtKeyPremises: "POS1", fiskal.ExtKeyDevice: "2", fiskal.ExtKeySequenceScope: fiskal.SequenceDevice,
	})))
	assert.Empty(t, fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyDevice: "2"})))
	assert.Empty(t, fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyPremises: "POS1", fiskal.ExtKeySequenceScope: fiskal.SequenceDevice})))
}

func TestRounding(t *testing.T) {
	inv := testInvoice()
	require.NoError(t, inv.Calculate())
	r := num.MakeAmount(-5, 2)
	inv.Totals.Rounding = &r
	require.NoError(t, inv.Calculate())
	faults := rules.Validate(inv)
	require.Error(t, faults)
	assert.True(t, faults.HasCode(faultCode("20")), "got %s", faults)
}

func TestTaxComboRules(t *testing.T) {
	inv := testInvoice()
	inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyReverseCharge}}
	require.NoError(t, inv.Calculate())
	faults := rules.Validate(inv)
	require.Error(t, faults)
	assert.True(t, faults.HasCode(rules.GOBL.Add(fiskal.Namespace).Add("TAX-COMBO").Add("02")), "got %s", faults)
}

func TestVATPercent(t *testing.T) {
	for _, p := range []num.Percentage{
		num.MakePercentage(0, 2),
		num.MakePercentage(5, 2),
		num.MakePercentage(130, 3), // 13.0 %, as the regime writes it
		num.MakePercentage(25, 2),
	} {
		t.Run("accepts "+p.String(), func(t *testing.T) {
			inv := testInvoice()
			inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Percent: &p}}
			require.NoError(t, inv.Calculate())
			require.NoError(t, rules.Validate(inv))
		})
	}
	for _, p := range []num.Percentage{num.MakePercentage(10, 2), num.MakePercentage(255, 3)} {
		t.Run("refuses "+p.String(), func(t *testing.T) {
			inv := testInvoice()
			inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Percent: &p}}
			require.NoError(t, inv.Calculate())
			faults := rules.Validate(inv)
			require.Error(t, faults)
			assert.True(t, faults.HasCode(rules.GOBL.Add(fiskal.Namespace).Add("TAX-COMBO").Add("03")), "got %s", faults)
		})
	}
	t.Run("accepts an exempt line without a percent", func(t *testing.T) {
		inv := testInvoice()
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
}

// eracunStandIn registers an addon with the eRačun key, as this module does
// not import gobl.hr.eracun. GOBL refuses an invoice with an addon it does
// not know.
var eracunStandIn = func() cbc.Key {
	k := cbc.Key("hr-eracun-v1")
	tax.RegisterAddonDef(&tax.AddonDef{Key: k, Name: i18n.NewString("Croatia eRačun (stand-in)")})
	return k
}()

func TestNotAnEracun(t *testing.T) {
	inv := testInvoice()
	inv.Addons = tax.WithAddons(fiskal.V1, eracunStandIn)
	assertFault(t, inv, faultCode("24"))
}

// outsideVAT makes the supplier one outside the VAT system: a tax ID with
// only the country HR, and its OIB as an identity.
func outsideVAT(inv *bill.Invoice) {
	inv.Supplier.TaxID = &tax.Identity{Country: "HR"}
	inv.Supplier.Identities = []*org.Identity{{Type: hr.IdentityTypeOIB, Code: supplierOIB}}
}

// TestPartyOIB checks where the issuer's OIB comes from: the Croatian VAT
// number, or else the OIB identity.
func TestPartyOIB(t *testing.T) {
	oib := func(code cbc.Code) []*org.Identity { return []*org.Identity{{Type: hr.IdentityTypeOIB, Code: code}} }
	tests := []struct {
		name  string
		party *org.Party
		want  cbc.Code
	}{
		{"no party", nil, ""},
		{"VAT number", &org.Party{TaxID: &tax.Identity{Country: "HR", Code: "HR80432461483"}}, "80432461483"},
		{"OIB identity with no VAT number", &org.Party{TaxID: &tax.Identity{Country: "HR"}, Identities: oib("HR 80432461483")}, "80432461483"},
		{"VAT number before the OIB identity", &org.Party{TaxID: &tax.Identity{Country: "HR", Code: "80432461483"}, Identities: oib("12345678903")}, "80432461483"},
		{"foreign VAT number", &org.Party{TaxID: &tax.Identity{Country: "SI", Code: "92950663"}}, ""},
		{"neither", &org.Party{TaxID: &tax.Identity{Country: "HR"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fiskal.PartyOIB(tt.party))
		})
	}
}

// TestSupplierFaults checks that one mistake in the supplier's tax ID gives
// one fault: a foreign tax ID is not also missing an OIB, and a missing tax
// ID is not taken for a supplier outside the VAT system.
func TestSupplierFaults(t *testing.T) {
	t.Run("foreign tax ID", func(t *testing.T) {
		inv := testInvoice()
		inv.Supplier.TaxID = &tax.Identity{Country: "SI", Code: "92950663"}
		require.NoError(t, inv.Calculate())
		faults := rules.Validate(inv)
		require.Error(t, faults)
		assert.True(t, faults.HasCode(faultCode("12")), "got %s", faults)
		assert.False(t, faults.HasCode(faultCode("13")), "got %s", faults)
	})
	t.Run("no tax ID", func(t *testing.T) {
		inv := testInvoice()
		inv.Supplier.TaxID = nil
		require.NoError(t, inv.Calculate())
		faults := rules.Validate(inv)
		require.Error(t, faults)
		assert.True(t, faults.HasCode(faultCode("11")), "got %s", faults)
		assert.False(t, faults.HasCode(faultCode("16")), "got %s", faults)
		assert.Empty(t, inv.Tax.Notes)
	})
}
