// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBudgetBC4InlineLimitWritesHistory(t *testing.T) {
	for _, method := range []string{"enter", "save"} {
		t.Run(method, func(t *testing.T) {
			a := guardTestApp(t)
			b := quickFillFixtureBudget()
			b.Limit = money.New(30000, "USD")
			sep := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
			oct := sep.AddDate(0, 1, 0)
			if err := a.PutBudgetAt(b, sep); err != nil {
				t.Fatal(err)
			}
			atom := state.NewGlobalAtom("dashboard:period", period.Window{})
			prior := atom.Get()
			t.Cleanup(func() { atom.Set(prior) })
			seedViewedPeriod(oct)
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(BudgetRow, budgetRowProps{Status: budgeting.Status{Budget: b, Limit: b.Limit, Spent: money.Zero("USD"), Remaining: b.Limit}, Category: "Transportation", OnDelete: func(string) {}}))
			open := byTestID(f, "button", "budget-limit-btn-"+b.ID)
			if open == nil {
				t.Fatal("missing inline editor trigger")
			}
			open.Click()
			f.Stabilize()
			field := byTestID(f, "input", "budget-limit-input-"+b.ID)
			if field == nil {
				t.Fatal("missing inline input")
			}
			field.Input("400")
			f.Stabilize()
			if method == "enter" {
				field.Dispatch("onkeydown", render.Event{Key: "Enter"})
			} else {
				form := byTestID(f, "form", "budget-limit-editor-"+b.ID)
				if form == nil {
					for _, candidate := range f.AllByTag("form") {
						if strings.Contains(candidate.Attr("class"), "budget-limit-editform") {
							form = candidate
							break
						}
					}
				}
				if form == nil {
					t.Fatal("missing inline save form")
				}
				form.Submit()
			}
			f.Stabilize()
			for _, date := range []time.Time{sep, oct, oct.AddDate(0, 4, 0)} {
				in, err := a.BudgetViewInput(date)
				if err != nil {
					t.Fatal(err)
				}
				got := budgeting.EvaluateViewBudget(a.Budgets()[0], in)
				want := int64(40000)
				if date.Equal(sep) {
					want = 30000
				}
				if !got.Valid || got.Status.Limit.Amount != want {
					t.Fatalf("%s: cap=%v valid=%v error=%v want=%d", date, got.Status.Limit, got.Valid, got.Error, want)
				}
			}
		})
	}
}

func TestBudgetBC4SuggestionApplyWritesHistory(t *testing.T) {
	a := guardTestApp(t)
	b := quickFillFixtureBudget()
	b.Limit = money.New(30000, "USD")
	now := budgetNow()
	start, _ := budgeting.PeriodRange(b.Period, now, time.Sunday)
	if err := a.PutBudgetAt(b, start.AddDate(0, -3, 0)); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		date := start.AddDate(0, -i, 9)
		if err := a.Store().PutTransaction(domain.Transaction{ID: date.Format(time.DateOnly), CategoryID: b.CategoryID, Date: date, Amount: money.New(-40000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "current-period-overspend", CategoryID: b.CategoryID, Date: now, Amount: money.New(-40000, "USD")}); err != nil {
		t.Fatal(err)
	}
	atom := state.NewGlobalAtom("dashboard:period", period.Window{})
	prior := atom.Get()
	t.Cleanup(func() { atom.Set(prior) })
	seedViewedPeriod(now)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetSuggestHint, budgetSuggestProps{Budget: b}))
	apply := byTestID(f, "button", "smart-rowhint-act-sm12-"+b.ID)
	if apply == nil {
		t.Fatal("missing evidenced limit suggestion")
	}
	apply.Click()
	f.Stabilize()
	for _, date := range []time.Time{start.AddDate(0, -1, 0), now, start.AddDate(0, 1, 0)} {
		in, err := a.BudgetViewInput(date)
		if err != nil {
			t.Fatal(err)
		}
		v := budgeting.EvaluateViewBudget(a.Budgets()[0], in)
		want := int64(40000)
		if date.Before(start) {
			want = 30000
		}
		if !v.Valid || v.Status.Limit.Amount != want {
			t.Fatalf("%s: cap=%v valid=%v error=%v want=%d", date, v.Status.Limit, v.Valid, v.Error, want)
		}
	}
}
