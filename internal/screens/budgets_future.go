// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetFutureMaxRows caps the projected-occurrence list so a long future window
// (e.g. a whole year) stays glanceable; the remainder is summarized as "+ N more".
const budgetFutureMaxRows = 8

type budgetFutureViewKey struct {
	app         *appstate.App
	revision    uint64
	member      string
	start       time.Time
	end         time.Time
	weekStart   time.Weekday
	rollover    bool
	incomeMode  string
	paycheckMin int64
	fixedIncome int64
	avgMonths   int
	payAnchor   string
	viewAnchor  string
	categoryIDs string
}

type budgetFutureTotals struct {
	TotalLimit      int64
	SavingsAssigned int64
	BannerIncome    int64
}

var budgetFutureViews = map[budgetFutureViewKey]budgetFutureTotals{}
var budgetFutureViewCalculations uint64

// budgetFutureWidget answers coworker feedback #6 — "when navigating future
// timelines, show future-related things, not empty values." When the global
// period control is paged to a window that BEGINS in the future, the budgets
// page would otherwise show empty actuals (no transactions exist yet). This
// self-gating tile fills that gap: it projects the household's recurring bills
// and income into the viewed window and shows what's set to land — money in,
// money out, and the individual occurrences by date. It renders nothing for the
// current or a past period, so it never clutters the normal view.
func budgetFutureWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	w := uistate.UsePeriod().Get()
	activeMember := uistate.UseActiveMember().Get()
	prefs := uistate.UsePrefs().Get()
	open := ui.UseState(false)
	toggleOpen := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	return renderBudgetFutureWidget(props, w, dateutil.WallClockUTC(time.Now()), activeMember, prefs, open.Get(), toggleOpen)
}

func renderBudgetFutureWidget(props budgetSummaryProps, w period.Window, now time.Time, activeMember string, prefs prefs.Prefs, open bool, toggleOpen ui.Handler) ui.Node {
	toggleOnClick := OnClick(toggleOpen)
	app := props.App
	start, end := w.Range()
	if !isFuturePeriod(w, now) {
		// Not a future window: render a hidden placeholder rather than an empty
		// Fragment. An empty→content transition would late-mount and append the tile
		// to the end of the bento (losing its lead position); a keyed placeholder
		// holds the slot so it mutates in place when the user pages into the future.
		return Div(Attr("data-testid", "budget-future-ph"), Style(map[string]string{"display": "none"}))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Active recurrings only, with a currency lookup for base conversion (Ghost
	// carries a signed minor amount but not its currency).
	active := make([]domain.Recurring, 0)
	curOf := make(map[string]string)
	for _, r := range app.Recurring() {
		if r.Active() {
			active = append(active, r)
			curOf[r.ID] = r.Amount.Currency
		}
	}
	ghosts := txncalendar.Ghosts(active, start, end)
	sort.SliceStable(ghosts, func(i, j int) bool { return ghosts[i].Date.Before(ghosts[j].Date) })

	// Keep the future income figure on the same basis as the budget hero. Scheduled
	// deposits remain visible as individual recurring rows, while this total is the
	// plan's expected income, not a second estimate derived from the recurrence list.
	view, err := cachedBudgetFutureView(app, activeMember, w, prefs)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	expectedWindowIncome, err := budgetplan.ScaleMonthlyIncome(money.New(view.BannerIncome, base), start, end)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	expectedIncome := expectedWindowIncome.Amount
	var outflow int64
	for _, g := range ghosts {
		conv, err := currency.ConvertBetween(g.Amount, curOf[g.RecurringID], base, rates)
		if err != nil {
			continue
		}
		if conv < 0 {
			positive, err := money.Neg(conv)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
			outflow, err = money.Add(outflow, positive)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
		}
	}
	var receivedIncome int64
	if lines, err := reports.IncomeByCategory(app.Transactions(), start, end, rates); err == nil {
		for _, line := range lines {
			receivedIncome, err = money.Add(receivedIncome, line.Amount)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
		}
	}
	planned, err := money.Add(view.TotalLimit, view.SavingsAssigned)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	net, err := money.Sub(expectedIncome, outflow)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}

	stats := Div(css.Class("bfut-stats"),
		bfutStat("budgets.future.expectedIncome", money.New(expectedIncome, base), "text-up"),
		bfutStat("budgets.future.out", money.New(outflow, base), "text-down"),
		bfutStat("budgets.future.net", money.New(net, base), netTone(net)),
	)

	var listBody ui.Node
	if len(ghosts) == 0 {
		listBody = P(css.Class("empty t-body", tw.TextDim), Attr("data-testid", "budgets-future-empty"),
			uistate.T("budgets.future.none"))
	} else {
		rows := make([]ui.Node, 0, budgetFutureMaxRows+1)
		for i, g := range ghosts {
			if i >= budgetFutureMaxRows {
				rows = append(rows, P(css.Class("brc-more t-caption", tw.TextFaint),
					uistate.T("budgets.future.more", len(ghosts)-budgetFutureMaxRows)))
				break
			}
			rows = append(rows, budgetFutureRow(g, curOf[g.RecurringID]))
		}
		listBody = Div(css.Class("brc-rows"), rows)
	}

	billCount := 0
	for _, g := range ghosts {
		if g.Amount < 0 {
			billCount++
		}
	}
	foldLabel := uistate.TN("budgets.future.foldHintOne", "budgets.future.foldHintMany",
		billCount, fmtMoney(money.New(outflow, base)))
	buttonLabel := uistate.T("budgets.future.showAria")
	if open {
		buttonLabel = uistate.T("budgets.future.hideAria")
	}
	fold := Button(css.Class("budget-fold-toggle"), Type("button"),
		Attr("data-testid", "budgets-future-toggle"), Attr("aria-expanded", ariaBool(open)),
		Attr("aria-label", buttonLabel), toggleOnClick,
		Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.future.title", w.Label())+" · "),
		Span(css.Class("budget-fold-toggle-hint"), foldLabel),
	)

	var body ui.Node = Fragment()
	if open {
		body = Fragment(
			P(css.Class("budget-future-plan", tw.Text13), Attr("data-testid", "budgets-future-planning"), uistate.T("budgets.future.planning",
				fmtMoney(money.New(planned, base)), fmtMoney(money.New(expectedIncome, base)))),
			P(css.Class("budget-future-received", tw.Text12, tw.TextDim), Attr("data-testid", "budgets-future-received"), uistate.T("budgets.future.received",
				fmtMoney(money.New(receivedIncome, base)))),
			P(css.Class("muted", tw.Text13), uistate.T("budgets.future.desc")),
			stats,
			// EC-11: whether the month ahead is actually set up, as sentences
			// naming what is missing. A score would tell the reader something is
			// wrong and nothing about what.
			readinessNotes(app, now, rates, base),
			listBody,
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-future", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("brc budget-fold"), Attr("data-testid", "budgets-future"), fold, body),
	})
}

// cachedBudgetFutureView avoids rerunning the household's full ledger budget
// evaluation when the future projection is revisited with the same data and
// period. The bounded cache key contains every view input; no timestamp is used
// to choose or replace a result.
func cachedBudgetFutureView(app *appstate.App, member string, w period.Window, p prefs.Prefs) (budgetFutureTotals, error) {
	start, end := w.Range()
	key := budgetFutureViewKey{
		app: app, revision: app.Rev(), member: member, start: start, end: end,
		weekStart: p.WeekStartWeekday(), rollover: p.BudgetRolloverLeftover,
		incomeMode: p.BudgetIncomeMode, paycheckMin: p.BudgetPaycheckMinMinor,
		fixedIncome: p.MonthlyIncomeMinor, avgMonths: p.BudgetIncomeAvgMonths,
		payAnchor: p.PayCycleAnchor, viewAnchor: dateutil.FormatDate(dateutil.WallClockUTC(time.Now())),
		categoryIDs: strings.Join(p.BudgetIncomeCategoryIDs, "\x00"),
	}
	if view, ok := budgetFutureViews[key]; ok {
		return view, nil
	}
	if len(budgetFutureViews) >= 8 {
		budgetFutureViews = make(map[budgetFutureViewKey]budgetFutureTotals)
	}
	budgetFutureViewCalculations++
	view, err := calculateBudgetFutureTotals(app, member, w, p)
	if err != nil {
		return budgetFutureTotals{}, err
	}
	budgetFutureViews[key] = view
	return view, nil
}

// calculateBudgetFutureTotals computes only the plan figures the future tile
// displays. It skips annotations that are useful on the current-month cards but
// expensive over the full ledger, such as transfer-suspect scans and age of
// money, while using the same tested budget evaluation and rollup services.
func calculateBudgetFutureTotals(app *appstate.App, member string, w period.Window, p prefs.Prefs) (budgetFutureTotals, error) {
	settings := app.Settings()
	base := budgetBaseCurrency(settings.BaseCurrency)
	rates := currency.Rates{Base: base, Rates: settings.FXRates}
	start, end := w.Range()
	now := dateutil.WallClockUTC(time.Now())
	transactions := app.Transactions()
	categories := app.Categories()
	input := budgeting.ViewInput{
		Period:       budgeting.ResolveViewPeriod(start, end, now, p.WeekStartWeekday(), budgetPayCycleAnchor(p.PayCycleAnchor)),
		Transactions: transactions,
		Categories:   categories,
		Recurring:    app.Recurring(),
		Rates:        rates,
		Base:         base,
		PoolRollover: p.BudgetRolloverLeftover,
	}
	budgets := budgeting.ViewBudgets(app.Budgets(), member)
	statuses := make([]budgeting.Status, 0, len(budgets))
	for _, budget := range budgets {
		budget = budget.WithBaseCurrency(base)
		statuses = append(statuses, budgeting.Status{
			Budget: budget, Limit: budget.Limit, Spent: money.Zero(base), Remaining: budget.Limit,
		})
	}
	rollup, _, err := budgeting.SummarizeViewStatuses(statuses, base, rates)
	if err != nil {
		return budgetFutureTotals{}, err
	}
	basis := budgeting.ViewIncomeBasis{
		Mode: p.BudgetIncomeMode, PaycheckMin: p.BudgetPaycheckMinMinor,
		Fixed: p.MonthlyIncomeMinor, CategoryIDs: p.BudgetIncomeCategoryIDs,
		AverageMonths: p.BudgetIncomeAvgMonths,
	}
	incomePeriod := budgetFutureIncomePeriod(input.Period, now)
	income, err := budgeting.ResolveViewIncome(basis, transactions, categories, incomePeriod, base, rates)
	if err != nil {
		return budgetFutureTotals{}, err
	}
	explicitIncome := basis.Mode == budgeting.IncomeModeAll || basis.Mode == budgeting.IncomeModePaychecks || basis.Mode == budgeting.IncomeModeFixed || basis.Mode == budgeting.IncomeModeCategories
	if !explicitIncome && income == 0 {
		monthStart, _ := budgeting.PeriodRange(domain.PeriodMonthly, incomePeriod.Anchor, incomePeriod.WeekStart)
		if prior, _, priorErr := ledger.PeriodTotals(transactions, dateutil.AddMonths(monthStart, -1), monthStart, rates); priorErr == nil {
			income = prior.Amount
		}
	}
	var savingsAssigned int64
	for _, account := range computeSavingsAccounts(app, member, input.Period.Anchor, base, rates) {
		if account.ConversionError != nil {
			return budgetFutureTotals{}, account.ConversionError
		}
		savingsAssigned, err = money.Add(savingsAssigned, account.AssignedBase)
		if err != nil {
			return budgetFutureTotals{}, err
		}
	}
	savingsAssigned, err = money.Add(savingsAssigned, rollup.SavingsBudgetedMinor)
	if err != nil {
		return budgetFutureTotals{}, err
	}
	return budgetFutureTotals{TotalLimit: rollup.LimitMinor, SavingsAssigned: savingsAssigned, BannerIncome: income}, nil
}

// budgetFutureIncomePeriod keeps income history anchored to the current wall-clock
// day while future budgets and savings remain anchored to the selected window.
func budgetFutureIncomePeriod(view budgeting.ViewPeriod, now time.Time) budgeting.ViewPeriod {
	view.Anchor = dateutil.WallClockUTC(now)
	return view
}

// budgetWindowIsHistorical reports whether the selected window has ended as of
// the user's wall-clock calendar day.
func budgetWindowIsHistorical(end, now time.Time) bool {
	return !dateutil.WallClockUTC(now).Before(end)
}

// isFuturePeriod reports whether the viewed window starts after today. Budgets uses
// this same decision to replace present-tense funding alarms with planning language.
func isFuturePeriod(w period.Window, now time.Time) bool {
	start, _ := w.Range()
	now = dateutil.WallClockUTC(now)
	return start.After(now)
}

// bfutStat renders one in/out/net figure block.
func bfutStat(labelKey string, m money.Money, tone string) ui.Node {
	return Div(css.Class("bfut-stat"),
		Attr("data-testid", "budgets-future-stat-"+strings.TrimPrefix(labelKey, "budgets.future.")),
		Span(css.Class("bfut-stat-label", tw.TextDim), uistate.T(labelKey)),
		Span(ClassStr("bfut-stat-val fig "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(tone)), fmtMoney(m)),
	)
}

// netTone picks the semantic colour for a net figure.
func netTone(net int64) string {
	if net < 0 {
		return "text-down"
	}
	return "text-up"
}

// budgetFutureRow renders one projected occurrence: its due date, the recurring's
// label, and its signed amount. Display-only, safe inside the row loop.
func budgetFutureRow(g txncalendar.Ghost, cur string) ui.Node {
	amt := money.New(g.Amount, cur)
	tone := "text-up"
	if g.Amount < 0 {
		tone = "text-down"
	}
	return Div(css.Class("brc-row"), Attr("data-testid", "budgets-future-row"),
		Span(css.Class("brc-cadence"), uistate.LoadPrefs().FormatDate(g.Date)),
		Div(css.Class("brc-body"),
			Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), g.Label),
		),
		Div(css.Class("brc-amtcol"),
			Span(ClassStr("brc-amt fig "+tw.ColorClass(tone)), fmtMoney(amt)),
		),
	)
}
