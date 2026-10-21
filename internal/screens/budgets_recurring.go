// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	recurringview "github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetRecurringWidget reads snapshots and binds navigation and budget-form
// actions. All bill selection, calculations and presentation decisions are pure.
func budgetRecurringWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	member := uistate.UseActiveMember().Get()
	nav := router.UseNavigate()
	open := ui.UseState(false)
	start, end := uistate.UsePeriod().Get().Range()
	app := props.App
	settings := app.Settings()
	input := recurringview.Input{
		Recurring: app.Recurring(), Budgets: app.Budgets(), Categories: app.Categories(), Accounts: app.Accounts(),
		BaseCurrency: settings.BaseCurrency, Rates: settings.FXRates, MemberID: member,
		Now: time.Now(), Start: start, End: end,
	}
	model, err := recurringview.Build(input, uistate.T("budgets.recurring.uncategorized"), uistate.T("budgets.recurring.unnamedBill"))
	if err != nil {
		return Div(Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	if !model.Visible {
		return Fragment()
	}
	text := recurringview.Text{T: uistate.T, TN: uistate.TN, Date: uistate.LoadPrefs().FormatDate, FaintClass: tw.Fold(tw.TextFaint)}
	summary := recurringview.PresentSummary(model, text)
	fold := ui.CreateElement(budgetRecurringFold, budgetRecurringFoldProps{
		Open: open.Get(), Hint: summary.FoldHint,
		OnToggle: func() { open.Set(recurringview.PresentDisclosure(open.Get()).NextOpen) },
	})
	var body ui.Node = Fragment()
	if open.Get() {
		body = ui.CreateElement(budgetRecurringBody, budgetRecurringBodyProps{
			Model: model, Summary: summary, Input: input, Text: text,
			OnManage: func() { nav.Navigate(uistate.RoutePath("/recurring")) },
			OnBudget: func(row recurringview.Category) {
				uistate.SetBudgetAddSeed(uistate.BudgetAddSeed{Name: row.CategoryName, CategoryID: row.CategoryID, LimitMajor: row.LimitMajor})
				uistate.SetAddTarget("budget")
			},
		})
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-recurring", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("brc budget-fold"), Attr("data-testid", "budgets-recurring"), fold, body),
	})
}

type budgetRecurringFoldProps struct {
	Open     bool
	Hint     string
	OnToggle func()
}

// budgetRecurringFold owns the disclosure's hooks at a fixed render position.
func budgetRecurringFold(props budgetRecurringFoldProps) ui.Node {
	toggle := ui.UseEvent(Prevent(props.OnToggle))
	disclosure := recurringview.PresentDisclosure(props.Open)
	return Div(css.Class("budget-fold-head"),
		Button(css.Class("budget-fold-toggle"), Type("button"), Attr("data-testid", "budgets-recurring-toggle"),
			Attr("aria-expanded", disclosure.Expanded), Attr("aria-label", uistate.T(disclosure.AriaKey)), OnClick(toggle),
			Span(ClassStr(disclosure.CaretClass), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.recurring.allCategoriesTitle")),
			Span(css.Class("budget-fold-toggle-hint"), props.Hint),
		),
	)
}

type budgetRecurringBodyProps struct {
	Model    recurringview.Model
	Summary  recurringview.Summary
	Input    recurringview.Input
	Text     recurringview.Text
	OnManage func()
	OnBudget func(recurringview.Category)
}

// budgetRecurringBody composes the unchanged covered, unbudgeted and excluded
// sections. Interactive list rows are keyed components with their own hooks.
func budgetRecurringBody(props budgetRecurringBodyProps) ui.Node {
	return Fragment(
		ui.CreateElement(budgetRecurringHeadline, props.Summary),
		P(css.Class("muted", tw.Text13), uistate.T("budgets.recurring.coverageDesc")),
		Div(css.Class("brc-section"),
			Div(css.Class("brc-section-head"),
				H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.coveredTitle")),
				Span(css.Class("brc-section-total"), Attr("data-testid", "budgets-recurring-covered-total"), props.Summary.Covered)),
			Ul(css.Class("brc-rows"), MapKeyed(props.Model.Covered, func(r domain.Recurring) any { return r.ID }, func(r domain.Recurring) ui.Node {
				return ui.CreateElement(budgetRecurringRow, recurringview.PresentRow(r, props.Model.Categories, props.Input.Now, props.Input.Start, props.Input.End, props.Text))
			})),
		),
		Div(css.Class("brc-section"),
			Div(css.Class("brc-section-head"),
				H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.unbudgetedTitle")),
				Span(css.Class("brc-section-total"), Attr("data-testid", "budgets-recurring-unbudgeted-total"), props.Summary.Unbudgeted)),
			Ul(css.Class("brc-rows"), MapKeyed(props.Model.Unbudgeted, func(row recurringview.Category) any { return row.CategoryID }, func(row recurringview.Category) ui.Node {
				return ui.CreateElement(unbudgetedRecurringCategory, unbudgetedRecurringCategoryProps{Row: row, OnBudget: props.OnBudget})
			})),
		),
		If(props.Summary.ShowExcluded, Div(css.Class("brc-section brc-excluded"),
			Div(css.Class("brc-section-head"),
				H2(css.Class("brc-section-title"), uistate.T("budgets.recurring.excludedTitle")),
				Span(css.Class("brc-section-total"), props.Summary.ExcludedCount),
			),
			Ul(css.Class("brc-excluded-list"), Map(props.Model.Excluded, func(label string) ui.Node { return Li(label) })),
			P(css.Class("muted"), uistate.T("budgets.recurring.excludedReason")),
		)),
		ui.CreateElement(budgetRecurringManage, props.OnManage),
	)
}

func budgetRecurringHeadline(summary recurringview.Summary) ui.Node {
	return Div(css.Class("brc-head"),
		Div(
			Span(css.Class("brc-total-label", tw.TextDim), uistate.T("budgets.recurring.monthlyBillsLabel")),
			Span(css.Class("brc-total-val fig", tw.Fold(tw.FontDisplay)), summary.Total),
		),
		Span(css.Class("brc-count", tw.TextDim), summary.Count),
	)
}

// budgetRecurringManage owns the navigation button's event hooks.
func budgetRecurringManage(onManage func()) ui.Node {
	manage := ui.UseEvent(Prevent(onManage))
	return Div(css.Class("brc-foot"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-recurring-manage"),
			OnClick(manage),
			uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("budgets.recurring.manage"))),
	)
}

type unbudgetedRecurringCategoryProps struct {
	Row      recurringview.Category
	OnBudget func(recurringview.Category)
}

// unbudgetedRecurringCategory owns the per-row Budget this event hook.
func unbudgetedRecurringCategory(props unbudgetedRecurringCategoryProps) ui.Node {
	openBudget := ui.UseEvent(Prevent(func() { props.OnBudget(props.Row) }))
	openBudgetOnClick := OnClick(openBudget)
	row := []ui.Node{
		Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), props.Row.CategoryName),
		Span(css.Class("brc-amt fig", tw.Fold(tw.FontDisplay)), props.Row.Amount),
	}
	if props.Row.CanBudget {
		row = append(row, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "budget-this-recurring-"+props.Row.CategoryID),
			openBudgetOnClick, uistate.T("budgets.budgetThis")))
	}
	return Li(css.Class("brc-row", "brc-unbudgeted-row"), Attr("data-testid", "budgets-recurring-unbudgeted-row"), row)
}

// budgetRecurringRow renders the already formatted bill, date and monthly amount.
func budgetRecurringRow(row recurringview.Row) ui.Node {
	meta := []ui.Node{Span(row.Category)}
	if row.Autopay {
		meta = append(meta, Span(css.Class(tw.TextFaint), row.AutopayText))
	}
	if row.ShowDate {
		meta = append(meta, Span(ClassStr(row.DateClass), Attr("data-testid", row.DateTestID), row.Date))
	}
	amountNodes := []ui.Node{Span(css.Class("brc-amt fig", tw.Fold(tw.FontDisplay)), row.Amount)}
	if row.ShowPerMonth {
		amountNodes = append(amountNodes, Span(css.Class("brc-permo fig", tw.TextFaint), row.PerMonth))
	}
	return Li(css.Class("brc-row"), Attr("data-testid", "budgets-recurring-row"),
		Span(css.Class("brc-cadence"), row.Cadence),
		Div(css.Class("brc-body"),
			Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), row.Label),
			Div(css.Class("brc-meta", tw.Text12, tw.TextDim), meta),
		),
		Div(css.Class("brc-amtcol"), amountNodes),
	)
}

// These adapters keep existing private-symbol wasm tests unchanged. Decisions
// and calculations live solely in the native-tested presentation package.
func recurringBudgetCoverage(budgets []domain.Budget, categories []domain.Category, memberID string) map[string]bool {
	return recurringview.Coverage(budgets, categories, memberID)
}

func recurringInMemberLens(r domain.Recurring, memberID string, accounts []domain.Account) bool {
	return recurringview.InMemberLens(r, memberID, accounts)
}

func recurringFXExcludedLabel(r domain.Recurring) string {
	return recurringview.ExcludedLabel(r, uistate.T("budgets.recurring.unnamedBill"))
}

func recurringBudgetAmount(r domain.Recurring, base string, rates currency.Rates) (int64, string, error) {
	return recurringview.BudgetAmount(r, base, rates, uistate.T("budgets.recurring.unnamedBill"))
}

func recurringDateState(next, now, start, end time.Time) budgeting.RecurDateState {
	return recurringview.DateState(next, now, start, end)
}

func recurringMonthlyEquivalentChecked(r domain.Recurring) (int64, error) {
	return recurringview.MonthlyEquivalent(r)
}

func recurringMonthlyBase(r domain.Recurring, base string, rates currency.Rates) (int64, error) {
	return recurringview.MonthlyBase(r, base, rates)
}
