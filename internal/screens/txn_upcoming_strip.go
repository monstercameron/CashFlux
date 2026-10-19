// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnUpcomingStrip is the ledger's pending-vs-posted surface (parity scan):
// the rest of THIS month's scheduled charges (bills + recurring) rendered as
// visually distinct ghost rows above the table — dimmed, dashed, badged
// UPCOMING — so "what's about to hit" and "what has posted" never blur. The
// set comes from bills.PendingInWindow, which suppresses any occurrence the
// billmatch engine can already settle against a posted transaction, so an
// early payment never shows twice. These are schedule entries, not
// transactions: they carry no checkbox, count in no totals, and click through
// to /recurring where the schedule is managed.
// txnUpcomingStrip preserves the old widget slot as an empty anchor. The preview
// now belongs to the count line and is rendered by txnUpcomingInline.
func txnUpcomingStrip(struct{}) ui.Node { return Fragment() }

func txnUpcomingInline(struct{}) ui.Node {
	app := appstate.Default
	pr := uistate.UsePrefs().Get()
	filterAtom := uistate.UseTxFilter()
	// Hooks first and unconditionally, before any early return: the framework tracks
	// hooks positionally, so a return above one shifts every later render's slots.
	expanded := ui.UseState(uistate.KVGet(txnUpcomingOpenKV) == "1")
	toggleAction := func() {
		next := !expanded.Get()
		expanded.Set(next)
		v := "0"
		if next {
			v = "1"
		}
		uistate.KVSet(txnUpcomingOpenKV, v)
		uistate.RequestPersist()
	}
	toggle := ui.UseEvent(Prevent(toggleAction))
	open := expanded.Get()
	if app == nil {
		return Fragment()
	}

	now := dateutil.WallClockUTC(time.Now())
	// C560: the strip is "what is about to hit THIS month". When the ledger is paged
	// to a period that does not contain today, that band is answering a question the
	// page is not asking — a row of future charges floating over March 2025 reads as
	// part of that month. Nothing about a past period is upcoming, so the strip steps
	// aside rather than projecting the present onto it.
	if !txnUpcomingScopeIncludes(txnscope.Of(filterAtom.Get().From, filterAtom.Get().To, now), now) {
		return Fragment()
	}
	monthEnd := dateutil.AddMonths(dateutil.MonthStart(now), 1)
	all := bills.UpcomingAll(app.Accounts(), app.Recurring(), now)
	if len(all) == 0 {
		return Fragment()
	}
	resolver := app.PayeeResolver()
	monthStart := dateutil.MonthStart(now)
	var posted []billmatch.Txn
	settled := map[string]bool{}
	for _, t := range app.Transactions() {
		// Only transactions dated from this month's first day through today can
		// settle this month's preview. Later-dated imports are still planned work.
		if !txnUpcomingSettlementDate(t.Date, monthStart, now) {
			continue
		}
		// An explicit bill link (or a transfer INTO the liability) settles that
		// account's bill exactly — stronger than the fuzzy matcher below.
		if t.BillAccountID != "" {
			settled[t.BillAccountID] = true
		}
		if t.IsTransfer() {
			if t.Amount.Amount > 0 {
				settled[t.AccountID] = true // the receiving leg of a payment transfer
			}
			continue
		}
		posted = append(posted, billmatch.Txn{
			ID:          t.ID,
			Date:        t.Date,
			Payee:       strings.TrimSpace(resolver.Resolve(firstNonEmpty(t.Payee, t.Desc))),
			CategoryID:  t.CategoryID,
			AmountMinor: t.Amount.Amount,
			Currency:    t.Amount.Currency,
		})
	}
	pending := bills.PendingInWindow(all, posted, settled, now, monthEnd)
	if len(pending) == 0 {
		return Fragment()
	}

	// Compressed strip (2026-07-19 two-row-toolbar pass): cap the ghost rows low so the
	// pending preview stays a thin band above the ledger — the overflow rolls into the
	// "+N more" line that clicks through to /recurring for the full schedule.
	const maxRows = 3
	shown := pending
	extra := 0
	if len(shown) > maxRows {
		extra = len(shown) - maxRows
		shown = shown[:maxRows]
	}
	rows := make([]ui.Node, 0, len(shown)+1)
	for _, b := range shown {
		rows = append(rows, Div(css.Class("txn-upcoming-row"),
			Span(css.Class("txn-upcoming-badge"), uistate.T("transactions.upcomingBadge")),
			Span(css.Class("txn-upcoming-date", tw.TextDim), pr.FormatDate(b.DueDate)),
			Span(css.Class("txn-upcoming-name", tw.Truncate), b.Name),
			Span(css.Class("fig txn-upcoming-amt", tw.FontDisplay, tw.MlAuto), fmtMoney(b.Amount.Neg())),
		))
	}
	if extra > 0 {
		rows = append(rows, Div(css.Class("txn-upcoming-row", tw.TextDim), Span(uistate.T("transactions.upcomingMore", extra))))
	}

	// C582: collapsed by default. This band is a PLANNING tool sitting on top of a
	// page whose job is scanning and correcting posted transactions, and at four
	// lines it pushed the first ledger row past the halfway mark of a 900px viewport
	// — the ledger arrived below the fold on the page named after it. What a user
	// scanning transactions needs from it is one fact ("seven things are still to
	// hit, worth $458"); the itemised preview is what they need once that fact
	// interests them. So it is one line until asked, the choice is remembered, and
	// expanding it is a single visible click.
	//
	// The whole band was also one big button that navigated away — a 130px click
	// target over a summary, with no way to look without leaving. Navigation is now
	// its own named link at the end of the expanded rows.
	var total money.Money
	for _, b := range pending {
		if total.Currency == "" {
			total = b.Amount
		} else if sum, err := total.Add(b.Amount); err == nil {
			total = sum
		}
	}
	// State the date alongside the due amount so the collapsed band carries a
	// useful date rather than requiring the reader to open it.
	summary := txnUpcomingDueSummary(pending, fmtMoney(total), pr.FormatDate)

	content := Div(css.Class("txn-upcoming-rows txn-count-upcoming-rows"), rows,
		Span(css.Class("t-caption", tw.TextDim), uistate.T("transactions.upcomingNote")),
		txnUpcomingScheduleLink())
	return txnUpcomingDisclosure(open, summary, toggle, content, toggleAction)
}

type txnUpcomingDisclosureProps struct {
	open         bool
	summary      string
	toggle       ui.Handler
	content      ui.Node
	toggleAction func()
}

func txnUpcomingDisclosure(open bool, summary string, toggle ui.Handler, content ui.Node, toggleAction ...func()) ui.Node {
	var action func()
	if len(toggleAction) > 0 {
		action = toggleAction[0]
	}
	return ui.CreateElement(txnUpcomingDisclosureComponent, txnUpcomingDisclosureProps{
		open: open, summary: summary, toggle: toggle, content: content, toggleAction: action,
	})
}

func txnUpcomingDisclosureComponent(props txnUpcomingDisclosureProps) ui.Node {
	ui.UseEffect(func() func() {
		if props.open {
			focusUpcomingElement("txn-upcoming-popover")
		}
		return nil
	}, props.open)
	onPopoverKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		if e.GetKey() != "Escape" {
			return
		}
		e.PreventDefault()
		if props.toggleAction != nil {
			props.toggleAction()
		}
		focusUpcomingElement("txn-upcoming-toggle")
	})
	return Div(ClassStr("txn-count-upcoming"+collapsedClass(props.open)),
		Attr("data-testid", "txn-upcoming-strip"),
		Button(css.Class("btn-link txn-upcoming-disc"), Type("button"),
			Attr("data-testid", "txn-upcoming-toggle"), Attr("aria-controls", "txn-upcoming-popover"),
			Attr("aria-expanded", ariaBool(props.open)), Attr("aria-label", discLabel(props.open, props.summary)),
			OnClick(props.toggle), props.summary),
		If(props.open, Div(css.Class("txn-upcoming-popover"), Attr("id", "txn-upcoming-popover"), Attr("data-testid", "txn-upcoming-popover"),
			Attr("data-uxt5-disclosure", "popover"), Attr("role", "dialog"), Attr("aria-modal", "false"), Attr("aria-label", props.summary),
			Attr("tabindex", "-1"), OnKeyDown(onPopoverKey), props.content)),
	)
}

func txnUpcomingScopeIncludes(scope txnscope.Scope, now time.Time) bool {
	if scope.Kind == txnscope.All {
		return true
	}
	today := dateutil.CalendarDayUTC(now)
	if !scope.From.IsZero() && today.Before(dateutil.CalendarDayUTC(scope.From)) {
		return false
	}
	if !scope.To.IsZero() && today.After(dateutil.CalendarDayUTC(scope.To)) {
		return false
	}
	return true
}

func focusUpcomingElement(id string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() || doc.Get("getElementById").Type() != js.TypeFunction {
		return
	}
	element := doc.Call("getElementById", id)
	if element.Truthy() && element.Get("focus").Type() == js.TypeFunction {
		element.Call("focus", map[string]any{"preventScroll": true})
	}
}

func txnUpcomingDueSummary(items []bills.Bill, total string, formatDate func(time.Time) string) string {
	if len(items) == 0 {
		return ""
	}
	earliest := items[0].DueDate
	for _, item := range items[1:] {
		if item.DueDate.Before(earliest) {
			earliest = item.DueDate
		}
	}
	date := formatDate(earliest)
	total = strings.TrimSuffix(total, ".00")
	if len(items) == 1 {
		return uistate.T("tv7e1.upcomingOne", total, date)
	}
	return uistate.T("tv7e1.upcomingMany", len(items), date, total)
}

func txnUpcomingSummary(count int, total string) string {
	key := "r2a.upcomingMany"
	if count == 1 {
		key = "r2a.upcomingOne"
	}
	return uistate.T(key, count, total)
}

// txnUpcomingScheduleLink renders the pending strip's destination as a real route link.
func txnUpcomingScheduleLink() ui.Node {
	return A(css.Class("btn-link txn-upcoming-link"),
		Attr("data-testid", "txn-upcoming-schedule"),
		Attr("title", uistate.T("transactions.upcomingTitle")),
		Attr("href", uistate.RoutePath("/recurring")),
		uistate.T("transactions.upcomingSeeSchedule"))
}

// txnUpcomingOpenKV remembers whether the pending band is expanded, so the choice
// survives navigation and reloads: a power user who wants the preview should not
// have to re-open it on every visit, and a user who collapsed it should not find
// it back on top of their ledger.
const txnUpcomingOpenKV = "txn.upcomingOpen"

func collapsedClass(open bool) string {
	if open {
		return " is-open"
	}
	return " is-collapsed"
}

func discText(open bool) string {
	if open {
		return uistate.T("transactions.upcomingHide")
	}
	return uistate.T("transactions.upcomingShow")
}

func discLabel(open bool, summary string) string {
	if open {
		return uistate.T("transactions.upcomingHideAria", summary)
	}
	return uistate.T("transactions.upcomingShowAria", summary)
}
