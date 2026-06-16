//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
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
	_ = uistate.UseDataRevision().Get() // re-render after import / load-sample / wipe

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

	incCount, expCount := 0, 0
	for _, t := range txns {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			incCount++
		case t.IsExpense():
			expCount++
		}
	}

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	// Net-worth change since the start of this month (end of last month).
	nwSub, nwTone := fmt.Sprintf("Assets %s", fmtAccounting(assets)), "text-dim"
	if prev, _ := ledger.NetWorthSeries(accounts, txns, []time.Time{dateutil.MonthStart(time.Now())}, rates); len(prev) == 1 && prev[0].Amount != 0 {
		d := (net.Amount - prev[0].Amount) * 100 / prev[0].Amount
		if d < 0 {
			nwTone, nwSub = "text-down", fmt.Sprintf("▼ %d%% this month", -d)
		} else {
			nwTone, nwSub = "text-up", fmt.Sprintf("▲ %d%% this month", d)
		}
	}

	return Div(Class("bento"),
		dashboardHeaderCell(),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-networth", Title: "Net worth", Draggable: true, Resizable: true,
			GridColumn: "1", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(net), figTone(net), nwSub, nwTone),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-income", Title: "Income", Draggable: true, Resizable: true,
			GridColumn: "2", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(income), "text-up", periodLabel+" · "+plural(incCount, "deposit"), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-spending", Title: "Spending", Draggable: true, Resizable: true,
			GridColumn: "3", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(expense), "text-down", periodLabel+" · "+plural(expCount, "transaction"), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-liabilities", Title: "Liabilities", Draggable: true, Resizable: true,
			GridColumn: "4", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(liabilities), "", fmt.Sprintf("%d accounts", active), "text-dim"),
		}),
		recentWidget(txns),
		budgetsWidget(app, txns, rates),
		goalsWidget(app),
		todoWidget(app),
		accountsWidget(app, txns),
		netWorthTrendWidget(accounts, txns, rates, net),
		cashFlowWidget(txns, rates),
		savingsRateWidget(income, expense),
		spendingBreakdownWidget(app, txns, rates, start, end),
		upcomingBillsWidget(app),
		freshnessWidget(accounts, app.FreshnessWindows()),
	)
}

// freshnessWidget is the full-width Freshness nudge: a friendly reminder of which
// account balances look stale (via internal/freshness), with how long since each
// was last updated.
func freshnessWidget(accounts []domain.Account, windows freshness.Windows) ui.Node {
	now := time.Now()
	stale := freshness.StaleAccounts(accounts, windows, now)
	var body ui.Node
	if len(stale) == 0 {
		body = P(Class("text-up text-[13px]"), "Everything's up to date — nice work.")
	} else {
		chips := make([]ui.Node, 0, len(stale))
		for _, a := range stale {
			chips = append(chips, Span(Class("member-chip"),
				Span(a.Name),
				Span(Class("text-warn fig"), fmt.Sprintf("· %dd", freshness.DaysSinceUpdate(a, now))),
			))
		}
		body = Div(
			P(Class("text-dim text-[13px] mb-2"), fmt.Sprintf("%d balances could use a refresh.", len(stale))),
			Div(Class("flex flex-wrap gap-2"), chips),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "freshness", Title: "Freshness", Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "8", Body: body,
	})
}

// upcomingBillsWidget is the 2×1 Upcoming bills widget: the next due date and
// minimum payment for each liability account that has them, soonest first.
func upcomingBillsWidget(app *appstate.App) ui.Node {
	now := time.Now()
	type bill struct {
		name   string
		due    time.Time
		amount money.Money
	}
	var bills []bill
	for _, a := range app.Accounts() {
		if a.Archived || a.Class != domain.ClassLiability || a.MinPayment.Amount <= 0 || a.DueDayOfMonth <= 0 {
			continue
		}
		bills = append(bills, bill{name: a.Name, due: nextDue(now, a.DueDayOfMonth), amount: a.MinPayment})
	}
	sort.Slice(bills, func(i, j int) bool { return bills[i].due.Before(bills[j].due) })

	var body ui.Node
	if len(bills) == 0 {
		body = P(Class("empty text-dim text-[13px]"), "No upcoming bills.")
	} else {
		if len(bills) > 4 {
			bills = bills[:4]
		}
		rows := make([]ui.Node, 0, len(bills))
		for _, b := range bills {
			dueTone := "text-faint"
			if dateutil.DaysBetween(now, b.due) <= 7 {
				dueTone = "text-warn"
			}
			rows = append(rows, Div(Class("flex justify-between"),
				Span(b.name),
				Span(Class(dueTone), b.due.Format("Jan 2")),
				Span(Class("font-display fig text-down w-24 text-right"), fmtAccounting(b.amount.Neg())),
			))
		}
		body = Div(Class("text-[13px] space-y-2.5"), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "bills", Title: "Upcoming bills", Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "6",
		Body: body,
	})
}

// nextDue returns the next occurrence of a monthly due-day on or after today
// (the day is clamped to 28 to stay valid in every month).
func nextDue(now time.Time, day int) time.Time {
	if day > 28 {
		day = 28
	}
	y, m, _ := now.Date()
	due := time.Date(y, m, day, 0, 0, 0, 0, now.Location())
	today := time.Date(y, m, now.Day(), 0, 0, 0, 0, now.Location())
	if due.Before(today) {
		due = dateutil.AddMonths(due, 1)
	}
	return due
}

// spendingBreakdownWidget is the 2×1 Spending breakdown widget: a segmented bar
// of the period's expenses by category (top three plus "Other") with a legend.
func spendingBreakdownWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time) ui.Node {
	catName := make(map[string]string)
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	totals := make(map[string]int64)
	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			continue
		}
		amt := conv.Amount
		if amt < 0 {
			amt = -amt
		}
		totals[t.CategoryID] += amt
		total += amt
	}

	if total == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "breakdown", Title: "Spending breakdown", Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
			Body: P(Class("empty text-dim text-[13px]"), "No spending in this period."),
		})
	}

	type seg struct {
		name string
		amt  int64
	}
	segs := make([]seg, 0, len(totals))
	for cid, amt := range totals {
		name := catName[cid]
		if name == "" {
			name = "Uncategorized"
		}
		segs = append(segs, seg{name: name, amt: amt})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].amt > segs[j].amt })

	// Top three categories, the rest lumped into "Other".
	if len(segs) > 4 {
		var other int64
		for _, s := range segs[3:] {
			other += s.amt
		}
		segs = append(segs[:3], seg{name: "Other", amt: other})
	}

	tones := []string{"bg-up", "bg-warn", "bg-dim", "bg-down"}
	barParts := make([]ui.Node, 0, len(segs))
	legend := make([]ui.Node, 0, len(segs))
	for i, s := range segs {
		tone := tones[i%len(tones)]
		pct := int(s.amt * 100 / total)
		barParts = append(barParts, Div(Class(tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		legend = append(legend, Span(Class("flex items-center gap-1.5"),
			Span(Class("w-2 h-2 rounded-full "+tone)),
			Textf("%s %d%%", s.name, pct),
		))
	}

	body := Div(
		Div(Class("h-2.5 rounded-full overflow-hidden flex"), barParts),
		Div(Class("flex flex-wrap gap-x-4 gap-y-1 mt-3 text-[12px] text-dim"), legend),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "breakdown", Title: "Spending breakdown", Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
		Body: body,
	})
}

// savingsRateWidget is the 2×1 Savings rate widget: the share of the period's
// income that wasn't spent, as a big figure and a bar.
func savingsRateWidget(income, expense money.Money) ui.Node {
	pct := 0
	if income.Amount > 0 {
		pct = int((income.Amount - expense.Amount) * 100 / income.Amount)
	}
	tone, bar := "text-up", "bg-up"
	if pct < 0 {
		tone, bar = "text-down", "bg-down"
	}
	body := Div(Class("flex items-center gap-5"),
		Div(
			Div(Class("font-display fig text-[34px] leading-none "+tone), fmt.Sprintf("%d%%", pct)),
			Div(Class("text-[12px] text-dim mt-1"), "of income saved"),
		),
		Div(Class("flex-1"),
			uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: bar}),
			Div(Class("text-[11px] text-faint mt-2"), "this period"),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "savings", Title: "Savings rate", Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "7",
		Body: body,
	})
}

// cashFlowWidget is the 2×1 Cash flow widget: income (up) vs expense (down) bars
// for the last four months, scaled to the largest bar, with the current month's
// net to the right. Totals via ledger.PeriodTotals.
func cashFlowWidget(txns []domain.Transaction, rates currency.Rates) ui.Node {
	type monthBar struct {
		label           string
		income, expense int64
	}
	start := dateutil.MonthStart(time.Now())
	months := make([]monthBar, 0, 4)
	var maxv int64 = 1
	for i := 0; i < 4; i++ {
		ms := dateutil.AddMonths(start, i-3) // three months ago … current
		s, e := dateutil.MonthRange(ms)
		inc, exp, _ := ledger.PeriodTotals(txns, s, e, rates)
		mb := monthBar{label: ms.Format("Jan"), income: inc.Amount, expense: exp.Amount}
		if mb.income > maxv {
			maxv = mb.income
		}
		if mb.expense > maxv {
			maxv = mb.expense
		}
		months = append(months, mb)
	}

	bars := make([]ui.Node, 0, len(months))
	for i, mb := range months {
		labelTone := "text-faint"
		if i == len(months)-1 {
			labelTone = "text-fg"
		}
		bars = append(bars, Div(Class("flex flex-col items-center gap-1.5"),
			Div(Class("flex items-end gap-1 h-14"),
				Div(Class("w-3 bg-up"), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.income*100/maxv))})),
				Div(Class("w-3 bg-down"), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.expense*100/maxv))})),
			),
			Span(Class("text-[11px] "+labelTone), mb.label),
		))
	}

	last := months[len(months)-1]
	netMoney := money.New(last.income-last.expense, rates.Base)
	netTone := "text-up"
	if last.income-last.expense < 0 {
		netTone = "text-down"
	}
	netBlock := Div(Class("ml-auto text-right"),
		Div(Class("text-[11px] text-faint"), "net · "+last.label),
		Div(Class("font-display fig text-lg "+netTone), fmtAccounting(netMoney)),
	)

	return uiw.Widget(uiw.WidgetProps{
		ID: "cashflow", Title: "Cash flow", Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "6",
		Body: Div(Class("flex items-end gap-5"), bars, netBlock),
	})
}

// netWorthTrendWidget is the 1×2 Net worth trend widget: the current figure over
// a six-month end-of-month area chart (via ledger.NetWorthSeries + the chart
// geometry helpers).
func netWorthTrendWidget(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, net money.Money) ui.Node {
	start := dateutil.MonthStart(time.Now())
	cutoffs := make([]time.Time, 0, 6)
	for i := 0; i < 6; i++ {
		cutoffs = append(cutoffs, dateutil.AddMonths(start, i-4)) // end of month M-5 … current month M
	}
	series, _ := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	values := make([]float64, len(series))
	for i, m := range series {
		values[i] = float64(m.Amount)
	}
	body := Div(Class("flex flex-col h-full"),
		Div(Class("font-display fig text-[22px]"), fmtAccounting(net)),
		uiw.AreaChart(uiw.AreaChartProps{Values: values, GradientID: "cf-networth"}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "trend", Title: "Net worth", Draggable: true, Resizable: true, GridColumn: "4", GridRow: "3 / span 2",
		BodyClass: "flex flex-col", Body: body,
	})
}

// accountsWidget is the 2×1 Accounts widget: a small grid of up to six active
// account balances (accounting figures, negatives toned red) via ledger.Balance.
func accountsWidget(app *appstate.App, txns []domain.Transaction) ui.Node {
	cells := make([]ui.Node, 0, 6)
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		bal, _ := ledger.Balance(a, txns)
		tone := ""
		if bal.IsNegative() {
			tone = "text-down"
		}
		cells = append(cells, Div(
			Div(Class("text-dim"), a.Name),
			Div(Class("font-display fig mt-0.5 "+tone), fmtAccounting(bal)),
		))
		if len(cells) >= 6 {
			break
		}
	}
	var body ui.Node
	if len(cells) == 0 {
		body = P(Class("empty text-dim text-[13px]"), "No accounts yet.")
	} else {
		body = Div(Class("grid grid-cols-3 gap-4 text-[13px]"), cells)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "accounts", Title: "Accounts", Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "5",
		Body: body,
	})
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
		ID: "todo", Title: "To-do", Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
		Body: body,
	})
}

// goalsWidget is the 1×1 Goals widget: the first goal's progress (% + saved /
// target) via internal/goals.
func goalsWidget(app *appstate.App) ui.Node {
	list := app.Goals()
	if len(list) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goals", Title: "Goals", Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
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
		ID: "goals", Title: "Goal · " + g.Name, Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
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
		ID: "budgets", Title: "Budgets", Draggable: true, Resizable: true,
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
		ID: "recent", Title: "Recent transactions", Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "3 / span 2", BodyClass: "overflow-hidden",
		Body: body,
	})
}

// dashboardHeaderCell is the full-width intro cell at the top of the bento grid,
// with a Reset layout action that restores the default arrangement.
func dashboardHeaderCell() ui.Node {
	layoutAtom := uistate.UseLayout()
	reset := func() {
		d := dashlayout.Default()
		layoutAtom.Set(d)
		uistate.PersistLayout(d)
	}
	return Div(Class("w"), Style(map[string]string{"grid-column": "1 / -1", "grid-row": "1"}),
		Div(Class("flex-1 flex items-center px-5 gap-3"),
			Div(Class("flex-1"),
				H1(Class("font-display text-2xl font-semibold tracking-tight"), "Your dashboard"),
				P(Class("text-dim mt-0.5 text-[13px]"), "Drag tiles to move · grab the edge handles to resize"),
			),
			Button(Class("data-btn"), Type("button"), OnClick(reset), "Reset layout"),
		),
	)
}

// plural renders a count with a singular/plural noun, e.g. "1 deposit" or
// "3 deposits".
func plural(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", n, singular)
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
