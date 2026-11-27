package fiskal_test

import (
	"strings"
	"testing"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
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
				fiskal.ExtKeyPremises: "SHOP1",
				fiskal.ExtKeyDevice:   "1",
			}),
		},
		Supplier: &org.Party{
			Name:  "Invopop d.o.o.",
			TaxID: &tax.Identity{Country: "HR", Code: supplierOIB},
			Ext:   tax.ExtensionsOf(cbc.CodeMap{fiskal.ExtKeyVATStatus: fiskal.VATStatusRegistered}),
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
	assert.True(t, fiskal.RegisteredForVAT(inv.Supplier))
	assert.Equal(t, cbc.Code(supplierOIB), fiskal.PartyOIB(inv.Supplier))
	assert.Empty(t, inv.Supplier.Identities)
	assert.Empty(t, inv.Tax.Notes)
}

func TestNormalizeKeepsExtensions(t *testing.T) {
	inv := testInvoice()
	inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeySequenceScope, fiskal.SequenceDevice)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, fiskal.SequenceDevice, inv.Tax.Ext.Get(fiskal.ExtKeySequenceScope))
}

// smallTaxpayer is testInvoice from a supplier not registered for VAT, with
// its line exempt.
func smallTaxpayer() *bill.Invoice {
	inv := testInvoice()
	notRegistered(inv)
	inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
	return inv
}

func TestSmallTaxpayer(t *testing.T) {
	t.Run("an exempt line, with the note", func(t *testing.T) {
		inv := smallTaxpayer()
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.False(t, fiskal.RegisteredForVAT(inv.Supplier))
		assert.Equal(t, cbc.Code(supplierOIB), fiskal.PartyOIB(inv.Supplier))
		assert.Equal(t, "HR"+supplierOIB, inv.Supplier.TaxID.String(), "the sequence step counts per tax ID, so each small taxpayer needs its own")
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

func TestIssueTimeRequired(t *testing.T) {
	t.Run("dated today without a time", func(t *testing.T) {
		inv := testInvoice()
		inv.IssueDate = cal.TodayIn(tax.RegimeDefFor(l10n.HR).TimeLocation())
		inv.IssueTime = nil
		assertFault(t, inv, faultCode("02"))
		assert.Nil(t, inv.IssueTime, "the addon must not fill in the time")
	})

	t.Run("not dated and without a time", func(t *testing.T) {
		inv := testInvoice()
		inv.IssueDate = cal.Date{}
		inv.IssueTime = nil
		assertFault(t, inv, faultCode("02"))
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
		{"supplier tax ID without its OIB", func(inv *bill.Invoice) { inv.Supplier.TaxID.Code = "" }, "13"},
		{"supplier tax ID without its OIB, with an OIB identity", func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = ""
			inv.Supplier.Identities = []*org.Identity{{Type: hr.IdentityTypeOIB, Code: supplierOIB}}
		}, "13"},
		{"supplier with no tax ID", func(inv *bill.Invoice) { inv.Supplier.TaxID = nil }, "11"},
		{"supplier without its VAT status", func(inv *bill.Invoice) { inv.Supplier.Ext = inv.Supplier.Ext.Delete(fiskal.ExtKeyVATStatus) }, "30"},
		{"unknown VAT status", func(inv *bill.Invoice) { inv.Supplier.Ext = inv.Supplier.Ext.Set(fiskal.ExtKeyVATStatus, "exempt") }, "31"},
		{"Croatian business customer without an OIB", func(inv *bill.Invoice) {
			inv.Customer = &org.Party{Name: "Kupac d.o.o.", TaxID: &tax.Identity{Country: "HR"}}
		}, "29"},
		{"Croatian business customer with its OIB only as an identity", func(inv *bill.Invoice) {
			inv.Customer = &org.Party{
				Name:       "Kupac d.o.o.",
				TaxID:      &tax.Identity{Country: "HR"},
				Identities: []*org.Identity{{Type: hr.IdentityTypeOIB, Code: "93476558444"}},
			}
		}, "29"},
		{"no operator", func(inv *bill.Invoice) { inv.Supplier.People = nil }, "14"},
		{"two operators", func(inv *bill.Invoice) {
			inv.Supplier.People = append(inv.Supplier.People, inv.Supplier.People[0])
		}, "14"},
		{"operator without an OIB", func(inv *bill.Invoice) { inv.Supplier.People[0].Identities = nil }, "15"},
		{"no payment", func(inv *bill.Invoice) { inv.Payment = nil }, "34"},
		{"a payment with no advance or instructions", func(inv *bill.Invoice) { inv.Payment = &bill.PaymentDetails{} }, "34"},
		{"an advance without a payment means", func(inv *bill.Invoice) { inv.Payment.Advances[0].Key = "" }, "35"},
		{"a credit note", func(inv *bill.Invoice) { inv.Type = bill.InvoiceTypeCreditNote }, "18"},
		{"a negative total", func(inv *bill.Invoice) { inv.Lines[0].Item.Price = num.NewAmount(-250, 2) }, "19"},
		{"a fee without a reason", func(inv *bill.Invoice) {
			inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(10, 2)}}
		}, "26"},
		{"a fee with a reason longer than 100 characters", func(inv *bill.Invoice) {
			inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(10, 2), Reason: strings.Repeat("ž", 101)}}
		}, "26"},
		{"a tax as a charge without taxes", func(inv *bill.Invoice) {
			inv.Charges = []*bill.Charge{{Key: bill.ChargeKeyTax, Amount: num.MakeAmount(2000, 0), Reason: "PPMV"}}
		}, "27"},
		{"a discount without taxes", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Amount: num.MakeAmount(10, 2), Reason: "Promo"}}
		}, "22"},
		{"a discount with only PP", func(inv *bill.Invoice) {
			inv.Discounts = []*bill.Discount{{Amount: num.MakeAmount(10, 2), Reason: "Promo", Taxes: ppOnly()}}
		}, "22"},
		{"a line without taxes", func(inv *bill.Invoice) { inv.Lines[0].Taxes = nil }, "23"},
		{"a line with only PP", func(inv *bill.Invoice) { inv.Lines[0].Taxes = ppOnly() }, "23"},
		{"VAT from a supplier not registered for VAT", func(inv *bill.Invoice) { notRegistered(inv) }, "16"},
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

	t.Run("Croatian business customer not registered for VAT, with its OIB in the tax ID", func(t *testing.T) {
		inv := testInvoice()
		inv.Customer = &org.Party{Name: "Kupac d.o.o.", TaxID: &tax.Identity{Country: "HR", Code: "93476558444"}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("payment instructions with no advance, for a bank transfer", func(t *testing.T) {
		inv := testInvoice()
		inv.Payment = &bill.PaymentDetails{Instructions: &pay.Instructions{Key: pay.MeansKeyCreditTransfer}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("private customer with no tax ID", func(t *testing.T) {
		inv := testInvoice()
		inv.Customer = &org.Party{Name: "Ana Kovač"}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("another currency with an exchange rate to EUR", func(t *testing.T) {
		inv := testInvoice()
		inv.Currency = "USD"
		inv.ExchangeRates = []*currency.ExchangeRate{{From: "USD", To: "EUR", Amount: num.MakeAmount(875, 3)}}
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
		assert.Equal(t, cbc.Code("SHOP1"), inv.Series)
	})
	t.Run("series per device", func(t *testing.T) {
		inv := testInvoice()
		inv.Tax.Ext = inv.Tax.Ext.Set(fiskal.ExtKeySequenceScope, fiskal.SequenceDevice)
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
		assert.Equal(t, cbc.Code("SHOP1/1"), inv.Series)
	})
	t.Run("a series that matches the labels", func(t *testing.T) {
		inv := testInvoice()
		inv.Series = "SHOP1"
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
		inv.Series = "SHOP1/1"
		assertFault(t, inv, faultCode("17"))
	})
}

func TestSeries(t *testing.T) {
	ext := func(m cbc.CodeMap) tax.Extensions { return tax.ExtensionsOf(m) }
	assert.Equal(t, cbc.Code("SHOP1"), fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyPremises: "SHOP1", fiskal.ExtKeyDevice: "2"})))
	assert.Equal(t, cbc.Code("SHOP1/2"), fiskal.Series(ext(cbc.CodeMap{
		fiskal.ExtKeyPremises: "SHOP1", fiskal.ExtKeyDevice: "2", fiskal.ExtKeySequenceScope: fiskal.SequenceDevice,
	})))
	assert.Empty(t, fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyDevice: "2"})))
	assert.Empty(t, fiskal.Series(ext(cbc.CodeMap{fiskal.ExtKeyPremises: "SHOP1", fiskal.ExtKeySequenceScope: fiskal.SequenceDevice})))
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

// ppOnly is a tax set with the consumption tax and no VAT.
func ppOnly() tax.Set {
	return tax.Set{{
		Category: hr.TaxCategoryPP,
		Percent:  num.NewPercentage(3, 2),
		Ext:      tax.ExtensionsOf(cbc.CodeMap{fiskal.ExtKeyPP: fiskal.PPExcludedFromPrice}),
	}}
}

func TestConsumptionTax(t *testing.T) {
	ppLine := func(p *num.Percentage) *bill.Invoice {
		inv := testInvoice()
		inv.Lines[0].Taxes = tax.Set{
			{Category: tax.CategoryVAT, Rate: tax.RateGeneral},
			{Category: hr.TaxCategoryPP, Percent: p},
		}
		return inv
	}
	for _, p := range []num.Percentage{
		num.MakePercentage(0, 2),
		num.MakePercentage(15, 3), // 1.5 %, as in Rijeka
		num.MakePercentage(3, 2),
	} {
		t.Run("accepts "+p.String(), func(t *testing.T) {
			inv := ppLine(&p)
			require.NoError(t, inv.Calculate())
			require.NoError(t, rules.Validate(inv))
		})
	}
	ppFault := rules.GOBL.Add(fiskal.Namespace).Add("TAX-COMBO").Add("04")
	for _, p := range []num.Percentage{num.MakePercentage(-1, 2), num.MakePercentage(35, 3)} {
		t.Run("refuses "+p.String(), func(t *testing.T) {
			assertFault(t, ppLine(&p), ppFault)
		})
	}
	t.Run("refuses a line without a percent", func(t *testing.T) {
		assertFault(t, ppLine(nil), ppFault)
	})
	t.Run("does not check the PP percent against the VAT rates", func(t *testing.T) {
		p := num.MakePercentage(3, 2)
		inv := ppLine(&p)
		require.NoError(t, inv.Calculate())
		faults := rules.Validate(inv)
		assert.False(t, faults != nil && faults.HasCode(rules.GOBL.Add(fiskal.Namespace).Add("TAX-COMBO").Add("03")), "got %s", faults)
	})
}

// A charge without taxes is a fee, such as a bottle deposit ("povratna
// naknada"), reported by its reason and amount.
func TestFees(t *testing.T) {
	t.Run("accepts a fee with a reason", func(t *testing.T) {
		inv := testInvoice()
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(60, 2), Reason: "Povratna naknada"}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
	t.Run("accepts a reason of 100 characters", func(t *testing.T) {
		inv := testInvoice()
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(60, 2), Reason: strings.Repeat("ž", 100)}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
	t.Run("an informative charge is not a fee", func(t *testing.T) {
		inv := testInvoice()
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(60, 2), Informative: true}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
}

// notRegistered makes the supplier one not registered for VAT. Its tax ID
// keeps its OIB.
func notRegistered(inv *bill.Invoice) {
	inv.Supplier.Ext = inv.Supplier.Ext.Set(fiskal.ExtKeyVATStatus, fiskal.VATStatusNotRegistered)
}

// TestPartyOIB checks that the OIB comes only from the Croatian tax ID.
func TestPartyOIB(t *testing.T) {
	oib := func(code cbc.Code) []*org.Identity { return []*org.Identity{{Type: hr.IdentityTypeOIB, Code: code}} }
	tests := []struct {
		name  string
		party *org.Party
		want  cbc.Code
	}{
		{"no party", nil, ""},
		{"tax ID", &org.Party{TaxID: &tax.Identity{Country: "HR", Code: "80432461483"}}, "80432461483"},
		{"tax ID with the HR prefix", &org.Party{TaxID: &tax.Identity{Country: "HR", Code: "HR80432461483"}}, "80432461483"},
		{"tax ID before an OIB identity", &org.Party{TaxID: &tax.Identity{Country: "HR", Code: "80432461483"}, Identities: oib("12345678903")}, "80432461483"},
		{"only an OIB identity", &org.Party{TaxID: &tax.Identity{Country: "HR"}, Identities: oib("80432461483")}, ""},
		{"foreign tax ID", &org.Party{TaxID: &tax.Identity{Country: "SI", Code: "92950663"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fiskal.PartyOIB(tt.party))
		})
	}
}

// TestSupplierFaults checks that one mistake in the supplier's tax ID gives
// one fault: a foreign tax ID, or a missing one, is not also missing an OIB.
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
		assert.False(t, faults.HasCode(faultCode("13")), "got %s", faults)
	})
}

// TestSupplierOIBIdentityIgnored checks that a supplier may also give its OIB
// as an identity, as the hr-eracun-v1 addon asks of one not registered for
// VAT: the addon reads only the tax ID.
func TestSupplierOIBIdentityIgnored(t *testing.T) {
	inv := testInvoice()
	notRegistered(inv)
	inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyExempt}
	inv.Supplier.Identities = []*org.Identity{{Type: hr.IdentityTypeOIB, Code: "12345678903"}}
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	assert.Equal(t, cbc.Code(supplierOIB), fiskal.PartyOIB(inv.Supplier))
}
