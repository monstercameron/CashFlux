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
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
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
	widgetCfgs := uistate.UseWidgetConfigs().Get()
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

	// "Remind me" on the freshness nudge → create a to-do and jump to the list.
	nav := router.UseNavigate()
	noticeAtom := uistate.UseNotice()
	remindToUpdate := ui.UseEvent(func() {
		if err := app.PutTask(domain.Task{
			ID: id.New(), Title: uistate.T("dashboard.staleTaskTitle"),
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceNudge,
		}); err != nil {
			noticeAtom.Set(noticeAtom.Get().With("Couldn't create the reminder: "+err.Error(), true))
			return
		}
		nav.Navigate("/todo")
	})

	// Net-worth change since the start of this month (end of last month).
	nwSub, nwTone := uistate.T("dashboard.assetsSub", fmtAccounting(assets)), "text-dim"
	if prev, _ := ledger.NetWorthSeries(accounts, txns, []time.Time{dateutil.MonthStart(time.Now())}, rates); len(prev) == 1 {
		if d, ok := ledger.PercentChange(net.Amount, prev[0].Amount); ok {
			if d < 0 {
				nwTone, nwSub = "text-down", fmt.Sprintf("▼ %d%% this month", -d)
			} else {
				nwTone, nwSub = "text-up", fmt.Sprintf("▲ %d%% this month", d)
			}
		}
	}

	return Div(Class("bento"),
		dashboardHeaderCell(),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-networth", Title: uistate.T("dashboard.netWorth"), Draggable: true, Resizable: true,
			GridColumn: "1", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(net), figTone(net), nwSub, nwTone),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-income", Title: uistate.T("dashboard.income"), Draggable: true, Resizable: true,
			GridColumn: "2", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(income), "text-up", periodLabel+" · "+plural(incCount, "deposit"), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-spending", Title: uistate.T("dashboard.spending"), Draggable: true, Resizable: true,
			GridColumn: "3", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(expense), "text-down", periodLabel+" · "+plural(expCount, "transaction"), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-liabilities", Title: uistate.T("dashboard.liabilities"), Draggable: true, Resizable: true,
			GridColumn: "4", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(liabilities), "", uistate.T("dashboard.accountsCount", active), "text-dim"),
		}),
		recentWidget(txns, widgetCfgs.For("recent")),
		budgetsWidget(app, txns, rates),
		goalsWidget(app),
		todoWidget(app),
		accountsWidget(app, txns),
		netWorthTrendWidget(accounts, txns, rates, net, widgetCfgs.For("trend")),
		cashFlowWidget(txns, rates),
		savingsRateWidget(income, expense, widgetCfgs.For("savings")),
		spendingBreakdownWidget(app, txns, rates, start, end, widgetCfgs.For("breakdown")),
		upcomingBillsWidget(app),
		freshnessWidget(accounts, app.FreshnessWindows(), remindToUpdate),
		topHighlightWidget(txns, app.Categories(), rates),
	)
}

// freshnessWidget is the full-width Freshness nudge: a friendly reminder of which
// account balances look stale (via internal/freshness), with how long since each
// was last updated.
func freshnessWidget(accounts []domain.Account, windows freshness.Windows, onRemind ui.Handler) ui.Node {
	now := time.Now()
	stale := freshness.StaleAccounts(accounts, windows, now)
	var body ui.Node
	if len(stale) == 0 {
		body = P(Class("text-up text-[13px]"), uistate.T("dashboard.allFresh"))
	} else {
		chips := make([]ui.Node, 0, len(stale))
		for _, a := range stale {
			chips = append(chips, Span(Class("member-chip"),
				Span(a.Name),
				Span(Class("text-warn fig"), fmt.Sprintf("· %dd", freshness.DaysSinceUpdate(a, now))),
			))
		}
		body = Div(
			P(Class("text-dim text-[13px] mb-2"), uistate.T("dashboard.staleCount", len(stale))),
			Div(Class("flex flex-wrap gap-2 items-center"), chips),
			Button(Class("btn mt-2"), Type("button"), Title(uistate.T("dashboard.remindTitle")), OnClick(onRemind), uistate.T("dashboard.remind")),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "freshness", Title: uistate.T("dashboard.freshness"), Draggable: true, Resizable: true,
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
		bills = append(bills, bill{name: a.Name, due: dateutil.NextMonthlyDue(now, a.DueDayOfMonth), amount: a.MinPayment})
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
		ID: "bills", Title: uistate.T("dashboard.upcomingBills"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "6",
		Body: body,
	})
}

// spendingBreakdownWidget is the 2×1 Spending breakdown widget: a segmented bar
// of the period's expenses by category (top three plus "Other") with a legend.
func spendingBreakdownWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time, cfg widgetcfg.Config) ui.Node {
	topN := 3
	if sch, ok := widgetcfg.SchemaFor("breakdown"); ok {
		if f, ok := sch.FieldByKey("topN"); ok {
			topN = f.Int(cfg)
		}
	}
	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	parent := make(map[string]string, len(cats))
	exists := make(map[string]bool, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		parent[c.ID] = c.ParentID
		exists[c.ID] = true
	}
	// rootOf walks up to the top-level ancestor so sub-category spend rolls up to
	// its parent. Cycle/orphan-safe (stops at a missing parent or a repeat).
	rootOf := func(id string) string {
		seen := map[string]bool{}
		for {
			p := parent[id]
			if p == "" || !exists[p] || seen[id] {
				break
			}
			seen[id] = true
			id = p
		}
		return id
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
		totals[rootOf(t.CategoryID)] += amt
		total += amt
	}

	if total == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
			Body: P(Class("empty text-dim text-[13px]"), uistate.T("dashboard.noSpending")),
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

	// Top N categories, the rest lumped into "Other".
	if len(segs) > topN+1 {
		var other int64
		for _, s := range segs[topN:] {
			other += s.amt
		}
		segs = append(segs[:topN], seg{name: "Other", amt: other})
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
		ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
		Body: body,
	})
}

// savingsRateWidget is the 2×1 Savings rate widget: the share of the period's
// income that wasn't spent, as a big figure and a bar.
func savingsRateWidget(income, expense money.Money, cfg widgetcfg.Config) ui.Node {
	pct := ledger.SavingsRate(income.Amount, expense.Amount)

	// Widget settings (gear → flip): target savings rate and whether to show the bar.
	target, showBar := 20, true
	if sch, ok := widgetcfg.SchemaFor("savings"); ok {
		if f, ok := sch.FieldByKey("target"); ok {
			target = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("showBar"); ok {
			showBar = f.Bool(cfg)
		}
	}

	// Tone reflects performance against the user's target: at/above = good,
	// positive-but-short = warning, negative = bad.
	tone, bar := "text-up", "bg-up"
	switch {
	case pct < 0:
		tone, bar = "text-down", "bg-down"
	case pct < target:
		tone, bar = "text-warn", "bg-warn"
	}

	left := Div(
		Div(Class("font-display fig text-[34px] leading-none "+tone), fmt.Sprintf("%d%%", pct)),
		Div(Class("text-[12px] text-dim mt-1"), uistate.T("dashboard.savingsSub", target)),
	)
	var right ui.Node = Fragment()
	if showBar {
		right = Div(Class("flex-1"),
			uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: bar}),
			Div(Class("text-[11px] text-faint mt-2"), uistate.T("dashboard.thisPeriod")),
		)
	}
	body := Div(Class("flex items-center gap-5"), left, right)
	return uiw.Widget(uiw.WidgetProps{
		ID: "savings", Title: uistate.T("dashboard.savingsRate"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "7",
		Body: body,
	})
}

// topHighlightWidget surfaces the single most significant spending change this
// month (via the shared anomaly detection) as a one-line plain-English highlight,
// or a calm "nothing notable" message when there's nothing to flag. It links the
// dashboard to the fuller Spending highlights card on the Insights screen.
func topHighlightWidget(txns []domain.Transaction, categories []domain.Category, rates currency.Rates) ui.Node {
	anomalies := detectSpendingAnomalies(txns, categories, rates)
	var body ui.Node
	if len(anomalies) == 0 {
		body = P(Class("text-dim text-[13px]"), uistate.T("dashboard.noHighlights"))
	} else {
		a := anomalies[0]
		body = Div(Class("flex items-start gap-2"),
			Span(Class("insight-dot "+highlightTone(a)), Text(highlightArrow(a))),
			Span(Class("text-[13px]"), highlightText(a, rates.Base)),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "highlight", Title: uistate.T("dashboard.highlight"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "9", Body: body,
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
		ID: "cashflow", Title: uistate.T("dashboard.cashFlow"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "6",
		Body: Div(Class("flex items-end gap-5"), bars, netBlock),
	})
}

// netWorthTrendWidget is the 1×2 Net worth trend widget: the current figure over
// a six-month end-of-month area chart (via ledger.NetWorthSeries + the chart
// geometry helpers).
func netWorthTrendWidget(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, net money.Money, cfg widgetcfg.Config) ui.Node {
	months := 6
	if sch, ok := widgetcfg.SchemaFor("trend"); ok {
		if f, ok := sch.FieldByKey("months"); ok {
			months = f.Int(cfg)
		}
	}
	start := dateutil.MonthStart(time.Now())
	cutoffs := make([]time.Time, 0, months)
	for i := 0; i < months; i++ {
		cutoffs = append(cutoffs, dateutil.AddMonths(start, i-(months-2))) // window ends at the current month +1
	}
	series, _ := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	values := make([]float64, len(series))
	for i, m := range series {
		values[i] = float64(m.Amount)
	}
	body := Div(Class("flex flex-col h-full"),
		Div(Class("font-display fig text-[22px]"), fmtAccounting(net)),
		uiw.AreaChart(uiw.AreaChartProps{Values: values, GradientID: "cf-networth", Label: uistate.T("dashboard.netWorthChartLabel", fmtAccounting(net))}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "trend", Title: uistate.T("dashboard.netWorth"), Draggable: true, Resizable: true, GridColumn: "4", GridRow: "3 / span 2",
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
		ID: "accounts", Title: uistate.T("nav.accounts"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "5",
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
			// Distinguish priority by shape as well as color (▲/●/○), and give the
			// marker an accessible name — so it doesn't rely on color alone and
			// isn't a silent glyph to screen readers.
			dotTone, dot, prio := "text-faint", "○", "Low priority"
			switch t.Priority {
			case domain.PriorityHigh:
				dotTone, dot, prio = "text-warn", "▲", "High priority"
			case domain.PriorityMedium:
				dotTone, dot, prio = "text-dim", "●", "Medium priority"
			}
			rows = append(rows, Div(Class("flex gap-2 items-center"),
				Span(Class(dotTone), Attr("title", prio), Attr("aria-label", prio), dot),
				Span(t.Title),
			))
		}
		body = Div(Class("text-[13px] space-y-2"), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo", Title: uistate.T("nav.todo"), Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
		Body: body,
	})
}

// goalsWidget is the 1×1 Goals widget: the first goal's progress (% + saved /
// target) via internal/goals.
func goalsWidget(app *appstate.App) ui.Node {
	list := app.Goals()
	if len(list) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goals", Title: uistate.T("nav.goals"), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
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
		ID: "goals", Title: uistate.T("dashboard.goalPrefix", g.Name), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
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
		ID: "budgets", Title: uistate.T("nav.budgets"), Draggable: true, Resizable: true,
		GridColumn: "3", GridRow: "3 / span 2", Body: body,
	})
}

// recentWidget is the 2×2 Recent transactions widget: newest activity as a
// compact table with accounting amounts. Display-only, so rows build in a loop.
func recentWidget(txns []domain.Transaction, cfg widgetcfg.Config) ui.Node {
	count := 6
	if sch, ok := widgetcfg.SchemaFor("recent"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			count = f.Int(cfg)
		}
	}
	recent := recentTransactions(txns, count)
	var body ui.Node
	if len(recent) == 0 {
		body = P(Class("empty text-dim text-[13px]"), uistate.T("dashboard.noTransactions"))
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
		ID: "recent", Title: uistate.T("dashboard.recent"), Draggable: true, Resizable: true,
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
				H1(Class("font-display text-2xl font-semibold tracking-tight"), uistate.T("dashboard.title")),
				P(Class("text-dim mt-0.5 text-[13px]"), uistate.T("dashboard.hint")),
			),
			Button(Class("data-btn"), Type("button"), OnClick(reset), uistate.T("dashboard.reset")),
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
