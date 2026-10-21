// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recurringview"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// agendaWindowDays is the forward horizon for the up-next agenda (through the
// pay cycle; matches the tideline's cap).
const agendaWindowDays = 45

// agendaItem adds only display and action affordances to native recurring facts.
type agendaItem struct {
	recurringview.AgendaOccurrence
	ModeLabel  string
	ModeHint   string
	ModeCls    string
	Fit        *billFitChip
	Negotiable bool
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
	span := prepareAgendaSpan(app, now, now, until, base)
	return presentAgendaSpan(app, recurringview.ForwardAgenda(span, now), now)
}

// prepareAgendaSpan gathers durable evidence, then delegates occurrence dates,
// merge policy, FX conversion, and settlement rules to the pure recurring view.
func prepareAgendaSpan(app *appstate.App, now, from, until time.Time, base string) recurringview.AgendaSpan {
	if app == nil {
		return recurringview.AgendaSpan{Base: base}
	}
	settings := app.Settings()
	recurring := app.Recurring()
	span := recurringview.BuildAgendaSpan(recurringview.AgendaSpanInput{
		Accounts: app.Accounts(), Recurring: recurring, Now: now, From: from, Until: until, Base: base,
		Rates: currency.Rates{Base: base, Rates: settings.FXRates},
	})

	matched := app.BillMatchPaidOccurrences()
	evidence := make([]recurringview.AgendaSettlementEvidence, 0, len(span.Occurrences))
	for _, occurrence := range span.Occurrences {
		if occurrence.Income {
			continue
		}
		paid := app.OccurrencePaid(occurrence.AccountID, occurrence.Date)
		isMatched := occurrence.RecurringID != "" && matched[billmatch.Key(occurrence.RecurringID, occurrence.Date)]
		if paid || isMatched {
			evidence = append(evidence, recurringview.AgendaSettlementEvidence{
				BillID: occurrence.AccountID, RecurringID: occurrence.RecurringID,
				DueDate: occurrence.Date, Paid: paid, Matched: isMatched,
			})
		}
	}
	return recurringview.WithSettlementEvidence(span, evidence)
}

// presentAgendaSpan attaches UI labels and budget-fit chips to prepared facts.
func presentAgendaSpan(app *appstate.App, span recurringview.AgendaSpan, now time.Time) []agendaItem {
	if app == nil || len(span.Occurrences) == 0 {
		return nil
	}
	recurring := app.Recurring()
	recByID := make(map[string]domain.Recurring, len(recurring))
	var candidates []bills.Bill
	for _, flow := range recurring {
		recByID[flow.ID] = flow
	}
	for _, occurrence := range span.Occurrences {
		if !occurrence.Income {
			candidates = append(candidates, occurrence.Bill)
		}
	}
	fitModel := prepareBillFitModel(app, candidates, now)
	items := make([]agendaItem, 0, len(span.Occurrences))
	for _, fact := range span.Occurrences {
		item := agendaItem{AgendaOccurrence: fact}
		if fact.Income {
			item.ModeLabel, item.ModeHint, item.ModeCls = postingMode(recByID[fact.RecurringID])
		} else {
			item.Fit = billFitFor(fitModel, fact.Bill)
			if rid, ok := bills.RecurringIDFromAccount(fact.AccountID); ok {
				flow := recByID[rid]
				item.ModeLabel, item.ModeHint, item.ModeCls = postingMode(flow)
				item.Negotiable = !flow.Autopay
			} else if fact.Bill.Autopay {
				item.ModeLabel, item.ModeHint, item.ModeCls = uistate.T("rhythm.modeWatch"), uistate.T("rhythm.modeWatchHint"), "is-watch"
			} else {
				item.ModeLabel, item.ModeHint, item.ModeCls = uistate.T("rhythm.modeManual"), uistate.T("rhythm.modeManualHint"), ""
				item.Negotiable = true
			}
		}
		items = append(items, item)
	}
	return items
}

// rhySettled gathers app-state evidence for the overdue strip and keeps the
// shared settlement rule in recurringview.
func rhySettled(app *appstate.App, recurring domain.Recurring, due time.Time) bool {
	if app == nil {
		return false
	}
	evidence := recurringview.AgendaSettlementEvidence{
		BillID: "recurring:" + recurring.ID, RecurringID: recurring.ID, DueDate: due,
		Paid: app.OccurrencePaid("recurring:"+recurring.ID, due),
	}
	_, evidence.Matched = app.BillMatchForOccurrence(recurring.ID, due)
	return recurringview.OccurrenceSettled(recurring, due, evidence)
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
	// DP1: overdue occurrences get their own heading rather than sitting under a
	// month label. They are dated in the PAST, so the month break above them says
	// something like "July 2026" beneath a forward-looking "Up next" — which reads,
	// for the moment it takes to spot the OVERDUE pill, as a claim that a past
	// month is upcoming. A heading costs one dim line and removes the contradiction
	// instead of asking the reader to resolve it from a small badge.
	month := ""
	overdueHeaded := false
	for _, it := range shown {
		row := it
		if row.Past {
			if !overdueHeaded {
				overdueHeaded = true
				// Reset the month so the first FORWARD row re-emits its heading; without
				// this an overdue July row and an upcoming July row would share one
				// label across the break, putting them back in the same group.
				month = ""
				rows = append(rows, Div(css.Class("rhy-ag-month rhy-ag-overdue"),
					Attr("role", "presentation"), Attr("data-testid", "rhy-agenda-overdue-head"),
					uistate.T("rhythm.agendaOverdue")))
			}
			rows = append(rows, ui.CreateElement(rhyAgendaRow, rhyAgendaRowProps{Item: row, Base: base}))
			continue
		}
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
	span := prepareAgendaSpan(app, now, disp.AddDate(0, 0, -7), monthEnd.AddDate(0, 0, 7), base)
	items := presentAgendaSpan(app, span, now)
	byDay := map[string][]agendaItem{}
	for _, it := range items {
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
	// WF9: project the account forward across the displayed month, from the same
	// agenda items the grid is drawing — so the balances and the chips can never
	// describe different obligations.
	closing, shortNote := rhyProjectedClosing(app, span, disp, base, now)
	return Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb2),
			Span(css.Class("rhy-sec-note"), Style(map[string]string{"margin": "0"}), monthLabel(disp)), nav),
		rhyCalendarGrid(grid, pr.WeekStartWeekday(), now, byDay, closing, base),
		shortNote,
	)
}

// rhyCalendarGrid draws the month grid with the day's real amounts in-cell.
// Display-only (no hooks), so it is safe inside the day loop.
// rhyCalendarGrid renders the live agenda calendar.
//
// closing carries the projected end-of-day balance per date (WF9). A calendar of
// due dates answers "what is coming"; it does not answer "can I cover it", which
// is the question somebody opens this page with.
func rhyCalendarGrid(grid [][]bills.CalendarDay, weekStart time.Weekday, now time.Time, byDay map[string][]agendaItem, closing map[string]int64, base string) ui.Node {
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
			// The projected closing balance, and a marker when the day ends short.
			// Only for days inside the month: a padding day belongs to a neighbouring
			// month and a balance there would read as this one's.
			var balNode ui.Node = Fragment()
			if v, ok := closing[key]; ok && day.InMonth {
				balCls := "cal-bal"
				if v < 0 {
					balCls, cls = "cal-bal is-short", cls+" short"
				}
				balNode = Span(css.Class(balCls), Attr("data-testid", "cal-bal-"+key),
					fmtMoney(money.New(v, base)))
			}
			cell := []any{ClassStr(cls), Attr("data-testid", "rhy-cal-"+key),
				Span(css.Class("rhy-cal-day"), day.Date.Format("2")), balNode}
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
				amountLabel := rhyAgendaAmountText(it)
				tip := it.Name + " · " + amountLabel
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
					Span(css.Class("rhy-cal-amt"), amountLabel),
				))
			}
			args = append(args, Div(cell...))
		}
	}
	return Div(args...)
}

func rhyAgendaAmountText(item agendaItem) string {
	if !item.AmountAvailable {
		return uistate.T("transactions.calScheduledAmountUnavailable")
	}
	return fmtMoney(item.Amount)
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
		if app == nil || !it.AmountAvailable {
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
	// Negotiate hands the user the HAGGLING SCRIPT — that is the whole feature, and
	// the to-do is only the follow-up that keeps it from being forgotten. Creating
	// a bare "Negotiate Rewards Credit Card" task, as this briefly did, delivers
	// the reminder and withholds the thing being reminded about. So it opens the
	// task composer seeded with the talking points, exactly as the retired bills
	// row did, and the user sees the script before the task exists.
	negotiate := ui.UseEvent(Prevent(func() {
		uistate.SetTaskAddSeed(uistate.TaskAddSeed{
			Title: uistate.T("bills.negotiateTaskTitle", it.Name),
			Notes: subscriptions.ChecklistNotes("", subscriptions.NegotiationTips(it.Name)),
		})
		uistate.SetAddTarget("task")
	}))

	rowCls := "rhy-ag-row"
	var verb ui.Node = Fragment()
	if !it.Income {
		if it.Paid {
			verb = Span(css.Class("pill", tw.ColorClass("text-ok")), Attr("data-testid", "bill-paid"),
				Title(uistate.T("bills.paidBadgeTitle")), uistate.T("bills.paidBadge"))
		} else if !it.AmountAvailable {
			verb = Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "rhy-ag-markpaid-"+it.AccountID),
				Title(uistate.T("transactions.calScheduledAmountUnavailable")), Disabled(true), uistate.T("bills.markPaid"))
		} else {
			verb = Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "rhy-ag-markpaid-"+it.AccountID),
				Title(uistate.T("bills.markPaidTitle")), OnClick(markPaid), uistate.T("bills.markPaid"))
		}
	}

	var fitNode ui.Node = Fragment()
	if it.Fit != nil {
		if !it.Fit.Available {
			fitNode = Span(css.Class("pill bill-fit bill-fit-unavailable"),
				Attr("data-testid", "bill-fit-"+it.AccountID),
				Attr("aria-label", uistate.T("common.notReady")),
				Title(uistate.T("common.notReady")), uistate.T("common.notReady"))
		} else {
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

	amountClass := "rhy-ag-amt"
	if it.AmountAvailable {
		amountClass += " " + recurAmountTone(it.Amount)
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
		Span(ClassStr(amountClass), rhyAgendaAmountText(it)),
		Div(css.Class("rhy-ag-verb"), verb, kebab),
	)
}

// rhyProjectedClosing projects the liquid balance across the displayed month
// (WF9), returning the end-of-day balance per date and a note naming the first
// day it goes short.
//
// Projects from TODAY forward, so a month already past shows nothing: a
// "projected" balance for days that have happened would be a forecast of the
// past, and the ledger already knows what occurred.
func rhyProjectedClosing(app *appstate.App, span recurringview.AgendaSpan, month time.Time,
	base string, now time.Time) (map[string]int64, ui.Node) {

	if app == nil {
		return nil, Fragment()
	}
	monthEnd := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
	if monthEnd.Before(now) {
		return nil, Fragment()
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	liq, err := ledger.LiquidBalance(app.Accounts(), app.Transactions(), rates)
	if err != nil {
		return nil, Fragment()
	}
	pr, ok := recurringview.ProjectAgendaMonth(span, liq.Amount, month, now)
	if !ok {
		return nil, Fragment()
	}
	closing := make(map[string]int64, len(pr.Days))
	for _, d := range pr.Days {
		closing[d.Date.Format("2006-01-02")] = d.ClosingMinor
	}
	var note ui.Node = Fragment()
	if pr.AnyShortfall() {
		// The FIRST short day leads, not the deepest: "when does this break" is
		// actionable in a way "how bad does it get" is not.
		note = P(ClassStr("muted "+tw.ColorClass("text-down")), Attr("data-testid", "rhy-cal-short"),
			uistate.T("bills.calShortfall",
				uistate.LoadPrefs().FormatDate(pr.FirstShortDate),
				fmtMoney(money.New(pr.LowMinor, base)), pr.DaysShort))
	}
	return closing, note
}
