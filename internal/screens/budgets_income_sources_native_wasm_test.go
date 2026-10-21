// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type incomeSourceNativeControls struct {
	Draft state.Atom[uistate.BudgetBasisDraft]
}
type incomeSourceNativeProps struct {
	Window   period.Window
	Draft    uistate.BudgetBasisDraft
	Controls *incomeSourceNativeControls
}

func incomeSourceNativeHost(props incomeSourceNativeProps) ui.Node {
	window := uistate.UsePeriod()
	draft := uistate.UseBudgetBasisDraft()
	initialized := ui.UseState(false)
	props.Controls.Draft = draft
	if !initialized.Get() {
		initialized.Set(true)
		window.Set(props.Window)
		draft.Set(props.Draft)
	}
	return ui.CreateElement(BudgetBasisBody, struct{}{})
}

func incomeSourceNativeApp(t *testing.T) (*appstate.App, period.Window) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	for _, category := range []domain.Category{{ID: "a", Name: "Alpha", Kind: domain.KindIncome}, {ID: "b", Name: "Beta", Kind: domain.KindIncome}, {ID: "empty", Name: "Zero", Kind: domain.KindIncome}} {
		if err := app.Store().PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	return app, period.NewWindow(period.Month, time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), time.Monday)
}

func incomeSourceNativePut(t *testing.T, app *appstate.App, id, category string, month time.Month, amount int64, code string) {
	t.Helper()
	if err := app.Store().PutTransaction(domain.Transaction{ID: id, CategoryID: category, Date: time.Date(2024, month, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(amount, code)}); err != nil {
		t.Fatal(err)
	}
}

func incomeSourceNativeFixture(t *testing.T) *render.Fixture {
	t.Helper()
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	return f
}

func incomeSourceNativeText(t *testing.T, f *render.Fixture, id string) string {
	t.Helper()
	node := byTestID(f, "span", id)
	if node == nil {
		t.Fatalf("missing %s; text=%s", id, f.Text())
	}
	return node.Text()
}

func TestBudgetIncomeSourcesNativeHostKeepsSelectionAndLastMonthOrderAcrossAveraging(t *testing.T) {
	app, window := incomeSourceNativeApp(t)
	incomeSourceNativePut(t, app, "a-last", "a", time.March, 10000, "USD")
	incomeSourceNativePut(t, app, "a-older", "a", time.January, 50000, "USD")
	incomeSourceNativePut(t, app, "b-last", "b", time.March, 20000, "USD")
	before := app.Transactions()
	controls := &incomeSourceNativeControls{}
	f := incomeSourceNativeFixture(t)
	f.Render(ui.CreateElement(incomeSourceNativeHost, incomeSourceNativeProps{Window: window, Draft: uistate.BudgetBasisDraft{Mode: budgeting.IncomeModeCategories, Cats: []string{"a"}}, Controls: controls}))
	f.Stabilize()
	assertSources := func(a, b, total string) {
		t.Helper()
		for id, want := range map[string]string{"budgets-zbb-source-amount-a": a, "budgets-zbb-source-amount-b": b, "budgets-zbb-sources-total": total} {
			if got := incomeSourceNativeText(t, f, id); got != want {
				t.Fatalf("%s=%q want%q", id, got, want)
			}
		}
		var order []string
		for _, label := range f.AllByTag("label") {
			if id := label.Attr("data-testid"); strings.HasPrefix(id, "budgets-zbb-source-") {
				order = append(order, id)
			}
		}
		if !reflect.DeepEqual(order, []string{"budgets-zbb-source-b", "budgets-zbb-source-a", "budgets-zbb-source-empty"}) {
			t.Fatalf("averaging changed last-month ranking: %v", order)
		}
	}
	assertSources("$100.00", "$200.00", "$100.00")
	byTestID(f, "input", "budgets-zbb-average").Change("true")
	f.Stabilize()
	assertSources("$200.00", "$66.66", "$200.00")
	if !reflect.DeepEqual(controls.Draft.Get().Cats, []string{"a"}) {
		t.Fatal("averaging discarded selection", controls.Draft.Get())
	}
	byTestID(f, "label", "budgets-zbb-source-b").Children()[0].Change("true")
	f.Stabilize()
	assertSources("$200.00", "$66.66", "$266.66")
	byTestID(f, "input", "budgets-zbb-average").Change("false")
	f.Stabilize()
	assertSources("$100.00", "$200.00", "$300.00")
	if !reflect.DeepEqual(controls.Draft.Get().Cats, []string{"a", "b"}) || !reflect.DeepEqual(before, app.Transactions()) {
		t.Fatal("native source preparation lost selection or mutated financial records")
	}
}

func TestBudgetIncomeSourcesNativeHostSeparatesUnknownSourcesFromKnownFixedPreview(t *testing.T) {
	app, window := incomeSourceNativeApp(t)
	incomeSourceNativePut(t, app, "foreign", "a", time.March, 100, "EUR")
	controls := &incomeSourceNativeControls{}
	f := incomeSourceNativeFixture(t)
	f.Render(ui.CreateElement(incomeSourceNativeHost, incomeSourceNativeProps{Window: window, Draft: uistate.BudgetBasisDraft{Mode: budgeting.IncomeModeCategories, Cats: []string{"a"}, Fixed: 50000}, Controls: controls}))
	f.Stabilize()
	if got := incomeSourceNativeText(t, f, "budgets-zbb-source-amount-a"); got != uistate.T("calculation.unavailable") {
		t.Fatalf("unknown source became zero/no-history: %q", got)
	}
	if got := incomeSourceNativeText(t, f, "budgets-zbb-sources-total"); got != "—" {
		t.Fatalf("unknown selected preview became zero: %q", got)
	}
	if byTestID(f, "p", "budgets-zbb-source-unavailable") == nil || byTestID(f, "p", "budgets-zbb-income-unavailable") == nil || strings.Contains(f.Text(), uistate.T("budgets.incomeSourceNoHistory")) {
		t.Fatalf("missing source failure disclosure: %s", f.Text())
	}
	selectAll := byTestID(f, "button", "budgets-zbb-select-all")
	if !isDisabled(selectAll) {
		t.Fatal("unknown positive-source selection remained automatic")
	}
	selectAll.Click()
	f.Stabilize()
	if !reflect.DeepEqual(controls.Draft.Get().Cats, []string{"a"}) {
		t.Fatal("unknown source bulk action overwrote manual choices")
	}
	byTestID(f, "select", "budgets-zbb-income-mode").Change(budgeting.IncomeModeFixed)
	f.Stabilize()
	if byTestID(f, "p", "budgets-zbb-income-unavailable") != nil {
		t.Fatal("source failure contaminated independent known fixed income")
	}
	input := byTestID(f, "input", "budgets-zbb-fixed-amount")
	if input == nil {
		t.Fatalf("fixed amount input unavailable: %s", f.Text())
	}
	// The edit field intentionally trims trailing decimals (500, not 500.00).
	// Assert exact money and the canonical preview rather than display spelling.
	draft := controls.Draft.Get()
	base := budgetBaseCurrency(app.Settings().BaseCurrency)
	raw := input.Attr("data-cf-value")
	minor, parseErr := money.ParseMinor(raw, currency.Decimals(base))
	preview, previewErr := resolveIncomeBasis(app, draft.Mode, draft.PaycheckMin, draft.Fixed, draft.Cats, draft.AvgMonths,
		incomeBasisMonth(window, time.Monday), base, currency.Rates{Base: base, Rates: app.Settings().FXRates})
	if parseErr != nil || minor != 50000 || draft.Mode != budgeting.IncomeModeFixed || draft.Fixed != 50000 || previewErr != nil || preview != 50000 {
		t.Fatalf("independent fixed fact unavailable: raw=%q minor=%d parse=%v draft=%+v preview=%d previewErr=%v", raw, minor, parseErr, draft, preview, previewErr)
	}
	byTestID(f, "select", "budgets-zbb-income-mode").Change(budgeting.IncomeModeCategories)
	f.Stabilize()
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 1.5}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if got := incomeSourceNativeText(t, f, "budgets-zbb-source-amount-a"); got != "$1.50" {
		t.Fatalf("FX recovery failed: %q", got)
	}
	if got := incomeSourceNativeText(t, f, "budgets-zbb-sources-total"); got != "$1.50" {
		t.Fatalf("canonical preview did not recover: %q", got)
	}
	if byTestID(f, "p", "budgets-zbb-source-unavailable") != nil || byTestID(f, "p", "budgets-zbb-income-unavailable") != nil {
		t.Fatal("recovered model retained unavailable notices")
	}
}
