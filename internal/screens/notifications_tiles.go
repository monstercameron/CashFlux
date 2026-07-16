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

	// The severity chips ARE the filter: clicking one narrows the feed to that tier,
	// clicking "All" (or the active chip again) resets. This unifies the severity
	// breakdown with the control that used to live in a separate, near-empty strip.
	filter := uistate.UseNotifyFilter()
	f := filter.Get()
	pick := func(sev string) func() { return func() { filter.Set(sev) } }
	toggle := func(sev string) func() {
		return func() {
			if filter.Get() == sev {
				filter.Set("")
				return
			}
			filter.Set(sev)
		}
	}

	chips := []ui.Node{
		ui.CreateElement(notifSevChip, notifSevChipProps{
			Kind: "all", Label: uistate.T("notifications.filterAll"), Count: -1,
			Active: f == "", OnPick: pick(""),
		}),
	}
	if crit > 0 {
		chips = append(chips, ui.CreateElement(notifSevChip, notifSevChipProps{
			Kind: "sev-critical", Label: notifySeverityLabel("critical"), Count: crit,
			Active: f == "critical", OnPick: toggle("critical"),
		}))
	}
	if warn > 0 {
		chips = append(chips, ui.CreateElement(notifSevChip, notifSevChipProps{
			Kind: "sev-warning", Label: notifySeverityLabel("warning"), Count: warn,
			Active: f == "warning", OnPick: toggle("warning"),
		}))
	}
	if info > 0 {
		chips = append(chips, ui.CreateElement(notifSevChip, notifSevChipProps{
			Kind: "sev-info", Label: notifySeverityLabel("info"), Count: info,
			Active: f == "info", OnPick: toggle("info"),
		}))
	}

	clearAll := ui.UseEvent(Prevent(func() {
		feedAtom.Set(nil)
		uistate.PersistNotifyFeed(nil)
	}))

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

	filtersArgs := []any{css.Class("notif-summary-filters"), Attr("role", "group"),
		Attr("aria-label", uistate.T("notifications.showLabel"))}
	for _, c := range chips {
		filtersArgs = append(filtersArgs, c)
	}

	body := Div(css.Class("notif-summary"),
		Div(css.Class("notif-summary-lead"),
			Span(css.Class("notif-summary-count"), fmt.Sprintf("%d", len(visible))),
			Span(css.Class("notif-summary-label"),
				Span(css.Class("notif-summary-word"), uistate.T("notifications.alertsWord")),
				Span(css.Class("notif-summary-sub"), subLabel)),
		),
		Div(css.Class("notif-summary-actions"),
			Div(filtersArgs...),
			Button(css.Class("notif-clear"), Type("button"), Attr("data-testid", "notif-clear-all"),
				Attr("aria-label", uistate.T("notifications.clearAllAria")), OnClick(clearAll),
				Text(uistate.T("notifications.clearAll"))),
		),
		catchUp,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "notif-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// notifSevChipProps drives one interactive severity filter chip in the summary header.
// The parent passes a plain OnPick closure; the chip owns its own event hook (never
// registered inside the parent's chip loop — CLAUDE.md "CRITICAL gotchas").
type notifSevChipProps struct {
	Kind   string // "all" | "sev-critical" | "sev-warning" | "sev-info"
	Label  string
	Count  int // -1 => no count badge (the "All" chip)
	Active bool
	OnPick func()
}

// notifSevChip renders one filter chip: a severity dot + count + label as a pressable
// toggle. The active chip reads as selected (aria-pressed + tinted) so the header
// doubles as the current-filter indicator.
func notifSevChip(props notifSevChipProps) ui.Node {
	on := ui.UseEvent(Prevent(func() {
		if props.OnPick != nil {
			props.OnPick()
		}
	}))
	cls := "notif-sev-chip " + props.Kind
	if props.Active {
		cls += " is-active"
	}
	args := []any{ClassStr(cls), Type("button"),
		Attr("aria-pressed", ariaBool(props.Active)),
		Attr("data-testid", "notif-filter-"+props.Kind),
		OnClick(on)}
	if props.Kind != "all" {
		args = append(args, Span(css.Class("notif-sev-dot")))
	}
	if props.Count >= 0 {
		args = append(args, Span(css.Class("notif-sev-n"), fmt.Sprintf("%d", props.Count)))
	}
	args = append(args, Span(css.Class("notif-sev-name"), props.Label))
	return Button(args...)
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

	// Decide which kinds to collapse into a single summary card: a kind (rule) with
	// >= notifGroupMin visible items, none of them critical. Critical membership
	// disqualifies the whole kind so an urgent bill (due tomorrow) is never hidden
	// inside a collapsed group. This is the "friendly, never naggy" fix — a wall of
	// eight identical "needs a balance update" cards becomes one tidy row.
	kindCount := map[string]int{}
	kindHasCrit := map[string]bool{}
	for _, it := range visible {
		k := notifyGroupKind(it.ID)
		if k == "" {
			continue
		}
		kindCount[k]++
		if it.Severity == "critical" {
			kindHasCrit[k] = true
		}
	}
	grouped := map[string]bool{}
	for k, n := range kindCount {
		if n >= notifGroupMin && !kindHasCrit[k] {
			grouped[k] = true
		}
	}

	buildRow := func(it uistate.FeedItem) ui.Node {
		id := it.ID
		isRead := it.Read
		timeStr := relativeTime(it.At, now)
		if timeStr == "" {
			timeStr = pr.FormatDate(time.Unix(it.At, 0))
		}
		return ui.CreateElement(notifyRow, notifyRowProps{
			Item:      it,
			TimeStr:   timeStr,
			OnRead:    func() { uistate.MarkFeedItemRead(id, !isRead) },
			OnDismiss: func() { uistate.DismissFeedItem(id) },
			OnSnooze:  func() { uistate.SnoozeFeedItem(id, time.Now().Unix()+86400) },
		})
	}

	// Pre-build each grouped kind's child rows in visible (severity-sorted) order,
	// and record the kind's representative severity (its first, highest-tier member).
	groupChildren := map[string][]ui.Node{}
	groupSev := map[string]string{}
	for _, it := range visible {
		k := notifyGroupKind(it.ID)
		if !grouped[k] {
			continue
		}
		if _, ok := groupSev[k]; !ok {
			groupSev[k] = it.Severity
		}
		groupChildren[k] = append(groupChildren[k], buildRow(it))
	}

	// Emit the feed: a grouped kind renders one collapsed group card at the position
	// of its first (highest-severity) member; everything else renders as before.
	rows := make([]ui.Node, 0, len(visible))
	emitted := map[string]bool{}
	for _, it := range visible {
		k := notifyGroupKind(it.ID)
		if grouped[k] {
			if emitted[k] {
				continue
			}
			emitted[k] = true
			kind := k
			rows = append(rows, ui.CreateElement(notifGroupRow, notifGroupRowProps{
				Kind:     kind,
				Severity: groupSev[kind],
				Summary:  notifyGroupSummary(kind, kindCount[kind]),
				Count:    kindCount[kind],
				Children: groupChildren[kind],
				OnDismissAll: func() {
					uistate.RemoveFeedItems(func(fi uistate.FeedItem) bool {
						return notifyGroupKind(fi.ID) == kind
					})
				},
			}))
			continue
		}
		rows = append(rows, buildRow(it))
	}
	listArgs := []any{css.Class("notif-list"), Attr("role", "list")}
	for _, r := range rows {
		listArgs = append(listArgs, r)
	}
	return notifListTile(Div(listArgs...))
}

// notifListTile wraps the list body in the standard surface-host widget shell. The
// tile carries no heading of its own — the page title already says "Notifications",
// and the summary header above it owns the count + filter — so the feed reads as one
// continuous triage log rather than a card-titled-inside-a-titled-page.
func notifListTile(body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: "notif-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
