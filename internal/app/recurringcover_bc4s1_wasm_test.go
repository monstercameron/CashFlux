// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverformula"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRecurringBC4CoverShares(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amount  int64
		sources []domain.CoverShare
		want    map[string]int64
		fail    bool
	}{
		{"remainder", 101, []domain.CoverShare{{BudgetID: "a", Weight: 1}, {BudgetID: "b", Weight: 1}}, map[string]int64{"a": 50, "b": 51}, false},
		{"duplicate source", 101, []domain.CoverShare{{BudgetID: "a", Weight: 1}, {BudgetID: "a", Weight: 1}}, map[string]int64{"a": 101}, false},
		{"large product", math.MaxInt64, []domain.CoverShare{{BudgetID: "a", Weight: 100}, {BudgetID: "b", Weight: 100}}, map[string]int64{"a": math.MaxInt64 / 2, "b": math.MaxInt64/2 + 1}, false},
		{"zero", 0, []domain.CoverShare{{BudgetID: "a", Weight: 1}}, map[string]int64{}, false},
		{"missing", 100, []domain.CoverShare{{BudgetID: "missing", Weight: 1}}, nil, true},
		{"formula failure", 100, []domain.CoverShare{{BudgetID: "a", WeightFormula: "unknown_function("}}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := domain.Budget{ID: "to", Limit: money.New(10000, "USD"), RecurringCover: &domain.RecurringCover{AmountMinor: tc.amount, Sources: tc.sources}}
			byID := map[string]domain.Budget{"a": {ID: "a"}, "b": {ID: "b"}}
			got, err := recurringCoverShares(b, byID, coverformula.Context{}, "USD")
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v fail=%v", err, tc.fail)
			}
			if tc.fail {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("shares=%v want=%v", got, tc.want)
			}
			for id, want := range tc.want {
				if got[id] != money.New(want, "USD") {
					t.Fatalf("%s share=%v want=%d", id, got[id], want)
				}
			}
		})
	}
}

func TestRecurringBC4AmountFormulaFailureIsNotFixedFallback(t *testing.T) {
	b := domain.Budget{RecurringCover: &domain.RecurringCover{AmountMinor: 100, AmountFormula: "unknown_function("}}
	if _, err := recurringCoverShares(b, nil, coverformula.Context{}, "USD"); err == nil {
		t.Fatal("invalid formula silently fell back to fixed cover")
	}
}

func TestRecurringBC4BootReplayUsesPeriodBoosts(t *testing.T) {
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Store().Close()
	prior := appstate.Default
	appstate.Default = a
	defer func() { appstate.Default = prior }()
	for _, b := range []domain.Budget{
		{ID: "from", Name: "Food", CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(30000, "USD")},
		{ID: "to", Name: "Dining", CategoryID: "dining", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), RecurringCover: &domain.RecurringCover{AmountMinor: 5000, Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}},
	} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		applyRecurringCovers()
		in, err := a.BudgetViewInput(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range a.Budgets() {
			wantBase, wantCap := int64(30000), int64(25000)
			if b.ID == "to" {
				wantBase, wantCap = 10000, 15000
				if b.RecurringCover.LastAppliedPeriod == "" {
					t.Fatal("cover not stamped")
				}
			}
			v := budgeting.EvaluateViewBudget(b, in)
			if b.Limit.Amount != wantBase || !v.Valid || v.Status.Limit.Amount != wantCap {
				t.Fatalf("boot %d %s: base=%v cap=%v err=%v", i, b.ID, b.Limit, v.Status.Limit, v.Error)
			}
		}
	}
}
