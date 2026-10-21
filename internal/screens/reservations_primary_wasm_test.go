// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func reservationHostFixture(t *testing.T) (*appstate.App, domain.Account, domain.Goal, domain.Goal) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	s := app.Settings()
	s.BaseCurrency, s.FXRates = "USD", map[string]float64{"EUR": 2}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	a := netWorthFixtureAccount("bank", "USD", 10000, false)
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	first := domain.Goal{ID: "retained", Name: "Retained", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Archived: true, TargetAmount: money.New(20000, "EUR"), CurrentAmount: money.Zero("EUR"), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(3000, "EUR")}}}
	second := domain.Goal{ID: "active", Name: "Active", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(20000, "USD"), CurrentAmount: money.Zero("USD"), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}}
	for _, g := range []domain.Goal{first, second} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	return app, a, first, second
}

func TestPrimaryReservationWarningsStayCoherentAcrossHealthyBreachAndUnknown(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app, a, first, _ := reservationHostFixture(t)
	oldFundingCache := goalFundingCache
	goalFundingCache = newViewCache[goalview.FundingIndex](1)
	defer func() { goalFundingCache = oldFundingCache }()
	f := render.New(t)
	defer f.Cleanup()
	var edit state.Atom[uistate.AccountEdit]
	var view state.Atom[string]
	f.Render(ui.CreateElement(func() ui.Node {
		edit, view = uistate.UseAccountEdit(), uistate.UseGoalsView()
		// Deliberately stable host props and no parent data-revision hook.
		return Div(ui.CreateElement(accountEarmarkWarning, accountEarmarkWarnProps{Account: a, Balance: a.OpeningBalance}), ui.CreateElement(goalConflictStrip, struct{}{}))
	}))
	oldEdit, oldView := edit.Get(), view.Get()
	defer func() { edit.Set(oldEdit); view.Set(oldView) }()
	assertState := func(want string) {
		t.Helper()
		account, conflict := byTestID(f, "div", "acct-earmark-warn-"+a.ID), byTestID(f, "div", "goal-conflict-"+a.ID)
		if want == "healthy" {
			if account != nil || conflict != nil {
				t.Fatalf("healthy account warned: %s", f.Text())
			}
			return
		}
		if account == nil || conflict == nil {
			t.Fatalf("%s warning missing: %s", want, f.Text())
		}
		for _, node := range []*render.QueryNode{account, conflict} {
			if want == "unknown" {
				if !strings.Contains(node.Text(), "We can’t check") || strings.Contains(node.Text(), "more than it can back") || strings.Contains(node.Text(), "of goal money has been spent") {
					t.Fatalf("unknown total presented as known: %s", node.Text())
				}
			} else {
				for _, amount := range []string{"130.00", "100.00", "30.00"} {
					if !strings.Contains(node.Text(), amount) {
						t.Fatalf("foreign reservation mismatch %q: %s", amount, node.Text())
					}
				}
			}
		}
	}
	f.Stabilize()
	assertState("healthy")
	first.Allocations = append(first.Allocations, domain.GoalAllocation{AccountID: a.ID, Amount: money.New(3000, "EUR")})
	if err := app.PutGoal(first); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("breach")
	byTestID(f, "button", "acct-earmark-transfer-"+a.ID).Click()
	f.Stabilize()
	if edit.Get().ID != a.ID || edit.Get().Mode != uistate.AcctEditModeTransfer {
		t.Fatalf("transfer action changed: %+v", edit.Get())
	}
	byTestID(f, "button", "goals-conflict-review").Click()
	f.Stabilize()
	if view.Get() != uistate.GoalsViewEarmarks {
		t.Fatal("review action did not open earmarks")
	}
	s := app.Settings()
	s.FXRates = nil
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("unknown")
	s.FXRates = map[string]float64{"EUR": 1}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("healthy")
	s.FXRates["EUR"] = 2
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("breach")
	a.OpeningBalance = money.New(10000, "EUR")
	if err := app.Store().PutAccount(a); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("unknown")
	a.OpeningBalance = money.New(10000, "USD")
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertState("breach")
	if goalFundingCache.Len() != 0 {
		t.Fatal("warning views invoked the full funding/suggestion builder")
	}
}

func TestPrimaryReservationWarningsKeepDistinctIdentitiesAndInvalidAmountsExplicit(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app, a, first, second := reservationHostFixture(t)
	if err := app.DeleteGoal(second.ID); err != nil {
		t.Fatal(err)
	}
	first.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(12000, "USD")}, {AccountID: a.ID, Amount: money.New(1000, "USD")}}
	if err := app.PutGoal(first); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		return Div(ui.CreateElement(accountEarmarkWarning, accountEarmarkWarnProps{Account: a, Balance: a.OpeningBalance}), ui.CreateElement(goalConflictStrip, struct{}{}))
	}))
	f.Stabilize()
	if byTestID(f, "div", "acct-earmark-warn-"+a.ID) == nil || byTestID(f, "div", "goals-conflict-strip") != nil {
		t.Fatalf("duplicate rows became multiple goals: %s", f.Text())
	}
	for _, amounts := range [][]int64{{-1}, {math.MaxInt64, 1}} {
		first.Allocations = nil
		for _, amount := range amounts {
			first.Allocations = append(first.Allocations, domain.GoalAllocation{AccountID: a.ID, Amount: money.New(amount, "USD")})
		}
		if err := app.Store().PutGoal(first); err != nil {
			t.Fatal(err)
		}
		uistate.BumpDataRevision()
		f.Stabilize()
		node := byTestID(f, "div", "acct-earmark-warn-"+a.ID)
		if node == nil || !strings.Contains(node.Text(), "We can’t check") {
			t.Fatalf("invalid stored amounts were treated as healthy: %s", f.Text())
		}
	}
}
