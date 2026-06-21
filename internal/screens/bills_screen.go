//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
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
// amount, soonest first, with the total due up top, a month calendar, and a
// per-bill "Mark paid" that logs a payment transaction (C57).
func Bills() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(ClassStr("card"), P(ClassStr("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	now := time.Now()
	upcoming := bills.UpcomingAll(app.Accounts(), app.Recurring(), now)

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
	rev := uistate.UseDataRevision()
	markPaid := func(b bills.Bill) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.RecordBillPayment(b.AccountID, b.Name, b.Amount); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		notice.Set(notice.Get().With(uistate.T("bills.paidLogged", b.Name), false))
		rev.Set(rev.Get() + 1)
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

	// Cadence-correct yearly total: annualize each obligation by its own cadence,
	// then FX-convert and sum (C57) — not total×12, which mixes cadences.
	var annual int64
	for _, m := range bills.AnnualAmounts(app.Accounts(), app.Recurring()) {
		c, err := rates.Convert(m, base)
		if err != nil {
			c = money.New(m.Amount, base)
		}
		annual += c.Amount
	}

	rows := MapKeyed(billRows,
		// Composite key (account + due date + name): one account can yield more than
		// one bill (a liability statement plus a recurring on the same account), so
		// keying by AccountID alone would collide and silently drop a row (C57).
		func(r billRowData) any {
			return r.Bill.AccountID + "|" + r.Bill.DueDate.Format("2006-01-02") + "|" + r.Bill.Name
		},
		func(r billRowData) ui.Node {
			return ui.CreateElement(BillRow, billRowProps{Data: r, OnRemind: remind, OnMarkPaid: markPaid})
		},
	)

	var body ui.Node
	if len(billRows) == 0 {
		body = P(ClassStr("empty"), uistate.T("bills.empty"))
	} else {
		body = Div(ClassStr("rows"), rows)
	}

	nextDue := "—"
	if len(upcoming) > 0 {
		nextDue = pr.FormatDate(upcoming[0].DueDate)
	}

	return Div(
		If(len(upcoming) > 0, Div(ClassStr("stat-grid"),
			stat(uistate.T("bills.totalDue"), fmtMoney(money.New(total, base)), "neg"),
			stat(uistate.T("bills.annualCost"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("bills.count"), fmt.Sprintf("%d", len(upcoming)), ""),
			stat(uistate.T("bills.nextDue"), nextDue, ""),
		)),
		Section(ClassStr("card"),
			H2(ClassStr("card-title"), uistate.T("nav.bills")),
			body,
			If(len(upcoming) > 0, Div(ClassStr("flex flex-wrap gap-2 py-1"),
				Button(ClassStr("btn"), Type("button"), Title(uistate.T("bills.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(m money.Money) string {
						c, err := rates.Convert(m, base)
						if err != nil {
							c = money.New(m.Amount, base)
						}
						return money.FormatMinor(c.Amount, currency.Decimals(base))
					}
					downloadBytes("bills.csv", "text/csv", bills.CSV(upcoming, csvAmount))
				}), uistate.T("bills.downloadCsv")),
			)),
		),
		If(len(upcoming) > 0, Section(ClassStr("card"),
			H2(ClassStr("card-title"), uistate.T("bills.calendar", monthLabel(now))),
			billsCalendar(bills.MonthCalendar(upcoming, now.Year(), now.Month(), pr.WeekStartWeekday()), pr.WeekStartWeekday(), now),
		)),
	)
}

// monthLabel renders a month/year heading like "June 2026".
func monthLabel(t time.Time) string { return t.Format("January 2006") }

// billsCalendar renders the month grid: weekday headers plus a cell per day,
// dimming out-of-month days, outlining today, and dotting days with bills due.
func billsCalendar(grid [][]bills.CalendarDay, weekStart time.Weekday, now time.Time) ui.Node {
	todayKey := now.Format("2006-01-02")
	args := []any{ClassStr("cal-grid")}
	for i := 0; i < 7; i++ {
		wd := time.Weekday((int(weekStart) + i) % 7)
		args = append(args, Div(ClassStr("cal-head"), wd.String()[:3]))
	}
	for _, week := range grid {
		for _, day := range week {
			cls := "cal-cell"
			if !day.InMonth {
				cls += " out"
			}
			if day.Date.Format("2006-01-02") == todayKey {
				cls += " today"
			}
			var dot ui.Node = Fragment()
			if len(day.Bills) > 0 {
				names := day.Bills[0].Name
				for _, b := range day.Bills[1:] {
					names += ", " + b.Name
				}
				dot = Span(ClassStr("cal-dot"), Attr("title", names))
			}
			args = append(args, Div(ClassStr(cls),
				Span(ClassStr("cal-day"), strconv.Itoa(day.Date.Day())),
				dot,
			))
		}
	}
	return Div(args...)
}

// billRowData is one bill plus its display-ready amount and date.
type billRowData struct {
	Bill     bills.Bill
	Shown    money.Money // amount converted to the base currency
	DueLabel string      // pre-formatted due date
}

type billRowProps struct {
	Data       billRowData
	OnRemind   func(b bills.Bill, shown money.Money, dueLabel string)
	OnMarkPaid func(b bills.Bill)
}

// BillRow renders one upcoming bill with a "remind me" action. It owns its click
// hook (per the On*-hooks-in-loops rule) so the list can render many rows safely.
func BillRow(props billRowProps) ui.Node {
	d := props.Data
	remind := ui.UseEvent(Prevent(func() { props.OnRemind(d.Bill, d.Shown, d.DueLabel) }))
	markPaid := ui.UseEvent(Prevent(func() {
		if props.OnMarkPaid != nil {
			props.OnMarkPaid(d.Bill)
		}
	}))
	meta := d.DueLabel + " · " + daysUntilLabel(d.Bill.DaysUntil)
	// Urgency tone so an imminent bill stands out at a glance (C57): danger when
	// due today/past, warn within three days. The "due today / in N days" wording
	// carries the meaning too, so it's colour + text (B15).
	metaCls := "row-meta"
	if t := billUrgencyTone(d.Bill.DaysUntil); t != "" {
		metaCls += " " + t
	}
	return Div(ClassStr("row"),
		Div(ClassStr("row-main"),
			Span(ClassStr("row-desc"), d.Bill.Name),
			Span(ClassStr(metaCls), meta),
		),
		Span(ClassStr("budget-amount"), fmtMoney(d.Shown)),
		Button(ClassStr("btn btn-primary"), Type("button"), Title(uistate.T("bills.markPaidTitle")), OnClick(markPaid), uistate.T("bills.markPaid")),
		Button(ClassStr("btn"), Type("button"), Title(uistate.T("bills.remindTitle")), OnClick(remind), uistate.T("bills.remind")),
	)
}

// billUrgencyTone maps days-until-due to a tone class: danger when due today or
// past, warn within three days, none otherwise (C57).
func billUrgencyTone(n int) string {
	switch {
	case n <= 0:
		return "text-down"
	case n <= 3:
		return "text-warn"
	default:
		return ""
	}
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
