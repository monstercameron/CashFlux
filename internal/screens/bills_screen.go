// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// billsHorizonDays is the default look-ahead window. Bills beyond this are hidden
// unless the user enables "Show all" (G11 follow-up).
const billsHorizonDays = 90

// BillsPanelProps holds configuration for BillsPanel. Currently the panel reads
// all state from appstate.Default; the struct exists so call sites pass
// BillsPanelProps{} and future props can be added without altering callers.
type BillsPanelProps struct{}

// BillsPanel is a registered component that owns all bills-view logic and hooks.
// It is mounted on the /bills route (via the Bills() thin shell) and embedded in
// the tabbed /recurring hub (FEATURE_MAP §5.3/§5.7b). Each mount gets an isolated
// hook scope so tab-switching does not share calendar or mark-paid state.
func BillsPanel(p BillsPanelProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}

	// === Hooks — all unconditional (GWC rule) ===
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

	notice := uistate.UseNotice()
	rev := uistate.UseDataRevision()

	// Hovering a bill row highlights its due date on the calendar — a delegated
	// native listener that toggles a class directly (no per-hover Go re-render,
	// same pattern as the back-to-top button). Rows carry data-due, calendar cells
	// carry data-date; the listener lives on document so it survives re-renders,
	// and the cleanup runs on unmount.
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil
		}
		setHl := func(due string, on bool) {
			if due == "" {
				return
			}
			cell := doc.Call("querySelector", `.cal-cell[data-date="`+due+`"]`)
			if !cell.Truthy() {
				return
			}
			if on {
				cell.Get("classList").Call("add", "cal-hl")
			} else {
				cell.Get("classList").Call("remove", "cal-hl")
			}
		}
		dueOf := func(args []js.Value) string {
			if len(args) == 0 {
				return ""
			}
			t := args[0].Get("target")
			if !t.Truthy() || t.Get("closest").IsUndefined() {
				return ""
			}
			row := t.Call("closest", "[data-due]")
			if !row.Truthy() {
				return ""
			}
			return row.Call("getAttribute", "data-due").String()
		}
		over := js.FuncOf(func(_ js.Value, args []js.Value) any { setHl(dueOf(args), true); return nil })
		out := js.FuncOf(func(_ js.Value, args []js.Value) any { setHl(dueOf(args), false); return nil })
		doc.Call("addEventListener", "mouseover", over)
		doc.Call("addEventListener", "mouseout", out)
		return func() {
			doc.Call("removeEventListener", "mouseover", over)
			doc.Call("removeEventListener", "mouseout", out)
			over.Release()
			out.Release()
		}
	}, true)

	// === Rendering (nil-guarded) ===
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

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

	// Smart pay schedule (billsched): computed once, shared with the modal so the
	// rows/calendar and the plan preview always agree. The engine runs regardless
	// (its figures feed the bills_* variables); the config decides whether the
	// views use it. The plan window is the standard 60 days (this month + next),
	// extended to cover whatever month the calendar is paged forward to — so a
	// bill shown in a future month always has its pay-on mapping.
	planUntil := now.AddDate(0, 0, engineenv.BillsSmartHorizonDays)
	if off := calMonthOffset.Get(); off > 0 {
		// Last day of the displayed month.
		if mEnd := dateutil.AddMonths(dateutil.MonthStart(now), off+1).AddDate(0, 0, -1); mEnd.After(planUntil) {
			planUntil = mEnd
		}
	}
	smartPlan := computeBillsSmart(app, now, planUntil)
	smartCfg := smartPlan.Cfg
	hasAnchor := smartPlan.HasAnchor
	viewSmart := smartCfg.Enabled && smartCfg.ViewSmart && hasAnchor
	payOnFor := func(b bills.Bill) time.Time {
		if p, ok := smartPlan.Res.PayOnByID[billItemID(b)]; ok {
			return p
		}
		return b.DueDate
	}
	// Every occurrence in the plan window, shared by the pay-ahead cadence check,
	// the extra plan rows, and the calendar.
	occAll := bills.OccurrencesWithin(app.Accounts(), app.Recurring(), now, planUntil)
	// prevOccDue maps an occurrence ID to the SAME bill's immediately-prior
	// occurrence due date in the window (occurrences arrive date-sorted).
	prevOccDue := map[string]time.Time{}
	{
		last := map[string]time.Time{}
		for _, b := range occAll {
			k := b.AccountID + "|" + b.Name
			if p, ok := last[k]; ok {
				prevOccDue[billItemID(b)] = p
			}
			last[k] = b.DueDate
		}
	}
	// aheadTagFor: flag only the payments the plan fronts a pay CYCLE early —
	// and only until the cadence is established (this occurrence unpaid AND the
	// prior occurrence not already paid). Per Cam: "flagged until you did it
	// once, and recurring bills acknowledge that you already paid ahead."
	aheadTagFor := func(b bills.Bill, isPaid bool) bool {
		if !viewSmart || isPaid || !smartPlan.Res.AheadByID[billItemID(b)] {
			return false
		}
		if prev, ok := prevOccDue[billItemID(b)]; ok && app.OccurrencePaid(b.AccountID, prev) {
			return false
		}
		return true
	}

	// remind creates a to-do dated to the bill's due date, so a "pay this" task
	// surfaces in time (B22, via the existing to-do system).
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

	// Local bill-negotiation helper: opens the add-task modal pre-filled with a
	// talking-points checklist for haggling this bill down — the on-device stand-in
	// for a bill-negotiation service (which needs a paid integration). No money moves;
	// it just hands the user the script and tracks the follow-up.
	negotiate := func(b bills.Bill) {
		uistate.SetTaskAddSeed(uistate.TaskAddSeed{
			Title: uistate.T("bills.negotiateTaskTitle", b.Name),
			Notes: subscriptions.ChecklistNotes("", subscriptions.NegotiationTips(b.Name)),
		})
		uistate.SetAddTarget("task")
	}

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
		// C154: persist the paid mark so status survives reloads.
		if err := app.MarkOccurrencePaid(b.AccountID, b.DueDate); err != nil {
			app.Log().Error("mark occurrence paid", "billID", b.AccountID, "err", err)
		}
		notice.Set(notice.Get().With(uistate.T("bills.paidLogged", b.Name), false))
		rev.Set(rev.Get() + 1)
	}

	unmarkPaid := func(b bills.Bill) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.UnmarkOccurrencePaid(b.AccountID, b.DueDate); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		notice.Set(notice.Get().With(uistate.T("bills.unpaidLogged", b.Name), false))
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
		isPaid := app.OccurrencePaid(b.AccountID, b.DueDate)
		payOn := payOnFor(b)
		billRows = append(billRows, billRowData{
			Bill: b, Shown: amt, DueLabel: pr.FormatDate(b.DueDate), IsPaid: isPaid,
			PayOn: payOn, PayOnLabel: pr.FormatDate(payOn),
			PlanActive: viewSmart && !sameDay(payOn, b.DueDate),
			AheadTag:   aheadTagFor(b, isPaid),
		})
	}

	// In the pay-on plan view, LATER occurrences pulled onto an earlier payday
	// become their own rows, tagged pay-ahead — the "double payment" month
	// (this month's bill plus next month's paid early) is visible in the list,
	// not implied. The hero's Total-due stays the raw window (those amounts are
	// due next month; the plan only changes when they get paid).
	if viewSmart {
		first := make(map[string]bool, len(allUpcoming))
		for _, b := range allUpcoming {
			first[billItemID(b)] = true
		}
		for _, b := range occAll {
			if first[billItemID(b)] {
				continue // each bill's next occurrence is already listed above
			}
			payOn := payOnFor(b)
			if sameDay(payOn, b.DueDate) {
				continue // unmoved future occurrence — next month's business
			}
			amt, err := rates.Convert(b.Amount, base)
			if err != nil {
				amt = money.New(b.Amount.Amount, base)
			}
			isPaid := app.OccurrencePaid(b.AccountID, b.DueDate)
			billRows = append(billRows, billRowData{
				Bill: b, Shown: amt, DueLabel: pr.FormatDate(b.DueDate),
				IsPaid: isPaid,
				PayOn:  payOn, PayOnLabel: pr.FormatDate(payOn),
				PlanActive: true,
				AheadTag:   aheadTagFor(b, isPaid),
			})
		}
		// The plan view lists by when you PAY (the actionable order), not by
		// the raw deadline.
		sort.SliceStable(billRows, func(i, j int) bool { return billRows[i].PayOn.Before(billRows[j].PayOn) })
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
				Data: r, OnRemind: remind, OnMarkPaid: markPaid, OnUnmarkPaid: unmarkPaid, OnNegotiate: negotiate,
				SmartSettings: billSmartSettings,
				SmartByEntity: billByEntity,
			})
		},
	)

	var body ui.Node
	if len(billRows) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("bills.empty"), CTALabel: uistate.T("bills.addFirst"), Href: "/accounts"})
	} else {
		body = Div(css.Class("rows rec-cardrows bills-scroll"), Attr("data-testid", "bills-scroll"), rows)
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

	// The bills tab shares the recurring surface chrome: a bento host with a hero
	// tile (Total due in the display serif + the annual/count/next-due chips) over a
	// list+calendar tile (two-column at ≥1024 px via .bills-layout).
	hero := Div(css.Class("rec-hero"),
		Div(css.Class("rec-hero-main"),
			Div(css.Class("rec-hero-label "+tw.Fold(tw.TextDim, tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
				uistate.T("bills.totalDue"),
				smartTooltipFor(billSmartSettings, "bills-due", uistate.T("bills.totalDue"), uistate.T("smart.tipBillsDue")),
			),
			Div(ClassStr("rec-hero-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass("text-down")), fmtMoney(money.New(total, base))),
		),
		Div(css.Class("debt-chips"),
			recurStatChip(uistate.T("bills.annualCost"), fmtMoney(money.New(annual, base)), ""),
			recurStatChip(uistate.T("bills.count"), fmt.Sprintf("%d", len(upcoming)), ""),
			recurStatChip(uistate.T("bills.nextDue"), nextDue, ""),
		),
	)

	listSection := recurSection("sec-bills", uistate.T("nav.bills"), smartSectionAction(billSmartSettings),
		Fragment(
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
		))

	calendarSection := Fragment()
	if len(allUpcoming) > 0 {
		dispMonth := dateutil.AddMonths(dateutil.MonthStart(now), calMonthOffset.Get())
		nav := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap1),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-prev"),
				Attr("aria-label", uistate.T("bills.calPrev")), Title(uistate.T("bills.calPrev")), OnClick(calPrev), "◀"),
			If(calMonthOffset.Get() != 0, Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-today"),
				OnClick(calToday), uistate.T("bills.calThisMonth"))),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "cal-next"),
				Attr("aria-label", uistate.T("bills.calNext")), Title(uistate.T("bills.calNext")), OnClick(calNext), "▶"),
		)
		// The grid shows the ACTIVE schedule (smart pay-on dates when that view is
		// on); the inactive schedule's dates render as hollow ghost markers so the
		// plan and the deadline are always both visible. It renders OCCURRENCES
		// (every repeat in the window, out to whatever month is displayed) — with
		// only each bill's next occurrence, a paged-forward month would show
		// nothing and pay-ahead's cross-month move would be invisible.
		occ := occAll
		calBills := occ
		ghost := map[string]int{}
		// ahead marks the pay-on dates carrying MOVED payments, so those dots get
		// a distinct treatment — a plan whose moves all pull NEXT month's bills
		// onto this month's paydays otherwise reads as "nothing changed" (the new
		// dot looks like any ordinary due date, and the vacated dates are in the
		// other month's grid).
		ahead := map[string]int{}
		if viewSmart {
			calBills = make([]bills.Bill, len(occ))
			for i, b := range occ {
				moved := b
				moved.DueDate = payOnFor(b)
				calBills[i] = moved
				if !sameDay(moved.DueDate, b.DueDate) {
					ghost[b.DueDate.Format("2006-01-02")]++
					ahead[moved.DueDate.Format("2006-01-02")]++
				}
			}
		} else if smartCfg.Enabled && hasAnchor {
			for _, b := range occ {
				if p := payOnFor(b); !sameDay(p, b.DueDate) {
					ghost[p.Format("2006-01-02")]++
				}
			}
		}
		var legend ui.Node = Fragment()
		if smartCfg.Enabled && hasAnchor {
			key := "bills.calLegendRaw"
			if viewSmart {
				key = "bills.calLegendSmart"
			}
			legend = P(css.Class("muted bills-cal-legend"), Attr("data-testid", "bills-cal-legend"), uistate.T(key))
		}
		calendarSection = recurSection("sec-bills-calendar", uistate.T("bills.calendar", monthLabel(dispMonth)), nav,
			Fragment(
				billsCalendar(bills.MonthCalendar(calBills, dispMonth.Year(), dispMonth.Month(), pr.WeekStartWeekday()), pr.WeekStartWeekday(), now, ghost, ahead),
				legend,
			))
	}

	return Div(css.Class("bento bento-recurring"),
		If(len(upcoming) > 0, recurTile("bills-hero", hero)),
		ui.CreateElement(billsSmartSummaryTile, billsSmartSummaryProps{Plan: smartPlan}),
		recurTile("bills-main", Div(css.Class("bills-layout"),
			listSection,
			Div(css.Class("bills-cal-sticky"), calendarSection),
		)),
	)
}

// Bills is the /bills route — a thin shell that renders BillsPanel. The shell
// provides the heading and subtitle from the route registry (nav.bills /
// screen.billsSub); BillsPanel owns all content, hooks, and logic
// (FEATURE_MAP §5.3/§5.7b).
func Bills() ui.Node {
	return ui.CreateElement(BillsPanel, BillsPanelProps{})
}

// monthLabel renders a month/year heading like "June 2026".
func monthLabel(t time.Time) string { return t.Format("January 2006") }

// billsCalendar renders the month grid: weekday headers plus a cell per day,
// dimming out-of-month days, outlining today, and dotting days with bills due.
// ghost marks the inactive schedule's dates (hollow markers); ahead marks pay-on
// dates carrying payments the smart plan MOVED there (accent treatment) so a
// pulled-forward payment is distinguishable from an ordinary due date.
func billsCalendar(grid [][]bills.CalendarDay, weekStart time.Weekday, now time.Time, ghost, ahead map[string]int) ui.Node {
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
				if n := ahead[day.Date.Format("2006-01-02")]; n > 0 {
					// This day carries payments the plan moved here — accent it and
					// say so, or the plan's work is indistinguishable from a due date.
					dotCls += " cal-dot--payahead"
					names = uistate.T("bills.aheadTitle", n) + " — " + names
				}
				if len(day.Bills) > 1 {
					// Render the count inside the dot so a busy day reads at a glance.
					dot = Span(ClassStr(dotCls+" cal-dot--count"), Attr("title", names), Attr("aria-label", names), strconv.Itoa(len(day.Bills)))
				} else {
					dot = Span(ClassStr(dotCls), Attr("title", names), Attr("aria-label", names))
				}
			}
			// Ghost marker: the inactive schedule (raw deadline vs smart plan) has a
			// bill on this day — hollow so it reads as "the other view". Rendered on
			// out-of-month cells too: a cross-month pay-ahead's vacated due date is
			// usually in NEXT month's leading/trailing cells, and hiding it there is
			// exactly what made the plan look like it did nothing.
			var ghostDot ui.Node = Fragment()
			if n := ghost[day.Date.Format("2006-01-02")]; n > 0 {
				ghostDot = Span(css.Class("cal-dot cal-dot--ghost"), Attr("title", uistate.T("bills.ghostTitle", n)), Attr("aria-label", uistate.T("bills.ghostTitle", n)))
			}
			args = append(args, Div(ClassStr(cls), Attr("data-date", day.Date.Format("2006-01-02")),
				Span(css.Class("cal-day"), strconv.Itoa(day.Date.Day())),
				dot,
				ghostDot,
			))
		}
	}
	return Div(args...)
}

// billRowData is one bill plus its display-ready amount and dates.
type billRowData struct {
	Bill       bills.Bill
	Shown      money.Money // amount converted to the base currency
	DueLabel   string      // pre-formatted raw due date
	IsPaid     bool        // C154: whether this occurrence has been marked paid
	PayOn      time.Time   // the smart schedule's pay-on date (= due when unmoved)
	PayOnLabel string      // pre-formatted pay-on date
	PlanActive bool        // the smart view is on AND this bill's pay-on differs from its due
	// AheadTag: show the "Pay ahead" flag — the plan fronts this payment a pay
	// cycle early AND the user hasn't established the cadence yet (per Cam: the
	// flag stays "until you did it once"; once this or the prior occurrence is
	// marked paid, the revised cadence is just the schedule).
	AheadTag bool
}

// sameDay reports whether two times fall on the same calendar date.
func sameDay(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }

type billRowProps struct {
	Data         billRowData
	OnRemind     func(b bills.Bill, shown money.Money, dueLabel string)
	OnMarkPaid   func(b bills.Bill)
	OnUnmarkPaid func(b bills.Bill) // C154: removes the paid mark for this occurrence
	OnNegotiate  func(b bills.Bill) // opens an add-task pre-filled with negotiation tips
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
	// C154: unmark-paid hook must always be registered (stable hook count per
	// component; never inside a conditional).
	unmarkPaid := ui.UseEvent(Prevent(func() {
		if props.OnUnmarkPaid != nil {
			props.OnUnmarkPaid(d.Bill)
		}
	}))
	negotiate := ui.UseEvent(Prevent(func() {
		if props.OnNegotiate != nil {
			props.OnNegotiate(d.Bill)
		}
	}))
	meta := d.DueLabel + " · " + daysUntilLabel(d.Bill.DaysUntil)
	activeDate := d.Bill.DueDate
	if d.PlanActive {
		// Smart view: lead with the plan's pay-on date; the raw due date stays
		// visible as the deadline.
		meta = uistate.T("bills.payOnMeta", d.PayOnLabel, d.DueLabel)
		activeDate = d.PayOn
	}
	// Urgency tone so an imminent bill stands out at a glance (C57): danger when
	// due today/past, warn within three days. The "due today / in N days" wording
	// carries the meaning too, so it's colour + text (B15).
	metaCls := "row-meta"
	if t := billUrgencyTone(d.Bill.DaysUntil); t != "" {
		metaCls += " " + t
	}
	// C154: when the occurrence is marked paid, hide the urgency tone and show a
	// "Paid" chip so the user can see at a glance this bill is settled. The
	// "Unmark paid" button lets them undo in one click.
	if d.IsPaid {
		metaCls = "row-meta"
	}
	// data-plan-move lets tooling (and the e2e) count rows the plan re-dated,
	// independent of the pay-ahead flag (which only marks cycle-ahead moves).
	rowArgs := []any{css.Class("row"), Attr("data-due", activeDate.Format("2006-01-02"))}
	if d.PlanActive {
		rowArgs = append(rowArgs, Attr("data-plan-move", "1"))
	}
	return Div(append(rowArgs,
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), d.Bill.Name,
				smartBadgeFor(props.SmartSettings, props.SmartByEntity, d.Bill.AccountID),
			),
			Span(ClassStr(metaCls), meta),
			// C154: surface autopay so the user knows this bill is charged automatically
			// (no manual payment needed — just keep funds available).
			If(d.Bill.Autopay, Span(css.Class("pill", tw.TextDim), Attr("data-testid", "bill-autopay"), Attr("title", uistate.T("recurring.autopayHint")), uistate.T("recurring.autopayBadge"))),
			// Smart-schedule tag: this payment jumps a pay CYCLE ahead (fronted
			// money, until the cadence is established) — consolidation onto the due
			// date's own payday carries no flag, just the pay-on meta.
			If(d.AheadTag, Span(css.Class("rec-tag"), Attr("data-testid", "bill-payahead"), Attr("title", uistate.T("bills.payAheadHint")), uistate.T("bills.smartPayAhead"))),
			// C154: paid chip — visible when this occurrence is marked paid.
			If(d.IsPaid, Span(css.Class("pill", tw.ColorClass("text-ok")), Attr("data-testid", "bill-paid"), Attr("title", uistate.T("bills.paidBadgeTitle")), uistate.T("bills.paidBadge"))),
		),
		Span(css.Class("budget-amount"), fmtMoney(d.Shown)),
		// bill-sub-actions: fixed trailing group so action buttons don't crowd the
		// name/amount area, mirroring the .sub-actions pattern from G10 (G11 follow-up).
		Div(css.Class("bill-sub-actions"),
			// C154: show "Unmark paid" when already paid, "Mark paid" otherwise.
			IfElse(d.IsPaid,
				Button(css.Class("btn btn-sm"), Type("button"), Title(uistate.T("bills.unmarkPaidTitle")), Attr("data-testid", "bill-unmark-paid"), OnClick(unmarkPaid), uistate.T("bills.unmarkPaid")),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"), Title(uistate.T("bills.markPaidTitle")), OnClick(markPaid), uistate.T("bills.markPaid")),
			),
			Button(css.Class("btn btn-sm"), Type("button"), Title(uistate.T("bills.remindTitle")), OnClick(remind), uistate.T("bills.remind")),
			// Local bill-negotiation helper: seeds a to-do with talking points + a
			// savings prompt (no external service). Shown for non-autopay recurring bills
			// worth haggling (internet, insurance, phone) — small everyday bills aren't.
			If(props.OnNegotiate != nil, Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "bill-negotiate-"+d.Bill.AccountID),
				Title(uistate.T("bills.negotiateTitle")), OnClick(negotiate), uistate.T("bills.negotiate"))),
		),
	)...)
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
