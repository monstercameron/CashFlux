// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// NotificationCenter lists the notifications surfaced by the catch-up engine (bill
// due, budget thresholds, stale balances, digests, …) — the persisted feed (C75).
// Opening it marks everything read; "Clear all" empties it.
func NotificationCenter() ui.Node {
	feedAtom := uistate.UseNotifyFeed()
	feed := feedAtom.Get()

	// Mark all read when the center is open (so the rail badge clears).
	ui.UseEffect(func() func() {
		if uistate.UnreadNotifyCount(feed) == 0 {
			return nil
		}
		next := make([]uistate.FeedItem, len(feed))
		for i, it := range feed {
			it.Read = true
			next[i] = it
		}
		feedAtom.Set(next)
		uistate.PersistNotifyFeed(next)
		return nil
	}, fmt.Sprintf("notif-read:%d", len(feed)))

	clearAll := ui.UseEvent(func() {
		feedAtom.Set(nil)
		uistate.PersistNotifyFeed(nil)
	})

	if len(feed) == 0 {
		return uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.notifications"),
			Body:  P(css.Class("empty"), uistate.T("notifications.empty")),
		})
	}

	pr := uistate.UsePrefs().Get()
	items := make([]ui.Node, 0, len(feed))
	for _, it := range feed {
		when := time.Unix(it.At, 0)
		items = append(items, Div(css.Class("row"), Attr("role", "listitem"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), it.Title),
				If(it.Body != "", Span(css.Class("row-meta"), it.Body)),
			),
			Span(css.Class("row-meta", tw.TextFaint), pr.FormatDate(when)),
		))
	}

	// Build the list body manually (role="list" + role="listitem" semantics) and
	// pass it via Body rather than Rows so we never touch EntityListSection itself
	// (it is off-limits as part of the ui-refactor churn). Append items into an
	// []any so the variadic Div call receives a single flat argument list.
	listArgs := []any{css.Class("rows"), Attr("role", "list")}
	for _, item := range items {
		listArgs = append(listArgs, item)
	}
	listBody := Div(listArgs...)

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Header: Div(css.Class("budget-head"),
			H2(css.Class("card-title"), uistate.T("nav.notifications")),
			Button(css.Class("btn"), Type("button"), OnClick(clearAll),
				Attr("aria-label", uistate.T("notifications.clearAllAria")),
				uistate.T("notifications.clearAll")),
		),
		Body: listBody,
	})
}
