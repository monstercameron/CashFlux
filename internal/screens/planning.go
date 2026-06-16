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
	"github.com/monstercameron/CashFlux/internal/forecast"
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
			uiw.AreaChart(uiw.AreaChartProps{Values: values, Stroke: stroke, GradientID: "cf-forecast"}),
			Form(Class("form-grid"),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("planning.trimPlaceholder", base)), Value(trimStr.Get()), Step("0.01"), OnInput(onTrim)),
			),
			trimNote,
		)
	}

	return Div(
		forecastCard,
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
