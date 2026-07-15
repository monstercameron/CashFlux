// SPDX-License-Identifier: MIT

package roundups

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func expense(id, acct string, cents int64, day int) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Payee: id,
		Amount: money.New(-cents, "USD"),
		Date:   time.Date(2026, time.July, day, 12, 0, 0, 0, time.UTC),
	}
}

func TestAccrue(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)

	txns := []domain.Transaction{
		expense("a", "chk", 347, 2),  // spare 53
		expense("b", "chk", 1299, 3), // spare 1
		expense("c", "chk", 500, 4),  // exact dollar → 0, skipped
		{ID: "inc", AccountID: "chk", Amount: money.New(20000, "USD"), Date: time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)},
		{ID: "xfer", AccountID: "chk", TransferAccountID: "sav", Amount: money.New(-10000, "USD"), Date: time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC)},
	}

	got := Accrue(txns, nil, nil, since, now)
	if got.TotalCents != 54 {
		t.Fatalf("total = %d, want 54", got.TotalCents)
	}
	if len(got.Contributions) != 2 {
		t.Fatalf("contributions = %d, want 2", len(got.Contributions))
	}
	if got.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", got.Currency)
	}
}

func TestAccrueParticipatingFilter(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		expense("a", "chk", 347, 2),   // 53
		expense("b", "other", 111, 3), // 89, excluded
	}
	got := Accrue(txns, map[string]bool{"chk": true}, nil, since, now)
	if got.TotalCents != 53 {
		t.Fatalf("total = %d, want 53 (only participating account)", got.TotalCents)
	}
}

func TestAccrueSkipsRefundPaired(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{expense("orig", "chk", 347, 2)} // would be 53
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"orig", "refund"}}}
	got := Accrue(txns, nil, links, since, now)
	if got.TotalCents != 0 {
		t.Fatalf("total = %d, want 0 (refund-paired skipped)", got.TotalCents)
	}
}

func TestAccrueWindowBounds(t *testing.T) {
	since := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		expense("before", "chk", 347, 5), // before since → excluded
		expense("in", "chk", 347, 15),    // in window → 53
		expense("after", "chk", 347, 25), // after now → excluded
	}
	got := Accrue(txns, nil, nil, since, now)
	if got.TotalCents != 53 {
		t.Fatalf("total = %d, want 53 (window bounds)", got.TotalCents)
	}
}
