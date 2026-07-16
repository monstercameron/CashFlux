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
		// Horizontal scroll lives INSIDE the card so the page body never scrolls sideways
		// (the .budget-annualgrid-scroll frame owns overflow-x + max-width).
		Div(css.Class("budget-annualgrid-scroll"),
			annualGridTable(grid, props)),
	)
}

// gridColClass returns the modifier classes for a data/header column at month index i:
// the current-month accent band and the leading-divider Total column. Empty for a
// plain month column. (i == 12 is the Total column.)
func gridColClass(i, current int) string {
	switch {
	case i == 12:
		return " is-total"
	case i == current:
		return " is-current"
	default:
		return ""
	}
}

// annualGridTable renders the matrix itself. A sticky header row and sticky first
// column keep the month labels and budget names visible while scrolling; all styling
// is class-driven (see rules_annualgrid.go), so cells read as one designed grid.
func annualGridTable(grid budgeting.AnnualGrid, props budgetAnnualGridProps) ui.Node {
	// Header row: "Budget", the twelve months (current one banded), then "Total".
	headCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner"), Attr("scope", "col"),
			uistate.T("budgets.annualGridBudgetCol")),
	}
	for i, name := range annualGridMonths {
		headCells = append(headCells, Th(ClassStr("budget-annualgrid-th"+gridColClass(i, grid.CurrentMonth)), Attr("scope", "col"), name))
	}
	headCells = append(headCells, Th(css.Class("budget-annualgrid-th", "is-total"), Attr("scope", "col"),
		uistate.T("budgets.annualGridTotalCol")))

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
		Th(css.Class("budget-annualgrid-corner", "is-foot"), Attr("scope", "row"),
			uistate.T("budgets.annualGridTotalCol")),
	}
	for i := 0; i < 12; i++ {
		footCells = append(footCells, Td(ClassStr("budget-annualgrid-td is-foot"+gridColClass(i, grid.CurrentMonth)),
			fmtMoney(grid.MonthActualTotals[i])))
	}
	footCells = append(footCells, Td(css.Class("budget-annualgrid-td", "is-foot", "is-total"), fmtMoney(grid.GrandActual)))

	return Table(css.Class("budget-annualgrid-table"),
		Thead(Tr(headCells)),
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
	cells := []ui.Node{
		Th(css.Class("budget-annualgrid-rowhead"), Attr("scope", "row"), row.Name),
	}
	for m := 0; m < 12; m++ {
		cell := row.Cells[m]
		from := time.Date(props.Year, time.Month(m+1), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		to := time.Date(props.Year, time.Month(m+2), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		cells = append(cells, ui.CreateElement(annualGridCell, annualGridCellProps{
			BudgetID: row.BudgetID, CategoryIDs: props.CategoryIDs, Cell: cell,
			Current: m == props.CurrentMonth, From: from, To: to, OnCell: props.OnCell,
		}))
	}
	cells = append(cells, Td(css.Class("budget-annualgrid-td", "is-total"), fmtMoney(row.ActualTotal)))
	return Tr(css.Class("budget-annualgrid-tr"), cells)
}

// annualGridCellProps is one plan-vs-actual cell.
type annualGridCellProps struct {
	BudgetID    string
	CategoryIDs []string
	Cell        budgeting.AnnualGridCell
	Current     bool // in the current-month column (gets the accent band)
	From, To    string
	OnCell      func(categoryIDs []string, from, to string)
}

// annualGridCell renders one cell (actual over plan) as a button that drills to the
// month's filtered transactions. Its own component so the click handler is a stable
// hook. All styling is class-driven: .is-current bands the current month, .is-over
// tints an overspent cell and reddens its actual figure (see rules_annualgrid.go).
func annualGridCell(props annualGridCellProps) ui.Node {
	click := ui.UseEvent(Prevent(func() {
		if props.OnCell != nil {
			props.OnCell(props.CategoryIDs, props.From, props.To)
		}
	}))
	cls := "budget-annualgrid-td"
	if props.Current {
		cls += " is-current"
	}
	if props.Cell.Over {
		cls += " is-over"
	}
	return Td(ClassStr(cls),
		Button(css.Class("budget-annualgrid-cell"), Type("button"),
			Attr("data-testid", "annualgrid-cell-"+props.BudgetID+"-"+props.From),
			OnClick(click),
			Span(css.Class("budget-annualgrid-actual"), fmtMoney(props.Cell.Actual)),
			Span(css.Class("budget-annualgrid-plan"), fmtMoney(props.Cell.Plan)),
		),
	)
}
