// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestRefundNettingReducesOriginalMonthSpend proves the XC2 read-model contract
// for budgets: a $120 clothing purchase in March returned $40 in April reads as
// $80 net spend in MARCH's budget, and the refund inflates nothing — while the
// ledger transactions are untouched.
func TestRefundNettingReducesOriginalMonthSpend(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "clothing", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}

	buy := domain.Transaction{ID: "buy", Amount: usd(-12000), CategoryID: "clothing", Date: mustDate("2026-03-10")}
	refund := domain.Transaction{ID: "ref", Amount: usd(4000), CategoryID: "clothing", Date: mustDate("2026-04-05")}
	all := []domain.Transaction{buy, refund}

	links := []domain.TxnLink{{
		ID: "p", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "ref"}, Amount: usd(4000),
	}}

	marchStart := mustDate("2026-03-01")
	marchEnd := mustDate("2026-04-01")
	aprStart := mustDate("2026-04-01")
	aprEnd := mustDate("2026-05-01")

	// Baseline: no netting installed → March shows the full $120 blowout.
	SetRefundLinks(nil)
	base, err := Spent(budget, all, marchStart, marchEnd, rates)
	if err != nil {
		t.Fatalf("baseline Spent: %v", err)
	}
	if base.Amount != 12000 {
		t.Fatalf("baseline March spend = %d, want 12000", base.Amount)
	}

	// With netting: March nets to $80, April stays $0 (refund is not spend and is
	// zeroed anyway — no phantom).
	SetRefundLinks(links)
	defer SetRefundLinks(nil)

	march, err := Spent(budget, all, marchStart, marchEnd, rates)
	if err != nil {
		t.Fatalf("netted March Spent: %v", err)
	}
	if march.Amount != 8000 {
		t.Errorf("netted March spend = %d, want 8000 (120 - 40)", march.Amount)
	}

	april, err := Spent(budget, all, aprStart, aprEnd, rates)
	if err != nil {
		t.Fatalf("netted April Spent: %v", err)
	}
	if april.Amount != 0 {
		t.Errorf("netted April spend = %d, want 0 (no phantom)", april.Amount)
	}

	// The ledger atoms are untouched.
	if all[0].Amount.Amount != -12000 || all[1].Amount.Amount != 4000 {
		t.Errorf("netting mutated the ledger transactions: %+v", all)
	}
}
