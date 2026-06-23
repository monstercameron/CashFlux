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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
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
	pr := uistate.UsePrefs().Get()

	// The inline Undo button appears only for the first (newest) row when the
	// undo stack is non-empty.  Clicking it calls the same path as Ctrl+Z.
	doUndo := ui.UseEvent(func() {
		auditview.UndoFunc()
	})

	showUndo := props.IsFirst && auditview.CanUndoFunc()

	var timeLabel string
	if e.At.IsZero() {
		timeLabel = "—"
	} else {
		timeLabel = pr.FormatDate(e.At)
	}

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

	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Div(css.Class("row-desc"),
				Span(css.Class(tw.FontMedium), actionLabel),
				If(e.EntityType != "", Span(css.Class("row-meta", tw.TextFaint), " · "+e.EntityType)),
			),
			Span(css.Class("row-meta"), e.Summary),
		),
		Div(css.Class("row-aside"),
			Span(css.Class("row-meta", tw.TextFaint), timeLabel),
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

// Activity is the Activity / History timeline screen.
func Activity() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}
	_ = uistate.UseDataRevision().Get() // re-render on undo/redo, import, wipe

	entries := buildActivityFeed(app)

	navTitle := uistate.T("nav.activity")
	if navTitle == "nav.activity" {
		navTitle = "Activity"
	}
	subTitle := uistate.T("screen.activitySub")
	if subTitle == "screen.activitySub" {
		subTitle = "Recent changes and history"
	}
	emptyMsg := uistate.T("activity.empty")
	if emptyMsg == "activity.empty" {
		emptyMsg = "No changes recorded yet — start by adding a transaction."
	}

	if len(entries) == 0 {
		return Section(css.Class("card"),
			H2(css.Class("card-title"), navTitle),
			P(css.Class("empty"), emptyMsg),
		)
	}

	rows := make([]ui.Node, 0, len(entries))
	for i, e := range entries {
		e := e
		rows = append(rows, ui.CreateElement(activityRow, activityRowProps{
			Entry:   e,
			IsFirst: i == 0,
		}))
	}

	return Section(css.Class("card"),
		H2(css.Class("card-title"), navTitle),
		P(css.Class("row-meta", tw.TextFaint), subTitle),
		Div(css.Class("rows"), rows),
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
