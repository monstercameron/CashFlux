// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func desc(id, account string, when time.Time, amount int64, d string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: account, Date: when, Amount: usd(amount), Desc: d}
}

func TestT2Duplicates(t *testing.T) {
	in := baseInput()
	day := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	in.Transactions = []domain.Transaction{
		desc("a", "x", day, -4200, "Coffee Shop"),
		desc("b", "x", day, -4200, "coffee shop"), // same day/amount/desc → dup
		desc("c", "x", day, -999, "Other"),
	}
	got := t2Duplicates(in)
	if len(got) != 1 {
		t.Fatalf("want 1 duplicate group, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 4200 {
		t.Errorf("amount should be magnitude 4200, got %d", got[0].Amount.Amount)
	}
}

func TestT7MissingTxn(t *testing.T) {
	in := baseInput() // now June 15
	// A monthly Netflix charge on the 5th, last seen in May → June charge overdue.
	var txns []domain.Transaction
	for _, m := range []time.Month{time.March, time.April, time.May} {
		txns = append(txns, domain.Transaction{
			ID: "n" + m.String(), AccountID: "x", Date: time.Date(2026, m, 5, 0, 0, 0, 0, time.UTC),
			Amount: usd(-1599), Desc: "Netflix",
		})
	}
	in.Transactions = txns
	got := t7MissingTxn(in)
	if len(got) != 1 {
		t.Fatalf("want 1 missing-charge insight, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityWarn {
		t.Errorf("missing charge should warn, got %v", got[0].Severity)
	}
}

func TestT7PresentNotFlagged(t *testing.T) {
	in := baseInput()
	var txns []domain.Transaction
	// Includes a June 5 charge → not overdue.
	for _, m := range []time.Month{time.March, time.April, time.May, time.June} {
		txns = append(txns, domain.Transaction{
			ID: "n" + m.String(), AccountID: "x", Date: time.Date(2026, m, 5, 0, 0, 0, 0, time.UTC),
			Amount: usd(-1599), Desc: "Netflix",
		})
	}
	in.Transactions = txns
	if got := t7MissingTxn(in); len(got) != 0 {
		t.Errorf("charge present — want 0, got %d: %+v", len(got), got)
	}
}

func TestT13RefundMatch(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		desc("charge", "x", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), -8000, "Zappos"),
		desc("refund", "x", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), 8000, "Zappos"),
	}
	got := t13RefundMatch(in)
	if len(got) != 1 {
		t.Fatalf("want 1 refund match, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-T13:refund" {
		t.Errorf("wrong refund flagged: %s", got[0].Key)
	}
}

func TestT13NoMatchDifferentMerchant(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		desc("charge", "x", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), -8000, "Zappos"),
		desc("income", "x", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), 8000, "Paycheck"),
	}
	if got := t13RefundMatch(in); len(got) != 0 {
		t.Errorf("different merchant — want 0, got %d: %+v", len(got), got)
	}
}
