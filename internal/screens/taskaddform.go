// SPDX-License-Identifier: MIT

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
	dueStr := ui.UseState("")
	notes := ui.UseState("")
	errMsg := ui.UseState("")
	addLinkType := ui.UseState(string(domain.RelatedNone))
	addLinkID := ui.UseState("")
	addRecur := ui.UseState("")

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
			Status: domain.StatusOpen, Priority: domain.TaskPriority(priority.Get()), Due: due, Source: domain.SourceManual,
			RelatedType: rt, RelatedID: rid,
			Recurrence: domain.RecurringCadence(addRecur.Get()),
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
		errMsg.Set("")
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

	// Live summary shown in the footer — reads back the slip as you build it
	// ("High priority · due Jul 1 · Weekly · Links to").
	plabel, _ := priorityMeta(domain.TaskPriority(priority.Get()))
	sum := []string{plabel + " " + strings.ToLower(uistate.T("priority.label"))}
	if ds := strings.TrimSpace(dueStr.Get()); ds != "" {
		if d, derr := dateutil.ParseDate(ds); derr == nil {
			sum = append(sum, uistate.T("todo.due")+" "+pr.FormatDate(d))
		}
	}
	if addRecur.Get() != "" {
		sum = append(sum, taskCadenceLabel(domain.RecurringCadence(addRecur.Get())))
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
				Div(css.Class("tc-rail-row"),
					Span(css.Class("tc-rail-label"), uistate.T("todo.repeat")),
					uiw.SelectInput(uiw.SelectInputProps{
						Options: cadenceSelectOptions(addRecur.Get()), Selected: addRecur.Get(),
						OnChange: func(v string) { addRecur.Set(v) }, AriaLabel: uistate.T("todo.repeat"), TestID: "task-add-repeat",
					}),
				),
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
