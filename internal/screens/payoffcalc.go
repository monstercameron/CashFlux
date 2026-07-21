// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PayoffCalculatorPanelProps configures a PayoffCalculatorPanel. It carries no
// fields today (the panel reads base currency from appstate), but the struct
// keeps the component CreateElement-shaped and trivially extensible.
type PayoffCalculatorPanelProps struct{}

// PayoffCalculatorPanel is the manual single-debt payoff what-if: enter a
// balance, APR, monthly payment, and an optional extra payment to see months to
// debt-free, the payoff date, and total interest/paid, with an extra-payment
// impact note. It is a registered component (ui.CreateElement) so its four input
// hooks live in their own isolated scope.
//
// It was moved off /planning to /debt as part of the themed remap (FEATURE_MAP
// §5.3): debt content belongs on the "What you owe" page, so /planning stays a
// pure forecasting screen. The compute is identical to the former inline block.
func PayoffCalculatorPanel(_ PayoffCalculatorPanelProps) ui.Node {
	base := "USD"
	if app := appstate.Default; app != nil {
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
		case bal <= 0 || pay <= 0 || apr < 0:
			// The number inputs carry min="0", but a typed/pasted negative still reaches
			// here — a negative balance would short-circuit Project into a bogus "0
			// months, $0" result. Reject the range explicitly instead.
			resultBody = P(css.Class("err"), Attr("role", "alert"), uistate.T("debt.calcRangeError"))
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

	return Div(
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title:        uistate.T("planning.payoffTitle"),
			HeaderAction: debtOwnerLink("/planning", uistate.T("debt.linkPlanning")),
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
