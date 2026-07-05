// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type notifProps struct{ App *appstate.App }

// notifSeverityCounts tallies visible items by severity tier.
func notifSeverityCounts(items []uistate.FeedItem) (crit, warn, info int) {
	for _, it := range items {
		switch it.Severity {
		case "critical":
			crit++
		case "warning":
			warn++
		default:
			info++
		}
	}
	return
}

// --- notif-summary ---------------------------------------------------------------

// notifSummaryWidget is the headline tile: a hero alert count + a severity breakdown +
// the "N new since your last visit" catch-up. It also owns the mark-all-read-on-open
// effect (clears the rail badge) and stamps last-seen for the next visit. Renders
// nothing when the feed is empty (the list tile owns the empty state).
func notifSummaryWidget(props notifProps) ui.Node {
	_ = props
	feedAtom := uistate.UseNotifyFeed()
	feed := feedAtom.Get()
	now := time.Now().Unix()
	visible := uistate.VisibleFeed(feed, now)

	// Session-stable baseline: captured once on first mount so the catch-up count doesn't
	// vanish when the open-effect stamps last-seen = now.
	seen := ui.UseState(loadLastSeen())
	newCount := len(uistate.NewSinceLastSeen(visible, seen.Get()))
	unread := uistate.UnreadNotifyCount(visible)
	crit, warn, info := notifSeverityCounts(visible)

	// Mark everything read on open + record last-seen for the next visit. Read the LIVE
	// feed inside the effect (not the render-time closure) — the effect fires a tick after
	// mount, and using a stale snapshot would clobber a snooze/dismiss the user managed to
	// click in that window (their action would be overwritten by the old feed).
	ui.UseEffect(func() func() {
		saveLastSeen(now)
		// Mark read off the LIVE persisted feed (not the render snapshot) so this can't
		// overwrite a snooze/dismiss the user clicked before the effect fired.
		uistate.MarkAllNotifyRead()
		return nil
	}, "notif-open-once")

	if len(visible) == 0 {
		return Fragment()
	}

	subLabel := uistate.T("notifications.allRead")
	if unread > 0 {
		subLabel = uistate.T("notifications.unreadCount", unread)
	}

	sevChip := func(cls, label string, n int) ui.Node {
		if n == 0 {
			return Fragment()
		}
		return Span(ClassStr("notif-sev-chip "+cls),
			Span(css.Class("notif-sev-dot")),
			Span(css.Class("notif-sev-n"), fmt.Sprintf("%d", n)),
			Span(css.Class("notif-sev-name"), label))
	}

	var catchUp ui.Node = Fragment()
	if newCount > 0 && seen.Get() > 0 {
		label := uistate.T("notifications.sinceLastVisitOne")
		if newCount > 1 {
			label = uistate.T("notifications.sinceLastVisit", newCount)
		}
		catchUp = Div(css.Class("notif-catchup"), Attr("role", "status"), Attr("aria-live", "polite"),
			Span(css.Class("notif-catchup-dot")),
			Span(css.Class("notif-catchup-text"), label))
	}

	body := Div(css.Class("notif-summary"),
		Div(css.Class("notif-summary-lead"),
			Span(css.Class("notif-summary-count"), fmt.Sprintf("%d", len(visible))),
			Span(css.Class("notif-summary-label"),
				Span(css.Class("notif-summary-word"), uistate.T("notifications.alertsWord")),
				Span(css.Class("notif-summary-sub"), subLabel)),
		),
		Div(css.Class("notif-summary-sevs"),
			sevChip("sev-critical", notifySeverityLabel("critical"), crit),
			sevChip("sev-warning", notifySeverityLabel("warning"), warn),
			sevChip("sev-info", notifySeverityLabel("info"), info),
		),
		catchUp,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "notif-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- notif-toolbar ---------------------------------------------------------------

// notifToolbarWidget is the shared filter strip: a severity filter on the left and the
// destructive "Clear all" on the right. (Opening the center already marks everything
// read, so there's no redundant "mark all read" control.)
func notifToolbarWidget(props notifProps) ui.Node {
	_ = props
	feedAtom := uistate.UseNotifyFeed()
	filter := uistate.UseNotifyFilter()
	onFilter := ui.UseEvent(func(e ui.Event) { filter.Set(e.GetValue()) })
	clearAll := ui.UseEvent(Prevent(func() {
		feedAtom.Set(nil)
		uistate.PersistNotifyFeed(nil)
	}))
	f := filter.Get()

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Label(css.Class("todo-ctrl"),
				Span(css.Class("todo-ctrl-label"), uistate.T("notifications.showLabel")),
				Select(css.Class("todo-select"), Attr("data-testid", "notif-filter"), Attr("aria-label", uistate.T("notifications.showLabel")), OnChange(onFilter),
					Option(Value(""), SelectedIf(f == ""), uistate.T("notifications.filterAll")),
					Option(Value("critical"), SelectedIf(f == "critical"), notifySeverityLabel("critical")),
					Option(Value("warning"), SelectedIf(f == "warning"), notifySeverityLabel("warning")),
					Option(Value("info"), SelectedIf(f == "info"), notifySeverityLabel("info")),
				),
			),
		),
		Button(css.Class("strip-toggle notif-clear"), Type("button"), Attr("data-testid", "notif-clear-all"),
			Attr("aria-label", uistate.T("notifications.clearAllAria")), OnClick(clearAll), Text(uistate.T("notifications.clearAll"))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "notif-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- notif-list ------------------------------------------------------------------

// notifListWidget is the feed tile: the visible items, filtered by the severity strip,
// severity-sorted (critical first, recency within a tier), each rendered as a notifyRow
// card with a ⋯ menu. Owns the per-item mutation callbacks + the empty state.
func notifListWidget(props notifProps) ui.Node {
	_ = props
	feedAtom := uistate.UseNotifyFeed()
	filter := uistate.UseNotifyFilter()
	feed := feedAtom.Get()
	now := time.Now().Unix()
	visible := uistate.VisibleFeed(feed, now)

	// Severity filter (empty severity is treated as "info").
	if f := filter.Get(); f != "" {
		kept := visible[:0:0]
		for _, it := range visible {
			sev := it.Severity
			if sev == "" {
				sev = "info"
			}
			if sev == f {
				kept = append(kept, it)
			}
		}
		visible = kept
	}

	// Prioritize by severity; stable sort keeps recency within each tier.
	sort.SliceStable(visible, func(i, j int) bool {
		return notifySeverityRank(visible[i].Severity) > notifySeverityRank(visible[j].Severity)
	})

	if len(feed) == 0 {
		return notifListTile(P(css.Class("empty"), uistate.T("notifications.empty")))
	}
	if len(visible) == 0 {
		return notifListTile(P(css.Class("empty"), uistate.T("notifications.noneMatch")))
	}

	pr := uistate.UsePrefs().Get()
	rows := make([]ui.Node, 0, len(visible))
	for _, it := range visible {
		id := it.ID
		isRead := it.Read
		timeStr := relativeTime(it.At, now)
		if timeStr == "" {
			timeStr = pr.FormatDate(time.Unix(it.At, 0))
		}
		rows = append(rows, ui.CreateElement(notifyRow, notifyRowProps{
			Item:      it,
			TimeStr:   timeStr,
			OnRead:    func() { uistate.MarkFeedItemRead(id, !isRead) },
			OnDismiss: func() { uistate.DismissFeedItem(id) },
			OnSnooze:  func() { uistate.SnoozeFeedItem(id, time.Now().Unix()+86400) },
		}))
	}
	listArgs := []any{css.Class("notif-list"), Attr("role", "list")}
	for _, r := range rows {
		listArgs = append(listArgs, r)
	}
	return notifListTile(Div(listArgs...))
}

// notifListTile wraps the list body in the standard surface-host widget shell.
func notifListTile(body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: "notif-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: uiw.EntityListSection(uiw.EntityListSectionProps{Title: uistate.T("nav.notifications"), Body: body}),
	})
}
