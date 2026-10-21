// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	recurringview "github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func csrecText() recurringview.Text {
	return recurringview.Text{T: uistate.T, TN: uistate.TN, Date: uistate.LoadPrefs().FormatDate, FaintClass: tw.Fold(tw.TextFaint)}
}

func csrecBody(t *testing.T) budgetRecurringBodyProps {
	t.Helper()
	in := recurringview.Input{
		Now:   time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Recurring: []domain.Recurring{
			{ID: "rent", Label: "Rent", CategoryID: "rent", Amount: money.New(-10000, "USD"), Cadence: domain.CadenceMonthly},
			{ID: "food", Label: "Food", CategoryID: "food", Amount: money.New(-1200, "USD"), Cadence: domain.CadenceMonthly},
			{ID: "apps", Label: "Apps", CategoryID: "apps", Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly},
			{ID: "uncategorized", Label: "Unknown", Amount: money.New(-500, "USD"), Cadence: domain.CadenceMonthly},
			{ID: "excluded", Label: " Euro bill ", Amount: money.New(-1000, "EUR"), Cadence: domain.CadenceMonthly},
		},
		Budgets:    []domain.Budget{{CategoryID: "rent"}},
		Categories: []domain.Category{{ID: "rent", Name: "Housing"}, {ID: "food", Name: "Food"}, {ID: "apps", Name: "Apps"}},
	}
	m, err := recurringview.Build(in, uistate.T("budgets.recurring.uncategorized"), uistate.T("budgets.recurring.unnamedBill"))
	if err != nil {
		t.Fatal(err)
	}
	text := csrecText()
	return budgetRecurringBodyProps{Model: m, Summary: recurringview.PresentSummary(m, text), Input: in, Text: text, OnManage: func() {}, OnBudget: func(recurringview.Category) {}}
}

func TestCSRECFoldKeepsMarkupAndUpdatedCallback(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	calls := 0
	for _, open := range []bool{false, true, false, true} {
		step := 1
		if open {
			step = 10
		}
		f.Render(ui.CreateElement(budgetRecurringFold, budgetRecurringFoldProps{Open: open, Hint: "4 bills · $127.00/mo", OnToggle: func() { calls += step }}))
		n := byTestID(f, "button", "budgets-recurring-toggle")
		d := recurringview.PresentDisclosure(open)
		if n == nil || n.Attr("type") != "button" || n.Attr("class") != "budget-fold-toggle" || n.Attr("aria-expanded") != d.Expanded || n.Attr("aria-label") != uistate.T(d.AriaKey) || !strings.Contains(n.Text(), "4 bills · $127.00/mo") {
			t.Fatalf("fold=%v", n)
		}
		caret := false
		for _, span := range f.AllByTag("span") {
			if span.Attr("class") == d.CaretClass && span.Attr("aria-hidden") == "true" {
				caret = true
			}
		}
		if !caret {
			t.Fatal("missing original caret")
		}
		n.Click()
	}
	if calls != 22 {
		t.Fatalf("callback captured stale props: %d", calls)
	}
}

func TestCSRECHeadlineKeepsLabelsClassesAndCounts(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	p := csrecBody(t)
	f.Render(ui.CreateElement(budgetRecurringHeadline, p.Summary))
	if len(f.AllByTag("span")) != 3 {
		t.Fatal("headline element structure changed")
	}
	want := []string{uistate.T("budgets.recurring.monthlyBillsLabel"), p.Summary.Total, p.Summary.Count}
	for i, n := range f.AllByTag("span") {
		if n.Text() != want[i] {
			t.Fatalf("headline span %d=%q want %q", i, n.Text(), want[i])
		}
	}
	head := false
	for _, div := range f.AllByTag("div") {
		head = head || div.Attr("class") == "brc-head"
	}
	if !head || f.AllByTag("span")[1].Attr("class") != "brc-total-val fig "+tw.Fold(tw.FontDisplay) {
		t.Fatal("headline class changed")
	}
}

func TestCSRECManageOwnsNavigationEvent(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	calls := 0
	f.Render(ui.CreateElement(budgetRecurringManage, func() { calls++ }))
	n := byTestID(f, "button", "budgets-recurring-manage")
	foot := false
	for _, div := range f.AllByTag("div") {
		foot = foot || div.Attr("class") == "brc-foot"
	}
	if n == nil || n.Attr("type") != "button" || n.Attr("class") != "btn btn-sm" || n.Text() != uistate.T("budgets.recurring.manage") || !foot {
		t.Fatalf("manage=%v", n)
	}
	n.Click()
	if calls != 1 {
		t.Fatal("manage action was not delivered")
	}
}

func TestCSRECBodyKeepsSectionsAndExclusionCopy(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	p := csrecBody(t)
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	if n := byTestID(f, "span", "budgets-recurring-covered-total"); n == nil || n.Text() != "$100.00" {
		t.Fatalf("covered total=%v", n)
	}
	if n := byTestID(f, "span", "budgets-recurring-unbudgeted-total"); n == nil || n.Text() != uistate.T("budgets.recurring.unbudgetedMonthly", "$27.00") {
		t.Fatalf("unbudgeted total=%v", n)
	}
	if len(f.AllByTag("h2")) != 3 || len(f.AllByTag("ul")) != 3 || len(f.AllByTag("li")) != 5 {
		t.Fatal("section/list structure changed")
	}
	wantTitles := []string{uistate.T("budgets.recurring.coveredTitle"), uistate.T("budgets.recurring.unbudgetedTitle"), uistate.T("budgets.recurring.excludedTitle")}
	for i, n := range f.AllByTag("h2") {
		if n.Text() != wantTitles[i] || n.Attr("class") != "brc-section-title" {
			t.Fatalf("section heading=%v", n)
		}
	}
	list := f.AllByTag("ul")[2]
	if list.Attr("class") != "brc-excluded-list" || list.Text() != "Euro bill" {
		t.Fatalf("excluded=%v", list)
	}
	p.Model.Excluded = nil
	p.Summary = recurringview.PresentSummary(p.Model, p.Text)
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	if len(f.AllByTag("h2")) != 2 || len(f.AllByTag("ul")) != 2 {
		t.Fatal("empty exclusion section did not disappear")
	}
}

func TestCSRECBodyKeepsRowActionsThroughReorderShrinkAndEmpty(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	p := csrecBody(t)
	var calls []string
	p.OnBudget = func(row recurringview.Category) { calls = append(calls, row.CategoryID+":"+row.LimitMajor) }
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	byTestID(f, "button", "budget-this-recurring-food").Click()
	// Reuse the same slice backing array and change its order and contents.
	p.Model.Unbudgeted[0], p.Model.Unbudgeted[1] = p.Model.Unbudgeted[1], p.Model.Unbudgeted[0]
	p.Model.Unbudgeted[0].LimitMajor = "17.00"
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	byTestID(f, "button", "budget-this-recurring-apps").Click()
	p.Model.Unbudgeted = p.Model.Unbudgeted[:1]
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	if byTestID(f, "button", "budget-this-recurring-food") != nil {
		t.Fatal("removed category remains")
	}
	byTestID(f, "button", "budget-this-recurring-apps").Click()
	p.Model.Unbudgeted = nil
	p.Model.Covered = nil
	f.Render(ui.CreateElement(budgetRecurringBody, p))
	if byTestID(f, "li", "budgets-recurring-row") != nil || byTestID(f, "li", "budgets-recurring-unbudgeted-row") != nil {
		t.Fatal("empty list kept rows")
	}
	if !reflect.DeepEqual(calls, []string{"food:12.00", "apps:17.00", "apps:17.00"}) {
		t.Fatalf("wrong row callback: %v", calls)
	}
}

func TestCSRECUnbudgetedRowOwnsActionAndUncategorizedHasNone(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	p := csrecBody(t)
	row := p.Model.Unbudgeted[0]
	var selected recurringview.Category
	props := unbudgetedRecurringCategoryProps{Row: row, OnBudget: func(r recurringview.Category) { selected = r }}
	f.Render(ui.CreateElement(unbudgetedRecurringCategory, props))
	n := byTestID(f, "li", "budgets-recurring-unbudgeted-row")
	spans := f.AllByTag("span")
	if n == nil || n.Attr("class") != "brc-row brc-unbudgeted-row" || len(spans) != 2 || spans[0].Text() != "Food" || spans[1].Text() != "$12.00" {
		t.Fatalf("category row=%v", n)
	}
	byTestID(f, "button", "budget-this-recurring-food").Click()
	if selected.CategoryID != "food" || selected.CategoryName != "Food" || selected.LimitMajor != "12.00" {
		t.Fatalf("seed=%+v", selected)
	}
	props.Row = p.Model.Unbudgeted[2]
	f.Render(ui.CreateElement(unbudgetedRecurringCategory, props))
	if len(f.AllByTag("button")) != 0 {
		t.Fatal("uncategorized row offers Budget this")
	}
}

func TestCSRECRecurringRowKeepsCadenceAutopayDateAndSignedAmounts(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	p := csrecBody(t)
	r := domain.Recurring{ID: "annual", Label: "Insurance", CategoryID: "rent", Amount: money.New(-120000, "USD"), Cadence: domain.CadenceYearly, Autopay: true, NextDue: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
	f.Render(ui.CreateElement(budgetRecurringRow, recurringview.PresentRow(r, p.Model.Categories, p.Input.Now, p.Input.Start, p.Input.End, p.Text)))
	n := byTestID(f, "li", "budgets-recurring-row")
	if n == nil || n.Attr("class") != "brc-row" || !strings.Contains(n.Text(), "Insurance") || !strings.Contains(n.Text(), uistate.T("recurring.cadenceYearly")) || !strings.Contains(n.Text(), uistate.T("budgets.recurring.autopay")) || !strings.Contains(n.Text(), "($1,200.00)") || !strings.Contains(n.Text(), uistate.T("budgets.recurring.perMonth", "($100.00)")) {
		t.Fatalf("bill row=%v", n)
	}
	if date := byTestID(f, "span", "brc-date-overdue"); date == nil || date.Attr("class") != "brc-date is-overdue" || date.Text() != ", "+uistate.T("budgets.recurring.overdue", p.Text.Date(r.NextDue)) {
		t.Fatalf("date=%v", date)
	}
	r.Cadence = domain.CadenceMonthly
	r.NextDue = time.Time{}
	r.Autopay = false
	r.CategoryID = "missing"
	f.Render(ui.CreateElement(budgetRecurringRow, recurringview.PresentRow(r, nil, p.Input.Now, p.Input.Start, p.Input.End, p.Text)))
	if byTestID(f, "span", "brc-date-overdue") != nil || strings.Contains(byTestID(f, "li", "budgets-recurring-row").Text(), uistate.T("budgets.recurring.autopay")) {
		t.Fatal("optional metadata remained")
	}
	for _, span := range f.AllByTag("span") {
		if strings.Contains(span.Attr("class"), "brc-permo") {
			t.Fatal("monthly bill has monthly equivalent")
		}
	}
}

func TestCSRECWidgetHidesEmptyAndFailsClosedOnAggregateOverflow(t *testing.T) {
	app := guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(recurringBillsHarness, budgetSummaryProps{App: app}))
	if byTestID(f, "div", "budgets-recurring") != nil {
		t.Fatal("empty tile visible")
	}
	for _, r := range []domain.Recurring{{ID: "large", Label: "Large", Amount: money.New(-math.MaxInt64, "USD"), Cadence: domain.CadenceMonthly}, {ID: "small", Label: "Small", Amount: money.New(-1, "USD"), Cadence: domain.CadenceMonthly}} {
		if err := app.PutRecurring(r); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Render(ui.CreateElement(recurringBillsHarness, budgetSummaryProps{App: app}))
	f.Stabilize()
	alert := false
	for _, n := range f.AllByTag("div") {
		if n.Attr("role") == "alert" && n.Text() == uistate.T("budgets.annualGridError") {
			alert = true
		}
	}
	if !alert || byTestID(f, "div", "budgets-recurring") != nil {
		t.Fatal("overflow must replace tile with original alert")
	}
}

type csrecWidgetProps struct {
	Budget  budgetSummaryProps
	Observe func(uistate.BudgetAddSeed, string)
}

func csrecWidgetHarness(props csrecWidgetProps) ui.Node {
	seed := uistate.UseBudgetAddSeed().Get()
	target := uistate.UseAddTarget().Get()
	props.Observe(seed, target)
	return ui.CreateElement(budgetRecurringWidget, props.Budget)
}

func TestCSRECWidgetBudgetSeedTracksUpdatedAmountAcrossFolds(t *testing.T) {
	app := guardTestApp(t)
	r := domain.Recurring{ID: "seed", Label: "Gas", CategoryID: "c-gas", Amount: money.New(-1234, "USD"), Cadence: domain.CadenceMonthly}
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	var seed uistate.BudgetAddSeed
	var target string
	props := csrecWidgetProps{Budget: budgetSummaryProps{App: app}, Observe: func(s uistate.BudgetAddSeed, v string) { seed, target = s, v }}
	f.Render(ui.CreateElement(csrecWidgetHarness, props))
	if byTestID(f, "button", "budgets-recurring-manage") != nil {
		t.Fatal("tile did not start collapsed")
	}
	for _, amount := range []int64{-1234, -2345} {
		r.Amount = money.New(amount, "USD")
		if err := app.PutRecurring(r); err != nil {
			t.Fatal(err)
		}
		f.Render(ui.CreateElement(csrecWidgetHarness, props))
		byTestID(f, "button", "budgets-recurring-toggle").Click()
		f.Stabilize()
		button := byTestID(f, "button", "budget-this-recurring-c-gas")
		if button == nil {
			t.Fatal("missing Budget this action")
		}
		button.Click()
		f.Stabilize()
		if seed.CategoryID != "c-gas" || seed.Name != "Gas" || seed.LimitMajor != money.FormatMinor(-amount, 2) || target != "budget" {
			t.Fatalf("budget seed=%+v target=%q", seed, target)
		}
		byTestID(f, "button", "budgets-recurring-toggle").Click()
		f.Stabilize()
		if byTestID(f, "button", "budgets-recurring-manage") != nil {
			t.Fatal("collapsed tile retained body")
		}
	}
}
