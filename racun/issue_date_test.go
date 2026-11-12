package racun

import (
	"testing"
	"time"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckIssueDate(t *testing.T) {
	// 12 November 2026, 14:00 on a Croatian clock (CET, UTC+1).
	now := time.Date(2026, time.November, 12, 13, 0, 0, 0, time.UTC)
	issuedAt := func(d cal.Date, tm *cal.Time) *bill.Invoice {
		return &bill.Invoice{IssueDate: d, IssueTime: tm}
	}

	tests := []struct {
		name string
		inv  *bill.Invoice
		err  string
	}{
		{name: "issued now", inv: issuedAt(cal.MakeDate(2026, 11, 12), cal.NewTime(14, 0, 0))},
		{name: "issued a minute ago", inv: issuedAt(cal.MakeDate(2026, 11, 12), cal.NewTime(13, 59, 0))},
		{name: "issued a second from now", inv: issuedAt(cal.MakeDate(2026, 11, 12), cal.NewTime(14, 0, 1)), err: "CIS error 177"},
		{name: "issued tomorrow", inv: issuedAt(cal.MakeDate(2026, 11, 13), cal.NewTime(0, 0, 0)), err: "CIS error 177"},
		{name: "issued 30 days ago", inv: issuedAt(cal.MakeDate(2026, 10, 13), cal.NewTime(0, 0, 0))},
		{name: "issued 31 days ago", inv: issuedAt(cal.MakeDate(2026, 10, 12), cal.NewTime(23, 59, 59)), err: "CIS error 176"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckIssueDate(tt.inv, now)
			if tt.err == "" {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrIssueDate)
			assert.ErrorContains(t, err, tt.err)
		})
	}

	t.Run("compares on a Croatian clock", func(t *testing.T) {
		// 23:30 UTC on 12 November is 00:30 on 13 November in Zagreb.
		late := time.Date(2026, time.November, 12, 23, 30, 0, 0, time.UTC)
		inv := issuedAt(cal.MakeDate(2026, 11, 13), cal.NewTime(0, 20, 0))
		assert.NoError(t, CheckIssueDate(inv, late))
	})

	t.Run("in summer time", func(t *testing.T) {
		// 12:00 UTC on 1 July is 14:00 in Zagreb (CEST, UTC+2).
		summer := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
		assert.NoError(t, CheckIssueDate(issuedAt(cal.MakeDate(2026, 7, 1), cal.NewTime(14, 0, 0)), summer))
		assert.ErrorIs(t, CheckIssueDate(issuedAt(cal.MakeDate(2026, 7, 1), cal.NewTime(14, 0, 1)), summer), ErrIssueDate)
	})

	t.Run("without an issue time", func(t *testing.T) {
		err := CheckIssueDate(issuedAt(cal.MakeDate(2026, 11, 12), nil), now)
		assert.ErrorIs(t, err, ErrMissing)
	})

	t.Run("no invoice", func(t *testing.T) {
		assert.ErrorIs(t, CheckIssueDate(nil, now), ErrNotInvoice)
	})
}
