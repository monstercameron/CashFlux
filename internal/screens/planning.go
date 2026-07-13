// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/planning"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/safespend"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// planTile wraps a tile body in the shared Widget chrome + the full-width bento column.
func planTile(id string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: id, Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true, Body: body,
	})
}

// planSection wraps a tile body with a serif section title + optional action, reusing the
// debt-section chrome so /planning matches /debt, /investments, and /allocate.
func planSection(id, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if id != "" {
		args = append(args, Attr("id", id))
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

// Planning hosts the debt-payoff calculator: enter a balance, APR, and monthly
// payment to see months-to-zero, total interest, and total paid (via the pure
// internal/payoff engine). The projection updates live as you type.
func Planning() ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}

	_ = uistate.UseDataRevision().Get()
	_ = uistate.UsePrefs().Get() // re-render when the accent/theme changes

	// Client-side navigation for the in-page links: a raw <a href> does a full page
	// reload, which drops the in-memory app-lock passcode and forces a re-unlock. These
	// keep the href for accessibility (focus, open-in-new-tab) but intercept the click.
	nav := router.UseNavigate()
	goRecurring := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/recurring")) }))
	goNetworth := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/networth")) }))
	goTransactions := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/transactions")) }))
	accent := chartLineColor(uistate.CurrentAccent())
	dec := currency.Decimals(base)
	cfg := uistate.PlanningConfigGet()
	seedMinor := func(m int64) string {
		if m > 0 {
			return money.FormatMinor(m, dec)
		}
		return ""
	}

	// showFormulas reveals the opt-in planning-metrics FormulaBuilder tile.
	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))

	trimStr := ui.UseState("")
	onTrim := ui.UseEvent(func(v string) { trimStr.Set(v) })
	// compareID is the ID of the saved plan whose projection curve is overlaid on
	// the 12-month forecast chart for side-by-side comparison (L27 enhancement).
	// Empty string means baseline-only (no overlay).
	compareID := ui.UseState("")
	onCompare := ui.UseEvent(func(e ui.Event) { compareID.Set(e.GetValue()) })
	// "Can I afford it?" — a purchase amount checked against projected cash (L8).
	afAmount := ui.UseState("")
	afMonths := ui.UseState("")
	afReserve := ui.UseState(seedMinor(cfg.AffordReserveMinor))
	onAfAmount := ui.UseEvent(func(v string) { afAmount.Set(v) })
	onAfMonths := ui.UseEvent(func(v string) { afMonths.Set(v) })
	onAfReserve := ui.UseEvent(func(v string) { afReserve.Set(v) })

	// Cash runway: a daily projection of liquid balance against scheduled recurring
	// cash flows, flagging the day it dips below a buffer (L13).
	rwBuffer := ui.UseState(seedMinor(cfg.RunwayBufferMinor))
	onRwBuffer := ui.UseEvent(func(v string) { rwBuffer.Set(v) })

	// Persist the runway buffer + affordability reserve so they survive a reload and feed the
	// runway_buffer engine variable. Silent (no data-revision bump) — a keyed effect.
	planPersistKey := rwBuffer.Get() + "|" + afReserve.Get()
	ui.UseEffect(func() func() {
		pc := uistate.PlanningConfigGet()
		pc.RunwayBufferMinor, _ = money.ParseMinor(strings.TrimSpace(rwBuffer.Get()), dec)
		pc.AffordReserveMinor, _ = money.ParseMinor(strings.TrimSpace(afReserve.Get()), dec)
		uistate.SetPlanningConfig(pc)
		return nil
	}, planPersistKey)

	// planAddOpen drives the "Add plan" FlipPanel modal (PlanAddForm). It is local state
	// (not a shell-root atom) so the trigger button and the modal share it directly and the
	// re-render is reliable; the FlipPanel renders as a sibling of the bento (not inside a
	// tile), so no tile transform breaks its position:fixed centring. Adds/deletes re-render
	// Planning() via UseDataRevision (subscribed above), so no manual revision counter.
	planAddOpen := ui.UseState(false)
	openPlanAdd := ui.UseEvent(Prevent(func() { planAddOpen.Set(true) }))
	closePlanAdd := func() { planAddOpen.Set(false) }
	deletePlan := func(pid string) {
		if app == nil {
			return
		}
		// A saved what-if scenario is user work — confirm before it's gone (was
		// an instant, unconfirmed delete; every other saved-artifact delete in
		// the app goes through ConfirmModal).
		name := uistate.T("planning.thisScenario")
		for _, p := range app.Plans() {
			if p.ID == pid && p.Name != "" {
				name = p.Name
				break
			}
		}
		uistate.ConfirmModal(uistate.T("planning.deleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			_ = app.DeletePlan(pid)
			uistate.BumpDataRevision()
		})
	}

	forecastCard := Fragment()
	if app != nil {
		accounts := app.Accounts()
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
		// Base the forecast on the average of the last 3 complete months, not just
		// this (possibly atypical) month — a one-off purchase or bonus shouldn't skew
		// the 12-month projection (L27).
		monthlyNet, _ := reports.TrailingMonthlyNet(txns, time.Now(), 3, rates)

		series := forecast.Project(net.Amount, []forecast.Recurring{{Label: "net cash flow", Monthly: monthlyNet}}, nil, 12)
		endVal := money.New(series[len(series)-1], base)

		// C170: warn if the projection dips below zero at any point in the horizon —
		// an end value alone hides a mid-horizon shortfall. Find the first month the
		// balance crosses negative (month 0 = now).
		dipMonth, dipsBelowZero := -1, false
		for i, v := range series {
			if v < 0 {
				dipMonth, dipsBelowZero = i, true
				break
			}
		}
		var dipWarning ui.Node = Fragment()
		if dipsBelowZero {
			when := time.Now().AddDate(0, dipMonth, 0).Format("Jan 2006")
			dipWarning = P(ClassStr("t-body "+tw.ColorClass("text-down")), Attr("data-testid", "forecast-dip-warning"),
				uistate.T("planning.forecastDip", when))
		}

		// Plot in major units (dollars) with a compact-currency axis (C16), and
		// overlay the trimmed scenario beside the baseline when a trim is set, so
		// the two net-worth curves can be compared directly (D10).
		toPoints := func(vals []int64) []chartspec.Point {
			pts := make([]chartspec.Point, len(vals))
			for i, v := range vals {
				pts[i] = chartspec.Point{X: float64(i), Y: currency.MajorFromMinor(v, base)}
			}
			return pts
		}
		yFmt := ".3~s"
		if currency.Symbol(base) == "$" {
			yFmt = "$.3~s"
		}
		chartSeries := []chartspec.Series{{Name: uistate.T("planning.seriesBaseline"), Color: accent, Points: toPoints(series)}}
		// Calendar X-axis labels (G7 §7 / L61): the chart's X values are month indices
		// 0..12, which read as opaque "0 · 2 · 4 …". Attach a real month label to each
		// baseline point ("Jul 2026"); chart.js renders point labels as X ticks when no
		// x.format is set, so Dev can answer "by when?" instead of "at which index?".
		forecastStart := time.Now()
		for i := range chartSeries[0].Points {
			chartSeries[0].Points[i].Label = forecastStart.AddDate(0, i, 0).Format("Jan 2006")
		}

		trimNote := Fragment()
		if trim, terr := money.ParseMinor(strings.TrimSpace(trimStr.Get()), currency.Decimals(base)); terr == nil && trim > 0 {
			series2 := forecast.Project(net.Amount, []forecast.Recurring{{Monthly: monthlyNet + trim}}, nil, 12)
			chartSeries = append(chartSeries, chartspec.Series{Name: uistate.T("planning.seriesTrim"), Color: "#cfa14e", Points: toPoints(series2)})
			end2 := series2[len(series2)-1]
			trimNote = P(css.Class("muted"), uistate.T("planning.trimNote",
				fmtMoney(money.New(trim, base)), fmtMoney(money.New(end2, base)), fmtMoney(money.New(end2-series[len(series)-1], base))))
		}

		// Side-by-side plan comparison overlay (L27 enhancement): when a saved plan is
		// selected in the compare-with picker, project its monthly change from the same
		// baseline and overlay its curve on the forecast chart in a distinct color.
		compareNote := Fragment()
		savedPlans := app.Plans()
		cid := compareID.Get()
		if cid != "" {
			for _, cp := range savedPlans {
				if cp.ID != cid {
					continue
				}
				cMonthly := planning.MonthlyNet(cp)
				cSeries := forecast.Project(net.Amount, []forecast.Recurring{{Monthly: cMonthly}}, nil, 12)
				chartSeries = append(chartSeries, chartspec.Series{
					Name:   cp.Name,
					Color:  "#7b68ee",
					Points: toPoints(cSeries),
				})
				cEnd := cSeries[len(cSeries)-1]
				baseEnd := series[len(series)-1]
				diff := cEnd - baseEnd
				compareNote = P(css.Class("muted"),
					Attr("data-testid", "plan-compare-note"),
					uistate.T("plans.compareNote", cp.Name,
						fmtMoney(money.New(cEnd, base)),
						fmtMoney(money.New(baseEnd, base)),
						fmtMoney(money.New(diff, base)),
					),
				)
				break
			}
		}

		// Build compare-with plan select options.
		compareOpts := []ui.Node{Option(Value(""), SelectedIf(cid == ""), uistate.T("plans.compareNone"))}
		for _, cp := range savedPlans {
			compareOpts = append(compareOpts, Option(Value(cp.ID), SelectedIf(cid == cp.ID), cp.Name))
		}

		spec := chartspec.Spec{
			Kind:   chartspec.Line,
			Series: chartSeries,
			Y:      chartspec.Axis{Format: yFmt},
			Legend: len(chartSeries) > 1,
		}
		planSmartSettings := uistate.LoadSmartSettings()
		forecastCard = planTile("plan-forecast", planSection("sec-forecast", uistate.T("planning.forecastTitle"), smartSectionAction(planSmartSettings),
			Fragment(
				// Headline answer (G7 §4/§5): surface the projected 12-month net worth as a
				// display-weight figure so Dev's primary question ("where will I be?") is
				// answerable at glance-speed, before parsing the chart or the hint sentence.
				Div(css.Class("stat-grid"),
					// Projected net worth is the key planning figure — tooltip explains the methodology.
					Div(css.Class("stat"),
						Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
							uistate.T("planning.projectedNetWorth"),
							smartTooltipFor(planSmartSettings, "planning-forecast", uistate.T("planning.projectedNetWorth"), uistate.T("smart.tipPlanningForecast")),
						),
						Div(ClassStr("stat-value is-hero "+accentFor(endVal)), fmtMoney(endVal)),
					),
					stat(uistate.T("planning.avgMonthlyNet"), fmtMoney(money.New(monthlyNet, base)), accentFor(money.New(monthlyNet, base))),
				),
				dipWarning,
				P(css.Class("muted"), uistate.T("planning.forecastHint", fmtMoney(money.New(monthlyNet, base)), fmtMoney(endVal))),
				P(css.Class("muted"), Attr("data-testid", "forecast-basis"), uistate.T("planning.forecastBasis")),
				uiw.Chart(uiw.ChartProps{Spec: spec, Height: "180px", Label: uistate.T("planning.forecastChartLabel", fmtMoney(endVal))}),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.trimLabel", base), Input(css.Class("field"), Type("number"), Value(trimStr.Get()), Step("0.01"), OnInput(onTrim))),
					If(len(savedPlans) > 0,
						// G7: compare-with is a secondary overlay action; compact class
						// keeps it visually subordinate to the primary trim input.
						Label(css.Class("field-label"), uistate.T("plans.compareLabel"),
							Select(css.Class("field", "plan-compare-select--compact"), Attr("aria-label", uistate.T("plans.compareLabel")),
								Attr("data-testid", "plan-compare-select"), OnChange(onCompare), compareOpts),
						),
					),
				),
				trimNote,
				compareNote,
			)))
	}

	// C141: compute safe-to-spend once at top-level so both the runway card (the
	// lead section per C168) and the affordability card share the same canonical
	// figure. Zero when app is nil (no accounts yet).
	planSafeToSpend := func() int64 {
		if app == nil {
			return 0
		}
		accts := app.Accounts()
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		liq, _ := ledger.LiquidBalance(accts, txns, rates)
		_, mEnd := dateutil.MonthRange(time.Now())
		toBase := safespend.ToBaseFunc(rates)
		billsDue := safespend.BillsDueBefore(accts, app.Recurring(), time.Now(), mEnd, toBase)
		goalNeeds := safespend.GoalContributionsProrated(app.Goals(), time.Now(), toBase)
		return safespend.Compute(liq.Amount, billsDue, goalNeeds, 0, base).SafeToSpend
	}()

	// Affordability check (L8): "can I afford $X (in N months, keeping a buffer)?"
	// projected from today's net worth and this month's net cash flow, via the pure
	// internal/afford engine — a deterministic answer, not an AI guess.
	affordCard := Fragment()
	if app != nil {
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		// C175/C141: safeStart comes from planSafeToSpend (liquid minus bills/goals)
		// so the "can I afford it?" card is consistent with the Safe to spend tile
		// at the top of the page. Monthly net is still used to project future months.
		safeStart := planSafeToSpend
		mStart, mEnd := dateutil.MonthRange(time.Now())
		income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
		monthlyNet := income.Amount - expense.Amount

		var afBody ui.Node = P(css.Class("muted"), uistate.T("planning.affordEnter"))
		if amt, aerr := money.ParseMinor(strings.TrimSpace(afAmount.Get()), currency.Decimals(base)); aerr == nil && amt > 0 {
			months, _ := strconv.Atoi(strings.TrimSpace(afMonths.Get()))
			reserved, _ := money.ParseMinor(strings.TrimSpace(afReserve.Get()), currency.Decimals(base))
			if reserved < 0 {
				reserved = 0
			}
			res := afford.CanAfford(amt, safeStart, monthlyNet, months, reserved)
			var verdict ui.Node
			if res.Affordable {
				verdict = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("planning.affordYes"))
			} else {
				when := uistate.T("planning.affordNever")
				if res.MonthsNeeded > 0 {
					when = uistate.T("planning.affordWhen", plural(res.MonthsNeeded, "month"))
				}
				verdict = Div(
					P(css.Class("err"), Attr("role", "alert"), uistate.T("planning.affordShort", fmtMoney(money.New(res.Shortfall, base)))),
					P(css.Class("muted"), when),
				)
			}
			afBody = Div(
				Div(css.Class("stat-grid"),
					stat(uistate.T("planning.affordProjected"), fmtMoney(money.New(res.ProjectedBalance, base)), ""),
					// C142: unified "Safe to spend" terminology — was "Free to spend".
					stat(uistate.T("planning.affordAvailableLabel"), fmtMoney(money.New(res.Available, base)), ""),
				),
				verdict,
			)
		}

		affordCard = planTile("plan-afford", planSection("sec-afford", uistate.T("planning.affordTitle"), Fragment(),
			Fragment(
				P(css.Class("muted"), uistate.T("planning.affordHint")),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.affordAmountPlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afAmount.Get()), Step("0.01"), OnInput(onAfAmount))),
					labeledField(uistate.T("planning.affordMonthsPlaceholder"), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afMonths.Get()), Step("1"), OnInput(onAfMonths))),
					labeledField(uistate.T("planning.affordReservePlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afReserve.Get()), Step("0.01"), OnInput(onAfReserve))),
				),
				afBody,
			)))
	}

	// Cash runway (L13): project liquid balance over the next 60 days against the
	// scheduled recurring cash flows (via internal/runway) and warn about the first
	// day it dips below the buffer — short-term liquidity, distinct from the 12-month
	// net-worth forecast above.
	runwayCard := Fragment()
	if app != nil {
		recs := app.Recurring()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		// C171: cash runway must start from LIQUID cash (checking/savings/cash), not
		// total assets — projecting against a 401(k)/home balance wildly overstates how
		// long money lasts against day-to-day bills.
		liquid, _ := ledger.LiquidBalance(app.Accounts(), app.Transactions(), rates)
		buffer, _ := money.ParseMinor(strings.TrimSpace(rwBuffer.Get()), currency.Decimals(base))
		if buffer < 0 {
			buffer = 0
		}
		const runwayDays = 60

		var rwBody ui.Node = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("planning.runwayEmpty"), CTALabel: uistate.T("recurring.add"), FocusID: "recurring-add"}) // C174: actionable empty-state
		if len(recs) > 0 {
			if proj, perr := runway.Project(liquid.Amount, recs, time.Now(), runwayDays, buffer, rates); perr == nil {
				lowTone := ""
				if proj.MinBalance < 0 {
					lowTone = "neg"
				}
				lowDate := time.Now().AddDate(0, 0, proj.MinDay).Format("Jan 2")
				// C169: payday-anchored balance — "what will my liquid cash be on my next
				// payday?" Anchors on the day-of-month derived from the configured pay-cycle
				// anchor (prefs.PayCycleAnchor). Omitted entirely when no anchor is set.
				var paydayStat ui.Node = Fragment()
				if curPrefs := uistate.LoadPrefs(); curPrefs.PayCycleAnchor != "" {
					if anchor, aerr := time.Parse("2006-01-02", curPrefs.PayCycleAnchor); aerr == nil {
						ph := runway.NextPaydayHorizon(time.Now(), anchor.Day(), runwayDays)
						paydayBal := cashflow.PaydayBalance(proj, ph)
						paydayDate := time.Now().AddDate(0, 0, ph).Format("Jan 2")
						payTone := ""
						if paydayBal < 0 {
							payTone = "neg"
						}
						paydayStat = stat(uistate.T("planning.paydayBalance", paydayDate), fmtMoney(money.New(paydayBal, base)), payTone)
					}
				}
				var verdict ui.Node
				if proj.WillBreach() {
					breachDate := time.Now().AddDate(0, 0, proj.BreachDay).Format("Jan 2")
					// Build the suggested-action line alongside the warning.
					sug := runway.SuggestCover(proj.BreachShortfall, app.Accounts(), app.Transactions(), rates)
					var sugNode ui.Node
					switch {
					case sug.Found && sug.AmountMinor >= proj.BreachShortfall:
						sugNode = Div(css.Class("runway-suggest"),
							Attr("data-testid", "runway-suggest"),
							Span(uistate.T("planning.runwaySuggest",
								fmtMoney(money.New(sug.AmountMinor, base)),
								sug.SourceName,
							)),
							A(css.Class("btn btn-sm"), Href(uistate.RoutePath("/transactions")), OnClick(goTransactions), uistate.T("planning.runwaySuggestAction")),
						)
					case sug.Found:
						sugNode = Div(css.Class("runway-suggest"),
							Attr("data-testid", "runway-suggest"),
							Span(uistate.T("planning.runwaySuggestPartial",
								fmtMoney(money.New(sug.AmountMinor, base)),
								sug.SourceName,
							)),
							A(css.Class("btn btn-sm"), Href(uistate.RoutePath("/transactions")), OnClick(goTransactions), uistate.T("planning.runwaySuggestAction")),
						)
					default:
						sugNode = P(css.Class("muted"),
							Attr("data-testid", "runway-suggest"),
							uistate.T("planning.runwaySuggestNone"),
						)
					}
					verdict = Div(
						P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "runway-breach"), uistate.T("planning.runwayBreach", breachDate, fmtMoney(money.New(proj.BreachShortfall, base)))),
						sugNode,
					)
				} else {
					verdict = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("planning.runwaySafe", runwayDays))
				}
				// C172: render the per-day balance curve (it was computed but never shown —
				// only summary stats were). A line over the 60-day horizon, X = day index.
				dayPts := make([]chartspec.Point, len(proj.Daily))
				rwStart := time.Now()
				for i, d := range proj.Daily {
					dayPts[i] = chartspec.Point{X: float64(d.Day), Y: currency.MajorFromMinor(d.Balance, base), Label: rwStart.AddDate(0, 0, d.Day).Format("Jan 2")}
				}
				rwYFmt := ".3~s"
				if currency.Symbol(base) == "$" {
					rwYFmt = "$.3~s"
				}
				rwSeries := []chartspec.Series{{Name: uistate.T("planning.runwayBalanceSeries"), Color: accent, Points: dayPts}}
				// When a warning buffer is set, overlay it as a flat amber threshold line so the
				// user can see where their floor sits against the projected balance — the input
				// now has a visible effect (the balance dipping under it is the breach).
				if buffer > 0 {
					bufMajor := currency.MajorFromMinor(buffer, base)
					bufPts := make([]chartspec.Point, len(dayPts))
					for i, dp := range dayPts {
						bufPts[i] = chartspec.Point{X: dp.X, Y: bufMajor, Label: dp.Label}
					}
					rwSeries = append(rwSeries, chartspec.Series{Name: uistate.T("planning.runwayBufferSeries"), Color: "#e0a93b", Points: bufPts})
				}
				rwSpec := chartspec.Spec{
					Kind:   chartspec.Line,
					Series: rwSeries,
					Y:      chartspec.Axis{Format: rwYFmt},
					Legend: len(rwSeries) > 1,
				}
				// C141: surface Safe to spend as the headline tile in the runway
				// section (which is now the page lead per C168), matching the
				// dashboard kpi-safetospend terminology exactly.
				s2sTone := ""
				if planSafeToSpend < 0 {
					s2sTone = "neg"
				}
				_ = s2sTone
				_ = lowDate
				rwBody = Div(
					// Safe-to-spend is the runway's headline (its bottom margin matches the grid
					// gutter so the gap to the secondary figures is uniform); the secondary stats
					// stretch across the row below.
					Div(css.Class("stat plan-runway-hero"),
						Div(css.Class("stat-label", tw.TextDim), uistate.T("planning.safeToSpend")),
						Div(ClassStr("stat-value is-hero "+tw.Fold(tw.FontDisplay)+" "+accentFor(money.New(planSafeToSpend, base))), fmtMoney(money.New(planSafeToSpend, base))),
					),
					Div(css.Class("stat-grid"),
						stat(uistate.T("planning.runwayStart"), fmtMoney(money.New(liquid.Amount, base)), ""),
						stat(uistate.T("planning.runwayLowLabel"), fmtMoney(money.New(proj.MinBalance, base)), lowTone),
						paydayStat,
					),
					verdict,
					// The daily balance curve over the horizon (date-labelled X axis).
					uiw.Chart(uiw.ChartProps{Spec: rwSpec, Height: "160px", Label: uistate.T("planning.runwayChartLabel")}),
				)
			}
		}

		runwayCard = planTile("plan-runway", planSection("sec-runway", uistate.T("planning.runwayTitle"), Fragment(),
			Fragment(
				P(css.Class("muted"), uistate.T("planning.runwayHint")),
				rwBody,
				// Controls after results (consistent with the other tiles); a compact,
				// placeholdered buffer input rather than a full-width empty box.
				Div(css.Class("plan-inline-field"),
					labeledField(uistate.T("planning.runwayBufferPlaceholder", base),
						Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder("500"), Value(rwBuffer.Get()), Step("0.01"), OnInput(onRwBuffer))),
				),
			)))
	}

	plansCard := Fragment()
	if app != nil {
		plans := app.Plans()
		// The add-plan form lives in a FlipPanel modal (PlanAddForm) opened from the
		// section-header "Add plan" button; the tile itself is just the scenario list.
		addAction := Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "plan-add-open"), OnClick(openPlanAdd), uistate.T("plans.add"))
		list := IfElse(len(plans) == 0,
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("plans.empty"), Icon: icon.Planning}),
			Div(css.Class("rows"), MapKeyed(plans,
				func(p domain.Plan) any { return p.ID },
				func(p domain.Plan) ui.Node {
					return ui.CreateElement(PlanRow, planRowProps{Plan: p, Base: base, OnDelete: deletePlan})
				},
			)),
		)
		plansCard = planTile("plan-scenarios", planSection("sec-scenarios", uistate.T("plans.title"), addAction, list))
	}

	// C168: lead with the near-term liquid cash-flow section (runway/safe-to-spend),
	// not the 12-month net-worth chart — the payday question is more immediately
	// actionable. forecastCard is demoted below affordability and runway.
	// The recurring cash-flow manager lives at /recurring (RecurringManagerPanel in
	// recurring.go, FEATURE_MAP §5.7a) — it is not embedded here.
	// Toolbar tile: a plan-metrics toggle + a link to the recurring cash-flow manager (which
	// owns the schedule the runway projects against).
	metricsCls := "strip-toggle"
	if showFormulas.Get() {
		metricsCls += " is-on"
	}
	toolbar := planTile("plan-toolbar", Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
				Attr("data-testid", "planning-toggle-formulas"), Title(uistate.T("planning.metricsTitle")),
				OnClick(toggleFormulas), Text(planMetricsLabel(showFormulas.Get()))),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/recurring")), OnClick(goRecurring), uistate.T("planning.manageRecurring")),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/networth")), OnClick(goNetworth), uistate.T("debt.linkNetWorth")),
		),
	))

	// C168: lead with the near-term liquid runway, then affordability, the 12-month forecast,
	// and the saved what-if scenarios. Widgetized bento surface (like /debt, /investments).
	tiles := []ui.Node{toolbar, runwayCard, affordCard, forecastCard, plansCard}
	if showFormulas.Get() {
		tiles = append(tiles, planTile("plan-formula", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("planning.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("planning.metricsTitle"), ShowSaved: true}),
		)))
	}
	// The add-scenario form lives in a FlipPanel modal rendered as a sibling of the bento
	// (not inside a tile) so no tile transform breaks its position:fixed centring. It mounts
	// only while open; OnClose / the form's OnDone clear planAddOpen to dismiss it.
	var addModal ui.Node = Fragment()
	if planAddOpen.Get() {
		addModal = uiw.FlipPanel(uiw.FlipPanelProps{
			Title:    uistate.T("plans.addTitle"),
			Width:    uiw.FlipMediumW, // standard entity-add width
			Height:   "min(90vh, 520px)",
			NoFooter: true,
			OnClose:  closePlanAdd,
			Back:     ui.CreateElement(PlanAddForm, PlanAddFormProps{OnDone: closePlanAdd}),
		})
	}
	return Fragment(Div(css.Class("bento bento-planning"), tiles), addModal)
}

// planMetricsLabel is the plan-metrics toggle label.
func planMetricsLabel(on bool) string {
	if on {
		return uistate.T("planning.metricsHide")
	}
	return uistate.T("planning.metricsShow")
}

type recurringRowProps struct {
	Recurring  domain.Recurring
	Accounts   []domain.Account
	Categories []domain.Category
	Base       string
	OnDelete   func(string)
	OnSave     func(domain.Recurring) // C153: persist an inline edit
}

// RecurringRow renders one recurring cash flow (amount colored by sign) with
// inline edit + remove. It owns its own hooks (per the no-hooks-in-loops rule);
// all hooks are declared unconditionally so the edit toggle never reorders them.
func RecurringRow(props recurringRowProps) ui.Node {
	r := props.Recurring
	editing := ui.UseState(false)
	labelS := ui.UseState(r.Label)
	amountS := ui.UseState(money.FormatMinor(r.Amount.Abs().Amount, currency.Decimals(r.Amount.Currency)))
	cadenceS := ui.UseState(string(r.Cadence))
	accountS := ui.UseState(r.AccountID)
	categoryS := ui.UseState(r.CategoryID)
	autopayS := ui.UseState(r.Autopay)
	expenseS := ui.UseState(r.Amount.IsNegative()) // preserve money-out vs money-in
	onLabel := ui.UseEvent(func(v string) { labelS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onCadence := ui.UseEvent(func(e ui.Event) { cadenceS.Set(e.GetValue()) })
	onAccount := ui.UseEvent(func(e ui.Event) { accountS.Set(e.GetValue()) })
	onCategory := ui.UseEvent(func(e ui.Event) { categoryS.Set(e.GetValue()) })
	del := ui.UseEvent(Prevent(func() { props.OnDelete(r.ID) }))
	startEdit := ui.UseEvent(Prevent(func() {
		labelS.Set(r.Label)
		amountS.Set(money.FormatMinor(r.Amount.Abs().Amount, currency.Decimals(r.Amount.Currency)))
		cadenceS.Set(string(r.Cadence))
		accountS.Set(r.AccountID)
		categoryS.Set(r.CategoryID)
		autopayS.Set(r.Autopay)
		expenseS.Set(r.Amount.IsNegative())
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	base := props.Base
	if base == "" {
		base = r.Amount.Currency
	}
	saveEdit := ui.UseEvent(Prevent(func() {
		amt, err := money.ParseMinor(strings.TrimSpace(amountS.Get()), currency.Decimals(base))
		if err != nil || amt == 0 {
			return // invalid amount — keep the editor open
		}
		if expenseS.Get() {
			amt = -amt
		}
		props.OnSave(domain.Recurring{
			ID: r.ID, Label: strings.TrimSpace(labelS.Get()), Amount: money.New(amt, base),
			Cadence: domain.RecurringCadence(cadenceS.Get()), NextDue: r.NextDue,
			AccountID: accountS.Get(), CategoryID: categoryS.Get(),
			Autopost: r.Autopost, Autopay: autopayS.Get(),
		})
		editing.Set(false)
	}))

	if editing.Get() {
		cadOpts := []ui.Node{
			Option(Value(string(domain.CadenceWeekly)), SelectedIf(cadenceS.Get() == string(domain.CadenceWeekly)), uistate.T("recurring.cadenceWeekly")),
			Option(Value(string(domain.CadenceBiweekly)), SelectedIf(cadenceS.Get() == string(domain.CadenceBiweekly)), uistate.T("recurring.cadenceBiweekly")),
			Option(Value(string(domain.CadenceMonthly)), SelectedIf(cadenceS.Get() == string(domain.CadenceMonthly)), uistate.T("recurring.cadenceMonthly")),
			Option(Value(string(domain.CadenceSemimonthly)), SelectedIf(cadenceS.Get() == string(domain.CadenceSemimonthly)), uistate.T("recurring.cadenceSemimonthly")),
			Option(Value(string(domain.CadenceQuarterly)), SelectedIf(cadenceS.Get() == string(domain.CadenceQuarterly)), uistate.T("recurring.cadenceQuarterly")),
			Option(Value(string(domain.CadenceYearly)), SelectedIf(cadenceS.Get() == string(domain.CadenceYearly)), uistate.T("recurring.cadenceYearly")),
		}
		acctOpts := []ui.Node{Option(Value(""), SelectedIf(accountS.Get() == ""), uistate.T("recurring.noAccount"))}
		for _, a := range props.Accounts {
			acctOpts = append(acctOpts, Option(Value(a.ID), SelectedIf(accountS.Get() == a.ID), a.Name))
		}
		catOpts := []ui.Node{Option(Value(""), SelectedIf(categoryS.Get() == ""), uistate.T("recurring.noCategory"))}
		for _, c := range props.Categories {
			catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(categoryS.Get() == c.ID), c.Name))
		}
		return Div(css.Class("row row-edit"),
			Form(css.Class("form-grid"), Attr("data-testid", "recurring-edit-"+r.ID), OnSubmit(saveEdit),
				labeledField(uistate.T("recurring.labelPlaceholder"),
					Input(css.Class("field"), Type("text"), Value(labelS.Get()), OnInput(onLabel))),
				labeledField(uistate.T("recurring.amountPlaceholder", base),
					Input(css.Class("field"), Type("number"), Step("0.01"), Value(amountS.Get()), OnInput(onAmount))),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.cadence")), OnChange(onCadence), cadOpts),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.account")), OnChange(onAccount), acctOpts),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.category")), OnChange(onCategory), catOpts),
				uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopay"), On: autopayS.Get(), OnChange: func(v bool) { autopayS.Set(v) }}),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	meta := cadenceLabel(r.Cadence) + " · " + uistate.T("recurring.nextDue", uistate.LoadPrefs().FormatDate(r.NextDue)) // C155: respect date-format preference
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), r.Label),
			Span(css.Class("row-meta"), meta),
			// C157: surface autopay so the user knows the biller charges this
			// automatically (no manual payment needed — just keep funds available).
			If(r.Autopay, Span(css.Class("pill", tw.TextDim), Attr("data-testid", "recurring-autopay"), Attr("title", uistate.T("recurring.autopayHint")), uistate.T("recurring.autopayBadge"))),
		),
		Span(ClassStr(amountClass(r.Amount)), fmtMoney(r.Amount)),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("aria-label", uistate.T("recurring.editTitle")), Title(uistate.T("recurring.editTitle")), Attr("data-testid", "recurring-edit-btn"), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("recurring.deleteTitle")), Title(uistate.T("recurring.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

type planRowProps struct {
	Plan     domain.Plan
	Base     string
	OnDelete func(string)
}

// PlanRow renders one saved what-if plan: its name, the horizon/start/monthly
// assumptions, the projected end-of-horizon balance, and — when the projected
// balance crosses zero within the horizon — a runway readout ("Money lasts ~N
// months") in the danger tone plus a danger badge. Its own component per the
// no-hooks-in-loops rule.
func PlanRow(props planRowProps) ui.Node {
	p := props.Plan
	end := money.New(planning.EndBalance(p), props.Base)
	monthly := money.New(planning.MonthlyNet(p), props.Base)
	meta := uistate.T("plans.rowMeta", p.HorizonMonths, fmtMoney(money.New(p.StartBalance, props.Base)), fmtMoney(monthly))

	// A compact sparkline of the projected balance over the horizon, toned by
	// whether the plan ends up or down vs. its starting balance.
	curve := planning.Project(p)
	vals := make([]float64, len(curve))
	for i, v := range curve {
		vals[i] = float64(v)
	}
	// Tone the projected figure + sparkline by whether the plan ends up (accent) or down (red)
	// vs. its starting balance.
	up := planning.EndBalance(p) >= p.StartBalance
	endToneCls := "text-up"
	stroke := chartLineColor(uistate.CurrentAccent())
	if !up {
		endToneCls = "text-down"
		stroke = "#d8716f"
	}

	// Delete handler for the ⋯ overflow menu (a stable hook position — not in a loop).
	del := ui.UseEvent(Prevent(func() { props.OnDelete(p.ID) }))

	// Runway readout: how long does the balance last before crossing zero?
	runwayMo, depletes := planning.RunwayMonths(p)
	var runwayNode ui.Node = Fragment()
	if depletes {
		runwayNode = Span(css.Class("plan-scenario-runway is-danger plan-runway--danger"), Attr("role", "status"), Attr("aria-label", uistate.T("plans.runwayDanger")),
			Span(css.Class("plan-runway__badge"), "⚠"),
			Span(css.Class("plan-runway__text text-down"), uistate.T("plans.runwayMonths", strconv.FormatFloat(runwayMo, 'f', 1, 64))),
		)
	} else if p.HorizonMonths > 0 {
		runwayNode = Span(css.Class("plan-scenario-runway is-ok", tw.TextDim), uistate.T("plans.staysPositive", p.HorizonMonths))
	}

	return Div(css.Class("plan-scenario"), Attr("data-testid", "plan-"+p.ID), Attr("role", "listitem"),
		Div(css.Class("plan-scenario-head"),
			Div(css.Class("plan-scenario-title"),
				Span(css.Class("plan-scenario-name"), p.Name),
				Span(css.Class("plan-scenario-meta", tw.TextDim), meta),
			),
			Div(css.Class("plan-scenario-figs"),
				Span(ClassStr("plan-scenario-end "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(endToneCls)), fmtMoney(end)),
				runwayNode,
			),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "plan-menu-" + p.ID,
				AriaLabel:    uistate.T("plans.moreActions"),
				ToggleTestID: "plan-menu-" + p.ID,
				WrapClass:    "plan-scenario-menu",
				Items: []ui.Node{
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
						Attr("data-testid", "plan-del-"+p.ID), Title(uistate.T("plans.deleteTitle")),
						OnClick(del), uistate.T("plans.delete")),
				},
			}),
		),
		If(len(vals) > 1, Div(css.Class("plan-scenario-chart"),
			uiw.AreaChart(uiw.AreaChartProps{Values: vals, Stroke: stroke, GradientID: "cf-plan-" + p.ID, Label: uistate.T("plans.chartLabel", fmtMoney(end))}),
		)),
	)
}

// detectedRecurringRowProps drives one auto-detected recurring charge row (C147).
type detectedRecurringRowProps struct {
	Name    string
	Monthly string // pre-formatted "~$X/mo · Monthly" descriptor
	OnAdd   func()
}

// detectedRecurringRow renders one auto-detected recurring charge with an
// "Add to plan" button. It is its own component so the button's OnClick hook sits
// at a stable render position — the detected list is variable-length (the
// framework no-hooks-in-loops gotcha).
func detectedRecurringRow(props detectedRecurringRowProps) ui.Node {
	add := ui.UseEvent(Prevent(func() { props.OnAdd() }))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Name),
			Span(css.Class("row-meta"), props.Monthly),
		),
		Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "detected-add"),
			Attr("aria-label", uistate.T("recurring.addToPlanAria", props.Name)),
			Title(uistate.T("recurring.addToPlan")), OnClick(add), uistate.T("recurring.addToPlan")),
	)
}

// cadenceLabel localizes a recurring cadence.
func cadenceLabel(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return uistate.T("recurring.cadenceWeekly")
	case domain.CadenceBiweekly:
		return uistate.T("recurring.cadenceBiweekly")
	case domain.CadenceSemimonthly:
		return uistate.T("recurring.cadenceSemimonthly")
	case domain.CadenceQuarterly:
		return uistate.T("recurring.cadenceQuarterly")
	case domain.CadenceYearly:
		return uistate.T("recurring.cadenceYearly")
	default:
		return uistate.T("recurring.cadenceMonthly")
	}
}
