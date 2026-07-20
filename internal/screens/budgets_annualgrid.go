// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetAnnualGridProps feeds the BG9 annual grid: the raw dataset it projects and
// a drill callback that opens Transactions filtered to a clicked cell's budget +
// month. CatName resolves category IDs to display names for the drill. Recurrings
// and Goals feed the future-month projection (C394).
type budgetAnnualGridProps struct {
	Budgets    []domain.Budget
	Txns       []domain.Transaction
	Cats       []domain.Category
	Recurrings []domain.Recurring
	Goals      []domain.Goal
	Rates      currency.Rates
	WeekStart  time.Weekday
	Now        time.Time
	// OnCell opens Transactions filtered to the clicked cell: the budget's tracked
	// categories over the [from, to) month window (dates as "2006-01-02").
	OnCell func(categoryIDs []string, from, to string)
}

var annualGridMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// scenarioStepMinor is one press of the income scenario stepper: ±$100 a month.
const scenarioStepMinor int64 = 10000

// BudgetAnnualGrid renders the BG9 categories×months plan-vs-actual matrix as a
// collapsible, view-only "Plan the year" section (C371): a wide table with a
// sticky header row and sticky first column, row/column totals, the current month
// highlighted and scrolled into view, future months toned distinctly, future
// cells pre-filled with projected recurring + goal amounts (C394), and an
// ephemeral income scenario mode that flags what goes underfunded (C393). Clicking
// a cell drills to that month's filtered transactions. It owns its own hooks so it
// never disturbs the surrounding list's hook order.
func BudgetAnnualGrid(props budgetAnnualGridProps) ui.Node {
	open := ui.UseState(false)
	year := ui.UseState(props.Now.Year())
	scenarioOn := ui.UseState(false)
	incomeDelta := ui.UseState(int64(0))

	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	prevYear := ui.UseEvent(Prevent(func() { year.Set(year.Get() - 1) }))
	nextYear := ui.UseEvent(Prevent(func() { year.Set(year.Get() + 1) }))
	scenarioToggle := ui.UseEvent(Prevent(func() { scenarioOn.Set(!scenarioOn.Get()) }))
	deltaLess := ui.UseEvent(Prevent(func() { incomeDelta.Set(incomeDelta.Get() - scenarioStepMinor) }))
	deltaMore := ui.UseEvent(Prevent(func() { incomeDelta.Set(incomeDelta.Get() + scenarioStepMinor) }))
	deltaReset := ui.UseEvent(Prevent(func() { incomeDelta.Set(0) }))

	// The current-month column index for THIS displayed year (−1 when the displayed
	// year is not the current year), computed without building the grid so the
	// scroll effect below can run at a stable hook position.
	curMonth := -1
	if props.Now.Year() == year.Get() {
		curMonth = int(props.Now.Month()) - 1
	}
	// When opened (or the year changes), bring the current-month column into view so
	// the grid "lands on" today rather than January (C371).
	ui.UseEffect(func() func() {
		if open.Get() && curMonth >= 0 {
			scrollAnnualGridToCurrent()
		}
		return nil
	}, fmt.Sprintf("annualgrid-scroll-%v-%d-%d", open.Get(), year.Get(), curMonth))

	// A proper collapsible-section toggle framed as "Plan the year" (C371): a
	// rotating disclosure caret, a clear title, and a hint of what it opens.
	caretCls := "budget-annualgrid-caret"
	toggleAria := uistate.T("budgets.planYearShowAria")
	if open.Get() {
		caretCls += " is-open"
		toggleAria = uistate.T("budgets.planYearHideAria")
	}
	header := Div(css.Class("budget-annualgrid-head"),
		Button(css.Class("budget-annualgrid-toggle"), Type("button"), Attr("data-testid", "budget-annualgrid-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", toggleAria), OnClick(toggle),
			Span(ClassStr(caretCls), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-annualgrid-toggle-label"), uistate.T("budgets.planYearTitle")),
			Span(css.Class("budget-annualgrid-toggle-hint"), uistate.T("budgets.planYearHint")),
		),
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

	// fromMonth is the first month treated as "future" (projected, distinct wash):
	// the month after the current one for the current year, the whole year (0) for a
	// future year, and none (12) for a past year.
	fromMonth := 12
	if grid.CurrentMonth >= 0 {
		fromMonth = grid.CurrentMonth + 1
	} else if grid.Year > props.Now.Year() {
		fromMonth = 0
	}

	budgetIDs := make([]string, 0, len(grid.Rows))
	for _, r := range grid.Rows {
		budgetIDs = append(budgetIDs, r.BudgetID)
	}
	// C394 — project recurring bills + goal contributions into the future months,
	// folded onto each budget row through its cover set.
	projection := budgetplan.Project(props.Recurrings, props.Goals, grid.Year, fromMonth, grid.Currency, props.Rates)
	perBudget := projection.PerBudget(budgetIDs, covers)

	// C393 — scenario: which budget/month cells go underfunded at the chosen income
	// delta. Ephemeral, view-only — nothing persists.
	scOn := scenarioOn.Get()
	delta := incomeDelta.Get()
	var shortfalls map[string][12]int64
	underCount := 0
	if scOn {
		shortfalls, underCount = annualGridScenario(grid, delta)
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

	scenarioBar := annualGridScenarioBar(scOn, delta, underCount, grid.Currency,
		scenarioToggle, deltaLess, deltaMore, deltaReset)

	return Div(css.Class("budget-annualgrid"),
		header,
		Div(css.Class("budget-annualgrid-head"), yearControls, scenarioBar),
		annualGridLegend(fromMonth <= 11, scOn),
		// Top-anchored scroll cue (E3): the wide 12-month matrix overflows a narrow
		// (expanded-sidebar) pane, and the only scrollbar is at the very bottom of a
		// tall grid — easy to miss. This quiet hint above the frame signals "there's
		// more to the right"; CSS hides it once the pane is wide enough to fit the
		// whole year (rules_qpassEannual.go). aria-hidden: a screen reader already
		// reaches every cell, so this is a sighted-only nudge.
		Div(css.Class("budget-annualgrid-scrollcue"), Attr("aria-hidden", "true"),
			Span(uistate.T("budgets.annualGridScrollCue"))),
		// Horizontal scroll lives INSIDE the card so the page body never scrolls sideways
		// (the .budget-annualgrid-scroll frame owns overflow-x + max-width).
		Div(css.Class("budget-annualgrid-scroll"),
			annualGridTable(grid, props, fromMonth, perBudget, shortfalls)),
	)
}

// scrollAnnualGridToCurrent brings the current-month header column into view
// inside the grid's own horizontal scroll frame (C371). A short timeout lets the
// re-rendered table paint first; block:nearest keeps the page from jumping
// vertically.
func scrollAnnualGridToCurrent() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		cb.Release()
		el := doc.Call("getElementById", "budget-annualgrid-current")
		if !el.Truthy() {
			return nil
		}
		el.Call("scrollIntoView", js.ValueOf(map[string]any{"behavior": "smooth", "inline": "center", "block": "nearest"}))
		return nil
	})
	js.Global().Call("setTimeout", cb, 80)
}

// annualGridScenario runs the C393 funding waterfall for every month at the given
// income delta and returns the per-budget, per-month shortfall (minor units; 0 =
// funded) plus the total count of underfunded cells. The month's baseline income
// is its own total plan, so a zero delta funds everything and a negative delta
// reveals exactly which budgets fall off — funded top-to-bottom in row order.
func annualGridScenario(grid budgeting.AnnualGrid, deltaMinor int64) (map[string][12]int64, int) {
	out := map[string][12]int64{}
	count := 0
	for m := 0; m < 12; m++ {
		plans := make([]budgetplan.BudgetPlan, 0, len(grid.Rows))
		for _, r := range grid.Rows {
			plans = append(plans, budgetplan.BudgetPlan{BudgetID: r.BudgetID, Name: r.Name, PlanMinor: r.Cells[m].Plan.Amount})
		}
		res := budgetplan.Evaluate(budgetplan.ScenarioInput{
			IncomeMinor:      grid.MonthPlanTotals[m].Amount,
			IncomeDeltaMinor: deltaMinor,
			Plans:            plans,
		})
		for _, f := range res.Funded {
			if f.ShortfallMinor > 0 {
				row := out[f.BudgetID]
				row[m] = f.ShortfallMinor
				out[f.BudgetID] = row
				count++
			}
		}
	}
	return out, count
}

// annualGridScenarioBar renders the ephemeral income-scenario control (C393): a
// toggle, and when on a ±$100 income stepper, a reset, and a live underfunded
// count. All handlers are stable hooks passed in from the parent.
func annualGridScenarioBar(on bool, deltaMinor int64, underCount int, cur string, toggle, less, more, reset ui.Handler) ui.Node {
	cls := "budget-annualgrid-scenario"
	if on {
		cls += " is-on"
	}
	children := []ui.Node{
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-toggle"),
			Attr("aria-pressed", ariaBool(on)), Attr("aria-label", uistate.T("budgets.scenarioToggleAria")),
			Attr("title", uistate.T("budgets.scenarioHint")), OnClick(toggle),
			Span(uistate.T("budgets.scenarioToggle"))),
	}
	if on {
		var status ui.Node
		if underCount > 0 {
			status = Span(css.Class("budget-annualgrid-scenario-status", "is-under"),
				uistate.T("budgets.scenarioUnderfunded", underCount))
		} else {
			status = Span(css.Class("budget-annualgrid-scenario-status", "is-clear"),
				uistate.T("budgets.scenarioAllFunded"))
		}
		children = append(children,
			Span(css.Class("budget-annualgrid-scenario-label"), uistate.T("budgets.scenarioLabel")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-less"),
				Attr("aria-label", uistate.T("budgets.scenarioLess")), OnClick(less), Span(Attr("aria-hidden", "true"), "−")),
			Span(css.Class("budget-annualgrid-scenario-delta"), Attr("data-testid", "budget-annualgrid-scenario-delta"),
				signedMoney(money.New(deltaMinor, cur), currency.Decimals(cur))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-more"),
				Attr("aria-label", uistate.T("budgets.scenarioMore")), OnClick(more), Span(Attr("aria-hidden", "true"), "+")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-reset"),
				Attr("aria-label", uistate.T("budgets.scenarioReset")), OnClick(reset), Span(uistate.T("budgets.scenarioReset"))),
			status,
		)
	}
	return Div(ClassStr(cls), children)
}

// annualGridLegend renders the plan/actual/projected (and, in scenario mode,
// underfunded) key so the three cell states are self-explaining (C394).
func annualGridLegend(hasFuture, scenarioOn bool) ui.Node {
	items := []ui.Node{
		annualGridLegendItem("is-actual", uistate.T("budgets.gridLegendActual")),
		annualGridLegendItem("is-planned", uistate.T("budgets.gridLegendPlanned")),
	}
	if hasFuture {
		items = append(items, annualGridLegendItem("is-projected", uistate.T("budgets.gridLegendProjected")))
	}
	if scenarioOn {
		items = append(items, annualGridLegendItem("is-under", uistate.T("budgets.scenarioLegend")))
	}
	return Div(css.Class("budget-annualgrid-legend"), items)
}

// annualGridLegendItem is one swatch + label (no hooks, safe at any position).
func annualGridLegendItem(swatchMod, label string) ui.Node {
	return Span(css.Class("budget-annualgrid-legend-item"),
		Span(ClassStr("budget-annualgrid-swatch "+swatchMod), Attr("aria-hidden", "true")),
		Span(label))
}

// gridColClass returns the modifier classes for a data/header column at month index i:
// the current-month accent band, the future-month wash, and the leading-divider Total
// column. Empty for a plain past month column. (i == 12 is the Total column.)
func gridColClass(i, current, fromMonth int) string {
	switch {
	case i == 12:
		return " is-total"
	case i == current:
		return " is-current"
	case i >= fromMonth:
		return " is-future"
	default:
		return ""
	}
}

// annualGridTable renders the matrix itself. A sticky header row and sticky first
// column keep the month labels and budget names visible while scrolling; all styling
// is class-driven (see rules_annualgrid.go / rules_annualgridplan.go), so cells read
// as one designed grid.
func annualGridTable(grid budgeting.AnnualGrid, props budgetAnnualGridProps, fromMonth int, perBudget map[string]budgetplan.MonthAmounts, shortfalls map[string][12]int64) ui.Node {
	// Header row: "Budget", the twelve months (current banded, future toned), "Total".
	headCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner"), Attr("scope", "col"),
			uistate.T("budgets.annualGridBudgetCol")),
	}
	for i, name := range annualGridMonths {
		th := Th(ClassStr("budget-annualgrid-th"+gridColClass(i, grid.CurrentMonth, fromMonth)), Attr("scope", "col"), name)
		if i == grid.CurrentMonth {
			// id anchors the scroll-into-view (C371).
			th = Th(ClassStr("budget-annualgrid-th"+gridColClass(i, grid.CurrentMonth, fromMonth)),
				Attr("scope", "col"), Attr("id", "budget-annualgrid-current"), name)
		}
		headCells = append(headCells, th)
	}
	headCells = append(headCells, Th(css.Class("budget-annualgrid-th", "is-total"), Attr("scope", "col"),
		uistate.T("budgets.annualGridTotalCol")))

	// Map each budget to its tracked categories so a cell click can filter Transactions.
	catsByBudget := map[string][]string{}
	for _, b := range props.Budgets {
		catsByBudget[b.ID] = b.TrackedCategoryIDs()
	}
	var bodyRows []ui.Node
	var projColTotals [12]int64
	for _, row := range grid.Rows {
		proj := perBudget[row.BudgetID]
		for m := 0; m < 12; m++ {
			projColTotals[m] += proj[m]
		}
		bodyRows = append(bodyRows, ui.CreateElement(annualGridRow, annualGridRowProps{
			Row: row, CategoryIDs: catsByBudget[row.BudgetID], CurrentMonth: grid.CurrentMonth,
			FromMonth: fromMonth, Year: grid.Year, Base: grid.Currency,
			Projected: proj, Shortfall: shortfalls[row.BudgetID], OnCell: props.OnCell,
		}))
	}

	// Footer: column totals across all budgets + the grand total. Future months show
	// the projected column total (toned) since no actuals exist yet.
	footCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner", "is-foot"), Attr("scope", "row"),
			uistate.T("budgets.annualGridTotalCol")),
	}
	for i := 0; i < 12; i++ {
		cls := "budget-annualgrid-td is-foot" + gridColClass(i, grid.CurrentMonth, fromMonth)
		if i >= fromMonth && projColTotals[i] > 0 {
			footCells = append(footCells, Td(ClassStr(cls),
				Span(css.Class("budget-annualgrid-projected"), fmtMoney(money.New(projColTotals[i], grid.Currency)))))
			continue
		}
		footCells = append(footCells, Td(ClassStr(cls), fmtMoney(grid.MonthActualTotals[i])))
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
	FromMonth    int
	Year         int
	Base         string
	Projected    budgetplan.MonthAmounts // per-month projected minor units (future only)
	Shortfall    [12]int64               // per-month scenario shortfall (0 = funded)
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
			Current: m == props.CurrentMonth, IsFuture: m >= props.FromMonth,
			Projected: props.Projected[m], Shortfall: props.Shortfall[m], Base: props.Base,
			From: from, To: to, OnCell: props.OnCell,
		}))
	}
	cells = append(cells, Td(css.Class("budget-annualgrid-td", "is-total"), fmtMoney(row.ActualTotal)))
	return Tr(css.Class("budget-annualgrid-tr"), cells)
}

// annualGridCellProps is one plan-vs-actual (or plan-vs-projected) cell.
type annualGridCellProps struct {
	BudgetID    string
	CategoryIDs []string
	Cell        budgeting.AnnualGridCell
	Current     bool  // in the current-month column (accent band)
	IsFuture    bool  // a future month (projected, distinct wash)
	Projected   int64 // projected minor units for a future cell (grid currency)
	Shortfall   int64 // scenario shortfall minor units (0 = funded)
	Base        string
	From, To    string
	OnCell      func(categoryIDs []string, from, to string)
}

// annualGridCell renders one cell as a button that drills to the month's filtered
// transactions. Its own component so the click handler is a stable hook. Past/
// current months show actual-over-plan; future months show the projected figure
// (dotted, toned) over plan; a scenario shortfall reddens the cell and explains
// itself in the title. All coloring is class-driven.
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
	if props.IsFuture {
		cls += " is-future"
	}
	if props.Cell.Over && !props.IsFuture {
		cls += " is-over"
	}
	if props.Shortfall > 0 {
		cls += " is-underfunded"
	}

	// Primary figure: projected for a future cell that has a projection, else actual.
	var primary ui.Node
	title := ""
	if props.IsFuture && props.Projected > 0 {
		primary = Span(css.Class("budget-annualgrid-projected"), fmtMoney(money.New(props.Projected, props.Base)))
		title = uistate.T("budgets.gridProjectedTitle", fmtMoney(money.New(props.Projected, props.Base)))
	} else {
		primary = Span(css.Class("budget-annualgrid-actual"), fmtMoney(props.Cell.Actual))
	}
	if props.Shortfall > 0 {
		title = uistate.T("budgets.scenarioUnderTitle", fmtMoney(money.New(props.Shortfall, props.Base)))
	}

	btn := Button(css.Class("budget-annualgrid-cell"), Type("button"),
		Attr("data-testid", "annualgrid-cell-"+props.BudgetID+"-"+props.From),
		OnClick(click), primary,
		Span(css.Class("budget-annualgrid-plan"), fmtMoney(props.Cell.Plan)),
	)
	if title != "" {
		btn = Button(css.Class("budget-annualgrid-cell"), Type("button"),
			Attr("data-testid", "annualgrid-cell-"+props.BudgetID+"-"+props.From), Attr("title", title),
			OnClick(click), primary,
			Span(css.Class("budget-annualgrid-plan"), fmtMoney(props.Cell.Plan)),
		)
	}
	return Td(ClassStr(cls), btn)
}
