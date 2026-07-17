// SPDX-License-Identifier: MIT

package tasksuggest

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
)

var now = time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

func usd(m int64) money.Money { return money.New(m, "USD") }

func TestScanStaleAccounts(t *testing.T) {
	stale := domain.Account{ID: "a1", Name: "Old Card", Class: domain.ClassAsset,
		Type: domain.TypeCreditCard, Currency: "USD", BalanceAsOf: now.AddDate(0, 0, -60)}
	fresh := domain.Account{ID: "a2", Name: "Fresh", Class: domain.ClassAsset,
		Type: domain.TypeChecking, Currency: "USD", BalanceAsOf: now}
	got := Scan([]domain.Account{stale, fresh}, nil, nil, freshness.DefaultWindows(),
		currency.Rates{Base: "USD"}, now, time.Sunday)
	if len(got) != 1 {
		t.Fatalf("suggestions = %d, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.Kind != KindStaleAccount || s.Key != "stale:a1" || s.Name != "Old Card" ||
		s.RelatedType != domain.RelatedAccount || s.RelatedID != "a1" {
		t.Errorf("stale suggestion = %+v", s)
	}
	if s.Resolve != nil {
		t.Error("stale suggestions have no engine-var resolve condition")
	}
}

func TestScanUnreviewedThreshold(t *testing.T) {
	mk := func(n int) []domain.Transaction {
		out := make([]domain.Transaction, n)
		for i := range out {
			out[i] = domain.Transaction{ID: string(rune('a' + i)), AccountID: "x", Amount: usd(-100)}
		}
		return out
	}
	// Below threshold: silent.
	if got := Scan(nil, mk(UnreviewedThreshold-1), nil, freshness.Windows{}, currency.Rates{Base: "USD"}, now, time.Sunday); len(got) != 0 {
		t.Errorf("below threshold: %+v", got)
	}
	// At threshold: one aggregate suggestion with a self-resolve condition.
	got := Scan(nil, mk(UnreviewedThreshold), nil, freshness.Windows{}, currency.Rates{Base: "USD"}, now, time.Sunday)
	if len(got) != 1 || got[0].Kind != KindUnreviewed || got[0].Count != UnreviewedThreshold {
		t.Fatalf("at threshold: %+v", got)
	}
	if got[0].Resolve == nil || got[0].Resolve.Condition != "txns_unreviewed == 0" {
		t.Errorf("resolve = %+v", got[0].Resolve)
	}
	// Reviewed and transfer rows don't count.
	txns := mk(UnreviewedThreshold)
	txns[0].Reviewed = true
	txns[1].TransferAccountID = "y"
	if got := Scan(nil, txns, nil, freshness.Windows{}, currency.Rates{Base: "USD"}, now, time.Sunday); len(got) != 0 {
		t.Errorf("reviewed/transfer rows counted: %+v", got)
	}
}

func TestScanOverspentBudgets(t *testing.T) {
	b := domain.Budget{ID: "b1", Name: "Dining Out", Period: domain.PeriodMonthly,
		Limit: usd(10000), CategoryID: "cat1"}
	spend := domain.Transaction{ID: "t1", AccountID: "x", CategoryID: "cat1",
		Amount: usd(-15000), Date: now}
	got := Scan(nil, []domain.Transaction{spend}, []domain.Budget{b},
		freshness.Windows{}, currency.Rates{Base: "USD"}, now, time.Sunday)
	// The spend row is also unreviewed but below the threshold, so only the
	// overspend suggestion appears.
	if len(got) != 1 {
		t.Fatalf("suggestions = %d, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.Kind != KindOverspentBudget || s.Key != "overspent:b1" || s.RelatedID != "b1" {
		t.Errorf("overspend suggestion = %+v", s)
	}
	if s.Resolve == nil || s.Resolve.Condition == "" {
		t.Fatalf("overspend must self-resolve, got %+v", s.Resolve)
	}
	// Under-limit budgets stay silent.
	b.Limit = usd(20000)
	if got := Scan(nil, []domain.Transaction{spend}, []domain.Budget{b},
		freshness.Windows{}, currency.Rates{Base: "USD"}, now, time.Sunday); len(got) != 0 {
		t.Errorf("under-limit budget suggested: %+v", got)
	}
}
