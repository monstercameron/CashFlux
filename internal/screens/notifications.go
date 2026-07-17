// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/notify"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// notifGroupMin is the number of same-kind, non-critical notifications above which
// the feed collapses them into one summary row (task: "friendly, never naggy").
// Below this, the individual cards read fine; at or above it, a wall of near-
// identical nags (e.g. eight "needs a balance update" cards) is one tidy card.
const notifGroupMin = 3

// notifyGroupKind returns the rule kind a feed item belongs to — the ID prefix
// before the first '@' (feed IDs are notify.DedupeKey(ruleID, occurrence) =
// "ruleID@occurrence"). Same-kind items are the ones worth grouping.
func notifyGroupKind(id string) string {
	if i := strings.IndexByte(id, '@'); i >= 0 {
		return id[:i]
	}
	return ""
}

// notifyGroupSummary renders the plain-English one-line summary shown on a
// collapsed group ("8 accounts need a balance update"), by rule kind.
func notifyGroupSummary(kind string, n int) string {
	switch kind {
	case "default-stale":
		return uistate.T("notifications.groupStale", n)
	case "default-bill-due":
		return uistate.T("notifications.groupBill", n)
	case "default-budget":
		return uistate.T("notifications.groupBudget", n)
	case "default-low-balance":
		return uistate.T("notifications.groupLowBal", n)
	case "default-large":
		return uistate.T("notifications.groupLarge", n)
	default:
		return uistate.T("notifications.groupGeneric", n)
	}
}

// notifGroupRowProps drive one collapsed group card. Children are the already-
// built notifyRow nodes for the group's members (built in the list widget, which
// owns the per-item callback closures); the group just shows/hides them.
type notifGroupRowProps struct {
	Kind         string
	Severity     string
	Summary      string
	Count        int
	Children      []ui.Node
	OnDismissAll func()
}

// notifGroupRow renders a run of same-kind notifications as a single collapsed
// card: a severity medallion, the plain-English summary + count, a "Dismiss all",
// and a disclosure that expands to the individual rows. It owns its expanded
// state so the surrounding feed stays a flat list.
func notifGroupRow(props notifGroupRowProps) ui.Node {
	expanded := ui.UseState(false)
	open := expanded.Get()
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	dismissAll := ui.UseEvent(Prevent(func() {
		if props.OnDismissAll != nil {
			props.OnDismissAll()
		}
	}))

	cardCls := "notif-group " + notifySeverityClass(props.Severity)
	if open {
		cardCls += " is-open"
	}
	discLabel := uistate.T("notifications.groupShow")
	if open {
		discLabel = uistate.T("notifications.groupHide")
	}

	head := Div(css.Class("notif-group-head"),
		Button(css.Class("notif-group-toggle"), Type("button"),
			Attr("data-testid", "notif-group-toggle-"+props.Kind),
			Attr("aria-expanded", ariaBool(open)),
			Attr("aria-label", uistate.T("notifications.groupExpandAria", props.Count)),
			OnClick(toggle),
			Div(css.Class("notif-badge"), Attr("aria-hidden", "true"),
				uiw.Icon(notifySeverityIcon(props.Severity), css.Class(tw.W4, tw.H4))),
			Div(css.Class("notif-group-body-text"),
				Span(css.Class("notif-group-summary"), props.Summary),
				Span(css.Class("notif-group-hint"), uistate.T("notifications.groupHint")),
			),
			Span(css.Class("notif-group-disc"), discLabel,
				uiw.Icon(icon.ChevronDown, css.Class(tw.W4, tw.H4))),
		),
		Button(css.Class("notif-icon-btn notif-dismiss"), Type("button"),
			Attr("data-testid", "notif-group-dismiss-"+props.Kind),
			Attr("aria-label", uistate.T("notifications.groupDismissAll")),
			Title(uistate.T("notifications.groupDismissAll")),
			OnClick(dismissAll), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)

	var body ui.Node = Fragment()
	if open {
		bodyArgs := []any{css.Class("notif-group-list"), Attr("role", "list")}
		for _, c := range props.Children {
			bodyArgs = append(bodyArgs, c)
		}
		body = Div(bodyArgs...)
	}

	return Div(ClassStr(cardCls), Attr("role", "listitem"), Attr("data-testid", "notif-group-"+props.Kind),
		head, body,
	)
}

// notifyLastSeenKey is the SQLite-backed KV key that persists the unix-second
// timestamp of the last time the user viewed the Notification Center (C271).
const notifyLastSeenKey = "cashflux:notify:lastSeen"

// UseNotifyView is the shared Live/History view selector for the Notifications
// surface ("live" = the current feed, the default; "history" = the persisted
// archive). Read by notifSurfaceShell.
func UseNotifyView() state.Atom[string] { return state.UseAtom("notify:view", "live") }

// notifSurfaceShellProps carries the app handle plus the already-built live
// surface node so the shell can swap it for the archive view without rebuilding.
type notifSurfaceShellProps struct {
	App  *appstate.App
	Live ui.Node
}

// notifSurfaceShell wraps the Notifications surface with a Live/History view
// toggle: "Live" shows the existing feed (props.Live); "History" shows the
// persisted archive (notificationHistoryView). On mount it folds the current
// live feed into the archive so past alerts accumulate — idempotent, since the
// seam dedupes by feed ID.
func notifSurfaceShell(props notifSurfaceShellProps) ui.Node {
	view := UseNotifyView()
	cur := view.Get()

	// Fill the archive from the live feed whenever the surface mounts. This is the
	// least-invasive RecordNotification hook: it runs once per mount, off the live
	// persisted feed, and dedupe-by-ID makes re-mounting safe (no double records).
	ui.UseEffect(func() func() {
		uistate.SyncFeedToArchive()
		return nil
	}, "notif-archive-sync")

	setLive := ui.UseEvent(Prevent(func() { view.Set("live") }))
	setHistory := ui.UseEvent(Prevent(func() { view.Set("history") }))

	toggle := Div(css.Class("nhx-toggle"), Attr("role", "tablist"), Attr("aria-label", uistate.T("nav.notifications")),
		Button(css.Class("nhx-toggle-btn"), Type("button"), Attr("role", "tab"),
			Attr("data-testid", "notif-view-live"), Attr("aria-selected", ariaBool(cur != "history")),
			OnClick(setLive), Text(uistate.T("notifHistory.live"))),
		Button(css.Class("nhx-toggle-btn"), Type("button"), Attr("role", "tab"),
			Attr("data-testid", "notif-view-history"), Attr("aria-selected", ariaBool(cur == "history")),
			OnClick(setHistory), Text(uistate.T("notifHistory.history"))),
	)

	var content ui.Node = props.Live
	if cur == "history" {
		content = ui.CreateElement(notificationHistoryView, notifHistoryProps{App: props.App})
	}

	return Div(css.Class("nhx-surface"),
		Div(css.Class("nhx-head"), toggle),
		content,
	)
}

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
// notifyTxnSearchLabel resolves the merchant/description a transaction-scoped
// notification should pre-search the ledger by, so the click lands on that exact
// charge rather than the full list. Empty when the transaction is gone (the caller
// then falls back to a plain page navigation).
func notifyTxnSearchLabel(txnID string) string {
	app := appstate.Default
	if app == nil {
		return ""
	}
	for _, t := range app.Transactions() {
		if t.ID != txnID {
			continue
		}
		if s := strings.TrimSpace(t.Payee); s != "" {
			return s
		}
		return strings.TrimSpace(t.Desc)
	}
	return ""
}

func notifyRow(props notifyRowProps) ui.Node {
	it := props.Item
	sev := it.Severity
	readLabel := uistate.T("notifications.markRead")
	if it.Read {
		readLabel = uistate.T("notifications.markUnread")
	}

	route := routeForNotify(it)
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	// A notification isn't just about a page — it's about one specific thing on it.
	// So landing filters/flashes that exact item: a flagged charge opens the ledger
	// pre-searched to its merchant; an account or budget alert scrolls to and pulses
	// its own card. Unresolvable targets fall back to a plain page navigation.
	goResource := ui.UseEvent(Prevent(func() {
		if route == "" {
			return
		}
		switch tgt := notify.ParseTarget(it.ID); tgt.Kind {
		case notify.TargetTxn:
			if label := notifyTxnSearchLabel(tgt.ID); label != "" {
				f := txFilter.Get()
				f.Text = label
				f = f.Normalize()
				txFilter.Set(f)
				uistate.PersistTxFilter(f)
				nav.Navigate(uistate.RoutePath("/transactions"))
				return
			}
		case notify.TargetAccount:
			if route == "/accounts" {
				uistate.SetDeepLinkFocus(`[data-testid="acct-row-` + tgt.ID + `"]`)
			}
		case notify.TargetBudget:
			if route == "/budgets" {
				uistate.SetDeepLinkFocus(`[data-testid="budget-card-` + tgt.ID + `"]`)
			}
		}
		nav.Navigate(uistate.RoutePath(route))
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
		Div(css.Class("notif-badge"), Attr("aria-hidden", "true"), uiw.Icon(notifySeverityIcon(sev), css.Class(tw.W4, tw.H4))),
		Div(css.Class("notif-body"),
			Div(css.Class("notif-top"),
				unreadDot,
				Span(css.Class("notif-title"), it.Title),
				If(route != "", Span(css.Class("notif-go"), Attr("aria-hidden", "true"), uiw.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4)))),
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
				Attr("aria-label", readLabel), Title(readLabel), OnClick(onRead), uiw.Icon(icon.Check, css.Class(tw.W4, tw.H4))),
			Button(css.Class("notif-icon-btn"), Type("button"), Attr("data-testid", "notif-snooze-"+it.ID),
				Attr("aria-label", uistate.T("notifications.snooze")), Title(uistate.T("notifications.snooze")), OnClick(onSnooze), uiw.Icon(icon.Clock, css.Class(tw.W4, tw.H4))),
			Button(css.Class("notif-icon-btn notif-dismiss"), Type("button"), Attr("data-testid", "notif-dismiss-"+it.ID),
				Attr("aria-label", uistate.T("notifications.dismiss")), Title(uistate.T("notifications.dismiss")), OnClick(onDismiss), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
}
