//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
	dsExtra := ui.UseState("")
	onDsExtra := ui.UseEvent(func(v string) { dsExtra.Set(v) })

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
		resultBody = P(Class("muted"), uistate.T("planning.payoffHint"))
	default:
		bal, errB := money.ParseMinor(strings.TrimSpace(balStr.Get()), currency.Decimals(base))
		pay, errP := money.ParseMinor(strings.TrimSpace(payStr.Get()), currency.Decimals(base))
		apr, errA := strconv.ParseFloat(strings.TrimSpace(aprStr.Get()), 64)
		switch {
		case errB != nil || errP != nil || errA != nil:
			resultBody = P(Class("err"), Attr("role", "alert"), uistate.T("planning.invalidNumbers"))
		default:
			if r, ok := payoff.Project(bal, apr, pay); ok {
				extraNote := Fragment()
				if extra, eerr := money.ParseMinor(strings.TrimSpace(extraStr.Get()), currency.Decimals(base)); eerr == nil && extra > 0 {
					if r2, ok2 := payoff.Project(bal, apr, pay+extra); ok2 {
						extraNote = P(Class("muted"), uistate.T("planning.extraNote",
							fmtMoney(money.New(extra, base)), r.Months-r2.Months, fmtMoney(money.New(r.TotalInterest-r2.TotalInterest, base)),
						))
					}
				}
				resultBody = Div(
					Div(Class("stat-grid"),
						stat(uistate.T("planning.months"), fmt.Sprintf("%d", r.Months), ""),
						stat(uistate.T("planning.totalInterest"), fmtMoney(money.New(r.TotalInterest, base)), "neg"),
						stat(uistate.T("planning.totalPaid"), fmtMoney(money.New(r.TotalPaid, base)), ""),
					),
					extraNote,
				)
			} else {
				min := payoff.MinimumViablePayment(bal, apr)
				resultBody = P(Class("err"), Attr("role", "alert"), uistate.T("planning.paymentTooLowMin", fmtMoney(money.New(min, base))))
			}
		}
	}

	forecastCard := Fragment()
	if app != nil {
		accounts := app.Accounts()
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
		mStart, mEnd := dateutil.MonthRange(time.Now())
		income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
		monthlyNet := income.Amount - expense.Amount

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

		trimNote := Fragment()
		if trim, terr := money.ParseMinor(strings.TrimSpace(trimStr.Get()), currency.Decimals(base)); terr == nil && trim > 0 {
			series2 := forecast.Project(net.Amount, []forecast.Recurring{{Monthly: monthlyNet + trim}}, nil, 12)
			chartSeries = append(chartSeries, chartspec.Series{Name: uistate.T("planning.seriesTrim"), Color: "#cfa14e", Points: toPoints(series2)})
			end2 := series2[len(series2)-1]
			trimNote = P(Class("muted"), uistate.T("planning.trimNote",
				fmtMoney(money.New(trim, base)), fmtMoney(money.New(end2, base)), fmtMoney(money.New(end2-series[len(series)-1], base))))
		}
		spec := chartspec.Spec{
			Kind:   chartspec.Line,
			Series: chartSeries,
			Y:      chartspec.Axis{Format: yFmt},
			Legend: len(chartSeries) > 1,
		}
		forecastCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("planning.forecastTitle")),
			P(Class("muted"), uistate.T("planning.forecastHint", fmtMoney(money.New(monthlyNet, base)), fmtMoney(endVal))),
			uiw.Chart(uiw.ChartProps{Spec: spec, Height: "180px", Label: uistate.T("planning.forecastChartLabel", fmtMoney(endVal))}),
			Form(Class("form-grid"),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.trimPlaceholder", base)), Value(trimStr.Get()), Step("0.01"), OnInput(onTrim)),
			),
			trimNote,
		)
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
			totalNote = P(Class("muted"), uistate.T("recurring.monthlyTotal", fmtMoney(money.New(monthlyTotal, base))))
		}
		list := IfElse(len(recs) == 0,
			P(Class("empty"), uistate.T("recurring.empty")),
			Div(Class("rows"), MapKeyed(recs,
				func(r domain.Recurring) any { return r.ID },
				func(r domain.Recurring) ui.Node {
					return ui.CreateElement(RecurringRow, recurringRowProps{Recurring: r, OnDelete: deleteRecurring})
				},
			)),
		)
		recurringCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("recurring.title")),
			P(Class("muted"), uistate.T("recurring.hint")),
			Form(Class("form-grid"), OnSubmit(addRecurring),
				Input(append([]any{Class("field"), Type("text"), Placeholder(uistate.T("recurring.labelPlaceholder")), Value(rLabel.Get()), OnInput(onRLabel)}, errAttrs("refi-err", rErr.Get())...)...),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("recurring.amountPlaceholder", base)), Value(rAmount.Get()), Step("0.01"), OnInput(onRAmount)),
				Select(Class("field"), Attr("aria-label", uistate.T("recurring.cadence")), Title(uistate.T("recurring.cadence")), OnChange(onRCadence), cadenceOpts),
				Select(Class("field"), Attr("aria-label", uistate.T("recurring.account")), Title(uistate.T("recurring.account")), OnChange(onRAccount), acctOpts),
				Select(Class("field"), Attr("aria-label", uistate.T("recurring.category")), Title(uistate.T("recurring.category")), OnChange(onRCategory), catOpts),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("recurring.add")),
			),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopost"), On: rAutopost.Get(), OnChange: func(v bool) { rAutopost.Set(v) }}),
			errText("refi-err", rErr.Get()),
			totalNote,
			list,
			Div(Class("flex items-center gap-2 mt-2"),
				Button(Class("btn"), Type("button"), Title(uistate.T("recurring.postDueTitle")), OnClick(postDue), uistate.T("recurring.postDue")),
				If(postMsg.Get() != "", Span(Class("muted"), postMsg.Get())),
			),
		)
	}

	plansCard := Fragment()
	if app != nil {
		plans := app.Plans()
		list := IfElse(len(plans) == 0,
			P(Class("empty"), uistate.T("plans.empty")),
			Div(Class("rows"), MapKeyed(plans,
				func(p domain.Plan) any { return p.ID },
				func(p domain.Plan) ui.Node {
					return ui.CreateElement(PlanRow, planRowProps{Plan: p, Base: base, OnDelete: deletePlan})
				},
			)),
		)
		plansCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("plans.title")),
			P(Class("muted"), uistate.T("plans.hint")),
			Form(Class("form-grid"), OnSubmit(addPlan),
				Input(append([]any{Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("plans.namePlaceholder")), Value(plName.Get()), OnInput(onPlName)}, errAttrs("plan-err", plErr.Get())...)...),
				Input(Class("field"), Type("number"), Title(uistate.T("plans.horizonTitle")), Attr("aria-required", "true"), Placeholder(uistate.T("plans.horizonPlaceholder")), Value(plHorizon.Get()), Step("1"), OnInput(onPlHorizon)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("plans.startPlaceholder", base)), Value(plStart.Get()), Step("0.01"), OnInput(onPlStart)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("plans.monthlyPlaceholder", base)), Value(plMonthly.Get()), Step("0.01"), OnInput(onPlMonthly)),
				Input(Class("field"), Type("number"), Title(uistate.T("plans.onceAmtTitle")), Placeholder(uistate.T("plans.onceAmtPlaceholder", base)), Value(plOnceAmt.Get()), Step("0.01"), OnInput(onPlOnceAmt)),
				Input(Class("field"), Type("number"), Title(uistate.T("plans.onceMonthTitle")), Placeholder(uistate.T("plans.onceMonthPlaceholder")), Value(plOnceMonth.Get()), Step("1"), OnInput(onPlOnceMonth)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("plans.add")),
			),
			errText("plan-err", plErr.Get()),
			list,
		)
	}

	// Debt strategy (D9): compare snowball vs avalanche across the household's
	// liability accounts, using their balances, rates, and minimum payments.
	debtCard := Fragment()
	if app != nil {
		txns := app.Transactions()
		var debts []payoff.Debt
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
			debts = append(debts, payoff.Debt{Name: a.Name, Balance: owed, AprPercent: a.InterestRateAPR, MinPayment: a.MinPayment.Abs().Amount})
		}

		var body ui.Node
		switch {
		case len(debts) == 0:
			body = P(Class("empty"), uistate.T("planning.debtStrategyEmpty"))
		default:
			extra, _ := money.ParseMinor(strings.TrimSpace(dsExtra.Get()), currency.Decimals(base))
			if extra < 0 {
				extra = 0
			}
			snow, okS := payoff.BuildPlan(debts, extra, payoff.Snowball)
			aval, okA := payoff.BuildPlan(debts, extra, payoff.Avalanche)
			if !okS || !okA {
				body = P(Class("err"), Attr("role", "alert"), uistate.T("planning.strategyNotViable"))
			} else {
				rec := Fragment()
				if saved := snow.TotalInterest - aval.TotalInterest; saved > 0 {
					rec = P(Class("muted"), uistate.T("planning.strategyRecommend", fmtMoney(money.New(saved, base))))
				}
				// When the two strategies are truly identical (typically at $0 extra,
				// or a single debt) the side-by-side is meaningless — explain why (L5).
				explain := Fragment()
				if snow.Months == aval.Months && snow.TotalInterest == aval.TotalInterest {
					explain = P(Class("budget-sub"), "Snowball and avalanche match here — add an extra monthly amount above to see them diverge.")
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
				body = Div(
					Div(Class("stat-grid"),
						stat(uistate.T("planning.snowball"), uistate.T("planning.strategyMonths", snow.Months), ""),
						stat(uistate.T("planning.avalanche"), uistate.T("planning.strategyMonths", aval.Months), ""),
					),
					P(Class("budget-sub font-display"), "Debt-free by "+snowDate+" (snowball) · "+avalDate+" (avalanche)."),
					P(Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.snowball"), fmtMoney(money.New(snow.TotalInterest, base)))),
					P(Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.avalanche"), fmtMoney(money.New(aval.TotalInterest, base)))),
					P(Class("muted"), "Payoff order: "+strings.Join(orderParts, " → ")),
					rec,
					explain,
				)
			}
		}
		debtCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("planning.debtStrategyTitle")),
			P(Class("muted"), uistate.T("planning.debtStrategyHint")),
			Form(Class("form-grid"),
				Input(Class("field"), Type("number"), Attr("aria-label", "Extra monthly payment"), Placeholder(uistate.T("planning.debtStrategyExtra", base)), Value(dsExtra.Get()), Step("0.01"), OnInput(onDsExtra)),
			),
			If(strings.TrimSpace(dsExtra.Get()) == "" && len(debts) > 0 && payoff.SuggestedExtra(debts) > 0,
				Div(Class("flex items-center gap-2 mt-2"),
					Span(Class("muted"), "At $0 extra the strategies tie."),
					Button(Class("btn"), Type("button"), Title("Fill a sensible extra to compare snowball vs avalanche"),
						OnClick(func() { dsExtra.Set(money.FormatMinor(payoff.SuggestedExtra(debts), currency.Decimals(base))) }),
						"Try "+fmtMoney(money.New(payoff.SuggestedExtra(debts), base))+"/mo"),
				),
			),
			body,
		)
	}

	return Div(
		forecastCard,
		recurringCard,
		plansCard,
		debtCard,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("planning.payoffTitle")),
			P(Class("muted"), uistate.T("planning.payoffDesc")),
			Form(Class("form-grid"),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.balancePlaceholder", base)), Value(balStr.Get()), Step("0.01"), OnInput(onBal)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.aprPlaceholder")), Value(aprStr.Get()), Step("0.01"), OnInput(onApr)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.paymentPlaceholder", base)), Value(payStr.Get()), Step("0.01"), OnInput(onPay)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.extraPlaceholder", base)), Value(extraStr.Get()), Step("0.01"), OnInput(onExtra)),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("planning.projectionTitle")),
			resultBody,
		),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), r.Label),
			Span(Class("row-meta"), meta),
		),
		Span(Class(amountClass(r.Amount)), fmtMoney(r.Amount)),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("recurring.deleteTitle")), Title(uistate.T("recurring.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
	)
}

type planRowProps struct {
	Plan     domain.Plan
	Base     string
	OnDelete func(string)
}

// PlanRow renders one saved what-if plan: its name, the horizon/start/monthly
// assumptions, and the projected end-of-horizon balance from internal/planning,
// with a remove button. Its own component per the no-hooks-in-loops rule.
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

	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), p.Name),
			Span(Class("row-meta"), meta),
		),
		If(len(vals) > 1, uiw.AreaChart(uiw.AreaChartProps{
			Values: vals, Stroke: stroke, GradientID: "cf-plan-" + p.ID,
			Width: 120, Height: 28, Label: uistate.T("plans.chartLabel", fmtMoney(end)),
		})),
		Span(Class("amount fig "+figTone(end)), uistate.T("plans.projected", fmtMoney(end))),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("plans.deleteTitle")), Title(uistate.T("plans.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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
