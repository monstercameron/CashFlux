// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetAnnualGridProps feeds the BG9 annual grid: the raw dataset it projects and
// a drill callback that opens Transactions filtered to a clicked cell's budget +
// month. CatName resolves category IDs to display names for the drill.
type budgetAnnualGridProps struct {
	Budgets   []domain.Budget
	Txns      []domain.Transaction
	Cats      []domain.Category
	Rates     currency.Rates
	WeekStart time.Weekday
	Now       time.Time
	// OnCell opens Transactions filtered to the clicked cell: the budget's tracked
	// categories over the [from, to) month window (dates as "2006-01-02").
	OnCell func(categoryIDs []string, from, to string)
}

var annualGridMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// BudgetAnnualGrid renders the BG9 categories×months plan-vs-actual matrix as a
// collapsible, view-only card: a wide table with a sticky header row and sticky
// first column, horizontal scroll inside the card, row/column totals, the current
// month highlighted, and over-cells toned. Clicking a cell drills to that month's
// filtered transactions. It owns its own hooks (year + open toggle) so it never
// disturbs the surrounding list's hook order.
func BudgetAnnualGrid(props budgetAnnualGridProps) ui.Node {
	open := ui.UseState(false)
	year := ui.UseState(props.Now.Year())

	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	prevYear := ui.UseEvent(Prevent(func() { year.Set(year.Get() - 1) }))
	nextYear := ui.UseEvent(Prevent(func() { year.Set(year.Get() + 1) }))

	header := Div(css.Class("budget-annualgrid-head"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle),
			uiw.Icon(icon.Budgets, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			uistate.T("budgets.annualGridTitle")),
	)

	if !open.Get() {
		return Div(css.Class("budget-annualgrid"), header)
	}

	// Build per-budget rollup cover sets, then the pure grid.
	covers := map[string]map[string]bool{}
	for _, b := range props.Budgets {
		covers[b.ID] = categorytree.DescendantsOfAll(props.Cats, b.TrackedCategoryIDs())
	}
	grid, err := budgeting.BuildAnnualGrid(props.Budgets, props.Txns, year.Get(), props.Rates, props.WeekStart, props.Now, covers)
	if err != nil {
		return Div(css.Class("budget-annualgrid"), header,
			Span(css.Class("budget-sub", tw.TextDown), Attr("role", "alert"), uistate.T("budgets.annualGridError")))
	}

	yearControls := Div(css.Class("budget-annualgrid-year"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-prev"),
			Attr("aria-label", uistate.T("budgets.annualGridPrevYear")), OnClick(prevYear),
			uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Span(css.Class("budget-annualgrid-yearlabel"), strconv.Itoa(grid.Year)),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-next"),
			Attr("aria-label", uistate.T("budgets.annualGridNextYear")), OnClick(nextYear),
			uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
	)

	return Div(css.Class("budget-annualgrid"),
		header,
		yearControls,
		// Horizontal scroll lives INSIDE the card so the page body never scrolls sideways.
		Div(css.Class("budget-annualgrid-scroll"), Attr("style", "overflow-x:auto;max-width:100%"),
			annualGridTable(grid, props)),
	)
}

// annualGridTable renders the matrix itself. A sticky header row and sticky first
// column keep the month labels and budget names visible while scrolling.
func annualGridTable(grid budgeting.AnnualGrid, props budgetAnnualGridProps) ui.Node {
	// Header row: "Budget", the twelve months (current one highlighted), then "Total".
	headCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner"), Attr("scope", "col"),
			Attr("style", "position:sticky;left:0;z-index:2;background:var(--bg-elev)"),
			uistate.T("budgets.annualGridBudgetCol")),
	}
	for i, name := range annualGridMonths {
		cls := "budget-annualgrid-th"
		style := "text-align:right"
		if i == grid.CurrentMonth {
			style += ";background:var(--bg-elev);font-weight:600"
		}
		headCells = append(headCells, Th(css.Class(cls), Attr("scope", "col"), Attr("style", style), name))
	}
	headCells = append(headCells, Th(css.Class("budget-annualgrid-th"), Attr("scope", "col"),
		Attr("style", "text-align:right;font-weight:600"), uistate.T("budgets.annualGridTotalCol")))

	// Map each budget to its tracked categories so a cell click can filter Transactions.
	catsByBudget := map[string][]string{}
	for _, b := range props.Budgets {
		catsByBudget[b.ID] = b.TrackedCategoryIDs()
	}
	var bodyRows []ui.Node
	for _, row := range grid.Rows {
		bodyRows = append(bodyRows, ui.CreateElement(annualGridRow, annualGridRowProps{
			Row: row, CategoryIDs: catsByBudget[row.BudgetID], CurrentMonth: grid.CurrentMonth, Year: grid.Year, OnCell: props.OnCell,
		}))
	}

	// Footer: column totals across all budgets + the grand total.
	footCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner"), Attr("scope", "row"),
			Attr("style", "position:sticky;left:0;z-index:1;background:var(--bg-elev);text-align:left"),
			uistate.T("budgets.annualGridTotalCol")),
	}
	for i := 0; i < 12; i++ {
		style := "text-align:right;font-weight:600"
		if i == grid.CurrentMonth {
			style += ";background:var(--bg-elev)"
		}
		footCells = append(footCells, Td(css.Class("budget-annualgrid-td"), Attr("style", style),
			fmtMoney(grid.MonthActualTotals[i])))
	}
	footCells = append(footCells, Td(css.Class("budget-annualgrid-td"),
		Attr("style", "text-align:right;font-weight:700"), fmtMoney(grid.GrandActual)))

	return Table(css.Class("budget-annualgrid-table"), Attr("style", "border-collapse:collapse;width:max-content"),
		Thead(Attr("style", "position:sticky;top:0;z-index:3"), Tr(headCells)),
		Tbody(bodyRows),
		Tfoot(Tr(footCells)),
	)
}

// annualGridRowProps is one budget's row in the annual grid.
type annualGridRowProps struct {
	Row          budgeting.AnnualGridRow
	CategoryIDs  []string
	CurrentMonth int
	Year         int
	OnCell       func(categoryIDs []string, from, to string)
}

// annualGridRow is a per-row component so each cell's click handler lives at a
// stable hook position (never a raw On* inside a variable-length loop).
func annualGridRow(props annualGridRowProps) ui.Node {
	row := props.Row
	// The budget's tracked categories drive the drill filter; captured once per row.
	// (AnnualGridRow carries only the ID/name, so drill uses the budget ID's cats via
	// the caller — here we pass through the row's categories embedded by the builder.)
	cells := []ui.Node{
		Th(css.Class("budget-annualgrid-rowhead"), Attr("scope", "row"),
			Attr("style", "position:sticky;left:0;z-index:1;background:var(--bg-card);text-align:left;white-space:nowrap"),
			row.Name),
	}
	for m := 0; m < 12; m++ {
		cell := row.Cells[m]
		tone := ""
		if cell.Over {
			tone = " " + tw.Fold(tw.TextDown)
		}
		style := "text-align:right;white-space:nowrap"
		if m == props.CurrentMonth {
			style += ";background:var(--bg-elev)"
		}
		from := time.Date(props.Year, time.Month(m+1), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		to := time.Date(props.Year, time.Month(m+2), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		cells = append(cells, ui.CreateElement(annualGridCell, annualGridCellProps{
			BudgetID: row.BudgetID, CategoryIDs: props.CategoryIDs, Cell: cell, ToneClass: tone, Style: style, From: from, To: to,
			OnCell: props.OnCell,
		}))
	}
	cells = append(cells, Td(css.Class("budget-annualgrid-td"),
		Attr("style", "text-align:right;font-weight:600"), fmtMoney(row.ActualTotal)))
	return Tr(css.Class("budget-annualgrid-tr"), cells)
}

// annualGridCellProps is one plan-vs-actual cell.
type annualGridCellProps struct {
	BudgetID    string
	CategoryIDs []string
	Cell        budgeting.AnnualGridCell
	ToneClass   string
	Style       string
	From, To    string
	OnCell      func(categoryIDs []string, from, to string)
}

// annualGridCell renders one cell (actual over plan) as a button that drills to the
// month's filtered transactions. Its own component so the click handler is a stable
// hook. It carries no category list; the drill re-derives the budget's categories at
// the callback (the caller closes over the budget set), so a click passes the
// budget-scoped window on.
func annualGridCell(props annualGridCellProps) ui.Node {
	click := ui.UseEvent(Prevent(func() {
		if props.OnCell != nil {
			props.OnCell(props.CategoryIDs, props.From, props.To)
		}
	}))
	return Td(css.Class("budget-annualgrid-td"), Attr("style", props.Style),
		Button(css.Class("budget-annualgrid-cell"), Type("button"),
			Attr("data-testid", "annualgrid-cell-"+props.BudgetID+"-"+props.From),
			Attr("style", "background:transparent;border:0;padding:2px 6px;font:inherit;color:inherit;cursor:pointer;text-align:right;width:100%"),
			OnClick(click),
			Span(ClassStr("budget-annualgrid-actual"+props.ToneClass), fmtMoney(props.Cell.Actual)),
			Span(css.Class("budget-annualgrid-plan", tw.TextFaint),
				Attr("style", "display:block;font-size:0.75em"), fmtMoney(props.Cell.Plan)),
		),
	)
}
