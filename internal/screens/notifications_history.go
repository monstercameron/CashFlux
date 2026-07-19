// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/notifyhistory"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// notifHistoryProps carries the app handle into the archive view.
type notifHistoryProps struct{ App *appstate.App }

// notifHistoryRowProps are the props for one archived-notification row. The row
// is its own component so its optional navigation On* hook sits at a stable
// render depth (never registered inside the parent's variable-length loop —
// CLAUDE.md "CRITICAL gotchas").
type notifHistoryRowProps struct {
	Rec     notifyhistory.Record
	TimeStr string
}

// notifHistoryRow renders one archived notification: a severity dot, the message,
// and a "severity · time" foot. When the record carries a route it becomes a
// button that navigates to the alerting resource.
func notifHistoryRow(props notifHistoryRowProps) ui.Node {
	rec := props.Rec
	sev := rec.Severity
	if sev == "" {
		sev = "info"
	}

	route := rec.Route
	nav := router.UseNavigate()
	goResource := ui.UseEvent(Prevent(func() {
		if route != "" {
			nav.Navigate(uistate.RoutePath(route))
		}
	}))

	rowCls := "nhx-row"
	if rec.Read {
		rowCls += " is-read"
	} else {
		rowCls += " is-unread"
	}

	rowArgs := []any{ClassStr(rowCls), Attr("role", "listitem"), Attr("data-testid", "notif-history-row-"+rec.ID)}
	if route != "" {
		rowArgs = append(rowArgs,
			Attr("role", "button"), Attr("tabindex", "0"),
			Attr("aria-label", uistate.T("notifications.openResource", rec.Message)),
			OnClick(goResource))
	}
	rowArgs = append(rowArgs,
		Span(ClassStr("nhx-dot "+notifySeverityClass(sev)), Attr("aria-hidden", "true")),
		Div(css.Class("nhx-body"),
			Div(css.Class("nhx-msg"), rec.Message),
			Div(css.Class("nhx-foot"),
				Span(css.Class("nhx-sev-tag"), notifySeverityLabel(sev)),
				Span(css.Class("nhx-sep"), "·"),
				Span(css.Class("nhx-time"), props.TimeStr),
			),
		),
	)
	return Div(rowArgs...)
}

// notificationHistoryView is the persisted archive surface: a search box, a
// severity filter, a "Clear history" action, and the archived rows (each its own
// component), with a friendly empty state. It reads the archive through the
// uistate seam, so it never touches the KV store or the pure package directly.
func notificationHistoryView(props notifHistoryProps) ui.Node {
	_ = props
	// Re-render when the archive (or any shared data) changes.
	_ = uistate.UseDataRevision().Get()

	search := ui.UseState("")
	sev := ui.UseState("")

	onSearch := ui.UseEvent(func(v string) { search.Set(v) })
	onSev := ui.UseEvent(func(e ui.Event) { sev.Set(e.GetValue()) })
	clearHistory := ui.UseEvent(Prevent(func() {
		uistate.ClearNotificationHistory()
		uistate.BumpDataRevision()
	}))

	q := search.Get()
	s := sev.Get()
	items := uistate.ArchiveItems(q, s)

	bar := Div(css.Class("nhx-bar"),
		Input(css.Class("nhx-search"), Type("search"), Attr("data-testid", "notif-history-search"),
			Placeholder(uistate.T("notifHistory.searchPlaceholder")), Attr("aria-label", uistate.T("notifHistory.searchAria")),
			Value(q), OnInput(onSearch)),
		Select(css.Class("nhx-select"), Attr("data-testid", "notif-history-filter"),
			Attr("aria-label", uistate.T("notifications.showLabel")), OnChange(onSev),
			Option(Value(""), SelectedIf(s == ""), uistate.T("notifHistory.filterAll")),
			Option(Value("critical"), SelectedIf(s == "critical"), notifySeverityLabel("critical")),
			Option(Value("warning"), SelectedIf(s == "warning"), notifySeverityLabel("warning")),
			Option(Value("info"), SelectedIf(s == "info"), notifySeverityLabel("info")),
		),
		// Clearing history is destructive, so it wears the same danger treatment as the
		// Needs-you tab's "Clear all" (notif-clear-danger), keeping nhx-clear's size and
		// placement (review #27).
		Button(css.Class("nhx-clear notif-clear-danger"), Type("button"), Attr("data-testid", "notif-history-clear"),
			Attr("aria-label", uistate.T("notifHistory.clearAria")), OnClick(clearHistory),
			Text(uistate.T("notifHistory.clear"))),
		Span(css.Class("nhx-count"), uistate.T("notifHistory.count", len(items))),
	)

	// Empty states: nothing archived at all vs. nothing matching the filters.
	if len(items) == 0 {
		title := uistate.T("notifHistory.empty")
		hint := uistate.T("notifHistory.emptyHint")
		if q != "" || s != "" {
			title = uistate.T("notifHistory.noMatch")
			hint = ""
		}
		empty := Div(css.Class("nhx-empty"),
			Span(css.Class("nhx-empty-title"), title),
			If(hint != "", Span(css.Class("nhx-empty-hint"), hint)),
		)
		return Div(bar, empty)
	}

	pr := uistate.UsePrefs().Get()
	now := time.Now().Unix()
	rows := make([]ui.Node, 0, len(items))
	for _, rec := range items {
		timeStr := relativeTime(rec.At, now)
		if timeStr == "" {
			timeStr = pr.FormatDate(time.Unix(rec.At, 0))
		}
		rows = append(rows, ui.CreateElement(notifHistoryRow, notifHistoryRowProps{Rec: rec, TimeStr: timeStr}))
	}
	listArgs := []any{css.Class("nhx-list"), Attr("role", "list")}
	for _, r := range rows {
		listArgs = append(listArgs, r)
	}
	return Div(bar, Div(listArgs...))
}
