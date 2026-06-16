//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Todo is the budgeting task list: add, complete/reopen, delete.
func Todo() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:tasks", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	title := ui.UseState("")
	priority := ui.UseState(string(domain.PriorityMedium))
	dueStr := ui.UseState("")
	notes := ui.UseState("")
	hideDone := ui.UseState(false)
	errMsg := ui.UseState("")

	onTitle := ui.UseEvent(func(v string) { title.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueStr.Set(v) })
	onNotes := ui.UseEvent(func(v string) { notes.Set(v) })
	onPriority := ui.UseEvent(func(e ui.Event) { priority.Set(e.GetValue()) })
	toggleHideDone := ui.UseEvent(func() { hideDone.Set(!hideDone.Get()) })

	add := ui.UseEvent(Prevent(func() {
		var due time.Time
		if ds := strings.TrimSpace(dueStr.Get()); ds != "" {
			d, err := dateutil.ParseDate(ds)
			if err != nil {
				errMsg.Set("Enter a valid due date (YYYY-MM-DD).")
				return
			}
			due = d
		}
		t := domain.Task{
			ID: id.New(), Title: strings.TrimSpace(title.Get()), Notes: strings.TrimSpace(notes.Get()),
			Status: domain.StatusOpen, Priority: domain.TaskPriority(priority.Get()), Due: due, Source: domain.SourceManual,
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		title.Set("")
		dueStr.Set("")
		notes.Set("")
		errMsg.Set("")
		bump()
	}))

	tasks := app.Tasks()
	byID := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	toggleTask := func(taskID string) {
		t, ok := byID[taskID]
		if !ok {
			return
		}
		if t.Status == domain.StatusDone {
			t.Status = domain.StatusOpen
		} else {
			t.Status = domain.StatusDone
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}
	deleteTask := func(taskID string) {
		if err := app.DeleteTask(taskID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	prioOptions := []ui.Node{
		Option(Value(string(domain.PriorityHigh)), SelectedIf(priority.Get() == string(domain.PriorityHigh)), "High"),
		Option(Value(string(domain.PriorityMedium)), SelectedIf(priority.Get() == string(domain.PriorityMedium)), "Medium"),
		Option(Value(string(domain.PriorityLow)), SelectedIf(priority.Get() == string(domain.PriorityLow)), "Low"),
	}

	form := Section(Class("card"),
		H2(Class("card-title"), "Add task"),
		Form(Class("form-grid"), OnSubmit(add),
			Input(Class("field field-wide"), Type("text"), Placeholder("What needs doing?"), Value(title.Get()), OnInput(onTitle)),
			Select(Class("field"), OnChange(onPriority), prioOptions),
			Input(Class("field"), Type("date"), Value(dueStr.Get()), OnInput(onDue)),
			Input(Class("field field-wide"), Type("text"), Placeholder("Notes (optional)"), Value(notes.Get()), OnInput(onNotes)),
			Button(Class("btn btn-primary"), Type("submit"), "Add"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
	)

	// Open tasks first, then by due date (dated before undated), then title.
	sort.Slice(tasks, func(i, j int) bool {
		ti, tj := tasks[i], tasks[j]
		if (ti.Status == domain.StatusOpen) != (tj.Status == domain.StatusOpen) {
			return ti.Status == domain.StatusOpen
		}
		if ti.Due.IsZero() != tj.Due.IsZero() {
			return !ti.Due.IsZero()
		}
		if !ti.Due.Equal(tj.Due) {
			return ti.Due.Before(tj.Due)
		}
		return ti.Title < tj.Title
	})

	shown := tasks
	if hideDone.Get() {
		shown = make([]domain.Task, 0, len(tasks))
		for _, t := range tasks {
			if t.Status != domain.StatusDone {
				shown = append(shown, t)
			}
		}
	}

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = P(Class("empty"), "No tasks yet.")
	case len(shown) == 0:
		listBody = P(Class("empty"), "All done 🎉")
	default:
		rows := MapKeyed(shown,
			func(t domain.Task) any { return t.ID },
			func(t domain.Task) ui.Node {
				return ui.CreateElement(TaskRow, taskRowProps{Task: t, OnToggle: toggleTask, OnDelete: deleteTask})
			},
		)
		listBody = Div(Class("rows"), rows)
	}

	hideLabel := "Hide done"
	if hideDone.Get() {
		hideLabel = "Show all"
	}

	return Div(
		form,
		Section(Class("card"),
			Div(Class("budget-head"),
				H2(Class("card-title"), "Tasks"),
				Button(Class("btn"), Type("button"), OnClick(toggleHideDone), hideLabel),
			),
			listBody,
		),
	)
}

type taskRowProps struct {
	Task     domain.Task
	OnToggle func(string)
	OnDelete func(string)
}

// TaskRow renders one task with a complete toggle and delete.
func TaskRow(props taskRowProps) ui.Node {
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(props.Task.ID) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Task.ID) }))

	t := props.Task
	done := t.Status == domain.StatusDone
	rowClass := "row"
	glyph := "☐"
	if done {
		rowClass = "row done"
		glyph = "☑"
	}
	plabel, pclass := priorityMeta(t.Priority)

	meta := []ui.Node{Span(Class("badge badge-prio "+pclass), plabel)}
	if !t.Due.IsZero() {
		meta = append(meta, Span(Class("row-meta"), "due "+dateutil.FormatDate(t.Due)))
	}
	if t.Notes != "" {
		meta = append(meta, Span(Class("row-meta"), t.Notes))
	}

	return Div(Class(rowClass),
		Button(Class("check"), Type("button"), Title("Toggle complete"), OnClick(toggle), glyph),
		Div(Class("row-main"),
			Span(Class("row-desc"), t.Title),
			Div(Class("task-meta"), meta),
		),
		Button(Class("btn-del"), Type("button"), Title("Delete task"), OnClick(del), "✕"),
	)
}

func priorityMeta(p domain.TaskPriority) (label, class string) {
	switch p {
	case domain.PriorityHigh:
		return "High", "prio-high"
	case domain.PriorityLow:
		return "Low", "prio-low"
	default:
		return "Medium", "prio-med"
	}
}
