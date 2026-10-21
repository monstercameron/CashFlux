// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/chart"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestPrimaryNetWorthExclusionEditorKeepsHeadlinesTrendAndDeltaTogether(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := primaryAccountsGoalsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, a := range []domain.Account{netWorthFixtureAccount("own", "USD", 10000, false), netWorthFixtureAccount("custodial", "USD", 1000000, false)} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	monthStart := dateutil.MonthStart(now)
	for _, txn := range []domain.Transaction{
		{ID: "own-income", Desc: "Own income", AccountID: "own", Date: monthStart, Amount: money.New(200, "USD")},
		{ID: "custodial-prior", Desc: "Custodial prior income", AccountID: "custodial", Date: monthStart.AddDate(0, -1, 0), Amount: money.New(5000, "USD")},
		{ID: "custodial-current", Desc: "Custodial current income", AccountID: "custodial", Date: monthStart, Amount: money.New(900000, "USD")},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	var lens state.Atom[scope.ReportScope]
	var window state.Atom[period.Window]
	var closeEditor func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		lens, window = uistate.UseActiveScope(), uistate.UsePeriod()
		open := ui.UseState(true)
		closeEditor = open.Set
		reopen := ui.UseEvent(func() { open.Set(true) })
		// The host does not subscribe to data revision: real children must react
		// to the editor's save event even when their incoming props are stable.
		return Div(ui.CreateElement(dashboardHero), ui.CreateElement(acctSummaryWidget, acctSummaryProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}}),
			Button(Attr("data-testid", "selection-reopen-editor"), OnClick(reopen), "Edit"),
			If(open.Get(), ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: "custodial", Mode: uistate.AcctEditModeEdit, OnDone: func() { open.Set(false) }})))
	}))
	oldLens, oldWindow := lens.Get(), window.Get()
	defer func() { closeEditor(false); lens.Set(oldLens); window.Set(oldWindow); f.Stabilize() }()
	lens.Set(scope.ReportScope{})
	window.Set(period.NewWindow(period.Month, now, time.Monday))
	sparkPath := func() string {
		for _, node := range f.AllByTag("path") {
			if strings.Contains(node.Attr("class"), "wonder-chart-line") {
				return node.Attr("d")
			}
		}
		return ""
	}
	deadline := time.Now().Add(3 * time.Second)
	for sparkPath() == "" && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(10 * time.Millisecond)
	}
	f.Stabilize()
	initialPath := sparkPath()
	if initialPath == "" {
		t.Fatal("actual hero never prepared its deferred trend")
	}
	assertFigures := func(want string) {
		t.Helper()
		for _, id := range []string{"hero-networth-value", "acct-networth-value"} {
			node := byTestID(f, "div", id)
			if node == nil || !strings.Contains(node.Text(), want) {
				t.Fatalf("%s expected %q: %s", id, want, f.Text())
			}
		}
	}
	assertDelta := func(want string) {
		t.Helper()
		hero, accounts := "", ""
		for _, node := range f.AllByTag("span") {
			if strings.Contains(node.Attr("class"), "home-hero-delta") {
				hero = node.Text()
			}
			if strings.Contains(node.Attr("class"), "stat-sub") && strings.Contains(node.Text(), "this month") {
				accounts = node.Text()
			}
		}
		if !strings.Contains(hero, want) || !strings.Contains(accounts, want) {
			t.Fatalf("delta wanted %q, hero=%q accounts=%q", want, hero, accounts)
		}
	}
	assertFigures("19,152.00")
	assertDelta("9,002.00")
	toggleAndSave := func() {
		t.Helper()
		byTestID(f, "button", "acct-edit-more").Click()
		f.Stabilize()
		checkbox := byTestID(f, "input", "acct-edit-exclude-networth")
		if checkbox == nil {
			t.Fatal("account exclusion editor missing")
		}
		checkbox.Dispatch("onchange", render.Event{})
		f.Stabilize()
		forms := f.AllByTag("form")
		if len(forms) != 1 {
			t.Fatalf("expected actual account editor, found %d forms", len(forms))
		}
		forms[0].Submit()
		f.Stabilize()
	}
	toggleAndSave()
	assertFigures("102.00")
	assertDelta("2.00")
	if byTestID(f, "p", "hero-networth-note") == nil || byTestID(f, "p", "acct-nw-excludes-by-choice") == nil {
		t.Fatalf("chosen exclusion lost its disclosure: %s", f.Text())
	}
	wantPath := chart.LinePath(chart.Points([]float64{10000, 10000, 10000, 10000, 10000, 10000, 10200}, 260, 72, 6))
	if got := sparkPath(); got != wantPath || got == initialPath {
		t.Fatalf("spark retained chosen account: got=%q want=%q initial=%q", got, wantPath, initialPath)
	}
	byTestID(f, "button", "selection-reopen-editor").Click()
	f.Stabilize()
	toggleAndSave()
	assertFigures("19,152.00")
	assertDelta("9,002.00")
	if sparkPath() != initialPath || byTestID(f, "p", "hero-networth-note") != nil || byTestID(f, "p", "acct-nw-excludes-by-choice") != nil {
		t.Fatalf("restoring inclusion retained stale totals or disclosure: %s", f.Text())
	}
}
