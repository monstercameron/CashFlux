// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func susTxn(amountMinor int64, desc, catID string) domain.Transaction {
	return domain.Transaction{
		ID: "t1", AccountID: "chk", CategoryID: catID,
		Amount: money.New(amountMinor, "USD"),
		Date:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Desc:   desc,
	}
}

// The leak this exists for: a row filed under a transfer category with no
// counterparty is counted as income or spending by every aggregation.
func TestSuspectedByCategory(t *testing.T) {
	for _, name := range []string{"Transfer", "transfers", "  Transfer   In ", "Credit Card Payment", "loan payment"} {
		reason, ok := Suspected(susTxn(-5000, "ACH DEBIT", "c1"), name)
		if !ok || reason != SuspectCategory {
			t.Errorf("category %q: reason=%q ok=%v, want category/true", name, reason, ok)
		}
	}
}

// A category name is matched WHOLE. "Transfer fees" is a fee, which is real
// spending, and hiding it would conceal money the household actually paid.
func TestCategoryNameIsNotMatchedAsASubstring(t *testing.T) {
	for _, name := range []string{"Transfer fees", "Wire transfer fee", "Transferrable benefits", ""} {
		if _, ok := Suspected(susTxn(-5000, "ACH DEBIT", "c1"), name); ok {
			t.Errorf("category %q was suspected; only canonical whole names should be", name)
		}
	}
}

func TestSuspectedByDescription(t *testing.T) {
	for _, desc := range []string{
		"Transfer to Savings *8945", "TRANSFER FROM CHECKING", "Online Xfer to HYSA",
		"ONLINE TRANSFER REF #99", "Payment to card ending 1234",
	} {
		reason, ok := Suspected(susTxn(-5000, desc, ""), "Groceries")
		if !ok || reason != SuspectDescription {
			t.Errorf("desc %q: reason=%q ok=%v, want description/true", desc, reason, ok)
		}
	}
}

// Ordinary spending must not be dragged into review by a loose phrase match.
func TestOrdinarySpendingIsNotSuspected(t *testing.T) {
	for _, desc := range []string{
		"WHOLE FOODS MKT", "Transferwise FX", "Netflix", "Shell Oil 4471", "Rent",
	} {
		if _, ok := Suspected(susTxn(-5000, desc, ""), "Groceries"); ok {
			t.Errorf("desc %q was suspected but is ordinary spending", desc)
		}
	}
}

// Anything already settled is not a suspect: a linked transfer is out of the
// totals already, an excluded row is not reaching them, and zero moves nothing.
func TestSettledRowsAreNotSuspected(t *testing.T) {
	linked := susTxn(-5000, "Transfer to Savings", "c1")
	linked.TransferAccountID = "sav"
	if _, ok := Suspected(linked, "Transfer"); ok {
		t.Error("a structurally linked transfer was suspected")
	}

	excluded := susTxn(-5000, "Transfer to Savings", "c1")
	excluded.ExcludeFromReports = true
	if _, ok := Suspected(excluded, "Transfer"); ok {
		t.Error("a row excluded from reports was suspected; it is not leaking anywhere")
	}

	if _, ok := Suspected(susTxn(0, "Transfer to Savings", "c1"), "Transfer"); ok {
		t.Error("a zero-amount row was suspected; it moves nothing either way")
	}
}

// Leaked states the SIZE of the error, which is what makes the report actionable
// rather than a count of cards to click.
func TestSuspectsAndLeakedTotals(t *testing.T) {
	in := susTxn(30000, "Transfer from Checking", "")
	in.ID = "in"
	out := susTxn(-30000, "Transfer to Savings", "")
	out.ID = "out"
	fine := susTxn(-1200, "Coffee", "")
	fine.ID = "fine"

	got := Suspects([]domain.Transaction{in, out, fine}, nil)
	if len(got) != 2 {
		t.Fatalf("suspects = %d, want 2 (%v)", len(got), got)
	}
	tot := Leaked(got)
	if tot.Count != 2 || tot.IncomeMinor != 30000 || tot.SpendingMinor != 30000 {
		t.Errorf("leaked = %+v, want count 2, income 30000, spending 30000", tot)
	}
}

// A nil resolver must not panic — a caller without the category list should
// simply get description-only detection.
func TestSuspectsWithNilCategoryResolver(t *testing.T) {
	got := Suspects([]domain.Transaction{susTxn(-5000, "ACH DEBIT", "c1")}, nil)
	if len(got) != 0 {
		t.Errorf("suspects = %d, want 0 with no category names available", len(got))
	}
}
