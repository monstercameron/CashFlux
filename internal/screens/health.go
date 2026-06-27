// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// healthLookbackMonths is the trailing window used to derive savings rate, average
// monthly spending, and monthly income — three full months, excluding the current
// partial month so a mid-month dip doesn't distort the score.
const healthLookbackMonths = 3

// buildHealthInputs derives the financial-health signals from the live store,
// reusing the existing tested ledger/reports primitives. Every factor carries an
// applicability flag so the model can drop what doesn't apply (e.g. no cards) and
// re-normalize, rather than penalizing a household for something it doesn't have.
func buildHealthInputs(app *appstate.App, now time.Time) healthscore.Inputs {
	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	var in healthscore.Inputs

	// Trailing three full months (exclude the current partial month).
	curMonth := dateutil.MonthStart(now)
	start := dateutil.AddMonths(curMonth, -healthLookbackMonths)
	flow, err := reports.IncomeVsExpense(txns, start, curMonth, rates)
	hasFlow := err == nil && (flow.Income > 0 || flow.Expense > 0)

	monthlyIncome := int64(0)
	avgMonthlySpend := int64(0)
	if hasFlow {
		monthlyIncome = flow.Income / healthLookbackMonths
		avgMonthlySpend = flow.Expense / healthLookbackMonths
		if flow.Income > 0 {
			in.HasIncome = true
			in.SavingsRatePct = ledger.SavingsRate(flow.Income, flow.Expense)
		}
	}

	// Emergency fund: liquid cash ÷ average monthly spending.
	if avgMonthlySpend > 0 {
		if liquid, lerr := ledger.LiquidBalance(accounts, txns, rates); lerr == nil {
			in.HasLiquidData = true
			in.EmergencyMonths = float64(liquid.Amount) / float64(avgMonthlySpend)
		}
	}

	// Debt payments vs income: Σ liability minimum payments ÷ monthly income.
	// Applicable whenever there's income; zero debt scores 100 (handled in the model).
	if in.HasIncome {
		var minSum int64
		anyLiab := false
		for _, a := range accounts {
			if a.Archived || !a.Type.IsLiability() {
				continue
			}
			anyLiab = true
			conv, cerr := currency.ConvertBetween(a.MinPayment.Amount, a.MinPayment.Currency, base, rates)
			if cerr != nil {
				conv = a.MinPayment.Amount
			}
			minSum += conv
		}
		in.HasLiabilities = anyLiab
		if monthlyIncome > 0 {
			in.ObligationRatioPct = int(minSum * 100 / monthlyIncome)
		}
	}

	// Budget adherence: share of budgets within their limit this period. Mirrors
	// the dashboard's budget evaluation (rollup over sub-categories, current period).
	if budgets := app.Budgets(); len(budgets) > 0 {
		cats := app.Categories()
		weekStart := uistate.UsePrefs().Get().WeekStartWeekday()
		total, within := 0, 0
		for _, b := range budgets {
			bs, be := budgeting.PeriodRange(b.Period, now, weekStart)
			st, berr := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
			if berr != nil {
				continue
			}
			total++
			if st.State != budgeting.StateOver {
				within++
			}
		}
		if total > 0 {
			in.HasBudgets = true
			in.BudgetAdherencePct = within * 100 / total
		}
	}

	// Aggregate credit utilization: Σ card balances ÷ Σ card limits.
	var balSum, limitSum int64
	for _, a := range accounts {
		if a.Archived || a.CreditLimit.Amount <= 0 {
			continue
		}
		bal, berr := ledger.Balance(a, txns)
		if berr != nil {
			continue
		}
		owed := bal.Amount
		if owed < 0 {
			owed = -owed
		}
		ob, cerr := currency.ConvertBetween(owed, bal.Currency, base, rates)
		if cerr != nil {
			ob = owed
		}
		ol, cerr := currency.ConvertBetween(a.CreditLimit.Amount, a.CreditLimit.Currency, base, rates)
		if cerr != nil {
			ol = a.CreditLimit.Amount
		}
		balSum += ob
		limitSum += ol
	}
	if limitSum > 0 {
		in.HasCredit = true
		if pct, ok := ledger.Utilization(balSum, limitSum); ok {
			in.AggUtilizationPct = pct
		}
	}

	// Net-worth trend: the trailing six-month change as a percentage, derived from
	// the same ledger.NetWorthSeries the dashboard's net-worth chart uses (so the
	// health factor and the chart agree). A meaningful percentage needs a positive
	// starting net worth; a zero or negative baseline leaves the factor inapplicable
	// (the model then excludes it and re-normalizes the remaining weights).
	const healthNWTrendMonths = 6
	nwStart := dateutil.AddMonths(curMonth, -healthNWTrendMonths)
	if series, nwErr := ledger.NetWorthSeries(accounts, txns, []time.Time{nwStart, now}, rates); nwErr == nil && len(series) == 2 && series[0].Amount > 0 {
		in.HasNWTrend = true
		in.NWTrendPct = float64(series[1].Amount-series[0].Amount) / float64(series[0].Amount) * 100
	}

	return in
}

// healthHue maps a 0–100 score to a continuous red→amber→green hue (HSL), so the
// score ring shifts smoothly with the number rather than snapping at band edges.
// The band label carries the categorical meaning; the hue is the analog cue.
func healthHue(score int) int { return score * 13 / 10 } // 0 → 0 (red), 100 → 130 (green)

// healthColor is the ring/figure stroke for a score (a vivid HSL), or a calm dim
// grey for the "not enough data" state.
func healthColor(r healthscore.Result) string {
	if r.Band == healthscore.BandNoData {
		return "var(--dim, #6b7280)"
	}
	return fmt.Sprintf("hsl(%d, 64%%, 52%%)", healthHue(r.Score))
}

// healthTextTone maps a band to one of the app's semantic tone classes for the
// band label and figure text.
func healthTextTone(b healthscore.Band) string {
	switch b {
	case healthscore.BandExcellent, healthscore.BandGood:
		return "text-up"
	case healthscore.BandFair:
		return "text-warn"
	case healthscore.BandCritical, healthscore.BandNeedsWork:
		return "text-down"
	default:
		return "text-dim"
	}
}

// healthBarTone maps a factor score to a progress-bar background tone.
func healthBarTone(score int) string {
	switch {
	case score >= 60:
		return "bg-up"
	case score >= 40:
		return "bg-warn"
	default:
		return "bg-down"
	}
}

// healthRing renders the circular score gauge as an SVG: a faint full track plus a
// stroked arc whose length is the score and whose color is the continuous hue. The
// score figure is overlaid in the display font (and picks up the count-up tween via
// the `fig` class). size is the outer pixel diameter.
func healthRing(r healthscore.Result, size int) ui.Node {
	const radius = 52.0
	const circ = 2 * 3.141592653589793 * radius
	pct := float64(r.Score)
	if r.Band == healthscore.BandNoData {
		pct = 0
	}
	offset := circ * (1 - pct/100)
	color := healthColor(r)
	figure := fmt.Sprintf("%d", r.Score)
	if r.Band == healthscore.BandNoData {
		figure = "—"
	}
	px := fmt.Sprintf("%dpx", size)

	// R52/R64 a11y: the ring is the primary score visual, so give it a real
	// screen-reader name (role=img + a one-sentence label with the score and band)
	// rather than hiding it; the overlay number below is then aria-hidden so the
	// score isn't announced twice.
	ringLabel := uistate.T("health.ringLabel", r.Score, string(r.Band))
	if r.Band == healthscore.BandNoData {
		ringLabel = uistate.T("health.ringLabelNoData")
	}
	ring := Svg(
		Attr("viewBox", "0 0 120 120"),
		Attr("width", px), Attr("height", px),
		Attr("role", "img"), Attr("aria-label", ringLabel),
		// Faint full track.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", "var(--line, #2a2a2d)"), Attr("stroke-width", "10")),
		// Score arc — starts at 12 o'clock (rotate -90), rounded cap, animates length.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", color), Attr("stroke-width", "10"),
			Attr("stroke-linecap", "round"),
			Attr("stroke-dasharray", fmt.Sprintf("%.2f", circ)),
			Attr("stroke-dashoffset", fmt.Sprintf("%.2f", offset)),
			Attr("transform", "rotate(-90 60 60)"),
			Style(map[string]string{"transition": "stroke-dashoffset .9s cubic-bezier(.22,1,.36,1), stroke .6s ease"})),
	)

	overlay := Div(
		// Visual duplicate of the score the ring's aria-label already announces.
		Attr("aria-hidden", "true"),
		Style(map[string]string{
			"position": "absolute", "inset": "0",
			"display": "flex", "flex-direction": "column",
			"align-items": "center", "justify-content": "center",
		}),
		Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(healthTextTone(r.Band))),
			Style(map[string]string{"font-size": fmt.Sprintf("%dpx", size/3)}), figure),
		Div(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "2px"}), "out of 100"),
	)

	return Div(
		Style(map[string]string{"position": "relative", "width": px, "height": px, "flex": "0 0 " + px}),
		ring, overlay,
	)
}

// healthDeltaLine renders the "since last month" change chip from a prior score,
// or a calm baseline note when there's no earlier reading.
func healthDeltaLine(score int, prior int, hasPrior bool) ui.Node {
	if !hasPrior {
		return Div(css.Class("t-caption", tw.TextFaint), "First reading — we'll track your trend from here")
	}
	d := score - prior
	switch {
	case d > 0:
		return Div(ClassStr("t-caption "+tw.ColorClass("text-up")), fmt.Sprintf("▲ %d since last month", d))
	case d < 0:
		return Div(ClassStr("t-caption "+tw.ColorClass("text-down")), fmt.Sprintf("▼ %d since last month", -d))
	default:
		return Div(css.Class("t-caption", tw.TextDim), "No change since last month")
	}
}

// weakestApplicable returns the lowest-scoring applicable factor, ok=false when
// none are applicable (the no-data case).
func weakestApplicable(r healthscore.Result) (healthscore.Factor, bool) {
	var weakest healthscore.Factor
	found := false
	for _, f := range r.Factors {
		if !f.Applicable {
			continue
		}
		if !found || f.Score < weakest.Score {
			weakest = f
			found = true
		}
	}
	return weakest, found
}

// healthWidgetNode is the dashboard "Financial health" bento tile (R27). It is a
// component (rendered via ui.CreateElement) so it can own its hooks — recording the
// monthly snapshot and reading the trend atom — safely from the dashboard's widget
// map, where calling hooks inline would violate the stable-position rule.
func healthWidgetNode(struct{}) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Widget(uiw.WidgetProps{ID: "health", Title: uistate.T("dashboard.healthScore"),
			Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get() // re-render on any data mutation

	now := time.Now()
	month := now.Format("2006-01")
	in := buildHealthInputs(app, now)
	r := healthscore.Evaluate(in)

	trend := uistate.UseHealthTrend().Get()
	prior, hasPrior := uistate.PriorHealthScore(trend, month)

	// Record this month's snapshot once per (month, score) — after render, so the
	// atom Set doesn't recurse. PriorHealthScore ignores the current month, so the
	// delta above stays correct across the re-render this triggers.
	if r.Band != healthscore.BandNoData {
		ui.UseEffect(func() func() {
			uistate.RecordHealthSnapshot(month, r.Score, string(r.Band))
			return nil
		}, month+"|"+fmt.Sprintf("%d", r.Score))
	}

	nav := router.UseNavigate()

	var right ui.Node
	if r.Band == healthscore.BandNoData {
		right = Div(css.Class(tw.Flex1),
			Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass("text-dim")), string(healthscore.BandNoData)),
			P(css.Class("t-caption", tw.TextFaint, tw.Mt1), "Add income, accounts, or budgets to see your score"),
		)
	} else {
		weakLine := Fragment()
		if w, ok := weakestApplicable(r); ok {
			weakLine = Div(css.Class("t-caption", tw.TextDim, tw.Mt2),
				Span(css.Class(tw.TextFaint), "Weakest: "),
				Text(w.Label+" "+w.Value+" → "+w.Target))
		}
		right = Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
			Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(r.Band))), string(r.Band)),
			healthDeltaLine(r.Score, prior, hasPrior),
			weakLine,
			Div(css.Class(tw.Mt2),
				Button(css.Class("btn-link"), OnClick(func() { nav.Navigate(uistate.RoutePath("/health")) }), "View steps →")),
		)
	}

	body := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5),
		healthRing(r, 110),
		right,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "health", Title: uistate.T("dashboard.healthScore"),
		Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "8",
		Body: body,
	})
}

// HealthScreen is the full /health page: the score ring, a per-factor breakdown
// (value, contribution to the score, and a bar), the prioritized next steps, and a
// privacy note. Everything is computed locally from the user's own data.
func HealthScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	now := time.Now()
	month := now.Format("2006-01")
	in := buildHealthInputs(app, now)
	r := healthscore.Evaluate(in)
	trend := uistate.UseHealthTrend().Get()
	prior, hasPrior := uistate.PriorHealthScore(trend, month)

	// Hero: ring + band + delta.
	hero := uiw.Card(uiw.CardProps{Body: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5),
		healthRing(r, 150),
		Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
			Div(ClassStr("t-figure-lg "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(r.Band))), string(r.Band)),
			healthDeltaLine(r.Score, prior, hasPrior),
			If(r.NegativeCashFlow,
				Div(ClassStr("t-caption "+tw.ColorClass("text-down")), Style(map[string]string{"margin-top": "6px"}),
					"⚠ You're spending more than you earn right now")),
		),
	)})

	// Per-factor breakdown.
	rows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap4)}
	for _, f := range r.Factors {
		rows = append(rows, healthFactorRow(f))
	}
	breakdown := uiw.Card(uiw.CardProps{
		Title: "What goes into your score",
		Body:  Div(rows...),
	})

	// Prioritized steps.
	stepNodes := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap3)}
	for _, s := range r.Steps {
		stepNodes = append(stepNodes, Div(css.Class("row", tw.Flex, tw.FlexCol, tw.Gap1),
			Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), s.Factor),
			Div(css.Class("t-caption", tw.TextDim), s.Action),
			Div(css.Class("t-caption", tw.TextFaint), "Target: "+s.Target),
		))
	}
	var steps ui.Node = Fragment()
	if len(r.Steps) > 0 {
		steps = uiw.Card(uiw.CardProps{
			Title: "Where to focus next",
			Body:  Div(stepNodes...),
		})
	}

	privacy := P(css.Class("t-caption", tw.TextFaint),
		"Calculated on your device from your own data — never uploaded or shared.")

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5), hero, breakdown, steps, privacy)
}

// healthFactorRow renders one factor in the /health breakdown: its label + value, a
// score bar, its contribution share, and target. Inapplicable factors render as a
// calm "not applicable" line so the user understands why it's excluded.
func healthFactorRow(f healthscore.Factor) ui.Node {
	if !f.Applicable {
		return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
			Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), f.Label),
			Span(css.Class("t-caption", tw.TextFaint), "Not applicable to you"),
		)
	}
	head := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
		Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), f.Label),
		Span(ClassStr("t-body "+tw.ColorClass(healthTextTone(healthBandForScore(f.Score)))), f.Value),
	)
	bar := uiw.ProgressBar(uiw.ProgressBarProps{Percent: f.Score, Tone: healthBarTone(f.Score)})
	sub := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mt1),
		Span(css.Class("t-caption", tw.TextFaint), fmt.Sprintf("%d%% of your score", f.ContributionPct)),
		Span(css.Class("t-caption", tw.TextDim), "Target: "+f.Target),
	)
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), head, bar, sub)
}

// healthBandForScore mirrors the model's banding for per-factor tone (the model
// keeps bandFor unexported; this re-derives the tone thresholds for display only).
func healthBandForScore(score int) healthscore.Band {
	switch {
	case score >= 80:
		return healthscore.BandExcellent
	case score >= 60:
		return healthscore.BandGood
	case score >= 40:
		return healthscore.BandFair
	case score >= 25:
		return healthscore.BandNeedsWork
	default:
		return healthscore.BandCritical
	}
}
