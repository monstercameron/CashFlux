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
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/planning"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/safespend"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

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
	afReserve := ui.UseState("")
	onAfAmount := ui.UseEvent(func(v string) { afAmount.Set(v) })
	onAfMonths := ui.UseEvent(func(v string) { afMonths.Set(v) })
	onAfReserve := ui.UseEvent(func(v string) { afReserve.Set(v) })

	// Cash runway: a daily projection of liquid balance against scheduled recurring
	// cash flows, flagging the day it dips below a buffer (L13).
	rwBuffer := ui.UseState("")
	onRwBuffer := ui.UseEvent(func(v string) { rwBuffer.Set(v) })

	// plRev triggers a re-render of Planning() when plans are added or removed,
	// causing app.Plans() to be re-read without explicit dependency tracking.
	plRev := ui.UseState(0)

	// What-if plans: a starting balance projected over a horizon under a steady
	// monthly change (a recurring PlanItem). Persisted via appstate, projected
	// through internal/planning.
	plName := ui.UseState("")
	plHorizon := ui.UseState("12")
	plStart := ui.UseState("")
	// plAccount prefills the starting balance from a chosen account's current
	// balance; selecting one overwrites plStart with the account's balance in
	// minor units formatted as a major-unit decimal (L27 enhancement).
	plAccount := ui.UseState("")
	plMonthly := ui.UseState("")
	plOnceAmt := ui.UseState("")
	plOnceMonth := ui.UseState("")
	plErr := ui.UseState("")
	onPlName := ui.UseEvent(func(v string) { plName.Set(v) })
	onPlHorizon := ui.UseEvent(func(v string) { plHorizon.Set(v) })
	onPlStart := ui.UseEvent(func(v string) { plStart.Set(v) })
	onPlMonthly := ui.UseEvent(func(v string) { plMonthly.Set(v) })
	onPlOnceAmt := ui.UseEvent(func(v string) { plOnceAmt.Set(v) })
	onPlOnceMonth := ui.UseEvent(func(v string) { plOnceMonth.Set(v) })
	// onPlAccount prefills the starting balance from a chosen account's balance.
	// Selecting an account calculates its current balance via the ledger and sets
	// plStart so the user doesn't need to look it up manually (L27 enhancement).
	onPlAccount := ui.UseEvent(func(e ui.Event) {
		aid := e.GetValue()
		plAccount.Set(aid)
		if app == nil || aid == "" {
			return
		}
		for _, a := range app.Accounts() {
			if a.ID != aid {
				continue
			}
			if bal, err := ledger.Balance(a, app.Transactions()); err == nil {
				plStart.Set(money.FormatMinor(bal.Abs().Amount, currency.Decimals(a.Currency)))
			}
			return
		}
	})
	addPlan := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		name := strings.TrimSpace(plName.Get())
		if name == "" {
			plErr.Set(uistate.T("plans.nameRequired"))
			return
		}
		horizon, herr := strconv.Atoi(strings.TrimSpace(plHorizon.Get()))
		if herr != nil || horizon <= 0 {
			plErr.Set(uistate.T("plans.horizonRequired"))
			return
		}
		// Start balance and monthly change are optional; blank means 0.
		start, _ := money.ParseMinor(strings.TrimSpace(plStart.Get()), currency.Decimals(base))
		monthly, _ := money.ParseMinor(strings.TrimSpace(plMonthly.Get()), currency.Decimals(base))
		p := domain.Plan{ID: id.New(), Name: name, HorizonMonths: horizon, StartBalance: start}
		if monthly != 0 {
			p.Items = append(p.Items, domain.PlanItem{
				ID: id.New(), Label: uistate.T("plans.monthlyLabel"), Kind: domain.PlanItemRecurring, Amount: monthly,
			})
		}
		// Optional one-time amount in a chosen month (e.g. a bonus or big expense).
		// Only added when both an amount and an in-horizon month are given.
		onceAmt, _ := money.ParseMinor(strings.TrimSpace(plOnceAmt.Get()), currency.Decimals(base))
		onceMonth, monthErr := strconv.Atoi(strings.TrimSpace(plOnceMonth.Get()))
		if onceAmt != 0 && strings.TrimSpace(plOnceMonth.Get()) != "" {
			if monthErr != nil || onceMonth < 1 || onceMonth > horizon {
				plErr.Set(uistate.T("plans.onceMonthRange"))
				return
			}
			p.Items = append(p.Items, domain.PlanItem{
				ID: id.New(), Label: uistate.T("plans.onceLabel"), Kind: domain.PlanItemOneTime, Amount: onceAmt, Month: onceMonth,
			})
		}
		if err := app.PutPlan(p); err != nil {
			plErr.Set(err.Error())
			return
		}
		plName.Set("")
		plStart.Set("")
		plMonthly.Set("")
		plOnceAmt.Set("")
		plOnceMonth.Set("")
		plErr.Set("")
		plRev.Set(plRev.Get() + 1)
	}))
	deletePlan := func(pid string) {
		if app != nil {
			_ = app.DeletePlan(pid)
			plRev.Set(plRev.Get() + 1)
		}
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
		chartSeries := []chartspec.Series{{Name: uistate.T("planning.seriesBaseline"), Points: toPoints(series)}}
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
		forecastCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title:        uistate.T("planning.forecastTitle"),
			HeaderAction: smartSectionAction(planSmartSettings),
			Body: Fragment(
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
			),
		})
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

		affordCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.affordTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("planning.affordHint")),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.affordAmountPlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afAmount.Get()), Step("0.01"), OnInput(onAfAmount))),
					labeledField(uistate.T("planning.affordMonthsPlaceholder"), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afMonths.Get()), Step("1"), OnInput(onAfMonths))),
					labeledField(uistate.T("planning.affordReservePlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(afReserve.Get()), Step("0.01"), OnInput(onAfReserve))),
				),
				afBody,
			),
		})
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
							A(css.Class("btn btn-sm"), Href("/transactions"), uistate.T("planning.runwaySuggestAction")),
						)
					case sug.Found:
						sugNode = Div(css.Class("runway-suggest"),
							Attr("data-testid", "runway-suggest"),
							Span(uistate.T("planning.runwaySuggestPartial",
								fmtMoney(money.New(sug.AmountMinor, base)),
								sug.SourceName,
							)),
							A(css.Class("btn btn-sm"), Href("/transactions"), uistate.T("planning.runwaySuggestAction")),
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
				for i, d := range proj.Daily {
					dayPts[i] = chartspec.Point{X: float64(d.Day), Y: currency.MajorFromMinor(d.Balance, base)}
				}
				rwYFmt := ".3~s"
				if currency.Symbol(base) == "$" {
					rwYFmt = "$.3~s"
				}
				rwSpec := chartspec.Spec{
					Kind:   chartspec.Line,
					Series: []chartspec.Series{{Name: uistate.T("planning.runwayTitle"), Points: dayPts}},
					Y:      chartspec.Axis{Format: rwYFmt},
				}
				// C141: surface Safe to spend as the headline tile in the runway
				// section (which is now the page lead per C168), matching the
				// dashboard kpi-safetospend terminology exactly.
				s2sTone := ""
				if planSafeToSpend < 0 {
					s2sTone = "neg"
				}
				rwBody = Div(
					Div(css.Class("stat-grid"),
						stat(uistate.T("planning.safeToSpend"), fmtMoney(money.New(planSafeToSpend, base)), s2sTone),
						stat(uistate.T("planning.runwayStart"), fmtMoney(money.New(liquid.Amount, base)), ""),
						stat(uistate.T("planning.runwayLowLabel"), fmtMoney(money.New(proj.MinBalance, base)), lowTone),
						paydayStat,
					),
					verdict,
					// C173: the low-point line carries the date; tone it (danger) when the
					// balance actually dips negative so it reads as a warning, not a muted
					// footnote. Stays muted when the low-point is comfortably positive.
					P(ClassStr(runwayLowClass(lowTone)), uistate.T("planning.runwayLow", fmtMoney(money.New(proj.MinBalance, base)), lowDate)),
					// C172: the daily balance curve over the horizon.
					uiw.Chart(uiw.ChartProps{Spec: rwSpec, Height: "160px", Label: uistate.T("planning.runwayChartLabel")}),
				)
			}
		}

		runwayCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.runwayTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("planning.runwayHint")),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.runwayBufferPlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(rwBuffer.Get()), Step("0.01"), OnInput(onRwBuffer))),
				),
				rwBody,
			),
		})
	}

	plansCard := Fragment()
	if app != nil {
		plans := app.Plans()
		// Account prefill options: any non-archived account lets the user seed the
		// plan's starting balance from its current ledger balance (L27 enhancement).
		plAcctOpts := []ui.Node{Option(Value(""), SelectedIf(plAccount.Get() == ""), uistate.T("plans.prefillNone"))}
		for _, a := range app.Accounts() {
			if a.Archived {
				continue
			}
			plAcctOpts = append(plAcctOpts, Option(Value(a.ID), SelectedIf(plAccount.Get() == a.ID), a.Name))
		}
		list := IfElse(len(plans) == 0,
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("plans.empty"), CTALabel: uistate.T("plans.add"), FocusID: "plan-add"}),
			Div(css.Class("rows"), MapKeyed(plans,
				func(p domain.Plan) any { return p.ID },
				func(p domain.Plan) ui.Node {
					return ui.CreateElement(PlanRow, planRowProps{Plan: p, Base: base, OnDelete: deletePlan})
				},
			)),
		)
		plansCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("plans.title"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("plans.hint")),
				Form(css.Class("form-grid"), OnSubmit(addPlan),
					Input(append([]any{css.Class("field"), Attr("id", "plan-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("plans.namePlaceholder")), Value(plName.Get()), OnInput(onPlName)}, errAttrs("plan-err", plErr.Get())...)...),
					labeledField(uistate.T("plans.horizonPlaceholder"), Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("aria-required", "true"), Value(plHorizon.Get()), Step("1"), OnInput(onPlHorizon))),
					// Account prefill: selecting an account fills the start-balance input
					// from that account's current balance so the user doesn't look it up.
					Label(css.Class("field-label"), uistate.T("plans.prefillAccount"),
						Select(css.Class("field"), Attr("aria-label", uistate.T("plans.prefillAccount")),
							Attr("data-testid", "plan-prefill-account"), OnChange(onPlAccount), plAcctOpts),
					),
					labeledField(uistate.T("plans.startPlaceholder", base), Input(css.Class("field"), Type("number"), Value(plStart.Get()), Step("0.01"), OnInput(onPlStart))),
					labeledField(uistate.T("plans.monthlyPlaceholder", base), Input(css.Class("field"), Type("number"), Value(plMonthly.Get()), Step("0.01"), OnInput(onPlMonthly))),
					labeledField(uistate.T("plans.onceAmtPlaceholder", base), Input(css.Class("field"), Type("number"), Value(plOnceAmt.Get()), Step("0.01"), OnInput(onPlOnceAmt))),
					labeledField(uistate.T("plans.onceMonthPlaceholder"), Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", plHorizon.Get()), Value(plOnceMonth.Get()), Step("1"), OnInput(onPlOnceMonth))),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("plans.add")),
				),
				errText("plan-err", plErr.Get()),
				list,
			),
		})
	}

	// C168: lead with the near-term liquid cash-flow section (runway/safe-to-spend),
	// not the 12-month net-worth chart — the payday question is more immediately
	// actionable. forecastCard is demoted below affordability and runway.
	// The recurring cash-flow manager lives at /recurring (RecurringManagerPanel in
	// recurring.go, FEATURE_MAP §5.7a) — it is not embedded here.
	return Div(
		runwayCard,
		affordCard,
		forecastCard,
		plansCard,
	)
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
	del := ui.UseEvent(Prevent(func() { props.OnDelete(p.ID) }))
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
	stroke := "#2e8b57"
	if planning.EndBalance(p) < p.StartBalance {
		stroke = "#d8716f"
	}

	// Runway readout: how long does the balance last before crossing zero?
	runwayMo, depletes := planning.RunwayMonths(p)

	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), p.Name),
			Span(css.Class("row-meta"), meta),
		),
		If(len(vals) > 1, uiw.AreaChart(uiw.AreaChartProps{
			Values: vals, Stroke: stroke, GradientID: "cf-plan-" + p.ID,
			Width: 120, Height: 28, Label: uistate.T("plans.chartLabel", fmtMoney(end)),
		})),
		Span(ClassStr("amount fig "+figTone(end)), uistate.T("plans.projected", fmtMoney(end))),
		// Runway indicator: shown only when the balance depletes within the horizon.
		// Uses both colour (text-down) and text so the warning is not colour-alone (a11y).
		IfElse(depletes,
			Span(
				css.Class("plan-runway plan-runway--danger"),
				Attr("role", "status"),
				Attr("aria-label", uistate.T("plans.runwayDanger")),
				Span(css.Class("plan-runway__badge"), "⚠"),
				Span(
					css.Class("plan-runway__text text-down"),
					uistate.T("plans.runwayMonths", strconv.FormatFloat(runwayMo, 'f', 1, 64)),
				),
			),
			If(p.HorizonMonths > 0,
				Span(
					css.Class("plan-runway plan-runway--ok"),
					uistate.T("plans.staysPositive", p.HorizonMonths),
				),
			),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("plans.deleteTitle")), Title(uistate.T("plans.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
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
// runwayLowClass styles the runway low-point line: muted when the low-point stays
// positive, danger + semibold when it dips negative so it reads as a warning (C173).
func runwayLowClass(tone string) string {
	if tone == "neg" {
		return "t-body " + tw.ColorClass("text-down") // danger color = salient vs muted gray
	}
	return "muted"
}

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
