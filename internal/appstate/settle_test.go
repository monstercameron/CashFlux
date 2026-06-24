// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func sharedExpense(id, payer string, shares map[string]int64) domain.SharedExpense {
	e := domain.SharedExpense{ID: id, PayerID: payer}
	// Deterministic order isn't required for Net, but keep it stable for readability.
	for _, m := range []string{"priya", "sam", "lee"} {
		if amt, ok := shares[m]; ok {
			e.Shares = append(e.Shares, domain.SharedExpenseShare{MemberID: m, Amount: money.New(amt, "USD")})
		}
	}
	return e
}

func TestSettleUpFromPersistedRecords(t *testing.T) {
	a := newApp(t, false)
	// Priya fronts a $90 meal split three ways.
	if err := a.PutSharedExpense(sharedExpense("e1", "priya", map[string]int64{"priya": 3000, "sam": 3000, "lee": 3000})); err != nil {
		t.Fatalf("PutSharedExpense: %v", err)
	}
	if got := a.SharedExpenses(); len(got) != 1 {
		t.Fatalf("SharedExpenses len = %d, want 1", len(got))
	}

	net, transfers := a.SettleUp("USD")
	if net["priya"].Amount != 6000 || net["sam"].Amount != -3000 || net["lee"].Amount != -3000 {
		t.Errorf("net = %+v, want priya +6000, sam/lee -3000", net)
	}
	if len(transfers) != 2 {
		t.Fatalf("want 2 minimal transfers, got %+v", transfers)
	}
	for _, tr := range transfers {
		if tr.To != "priya" {
			t.Errorf("transfer to %s, want priya", tr.To)
		}
	}

	// Sam settles his $30 — now only Lee owes Priya.
	if err := a.RecordSettlement(domain.Settlement{ID: "s1", FromID: "sam", ToID: "priya", Amount: money.New(3000, "USD")}); err != nil {
		t.Fatalf("RecordSettlement: %v", err)
	}
	net2, transfers2 := a.SettleUp("USD")
	if net2["sam"].Amount != 0 {
		t.Errorf("sam net after settling = %d, want 0", net2["sam"].Amount)
	}
	if len(transfers2) != 1 || transfers2[0].From != "lee" || transfers2[0].To != "priya" || transfers2[0].Amount.Amount != 3000 {
		t.Errorf("after settling sam, want one lee->priya 3000, got %+v", transfers2)
	}
}

func TestSettleValidation(t *testing.T) {
	a := newApp(t, false)
	bad := []struct {
		name string
		err  error
	}{
		{"no id", a.PutSharedExpense(domain.SharedExpense{PayerID: "p", Shares: []domain.SharedExpenseShare{{MemberID: "p", Amount: money.New(1, "USD")}}})},
		{"no payer", a.PutSharedExpense(domain.SharedExpense{ID: "x", Shares: []domain.SharedExpenseShare{{MemberID: "p", Amount: money.New(1, "USD")}}})},
		{"no shares", a.PutSharedExpense(domain.SharedExpense{ID: "x", PayerID: "p"})},
		{"settlement same member", a.RecordSettlement(domain.Settlement{ID: "s", FromID: "p", ToID: "p", Amount: money.New(1, "USD")})},
		{"settlement non-positive", a.RecordSettlement(domain.Settlement{ID: "s", FromID: "a", ToID: "b", Amount: money.New(0, "USD")})},
	}
	for _, tc := range bad {
		if tc.err == nil {
			t.Errorf("%s: expected a validation error, got nil", tc.name)
		}
	}
}
