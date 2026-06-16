//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
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

	onBal := ui.UseEvent(func(v string) { balStr.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprStr.Set(v) })
	onPay := ui.UseEvent(func(v string) { payStr.Set(v) })

	var resultBody ui.Node
	switch {
	case strings.TrimSpace(balStr.Get()) == "" || strings.TrimSpace(payStr.Get()) == "":
		resultBody = P(Class("muted"), "Enter a balance, APR, and monthly payment to see the payoff.")
	default:
		bal, errB := money.ParseMinor(strings.TrimSpace(balStr.Get()), currency.Decimals(base))
		pay, errP := money.ParseMinor(strings.TrimSpace(payStr.Get()), currency.Decimals(base))
		apr, errA := strconv.ParseFloat(strings.TrimSpace(aprStr.Get()), 64)
		switch {
		case errB != nil || errP != nil || errA != nil:
			resultBody = P(Class("err"), "Enter valid numbers for balance, APR, and payment.")
		default:
			if r, ok := payoff.Project(bal, apr, pay); ok {
				resultBody = Div(Class("stat-grid"),
					stat("Months to pay off", fmt.Sprintf("%d", r.Months), ""),
					stat("Total interest", fmtMoney(money.New(r.TotalInterest, base)), "neg"),
					stat("Total paid", fmtMoney(money.New(r.TotalPaid, base)), ""),
				)
			} else {
				resultBody = P(Class("err"), "That payment won't cover the interest — the balance would never clear. Try a larger payment.")
			}
		}
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Debt payoff calculator"),
			P(Class("muted"), "See how long a debt takes to clear and how much interest it costs."),
			Form(Class("form-grid"),
				Input(Class("field"), Type("number"), Placeholder("Balance owed ("+base+")"), Value(balStr.Get()), Step("0.01"), OnInput(onBal)),
				Input(Class("field"), Type("number"), Placeholder("APR %"), Value(aprStr.Get()), Step("0.01"), OnInput(onApr)),
				Input(Class("field"), Type("number"), Placeholder("Monthly payment ("+base+")"), Value(payStr.Get()), Step("0.01"), OnInput(onPay)),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Projection"),
			resultBody,
		),
	)
}
