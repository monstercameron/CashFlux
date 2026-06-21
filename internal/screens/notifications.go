//go:build js && wasm

package screens

import (
	"fmt"
	"time"

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
		return Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("nav.notifications")),
			P(css.Class("empty"), uistate.T("notifications.empty")),
		)
	}

	pr := uistate.UsePrefs().Get()
	rows := make([]ui.Node, 0, len(feed))
	for _, it := range feed {
		when := time.Unix(it.At, 0)
		rows = append(rows, Div(css.Class("row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), it.Title),
				If(it.Body != "", Span(css.Class("row-meta"), it.Body)),
			),
			Span(css.Class("row-meta", tw.TextFaint), pr.FormatDate(when)),
		))
	}

	return Section(css.Class("card"),
		Div(css.Class("budget-head"),
			H2(css.Class("card-title"), uistate.T("nav.notifications")),
			Button(css.Class("btn"), Type("button"), OnClick(clearAll), uistate.T("notifications.clearAll")),
		),
		Div(css.Class("rows"), rows),
	)
}
