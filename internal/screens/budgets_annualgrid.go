// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetAnnualGridProps feeds the BG9 annual grid: the raw dataset it projects and
// a drill callback that opens Transactions filtered to a clicked cell's budget +
// month. CatName resolves category IDs to display names for the drill. Recurrings
// and Goals feed the future-month projection (C394).
type budgetAnnualGridProps struct {
	Budgets    []domain.Budget
	Txns       budgeting.NettedLedger
	App        *appstate.App
	Scope      string
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

var annualGridMonthKeys = [12]string{
	"budgets.month.jan", "budgets.month.feb", "budgets.month.mar", "budgets.month.apr",
	"budgets.month.may", "budgets.month.jun", "budgets.month.jul", "budgets.month.aug",
	"budgets.month.sep", "budgets.month.oct", "budgets.month.nov", "budgets.month.dec",
}

var annualGridMonthFullKeys = [12]string{
	"budgets.month.january", "budgets.month.february", "budgets.month.march", "budgets.month.april",
	"budgets.month.mayFull", "budgets.month.june", "budgets.month.july", "budgets.month.august",
	"budgets.month.september", "budgets.month.october", "budgets.month.november", "budgets.month.december",
}

// scenarioStepMinor is one press of the income scenario stepper: ±$100 a month.
const scenarioStepMinor = budgetview.ScenarioStepMinor

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
	monthWin := ui.UseState(annualWinAll)
	revision := uistate.UseDataRevision().Get()
	curMonth := budgetview.CurrentMonth(props.Now, year.Get())
	ui.UseEffect(func() func() {
		if open.Get() && curMonth >= 0 {
			scrollAnnualGridToCurrent()
		}
		return nil
	}, open.Get(), year.Get(), curMonth)
	deps := []any{revision, props.App, props.Scope, props.WeekStart, props.Now.Format("2006-01-02"), year.Get(), open.Get()}
	if props.App != nil {
		deps = append(deps, props.App.Rev())
	}
	// GWC compares slices by base pointer. Length makes a reslice an invalidation.
	deps = append(deps, props.Budgets, len(props.Budgets), props.Txns, len(props.Txns), props.Cats, len(props.Cats), props.Rates)
	build := ui.UseMemo(func() *budgetview.AnnualBuild {
		if !open.Get() {
			return &budgetview.AnnualBuild{}
		}
		value := budgetview.BuildAnnual(props.Budgets, props.Txns, props.Cats, year.Get(), props.Rates, props.WeekStart, props.Now)
		return &value
	}, deps...)
	projection := useBudgetAnnualProjection(props, build, open.Get(), revision)
	header := ui.CreateElement(annualGridHeader, annualGridHeaderProps{Open: open.Get(), Year: year.Get(), Budgets: props.Budgets, Rates: props.Rates, Toggle: func() { open.Set(!open.Get()) }})
	if !open.Get() {
		return Div(css.Class("budget-annualgrid"), header)
	}
	if build.Err != nil {
		return Div(css.Class("budget-annualgrid"), header, Span(css.Class("budget-sub", tw.TextDown), Attr("role", "alert"), uistate.T("budgets.annualGridError")))
	}
	if projection.Err != nil {
		return Div(css.Class("budget-annualgrid"), header, Span(css.Class("budget-sub", tw.TextDown), Attr("role", "alert"), uistate.T("budgets.annualGridError")))
	}
	grid := build.Grid
	var shortfalls map[string][12]int64
	underCount := 0
	if scenarioOn.Get() {
		shortfalls, underCount = budgetview.AnnualScenario(grid, incomeDelta.Get())
	}
	return Div(css.Class("budget-annualgrid"), header,
		Div(css.Class("budget-annualgrid-head"),
			ui.CreateElement(annualGridYearControls, annualGridYearControlsProps{Year: grid.Year, Previous: func() { year.Set(budgetview.StepYear(year.Get(), -1)) }, Next: func() { year.Set(budgetview.StepYear(year.Get(), 1)) }}),
			ui.CreateElement(annualGridScenarioControls, annualGridScenarioControlsProps{
				On: scenarioOn.Get(), Delta: incomeDelta.Get(), UnderCount: underCount, Currency: grid.Currency,
				Toggle: func() { scenarioOn.Set(!scenarioOn.Get()) }, Less: func() { incomeDelta.Set(budgetview.StepScenario(incomeDelta.Get(), -scenarioStepMinor)) },
				More: func() { incomeDelta.Set(budgetview.StepScenario(incomeDelta.Get(), scenarioStepMinor)) }, Reset: func() { incomeDelta.Set(0) },
			})),
		annualGridLegend(projection.FromMonth <= 11, scenarioOn.Get()),
		ui.CreateElement(annualGridWindowControls, annualGridWindowControlsProps{Window: monthWin.Get(), Select: monthWin.Set}),
		Div(css.Class("budget-annualgrid-scroll"), annualGridTable(grid, props, projection.FromMonth, projection.PerBudget, projection.Totals, shortfalls, monthWin.Get())),
	)
}

type annualGridHeaderProps struct {
	Open    bool
	Year    int
	Budgets []domain.Budget
	Rates   currency.Rates
	Toggle  func()
}

func annualGridHeader(props annualGridHeaderProps) ui.Node {
	caretCls, toggleKey := budgetview.AnnualDisclosure(props.Open)
	toggleAria := uistate.T(toggleKey)
	return Div(css.Class("budget-annualgrid-head"),
		Button(css.Class("budget-annualgrid-toggle"), Type("button"), Attr("data-testid", "budget-annualgrid-toggle"),
			Attr("aria-expanded", ariaBool(props.Open)), Attr("aria-label", toggleAria), OnClick(Prevent(props.Toggle)),
			Span(ClassStr(caretCls), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-annualgrid-toggle-label"), uistate.T("budgets.planYearTitle")),
			// Glanceable while closed: the year, and what the current plan comes to
			// across it. "See the whole year and plan ahead" described the SECTION;
			// a collapsed row should answer something instead of advertising itself
			// (Cam, 2026-08-31). The figure is the budgets' own limits normalized to
			// a year — no grid build, so it costs nothing while shut.
			Span(css.Class("budget-annualgrid-toggle-hint"),
				uistate.T("budgets.planYearStableHint", props.Year,
					annualPlanTotalLabel(props.Budgets, props.Rates))),
		),
	)

}

type annualGridYearControlsProps struct {
	Year           int
	Previous, Next func()
}

func annualGridYearControls(props annualGridYearControlsProps) ui.Node {
	return Div(css.Class("budget-annualgrid-year"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-prev"),
			Attr("aria-label", uistate.T("budgets.annualGridPrevYear")), OnClick(Prevent(props.Previous)),
			uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Span(css.Class("budget-annualgrid-yearlabel"), strconv.Itoa(props.Year)),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-next"),
			Attr("aria-label", uistate.T("budgets.annualGridNextYear")), OnClick(Prevent(props.Next)),
			uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
	)

}

type annualGridScenarioControlsProps struct {
	On                        bool
	Delta                     int64
	UnderCount                int
	Currency                  string
	Toggle, Less, More, Reset func()
}

func annualGridScenarioControls(props annualGridScenarioControlsProps) ui.Node {
	toggle := ui.UseEvent(Prevent(props.Toggle))
	less := ui.UseEvent(Prevent(props.Less))
	more := ui.UseEvent(Prevent(props.More))
	reset := ui.UseEvent(Prevent(props.Reset))
	return annualGridScenarioBar(props.On, props.Delta, props.UnderCount, props.Currency, toggle, less, more, reset)
}

type annualGridWindowControlsProps struct {
	Window string
	Select func(string)
}

func annualGridWindowControls(props annualGridWindowControlsProps) ui.Node {
	all := ui.UseEvent(Prevent(func() { props.Select(annualWinAll) }))
	h1 := ui.UseEvent(Prevent(func() { props.Select(annualWinH1) }))
	h2 := ui.UseEvent(Prevent(func() { props.Select(annualWinH2) }))
	return annualGridWindowBar(props.Window, all, h1, h2)
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

// annualGridScenarioBar renders the ephemeral income-scenario control (C393): a
// toggle, and when on a ±$100 income stepper, a reset, and a live underfunded
// count. All handlers are stable hooks passed in from the parent.
func annualGridScenarioBar(on bool, deltaMinor int64, underCount int, cur string, toggle, less, more, reset ui.Handler) ui.Node {
	toggleOnClick := OnClick(toggle)
	lessOnClick := OnClick(less)
	moreOnClick := OnClick(more)
	resetOnClick := OnClick(reset)
	presentation := budgetview.PresentAnnualScenario(on, deltaMinor, underCount, cur, uistate.T)
	children := []ui.Node{
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-toggle"),
			Attr("aria-pressed", ariaBool(on)), Attr("aria-label", uistate.T("budgets.scenarioToggleAria")),
			Attr("title", uistate.T("budgets.scenarioHint")), toggleOnClick,
			Span(uistate.T("budgets.scenarioToggle"))),
	}
	if on {
		status := Span(ClassStr(presentation.StatusClass), presentation.Status)
		children = append(children,
			Span(css.Class("budget-annualgrid-scenario-label"), uistate.T("budgets.scenarioLabel")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-less"),
				Attr("aria-label", uistate.T("budgets.scenarioLess")), lessOnClick, Span(Attr("aria-hidden", "true"), "−")),
			Span(css.Class("budget-annualgrid-scenario-delta"), Attr("data-testid", "budget-annualgrid-scenario-delta"),
				presentation.Delta),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-more"),
				Attr("aria-label", uistate.T("budgets.scenarioMore")), moreOnClick, Span(Attr("aria-hidden", "true"), "+")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-annualgrid-scenario-reset"),
				Attr("aria-label", uistate.T("budgets.scenarioReset")), resetOnClick, Span(uistate.T("budgets.scenarioReset"))),
			status,
		)
	}
	return Div(ClassStr(presentation.Class), children)
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
	return budgetview.GridColClass(i, current, fromMonth)
}

// annualGridTable renders the matrix itself. A sticky header row and sticky first
// column keep the month labels and budget names visible while scrolling; all styling
// is class-driven (see rules_annualgrid.go / rules_annualgridplan.go), so cells read
// as one designed grid.
func annualGridTable(grid budgeting.AnnualGrid, props budgetAnnualGridProps, fromMonth int, perBudget map[string]budgetplan.MonthAmounts, projectedTotals budgetplan.MonthAmounts, shortfalls map[string][12]int64, win string) ui.Node {
	firstM, lastM := annualWindowRange(win)
	// Header row: "Budget", the twelve months (current banded, future toned), "Total".
	headCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner"), Attr("scope", "col"),
			uistate.T("budgets.annualGridBudgetCol")),
	}
	for i, monthKey := range annualGridMonthKeys {
		if i < firstM || i > lastM {
			continue
		}
		// C608: the visible abbreviation is fine for a sighted reader scanning a
		// row of columns; a screen reader needs the month and year, because a
		// cell's own label references it.
		full := annualMonthLabel(i, grid.Year)
		th := Th(ClassStr("budget-annualgrid-th"+gridColClass(i, grid.CurrentMonth, fromMonth)),
			Attr("scope", "col"), Attr("aria-label", full), Attr("title", full), uistate.T(monthKey))
		if i == grid.CurrentMonth {
			// id anchors the scroll-into-view (C371).
			th = Th(ClassStr("budget-annualgrid-th"+gridColClass(i, grid.CurrentMonth, fromMonth)),
				Attr("scope", "col"), Attr("id", "budget-annualgrid-current"),
				Attr("aria-label", full), Attr("title", full), uistate.T(monthKey))
		}
		headCells = append(headCells, th)
	}
	headCells = append(headCells, Th(css.Class("budget-annualgrid-th", "is-total"), Attr("scope", "col"),
		uistate.T("budgets.annualGridTotalCol")))

	// Map each budget to its tracked categories so a cell click can filter Transactions.
	catsByBudget := budgetview.BudgetCategories(props.Budgets)
	var bodyRows []ui.Node
	for _, row := range grid.Rows {
		proj := perBudget[row.BudgetID]
		bodyRows = append(bodyRows, ui.CreateElement(annualGridRow, annualGridRowProps{
			Row: row, CategoryIDs: catsByBudget[row.BudgetID], CurrentMonth: grid.CurrentMonth,
			FromMonth: fromMonth, Year: grid.Year, Base: grid.Currency,
			Projected: proj, Shortfall: shortfalls[row.BudgetID], OnCell: props.OnCell,
			FirstMonth: firstM, LastMonth: lastM,
		}))
	}

	// Footer: column totals across all budgets + the grand total. Future months show
	// the projected column total (toned) since no actuals exist yet.
	footCells := []ui.Node{
		Th(css.Class("budget-annualgrid-corner", "is-foot"), Attr("scope", "row"),
			uistate.T("budgets.annualGridTotalCol")),
	}
	for i := 0; i < 12; i++ {
		if i < firstM || i > lastM {
			continue
		}
		cell := budgetview.PresentAnnualFooter(grid, projectedTotals, i, fromMonth)
		if cell.Projected {
			footCells = append(footCells, Td(ClassStr(cell.Class), Span(css.Class("budget-annualgrid-projected"), cell.Amount)))
			continue
		}
		footCells = append(footCells, Td(ClassStr(cell.Class), cell.Amount))
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
	// FirstMonth / LastMonth bound the visible month window (C608).
	FirstMonth, LastMonth int
	Row                   budgeting.AnnualGridRow
	CategoryIDs           []string
	CurrentMonth          int
	FromMonth             int
	Year                  int
	Base                  string
	Projected             budgetplan.MonthAmounts // per-month projected minor units (future only)
	Shortfall             [12]int64               // per-month scenario shortfall (0 = funded)
	OnCell                func(categoryIDs []string, from, to string)
}

// annualGridRow is a per-row component so each cell's click handler lives at a
// stable hook position (never a raw On* inside a variable-length loop).
func annualGridRow(props annualGridRowProps) ui.Node {
	row := props.Row
	if row.Error != nil {
		columnSpan := budgetview.ErrorColumnSpan(props.FirstMonth, props.LastMonth)
		return Tr(css.Class("budget-annualgrid-tr"),
			Th(css.Class("budget-annualgrid-rowhead"), Attr("scope", "row"), row.Name),
			Td(Attr("colspan", strconv.Itoa(columnSpan)), Attr("role", "alert"),
				Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.annualGridError"))))
	}
	cells := []ui.Node{
		Th(css.Class("budget-annualgrid-rowhead"), Attr("scope", "row"), row.Name),
	}
	for m := 0; m < 12; m++ {
		if m < props.FirstMonth || m > props.LastMonth {
			continue
		}
		cell := row.Cells[m]
		from, to := budgetview.MonthDates(props.Year, m)
		cells = append(cells, ui.CreateElement(annualGridCell, annualGridCellProps{
			BudgetID: row.BudgetID, CategoryIDs: props.CategoryIDs, Cell: cell,
			Current: m == props.CurrentMonth, IsFuture: m >= props.FromMonth,
			Projected: props.Projected[m], Shortfall: props.Shortfall[m], Base: props.Base,
			From: from, To: to, OnCell: props.OnCell,
			// C608: a cell's accessible name has to carry its own coordinates. As
			// two bare figures it read "$1,100.00 $1,300.00" — no month, no budget,
			// and no way to tell which number was which.
			BudgetName: row.Name, MonthLabel: annualMonthLabel(m, props.Year),
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
	// BudgetName / MonthLabel name the cell's coordinates for its accessible
	// label — without them the button announced two amounts and nothing else
	// (C608).
	BudgetName string
	MonthLabel string
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
	presentation := budgetview.PresentAnnualCell(props.Cell, props.Current, props.IsFuture, props.Projected, props.Shortfall, props.Base, props.BudgetName, props.MonthLabel, uistate.T)
	primary := Span(ClassStr(presentation.PrimaryClass), presentation.Primary)
	btn := Button(css.Class("budget-annualgrid-cell"), Type("button"),
		Attr("data-testid", "annualgrid-cell-"+props.BudgetID+"-"+props.From),
		Attr("aria-label", presentation.Aria), Attr("title", presentation.Title),
		OnClick(click), primary,
		Span(css.Class("budget-annualgrid-plan"), fmtMoney(props.Cell.Plan)),
	)
	return Td(ClassStr(presentation.Class), btn)
}

// C608 — the year planner's month window.
//
// Twelve columns do not fit a narrow pane, and the grid's only answer was a
// sighted-only "Scroll sideways for the full year →" pointing at a scrollbar
// under a tall table. Halving the year is a control: reachable by keyboard,
// announced to a screen reader, and it removes the horizontal scroll entirely
// rather than signposting it.
const (
	annualWinAll = "all"
	annualWinH1  = "h1"
	annualWinH2  = "h2"
)

// annualWindowRange turns a window into inclusive month indices.
func annualWindowRange(win string) (first, last int) {
	return budgetview.WindowRange(win)
}

// annualMonthLabel is a month's full name and year — what a cell's accessible
// name references, and what a column header announces.
func annualMonthLabel(m, year int) string {
	return budgetview.MonthLabel(m, year, uistate.T)
}

// annualGridWindowBar renders the month-window control. Handlers arrive from the
// parent so they sit at stable hook positions.
func annualGridWindowBar(win string, all, h1, h2 ui.Handler) ui.Node {
	opt := func(value, labelKey, testID string, on ui.Handler) ui.Node {
		cls := budgetview.AnnualWindowClass(win, value)
		return Button(ClassStr(cls), Type("button"), Attr("data-testid", testID),
			Attr("aria-pressed", ariaBool(win == value)), OnClick(on), uistate.T(labelKey))
	}
	return Div(css.Class("budget-annualgrid-winbar"), Attr("data-testid", "annualgrid-window"),
		Attr("role", "group"), Attr("aria-label", uistate.T("budgets.gridWindowLabel")),
		Span(css.Class("budget-annualgrid-winlabel", tw.TextFaint), uistate.T("budgets.gridWindowLabel")),
		opt(annualWinAll, "budgets.gridWindowAll", "annualgrid-win-all", all),
		opt(annualWinH1, "budgets.gridWindowH1", "annualgrid-win-h1", h1),
		opt(annualWinH2, "budgets.gridWindowH2", "annualgrid-win-h2", h2),
	)
}

// annualPlanTotal is the household's planned spend for a full year: every budget's
// limit scaled by how many of its periods fit in one. Budgets can run weekly,
// biweekly, monthly, quarterly or yearly, so a bare sum of limits would compare a
// weekly grocery cap with a yearly insurance one as though they were equal.
func annualPlanTotal(budgets []domain.Budget, base string, rateTables ...currency.Rates) int64 {
	return budgetview.AnnualPlanTotal(budgets, base, rateTables...)
}

func annualPlanTotalLabel(budgets []domain.Budget, rates currency.Rates) string {
	return budgetview.AnnualPlanLabel(budgets, rates, uistate.T)
}
