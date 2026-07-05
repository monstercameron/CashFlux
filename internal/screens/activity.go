// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

// Activity is the Activity / History timeline screen (C78 phase 4).
// It renders a reverse-chronological list of recent dataset changes drawn from
// two sources:
//
//  1. The in-memory audit feed (auditview.Feed) — populated by
//     internal/app.RecordAuditPoint once the Phase-2 commit seam
//     (appstate.App.commit) is built and calls it from captureUndoPoint.
//     Until then the feed is empty and the screen falls back to source 2.
//
//  2. A synthesised feed derived from live appstate data: recent transactions by
//     date, recent tasks by due-date.  This provides a useful timeline today
//     without requiring Phase-2 work.
//
// Each row is its own component (activityRow) so the OnClick handler for the
// inline "Undo" button is registered at a stable call-site — never inside a
// variable-length loop (CLAUDE.md §"CRITICAL gotchas").
//
// Route: /activity  (GroupTools / SubGroupData — audit & data-provenance tools).
// Route registration: the Route entry must be appended to screens.All() in
// screens.go (cannot be done here without editing that existing file).
//
// i18n keys needed (not yet in internal/i18n/en.go — add when screens.go is
// updated to register the route):
//
//	nav.activity          "Activity"
//	screen.activitySub    "Recent changes and history"
//	activity.empty        "No changes recorded yet — start by adding a transaction."
//	activity.undoBtn      "Undo this change"
//	activity.user         "You"
//	activity.system       "System"
//	activity.labelAdded   "Added"
//	activity.labelUpdated "Updated"
//	activity.labelDeleted "Deleted"
//	activity.labelChanged "Changed"
import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// activityRowProps are the props passed to each timeline row component.
type activityRowProps struct {
	Entry   auditlog.Entry
	IsFirst bool // first (newest) entry — may have an inline Undo affordance
}

// activityRow renders a single audit timeline row with an optional inline Undo
// affordance for the most-recent entry.  It is a dedicated component so that
// ui.UseEvent is registered at a stable, fixed call-site — never inside a
// variable-length loop.
func activityRow(props activityRowProps) ui.Node {
	e := props.Entry

	// The inline Undo button appears only for the first (newest) row when the
	// undo stack is non-empty.  Clicking it calls the same path as Ctrl+Z.
	doUndo := ui.UseEvent(func() {
		auditview.UndoFunc()
	})

	showUndo := props.IsFirst && auditview.CanUndoFunc()

	actionLabel := uistate.T("activity.label" + actCapitalize(e.Action))
	if actionLabel == "activity.label"+actCapitalize(e.Action) {
		// i18n key not yet in catalog — use the raw action word as fallback.
		actionLabel = actCapitalize(e.Action)
	}

	actorLabel := e.Actor
	switch actorLabel {
	case "user":
		if v := uistate.T("activity.user"); v != "activity.user" {
			actorLabel = v
		} else {
			actorLabel = "You"
		}
	case "system":
		if v := uistate.T("activity.system"); v != "activity.system" {
			actorLabel = v
		} else {
			actorLabel = "System"
		}
	}

	undoBtnLabel := uistate.T("activity.undoBtn")
	if undoBtnLabel == "activity.undoBtn" {
		undoBtnLabel = "Undo this change"
	}

	// Action tick tone: additions read in the accent, deletions in the danger
	// tone, everything else neutral. The day divider above the row carries the
	// date, so the aside holds just the actor (and the newest row's Undo).
	rowCls := "row act-entry"
	switch e.Action {
	case "added", "create", "created":
		rowCls += " act-add"
	case "deleted", "delete", "removed":
		rowCls += " act-del"
	}

	return Div(ClassStr(rowCls),
		Div(css.Class("row-main"),
			Div(css.Class("row-desc"),
				Span(css.Class(tw.FontMedium), actionLabel),
				If(e.EntityType != "", Span(css.Class("row-meta", tw.TextFaint), " · "+e.EntityType)),
			),
			Span(css.Class("row-meta"), e.Summary),
		),
		Div(css.Class("row-aside"),
			If(e.Actor != "", Span(css.Class("row-meta", tw.TextFaint), actorLabel)),
			If(showUndo,
				Button(
					css.Class("btn btn-xs"),
					Type("button"),
					Attr("aria-label", undoBtnLabel),
					OnClick(doUndo),
					undoBtnLabel,
				),
			),
		),
	)
}

// activityFilterAtomID is the state atom key for the entity-type filter on the
// Activity screen. The empty string means "all entity types".
const activityFilterAtomID = "activity:entityFilter"

// activityEntityOptions returns the SelectInput options for the entity-type
// filter. The first entry is always "All" (empty value = no filter).
//
// i18n keys (add to en.go when registering the route):
//
//	activity.filterAll          "All changes"
//	activity.filterTransaction  "Transactions"
//	activity.filterAccount      "Accounts"
//	activity.filterBudget       "Budgets"
//	activity.filterGoal         "Goals"
//	activity.filterTask         "Tasks"
//	activity.filterCategory     "Categories"
//	activity.filterMember       "Members"
func activityEntityOptions() []uiw.SelectOption {
	label := func(key, fallback string) string {
		if v := uistate.T(key); v != key {
			return v
		}
		return fallback
	}
	return []uiw.SelectOption{
		{Value: "", Label: label("activity.filterAll", "All changes")},
		{Value: "transaction", Label: label("activity.filterTransaction", "Transactions")},
		{Value: "account", Label: label("activity.filterAccount", "Accounts")},
		{Value: "budget", Label: label("activity.filterBudget", "Budgets")},
		{Value: "goal", Label: label("activity.filterGoal", "Goals")},
		{Value: "task", Label: label("activity.filterTask", "Tasks")},
		{Value: "category", Label: label("activity.filterCategory", "Categories")},
		{Value: "member", Label: label("activity.filterMember", "Members")},
	}
}

// Activity is the Activity / History timeline screen.
//
// Entity-type filter: a SelectInput above the timeline narrows the feed to one
// entity type, using the audit feed's per-entity data when available. This
// delivers the C78 "per-entity Recent changes" requirement at the Activity-screen
// level.
//
// Follow-up (noted): per-entity Recent changes embedded inline on entity detail
// screens (e.g. the transaction inline-editor) is a separate sub-task; it would
// call auditlog.ByEntity(entityType, entityID) and render a compact feed inline.
func Activity() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get() // re-render on undo/redo, import, wipe

	// Entity-type filter state — persists while the screen is mounted.
	filterAtom := state.UseAtom(activityFilterAtomID, "")
	selectedFilter := filterAtom.Get()

	entries := buildActivityFeed(app)

	// Apply entity-type filter when one is selected. The filter logic lives in the
	// pure, unit-tested auditlog package (§1.9 — no logic leaks into view code).
	entries = auditlog.FilterByEntityType(entries, selectedFilter)

	emptyMsg := uistate.T("activity.empty")
	if emptyMsg == "activity.empty" {
		emptyMsg = "No changes recorded yet — start by adding a transaction."
	}
	filterLabel := uistate.T("activity.filterLabel")
	if filterLabel == "activity.filterLabel" {
		filterLabel = "Filter by type"
	}

	filterControl := Div(css.Class("act-filter"), uiw.SelectInput(uiw.SelectInputProps{
		Options:   activityEntityOptions(),
		Selected:  selectedFilter,
		OnChange:  func(v string) { filterAtom.Set(v) },
		AriaLabel: filterLabel,
		TestID:    "activity-entity-filter",
	}))

	// ── Hero: the record's size, who changed what, and the latest change. ──────
	pr := uistate.UsePrefs().Get()
	allEntries := entries // pre-filter view feeds the hero figures
	if selectedFilter != "" {
		allEntries = buildActivityFeed(app)
	}
	youCount, otherCount := 0, 0
	for _, e := range allEntries {
		if e.Actor == "user" || e.Actor == "" {
			youCount++
		} else {
			otherCount++
		}
	}
	chips := []ui.Node{
		rptChip(uistate.T("activity.chipShown"), fmt.Sprintf("%d", len(entries)), ""),
	}
	if otherCount > 0 {
		chips = append(chips,
			rptChip(uistate.T("activity.chipYou"), fmt.Sprintf("%d", youCount), ""),
			rptChip(uistate.T("activity.chipOthers"), fmt.Sprintf("%d", otherCount), ""))
	}
	if auditview.CanUndoFunc() {
		chips = append(chips, rptChip(uistate.T("activity.chipUndo"), uistate.T("activity.chipUndoYes"), rptToneCls("pos")))
	}

	takeaway := uistate.T("act.takeEmpty")
	if len(allEntries) > 0 {
		latest := allEntries[0]
		who := latest.Actor
		switch who {
		case "user", "":
			who = uistate.T("activity.user")
		case "system":
			who = uistate.T("activity.system")
		}
		when := ""
		if !latest.At.IsZero() {
			when = who + ", " + pr.FormatDate(latest.At)
		} else {
			when = who
		}
		takeaway = uistate.T("act.takeLast", actTruncate(latest.Summary, 90), when)
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-record-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("activity.eyebrowTail")),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("activity.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), fmt.Sprintf("%d", len(allEntries))),
			),
		),
		Div(css.Class("debt-chips"), chips),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "record-takeaway"), takeaway),
	)
	heroTile := rptTile("record-hero", "1 / span 4", rptSection("", uistate.T("activity.heroTitle"), nil, heroBody))

	// ── Timeline: entries grouped under serif day dividers. ────────────────────
	var timeline ui.Node
	if len(entries) == 0 {
		timeline = P(css.Class("empty"), emptyMsg)
	} else {
		nodes := make([]ui.Node, 0, len(entries)+8)
		lastDay := "\x00"
		for i, e := range entries {
			day := uistate.T("activity.dayUndated")
			if !e.At.IsZero() {
				day = pr.FormatDate(e.At)
			}
			if day != lastDay {
				nodes = append(nodes, Div(css.Class("act-day", tw.FontDisplay), day))
				lastDay = day
			}
			nodes = append(nodes, ui.CreateElement(activityRow, activityRowProps{
				Entry:   e,
				IsFirst: i == 0,
			}))
		}
		timeline = hhRowsList(nodes)
	}

	return Div(css.Class("bento bento-record"),
		heroTile,
		rptTile("record-timeline", "1 / span 4",
			rptSection("sec-record-timeline", uistate.T("activity.timelineTitle"), filterControl, timeline)),
	)
}

// ─── feed synthesis ───────────────────────────────────────────────────────────

const activityFeedMax = 50 // maximum rows shown on the timeline

// buildActivityFeed returns at most activityFeedMax entries newest-first.
// It prefers the audit feed (auditview.Feed) and falls back to entity-timestamp
// synthesis when the feed is empty (Phase 2 not yet landed).
func buildActivityFeed(app *appstate.App) []auditlog.Entry {
	// Primary: in-process audit feed (populated by RecordAuditPoint).
	if primary := auditview.Feed.Recent(activityFeedMax); len(primary) > 0 {
		return primary
	}

	// Fallback: synthesise from live entity data.
	var entries []auditlog.Entry

	// Recent transactions — use the transaction business date as the at time.
	for _, t := range app.Transactions() {
		desc := t.Desc
		if desc == "" {
			desc = t.Payee
		}
		entries = append(entries, auditlog.Entry{
			ID:         "txn-" + t.ID,
			At:         t.Date,
			Actor:      "user",
			Action:     "added",
			EntityType: "transaction",
			EntityID:   t.ID,
			Summary:    auditlog.Redact(fmt.Sprintf("Transaction: %s", actTruncate(desc, 60))),
		})
	}

	// Recent tasks — use Due date (tasks have no CreatedAt field); zero-At
	// entries sort to the bottom but are still shown.
	for _, t := range app.Tasks() {
		entries = append(entries, auditlog.Entry{
			ID:         "task-" + t.ID,
			At:         t.Due,
			Actor:      "user",
			Action:     "added",
			EntityType: "task",
			EntityID:   t.ID,
			Summary:    auditlog.Redact(fmt.Sprintf("Task: %s", actTruncate(t.Title, 60))),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].At.After(entries[j].At)
	})
	if len(entries) > activityFeedMax {
		entries = entries[:activityFeedMax]
	}
	return entries
}

// ─── small helpers (prefixed act* to avoid clashing with other screens) ───────

func actCapitalize(s string) string {
	if s == "" {
		return ""
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

func actTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
