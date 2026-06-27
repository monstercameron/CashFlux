// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// billsHorizonDays is the default look-ahead window. Bills beyond this are hidden
// unless the user enables "Show all" (G11 follow-up).
const billsHorizonDays = 90

// Bills lists upcoming payments derived from liability accounts' due-day and
// minimum payment (B22): each bill's next due date, how soon it's due, and the
// amount, soonest first, with the total due up top, a month calendar, and a
// per-bill "Mark paid" that logs a payment transaction (C57).
func Bills() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	// showAll controls whether bills beyond the 90-day default horizon are shown
	// (G11 follow-up: default horizon + "Show all" toggle).
	showAll := ui.UseState(false)
	toggleShowAll := ui.UseEvent(Prevent(func() { showAll.Set(!showAll.Get()) }))

	// C148: the bills calendar was locked to the current month. calMonthOffset lets
	// the user page forward/back through months (0 = this month) so they can see
	// what's due next month or review last month, not just the current grid.
	calMonthOffset := ui.UseState(0)
	calPrev := ui.UseEvent(Prevent(func() { calMonthOffset.Set(calMonthOffset.Get() - 1) }))
	calNext := ui.UseEvent(Prevent(func() { calMonthOffset.Set(calMonthOffset.Get() + 1) }))
	calToday := ui.UseEvent(Prevent(func() { calMonthOffset.Set(0) }))

	now := time.Now()
	allUpcoming := bills.UpcomingAll(app.Accounts(), app.Recurring(), now)

	// Apply the 90-day horizon filter unless "Show all" is active.
	horizon := now.AddDate(0, 0, billsHorizonDays)
	upcoming := allUpcoming
	if !showAll.Get() {
		filtered := make([]bills.Bill, 0, len(allUpcoming))
		for _, b := range allUpcoming {
			if !b.DueDate.After(horizon) {
				filtered = append(filtered, b)
			}
		}
		upcoming = filtered
	}

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

	// Compute page-level smart insights once (not per row) so each BillRow can call
	// smartBadgeFor with its AccountID. Bills use account IDs as the related entity
	// (PageBills engines set RelatedID = account.ID for each liability account).
	// Pure computation — no hooks needed; re-renders whenever rev changes above.
	billSmartSettings := uistate.LoadSmartSettings()
	billSmartIn := buildSmartInput(app, pr.WeekStartWeekday())
	billInsights := smartengine.RunPage(billSmartIn, billSmartSettings, smart.PageBills)
	billByEntity := insightsByEntity(billInsights)

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
			return ui.CreateElement(BillRow, billRowProps{
				Data: r, OnRemind: remind, OnMarkPaid: markPaid,
				SmartSettings: billSmartSettings,
				SmartByEntity: billByEntity,
			})
		},
	)

	var body ui.Node
	if len(billRows) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("bills.empty"), CTALabel: uistate.T("bills.addFirst"), Href: "/accounts"})
	} else {
		body = Div(css.Class("rows"), rows)
	}

	nextDue := "—"
	if len(upcoming) > 0 {
		nextDue = pr.FormatDate(upcoming[0].DueDate)
	}

	// toggleLabel for the horizon toggle: show which mode we're switching to.
	var toggleLabel string
	if showAll.Get() {
		toggleLabel = "Show next 90 days"
	} else {
		toggleLabel = fmt.Sprintf("Show all (%d)", len(allUpcoming))
	}

	// bills-layout: stacked by default; two-column (list left, calendar right) at
	// ≥1024 px via CSS so the calendar is visible alongside the list (G11 follow-up).
	return Div(
		If(len(upcoming) > 0, Div(css.Class("stat-grid"),
			// Total due is the key bills figure — tooltip explains what it covers.
			Div(css.Class("stat"),
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("bills.totalDue"),
					smartTooltipFor(billSmartSettings, "bills-due", uistate.T("bills.totalDue"), uistate.T("smart.tipBillsDue")),
				),
				Div(css.Class("stat-value is-hero "+tw.ColorClass("text-down")), fmtMoney(money.New(total, base))),
			),
			stat(uistate.T("bills.annualCost"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("bills.count"), fmt.Sprintf("%d", len(upcoming)), ""),
			stat(uistate.T("bills.nextDue"), nextDue, ""),
		)),
		Div(css.Class("bills-layout"),
			uiw.EntityListSection(uiw.EntityListSectionProps{
				Title:        uistate.T("nav.bills"),
				HeaderAction: smartSectionAction(billSmartSettings),
				Body: Fragment(
					body,
					Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
						If(len(allUpcoming) > 0,
							Button(css.Class("btn btn-sm"), Type("button"), OnClick(toggleShowAll), toggleLabel),
						),
						If(len(upcoming) > 0, Button(css.Class("btn"), Type("button"), Title(uistate.T("bills.downloadCsvTitle")), OnClick(func() {
							csvAmount := func(m money.Money) string {
								c, err := rates.Convert(m, base)
								if err != nil {
									c = money.New(m.Amount, base)
								}
								return money.FormatMinor(c.Amount, currency.Decimals(base))
							}
							downloadBytes("bills.csv", "text/csv", bills.CSV(upcoming, csvAmount))
						}), uistate.T("bills.downloadCsv"))),
					),
				),
			}),
			If(len(allUpcoming) > 0, func() ui.Node {
				dispMonth := dateutil.AddMonths(dateutil.MonthStart(now), calMonthOffset.Get())
				nav := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap1),
					Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-prev"),
						Attr("aria-label", uistate.T("bills.calPrev")), Title(uistate.T("bills.calPrev")), OnClick(calPrev), "◀"),
					If(calMonthOffset.Get() != 0, Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-today"),
						OnClick(calToday), uistate.T("bills.calThisMonth"))),
					Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-next"),
						Attr("aria-label", uistate.T("bills.calNext")), Title(uistate.T("bills.calNext")), OnClick(calNext), "▶"),
				)
				return uiw.EntityListSection(uiw.EntityListSectionProps{
					Title:        uistate.T("bills.calendar", monthLabel(dispMonth)),
					HeaderAction: nav,
					Body:         billsCalendar(bills.MonthCalendar(allUpcoming, dispMonth.Year(), dispMonth.Month(), pr.WeekStartWeekday()), pr.WeekStartWeekday(), now),
				})
			}()),
		),
	)
}

// monthLabel renders a month/year heading like "June 2026".
func monthLabel(t time.Time) string { return t.Format("January 2006") }

// billsCalendar renders the month grid: weekday headers plus a cell per day,
// dimming out-of-month days, outlining today, and dotting days with bills due.
func billsCalendar(grid [][]bills.CalendarDay, weekStart time.Weekday, now time.Time) ui.Node {
	todayKey := now.Format("2006-01-02")
	args := []any{css.Class("cal-grid")}
	for i := 0; i < 7; i++ {
		wd := time.Weekday((int(weekStart) + i) % 7)
		args = append(args, Div(css.Class("cal-head"), wd.String()[:3]))
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
				// C150: the dot now conveys amount (per-bill name + amount in the title)
				// and urgency (color: danger when due today/overdue, warn within 3 days,
				// neutral when further out). Multiple bills on a day show the count.
				names := ""
				for i, bb := range day.Bills {
					if i > 0 {
						names += ", "
					}
					names += bb.Name + " (" + fmtMoney(bb.Amount) + ")"
				}
				dotCls := "cal-dot"
				switch d := day.Bills[0].DaysUntil; {
				case d <= 0:
					dotCls += " cal-dot--danger"
				case d <= 3:
					dotCls += " cal-dot--warn"
				default:
					dotCls += " cal-dot--soon"
				}
				if len(day.Bills) > 1 {
					// Render the count inside the dot so a busy day reads at a glance.
					dot = Span(ClassStr(dotCls+" cal-dot--count"), Attr("title", names), Attr("aria-label", names), strconv.Itoa(len(day.Bills)))
				} else {
					dot = Span(ClassStr(dotCls), Attr("title", names), Attr("aria-label", names))
				}
			}
			args = append(args, Div(ClassStr(cls),
				Span(css.Class("cal-day"), strconv.Itoa(day.Date.Day())),
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
	// Smart badge inputs: SmartSettings + byEntity index from the page's insight run.
	// Bills are liability accounts; the badge key is Bill.AccountID.
	SmartSettings smart.Settings
	SmartByEntity map[string][]smart.Insight
}

// BillRow renders one upcoming bill with action buttons in a fixed trailing group
// so the bill name and metadata have horizontal priority (G11 follow-up). It owns
// its click hooks (per the On*-hooks-in-loops rule) so the list renders safely.
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
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), d.Bill.Name,
				smartBadgeFor(props.SmartSettings, props.SmartByEntity, d.Bill.AccountID),
			),
			Span(ClassStr(metaCls), meta),
			// C154: surface autopay so the user knows this bill is charged automatically
			// (no manual payment needed — just keep funds available).
			If(d.Bill.Autopay, Span(css.Class("pill", tw.TextDim), Attr("data-testid", "bill-autopay"), Attr("title", uistate.T("recurring.autopayHint")), uistate.T("recurring.autopayBadge"))),
		),
		Span(css.Class("budget-amount"), fmtMoney(d.Shown)),
		// bill-sub-actions: fixed trailing group so action buttons don't crowd the
		// name/amount area, mirroring the .sub-actions pattern from G10 (G11 follow-up).
		Div(css.Class("bill-sub-actions"),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"), Title(uistate.T("bills.markPaidTitle")), OnClick(markPaid), uistate.T("bills.markPaid")),
			Button(css.Class("btn btn-sm"), Type("button"), Title(uistate.T("bills.remindTitle")), OnClick(remind), uistate.T("bills.remind")),
		),
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
