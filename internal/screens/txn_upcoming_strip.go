// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
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
	"github.com/monstercameron/GoWebComponents/v5/router"
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
func txnUpcomingStrip(struct{}) ui.Node {
	app := appstate.Default
	nav := router.UseNavigate()
	openRecurring := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/recurring")) })
	pr := uistate.UsePrefs().Get()
	filterAtom := uistate.UseTxFilter()
	// Hooks first and unconditionally, before any early return: the framework tracks
	// hooks positionally, so a return above one shifts every later render's slots.
	expanded := ui.UseState(uistate.KVGet(txnUpcomingOpenKV) == "1")
	toggle := ui.UseEvent(Prevent(func() {
		next := !expanded.Get()
		expanded.Set(next)
		v := "0"
		if next {
			v = "1"
		}
		uistate.KVSet(txnUpcomingOpenKV, v)
		uistate.RequestPersist()
	}))
	open := expanded.Get()
	if app == nil {
		return Fragment()
	}

	now := time.Now()
	// C560: the strip is "what is about to hit THIS month". When the ledger is paged
	// to a period that does not contain today, that band is answering a question the
	// page is not asking — a row of future charges floating over March 2025 reads as
	// part of that month. Nothing about a past period is upcoming, so the strip steps
	// aside rather than projecting the present onto it.
	if s := txnscope.Of(filterAtom.Get().From, filterAtom.Get().To, now); s.Kind != txnscope.All {
		if (!s.From.IsZero() && now.Before(s.From)) ||
			(!s.To.IsZero() && now.After(s.To.AddDate(0, 0, 1))) {
			return Fragment()
		}
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
		if t.Date.Before(monthStart) {
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
	// The magnitude, not the accounting negative: "($1,767.00) still to come" reads
	// as a credit. The itemised rows below keep the parenthesised outflow form,
	// where a column of figures is being compared.
	summary := uistate.T("transactions.upcomingSummary", len(pending), fmtMoney(total))

	return Div(ClassStr("txn-upcoming"+collapsedClass(open)),
		Attr("data-testid", "txn-upcoming-strip"),
		Div(css.Class("txn-upcoming-head"),
			Span(css.Class("txn-upcoming-badge"), uistate.T("transactions.upcomingBadge")),
			Span(css.Class("txn-upcoming-heading"), summary),
			If(open, Span(css.Class("t-caption", tw.TextDim), uistate.T("transactions.upcomingNote"))),
			Button(css.Class("btn-link txn-upcoming-disc"), Type("button"),
				Attr("data-testid", "txn-upcoming-toggle"),
				Attr("aria-expanded", ariaBool(open)),
				Attr("aria-label", discLabel(open, summary)),
				OnClick(toggle), discText(open)),
		),
		If(open, Div(css.Class("txn-upcoming-rows"), rows,
			Button(css.Class("btn-link txn-upcoming-link"), Type("button"),
				Attr("data-testid", "txn-upcoming-schedule"),
				Attr("title", uistate.T("transactions.upcomingTitle")),
				OnClick(openRecurring), uistate.T("transactions.upcomingSeeSchedule")))),
	)
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
