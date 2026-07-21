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
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/resilience"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/vitals"
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

// healthFactorMeter maps a factor to a value-on-scale gauge: the FILL fraction (the
// current value's position on a sensible 0..1 scale) and the TICK fraction (the target's
// position on the same scale), so the meter's LENGTH encodes the value and a marker sits
// at the target — comparable across factors, unlike the old score-length bar where an
// on-target value drew a full bar. Lower-is-better factors (debt, utilization) place value
// and target on the same axis, so a longer bar past the tick reads as "deeper into the
// danger zone". ok=false when the factor has no numeric gauge (inapplicable).
func healthFactorMeter(key string, in healthscore.Inputs) (fill, tick float64, ok bool) {
	norm := func(v, min, max float64) float64 {
		if max <= min {
			return 0
		}
		f := (v - min) / (max - min)
		if f < 0 {
			return 0
		}
		if f > 1 {
			return 1
		}
		return f
	}
	switch key {
	case "savings":
		if !in.HasIncome {
			return 0, 0, false
		}
		return norm(float64(in.SavingsRatePct), 0, 40), norm(20, 0, 40), true
	case "emergency":
		if !in.HasLiquidData {
			return 0, 0, false
		}
		return norm(in.EmergencyMonths, 0, 6), norm(3, 0, 6), true
	case "debt":
		if !in.HasIncome {
			return 0, 0, false
		}
		if !in.HasLiabilities {
			return 0, norm(36, 0, 50), true // no debt → empty bar, target tick still shown
		}
		return norm(float64(in.ObligationRatioPct), 0, 50), norm(36, 0, 50), true
	case "budget":
		if !in.HasBudgets {
			return 0, 0, false
		}
		return norm(float64(in.BudgetAdherencePct), 0, 100), norm(100, 0, 100), true
	case "utilization":
		if !in.HasCredit {
			return 0, 0, false
		}
		return norm(float64(in.AggUtilizationPct), 0, 100), norm(30, 0, 100), true
	case "nw-trend":
		if !in.HasNWTrend {
			return 0, 0, false
		}
		return norm(in.NWTrendPct, -10, 10), norm(5, -10, 10), true
	}
	return 0, 0, false
}

// healthMeterBar renders a factor value meter: a track with a fill whose width is the
// value (fill 0..1) and a slim tick at the target (tick 0..1). Fill color is the factor's
// status tone (good/warn/bad by score), so bar LENGTH and COLOR carry complementary facts.
func healthMeterBar(fill, tick float64, score int, label string) ui.Node {
	tone := "is-bad"
	switch {
	case score >= 60:
		tone = "is-good"
	case score >= 40:
		tone = "is-warn"
	}
	return Div(css.Class("hlt-meter"),
		Attr("role", "meter"), Attr("aria-label", label),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"),
		Attr("aria-valuenow", fmt.Sprintf("%d", int(fill*100+0.5))),
		Div(ClassStr("hlt-meter-fill "+tone), Attr("aria-hidden", "true"),
			Style(map[string]string{"width": fmt.Sprintf("%.1f%%", fill*100)})),
		Div(css.Class("hlt-meter-tick"), Attr("aria-hidden", "true"),
			Title(uistate.T("health.meterTarget")),
			Style(map[string]string{"left": fmt.Sprintf("%.1f%%", tick*100)})),
	)
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
	subLabel := Div(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "2px"}), uistate.T("health.outOf100"))
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
			Div(css.Class(tw.Mt2, tw.Flex, tw.ItemsCenter, tw.Gap2),
				Button(css.Class("btn-link"), OnClick(openSteps), uistate.T("health.viewSteps")),
				// AG7: ask the assistant to explain how this score is derived.
				ExplainChip(ExplainChipProps{VarName: "health_score", Label: "health score"})),
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
// MolFormula maps molecule name → formula so the tile can show a factor's live
// composition (molecule = atoms) for the molecule-backed factors.
type healthFactorTileProps struct {
	Factor     healthscore.Factor
	Route      string
	OnOpen     func()
	MolFormula map[string]string
	// Value meter (task #17): MeterFill is the current value's position on a per-factor
	// 0..1 scale (the bar LENGTH), MeterTick the target's position on the same scale (the
	// marker). HasMeter is false for factors with no numeric gauge, so the tile falls back
	// to the score meter.
	MeterFill float64
	MeterTick float64
	HasMeter  bool
}

// healthFactorEq returns a factor's underlying value variable, the right-hand side
// of its composition, and whether that value variable is a real engine MOLECULE.
// Molecule-backed factors (savings_rate, credit_utilization, net_worth) show their
// live formula pulled from the engine; the rest are Go-scored on-device, so their
// composition is the conceptual expression over the atoms that feed it.
func healthFactorEq(key string, molF map[string]string) (lhs, rhs string, isMol bool) {
	switch key {
	case "savings":
		return "savings_rate", molF["savings_rate"], true
	case "utilization":
		return "credit_utilization", molF["credit_utilization"], true
	case "nw-trend":
		return "net_worth", molF["net_worth"], true
	case "emergency":
		return "health_emergency_months", "liquid_cash ÷ avg_monthly_spend", false
	case "debt":
		return "health_obligation_pct", "Σ minimum_payments ÷ monthly_income", false
	case "budget":
		return "health_budget", "budgets_within_limit ÷ total_budgets × 100", false
	}
	return healthFactorVarName(key), "", false
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

	// The factor meter (task #17): its LENGTH encodes the current VALUE on a sensible
	// per-factor scale with a tick at the target, so an on-target factor no longer draws
	// a full bar regardless of value. Falls back to the score meter when no numeric gauge
	// is available. A zero score still renders a visible sliver on the fallback so it reads
	// as an intentional "0", not an unloaded bar.
	meterPct := f.Score
	if meterPct == 0 {
		meterPct = 2
	}
	meterNode := uiw.ProgressBar(uiw.ProgressBarProps{Percent: meterPct, Tone: healthBarTone(f.Score)})
	if p.HasMeter {
		meterNode = healthMeterBar(p.MeterFill, p.MeterTick, f.Score, f.Label)
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
	// The factor's composition: value variable = its formula (a live molecule for the
	// molecule-backed factors, else the on-device expression over atoms).
	eqLHS, eqRHS, eqIsMol := healthFactorEq(f.Key, p.MolFormula)
	eqNote := uistate.T("health.derivedNote")
	if eqIsMol {
		eqNote = uistate.T("health.molNote")
	}
	return hltSection("sec-hf-"+f.Key, f.Label, nil, Fragment(
		Div(css.Class("hlt-factor-head"),
			Span(ClassStr("hlt-factor-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(healthBandForScore(f.Score)))), f.Value),
			targetLine,
		),
		// Name the measurement window on the factors that average it, so this "Savings
		// rate" isn't confused with the current-period savings_rate formula (review #3).
		If(f.Key == "savings", P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "hf-period-"+f.Key), uistate.T("healthx.savingsPeriod"))),
		meterNode,
		P(css.Class("muted", tw.Mt2), uistate.T("health.f."+f.Key+".why")),
		Details(css.Class("hlt-curve"),
			Summary(Attr("aria-label", uistate.T("healthx.curveAria", f.Label)), uistate.T("health.curveSummary")),
			// Composition: plain-language formula, then the actual equation — a live
			// molecule for the molecule-backed factors, else the on-device composition
			// over atoms — so "= atoms" is visible, not just an opaque variable name.
			Div(css.Class("hlt-detail"),
				Span(css.Class("hlt-detail-label"), uistate.T("health.formulaLabel")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.f."+f.Key+".formula")),
				If(eqRHS != "", Fragment(
					Code(css.Class("hlt-eq"), Attr("data-testid", "hf-eq-"+f.Key), eqLHS+" = "+eqRHS),
					Span(css.Class("hlt-eq-note", tw.TextFaint), eqNote),
				)),
			),
			// Scoring: the curve that maps the value to 0–100, plus this factor's live
			// score variable and its weight share of the overall number.
			Div(css.Class("hlt-detail"),
				Span(css.Class("hlt-detail-label"), uistate.T("health.scoringLabel")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.f."+f.Key+".curve")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", f.Score, f.ContributionPct)),
				Div(css.Class("hlt-varchip"), Attr("data-testid", "hf-var-"+f.Key),
					Title(uistate.T("health.varChipTitle")),
					Code(varName), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", f.Score)),
				),
			),
			// Example: a worked illustration of this factor's impact on the overall score.
			Div(css.Class("hlt-detail"),
				Span(css.Class("hlt-detail-label"), uistate.T("health.exampleLabel")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.f."+f.Key+".example")),
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
	// When the metrics workspace is revealed it mounts at the bottom of a long page —
	// scroll it into view so the toggle visibly does something (review finding #1).
	ui.UseEffect(func() func() {
		if showFormulas.Get() {
			smoothScrollToSection("sec-health-formulas")
		}
		return nil
	}, showFormulas.Get())

	now := time.Now()
	month := now.Format("2006-01")
	in := liveHealthInputs(app, now)
	r := healthscore.Evaluate(in)
	resIn := liveResilienceInput(app, now)
	baseCur := app.Settings().BaseCurrency
	if baseCur == "" {
		baseCur = "USD"
	}
	trend := uistate.UseHealthTrend().Get()
	prior, hasPrior := uistate.PriorHealthScore(trend, month)

	// The score's formula identity: the health_score molecule as persisted (a user
	// edit under Formulas travels here too — the page reads what the engine reads).
	scoreFormula := ""
	molF := make(map[string]string)
	for _, m := range app.Molecules() {
		molF[m.Name] = m.Formula
		if m.Name == "health_score" {
			scoreFormula = m.Formula
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
					// Resilience co-headline: how long the buffer covers everything with no
					// income — the forward-looking answer to "am I okay if something goes wrong?"
					If(resIn.MonthlySpend > 0,
						Div(ClassStr("t-caption "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass("text-dim")),
							Attr("data-testid", "health-runway-hero"), Style(map[string]string{"margin-top": "6px"}),
							uistate.T("healthx.runwayHero", fmtMonthsHuman(int(resilience.RunwayMonths(resIn)+0.0001))))),
					If(r.NegativeCashFlow,
						Div(ClassStr("t-caption "+tw.ColorClass("text-down")), Style(map[string]string{"margin-top": "6px"}),
							uistate.T("health.deficitWarning"))),
				),
			),
			// "Why this score?" — the per-factor point contributions as one segmented
			// bar, so the number is explained without opening the six disclosures.
			healthContribBar(r),
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
	// Stress tests up top (after the hero): the forward-looking "what if something
	// goes wrong" battery, interactive. A component so its own state hooks are isolated.
	if r.Band != healthscore.BandNoData {
		tiles = append(tiles, ui.CreateElement(healthStressTile, healthStressProps{In: resIn, Base: baseCur}))
	}
	for _, f := range r.Factors {
		route := healthStepRoute(f.Key)
		fill, tick, hasMeter := healthFactorMeter(f.Key, in)
		tiles = append(tiles, hltTile("hlt-"+f.Key, "span 2",
			ui.CreateElement(healthFactorTile, healthFactorTileProps{
				Factor: f, Route: route, MolFormula: molF,
				MeterFill: fill, MeterTick: tick, HasMeter: hasMeter,
				OnOpen: func() {
					if route != "" {
						nav.Navigate(uistate.RoutePath(route))
					}
				},
			})))
	}

	// ── Money leaks: recurring load + spending creep (backward-looking analysis). ─
	if r.Band != healthscore.BandNoData {
		tiles = append(tiles, ui.CreateElement(healthLeaksTile, healthLeaksProps{App: app}))
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
		// Narrate the series (direction + current streak + recovery), not just the
		// first-vs-last delta — "up three months running" reads truer than "+4 overall".
		scores := make([]int, len(trend))
		for i, s := range trend {
			scores[i] = s.Score
		}
		tr := vitals.Classify(scores)
		delta := tr.Delta
		takeaway := uistate.T("health.historyFlat", tr.Latest)
		switch {
		case tr.InflectedUp:
			takeaway = uistate.T("healthx.historyRecover", tr.StreakLen, tr.Latest)
		case tr.StreakDir == vitals.Rising && tr.StreakLen >= 2:
			takeaway = uistate.T("healthx.historyStreakUp", tr.StreakLen, tr.Latest)
		case tr.StreakDir == vitals.Falling && tr.StreakLen >= 2:
			takeaway = uistate.T("healthx.historyStreakDown", tr.StreakLen, tr.Latest)
		case delta > 0:
			takeaway = uistate.T("health.historyUp", delta, len(trend))
		case delta < 0:
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
		tiles = append(tiles, hltTile("hlt-formula-builder", "1 / span 4", Div(Attr("id", "sec-health-formulas"),
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
		return "/networth" // the dedicated net-worth page (assets vs liabilities over time)
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
	// A clickable step: lay the text beside a chevron so it reads as an action, not a
	// static note (review finding #8).
	return Button(css.Class("row", "hlt-step-row", tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.TextLeft, tw.WFull, tw.HoverBgHover),
		Type("button"), Attr("data-testid", "health-step"),
		Attr("aria-label", uistate.T("health.stepOpen", p.Factor)),
		OnClick(open),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), body),
		uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextFaint)),
	)
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
