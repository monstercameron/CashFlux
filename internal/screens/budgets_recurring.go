// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// recurringBudgetCoverage returns every category covered directly or through a
// budget's parent category.
func recurringBudgetCoverage(budgets []domain.Budget, categories []domain.Category, memberID string) map[string]bool {
	roots := make([]string, 0)
	for _, budget := range budgets {
		if memberID != "" && budget.Scope == domain.ScopeIndividual && budget.OwnerID != memberID {
			continue
		}
		roots = append(roots, budget.TrackedCategoryIDs()...)
	}
	return categorytree.DescendantsOfAll(categories, roots)
}

// budgetRecurringWidget lists the household's active recurring commitments with
// their cadence (frequency) and per-month normalized cost, surfaced right on the
// budgets page (coworker feedback #5). It reads the confirmed recurring set
// (app.Recurring()), not raw guesses, so every row is a real repeating charge.
// Self-gates to nothing when there are no active recurrings, and the rows are
// display-only — the single footer button navigates to the full /recurring
// surface for editing (keeping per-row handlers out of the loop).
func budgetRecurringWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	activeMember := uistate.UseActiveMember().Get()
	nav := router.UseNavigate()
	goToRecurring := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/recurring")) }))
	open := ui.UseState(false)
	toggleOpen := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	toggleOnClick := OnClick(toggleOpen)
	manageOnClick := OnClick(goToRecurring)
	recurNow := time.Now().UTC()
	recurStart, recurEnd := uistate.UsePeriod().Get().Range()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Only active outflows belong in a bill list. Grouping is performed after
	// category coverage is resolved so income can never inflate either section.
	recs := make([]domain.Recurring, 0)
	accounts := app.Accounts()
	for _, r := range app.Recurring() {
		if r.Active() && r.Amount.IsNegative() && recurringInMemberLens(r, activeMember, accounts) {
			recs = append(recs, r)
		}
	}
	if len(recs) == 0 {
		return Fragment() // nothing detected yet — stay quiet
	}
	absMonthly := func(r domain.Recurring) int64 {
		m, err := recurringMonthlyEquivalentChecked(r)
		if err != nil {
			return 0
		}
		if m < 0 {
			abs, err := money.Abs(m)
			if err != nil {
				return 0
			}
			return abs
		}
		return m
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return absMonthly(recs[i]) > absMonthly(recs[j])
	})

	catName := make(map[string]string, len(app.Categories()))
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	coveredCategories := recurringBudgetCoverage(app.Budgets(), app.Categories(), activeMember)
	covered := make([]domain.Recurring, 0, len(recs))
	excluded := make([]string, 0)
	unbudgetedByCategory := make(map[string]int64)
	var coveredMonthly, unbudgetedMonthly int64
	validBillCount := 0
	for _, r := range recs {
		amount, excludedLabel, err := recurringBudgetAmount(r, base, rates)
		if err != nil {
			excluded = append(excluded, excludedLabel)
			continue
		}
		validBillCount++
		if coveredCategories[r.CategoryID] {
			covered = append(covered, r)
			coveredMonthly, err = money.Add(coveredMonthly, amount)
			if err != nil {
				return Div(Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
			continue
		}
		unbudgetedByCategory[r.CategoryID], err = money.Add(unbudgetedByCategory[r.CategoryID], amount)
		if err != nil {
			return Div(Attr("role", "alert"), uistate.T("budgets.annualGridError"))
		}
		unbudgetedMonthly, err = money.Add(unbudgetedMonthly, amount)
		if err != nil {
			return Div(Attr("role", "alert"), uistate.T("budgets.annualGridError"))
		}
	}
	if validBillCount == 0 && len(excluded) == 0 {
		return Fragment()
	}
	totalMonthly, err := money.Add(coveredMonthly, unbudgetedMonthly)
	if err != nil {
		return Div(Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	sort.SliceStable(covered, func(i, j int) bool {
		return absMonthly(covered[i]) > absMonthly(covered[j])
	})
	sort.Strings(excluded)

	// C609: every date is stated against a reference. The list showed "Next Jul 3,
	// 2026" beside "Next Sep 1, 2026" with nothing saying whether either had
	// happened, was about to, or belonged to the period on screen — three
	// different facts under one word.
	coveredRows := make([]ui.Node, 0, len(covered))
	for _, r := range covered {
		coveredRows = append(coveredRows, budgetRecurringRow(r, catName, recurNow, recurStart, recurEnd))
	}
	unbudgetedCategories := make([]string, 0, len(unbudgetedByCategory))
	for categoryID := range unbudgetedByCategory {
		unbudgetedCategories = append(unbudgetedCategories, categoryID)
	}
	sort.Slice(unbudgetedCategories, func(i, j int) bool {
		return unbudgetedByCategory[unbudgetedCategories[i]] > unbudgetedByCategory[unbudgetedCategories[j]]
	})
	unbudgetedRowsView := make([]ui.Node, 0, len(unbudgetedCategories))
	for _, categoryID := range unbudgetedCategories {
		cat := catName[categoryID]
		if cat == "" {
			cat = uistate.T("budgets.recurring.uncategorized")
		}
		limit := money.FormatMinor(unbudgetedByCategory[categoryID], currency.Decimals(base))
		unbudgetedRowsView = append(unbudgetedRowsView, ui.CreateElement(unbudgetedRecurringCategory, unbudgetedRecurringCategoryProps{
			CategoryID: categoryID, CategoryName: cat, Monthly: money.New(unbudgetedByCategory[categoryID], base),
			LimitMajor: limit,
		}))
	}

	head := Div(css.Class("brc-head"),
		Div(
			Span(css.Class("brc-total-label", tw.TextDim), uistate.T("budgets.recurring.monthlyBillsLabel")),
			Span(css.Class("brc-total-val fig", tw.Fold(tw.FontDisplay)),
				uistate.T("budgets.recurring.totalVal", fmtMoney(money.New(totalMonthly, base)))),
		),
		Span(css.Class("brc-count", tw.TextDim), uistate.TN("budgets.recurring.billOne", "budgets.recurring.billMany", validBillCount)),
	)

	// Collapsible, like the two sections below it. This is the tallest tile on the
	// surface — thirteen rows, over a thousand pixels — for a figure that changes a
	// few times a year, so it opens closed and states its own headline: the monthly
	// commitment and how many charges make it up. That is the whole answer most
	// visits need; you open it to see WHICH charges (Cam, 2026-08-31).
	caretCls := "budget-fold-caret"
	foldAria := uistate.T("budgets.recurring.showAria")
	if open.Get() {
		caretCls += " is-open"
		foldAria = uistate.T("budgets.recurring.hideAria")
	}
	fold := Div(css.Class("budget-fold-head"),
		Button(css.Class("budget-fold-toggle"), Type("button"), Attr("data-testid", "budgets-recurring-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", foldAria), toggleOnClick,
			Span(ClassStr(caretCls), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.recurring.allCategoriesTitle")),
			Span(css.Class("budget-fold-toggle-hint"),
				uistate.TN("budgets.recurring.billFoldOne", "budgets.recurring.billFoldMany",
					validBillCount, fmtMoney(money.New(totalMonthly, base)))),
		),
	)

	var body ui.Node = Fragment()
	if open.Get() {
		body = Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.recurring.coverageDesc")),
			Div(css.Class("brc-section"),
				Div(css.Class("brc-section-head"),
					H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.coveredTitle")),
					Span(css.Class("brc-section-total"), Attr("data-testid", "budgets-recurring-covered-total"), fmtMoney(money.New(coveredMonthly, base)))),
				Ul(css.Class("brc-rows"), coveredRows),
			),
			Div(css.Class("brc-section"),
				Div(css.Class("brc-section-head"),
					H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.unbudgetedTitle")),
					Span(css.Class("brc-section-total"), Attr("data-testid", "budgets-recurring-unbudgeted-total"), uistate.T("budgets.recurring.unbudgetedMonthly", fmtMoney(money.New(unbudgetedMonthly, base))))),
				Ul(css.Class("brc-rows"), unbudgetedRowsView),
			),
			If(len(excluded) > 0, Div(css.Class("brc-section brc-excluded"),
				Div(css.Class("brc-section-head"),
					H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.excludedTitle")),
					Span(css.Class("brc-section-total"), uistate.TN("budgets.recurring.excludedOne", "budgets.recurring.excludedMany", len(excluded))),
				),
				Ul(css.Class("brc-excluded-list"), Map(excluded, func(label string) ui.Node {
					return Li(label)
				})),
				P(css.Class("muted"), uistate.T("budgets.recurring.excludedReason")),
			)),
			Div(css.Class("brc-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-recurring-manage"),
					manageOnClick,
					uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("budgets.recurring.manage"))),
			),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-recurring", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("brc budget-fold"), Attr("data-testid", "budgets-recurring"), fold, body),
	})
}

func recurringInMemberLens(recurring domain.Recurring, memberID string, accounts []domain.Account) bool {
	if memberID == "" || recurring.AccountID == "" {
		return true
	}
	for _, account := range accounts {
		if account.ID != recurring.AccountID {
			continue
		}
		return account.Scope != domain.ScopeIndividual || account.OwnerID == memberID || account.OwnerID == domain.GroupOwnerID
	}
	return false
}

func recurringFXExcludedLabel(recurring domain.Recurring) string {
	label := strings.TrimSpace(recurring.Label)
	if label == "" {
		return uistate.T("budgets.recurring.unnamedBill")
	}
	return label
}

func recurringBudgetAmount(recurring domain.Recurring, base string, rates currency.Rates) (int64, string, error) {
	amount, err := recurringMonthlyBase(recurring, base, rates)
	if err != nil {
		return 0, recurringFXExcludedLabel(recurring), err
	}
	return amount, "", nil
}

type unbudgetedRecurringCategoryProps struct {
	CategoryID, CategoryName string
	Monthly                  money.Money
	LimitMajor               string
}

// unbudgetedRecurringCategory renders one category total and owns its Budget-this
// event hook, keeping hook registration out of the variable-length category list.
func unbudgetedRecurringCategory(props unbudgetedRecurringCategoryProps) ui.Node {
	openBudget := ui.UseEvent(Prevent(func() {
		if props.CategoryID == "" {
			return
		}
		uistate.SetBudgetAddSeed(uistate.BudgetAddSeed{
			Name: props.CategoryName, CategoryID: props.CategoryID, LimitMajor: props.LimitMajor,
		})
		uistate.SetAddTarget("budget")
	}))
	openBudgetOnClick := OnClick(openBudget)
	row := []ui.Node{
		Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), props.CategoryName),
		Span(css.Class("brc-amt fig", tw.Fold(tw.FontDisplay)), fmtMoney(props.Monthly)),
	}
	if props.CategoryID != "" {
		row = append(row, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "budget-this-recurring-"+props.CategoryID),
			openBudgetOnClick, uistate.T("budgets.budgetThis")))
	}
	return Li(css.Class("brc-row", "brc-unbudgeted-row"), Attr("data-testid", "budgets-recurring-unbudgeted-row"), row)
}

// budgetRecurringRow renders one recurring commitment: its label, category, the
// cadence as a frequency pill, next-due date, amount, and — when the cadence
// isn't monthly — the normalized per-month figure so charges compare fairly.
// Display-only (no handlers), safe to build inside the row loop.
func budgetRecurringRow(r domain.Recurring, catName map[string]string, now, periodStart, periodEnd time.Time) ui.Node {
	cat := catName[r.CategoryID]
	if cat == "" {
		cat = uistate.T("budgets.recurring.uncategorized")
	}
	meta := []ui.Node{Span(cat)}
	if r.Autopay {
		meta = append(meta, Span(css.Class(tw.TextFaint), ", "+uistate.T("budgets.recurring.autopay")))
	}
	// C609: the date carries its own meaning — overdue, due inside the period on
	// screen, or the schedule's next date after it.
	if state := budgeting.ClassifyRecurDate(r.NextDue, now, periodStart, periodEnd); state != budgeting.RecurUnscheduled {
		when := uistate.LoadPrefs().FormatDate(r.NextDue)
		meta = append(meta, Span(ClassStr(recurDateClass(state)), Attr("data-testid", "brc-date-"+string(state)),
			", "+uistate.T(recurDateKey(state), when)))
	}

	// Amount block: the charge as-is, plus a per-month equivalent when the cadence
	// isn't already monthly (so a $1,200 annual bill reads "≈ $100/mo" too).
	amountNodes := []ui.Node{Span(css.Class("brc-amt fig", tw.Fold(tw.FontDisplay)), fmtMoney(r.Amount))}
	monthly, monthlyErr := recurringMonthlyEquivalentChecked(r)
	if r.Cadence != domain.CadenceMonthly && monthlyErr == nil {
		amountNodes = append(amountNodes, Span(css.Class("brc-permo fig", tw.TextFaint),
			uistate.T("budgets.recurring.perMonth", fmtMoney(money.New(monthly, r.Amount.Currency)))))
	}

	return Li(css.Class("brc-row"), Attr("data-testid", "budgets-recurring-row"),
		Span(css.Class("brc-cadence"), recurCadence(r.Cadence)),
		Div(css.Class("brc-body"),
			Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), r.Label),
			Div(css.Class("brc-meta", tw.Text12, tw.TextDim), meta),
		),
		Div(css.Class("brc-amtcol"), amountNodes),
	)
}

func recurringMonthlyEquivalentChecked(r domain.Recurring) (int64, error) {
	switch r.Cadence {
	case domain.CadenceWeekly:
		return money.MulDiv(r.Amount.Amount, 52, 12, money.RoundTowardZero)
	case domain.CadenceBiweekly:
		return money.MulDiv(r.Amount.Amount, 26, 12, money.RoundTowardZero)
	case domain.CadenceSemimonthly:
		return money.MulDiv(r.Amount.Amount, 2, 1, money.RoundTowardZero)
	case domain.CadenceQuarterly:
		return money.MulDiv(r.Amount.Amount, 1, 3, money.RoundTowardZero)
	case domain.CadenceYearly:
		return money.MulDiv(r.Amount.Amount, 1, 12, money.RoundTowardZero)
	default:
		return r.Amount.Amount, nil
	}
}

func recurringMonthlyBase(r domain.Recurring, base string, rates currency.Rates) (int64, error) {
	monthly, err := recurringMonthlyEquivalentChecked(r)
	if err != nil {
		return 0, err
	}
	positive, err := money.Neg(monthly)
	if err != nil {
		return 0, err
	}
	return currency.ConvertBetween(positive, r.Amount.Currency, base, rates)
}

// recurDateKey / recurDateClass give each date state its own words and tone
// (C609). Only "overdue" is a call to action, so only it is toned; the other two
// are context and stay quiet.
func recurDateKey(state budgeting.RecurDateState) string {
	switch state {
	case budgeting.RecurOverdue:
		return "budgets.recurring.overdue"
	case budgeting.RecurAfterPeriod:
		return "budgets.recurring.afterPeriod"
	case budgeting.RecurBeforePeriod:
		return "budgets.recurring.beforePeriod"
	}
	return "budgets.recurring.dueInPeriod"
}

func recurDateClass(state budgeting.RecurDateState) string {
	if state == budgeting.RecurOverdue {
		return "brc-date is-overdue"
	}
	return "brc-date " + tw.Fold(tw.TextFaint)
}
