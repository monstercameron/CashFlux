//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/planning"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/runway"
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

	balStr := ui.UseState("")
	aprStr := ui.UseState("")
	payStr := ui.UseState("")
	extraStr := ui.UseState("")

	onBal := ui.UseEvent(func(v string) { balStr.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprStr.Set(v) })
	onPay := ui.UseEvent(func(v string) { payStr.Set(v) })
	onExtra := ui.UseEvent(func(v string) { extraStr.Set(v) })
	trimStr := ui.UseState("")
	onTrim := ui.UseEvent(func(v string) { trimStr.Set(v) })
	// compareID is the ID of the saved plan whose projection curve is overlaid on
	// the 12-month forecast chart for side-by-side comparison (L27 enhancement).
	// Empty string means baseline-only (no overlay).
	compareID := ui.UseState("")
	onCompare := ui.UseEvent(func(e ui.Event) { compareID.Set(e.GetValue()) })
	dsExtra := ui.UseState("")
	onDsExtra := ui.UseEvent(func(v string) { dsExtra.Set(v) })

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

	// Recurring cash-flow management.
	rev := ui.UseState(0)
	rLabel := ui.UseState("")
	rAmount := ui.UseState("")
	rCadence := ui.UseState(string(domain.CadenceMonthly))
	rAccount := ui.UseState("")
	rCategory := ui.UseState("")
	rAutopost := ui.UseState(false)
	rErr := ui.UseState("")
	postMsg := ui.UseState("")
	onRLabel := ui.UseEvent(func(v string) { rLabel.Set(v) })
	onRAmount := ui.UseEvent(func(v string) { rAmount.Set(v) })
	onRCadence := ui.UseEvent(func(e ui.Event) { rCadence.Set(e.GetValue()) })
	onRAccount := ui.UseEvent(func(e ui.Event) { rAccount.Set(e.GetValue()) })
	onRCategory := ui.UseEvent(func(e ui.Event) { rCategory.Set(e.GetValue()) })
	addRecurring := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		label := strings.TrimSpace(rLabel.Get())
		if label == "" {
			rErr.Set(uistate.T("recurring.labelRequired"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(rAmount.Get()), currency.Decimals(base))
		if err != nil || amt == 0 {
			rErr.Set(uistate.T("recurring.amountRequired"))
			return
		}
		r := domain.Recurring{
			ID: id.New(), Label: label, Amount: money.New(amt, base),
			Cadence: domain.RecurringCadence(rCadence.Get()), NextDue: time.Now(),
			AccountID: rAccount.Get(), CategoryID: rCategory.Get(), Autopost: rAutopost.Get(),
		}
		if err := app.PutRecurring(r); err != nil {
			rErr.Set(err.Error())
			return
		}
		rLabel.Set("")
		rAmount.Set("")
		rErr.Set("")
		rev.Set(rev.Get() + 1)
	}))
	deleteRecurring := func(rid string) {
		if app != nil {
			_ = app.DeleteRecurring(rid)
			rev.Set(rev.Get() + 1)
		}
	}
	postDue := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		n, err := app.PostDueRecurring(time.Now())
		if err != nil {
			postMsg.Set(err.Error())
			return
		}
		postMsg.Set(uistate.T("recurring.posted", plural(n, "transaction")))
		rev.Set(rev.Get() + 1)
	}))

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
		rev.Set(rev.Get() + 1)
	}))
	deletePlan := func(pid string) {
		if app != nil {
			_ = app.DeletePlan(pid)
			rev.Set(rev.Get() + 1)
		}
	}

	var resultBody ui.Node
	switch {
	case strings.TrimSpace(balStr.Get()) == "" || strings.TrimSpace(payStr.Get()) == "":
		resultBody = P(css.Class("muted"), uistate.T("planning.payoffHint"))
	default:
		bal, errB := money.ParseMinor(strings.TrimSpace(balStr.Get()), currency.Decimals(base))
		pay, errP := money.ParseMinor(strings.TrimSpace(payStr.Get()), currency.Decimals(base))
		apr, errA := strconv.ParseFloat(strings.TrimSpace(aprStr.Get()), 64)
		switch {
		case errB != nil || errP != nil || errA != nil:
			resultBody = P(css.Class("err"), Attr("role", "alert"), uistate.T("planning.invalidNumbers"))
		default:
			if r, ok := payoff.Project(bal, apr, pay); ok {
				extraNote := Fragment()
				if extra, eerr := money.ParseMinor(strings.TrimSpace(extraStr.Get()), currency.Decimals(base)); eerr == nil && extra > 0 {
					if r2, ok2 := payoff.Project(bal, apr, pay+extra); ok2 {
						extraNote = P(css.Class("muted"), uistate.T("planning.extraNote",
							fmtMoney(money.New(extra, base)), r.Months-r2.Months, fmtMoney(money.New(r.TotalInterest-r2.TotalInterest, base)),
						))
					}
				}
				resultBody = Div(
					Div(css.Class("stat-grid"),
						stat(uistate.T("planning.months"), fmt.Sprintf("%d", r.Months), ""),
						stat(uistate.T("planning.debtFreeBy"), payoff.DebtFreeMonth(time.Now(), r.Months).Format("Jan 2006"), ""),
						stat(uistate.T("planning.totalInterest"), fmtMoney(money.New(r.TotalInterest, base)), "neg"),
						stat(uistate.T("planning.totalPaid"), fmtMoney(money.New(r.TotalPaid, base)), ""),
					),
					extraNote,
				)
			} else {
				min := payoff.MinimumViablePayment(bal, apr)
				resultBody = P(css.Class("err"), Attr("role", "alert"), uistate.T("planning.paymentTooLowMin", fmtMoney(money.New(min, base))))
			}
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

		// Plot in major units (dollars) with a compact-currency axis (C16), and
		// overlay the trimmed scenario beside the baseline when a trim is set, so
		// the two net-worth curves can be compared directly (D10).
		divf := 1.0
		for i := 0; i < currency.Decimals(base); i++ {
			divf *= 10
		}
		toPoints := func(vals []int64) []chartspec.Point {
			pts := make([]chartspec.Point, len(vals))
			for i, v := range vals {
				pts[i] = chartspec.Point{X: float64(i), Y: float64(v) / divf}
			}
			return pts
		}
		yFmt := ".2~s"
		if currency.Symbol(base) == "$" {
			yFmt = "$.2~s"
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
		forecastCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.forecastTitle"),
			Body: Fragment(
				// Headline answer (G7 §4/§5): surface the projected 12-month net worth as a
				// display-weight figure so Dev's primary question ("where will I be?") is
				// answerable at glance-speed, before parsing the chart or the hint sentence.
				Div(css.Class("stat-grid"),
					stat(uistate.T("planning.projectedNetWorth"), fmtMoney(endVal), accentFor(endVal)),
					stat(uistate.T("planning.avgMonthlyNet"), fmtMoney(money.New(monthlyNet, base)), accentFor(money.New(monthlyNet, base))),
				),
				P(css.Class("muted"), uistate.T("planning.forecastHint", fmtMoney(money.New(monthlyNet, base)), fmtMoney(endVal))),
				P(css.Class("muted"), Attr("data-testid", "forecast-basis"), uistate.T("planning.forecastBasis")),
				uiw.Chart(uiw.ChartProps{Spec: spec, Height: "180px", Label: uistate.T("planning.forecastChartLabel", fmtMoney(endVal))}),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.trimPlaceholder", base), Input(css.Class("field"), Type("number"), Value(trimStr.Get()), Step("0.01"), OnInput(onTrim))),
					If(len(savedPlans) > 0,
						Label(css.Class("field-label"), uistate.T("plans.compareLabel"),
							Select(css.Class("field"), Attr("aria-label", uistate.T("plans.compareLabel")),
								Attr("data-testid", "plan-compare-select"), OnChange(onCompare), compareOpts),
						),
					),
				),
				trimNote,
				compareNote,
			),
		})
	}

	// Affordability check (L8): "can I afford $X (in N months, keeping a buffer)?"
	// projected from today's net worth and this month's net cash flow, via the pure
	// internal/afford engine — a deterministic answer, not an AI guess.
	affordCard := Fragment()
	if app != nil {
		accounts := app.Accounts()
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
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
			res := afford.CanAfford(amt, net.Amount, monthlyNet, months, reserved)
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
					stat(uistate.T("planning.affordAvailable"), fmtMoney(money.New(res.Available, base)), ""),
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
		_, assets, _, _ := ledger.NetWorth(app.Accounts(), app.Transactions(), rates)
		buffer, _ := money.ParseMinor(strings.TrimSpace(rwBuffer.Get()), currency.Decimals(base))
		if buffer < 0 {
			buffer = 0
		}
		const runwayDays = 60

		var rwBody ui.Node = P(css.Class("muted"), uistate.T("planning.runwayEmpty"))
		if len(recs) > 0 {
			if proj, perr := runway.Project(assets.Amount, recs, time.Now(), runwayDays, buffer, rates); perr == nil {
				lowTone := ""
				if proj.MinBalance < 0 {
					lowTone = "neg"
				}
				lowDate := time.Now().AddDate(0, 0, proj.MinDay).Format("Jan 2")
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
				rwBody = Div(
					Div(css.Class("stat-grid"),
						stat(uistate.T("planning.runwayStart"), fmtMoney(money.New(assets.Amount, base)), ""),
						stat(uistate.T("planning.runwayLowLabel"), fmtMoney(money.New(proj.MinBalance, base)), lowTone),
					),
					verdict,
					P(css.Class("muted"), uistate.T("planning.runwayLow", fmtMoney(money.New(proj.MinBalance, base)), lowDate)),
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

	recurringCard := Fragment()
	if app != nil {
		cadenceOpts := []ui.Node{
			Option(Value(string(domain.CadenceWeekly)), SelectedIf(rCadence.Get() == string(domain.CadenceWeekly)), uistate.T("recurring.cadenceWeekly")),
			Option(Value(string(domain.CadenceMonthly)), SelectedIf(rCadence.Get() == string(domain.CadenceMonthly)), uistate.T("recurring.cadenceMonthly")),
			Option(Value(string(domain.CadenceQuarterly)), SelectedIf(rCadence.Get() == string(domain.CadenceQuarterly)), uistate.T("recurring.cadenceQuarterly")),
			Option(Value(string(domain.CadenceYearly)), SelectedIf(rCadence.Get() == string(domain.CadenceYearly)), uistate.T("recurring.cadenceYearly")),
		}
		acctOpts := []ui.Node{Option(Value(""), SelectedIf(rAccount.Get() == ""), uistate.T("recurring.noAccount"))}
		for _, ac := range app.Accounts() {
			acctOpts = append(acctOpts, Option(Value(ac.ID), SelectedIf(rAccount.Get() == ac.ID), ac.Name))
		}
		catOpts := []ui.Node{Option(Value(""), SelectedIf(rCategory.Get() == ""), uistate.T("recurring.noCategory"))}
		for _, c := range app.Categories() {
			catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(rCategory.Get() == c.ID), c.Name))
		}
		recs := app.Recurring()
		var monthlyTotal int64
		for _, r := range recs {
			monthlyTotal += r.MonthlyEquivalent()
		}
		totalNote := Fragment()
		if len(recs) > 0 {
			totalNote = P(css.Class("muted"), uistate.T("recurring.monthlyTotal", fmtMoney(money.New(monthlyTotal, base))))
		}
		list := IfElse(len(recs) == 0,
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("recurring.empty"), CTALabel: uistate.T("recurring.add"), FocusID: "recurring-add"}),
			Div(css.Class("rows"), MapKeyed(recs,
				func(r domain.Recurring) any { return r.ID },
				func(r domain.Recurring) ui.Node {
					return ui.CreateElement(RecurringRow, recurringRowProps{Recurring: r, OnDelete: deleteRecurring})
				},
			)),
		)
		recurringCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("recurring.title"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("recurring.hint")),
				Form(css.Class("form-grid"), OnSubmit(addRecurring),
					Input(append([]any{css.Class("field"), Attr("id", "recurring-add"), Type("text"), Placeholder(uistate.T("recurring.labelPlaceholder")), Value(rLabel.Get()), OnInput(onRLabel)}, errAttrs("refi-err", rErr.Get())...)...),
					labeledField(uistate.T("recurring.amountPlaceholder", base), Input(css.Class("field"), Type("number"), Value(rAmount.Get()), Step("0.01"), OnInput(onRAmount))),
					Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.cadence")), Title(uistate.T("recurring.cadence")), OnChange(onRCadence), cadenceOpts),
					Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.account")), Title(uistate.T("recurring.account")), OnChange(onRAccount), acctOpts),
					Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.category")), Title(uistate.T("recurring.category")), OnChange(onRCategory), catOpts),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("recurring.add")),
				),
				uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopost"), On: rAutopost.Get(), OnChange: func(v bool) { rAutopost.Set(v) }}),
				errText("refi-err", rErr.Get()),
				totalNote,
				list,
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("recurring.postDueTitle")), OnClick(postDue), uistate.T("recurring.postDue")),
					If(postMsg.Get() != "", Span(css.Class("muted"), postMsg.Get())),
				),
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

	// Debt strategy (D9): compare snowball vs avalanche across the household's
	// liability accounts, using their balances, rates, and minimum payments.
	debtCard := Fragment()
	if app != nil {
		txns := app.Transactions()
		var debts []payoff.Debt
		var payoffLiabs []domain.Account // liabilities with a balance, for the include toggles
		for _, a := range app.Accounts() {
			if a.Archived || a.Class != domain.ClassLiability {
				continue
			}
			bal, err := ledger.Balance(a, txns)
			if err != nil {
				continue
			}
			owed := bal.Abs().Amount
			if owed <= 0 {
				continue
			}
			payoffLiabs = append(payoffLiabs, a)
			if !a.IncludedInPayoff() { // mortgage is excluded by default (L5)
				continue
			}
			debts = append(debts, payoff.Debt{Name: a.Name, Balance: owed, AprPercent: a.InterestRateAPR, MinPayment: a.MinPayment.Abs().Amount})
		}

		// Payoff progress vs a stored baseline (L5 gap 5): "paid off $X of $Y".
		var currentOwed int64
		for _, d := range debts {
			currentOwed += d.Balance
		}
		prog, since, tracking := app.PayoffProgress(currentOwed)
		var progressNode ui.Node = Fragment()
		if tracking {
			w := prog.Percent
			if w > 100 {
				w = 100
			}
			progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
				P(css.Class("budget-sub", tw.FontDisplay), "Paid off "+fmtMoney(money.New(prog.PaidOff, base))+" of "+fmtMoney(money.New(prog.Baseline, base))+" ("+strconv.Itoa(prog.Percent)+"%) since "+since.Format("Jan 2, 2006")+"."),
				Div(css.Class("bar"), Div(css.Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", w)))),
				Button(css.Class("btn"), Type("button"), Style(map[string]string{"margin-top": "0.4rem"}), OnClick(func() { _ = app.ClearPayoffTracking(); rev.Set(rev.Get() + 1) }), "Reset progress"),
			)
		} else if len(debts) > 0 {
			owed := currentOwed
			progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
				Button(css.Class("btn"), Type("button"), Title("Snapshot today's balances to track how much you pay off over time"),
					OnClick(func() { _ = app.StartPayoffTracking(owed, base); rev.Set(rev.Get() + 1) }), "Start tracking progress"),
			)
		}

		// Per-liability include/exclude toggles (each ToggleRow is its own component,
		// so the per-row hook is safe inside this loop).
		var includeToggles []ui.Node
		for _, a := range payoffLiabs {
			acc := a
			includeToggles = append(includeToggles, uiw.ToggleRow(uiw.ToggleRowProps{
				Label: acc.Name,
				On:    acc.IncludedInPayoff(),
				OnChange: func(on bool) {
					next := acc
					v := on
					next.IncludeInPayoff = &v
					if err := app.PutAccount(next); err != nil {
						return
					}
					rev.Set(rev.Get() + 1)
				},
			}))
		}

		var body ui.Node
		switch {
		case len(debts) == 0:
			body = P(css.Class("empty"), uistate.T("planning.debtStrategyEmpty"))
		default:
			extra, _ := money.ParseMinor(strings.TrimSpace(dsExtra.Get()), currency.Decimals(base))
			if extra < 0 {
				extra = 0
			}
			snow, okS := payoff.BuildPlan(debts, extra, payoff.Snowball)
			aval, okA := payoff.BuildPlan(debts, extra, payoff.Avalanche)
			if !okS || !okA {
				body = P(css.Class("err"), Attr("role", "alert"), uistate.T("planning.strategyNotViable"))
			} else {
				rec := Fragment()
				if saved := snow.TotalInterest - aval.TotalInterest; saved > 0 {
					rec = P(css.Class("muted"), uistate.T("planning.strategyRecommend", fmtMoney(money.New(saved, base))))
				}
				// When the two strategies are truly identical (typically at $0 extra,
				// or a single debt) the side-by-side is meaningless — explain why (L5).
				explain := Fragment()
				if snow.Months == aval.Months && snow.TotalInterest == aval.TotalInterest {
					explain = P(css.Class("budget-sub"), "Snowball and avalanche match here — add an extra monthly amount above to see them diverge.")
				}
				// A calendar debt-free date reads better than a bare month count
				// (L5), plus a "cleared by <month>" beside each debt in the order.
				now := time.Now()
				snowDate := payoff.DebtFreeMonth(now, snow.Months).Format("Jan 2006")
				avalDate := payoff.DebtFreeMonth(now, aval.Months).Format("Jan 2006")
				orderParts := make([]string, len(aval.Order))
				for i, n := range aval.Order {
					if i < len(aval.ClearedMonths) {
						orderParts[i] = n + " (" + payoff.DebtFreeMonth(now, aval.ClearedMonths[i]).Format("Jan 2006") + ")"
					} else {
						orderParts[i] = n
					}
				}
				// Burn-down chart (L5 gap 4): the remaining total balance falling to
				// zero, from the full starting balance through the avalanche schedule.
				burnChart := Fragment()
				if len(aval.Schedule) > 0 {
					divf := 1.0
					for k := 0; k < currency.Decimals(base); k++ {
						divf *= 10
					}
					var startTotal int64
					for _, d := range debts {
						startTotal += d.Balance
					}
					burnPts := make([]chartspec.Point, 0, len(aval.Schedule)+1)
					burnPts = append(burnPts, chartspec.Point{X: 0, Y: float64(startTotal) / divf})
					for i, b := range aval.Schedule {
						burnPts = append(burnPts, chartspec.Point{X: float64(i + 1), Y: float64(b) / divf})
					}
					yFmt := ".2~s"
					if currency.Symbol(base) == "$" {
						yFmt = "$.2~s"
					}
					burnChart = Div(Style(map[string]string{"margin-top": "0.6rem"}),
						P(css.Class("budget-sub"), "Balance burn-down to zero:"),
						uiw.Chart(uiw.ChartProps{
							Spec:   chartspec.Spec{Kind: chartspec.Area, Series: []chartspec.Series{{Name: "Remaining balance", Points: burnPts}}, Y: chartspec.Axis{Format: yFmt}},
							Height: "150px",
							Label:  "Debt balance falling to zero over " + strconv.Itoa(aval.Months) + " months",
						}),
					)
				}
				body = Div(
					Div(css.Class("stat-grid"),
						stat(uistate.T("planning.snowball"), uistate.T("planning.strategyMonths", snow.Months), ""),
						stat(uistate.T("planning.avalanche"), uistate.T("planning.strategyMonths", aval.Months), ""),
					),
					P(css.Class("budget-sub", tw.FontDisplay), "Debt-free by "+snowDate+" (snowball) · "+avalDate+" (avalanche)."),
					P(css.Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.snowball"), fmtMoney(money.New(snow.TotalInterest, base)))),
					P(css.Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.avalanche"), fmtMoney(money.New(aval.TotalInterest, base)))),
					P(css.Class("muted"), "Payoff order: "+strings.Join(orderParts, " → ")),
					rec,
					explain,
					burnChart,
				)
			}
		}
		debtCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.debtStrategyTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("planning.debtStrategyHint")),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.debtStrategyExtra", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(dsExtra.Get()), Step("0.01"), OnInput(onDsExtra))),
				),
				If(strings.TrimSpace(dsExtra.Get()) == "" && len(debts) > 0 && payoff.SuggestedExtra(debts) > 0,
					Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
						Span(css.Class("muted"), "At $0 extra the strategies tie."),
						Button(css.Class("btn"), Type("button"), Title("Fill a sensible extra to compare snowball vs avalanche"),
							OnClick(func() { dsExtra.Set(money.FormatMinor(payoff.SuggestedExtra(debts), currency.Decimals(base))) }),
							"Try "+fmtMoney(money.New(payoff.SuggestedExtra(debts), base))+"/mo"),
					),
				),
				body,
				progressNode,
				If(len(includeToggles) > 0, Div(Style(map[string]string{"margin-top": "0.6rem"}),
					P(css.Class("budget-sub"), "Include in payoff plan (a mortgage is excluded by default):"),
					Div(includeToggles),
				)),
			),
		})
	}

	return Div(
		forecastCard,
		affordCard,
		runwayCard,
		recurringCard,
		plansCard,
		debtCard,
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.payoffTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("planning.payoffDesc")),
				Form(css.Class("form-grid"),
					labeledField(uistate.T("planning.balancePlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(balStr.Get()), Step("0.01"), OnInput(onBal))),
					labeledField(uistate.T("planning.aprPlaceholder"), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(aprStr.Get()), Step("0.01"), OnInput(onApr))),
					labeledField(uistate.T("planning.paymentPlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(payStr.Get()), Step("0.01"), OnInput(onPay))),
					labeledField(uistate.T("planning.extraPlaceholder", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(extraStr.Get()), Step("0.01"), OnInput(onExtra))),
				),
			),
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("planning.projectionTitle"),
			Body:  resultBody,
		}),
	)
}

type recurringRowProps struct {
	Recurring domain.Recurring
	OnDelete  func(string)
}

// RecurringRow renders one recurring cash flow (amount colored by sign) with a
// remove button. It owns its own click handler (per the no-hooks-in-loops rule).
func RecurringRow(props recurringRowProps) ui.Node {
	r := props.Recurring
	del := ui.UseEvent(Prevent(func() { props.OnDelete(r.ID) }))
	meta := cadenceLabel(r.Cadence) + " · " + uistate.T("recurring.nextDue", r.NextDue.Format("Jan 2, 2006"))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), r.Label),
			Span(css.Class("row-meta"), meta),
		),
		Span(ClassStr(amountClass(r.Amount)), fmtMoney(r.Amount)),
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

// cadenceLabel localizes a recurring cadence.
func cadenceLabel(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return uistate.T("recurring.cadenceWeekly")
	case domain.CadenceQuarterly:
		return uistate.T("recurring.cadenceQuarterly")
	case domain.CadenceYearly:
		return uistate.T("recurring.cadenceYearly")
	default:
		return uistate.T("recurring.cadenceMonthly")
	}
}
