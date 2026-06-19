//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Todo is the budgeting task list: add, complete/reopen, delete.
func Todo() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
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
				errMsg.Set(uistate.T("todo.invalidDue"))
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
	saveTask := func(taskID, newTitle, prio, dueStr, newNotes string) {
		t, ok := byID[taskID]
		if !ok {
			return
		}
		if n := strings.TrimSpace(newTitle); n != "" {
			t.Title = n
		}
		if p := domain.TaskPriority(prio); p.Valid() {
			t.Priority = p
		}
		if ds := strings.TrimSpace(dueStr); ds != "" {
			d, err := dateutil.ParseDate(ds)
			if err != nil {
				errMsg.Set(uistate.T("todo.invalidDue"))
				return
			}
			t.Due = d
		} else {
			t.Due = time.Time{}
		}
		t.Notes = strings.TrimSpace(newNotes)
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	prioOptions := []ui.Node{
		Option(Value(string(domain.PriorityHigh)), SelectedIf(priority.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
		Option(Value(string(domain.PriorityMedium)), SelectedIf(priority.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
		Option(Value(string(domain.PriorityLow)), SelectedIf(priority.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
	}

	form := Section(Class("card"),
		H2(Class("card-title"), uistate.T("todo.addTitle")),
		Form(Class("form-grid"), OnSubmit(add),
			Input(append([]any{Class("field field-wide"), Attr("id", "task-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("todo.titlePlaceholder")), Value(title.Get()), OnInput(onTitle)}, errAttrs("todo-err", errMsg.Get())...)...),
			Select(Class("field"), OnChange(onPriority), prioOptions),
			Input(Class("field"), Type("date"), Value(dueStr.Get()), OnInput(onDue)),
			Input(Class("field field-wide"), Type("text"), Placeholder(uistate.T("todo.notesPlaceholder")), Value(notes.Get()), OnInput(onNotes)),
			Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		),
		errText("todo-err", errMsg.Get()),
	)

	// Order + filter for display (open first, soonest due, then title) lives in
	// the pure, tested internal/tasksort package.
	tasks = tasksort.Order(tasks)
	shown := tasksort.Visible(tasks, hideDone.Get())

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("todo.empty"), CTALabel: uistate.T("todo.addFirst"), FocusID: "task-add"})
	case len(shown) == 0:
		listBody = P(Class("empty"), uistate.T("todo.allDone"))
	default:
		rows := MapKeyed(shown,
			func(t domain.Task) any { return t.ID },
			func(t domain.Task) ui.Node {
				return ui.CreateElement(TaskRow, taskRowProps{Task: t, OnToggle: toggleTask, OnDelete: deleteTask, OnSave: saveTask})
			},
		)
		listBody = Div(Class("rows"), rows)
	}

	hideLabel := uistate.T("todo.hideDone")
	if hideDone.Get() {
		hideLabel = uistate.T("todo.showAll")
	}

	return Div(
		form,
		Section(Class("card"),
			Div(Class("budget-head"),
				H2(Class("card-title"), uistate.T("todo.listTitle")),
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
	OnSave   func(id, title, priority, due, notes string)
}

// TaskRow renders one task with complete/edit/delete. It can be edited inline
// (title, priority, due, notes). All hooks are declared unconditionally so the
// edit toggle never reorders them.
func TaskRow(props taskRowProps) ui.Node {
	t := props.Task
	dueISO := ""
	if !t.Due.IsZero() {
		dueISO = dateutil.FormatDate(t.Due)
	}

	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(t.ID) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(t.ID) }))
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	titleS := ui.UseState(t.Title)
	prioS := ui.UseState(string(t.Priority))
	dueS := ui.UseState(dueISO)
	notesS := ui.UseState(t.Notes)
	onTitle := ui.UseEvent(func(v string) { titleS.Set(v) })
	onPrio := ui.UseEvent(func(e ui.Event) { prioS.Set(e.GetValue()) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onNotes := ui.UseEvent(func(v string) { notesS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		titleS.Set(t.Title)
		prioS.Set(string(t.Priority))
		dueS.Set(dueISO)
		notesS.Set(t.Notes)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(t.ID, titleS.Get(), prioS.Get(), dueS.Get(), notesS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("task-edit-" + t.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field field-wide"), Attr("id", "task-edit-"+t.ID), Type("text"), Placeholder(uistate.T("todo.taskPlaceholder")), Value(titleS.Get()), OnInput(onTitle)),
				Select(Class("field"), OnChange(onPrio),
					Option(Value(string(domain.PriorityHigh)), SelectedIf(prioS.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
					Option(Value(string(domain.PriorityMedium)), SelectedIf(prioS.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
					Option(Value(string(domain.PriorityLow)), SelectedIf(prioS.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
				),
				Input(Class("field"), Type("date"), Value(dueS.Get()), OnInput(onDue)),
				Input(Class("field field-wide"), Type("text"), Placeholder(uistate.T("todo.notesEdit")), Value(notesS.Get()), OnInput(onNotes)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

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
		meta = append(meta, Span(Class("row-meta"), uistate.T("todo.due")+" "+pr.FormatDate(t.Due)))
	}
	if t.Notes != "" {
		meta = append(meta, Span(Class("row-meta"), t.Notes))
	}

	return Div(Class(rowClass),
		Button(Class("check"), Type("button"), Title(uistate.T("todo.toggle")), OnClick(toggle), glyph),
		Div(Class("row-main"),
			Span(Class("row-desc"), t.Title),
			Div(Class("task-meta"), meta),
		),
		Button(Class("btn inline-flex items-center gap-1.5"), Type("button"), Title(uistate.T("todo.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, Class("w-4 h-4 shrink-0")), Span(uistate.T("action.edit"))),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("todo.deleteTitle")), Title(uistate.T("todo.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
	)
}

func priorityMeta(p domain.TaskPriority) (label, class string) {
	switch p {
	case domain.PriorityHigh:
		return uistate.T("priority.high"), "prio-high"
	case domain.PriorityLow:
		return uistate.T("priority.low"), "prio-low"
	default:
		return uistate.T("priority.medium"), "prio-med"
	}
}
