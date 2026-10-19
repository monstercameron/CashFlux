// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBudgetBCS2BillsPaydayMigratesPeriodMoney(t *testing.T) {
	a := guardTestApp(t)
	pr := prefs.Default()
	pr.PayCycleAnchor = "2026-10-09"
	if err := uistate.PersistPrefs(pr); err != nil {
		t.Fatal(err)
	}
	b := quickFillFixtureBudget()
	b.Period = domain.PeriodBiweekly
	b.PeriodBoosts = map[string]int64{"2026-10-09": 5000}
	b.PeriodNotes = map[string]string{"2026-10-09": "Guests"}
	b.RecurringCover = &domain.RecurringCover{LastAppliedPeriod: "2026-10-09"}
	if err := a.PutBudgetAt(b, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(BillsSmartForm, BillsSmartFormProps{}))
	field := byTestID(f, "input", "bills-smart-anchor")
	if field == nil {
		t.Fatal("payday input missing")
	}
	field.Dispatch("onchange", render.Event{Value: "2026-10-10"})
	f.Stabilize()
	got := a.Budgets()[0]
	if got.PeriodBoosts["2026-10-10"] != 5000 || got.PeriodNotes["2026-10-10"] != "Guests" || got.RecurringCover.LastAppliedPeriod != "2026-10-10" {
		t.Fatalf("Bills orphaned money: %+v", got)
	}
	if uistate.LoadPrefs().PayCycleAnchor != "2026-10-10" {
		t.Fatal("payday was not saved")
	}
}

func TestBudgetBCS2SetupWeekStartMigratesBeforeAdvancing(t *testing.T) {
	a := guardTestApp(t)
	if err := uistate.PersistPrefs(prefs.Default()); err != nil {
		t.Fatal(err)
	}
	b := quickFillFixtureBudget()
	b.Period = domain.PeriodWeekly
	b.PeriodBoosts = map[string]int64{"2026-10-11": 5000}
	if err := a.PutBudgetAt(b, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var finish func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(rev uint64, done func(appstate.DurableResult)) { revision, finish = rev, done })
	f := render.New(t)
	f.Render(ui.CreateElement(SetupWizard))
	byTestID(f, "select", "setup-weekstart").Dispatch("onchange", render.Event{Value: "monday"})
	f.Stabilize()
	byTestID(f, "form", "setup-currency-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if finish == nil {
		t.Fatal("setup did not await durability")
	}
	if got := a.Budgets()[0]; got.PeriodBoosts["2026-10-12"] != 5000 {
		t.Fatalf("Setup orphaned money: %+v", got)
	}
	if byTestID(f, "input", "setup-income") != nil {
		t.Fatal("setup acknowledged the write before durability")
	}
	finish(appstate.DurableResult{Revision: revision})
	f.Stabilize()
	if byTestID(f, "input", "setup-income") == nil {
		t.Fatal("durable setup did not advance")
	}
}

func TestBudgetBCS2CoverListsExcludeSavingsAndUseEffectiveRoom(t *testing.T) {
	a := guardTestApp(t)
	pr := prefs.Default()
	if err := uistate.PersistPrefs(pr); err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"source", "saving", "over"} {
		b := quickFillFixtureBudget()
		b.ID = id
		b.Name = id
		b.CategoryID = "cat-" + id
		b.Limit = money.New(30000, "USD")
		if id == "saving" {
			b.Direction = domain.DirectionSave
		}
		if id == "source" {
			b.PeriodBoosts = map[string]int64{"2026-10-01": 20000}
		}
		if err := a.PutBudgetAt(b, anchor); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "spend", CategoryID: "cat-source", Date: anchor, Amount: money.New(-5000, "USD")}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "over", CategoryID: "cat-over", Date: anchor, Amount: money.New(-35000, "USD")}); err != nil {
		t.Fatal(err)
	}
	overs, sources, _ := coverAllData(a, "", period.NewWindow(period.Month, anchor, time.Sunday), pr, false)
	if len(overs) != 1 || overs[0].ID != "over" {
		t.Fatalf("overs=%+v", overs)
	}
	if len(sources) != 1 || sources[0].ID != "source" || sources[0].Avail.Amount != 45000 {
		t.Fatalf("sources=%+v", sources)
	}
}

func TestMonthCloseBCS2CopyShowsNativeCurrency(t *testing.T) {
	b := quickFillFixtureBudget()
	b.Limit.Currency = "EUR"
	f := render.New(t)
	f.Render(ui.CreateElement(func() ui.Node {
		return monthCloseSection("copy", "Copy", monthCloseCopyBody([]domain.Budget{b}, nil, map[string]int64{b.ID: 5000}, map[string]int64{b.ID: 5000}, nil, func(string) {}, nil, "USD")...)
	}))
	row := byTestID(f, "label", "monthclose-copy-row-"+b.ID)
	if row == nil || !strings.Contains(row.Text(), "€50.00") || strings.Contains(row.Text(), "$50.00") {
		t.Fatalf("wrong boost currency: %v", row)
	}
}

func TestAdjustBCS2ScopeCopyIsForwardOnly(t *testing.T) {
	if got := adjustEveryPeriodLabel(); got != "From this period on" {
		t.Fatalf("scope=%q", got)
	}
	if got := uistate.T(adjustApplyKey(budgeting.AdjustEveryPeriod)); got != "Apply from this period on" {
		t.Fatalf("apply=%q", got)
	}
	if got := adjustScopeDescription(budgeting.AdjustEveryPeriod); !strings.Contains(got, "Earlier periods keep their limits") {
		t.Fatalf("hint=%q", got)
	}
}
