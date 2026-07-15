// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestB13TrueUp(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cats := []domain.Category{{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense}}
	budget := domain.Budget{ID: "b1", Name: "Groceries", CategoryID: "groceries",
		Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}

	var txns []domain.Transaction
	for k := 1; k <= 6; k++ {
		d := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC).AddDate(0, -k, 0)
		txns = append(txns, domain.Transaction{ID: string(rune('a' + k)), CategoryID: "groceries",
			Date: d, Amount: money.New(-48000, "USD")})
	}

	in := Input{
		Now: now, Base: "USD", Rates: currency.Rates{Base: "USD"},
		Categories: cats, Budgets: []domain.Budget{budget}, Transactions: txns,
	}
	got := b13TrueUp(in)
	if len(got) != 1 {
		t.Fatalf("want 1 insight, got %d", len(got))
	}
	ins := got[0]
	if ins.Feature != "SMART-B13" {
		t.Errorf("feature = %s", ins.Feature)
	}
	if !strings.Contains(ins.Key, ":48000") {
		t.Errorf("key should encode suggested level: %s", ins.Key)
	}
	if !strings.Contains(ins.Title, "Groceries") {
		t.Errorf("title: %s", ins.Title)
	}
	if ins.Action == nil || ins.Action.RelatedID != "b1" {
		t.Errorf("action wrong: %+v", ins.Action)
	}
	// Delta amount = 480 - 400 = 80.
	if ins.Amount.Amount != 8000 {
		t.Errorf("delta amount = %d, want 8000", ins.Amount.Amount)
	}
}

func TestB13TrueUpNoDrift(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cats := []domain.Category{{ID: "g", Kind: domain.KindExpense}}
	budget := domain.Budget{ID: "b1", CategoryID: "g", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}
	// spend well under the limit
	txns := []domain.Transaction{{ID: "1", CategoryID: "g",
		Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(-10000, "USD")}}
	in := Input{Now: now, Base: "USD", Rates: currency.Rates{Base: "USD"},
		Categories: cats, Budgets: []domain.Budget{budget}, Transactions: txns}
	if got := b13TrueUp(in); len(got) != 0 {
		t.Errorf("no drift should yield no insight, got %+v", got)
	}
}
