package racun

import (
	"errors"
	"fmt"
	"time"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
)

// ErrIssueDate is an invoice whose issue date CIS refuses on the day it is
// sent. It is not an addon rule: a rule gives the same answer each time it
// validates the same invoice, and this answer changes with the day.
var ErrIssueDate = errors.New("issue date CIS refuses")

// maxIssueAge is how many days before the day it is sent an invoice can be
// dated: CIS refuses an older one (error 176).
const maxIssueAge = 30

// CheckIssueDate reports whether CIS would refuse the invoice for its issue
// date if it were sent at now (spec v2.7, restrictive errors): an issue date
// more than 30 days before now (176), or an issue date and time after now
// (177). Both are compared on a Croatian clock. The fiscalization step calls
// it before the invoice is numbered, so that an invoice CIS is certain to
// refuse does not use a number.
//
// The invoice must have its issue date and time; one without a time is
// ErrMissing, as in the conversion.
func CheckIssueDate(inv *bill.Invoice, now time.Time) error {
	if inv == nil {
		return ErrNotInvoice
	}
	if inv.IssueTime == nil {
		return fmt.Errorf("invoice issue time %w: set the issue date and time first", ErrMissing)
	}
	now = now.In(zagreb)
	issued := issueTime(inv)
	if issued.In(zagreb).After(now) {
		return fmt.Errorf("%w: issued at %s, after the time it is sent, %s (CIS error 177)",
			ErrIssueDate, formatDateTime(issued, " "), now.Format("02.01.2006 15:04:05"))
	}
	oldest := cal.DateOf(now).Add(0, 0, -maxIssueAge)
	if issued.Date().Before(oldest.Date) {
		return fmt.Errorf("%w: issued on %s, more than %d days before the day it is sent, %s (CIS error 176)",
			ErrIssueDate, issued.Date().Time().Format("02.01.2006"), maxIssueAge, now.Format("02.01.2006"))
	}
	return nil
}
