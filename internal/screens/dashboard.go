//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Dashboard shows headline metrics in the candidate-C bento grid, driven by the
// live store and the shared time-resolution window.
func Dashboard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Div(Class("bento"), Div(Class("w"), Div(Class("wbody"), P(Class("empty"), "App state is not ready yet."))))
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	net, assets, liabilities, _ := ledger.NetWorth(accounts, txns, rates)
	w := uistate.UsePeriod().Get()
	start, end := w.Range()
	income, expense, _ := ledger.PeriodTotals(txns, start, end, rates)
	periodLabel := w.FromLabel()
	if w.ToLabel() != w.FromLabel() {
		periodLabel += " – " + w.ToLabel()
	}

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	return Div(Class("bento"),
		dashboardHeaderCell(),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-networth", Title: "Net worth", Draggable: true, Resizable: true,
			GridColumn: "1", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(net), figTone(net), fmt.Sprintf("Assets %s", fmtAccounting(assets)), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-income", Title: "Income", Draggable: true,
			GridColumn: "2", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(income), "text-up", periodLabel, "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-spending", Title: "Spending", Draggable: true,
			GridColumn: "3", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(expense), "text-down", periodLabel, "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-liabilities", Title: "Liabilities", Draggable: true,
			GridColumn: "4", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(liabilities), "", fmt.Sprintf("%d accounts", active), "text-dim"),
		}),
		recentWidget(txns),
		budgetsWidget(app, txns, rates),
		goalsWidget(app),
		todoWidget(app),
	)
}

// todoWidget is the 1×1 To-do widget: up to three open tasks, dot-toned by
// priority (high = amber, others = dim/faint).
func todoWidget(app *appstate.App) ui.Node {
	var open []domain.Task
	for _, t := range app.Tasks() {
		if t.Status == domain.StatusOpen {
			open = append(open, t)
		}
	}
	var body ui.Node
	if len(open) == 0 {
		body = P(Class("empty text-dim text-[13px]"), "Nothing to do — nice.")
	} else {
		if len(open) > 3 {
			open = open[:3]
		}
		rows := make([]ui.Node, 0, len(open))
		for _, t := range open {
			dotTone, dot := "text-faint", "○"
			switch t.Priority {
			case domain.PriorityHigh:
				dotTone, dot = "text-warn", "●"
			case domain.PriorityMedium:
				dotTone, dot = "text-dim", "●"
			}
			rows = append(rows, Div(Class("flex gap-2"),
				Span(Class(dotTone), dot),
				Span(t.Title),
			))
		}
		body = Div(Class("text-[13px] space-y-2"), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo", Title: "To-do", Draggable: true, GridColumn: "2", GridRow: "5",
		Body: body,
	})
}

// goalsWidget is the 1×1 Goals widget: the first goal's progress (% + saved /
// target) via internal/goals.
func goalsWidget(app *appstate.App) ui.Node {
	list := app.Goals()
	if len(list) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goals", Title: "Goals", Draggable: true, GridColumn: "1", GridRow: "5",
			Body: P(Class("empty text-dim text-[13px]"), "No goals yet."),
		})
	}
	g := list[0]
	pct := goals.Percent(g)
	caption := fmt.Sprintf("%d%%", pct)
	if !g.TargetDate.IsZero() {
		caption += " · by " + g.TargetDate.Format("Jan 2")
	}
	body := Div(
		Div(Class("flex justify-between text-[13px]"),
			Span(Class("text-dim"), "saved"),
			Span(Class("font-display fig"), fmtAccounting(g.CurrentAmount)+" / "+fmtAccounting(g.TargetAmount)),
		),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: "bg-fg", Class: "mt-2"}),
		Div(Class("text-[12px] text-dim mt-1.5"), caption),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goals", Title: "Goal · " + g.Name, Draggable: true, GridColumn: "1", GridRow: "5",
		Body: body,
	})
}

// budgetsWidget is the 1×2 Budgets widget: current-month spend vs limit per
// budget with an ok/near/over progress bar (via internal/budgeting). Budgets are
// monthly, so it always evaluates the current month regardless of the dashboard
// window.
func budgetsWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates) ui.Node {
	budgets := app.Budgets()
	start, end := dateutil.MonthRange(time.Now())
	statuses, _ := budgeting.EvaluateAll(budgets, txns, start, end, rates, budgeting.DefaultNearThreshold)

	catName := make(map[string]string)
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	var body ui.Node
	if len(statuses) == 0 {
		body = P(Class("empty text-dim text-[13px]"), "No budgets yet.")
	} else {
		rows := make([]ui.Node, 0, len(statuses))
		for _, s := range statuses {
			tone, bar := "text-dim", "bg-up"
			switch s.State {
			case budgeting.StateNear:
				tone, bar = "text-warn", "bg-warn"
			case budgeting.StateOver:
				tone, bar = "text-down", "bg-down"
			}
			label := s.Budget.Name
			if label == "" {
				label = catName[s.Budget.CategoryID]
			}
			rows = append(rows, Div(
				Div(Class("flex justify-between"),
					Span(label),
					Span(Class("font-display fig "+tone), fmt.Sprintf("%d%%", s.Percent)),
				),
				uiw.ProgressBar(uiw.ProgressBarProps{Percent: s.Percent, Tone: bar, Class: "mt-1.5"}),
			))
		}
		body = Div(Class("space-y-4 text-[13px]"), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budgets", Title: "Budgets", Draggable: true,
		GridColumn: "3", GridRow: "3 / span 2", Body: body,
	})
}

// recentWidget is the 2×2 Recent transactions widget: newest activity as a
// compact table with accounting amounts. Display-only, so rows build in a loop.
func recentWidget(txns []domain.Transaction) ui.Node {
	recent := recentTransactions(txns, 6)
	var body ui.Node
	if len(recent) == 0 {
		body = P(Class("empty text-dim text-[13px]"), "No transactions yet.")
	} else {
		rows := make([]ui.Node, 0, len(recent))
		for _, t := range recent {
			rows = append(rows, Tr(Class("border-b border-line/70"),
				Td(Class("py-2.5 fig text-dim w-16"), t.Date.Format("Jan 2")),
				Td(Class("py-2.5"), t.Desc),
				Td(Class("py-2.5 text-right font-display fig "+figTone(t.Amount)), fmtAccounting(t.Amount)),
			))
		}
		body = Table(Class("w-full text-[13px]"), Tbody(rows))
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "recent", Title: "Recent transactions", Draggable: true,
		GridColumn: "1 / span 2", GridRow: "3 / span 2", BodyClass: "overflow-hidden",
		Body: body,
	})
}

// dashboardHeaderCell is the full-width intro cell at the top of the bento grid.
func dashboardHeaderCell() ui.Node {
	return Div(Class("w"), Style(map[string]string{"grid-column": "1 / -1", "grid-row": "1"}),
		Div(Class("flex-1 flex items-center px-5"),
			Div(
				H1(Class("font-display text-2xl font-semibold tracking-tight"), "Your dashboard"),
				P(Class("text-dim mt-0.5 text-[13px]"), "Drag tiles to move · grab the edge handles to resize"),
			),
		),
	)
}

// kpiBody renders a KPI tile's body: a large accounting figure with a small
// subline. figTone/subTone are color classes (e.g. "text-up", "text-dim").
func kpiBody(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(Class("font-display fig text-[24px] leading-tight "+figTone), figure),
		Div(Class("pt-1.5 text-[12px] "+subTone), subline),
	)
}

// recentTransactions returns the n most recent transactions, newest first.
func recentTransactions(txns []domain.Transaction, n int) []domain.Transaction {
	cp := append([]domain.Transaction(nil), txns...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Date.After(cp[j].Date) })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
