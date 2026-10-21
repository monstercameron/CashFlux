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
	"strings"

	"github.com/monstercameron/CashFlux/internal/activityview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditimpact"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditundo"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// activityRowProps are the props passed to each timeline row component.
type activityRowProps struct {
	Entry   auditlog.Entry
	IsFirst bool // first (newest) entry — may have an inline Undo affordance
	Undo    auditundo.Assessment
}

type activityDayProps struct {
	Group   activityview.Day
	Label   string
	FirstID string
}

func activityDay(props activityDayProps) ui.Node {
	return Fragment(
		Div(css.Class("act-day", tw.FontDisplay), props.Label),
		MapKeyed(props.Group.Rows,
			func(row activityview.Row) any { return row.Entry.ID },
			func(row activityview.Row) ui.Node {
				return ui.CreateElement(activityRow, activityRowProps{
					Entry: row.Entry, Undo: row.Undo, IsFirst: row.Entry.ID == props.FirstID,
				})
			},
		),
	)
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

	// #54: the actor doubles as the CAUSE — manual edit, rules engine, file
	// import, or the AI assistant — so every row answers "who or what did this".
	actorLabel := e.Actor
	causeCls := ""
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
	case auditview.ActorAssistant:
		actorLabel = uistate.T("activity.causeAI")
		causeCls = "act-cause"
	case auditview.ActorRule:
		actorLabel = uistate.T("activity.causeRule")
		causeCls = "act-cause"
	case auditview.ActorImport:
		actorLabel = uistate.T("activity.causeImport")
		causeCls = "act-cause"
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

	// Field-level before → after details (updates recorded since the diff layer
	// landed). Capped for scanability; the tail collapses to a count.
	var detailNodes []ui.Node
	const maxDiffLines = 5
	shown := e.Details
	extra := 0
	if len(shown) > maxDiffLines {
		extra = len(shown) - maxDiffLines
		shown = shown[:maxDiffLines]
	}
	for _, d := range shown {
		before := d.Before
		if before == "" {
			before = "—"
		}
		after := d.After
		if after == "" {
			after = "—"
		}
		detailNodes = append(detailNodes, Div(css.Class("act-diff-line"),
			Span(css.Class("act-diff-field"), actFieldLabel(d.Field)),
			Span(css.Class("act-diff-before"), before),
			Span(css.Class("act-diff-arrow"), Attr("aria-hidden", "true"), "→"),
			Span(css.Class("act-diff-after"), after),
		))
	}
	if extra > 0 {
		detailNodes = append(detailNodes, Div(css.Class("act-diff-line", tw.TextFaint), uistate.T("activity.moreChanges", extra)))
	}

	// #54: name the downstream figures this change recomputed — derived from the
	// entry's entity type, action, and changed fields via the pure auditimpact
	// package, so historical entries explain themselves too.
	var recalcLine ui.Node = Fragment()
	fieldNames := make([]string, 0, len(e.Details))
	for _, d := range e.Details {
		fieldNames = append(fieldNames, d.Field)
	}
	if imp := auditimpact.Recalculated(e.EntityType, e.Action, fieldNames); len(imp) > 0 {
		recalcLine = Div(css.Class("act-recalc"), Attr("data-testid", "act-recalc"),
			uistate.T("activity.recalc", strings.Join(imp, " · ")))
	}

	// WF-AUDIT: "undo where safe" — say what, if anything, could take this back.
	//
	// The log records DISPLAY TEXT, not values, so it cannot revert a field on its
	// own; a checkpoint from before the change can roll the whole dataset back to
	// that moment. Both facts are stated rather than hidden behind a disabled
	// button, because a control that does nothing and says nothing is read as a
	// broken feature rather than an honest limit.
	var undoLine ui.Node = Fragment()
	if props.Undo.Possible() {
		undoLine = Div(css.Class("act-undo", tw.TextFaint), Attr("data-testid", "act-undo-"+e.ID),
			uistate.T("activity.undoCheckpoint",
				uistate.LoadPrefs().FormatDate(props.Undo.CheckpointAt), props.Undo.AlsoReverses))
	} else if len(e.Details) > 0 {
		undoLine = Div(css.Class("act-undo", tw.TextFaint), Attr("data-testid", "act-undo-"+e.ID),
			uistate.T("activity.undoNone"))
	}

	actorCls := css.Class("row-meta", tw.TextFaint)
	if causeCls != "" {
		actorCls = css.Class(causeCls)
	}

	return Div(ClassStr(rowCls), Attr("data-testid", "activity-entry-"+e.ID),
		Div(css.Class("row-main"),
			Div(css.Class("row-desc"),
				Span(css.Class(tw.FontMedium), actionLabel),
				If(e.EntityType != "", Span(css.Class("row-meta", tw.TextFaint), " · "+e.EntityType)),
			),
			Span(css.Class("row-meta"), e.Summary),
			If(len(detailNodes) > 0, Div(css.Class("act-diff"), detailNodes)),
			undoLine,
			recalcLine,
		),
		Div(css.Class("row-aside"),
			If(e.Actor != "", Span(actorCls, Attr("data-testid", "act-actor"), actorLabel)),
			If(showUndo,
				Button(
					css.Class("btn btn-xs"),
					Type("button"),
					Attr("data-testid", "activity-undo-"+e.ID),
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

	auditEntries := auditview.Feed.Recent(activityFeedMax)
	var transactions []domain.Transaction
	var tasks []domain.Task
	if len(auditEntries) == 0 {
		transactions = app.Transactions()
		tasks = app.Tasks()
	}
	prepared := activityview.Build(activityview.Input{
		AuditEntries:      auditEntries,
		AuditAvailable:    true,
		Transactions:      transactions,
		Tasks:             tasks,
		SnapshotAvailable: true,
		Limit:             activityFeedMax,
		Filter:            selectedFilter,
		Checkpoints:       uistate.Checkpoints(),
	})

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
	chips := []ui.Node{
		rptChip(uistate.T("activity.chipShown"), fmt.Sprintf("%d", len(prepared.Rows)), ""),
	}
	if prepared.Hero.OtherCount > 0 {
		chips = append(chips,
			rptChip(uistate.T("activity.chipYou"), fmt.Sprintf("%d", prepared.Hero.YouCount), ""),
			rptChip(uistate.T("activity.chipOthers"), fmt.Sprintf("%d", prepared.Hero.OtherCount), ""))
	}
	if auditview.CanUndoFunc() {
		chips = append(chips, rptChip(uistate.T("activity.chipUndo"), uistate.T("activity.chipUndoYes"), rptToneCls("pos")))
	}

	takeaway := uistate.T("act.takeEmpty")
	if prepared.Hero.HasLatest {
		latest := prepared.Hero.Latest
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
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), fmt.Sprintf("%d", prepared.Hero.Count)),
			),
		),
		Div(css.Class("debt-chips"), chips),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "record-takeaway"), takeaway),
	)
	heroTile := rptTile("record-hero", "1 / span 4", rptSection("", uistate.T("activity.heroTitle"), nil, heroBody))

	// ── Timeline: entries grouped under serif day dividers. ────────────────────
	var timeline ui.Node
	if len(prepared.Rows) == 0 {
		timeline = P(css.Class("empty"), emptyMsg)
	} else {
		dayProps := make([]activityDayProps, 0, len(prepared.Days))
		firstID := prepared.Rows[0].Entry.ID
		for _, dayGroup := range prepared.Days {
			day := uistate.T("activity.dayUndated")
			if len(dayGroup.Rows) > 0 && !dayGroup.Rows[0].Entry.At.IsZero() {
				day = pr.FormatDate(dayGroup.Rows[0].Entry.At)
			}
			dayProps = append(dayProps, activityDayProps{Group: dayGroup, Label: day, FirstID: firstID})
		}
		timeline = hhRowsList(MapKeyed(dayProps,
			func(day activityDayProps) any {
				if day.Group.Key == "" {
					return "undated"
				}
				return day.Group.Key
			},
			func(day activityDayProps) ui.Node { return ui.CreateElement(activityDay, day) },
		))
	}

	return Div(css.Class("bento bento-record"),
		heroTile,
		rptTile("record-timeline", "1 / span 4",
			rptSection("sec-record-timeline", uistate.T("activity.timelineTitle"), filterControl, timeline)),
	)
}

// ─── prepared feed ───────────────────────────────────────────────────────────

const activityFeedMax = 50 // maximum rows shown on the timeline

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

// actFieldLabel humanizes a JSON field key for the diff lines: camelCase splits
// into words, a trailing "Id" drops (the value is already resolved to a name),
// and the result lowercases ("categoryId" → "category", "renameDesc" →
// "rename desc").
func actFieldLabel(key string) string {
	if key == "" {
		return key
	}
	var b []rune
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b = append(b, ' ')
		}
		b = append(b, r)
	}
	out := strings.ToLower(string(b))
	out = strings.TrimSuffix(out, " id")
	return out
}
