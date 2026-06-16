//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
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
			resultBody = P(Class("err"), uistate.T("planning.invalidNumbers"))
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
				resultBody = P(Class("err"), uistate.T("planning.paymentTooLow"))
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
		values := make([]float64, len(series))
		for i, s := range series {
			values[i] = float64(s)
		}
		endVal := money.New(series[len(series)-1], base)
		stroke := "#54b884"
		if monthlyNet < 0 {
			stroke = "#d8716f"
		}
		trimNote := Fragment()
		if trim, terr := money.ParseMinor(strings.TrimSpace(trimStr.Get()), currency.Decimals(base)); terr == nil && trim > 0 {
			series2 := forecast.Project(net.Amount, []forecast.Recurring{{Monthly: monthlyNet + trim}}, nil, 12)
			end2 := series2[len(series2)-1]
			trimNote = P(Class("muted"), uistate.T("planning.trimNote",
				fmtMoney(money.New(trim, base)), fmtMoney(money.New(end2, base)), fmtMoney(money.New(end2-series[len(series)-1], base))))
		}
		forecastCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("planning.forecastTitle")),
			P(Class("muted"), uistate.T("planning.forecastHint", fmtMoney(money.New(monthlyNet, base)), fmtMoney(endVal))),
			uiw.AreaChart(uiw.AreaChartProps{Values: values, Stroke: stroke, GradientID: "cf-forecast", Label: uistate.T("planning.forecastChartLabel", fmtMoney(endVal))}),
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
				Input(Class("field"), Type("text"), Placeholder(uistate.T("recurring.labelPlaceholder")), Value(rLabel.Get()), OnInput(onRLabel)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("recurring.amountPlaceholder", base)), Value(rAmount.Get()), Step("0.01"), OnInput(onRAmount)),
				Select(Class("field"), Title(uistate.T("recurring.cadence")), OnChange(onRCadence), cadenceOpts),
				Select(Class("field"), Title(uistate.T("recurring.account")), OnChange(onRAccount), acctOpts),
				Select(Class("field"), Title(uistate.T("recurring.category")), OnChange(onRCategory), catOpts),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("recurring.add")),
			),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopost"), On: rAutopost.Get(), OnChange: func(v bool) { rAutopost.Set(v) }}),
			If(rErr.Get() != "", P(Class("err"), rErr.Get())),
			totalNote,
			list,
			Div(Class("flex items-center gap-2 mt-2"),
				Button(Class("btn"), Type("button"), Title(uistate.T("recurring.postDueTitle")), OnClick(postDue), uistate.T("recurring.postDue")),
				If(postMsg.Get() != "", Span(Class("muted"), postMsg.Get())),
			),
		)
	}

	return Div(
		forecastCard,
		recurringCard,
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
		Button(Class("btn-del"), Type("button"), Title(uistate.T("recurring.deleteTitle")), OnClick(del), "✕"),
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
