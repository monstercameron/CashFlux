//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Bills lists upcoming payments derived from liability accounts' due-day and
// minimum payment (B22): each bill's next due date, how soon it's due, and the
// amount, soonest first, with the total due up top. Read-only over the pure
// bills core; the month calendar and mark-paid come next.
func Bills() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	upcoming := bills.Upcoming(app.Accounts(), time.Now())

	var total int64
	var rowNodes []ui.Node
	for _, b := range upcoming {
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		total += amt.Amount
		meta := pr.FormatDate(b.DueDate) + " · " + daysUntilLabel(b.DaysUntil)
		rowNodes = append(rowNodes, Div(Class("row"),
			Div(Class("row-main"),
				Span(Class("row-desc"), b.Name),
				Span(Class("row-meta"), meta),
			),
			Span(Class("budget-amount"), fmtMoney(amt)),
		))
	}

	var body ui.Node
	if len(rowNodes) == 0 {
		body = P(Class("empty"), uistate.T("bills.empty"))
	} else {
		body = Div(Class("rows"), rowNodes)
	}

	nextDue := "—"
	if len(upcoming) > 0 {
		nextDue = pr.FormatDate(upcoming[0].DueDate)
	}

	return Div(
		If(len(upcoming) > 0, Div(Class("stat-grid"),
			stat(uistate.T("bills.totalDue"), fmtMoney(money.New(total, base)), "neg"),
			stat(uistate.T("bills.count"), fmt.Sprintf("%d", len(upcoming)), ""),
			stat(uistate.T("bills.nextDue"), nextDue, ""),
		)),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.bills")),
			body,
		),
	)
}

// daysUntilLabel renders how soon a bill is due in friendly terms.
func daysUntilLabel(n int) string {
	switch {
	case n <= 0:
		return uistate.T("bills.dueToday")
	case n == 1:
		return uistate.T("bills.dueTomorrow")
	default:
		return uistate.T("bills.dueInDays", n)
	}
}
