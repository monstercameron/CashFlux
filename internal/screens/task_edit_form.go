// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// TaskEditFormProps drives the task editor rendered inside the shell-root flip modal
// (see internal/app TaskEditHost).
type TaskEditFormProps struct {
	TaskID string
	OnDone func() // clears the atom (closes the modal)
}

// TaskEditForm renders the full task editor (title / priority / due / notes / repeat /
// linked entity) as the body of the shell-root flip modal. It owns all its state and its
// own Save/Cancel and mutates the store directly, mirroring GoalEditForm. Living at the
// shell root keeps the modal centred (a task row lives under transformed tile ancestors).
func TaskEditForm(props TaskEditFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var t domain.Task
	found := false
	if app != nil {
		for _, tt := range app.Tasks() {
			if tt.ID == props.TaskID {
				t, found = tt, true
				break
			}
		}
	}
	dueISO := ""
	if found && !t.Due.IsZero() {
		dueISO = dateutil.FormatDate(t.Due)
	}

	// All hooks unconditionally at stable positions.
	titleS := ui.UseState(t.Title)
	prioS := ui.UseState(string(t.Priority))
	dueS := ui.UseState(dueISO)
	notesS := ui.UseState(t.Notes)
	linkTypeS := ui.UseState(string(t.RelatedType))
	linkIDS := ui.UseState(t.RelatedID)
	recurS := ui.UseState(string(t.Recurrence))
	errS := ui.UseState("")

	onTitle := ui.UseEvent(func(v string) { titleS.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onNotes := ui.UseEvent(func(v string) { notesS.Set(v) })

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		for _, tt := range app.Tasks() {
			if tt.ID != props.TaskID {
				continue
			}
			if n := strings.TrimSpace(titleS.Get()); n != "" {
				tt.Title = n
			}
			if p := domain.TaskPriority(prioS.Get()); p.Valid() {
				tt.Priority = p
			}
			if ds := strings.TrimSpace(dueS.Get()); ds != "" {
				d, derr := dateutil.ParseDate(ds)
				if derr != nil {
					errS.Set(uistate.T("todo.invalidDue"))
					return
				}
				tt.Due = d
			} else {
				tt.Due = time.Time{}
			}
			tt.Notes = strings.TrimSpace(notesS.Get())
			rt := domain.RelatedType(linkTypeS.Get())
			if rt == domain.RelatedNone || rt == "" {
				tt.RelatedType = domain.RelatedNone
				tt.RelatedID = ""
			} else {
				tt.RelatedType = rt
				tt.RelatedID = linkIDS.Get()
			}
			tt.Recurrence = domain.RecurringCadence(recurS.Get())
			if err := app.PutTask(tt); err != nil {
				errS.Set(err.Error())
				return
			}
			break
		}
		uistate.BumpDataRevision()
		done()
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	prioOpts := []uiw.SelectOption{
		{Value: string(domain.PriorityHigh), Label: uistate.T("priority.high")},
		{Value: string(domain.PriorityMedium), Label: uistate.T("priority.medium")},
		{Value: string(domain.PriorityLow), Label: uistate.T("priority.low")},
	}

	// The entity sub-picker only appears once a link type other than "none" is chosen.
	curType := domain.RelatedType(linkTypeS.Get())
	var entitySelect ui.Node = Fragment()
	if curType != domain.RelatedNone && curType != "" {
		entityOpts := buildEntitySelectOptions(curType, linkIDS.Get(), app.Accounts(), app.Budgets(), app.Goals(), app.Transactions())
		entitySelect = labeledField(uistate.T("todo.linkEntity"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: entityOpts, Selected: linkIDS.Get(),
				OnChange: func(v string) { linkIDS.Set(v) }, AriaLabel: uistate.T("todo.linkEntity"),
			}))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	return Form(css.Class("acct-edit-form"), Attr("id", "task-edit-form"), OnSubmit(save),
		labeledField(uistate.T("todo.taskLabel"),
			Input(css.Class("field"), Attr("id", "task-edit-"+t.ID), Attr("autofocus", ""), Type("text"),
				Placeholder(uistate.T("todo.taskPlaceholder")), Value(titleS.Get()), OnInput(onTitle))),
		labeledField(uistate.T("todo.priorityLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: prioOpts, Selected: prioS.Get(),
				OnChange: func(v string) { prioS.Set(v) }, AriaLabel: "Priority",
			})),
		labeledField(uistate.T("common.dueDate"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("common.dueDate")), Value(dueS.Get()), OnInput(onDue))),
		labeledField(uistate.T("todo.notesEdit"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("todo.notesEdit")), Value(notesS.Get()), OnInput(onNotes))),
		labeledField(uistate.T("todo.repeat"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: cadenceSelectOptions(recurS.Get()), Selected: recurS.Get(),
				OnChange: func(v string) { recurS.Set(v) }, AriaLabel: uistate.T("todo.repeat"), TestID: "task-edit-repeat-" + t.ID,
			})),
		labeledField(uistate.T("todo.linkTo"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: linkTypeSelectOptions(linkTypeS.Get()), Selected: linkTypeS.Get(),
				OnChange: func(v string) { linkTypeS.Set(v); linkIDS.Set("") }, AriaLabel: uistate.T("todo.linkTo"),
			})),
		entitySelect,
		errLine,
	)
}
