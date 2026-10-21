// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func checkedTaxFixture(t *testing.T) (*appstate.App, taxSectionProps) {
	t.Helper()
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	if err := app.PutCategory(domain.Category{ID: "c-gro", Name: "Groceries", Kind: domain.KindExpense, Deductible: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().SetSettingKV("cashflux:tax-plan", `{"effectiveRatePct":25}`); err != nil {
		t.Fatal(err)
	}
	oldCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(oldCSS) })
	return app, taxSectionProps{From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Year: 2026}
}

func TestReportsTaxCheckedNetIncomeUsesHouseholdRowsSplitsAndRefunds(t *testing.T) {
	app, window := checkedTaxFixture(t)
	// Parent category is nondeductible; only the marked split line belongs in
	// deductions. A later refund changes the original report period's deduction.
	if err := app.PutTransaction(domain.Transaction{ID: "tax-mixed", AccountID: "a-check", Desc: "Mixed receipt", Date: agrDay, CategoryID: "c-rent", Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "c-gro", Amount: money.New(-600, "USD")}, {CategoryID: "c-rent", Amount: money.New(-400, "USD")}}}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "tax-refund", AccountID: "a-check", Desc: "Later grocery refund", Date: window.To, Amount: money.New(2000, "USD")}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutTxnLink(domain.TxnLink{ID: "tax-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"cc33333333333333", "tax-refund"}}); err != nil {
		t.Fatal(err)
	}
	prepared := taxNetIncomeForWindow(app, window)
	if prepared.Err != nil || prepared.Value.IncomeMinor != 450000 || prepared.Value.DeductibleMinor != 18600 || prepared.Value.NetMinor != 431400 {
		t.Fatalf("canonical adapter facts: %+v", prepared)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(taxDepthSection, window))
	f.Stabilize()
	basis := byTestID(f, "p", "tax-est-basis")
	if basis == nil || !strings.Contains(basis.Text(), fmtMoney(money.New(431400, "USD"))) {
		t.Fatalf("prepared net not rendered: %s", f.Text())
	}
	if label := byTestID(f, "p", "tax-est-household"); label == nil || !strings.Contains(label.Text(), "Household tax records") {
		t.Fatal("household scope disclosure missing")
	}
	// Exact partial bounds select only the second grocery row; no month rounding.
	partial := taxNetIncomeForWindow(app, taxSectionProps{From: agrDay.AddDate(0, 0, 1), To: agrDay.AddDate(0, 0, 2), Year: 2026})
	if partial.Err != nil || partial.Value.IncomeMinor != 0 || partial.Value.NetMinor != -8000 {
		t.Fatalf("partial window rounded: %+v", partial)
	}
}

func TestReportsTaxCheckedMemoAndLifecycleStayFresh(t *testing.T) {
	app, window := checkedTaxFixture(t)
	builds := 0
	previous := reportModelBuildObserver
	reportModelBuildObserver = func(name string) {
		if name == "tax-net-income" {
			builds++
		}
	}
	t.Cleanup(func() { reportModelBuildObserver = previous })
	f := render.New(t, render.WithQueuedScheduler())
	var rerender func(int)
	f.Render(ui.CreateElement(func() ui.Node {
		count := ui.UseState(0)
		rerender = count.Set
		_ = count.Get()
		return ui.CreateElement(taxDepthSection, window)
	}))
	f.Stabilize()
	if builds != 1 {
		t.Fatalf("initial builders=%d want1", builds)
	}
	rerender(1)
	f.Stabilize()
	if builds != 1 {
		t.Fatalf("unchanged parent rebuilt financial model: %d", builds)
	}
	// Incomplete typed input changes local control state without saving a new
	// assumption. It must not rerun the checked ledger preparation.
	field := byTestID(f, "input", "tax-rate")
	if field == nil {
		t.Fatal("rate field missing")
	}
	field.Input("-")
	f.Stabilize()
	if builds != 1 {
		t.Fatalf("local field rerender rebuilt financial model: %d", builds)
	}
	if err := app.PutCategory(domain.Category{ID: "c-rent", Name: "Rent", Kind: domain.KindExpense, Deductible: true}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if builds != 2 {
		t.Fatalf("changed deduction category did not invalidate: %d", builds)
	}
	if basis := byTestID(f, "p", "tax-est-basis"); basis == nil || !strings.Contains(basis.Text(), fmtMoney(money.New(280000, "USD"))) {
		t.Fatalf("category facts stale: %s", f.Text())
	}
	appstate.Default = nil
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "input", "tax-rate") != nil {
		t.Fatal("unmounted source retained child controls")
	}
	appstate.Default = app
	uistate.BumpDataRevision()
	f.Stabilize()
	if builds != 2 || byTestID(f, "input", "tax-rate") == nil {
		t.Fatalf("empty-to-loaded hooks/model stale: builds=%d", builds)
	}
	if basis := byTestID(f, "p", "tax-est-basis"); basis == nil || !strings.Contains(basis.Text(), fmtMoney(money.New(280000, "USD"))) {
		t.Fatalf("same-app remount did not restore the prepared facts: %s", f.Text())
	}
}

func TestReportsTaxCheckedPreparationCacheTracksAppRevisionAndExactWindow(t *testing.T) {
	app, window := checkedTaxFixture(t)
	taxNetIncomeCache.Clear()
	builds := 0
	previous := reportModelBuildObserver
	reportModelBuildObserver = func(name string) {
		if name == "tax-net-income" {
			builds++
		}
	}
	t.Cleanup(func() { reportModelBuildObserver = previous })
	prepare := func(a *appstate.App, w taxSectionProps) {
		t.Helper()
		if got := taxNetIncomeForWindow(a, w); got.Err != nil {
			t.Fatal(got.Err)
		}
	}
	prepare(app, window)
	prepare(app, window)
	if builds != 1 {
		t.Fatalf("successful preparation was not reused: %d", builds)
	}
	prepare(app, taxSectionProps{From: window.From.Add(time.Nanosecond), To: window.To, Year: window.Year})
	if builds != 2 {
		t.Fatalf("exact period bounds were not part of cache key: %d", builds)
	}
	if err := app.PutCategory(domain.Category{ID: "tax-cache-rev", Name: "Cache revision", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	prepare(app, window)
	if builds != 3 {
		t.Fatalf("app revision did not invalidate preparation: %d", builds)
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 1.2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	prepare(app, window)
	if builds != 4 {
		t.Fatalf("FX revision did not invalidate preparation: %d", builds)
	}
	second := agrApp(t)
	t.Cleanup(func() { _ = second.Store().Close() })
	prepare(second, window)
	if builds != 5 {
		t.Fatalf("different app identity reused preparation: %d", builds)
	}
}

func TestReportsTaxCheckedSourceFailureNeverBecomesZeroEstimate(t *testing.T) {
	app, window := checkedTaxFixture(t)
	f := render.New(t, render.WithQueuedScheduler())
	f.Render(ui.CreateElement(taxDepthSection, window))
	f.Stabilize()
	if byTestID(f, "p", "tax-est-due") == nil {
		t.Fatal("known fixture estimate missing")
	}
	foreign := netWorthFixtureAccount("tax-eur", "EUR", 0, false)
	if err := app.PutAccount(foreign); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "tax-euro", AccountID: foreign.ID, Desc: "Foreign eligible record", Date: agrDay, Amount: money.New(-100, "EUR")}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "tax-est-unavailable") == nil || byTestID(f, "p", "tax-est-due") != nil || byTestID(f, "p", "tax-est-blocked") != nil {
		t.Fatalf("missing FX became zero/missing-income estimate: %s", f.Text())
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "tax-est-unavailable") != nil || byTestID(f, "p", "tax-est-due") == nil {
		t.Fatalf("fixed FX did not restore estimate: %s", f.Text())
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if unavailable := byTestID(f, "p", "tax-est-unavailable"); unavailable == nil || !strings.Contains(unavailable.Text(), "read tax settings") || byTestID(f, "p", "tax-est-due") != nil {
		t.Fatalf("closed store became zero estimate: %s", f.Text())
	}
}
