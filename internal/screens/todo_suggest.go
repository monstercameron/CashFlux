// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/tasksuggest"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// suggestDismissKey is the KV slot holding dismissed suggestion keys → unix
// seconds. A dismissal suppresses that suggestion for suggestSnoozeDays, then
// the condition (if still unresolved) may propose again — self-healing, never
// forever-silent.
const suggestDismissKey = "cashflux:tasksuggest:dismissed"

const suggestSnoozeDays = 30

func loadSuggestDismissals() map[string]int64 {
	out := map[string]int64{}
	if raw := uistate.KVGet(suggestDismissKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

func saveSuggestDismissal(key string, at time.Time) {
	m := loadSuggestDismissals()
	m[key] = at.Unix()
	if b, err := json.Marshal(m); err == nil {
		uistate.KVSet(suggestDismissKey, string(b))
	}
}

// suggestTitle renders a suggestion's localized task title.
func suggestTitle(s tasksuggest.Suggestion) string {
	switch s.Kind {
	case tasksuggest.KindStaleAccount:
		return uistate.T("todo.suggestStale", s.Name)
	case tasksuggest.KindUnreviewed:
		return uistate.T("todo.suggestUnreviewed", s.Count)
	default:
		return uistate.T("todo.suggestOverspent", s.Name)
	}
}

// todoSuggestStrip is the "Suggested" section above the task list: deterministic
// condition scans (tasksuggest.Scan) proposed as one-click tasks — never created
// silently. Add posts the task (with its self-resolve condition where one
// exists); Dismiss snoozes the proposal for a month.
func todoSuggestStrip(app *appstate.App) ui.Node {
	_ = uistate.UseDataRevision().Get()
	if app == nil {
		return Fragment()
	}
	s := app.Settings()
	rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
	now := time.Now()
	suggestions := tasksuggest.Scan(app.Accounts(), app.Transactions(), app.Budgets(),
		app.FreshnessWindows(), rates, now, uistate.LoadPrefs().WeekStartWeekday())

	// Drop dismissed-within-snooze and already-open proposals (an open task
	// linked to the same entity from a prior Add).
	dismissed := loadSuggestDismissals()
	open := map[string]bool{}
	for _, t := range app.Tasks() {
		if t.Status == domain.StatusOpen && t.RelatedID != "" {
			open[string(t.RelatedType)+":"+t.RelatedID] = true
		}
	}
	kept := suggestions[:0:0]
	for _, sg := range suggestions {
		if at, ok := dismissed[sg.Key]; ok && now.Sub(time.Unix(at, 0)) < suggestSnoozeDays*24*time.Hour {
			continue
		}
		if sg.RelatedID != "" && open[string(sg.RelatedType)+":"+sg.RelatedID] {
			continue
		}
		kept = append(kept, sg)
	}
	if len(kept) == 0 {
		return Fragment()
	}

	rows := make([]ui.Node, 0, len(kept))
	for _, sg := range kept {
		rows = append(rows, ui.CreateElement(todoSuggestRow, todoSuggestRowProps{S: sg, App: app}))
	}
	return Div(css.Class("todo-suggest"), Attr("data-testid", "todo-suggest-strip"),
		Style(map[string]string{"margin-bottom": "0.75rem", "padding": "0.6rem 0.75rem",
			"border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.4rem"}),
			uistate.T("todo.suggestHeading")),
		Div(Style(map[string]string{"display": "grid", "gap": "0.35rem"}), rows),
	)
}

type todoSuggestRowProps struct {
	S   tasksuggest.Suggestion
	App *appstate.App
}

// todoSuggestRow is one proposal line with its Add / Dismiss actions. Its own
// component so the click hooks sit at stable positions per row.
func todoSuggestRow(props todoSuggestRowProps) ui.Node {
	sg := props.S
	title := suggestTitle(sg)
	add := ui.UseEvent(Prevent(func() {
		app := props.App
		if app == nil {
			return
		}
		task := domain.Task{
			ID: id.New(), Title: title, Status: domain.StatusOpen,
			Priority: domain.PriorityMedium, Source: domain.SourceNudge,
			Due:         time.Now().AddDate(0, 0, sg.DueDays),
			RelatedType: sg.RelatedType, RelatedID: sg.RelatedID,
			Resolve: sg.Resolve,
		}
		if err := app.PutTask(task); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		saveSuggestDismissal(sg.Key, time.Now())
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("todo.suggestAdded", title))
	}))
	dismiss := ui.UseEvent(Prevent(func() {
		saveSuggestDismissal(sg.Key, time.Now())
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("todo.suggestDismissed"), false)
	}))
	return Div(Attr("data-testid", "todo-suggest-"+sg.Key),
		Style(map[string]string{"display": "flex", "gap": "0.6rem", "align-items": "center", "flex-wrap": "wrap"}),
		uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(Style(map[string]string{"flex": "1 1 auto"}), title),
		Button(css.Class("btn", "btn-sm", "btn-primary"), Type("button"),
			Attr("data-testid", "todo-suggest-add-"+sg.Key), OnClick(add), uistate.T("todo.suggestAdd")),
		Button(css.Class("btn", "btn-sm"), Type("button"),
			Attr("data-testid", "todo-suggest-dismiss-"+sg.Key), OnClick(dismiss), uistate.T("todo.suggestDismiss")),
	)
}
