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
}

// buildAgenda merges the forward bill occurrences (recurring-derived + liability
// statement) with the income paychecks into one date-sorted agenda over the
// standard pay-cycle window.
func buildAgenda(app *appstate.App, now time.Time, base string) []agendaItem {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return buildAgendaRange(app, now, today.AddDate(0, 0, agendaWindowDays), base)
}

// buildAgendaRange is buildAgenda over an explicit horizon — the calendar view
// pages to arbitrary months, so it needs the same merged model out to the end of
// whatever month is displayed.
func buildAgendaRange(app *appstate.App, now, until time.Time, base string) []agendaItem {
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	recByID := map[string]domain.Recurring{}
	for _, r := range app.Recurring() {
		recByID[r.ID] = r
	}

	var items []agendaItem
	for _, b := range bills.OccurrencesWithin(app.Accounts(), app.Recurring(), now, until) {
		if b.DueDate.Before(today) {
			continue // overdue lives in its own strip
		}
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		it := agendaItem{
			Date: b.DueDate, Name: b.Name, AccountID: b.AccountID,
			Amount: money.New(-amt.Amount, base), Fit: billFitFor(b),
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
		items = append(items, it)
	}

	// Income paychecks make the rhythm real.
	for _, r := range app.Recurring() {
		if !r.Active() || r.Amount.IsNegative() {
			continue
		}
		amt, err := rates.Convert(r.Amount, base)
		if err != nil {
			amt = money.New(r.Amount.Amount, base)
		}
		label, hint, cls := postingMode(r)
		d := r.NextDue
		for i := 0; i < 8 && !d.After(until); i++ {
			if !d.Before(today) {
				items = append(items, agendaItem{
					Date: d, Name: r.Label, Amount: money.New(amt.Amount, base),
					ModeLabel: label, ModeHint: hint, ModeCls: cls, Income: true,
				})
			}
			d = r.Cadence.Next(d)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.Before(items[j].Date) })
	return items
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

	view := ui.UseState(uistate.AgendaViewGet())
	setView := func(v string) {
		view.Set(v)
		uistate.AgendaViewSet(v)
	}
	onCompact := ui.UseEvent(Prevent(func() { setView(uistate.AgendaViewCompact) }))
	onCalendar := ui.UseEvent(Prevent(func() { setView(uistate.AgendaViewCalendar) }))
	showAll := ui.UseState(false)
	toggleAll := ui.UseEvent(Prevent(func() { showAll.Set(!showAll.Get()) }))
	calOffset := ui.UseState(0)
	calPrev := ui.UseEvent(Prevent(func() { calOffset.Set(calOffset.Get() - 1) }))
	calNext := ui.UseEvent(Prevent(func() { calOffset.Set(calOffset.Get() + 1) }))
	calToday := ui.UseEvent(Prevent(func() { calOffset.Set(0) }))

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	toggle := Div(css.Class("rhy-view-toggle", tw.InlineFlex, tw.Gap1), Attr("role", "group"), Attr("aria-label", uistate.T("rhythm.viewAria")),
		rhyToggleBtn(uistate.T("rhythm.viewCompact"), "rhy-view-compact", view.Get() == uistate.AgendaViewCompact, onCompact),
		rhyToggleBtn(uistate.T("rhythm.viewCalendar"), "rhy-view-calendar", view.Get() == uistate.AgendaViewCalendar, onCalendar),
	)

	var body ui.Node
	if view.Get() == uistate.AgendaViewCalendar {
		body = rhyAgendaCalendar(app, now, calOffset.Get(), base, calPrev, calNext, calToday)
	} else {
		body = rhyAgendaCompact(buildAgenda(app, now, base), base, showAll.Get(), toggleAll)
	}
	return rhySection("sec-agenda", uistate.T("rhythm.agendaTitle"), uistate.T("rhythm.agendaNote"), toggle, body)
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
	for _, it := range shown {
		row := it
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

// rhyAgendaCalendar renders the month-grid view of the SAME agenda data as the
// compact list — real amounts on the days, not bare dots, with income visually
// distinct from outflow. It keeps the cal-prev/cal-next/cal-today testids.
func rhyAgendaCalendar(app *appstate.App, now time.Time, offset int, base string, calPrev, calNext, calToday any) ui.Node {
	pr := uistate.LoadPrefs()
	disp := dateutil.AddMonths(dateutil.MonthStart(now), offset)
	monthEnd := dateutil.AddMonths(disp, 1)
	// Bucket the merged agenda (bills + income) by calendar day.
	byDay := map[string][]agendaItem{}
	for _, it := range buildAgendaRange(app, now, monthEnd.AddDate(0, 0, 1), base) {
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
				chip := "rhy-cal-amt"
				if it.Income {
					chip += " is-in"
				}
				cell = append(cell, Span(ClassStr(chip), Title(it.Name), fmtMoney(it.Amount)))
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
		fitNode = Span(ClassStr(cls), Attr("data-testid", "bill-fit-"+it.AccountID), label)
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
			fitNode,
		),
		Span(ClassStr("rhy-ag-amt "+recurAmountTone(it.Amount)), fmtMoney(it.Amount)),
		Div(css.Class("rhy-ag-verb"), verb, kebab),
	)
}
