// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// cadenceSelectOptions converts the cadence constants to []uiw.SelectOption for
// use with uiw.SelectInput.
func cadenceSelectOptions(selected string) []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("todo.repeatNone")},
		{Value: string(domain.CadenceWeekly), Label: uistate.T("todo.repeatWeekly")},
		{Value: string(domain.CadenceMonthly), Label: uistate.T("todo.repeatMonthly")},
		{Value: string(domain.CadenceQuarterly), Label: uistate.T("todo.repeatQuarterly")},
		{Value: string(domain.CadenceYearly), Label: uistate.T("todo.repeatYearly")},
	}
}

// reminderLeadSelectOptions returns the fixed "Remind me" lead choices for a
// recurring task, mapping a days-before-due value (as a string) to a plain label.
// 0 = on the due date; the positive values open the reminder window early.
func reminderLeadSelectOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "0", Label: uistate.T("todo.remindOnDue")},
		{Value: "1", Label: uistate.T("todo.remind1Day")},
		{Value: "3", Label: uistate.T("todo.remind3Days")},
		{Value: "7", Label: uistate.T("todo.remind1Week")},
	}
}

// linkTypeSelectOptions converts link-type constants to []uiw.SelectOption.
func linkTypeSelectOptions(selected string) []uiw.SelectOption {
	none := string(domain.RelatedNone)
	return []uiw.SelectOption{
		{Value: none, Label: uistate.T("todo.linkNone")},
		{Value: string(domain.RelatedAccount), Label: uistate.T("todo.linkAccount")},
		{Value: string(domain.RelatedBudget), Label: uistate.T("todo.linkBudget")},
		{Value: string(domain.RelatedGoal), Label: uistate.T("todo.linkGoal")},
		{Value: string(domain.RelatedTransaction), Label: uistate.T("todo.linkTransaction")},
	}
}

// buildEntitySelectOptions mirrors buildEntityOptions but returns []uiw.SelectOption
// for use with uiw.SelectInput.
func buildEntitySelectOptions(
	rt domain.RelatedType,
	selectedID string,
	accounts []domain.Account,
	budgets []domain.Budget,
	goals []domain.Goal,
	txns []domain.Transaction,
) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("todo.linkEntity")}}
	switch rt {
	case domain.RelatedAccount:
		for _, a := range accounts {
			opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
		}
	case domain.RelatedBudget:
		for _, b := range budgets {
			opts = append(opts, uiw.SelectOption{Value: b.ID, Label: b.Name})
		}
	case domain.RelatedGoal:
		for _, g := range goals {
			opts = append(opts, uiw.SelectOption{Value: g.ID, Label: g.Name})
		}
	case domain.RelatedTransaction:
		for _, tx := range txns {
			label := tx.Payee
			if label == "" {
				label = tx.Desc
			}
			opts = append(opts, uiw.SelectOption{Value: tx.ID, Label: label})
		}
	}
	return opts
}

// TaskAddFormProps configures the TaskAddForm component.
type TaskAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
	// ParentID, when set, makes the created task a sub-task of that parent — so the
	// full compose form (not a bare prompt) can create a nested to-do.
	ParentID string
	// PresetDue, when set (ISO yyyy-mm-dd), seeds the Due date field — used by the
	// To-do calendar view so clicking a day starts a task already scheduled for it.
	PresetDue string
}

// TaskAddForm is the standalone add-a-task form. It owns all its state and
// handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Todo() for use in the AddHost modal.
func TaskAddForm(props TaskAddFormProps) ui.Node {
	return ui.CreateElement(taskAddForm, props)
}

func taskAddForm(props TaskAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	accounts := app.Accounts()
	budgets := app.Budgets()
	goals := app.Goals()
	txns := app.Transactions()

	pr := uistate.UsePrefs().Get()
	title := ui.UseState("")
	priority := ui.UseState(string(domain.PriorityMedium))
	dueStr := ui.UseState(props.PresetDue)
	notes := ui.UseState("")
	errMsg := ui.UseState("")
	addLinkType := ui.UseState(string(domain.RelatedNone))
	addLinkID := ui.UseState("")
	addRecur := ui.UseState("")
	addRemind := ui.UseState("0") // ReminderLeadDays as a string; only meaningful when recurring

	onTitle := ui.UseEvent(func(v string) { title.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueStr.Set(v) })
	onNotes := ui.UseEvent(func(v string) { notes.Set(v) })
	// Priority segments (echo the list's check-ring colours). Fixed count, so these
	// hooks sit at stable positions.
	setLow := ui.UseEvent(Prevent(func() { priority.Set(string(domain.PriorityLow)) }))
	setMed := ui.UseEvent(Prevent(func() { priority.Set(string(domain.PriorityMedium)) }))
	setHigh := ui.UseEvent(Prevent(func() { priority.Set(string(domain.PriorityHigh)) }))
	// Quick-date chips.
	quickToday := ui.UseEvent(Prevent(func() { dueStr.Set(dateutil.FormatDate(time.Now())) }))
	quickWeek := ui.UseEvent(Prevent(func() { dueStr.Set(dateutil.FormatDate(time.Now().AddDate(0, 0, 7))) }))
	quickClear := ui.UseEvent(Prevent(func() { dueStr.Set("") }))
	cancel := ui.UseEvent(Prevent(func() { done() }))

	add := ui.UseEvent(Prevent(func() {
		if strings.TrimSpace(title.Get()) == "" {
			errMsg.Set(uistate.T("todo.titleRequired"))
			return
		}
		var due time.Time
		if ds := strings.TrimSpace(dueStr.Get()); ds != "" {
			d, err := dateutil.ParseDate(ds)
			if err != nil {
				errMsg.Set(uistate.T("todo.invalidDue"))
				return
			}
			due = d
		}
		rt := domain.RelatedType(addLinkType.Get())
		rid := addLinkID.Get()
		if rt == domain.RelatedNone || rt == "" {
			rid = ""
		}
		t := domain.Task{
			ID: id.New(), Title: strings.TrimSpace(title.Get()), Notes: strings.TrimSpace(notes.Get()),
			ParentID: props.ParentID,
			Status:   domain.StatusOpen, Priority: domain.TaskPriority(priority.Get()), Due: due, Source: domain.SourceManual,
			RelatedType: rt, RelatedID: rid,
		}
		// Repeat + reminder are anchored to a due date, so they only take effect when one
		// is set (the UI hides both controls otherwise). A reminder applies to any dated
		// task; a recurrence advances the due date.
		if !due.IsZero() {
			t.Recurrence = domain.RecurringCadence(addRecur.Get())
			if n, perr := strconv.Atoi(addRemind.Get()); perr == nil {
				t.ReminderLeadDays = n
			}
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		title.Set("")
		dueStr.Set("")
		notes.Set("")
		priority.Set(string(domain.PriorityMedium))
		addLinkType.Set(string(domain.RelatedNone))
		addLinkID.Set("")
		addRecur.Set("")
		addRemind.Set("0")
		errMsg.Set("")
		// The add form is a shell-root sibling (AddHost), so the /todo list won't
		// re-render on its own — bump the shared data revision (and confirm) so the
		// new task appears immediately, matching every other add form.
		uistate.BumpDataRevision()
		uistate.ResetTodoPage() // return to the top page so the new task is on-screen
		uistate.PostNotice(uistate.T("todo.taskAdded"), false)
		done()
	}))

	// Priority segmented control — three tinted pills that mirror the to-do list's
	// check-ring colours (faint low / accent medium / red high).
	seg := func(val, label string, on ui.Handler) ui.Node {
		cls := "task-seg-btn p-" + val
		if priority.Get() == val {
			cls += " is-active"
		}
		return Button(ClassStr(cls), Type("button"), Attr("role", "radio"), Attr("aria-checked", ariaBool(priority.Get() == val)),
			Attr("data-testid", "task-prio-"+val), OnClick(on),
			Span(css.Class("task-seg-dot")), Span(label))
	}

	curAddType := domain.RelatedType(addLinkType.Get())
	var addEntitySelect ui.Node = Fragment()
	if curAddType != domain.RelatedNone && curAddType != "" {
		entityOpts := buildEntitySelectOptions(curAddType, addLinkID.Get(), accounts, budgets, goals, txns)
		addEntitySelect = uiw.SelectInput(uiw.SelectInputProps{
			Options: entityOpts, Selected: addLinkID.Get(),
			OnChange: func(v string) { addLinkID.Set(v) }, AriaLabel: uistate.T("todo.linkEntity"),
		})
	}

	// The reminder-lead control only appears once the task is set to repeat — a
	// one-shot has nothing to remind against beyond its own due date.
	// Repeat + Remind me are both anchored to a DUE DATE (a reminder fires N days
	// before it; a recurrence advances it), so they only appear once a due date is
	// set — and a reminder works for any dated task, recurring or not.
	hasDue := strings.TrimSpace(dueStr.Get()) != ""
	var remindRow, repeatRow ui.Node = Fragment(), Fragment()
	if hasDue {
		remindRow = Div(css.Class("tc-rail-row"),
			Span(css.Class("tc-rail-label"), uistate.T("todo.remind")),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: reminderLeadSelectOptions(), Selected: addRemind.Get(),
				OnChange: func(v string) { addRemind.Set(v) }, AriaLabel: uistate.T("todo.remind"), TestID: "task-add-remind",
			}),
		)
		repeatRow = Div(css.Class("tc-rail-row"),
			Span(css.Class("tc-rail-label"), uistate.T("todo.repeat")),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: cadenceSelectOptions(addRecur.Get()), Selected: addRecur.Get(),
				OnChange: func(v string) { addRecur.Set(v) }, AriaLabel: uistate.T("todo.repeat"), TestID: "task-add-repeat",
			}),
		)
	}

	// Live summary shown in the footer — reads back the slip as you build it
	// ("High priority · due Jul 1 · Weekly · Links to").
	// Kept compact so the whole slip fits the modal footer without truncating to a
	// dangling "· …": the priority word alone (the coloured spine already signals it)
	// and a bare date (no "due" prefix).
	plabel, _ := priorityMeta(domain.TaskPriority(priority.Get()))
	sum := []string{plabel}
	if ds := strings.TrimSpace(dueStr.Get()); ds != "" {
		if d, derr := dateutil.ParseDate(ds); derr == nil {
			sum = append(sum, pr.FormatDate(d))
		}
	}
	if hasDue {
		if addRecur.Get() != "" {
			sum = append(sum, taskCadenceLabel(domain.RecurringCadence(addRecur.Get())))
		}
		if n, perr := strconv.Atoi(addRemind.Get()); perr == nil && n > 0 {
			sum = append(sum, taskReminderLabel(n))
		}
	}
	if curAddType != domain.RelatedNone && curAddType != "" {
		sum = append(sum, uistate.T("todo.linkTo"))
	}
	summary := strings.Join(sum, " · ")

	return Form(css.Class("tc"), Attr("data-testid", "task-add-form"), OnSubmit(add),
		Div(css.Class("tc-main"),
			// LEFT — the writing zone. A live priority "spine" (the coloured left edge)
			// glows faint / green / red as you pick priority: the slip is tinted by its urgency.
			Div(ClassStr("tc-write p-"+string(priority.Get())),
				Input(append([]any{css.Class("tc-title"), Attr("id", "task-add"), Attr("autofocus", ""), Type("text"), Attr("aria-required", "true"), Attr("aria-label", uistate.T("todo.titleLabel")), Placeholder(uistate.T("todo.titlePlaceholder")), Value(title.Get()), OnInput(onTitle)}, errAttrs("todo-err", errMsg.Get())...)...),
				errText("todo-err", errMsg.Get()),
				Textarea(css.Class("tc-notes"), Attr("aria-label", uistate.T("todo.notesEdit")), Placeholder(uistate.T("todo.notesCompose")), OnInput(onNotes), notes.Get()),
			),
			// RIGHT — the details rail (an inspector, not a labelled-field stack).
			Div(css.Class("tc-rail"),
				H3(css.Class("tc-rail-head"), uistate.T("todo.detailsHead")),
				Div(css.Class("tc-rail-row"),
					Span(css.Class("tc-rail-label"), uistate.T("priority.label")),
					Div(css.Class("task-seg", "is-rail"), Attr("role", "radiogroup"), Attr("aria-label", uistate.T("priority.label")),
						seg(string(domain.PriorityLow), uistate.T("priority.low"), setLow),
						seg(string(domain.PriorityMedium), uistate.T("priority.medium"), setMed),
						seg(string(domain.PriorityHigh), uistate.T("priority.high"), setHigh),
					),
				),
				Div(css.Class("tc-rail-row"),
					Span(css.Class("tc-rail-label"), uistate.T("common.dueDate")),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("common.dueDate")), Value(dueStr.Get()), OnInput(onDue)),
					Div(css.Class("task-quick"),
						Button(css.Class("task-quick-chip"), Type("button"), Attr("data-testid", "task-quick-today"), OnClick(quickToday), uistate.T("todo.quickToday")),
						Button(css.Class("task-quick-chip"), Type("button"), Attr("data-testid", "task-quick-week"), OnClick(quickWeek), uistate.T("todo.quickWeek")),
						Button(css.Class("task-quick-chip", "is-clear"), Type("button"), Attr("data-testid", "task-quick-clear"), OnClick(quickClear), uistate.T("todo.quickClear")),
					),
				),
				// Remind me sits directly under Due date (it's anchored to it), then Repeat.
				remindRow,
				repeatRow,
				Div(css.Class("tc-rail-row"),
					Span(css.Class("tc-rail-label"), uistate.T("todo.linkTo")),
					uiw.SelectInput(uiw.SelectInputProps{
						Options: linkTypeSelectOptions(addLinkType.Get()), Selected: addLinkType.Get(),
						OnChange: func(v string) { addLinkType.Set(v); addLinkID.Set("") }, AriaLabel: uistate.T("todo.linkTo"),
					}),
					addEntitySelect,
				),
			),
		),
		// Footer — a live summary of the slip + the actions.
		Div(css.Class("tc-foot"),
			Span(css.Class("tc-summary"), Attr("data-testid", "task-summary"), summary),
			Div(css.Class("tc-foot-actions"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("submit"), Attr("data-testid", "task-add-submit"),
					uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("todo.addTask"))),
			),
		),
	)
}
