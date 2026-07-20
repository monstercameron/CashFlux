// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// agendaWindowDays is the forward horizon for the up-next agenda (through the
// pay cycle; matches the tideline's cap).
const agendaWindowDays = 45

// agendaItem is one merged forward occurrence — a bill (outflow) or a paycheck
// (income). Amount is SIGNED (outflow negative, income positive) so the row tones
// it directly. AccountID is "recurring:<id>" or a real liability account for
// bills, empty for income.
type agendaItem struct {
	Date       time.Time
	Name       string
	AccountID  string
	Amount     money.Money
	ModeLabel  string
	ModeHint   string
	ModeCls    string
	Fit        *billFitChip
	Negotiable bool
	Paid       bool
	Income     bool
	// Past marks an occurrence whose due date has already gone by. The forward
	// agenda never carries these (overdue has its own strip), but the calendar
	// renders whole months, and a month whose first three weeks are blank because
	// they already happened reads as a broken calendar rather than a settled one.
	Past bool
	// Missed marks a past occurrence the app can actually SHOW went unpaid: no
	// settlement evidence, and the flow's own schedule never advanced past it.
	//
	// It is deliberately narrower than "past and not known paid". Absence of a
	// payment record is not evidence of a missed payment — most households settle
	// most bills without ever telling the app — so a past day with nothing known
	// about it recedes quietly instead of accusing.
	Missed bool
	// AnchorAccountID is the liability account this row settles when the row is a
	// merged obligation (the statement bill folded into its recurring flow), so
	// the single row keeps the account identity's capabilities.
	AnchorAccountID string
}

// buildAgenda merges the forward bill occurrences (recurring-derived + liability
// statement) with the income paychecks into one date-sorted agenda over the
// standard pay-cycle window.
func buildAgenda(app *appstate.App, now time.Time, base string) []agendaItem {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return buildAgendaRange(app, now, today.AddDate(0, 0, agendaWindowDays), base)
}

// buildAgendaRange is buildAgenda over an explicit forward horizon — everything
// due between now and until, overdue excluded (it has its own strip).
func buildAgendaRange(app *appstate.App, now, until time.Time, base string) []agendaItem {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	all := buildAgendaSpan(app, now, now, until, base)
	out := all[:0:0]
	for _, it := range all {
		if it.Date.Before(today) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// buildAgendaSpan is the merged agenda model over an ARBITRARY span, which may
// start in the past. The calendar view needs this: it renders whole months, and
// a stored schedule only knows its NEXT due date, so the flows are wound back to
// the start of the span (domain.RecurringCadence.Prev) before being projected
// forward across it. now stays separate from from — it decides what counts as
// already-happened and is not the window.
func buildAgendaSpan(app *appstate.App, now, from, until time.Time, base string) []agendaItem {
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	recByID := map[string]domain.Recurring{}
	for _, r := range app.Recurring() {
		recByID[r.ID] = r
	}
	rewound := rhyRewind(app.Recurring(), start)

	var items []agendaItem
	// A liability's statement bill and the monthly recurring flow that pays it are
	// ONE obligation — listing both double-counts the money owed. DedupeObligations
	// collapses them onto the recurring row and records the liability as its anchor.
	occurrences := bills.DedupeObligations(
		bills.OccurrencesWithin(app.Accounts(), rewound, start, until), app.Recurring())
	for _, b := range occurrences {
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		it := agendaItem{
			Date: b.DueDate, Name: b.Name, AccountID: b.AccountID,
			Amount: money.New(-amt.Amount, base), Fit: billFitFor(b),
			AnchorAccountID: b.AnchorAccountID,
		}
		if rid, ok := recurringIDFromBillAccount(b.AccountID); ok {
			it.ModeLabel, it.ModeHint, it.ModeCls = postingMode(recByID[rid])
			it.Negotiable = !recByID[rid].Autopay
		} else if b.Autopay {
			it.ModeLabel, it.ModeHint, it.ModeCls = uistate.T("rhythm.modeWatch"), uistate.T("rhythm.modeWatchHint"), "is-watch"
		} else {
			it.ModeLabel, it.ModeHint, it.ModeCls = uistate.T("rhythm.modeManual"), uistate.T("rhythm.modeManualHint"), ""
			it.Negotiable = true
		}
		it.Paid = app.OccurrencePaid(b.AccountID, b.DueDate)
		hasSchedule := false
		if rid, ok := recurringIDFromBillAccount(b.AccountID); ok {
			hasSchedule = true
			it.Paid = rhySettled(app, recByID[rid], b.DueDate)
		}
		it.Past = b.DueDate.Before(today)
		// A liability statement has no schedule of its own to reason about, so a
		// past one is never claimed as missed — only shown as history.
		it.Missed = it.Past && !it.Paid && hasSchedule
		items = append(items, it)
	}

	// Income paychecks make the rhythm real.
	for _, r := range rewound {
		if !r.Active() || r.Amount.IsNegative() {
			continue
		}
		amt, err := rates.Convert(r.Amount, base)
		if err != nil {
			amt = money.New(r.Amount.Amount, base)
		}
		label, hint, cls := postingMode(r)
		d := r.NextDue
		for i := 0; i < 24 && !d.After(until); i++ {
			if !d.Before(start) {
				items = append(items, agendaItem{
					Date: d, Name: r.Label, Amount: money.New(amt.Amount, base),
					ModeLabel: label, ModeHint: hint, ModeCls: cls, Income: true,
					Past: d.Before(today),
				})
			}
			next := r.Cadence.Next(d)
			if !next.After(d) {
				break
			}
			d = next
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.Before(items[j].Date) })
	return items
}

// rhySettled is THE test for "this occurrence of this flow is dealt with", used
// by both the overdue strip and the calendar's past-day states.
//
// It has to be one function. The strip and the calendar are two views of the same
// question, and when they answered it differently the page contradicted itself:
// the strip said three items were overdue for $127 while the calendar painted a
// fourth in the same warning tone. A user cannot tell which of two disagreeing
// claims to believe, so both become worthless.
//
// Three kinds of evidence settle an occurrence, and any one is enough:
//
//   - the user marked it paid (the durable occurrence record);
//   - a real transaction matched it (auto-matched bills are paid bills, and a
//     month of them must not render as a month of misses); or
//   - the flow's own schedule has advanced past it — NextDue sitting after this
//     date means the occurrence ran its course and the flow moved on.
//
// Everything else is merely unknown, which is NOT a miss. Absence of a payment
// record is not evidence of a missed payment; most households settle most bills
// without ever telling the app.
func rhySettled(app *appstate.App, r domain.Recurring, due time.Time) bool {
	if app.OccurrencePaid("recurring:"+r.ID, due) {
		return true
	}
	if _, matched := app.BillMatchForOccurrence(r.ID, due); matched {
		return true
	}
	return !r.NextDue.IsZero() && due.Before(r.NextDue)
}

// rhyRewindCap bounds the backward walk so a flow whose NextDue sits far in the
// future (or a degenerate imported cadence) cannot spin.
const rhyRewindCap = 400

// rhyRewind returns copies of the flows with NextDue wound back to the last
// occurrence on or before from, so a projection starting at from sees the
// occurrences that already happened rather than only the ones still ahead.
//
// The store keeps a schedule as its NEXT due date, which is everything a forward
// agenda needs and nothing a calendar does: asking "what was due on the 12th of
// last month" of a flow whose NextDue is the 12th of next month gets silence,
// which is what made past weeks render blank. Winding back through the cadence's
// own inverse (never a fixed day count — months are not 30 days long) keeps the
// anchor day exactly where the schedule puts it.
func rhyRewind(recs []domain.Recurring, from time.Time) []domain.Recurring {
	out := make([]domain.Recurring, 0, len(recs))
	for _, r := range recs {
		if !r.NextDue.IsZero() {
			d := r.NextDue
			for i := 0; i < rhyRewindCap && d.After(from); i++ {
				prev := r.Cadence.Prev(d)
				if !prev.Before(d) {
					break // a cadence that does not move backwards cannot be wound
				}
				d = prev
			}
			r.NextDue = d
		}
		out = append(out, r)
	}
	return out
}

// rhyAgendaProps configures the agenda section.
type rhyAgendaProps struct {
	Focus rhythmFocus
	Acts  rhyActions
}

// rhyAgendaSection is the up-next agenda with a persisted COMPACT | CALENDAR
// toggle. Its own component so the view + calendar-paging state stays isolated.
func rhyAgendaSection(props rhyAgendaProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	view := ui.UseState(uistate.AgendaViewGet())
	setView := func(v string) {
		view.Set(v)
		uistate.AgendaViewSet(v)
	}
	onCompact := ui.UseEvent(Prevent(func() { setView(uistate.AgendaViewCompact) }))
	onCalendar := ui.UseEvent(Prevent(func() { setView(uistate.AgendaViewCalendar) }))
	showAll := ui.UseState(false)
	toggleAll := ui.UseEvent(Prevent(func() { showAll.Set(!showAll.Get()) }))
	// The calendar opens on a month that has something IN it. Landing on today's
	// month unconditionally meant that opening the page late in the month showed a
	// grid whose remaining days were empty and whose next obligations were one
	// click away in a month the user could not see — strictly worse than the
	// compact list it is meant to be a peer of.
	calOffset := ui.UseState(rhyCalendarLanding(app, now, base))
	calPrev := ui.UseEvent(Prevent(func() { calOffset.Set(calOffset.Get() - 1) }))
	calNext := ui.UseEvent(Prevent(func() { calOffset.Set(calOffset.Get() + 1) }))
	calToday := ui.UseEvent(Prevent(func() { calOffset.Set(0) }))

	toggle := Div(css.Class("rhy-view-toggle", tw.InlineFlex, tw.Gap1), Attr("role", "group"), Attr("aria-label", uistate.T("rhythm.viewAria")),
		rhyToggleBtn(uistate.T("rhythm.viewCompact"), "rhy-view-compact", view.Get() == uistate.AgendaViewCompact, onCompact),
		rhyToggleBtn(uistate.T("rhythm.viewCalendar"), "rhy-view-calendar", view.Get() == uistate.AgendaViewCalendar, onCalendar),
	)

	// The note describes the window each view actually draws. The compact list runs
	// the whole agenda horizon — which is longer than one pay cycle, and saying
	// otherwise while listing September was the page overpromising its own scope.
	var body ui.Node
	note := uistate.T("rhythm.agendaNote", agendaWindowDays)
	if view.Get() == uistate.AgendaViewCalendar {
		body = rhyAgendaCalendar(app, now, calOffset.Get(), base, calPrev, calNext, calToday)
		note = uistate.T("rhythm.agendaNoteCal")
	} else {
		body = rhyAgendaCompact(buildAgenda(app, now, base), base, showAll.Get(), toggleAll)
	}
	return rhySection("sec-agenda", uistate.T("rhythm.agendaTitle"), note, toggle, body)
}

// rhyToggleBtn renders one segment of the view toggle.
func rhyToggleBtn(label, testid string, on bool, onClick any) ui.Node {
	cls := "rhy-lens"
	if on {
		cls += " is-on"
	}
	return Button(ClassStr(cls), Type("button"), Attr("data-testid", testid), Attr("aria-pressed", ariaBool(on)),
		OnClick(onClick), label)
}

// rhyAgendaCompact renders the dense single-line agenda (default), capped with a
// real "show all" expander rather than a silent "+N more".
//
// The list is broken by month. The window runs past a single cycle, so a monthly
// commitment legitimately appears in it twice — HOA dues on the 1st of August AND
// the 1st of September. Undivided, that reads as owing the same bill twice, which
// is the worst thing a bills list can say. A month heading costs one dim line and
// makes the repetition mean what it actually means.
func rhyAgendaCompact(items []agendaItem, base string, showAll bool, onToggleAll any) ui.Node {
	if len(items) == 0 {
		return P(css.Class("muted"), Attr("data-testid", "rhy-agenda-none"), uistate.T("rhythm.agendaNone"))
	}
	const maxRows = 12
	rows := []any{css.Class("rhy-agenda-list"), Attr("role", "list"), Attr("data-testid", "rhy-agenda")}
	shown := items
	if !showAll && len(items) > maxRows {
		shown = items[:maxRows]
	}
	month := ""
	for _, it := range shown {
		row := it
		if m := monthLabel(row.Date); m != month {
			month = m
			// No testid: it would have to carry the month, and a testid that changes
			// every month is a CI failure with a date on it. The class is the handle.
			rows = append(rows, Div(css.Class("rhy-ag-month"), Attr("role", "presentation"), m))
		}
		rows = append(rows, ui.CreateElement(rhyAgendaRow, rhyAgendaRowProps{Item: row, Base: base}))
	}
	list := Div(rows...)
	if len(items) > maxRows {
		label := uistate.T("rhythm.showAll", len(items))
		if showAll {
			label = uistate.T("rhythm.showFewer")
		}
		return Fragment(list, Button(css.Class("btn btn-sm", tw.Mt2), Type("button"),
			Attr("data-testid", "rhy-agenda-showall"), OnClick(onToggleAll), label))
	}
	return list
}

// rhyCalendarLanding picks the month the calendar opens on, as an offset from
// today's month: today's month when it still holds an obligation ahead, else the
// month containing the next one.
//
// It deliberately does NOT jump to a month merely because today's is sparse —
// the current month is where a user expects to land, and paging is one click.
// The jump happens only when this month has nothing left to show at all, which
// is the case that made the calendar read as broken.
func rhyCalendarLanding(app *appstate.App, now time.Time, base string) int {
	thisMonthEnd := dateutil.AddMonths(dateutil.MonthStart(now), 1)
	ahead := buildAgendaRange(app, now, thisMonthEnd.AddDate(0, 0, agendaWindowDays), base)
	for _, it := range ahead {
		if it.Date.Before(thisMonthEnd) {
			return 0 // this month still has something in it
		}
	}
	if len(ahead) == 0 {
		return 0
	}
	return rhyMonthOffset(now, ahead[0].Date)
}

// rhyMonthOffset counts whole calendar months from a's month to b's month.
func rhyMonthOffset(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// rhyAgendaCalendar renders the month-grid view of the SAME agenda data as the
// compact list — real amounts on the days, not bare dots, with income visually
// distinct from outflow. It keeps the cal-prev/cal-next/cal-today testids.
//
// The window is the WHOLE displayed month, not today onward: a calendar that
// only draws the future leaves the days that already happened blank, which reads
// as missing data rather than as settled obligations. Past days carry what
// actually happened — paid, or missed.
func rhyAgendaCalendar(app *appstate.App, now time.Time, offset int, base string, calPrev, calNext, calToday any) ui.Node {
	pr := uistate.LoadPrefs()
	disp := dateutil.AddMonths(dateutil.MonthStart(now), offset)
	monthEnd := dateutil.AddMonths(disp, 1)
	// Bucket the merged agenda (bills + income) by calendar day. The grid's
	// leading/trailing edge days belong to the neighbouring months, so the span is
	// widened a week each way to fill them too.
	byDay := map[string][]agendaItem{}
	for _, it := range buildAgendaSpan(app, now, disp.AddDate(0, 0, -7), monthEnd.AddDate(0, 0, 7), base) {
		k := it.Date.Format("2006-01-02")
		byDay[k] = append(byDay[k], it)
	}
	nav := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap1),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "cal-prev"),
			Attr("aria-label", uistate.T("bills.calPrev")), Title(uistate.T("bills.calPrev")), OnClick(calPrev), "◀"),
		If(offset != 0, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "cal-today"),
			OnClick(calToday), uistate.T("bills.calThisMonth"))),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "cal-next"),
			Attr("aria-label", uistate.T("bills.calNext")), Title(uistate.T("bills.calNext")), OnClick(calNext), "▶"),
	)
	// MonthCalendar supplies the tested date scaffolding (weeks, in/out-of-month);
	// the cells are ours so they can carry amounts.
	grid := bills.MonthCalendar(nil, disp.Year(), disp.Month(), pr.WeekStartWeekday())
	return Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb2),
			Span(css.Class("rhy-sec-note"), Style(map[string]string{"margin": "0"}), monthLabel(disp)), nav),
		rhyCalendarGrid(grid, pr.WeekStartWeekday(), now, byDay),
	)
}

// rhyCalendarGrid draws the month grid with the day's real amounts in-cell.
// Display-only (no hooks), so it is safe inside the day loop.
func rhyCalendarGrid(grid [][]bills.CalendarDay, weekStart time.Weekday, now time.Time, byDay map[string][]agendaItem) ui.Node {
	todayKey := now.Format("2006-01-02")
	args := []any{css.Class("cal-grid rhy-cal")}
	for i := 0; i < 7; i++ {
		wd := time.Weekday((int(weekStart) + i) % 7)
		args = append(args, Div(css.Class("cal-head"), wd.String()[:3]))
	}
	for _, week := range grid {
		for _, day := range week {
			key := day.Date.Format("2006-01-02")
			cls := "cal-cell rhy-cal-cell"
			if !day.InMonth {
				cls += " out"
			}
			if key == todayKey {
				cls += " today"
			}
			cell := []any{ClassStr(cls), Attr("data-testid", "rhy-cal-"+key),
				Span(css.Class("rhy-cal-day"), day.Date.Format("2"))}
			items := byDay[key]
			const maxChips = 3
			for i, it := range items {
				if i >= maxChips {
					cell = append(cell, Span(css.Class("rhy-cal-more"), uistate.T("rhythm.calMore", len(items)-maxChips)))
					break
				}
				// A calendar of anonymous numbers is half a feature — the name says
				// WHAT is due; CSS truncates it when the cell is narrow.
				chip := "rhy-cal-item"
				tip := it.Name + " · " + fmtMoney(it.Amount)
				// A day that has gone by says what HAPPENED, not what was due — but
				// only as far as the app actually knows. Settled, genuinely missed,
				// and simply-in-the-past are three different claims and read as three
				// different things.
				if it.Income {
					chip += " is-in"
				}
				switch {
				case it.Past && it.Paid:
					chip += " is-done"
					tip += " · " + uistate.T("bills.paidBadge")
				case it.Missed:
					chip += " is-missed"
					tip += " · " + uistate.T("rhythm.calMissed")
				case it.Past:
					chip += " is-past"
					tip += " · " + uistate.T("rhythm.calPast")
				}
				cell = append(cell, Div(ClassStr(chip), Title(tip),
					Span(css.Class("rhy-cal-name"), it.Name),
					Span(css.Class("rhy-cal-amt"), fmtMoney(it.Amount)),
				))
			}
			args = append(args, Div(cell...))
		}
	}
	return Div(args...)
}

// rhyAgendaRowProps drives one compact agenda row.
type rhyAgendaRowProps struct {
	Item agendaItem
	Base string
}

// rhyAgendaRow renders one dense agenda line: date · name · mode badge · budget-
// fit chip · amount · inline Mark-paid, with a per-row kebab holding Negotiate
// (which carries the bill-negotiate-<accountID> testid). Its own component so the
// action hooks stay stable.
func rhyAgendaRow(props rhyAgendaRowProps) ui.Node {
	it := props.Item
	markPaid := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		pay := money.New(-it.Amount.Amount, props.Base) // positive magnitude
		if err := app.RecordBillPayment(it.AccountID, it.Name, pay); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		if err := app.MarkOccurrencePaid(it.AccountID, it.Date); err != nil {
			app.Log().Error("mark occurrence paid", "billID", it.AccountID, "err", err)
		}
		uistate.BumpDataRevision()
	}))
	// The budget-fit chip is a CONTROL, not a caption: "this fits Groceries" is a
	// claim about another screen, and the chip is one click from the card that
	// makes it (reusing the notification deep-link focus machinery, so the
	// receiving budget flashes on arrival).
	nav := router.UseNavigate()
	openFit := ui.UseEvent(Prevent(func() {
		if it.Fit == nil {
			return
		}
		uistate.SetDeepLinkFocus(`[data-testid="budget-card-` + it.Fit.BudgetID + `"]`)
		nav.Navigate(uistate.RoutePath("/budgets"))
	}))
	negotiate := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		t := domain.Task{
			ID: id.New(), Title: uistate.T("bills.negotiateTaskTitle", it.Name),
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceNudge,
		}
		if err := app.PutTask(t); err == nil {
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("bills.negotiate"), false)
		}
	}))

	rowCls := "rhy-ag-row"
	var verb ui.Node = Fragment()
	if !it.Income {
		if it.Paid {
			verb = Span(css.Class("pill", tw.ColorClass("text-ok")), Attr("data-testid", "bill-paid"),
				Title(uistate.T("bills.paidBadgeTitle")), uistate.T("bills.paidBadge"))
		} else {
			verb = Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "rhy-ag-markpaid-"+it.AccountID),
				Title(uistate.T("bills.markPaidTitle")), OnClick(markPaid), uistate.T("bills.markPaid"))
		}
	}

	var fitNode ui.Node = Fragment()
	if it.Fit != nil {
		cls := "pill bill-fit bill-fit-ok"
		label := uistate.T("bills.budgetFits", it.Fit.BudgetName, it.Fit.Amount)
		if !it.Fit.Fits {
			cls = "pill bill-fit bill-fit-over"
			label = uistate.T("bills.budgetOver", it.Fit.Amount, it.Fit.BudgetName)
		}
		fitNode = Button(ClassStr(cls), Type("button"),
			Attr("data-testid", "bill-fit-"+it.AccountID),
			Attr("aria-label", uistate.T("bills.budgetFitAria", it.Fit.BudgetName)),
			Title(uistate.T("bills.budgetFitAria", it.Fit.BudgetName)),
			OnClick(openFit), label)
	}

	var kebab ui.Node = Fragment()
	if it.Negotiable {
		kebab = uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "rhy-ag-menu-" + it.AccountID,
			AriaLabel:    uistate.T("bills.moreActions") + " — " + it.Name,
			ToggleClass:  "btn btn-sm",
			ToggleTestID: "rhy-ag-menu-" + it.AccountID,
			Items: []ui.Node{
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "bill-negotiate-"+it.AccountID),
					Title(uistate.T("bills.negotiateTitle")), OnClick(negotiate), uistate.T("bills.negotiate")),
			},
		})
	}

	return Div(ClassStr(rowCls), Attr("role", "listitem"),
		Span(css.Class("rhy-ag-date"), uistate.LoadPrefs().FormatDate(it.Date)),
		Div(css.Class("rhy-ag-body"),
			Span(css.Class("rhy-ag-name"), it.Name),
			Span(ClassStr("rhy-badge "+it.ModeCls), Title(it.ModeHint), it.ModeLabel),
			// A merged obligation keeps the liability it settles visible, so folding
			// the statement bill into the flow loses no information.
			If(it.AnchorAccountID != "", Span(css.Class("rhy-chip"),
				Title(uistate.T("rhythm.anchorTitle", rhyAccountName(it.AnchorAccountID))),
				rhyAccountName(it.AnchorAccountID))),
			fitNode,
		),
		Span(ClassStr("rhy-ag-amt "+recurAmountTone(it.Amount)), fmtMoney(it.Amount)),
		Div(css.Class("rhy-ag-verb"), verb, kebab),
	)
}
