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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	if app == nil {
		return Fragment()
	}

	now := time.Now()
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

	const maxRows = 5
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

	return Button(css.Class("txn-upcoming"), Type("button"),
		Attr("data-testid", "txn-upcoming-strip"),
		Attr("title", uistate.T("transactions.upcomingTitle")),
		OnClick(openRecurring),
		Div(css.Class("txn-upcoming-head"),
			Span(css.Class("txn-upcoming-heading"), uistate.T("transactions.upcomingHead", len(pending))),
			Span(css.Class("t-caption", tw.TextDim), uistate.T("transactions.upcomingNote")),
		),
		Div(css.Class("txn-upcoming-rows"), rows),
	)
}
