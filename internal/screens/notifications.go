// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// notifyLastSeenKey is the SQLite-backed KV key that persists the unix-second
// timestamp of the last time the user viewed the Notification Center (C271).
const notifyLastSeenKey = "cashflux:notify:lastSeen"

// notifySeverityPill returns a compact labelled pill for the given severity
// string (C267). It uses a text label — not color alone — so the meaning is
// accessible to users who cannot distinguish colors (WCAG 1.4.1). The empty
// string and "info" both render the neutral info pill (legacy items are info).
func notifySeverityPill(sev string) ui.Node {
	switch sev {
	case "warning":
		return Span(css.Class("sev-pill", "sev-warning"), Attr("aria-label", "Warning"), "Warning")
	case "critical":
		return Span(css.Class("sev-pill", "sev-critical"), Attr("aria-label", "Critical"), "Critical")
	default:
		return Span(css.Class("sev-pill", "sev-info"), Attr("aria-label", "Info"), "Info")
	}
}

// notifyRowProps are the props passed to each notification row component (C268).
// Using a dedicated component (not inline closures) keeps On* hooks at a stable
// call-site depth — required by the framework (CLAUDE.md "CRITICAL gotchas").
type notifyRowProps struct {
	Item      uistate.FeedItem
	DateStr   string
	OnRead    func()
	OnDismiss func()
	OnSnooze  func()
}

// notifyRow is a self-contained component for one Notification Center row. It
// owns its own event hooks (mark-read, dismiss, snooze) which are passed down
// as plain func callbacks from the parent. Wrapping the props callbacks in
// ui.UseEvent here — not in the parent loop — is the correct pattern: the
// framework requires On* hooks to be registered at a stable component depth,
// never inside a variable-length loop (CLAUDE.md "CRITICAL gotchas").
func notifyRow(props notifyRowProps) ui.Node {
	readLabel := "Mark as read"
	readIcon := "○"
	if props.Item.Read {
		readLabel = "Mark as unread"
		readIcon = "●"
	}

	onRead := ui.UseEvent(func() {
		if props.OnRead != nil {
			props.OnRead()
		}
	})
	onDismiss := ui.UseEvent(func() {
		if props.OnDismiss != nil {
			props.OnDismiss()
		}
	})
	onSnooze := ui.UseEvent(func() {
		if props.OnSnooze != nil {
			props.OnSnooze()
		}
	})

	return Div(css.Class("row"), Attr("role", "listitem"),
		// Left: title + optional body.
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Item.Title),
			If(props.Item.Body != "", Span(css.Class("row-meta"), props.Item.Body)),
		),
		// Right: severity pill + date + per-item controls.
		Div(css.Class("row-meta", tw.Flex, tw.ItemsCenter, tw.Gap2),
			notifySeverityPill(props.Item.Severity),
			Span(css.Class(tw.TextFaint), props.DateStr),
			// Mark read/unread toggle.
			Button(
				css.Class("notif-ctrl-btn"),
				Type("button"),
				Attr("aria-label", readLabel),
				Attr("title", readLabel),
				OnClick(onRead),
				readIcon,
			),
			// Snooze 1 day.
			Button(
				css.Class("notif-ctrl-btn"),
				Type("button"),
				Attr("aria-label", "Snooze for 1 day"),
				Attr("title", "Snooze for 1 day"),
				OnClick(onSnooze),
				"⏱",
			),
			// Dismiss (remove).
			Button(
				css.Class("notif-ctrl-btn", "notif-ctrl-dismiss"),
				Type("button"),
				Attr("aria-label", "Dismiss notification"),
				Attr("title", "Dismiss"),
				OnClick(onDismiss),
				"✕",
			),
		),
	)
}

// loadLastSeen reads the persisted last-seen timestamp from the SQLite-backed
// KV store (C271). Returns 0 (treat all items as potentially new, but suppress
// the banner on first open) when absent or unparseable.
func loadLastSeen() int64 {
	raw := uistate.KVGet(notifyLastSeenKey)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// saveLastSeen persists the current unix-second timestamp as the last time the
// user viewed the Notification Center (C271).
func saveLastSeen(ts int64) {
	uistate.KVSet(notifyLastSeenKey, strconv.FormatInt(ts, 10))
}

// NotificationCenter lists the notifications surfaced by the catch-up engine (bill
// due, budget thresholds, stale balances, digests, …) — the persisted feed (C75).
// Opening it marks everything read; "Clear all" empties it. Per-item controls
// (mark-read/unread, dismiss, snooze 1 day) are available on each row (C268).
// A "Since your last visit" banner groups items newer than the persisted
// last-seen timestamp so users get a clear catch-up digest on re-open (C271).
func NotificationCenter() ui.Node {
	feedAtom := uistate.UseNotifyFeed()
	feed := feedAtom.Get()

	// Apply the snooze filter: hide items snoozed until a future time.
	now := time.Now().Unix()
	visible := uistate.VisibleFeed(feed, now)

	// C271: read lastSeen before the mark-read effect mutates state, so the
	// catch-up count reflects what arrived since the prior open, not zero.
	lastSeen := loadLastSeen()
	newSince := uistate.NewSinceLastSeen(visible, lastSeen)
	newCount := len(newSince)

	// Mark all visible items read when the center is open (so the rail badge clears).
	// Also stamp lastSeen = now so the next open has the correct baseline (C271).
	ui.UseEffect(func() func() {
		saveLastSeen(now)

		if uistate.UnreadNotifyCount(visible) == 0 {
			return nil
		}
		next := make([]uistate.FeedItem, len(feed))
		copy(next, feed)
		for i, it := range next {
			for _, v := range visible {
				if it.ID == v.ID {
					next[i].Read = true
					break
				}
			}
		}
		feedAtom.Set(next)
		uistate.PersistNotifyFeed(next)
		return nil
	}, fmt.Sprintf("notif-read:%d", len(visible)))

	clearAll := ui.UseEvent(func() {
		feedAtom.Set(nil)
		uistate.PersistNotifyFeed(nil)
	})

	if len(visible) == 0 {
		return uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.notifications"),
			Body:  P(css.Class("empty"), uistate.T("notifications.empty")),
		})
	}

	pr := uistate.UsePrefs().Get()

	// Build one component per visible item. Callbacks are plain func values
	// closed over the item's ID (a stable string), not On* hooks — the row
	// component owns the On* hook registration at its own stable depth.
	type rowEntry struct {
		item      uistate.FeedItem
		dateStr   string
		onRead    func()
		onDismiss func()
		onSnooze  func()
	}
	rows := make([]rowEntry, len(visible))
	for i, it := range visible {
		id := it.ID
		when := time.Unix(it.At, 0)
		isRead := it.Read
		rows[i] = rowEntry{
			item:    it,
			dateStr: pr.FormatDate(when),
			onRead: func() {
				uistate.MarkFeedItemRead(id, !isRead)
			},
			onDismiss: func() {
				uistate.DismissFeedItem(id)
			},
			onSnooze: func() {
				uistate.SnoozeFeedItem(id, time.Now().Unix()+86400)
			},
		}
	}

	items := make([]ui.Node, 0, len(rows))
	for _, r := range rows {
		items = append(items, ui.CreateElement(notifyRow, notifyRowProps{
			Item:      r.item,
			DateStr:   r.dateStr,
			OnRead:    r.onRead,
			OnDismiss: r.onDismiss,
			OnSnooze:  r.onSnooze,
		}))
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

	// C271: "Since your last visit" catch-up banner. Shown only when:
	//   • lastSeen > 0 (not the user's very first open — suppress on first open)
	//   • newCount > 0 (there are genuinely new items since last visit)
	// The banner is informational only; the items appear in the main list below.
	var catchUpBanner ui.Node
	if newCount > 0 && lastSeen > 0 {
		label := uistate.T("notifications.sinceLastVisitOne")
		if newCount > 1 {
			label = uistate.T("notifications.sinceLastVisit", newCount)
		}
		catchUpBanner = Div(
			css.Class("notif-catchup-banner"),
			Attr("role", "status"),
			Attr("aria-live", "polite"),
			Span(css.Class("notif-catchup-label"), uistate.T("notifications.catchUpHeader")),
			Span(css.Class("notif-catchup-count"), label),
		)
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Header: Div(css.Class("budget-head"),
			H2(css.Class("card-title"), uistate.T("nav.notifications")),
			Button(css.Class("btn"), Type("button"), OnClick(clearAll),
				Attr("aria-label", uistate.T("notifications.clearAllAria")),
				uistate.T("notifications.clearAll")),
		),
		Body: Div(
			If(catchUpBanner != nil, catchUpBanner),
			listBody,
		),
	})
}
