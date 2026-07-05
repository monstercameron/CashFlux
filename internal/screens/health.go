// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// liveHealthInputs derives the financial-health signals through the SINGLE
// shared pure derivation (engineenv.HealthInputs) — the same one the health_*
// engine variables and the health_score formula molecule are computed from —
// so the page, the dashboard tile, and the formula surface can never disagree.
func liveHealthInputs(app *appstate.App, now time.Time) healthscore.Inputs {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return engineenv.HealthInputs(engineenv.Data{
		Accounts: app.Accounts(), Transactions: app.Transactions(),
		Budgets: app.Budgets(), Categories: app.Categories(),
		WeekStart: uistate.LoadPrefs().WeekStartWeekday(),
		Rates:     currency.Rates{Base: base, Rates: app.Settings().FXRates},
		Now:       now,
	})
}

// healthHue maps a 0–100 score to a continuous red→amber→green hue (HSL), so the
// score ring shifts smoothly with the number rather than snapping at band edges.
// The band label carries the categorical meaning; the hue is the analog cue.
func healthHue(score int) int { return score * 13 / 10 } // 0 → 0 (red), 100 → 130 (green)

// healthColor is the ring/figure stroke for a score (a vivid HSL), or a calm dim
// grey for the "not enough data" state.
func healthColor(r healthscore.Result) string {
	if r.Band == healthscore.BandNoData {
		return "var(--text-dim)"
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
//
// The SVG geometry is delegated to the shared scoreRingNode helper; this wrapper
// is responsible only for deriving the health-specific color, figure text, aria
// label, and the BandNoData / no-data handling.
func healthRing(r healthscore.Result, size int) ui.Node {
	pct := float64(r.Score)
	if r.Band == healthscore.BandNoData {
		pct = 0
	}
	color := healthColor(r)
	figure := fmt.Sprintf("%d", r.Score)
	if r.Band == healthscore.BandNoData {
		figure = "—"
	}
	// R52/R64 a11y: the ring is the primary score visual, so give it a real
	// screen-reader name (role=img + a one-sentence label with the score and band)
	// rather than hiding it; the overlay number below is then aria-hidden so the
	// score isn't announced twice.
	ringLabel := uistate.T("health.ringLabel", r.Score, string(r.Band))
	if r.Band == healthscore.BandNoData {
		ringLabel = uistate.T("health.ringLabelNoData")
	}
	centerLabel := Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(healthTextTone(r.Band))),
		Style(map[string]string{"font-size": fmt.Sprintf("%dpx", size/3)}), figure)
	subLabel := Div(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "2px"}), "out of 100")
	return scoreRingNode(pct, color, size, ringLabel, centerLabel, subLabel)
}

// healthDeltaLine renders the "since last month" change chip from a prior score,
// or a calm baseline note when there's no earlier reading.
func healthDeltaLine(score int, prior int, hasPrior bool) ui.Node {
	if !hasPrior {
		return Div(css.Class("t-caption", tw.TextFaint), uistate.T("health.firstReading"))
	}
	d := score - prior
	switch {
	case d > 0:
		return Div(ClassStr("t-caption "+tw.ColorClass("text-up")), uistate.T("health.deltaUp", d))
	case d < 0:
		return Div(ClassStr("t-caption "+tw.ColorClass("text-down")), uistate.T("health.deltaDown", -d))
	default:
		return Div(css.Class("t-caption", tw.TextDim), uistate.T("health.deltaFlat"))
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
	in := liveHealthInputs(app, now)
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
	// Stable hook position (created every render, regardless of band) so the OnClick
	// below registers through UseEvent rather than a raw literal — keeps the hook
	// sequence consistent with the rest of the codebase.
	openSteps := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/health")) })

	var right ui.Node
	if r.Band == healthscore.BandNoData {
		right = Div(css.Class(tw.Flex1),
			Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass("text-dim")), string(healthscore.BandNoData)),
			P(css.Class("t-caption", tw.TextFaint, tw.Mt1), uistate.T("health.noDataHint")),
		)
	} else {
		weakLine := Fragment()
		if w, ok := weakestApplicable(r); ok {
			weakLine = Div(css.Class("t-caption", tw.TextDim, tw.Mt2),
				Span(css.Class(tw.TextFaint), uistate.T("health.weakestLabel")),
				Text(w.Label+" "+w.Value+" → "+w.Target))
		}
		right = Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
			Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(r.Band))), string(r.Band)),
			healthDeltaLine(r.Score, prior, hasPrior),
			weakLine,
			Div(css.Class(tw.Mt2),
				Button(css.Class("btn-link"), OnClick(openSteps), uistate.T("health.viewSteps"))),
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

// hltTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement ("1 / span 4" full-width, "span 2" for a half-width pair).
func hltTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// hltSection wraps a tile body with the shared serif section-title chrome.
func hltSection(sid, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if sid != "" {
		args = append(args, Attr("id", sid))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// healthFactorVarName mirrors engineenv's factor-key → variable mapping for the
// per-factor variable chips (the "addressable" identity of each factor).
func healthFactorVarName(key string) string {
	if key == "nw-trend" {
		return "health_trend"
	}
	return "health_" + key
}

// healthFactorTileProps drives one factor tile: the model factor plus its act
// route; its own component so the drill OnClick hook sits at a stable position.
type healthFactorTileProps struct {
	Factor healthscore.Factor
	Route  string
	OnOpen func()
}

// healthFactorTile renders one factor in depth: the current value vs its
// target, the 0–100 score meter, its exact contribution share, WHY the factor
// matters, the plain-English scoring curve, its live engine-variable identity,
// and an "Act on this" drill to the screen where the user improves it.
func healthFactorTile(p healthFactorTileProps) ui.Node {
	open := ui.UseEvent(Prevent(func() { p.OnOpen() }))
	f := p.Factor
	varName := healthFactorVarName(f.Key)

	if !f.Applicable {
		return hltSection("sec-hf-"+f.Key, f.Label, nil, Fragment(
			P(css.Class("empty"), Attr("data-testid", "hf-na-"+f.Key), uistate.T("health.notApplicable")),
			P(css.Class("muted"), uistate.T("health.f."+f.Key+".why")),
		))
	}

	// A zero score still renders a visible sliver so the meter reads as an
	// intentional "0", not an unloaded bar.
	meterPct := f.Score
	if meterPct == 0 {
		meterPct = 2
	}
	var act ui.Node = Fragment()
	if p.Route != "" {
		act = Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "hf-act-"+f.Key),
			Attr("aria-label", uistate.T("health.stepOpen", f.Label)), OnClick(open), uistate.T("health.act"))
	}
	// ONE number story per tile: the current value fused with its target into a
	// single met/unmet statement, and the meter as the only score visual. The
	// internal 0-100 score, the weight share, and the variable identity are
	// formula plumbing — they live inside the "How it's scored" disclosure.
	var targetLine ui.Node
	if f.TargetMet {
		targetLine = Span(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "hf-met-"+f.Key),
			uistate.T("health.onTarget", f.Target))
	} else {
		targetLine = Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "hf-unmet-"+f.Key),
			uistate.T("health.target", f.Target))
	}
	return hltSection("sec-hf-"+f.Key, f.Label, nil, Fragment(
		Div(css.Class("hlt-factor-head"),
			Span(ClassStr("hlt-factor-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(healthBandForScore(f.Score)))), f.Value),
			targetLine,
		),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: meterPct, Tone: healthBarTone(f.Score)}),
		P(css.Class("muted", tw.Mt2), uistate.T("health.f."+f.Key+".why")),
		Details(css.Class("hlt-curve"),
			Summary(uistate.T("health.curveSummary")),
			P(css.Class("t-caption", tw.TextFaint), uistate.T("health.f."+f.Key+".curve")),
			P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", f.Score, f.ContributionPct)),
			Div(css.Class("hlt-varchip"), Attr("data-testid", "hf-var-"+f.Key),
				Title(uistate.T("health.varChipTitle")),
				Code(varName), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", f.Score)),
			),
		),
		Div(css.Class("hlt-factor-foot"), Span(), act),
	))
}

// HealthScreen is the full /health page, a widgetized bento surface: the hero
// (score ring + band + delta + the score's FORMULA identity — the headline is
// the health_score molecule, auditable and even re-weightable under Formulas),
// six in-depth factor tiles (value vs target, meter, contribution, why, the
// scoring curve in plain English, the live variable chip, and an act drill),
// the prioritized focus-next steps, the monthly score history, and an opt-in
// FormulaBuilder. Everything is computed locally from the user's own data.
func HealthScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()

	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))

	now := time.Now()
	month := now.Format("2006-01")
	in := liveHealthInputs(app, now)
	r := healthscore.Evaluate(in)
	trend := uistate.UseHealthTrend().Get()
	prior, hasPrior := uistate.PriorHealthScore(trend, month)

	// The score's formula identity: the health_score molecule as persisted (a user
	// edit under Formulas travels here too — the page reads what the engine reads).
	scoreFormula := ""
	for _, m := range app.Molecules() {
		if m.Name == "health_score" {
			scoreFormula = m.Formula
			break
		}
	}

	// ── Hero tile: ring + band + delta + the formula identity. ──────────────────
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("health.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("health.metricsHide")
	}
	metricsBtn := Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
		Attr("data-testid", "health-toggle-formulas"), Title(uistate.T("health.metricsTitle")),
		OnClick(toggleFormulas), Text(metricsLabel))
	hero := hltTile("hlt-hero", "1 / span 4", hltSection("sec-health-hero", uistate.T("health.title"), metricsBtn,
		Fragment(
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5, tw.FlexWrap),
				healthRing(r, 150),
				Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
					Div(ClassStr("t-figure-lg "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(r.Band))), string(r.Band)),
					healthDeltaLine(r.Score, prior, hasPrior),
					If(r.NegativeCashFlow,
						Div(ClassStr("t-caption "+tw.ColorClass("text-down")), Style(map[string]string{"margin-top": "6px"}),
							uistate.T("health.deficitWarning"))),
				),
			),
			// The score IS a formula: the live molecule folds behind a quiet
			// disclosure so the hero stays glanceable, one click from the audit.
			If(scoreFormula != "", Details(css.Class("hlt-formula"), Attr("data-testid", "health-formula"),
				Summary(css.Class("t-caption", tw.TextDim), uistate.T("health.formulaTitle")),
				Code(css.Class("hlt-formula-code"), "health_score = "+scoreFormula),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.formulaNote")),
			)),
		)))

	// ── Factor tiles (span 2, three rows). ───────────────────────────────────────
	var tiles []ui.Node
	tiles = append(tiles, hero)
	for _, f := range r.Factors {
		route := healthStepRoute(f.Key)
		tiles = append(tiles, hltTile("hlt-"+f.Key, "span 2",
			ui.CreateElement(healthFactorTile, healthFactorTileProps{
				Factor: f, Route: route,
				OnOpen: func() {
					if route != "" {
						nav.Navigate(uistate.RoutePath(route))
					}
				},
			})))
	}

	// ── Focus-next steps (drill rows) + the privacy note. ───────────────────────
	if len(r.Steps) > 0 {
		stepNodes := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap3)}
		for _, s := range r.Steps {
			route := healthStepRoute(s.Key)
			stepNodes = append(stepNodes, ui.CreateElement(healthStepRow, healthStepRowProps{
				Factor: s.Factor, Action: s.Action, Target: s.Target, Route: route,
				OnOpen: func() {
					if route != "" {
						nav.Navigate(uistate.RoutePath(route))
					}
				},
			}))
		}
		tiles = append(tiles, hltTile("hlt-steps", "1 / span 4",
			hltSection("sec-health-steps", uistate.T("health.stepsTitle"), nil, Fragment(
				Div(stepNodes...),
				P(css.Class("t-caption", tw.TextFaint, tw.Mt2), uistate.T("health.privacy")),
			))))
	}

	// ── Monthly score history (when at least two snapshots exist). ──────────────
	if len(trend) >= 2 {
		vals := make([]float64, len(trend))
		labels := make([]string, len(trend))
		valueLabels := make([]string, len(trend))
		for i, s := range trend {
			vals[i] = float64(s.Score)
			if t, err := time.Parse("2006-01", s.Month); err == nil {
				labels[i] = t.Format("Jan")
			}
			valueLabels[i] = fmt.Sprintf("%d · %s", s.Score, s.Band)
		}
		delta := trend[len(trend)-1].Score - trend[0].Score
		takeaway := uistate.T("health.historyFlat", trend[len(trend)-1].Score)
		if delta > 0 {
			takeaway = uistate.T("health.historyUp", delta, len(trend))
		} else if delta < 0 {
			takeaway = uistate.T("health.historyDown", -delta, len(trend))
		}
		tiles = append(tiles, hltTile("hlt-history", "1 / span 4",
			hltSection("sec-health-history", uistate.T("health.historyTitle"), nil, Fragment(
				P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "health-history-takeaway"), takeaway),
				uiw.AreaChart(uiw.AreaChartProps{
					Values: vals, Stroke: chartLineColor(uistate.CurrentAccent()), GradientID: "hlt-history",
					Label: uistate.T("health.historyTitle"), Labels: labels, ValueLabels: valueLabels,
				}),
			))))
	}

	// ── Opt-in metrics tile: the score's variables in the FormulaBuilder. ────────
	if showFormulas.Get() {
		tiles = append(tiles, hltTile("hlt-formula-builder", "1 / span 4", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("health.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("health.metricsShow"), Initial: "health_score", ShowSaved: true}),
		)))
	}

	return Div(css.Class("bento bento-health"), tiles)
}

// healthStepRoute maps a health-factor key to the screen where the user acts on
// it, so a "Where to focus next" step becomes a one-click drill-down. Returns ""
// for keys with no natural destination (the step then renders non-clickable).
func healthStepRoute(key string) string {
	switch key {
	case "savings":
		return "/transactions" // trim spending — review where the money goes
	case "emergency":
		return "/goals" // build an emergency-fund goal
	case "debt":
		return "/debt" // the debt-payoff planner
	case "budget":
		return "/budgets" // bring over-budget categories back in line
	case "utilization":
		return "/credit" // pay down card balances
	case "nw-trend":
		return "/accounts" // net worth composes from accounts
	default:
		return ""
	}
}

type healthStepRowProps struct {
	Factor, Action, Target, Route string
	OnOpen                        func()
}

// healthStepRow renders one prioritized "focus next" step. When a Route exists
// the whole row is a button that drills to that screen (R52 decision→action); it
// is its own component so the OnClick hook sits at a stable position (steps render
// in a variable-length loop — the framework no-hooks-in-loops rule).
func healthStepRow(p healthStepRowProps) ui.Node {
	open := ui.UseEvent(Prevent(func() { p.OnOpen() }))
	body := Fragment(
		Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), p.Factor),
		Div(css.Class("t-caption", tw.TextDim), p.Action),
		Div(css.Class("t-caption", tw.TextFaint), uistate.T("health.targetLabel", p.Target)),
	)
	if p.Route == "" {
		return Div(css.Class("row", tw.Flex, tw.FlexCol, tw.Gap1), body)
	}
	return Button(css.Class("row", tw.Flex, tw.FlexCol, tw.Gap1, tw.TextLeft, tw.WFull, tw.HoverBgHover),
		Type("button"), Attr("data-testid", "health-step"),
		Attr("aria-label", uistate.T("health.stepOpen", p.Factor)),
		OnClick(open), body)
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
