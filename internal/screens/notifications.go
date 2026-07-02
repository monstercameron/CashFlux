// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// notifyLastSeenKey is the SQLite-backed KV key that persists the unix-second
// timestamp of the last time the user viewed the Notification Center (C271).
const notifyLastSeenKey = "cashflux:notify:lastSeen"

// routeForNotify resolves a notification's link from the persisted route config
// (uistate.NotifyRoutes — a store-backed, runtime-editable table, defaults seeded on
// first read), matched by the item's ID prefix. Empty = not clickable.
func routeForNotify(it uistate.FeedItem) string {
	return uistate.RouteForNotifyID(it.ID)
}

// notifySeverityRank orders severities for the prioritized feed: critical above
// warning above everything else (info / reminders). Higher = more urgent.
func notifySeverityRank(sev string) int {
	switch sev {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

// notifySeverityClass maps a severity to its card modifier class.
func notifySeverityClass(sev string) string {
	switch sev {
	case "critical":
		return "sev-critical"
	case "warning":
		return "sev-warning"
	default:
		return "sev-info"
	}
}

// notifySeverityIcon is the glyph shown in a notification's severity medallion.
func notifySeverityIcon(sev string) icon.Name {
	switch sev {
	case "critical":
		return icon.AlertTriangle
	case "warning":
		return icon.AlertCircle
	default:
		return icon.Bell
	}
}

// notifySeverityLabel is the accessible text label for a severity (color is never
// the only cue — WCAG 1.4.1).
func notifySeverityLabel(sev string) string {
	switch sev {
	case "critical":
		return uistate.T("notifications.sevCritical")
	case "warning":
		return uistate.T("notifications.sevWarning")
	default:
		return uistate.T("notifications.sevInfo")
	}
}

// relativeTime renders a compact "how long ago" for a feed item. It returns "" for
// anything older than a week so the caller falls back to an absolute date, and treats
// a future/just-arrived stamp as "just now".
func relativeTime(at, now int64) string {
	d := now - at
	switch {
	case d < 60:
		return uistate.T("notifications.justNow")
	case d < 3600:
		return uistate.T("notifications.minutesAgo", d/60)
	case d < 86400:
		return uistate.T("notifications.hoursAgo", d/3600)
	case d < 172800:
		return uistate.T("notifications.yesterday")
	case d < 604800:
		return uistate.T("notifications.daysAgo", d/86400)
	default:
		return ""
	}
}

// loadLastSeen reads the persisted last-seen timestamp from the SQLite-backed KV store
// (C271). Returns 0 when absent/unparseable (treat as first open — suppress the banner).
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

// saveLastSeen persists the current unix-second timestamp as the last time the user
// viewed the Notification Center (C271).
func saveLastSeen(ts int64) { uistate.KVSet(notifyLastSeenKey, strconv.FormatInt(ts, 10)) }

// notifyRowProps are the props for one notification card. Callbacks are plain funcs
// closed over the item ID in the parent; the row wraps them in its OWN On* hooks (at a
// stable depth), never inside the parent's loop (CLAUDE.md "CRITICAL gotchas").
type notifyRowProps struct {
	Item      uistate.FeedItem
	TimeStr   string
	OnRead    func()
	OnDismiss func()
	OnSnooze  func()
}

// notifyRow renders one notification as a card: a severity medallion (icon) on the left,
// the title (with an unread dot) + body + a "severity · time" foot, and INLINE one-click
// actions on the right (mark read/unread, snooze 1 day, dismiss) — faster than burying
// them in a menu. Read items dim; unread items carry a vivid severity accent — the feed
// reads like a triage log.
func notifyRow(props notifyRowProps) ui.Node {
	it := props.Item
	sev := it.Severity
	readLabel := uistate.T("notifications.markRead")
	if it.Read {
		readLabel = uistate.T("notifications.markUnread")
	}

	route := routeForNotify(it)
	nav := router.UseNavigate()
	goResource := ui.UseEvent(Prevent(func() {
		if route != "" {
			nav.Navigate(uistate.RoutePath(route))
		}
	}))
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

	cardCls := "notif " + notifySeverityClass(sev)
	if it.Read {
		cardCls += " is-read"
	} else {
		cardCls += " is-unread"
	}

	var unreadDot ui.Node = Fragment()
	if !it.Read {
		unreadDot = Span(css.Class("notif-dot"), Attr("aria-label", uistate.T("notifications.unread")))
	}

	// The badge + body is the navigable region: clicking it jumps to the alerting
	// resource (a bill → /bills, a budget → /budgets, …). It's a sibling of the actions
	// so the mark-read / snooze / dismiss buttons don't trigger navigation.
	mainCls := "notif-main"
	if route != "" {
		mainCls += " is-linked"
	}
	mainArgs := []any{ClassStr(mainCls)}
	if route != "" {
		mainArgs = append(mainArgs,
			Attr("role", "button"), Attr("tabindex", "0"),
			Attr("aria-label", uistate.T("notifications.openResource", it.Title)),
			Attr("data-testid", "notif-open-"+it.ID), OnClick(goResource))
	}
	mainArgs = append(mainArgs,
		Div(css.Class("notif-badge"), Attr("aria-hidden", "true"), uiw.Icon(notifySeverityIcon(sev), css.Class("w-4", "h-4"))),
		Div(css.Class("notif-body"),
			Div(css.Class("notif-top"),
				unreadDot,
				Span(css.Class("notif-title"), it.Title),
				If(route != "", Span(css.Class("notif-go"), Attr("aria-hidden", "true"), uiw.Icon(icon.ChevronRight, css.Class("w-4", "h-4")))),
			),
			If(it.Body != "", P(css.Class("notif-text"), it.Body)),
			Div(css.Class("notif-foot"),
				Span(ClassStr("notif-sev-tag "+notifySeverityClass(sev)), notifySeverityLabel(sev)),
				Span(css.Class("notif-sep"), "·"),
				Span(css.Class("notif-time"), props.TimeStr),
			),
		),
	)

	return Div(ClassStr(cardCls), Attr("role", "listitem"), Attr("data-testid", "notif-"+it.ID),
		Div(mainArgs...),
		// Inline, one-click actions (revealed on hover; always shown on touch). No ⋯ menu —
		// a per-notification menu is an extra click for actions you take constantly.
		Div(css.Class("notif-actions"),
			Button(css.Class("notif-icon-btn"), Type("button"), Attr("data-testid", "notif-read-"+it.ID),
				Attr("aria-label", readLabel), Title(readLabel), OnClick(onRead), uiw.Icon(icon.Check, css.Class("w-4", "h-4"))),
			Button(css.Class("notif-icon-btn"), Type("button"), Attr("data-testid", "notif-snooze-"+it.ID),
				Attr("aria-label", uistate.T("notifications.snooze")), Title(uistate.T("notifications.snooze")), OnClick(onSnooze), uiw.Icon(icon.Clock, css.Class("w-4", "h-4"))),
			Button(css.Class("notif-icon-btn notif-dismiss"), Type("button"), Attr("data-testid", "notif-dismiss-"+it.ID),
				Attr("aria-label", uistate.T("notifications.dismiss")), Title(uistate.T("notifications.dismiss")), OnClick(onDismiss), uiw.Icon(icon.Close, css.Class("w-4", "h-4"))),
		),
	)
}
