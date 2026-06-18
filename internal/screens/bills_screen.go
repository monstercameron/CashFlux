//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
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

	// remind creates a to-do dated to the bill's due date, so a "pay this" task
	// surfaces in time (B22, via the existing to-do system).
	notice := uistate.UseNotice()
	remind := func(b bills.Bill, shown money.Money, dueLabel string) {
		app := appstate.Default
		if app == nil {
			return
		}
		task := domain.Task{
			ID:       id.New(),
			Title:    uistate.T("bills.reminderTitle", b.Name),
			Notes:    uistate.T("bills.reminderNote", fmtMoney(shown), dueLabel),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Due:      b.DueDate,
			Source:   domain.SourceNudge,
		}
		if err := app.PutTask(task); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		notice.Set(notice.Get().With(uistate.T("bills.reminderAdded", b.Name), false))
	}

	var total int64
	billRows := make([]billRowData, 0, len(upcoming))
	for _, b := range upcoming {
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		total += amt.Amount
		billRows = append(billRows, billRowData{Bill: b, Shown: amt, DueLabel: pr.FormatDate(b.DueDate)})
	}

	rows := MapKeyed(billRows,
		func(r billRowData) any { return r.Bill.AccountID },
		func(r billRowData) ui.Node {
			return ui.CreateElement(BillRow, billRowProps{Data: r, OnRemind: remind})
		},
	)

	var body ui.Node
	if len(billRows) == 0 {
		body = P(Class("empty"), uistate.T("bills.empty"))
	} else {
		body = Div(Class("rows"), rows)
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

// billRowData is one bill plus its display-ready amount and date.
type billRowData struct {
	Bill     bills.Bill
	Shown    money.Money // amount converted to the base currency
	DueLabel string      // pre-formatted due date
}

type billRowProps struct {
	Data     billRowData
	OnRemind func(b bills.Bill, shown money.Money, dueLabel string)
}

// BillRow renders one upcoming bill with a "remind me" action. It owns its click
// hook (per the On*-hooks-in-loops rule) so the list can render many rows safely.
func BillRow(props billRowProps) ui.Node {
	d := props.Data
	remind := ui.UseEvent(Prevent(func() { props.OnRemind(d.Bill, d.Shown, d.DueLabel) }))
	meta := d.DueLabel + " · " + daysUntilLabel(d.Bill.DaysUntil)
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), d.Bill.Name),
			Span(Class("row-meta"), meta),
		),
		Span(Class("budget-amount"), fmtMoney(d.Shown)),
		Button(Class("btn"), Type("button"), Title(uistate.T("bills.remindTitle")), OnClick(remind), uistate.T("bills.remind")),
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
