//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
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

	accounts := app.Accounts()
	budgets := app.Budgets()
	goals := app.Goals()
	txns := app.Transactions()

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
	// onPriority/onAddLinkType/onAddLinkID/onAddRecur hook slots kept for stable hook
	// ordering; SelectInput owns the change event internally.
	ui.UseEvent(func(e ui.Event) { priority.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) {
		addLinkType.Set(e.GetValue())
		addLinkID.Set("")
	})
	ui.UseEvent(func(e ui.Event) { addLinkID.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { addRecur.Set(e.GetValue()) })

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
		// Reset fields.
		title.Set("")
		dueStr.Set("")
		notes.Set("")
		priority.Set(string(domain.PriorityMedium))
		addLinkType.Set(string(domain.RelatedNone))
		addLinkID.Set("")
		addRecur.Set("")
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	prioOpts := []uiw.SelectOption{
		{Value: string(domain.PriorityHigh), Label: uistate.T("priority.high")},
		{Value: string(domain.PriorityMedium), Label: uistate.T("priority.medium")},
		{Value: string(domain.PriorityLow), Label: uistate.T("priority.low")},
	}

	curAddType := domain.RelatedType(addLinkType.Get())
	var addEntitySelect ui.Node
	if curAddType != domain.RelatedNone && curAddType != "" {
		entityOpts := buildEntitySelectOptions(curAddType, addLinkID.Get(), accounts, budgets, goals, txns)
		addEntitySelect = uiw.FormField(uistate.T("todo.linkEntity"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   entityOpts,
				Selected:  addLinkID.Get(),
				OnChange:  func(v string) { addLinkID.Set(v) },
				AriaLabel: uistate.T("todo.linkEntity"),
			}))
	}

	return Form(css.Class("form-grid"), Attr("data-testid", "task-add-form"), OnSubmit(add),
		Input(append([]any{css.Class("field field-wide"), Attr("id", "task-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("todo.titlePlaceholder")), Value(title.Get()), OnInput(onTitle)}, errAttrs("todo-err", errMsg.Get())...)...),
		uiw.FormField("Priority",
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   prioOpts,
				Selected:  priority.Get(),
				OnChange:  func(v string) { priority.Set(v) },
				AriaLabel: "Priority",
			})),
		uiw.FormField("Due date",
			Input(css.Class("field"), Type("date"), Attr("aria-label", "Due date"), Value(dueStr.Get()), OnInput(onDue))),
		Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("todo.notesPlaceholder")), Value(notes.Get()), OnInput(onNotes)),
		uiw.FormField(uistate.T("todo.repeat"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   cadenceSelectOptions(addRecur.Get()),
				Selected:  addRecur.Get(),
				OnChange:  func(v string) { addRecur.Set(v) },
				AriaLabel: uistate.T("todo.repeat"),
				TestID:    "task-add-repeat",
			})),
		uiw.FormField(uistate.T("todo.linkTo"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   linkTypeSelectOptions(addLinkType.Get()),
				Selected:  addLinkType.Get(),
				OnChange:  func(v string) { addLinkType.Set(v); addLinkID.Set("") },
				AriaLabel: uistate.T("todo.linkTo"),
			})),
		addEntitySelect,
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		errText("todo-err", errMsg.Get()),
	)
}
