//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

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
	onPriority := ui.UseEvent(func(e ui.Event) { priority.Set(e.GetValue()) })
	onAddLinkType := ui.UseEvent(func(e ui.Event) {
		addLinkType.Set(e.GetValue())
		addLinkID.Set("") // reset entity selection when type changes
	})
	onAddLinkID := ui.UseEvent(func(e ui.Event) { addLinkID.Set(e.GetValue()) })
	onAddRecur := ui.UseEvent(func(e ui.Event) { addRecur.Set(e.GetValue()) })

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

	prioOptions := []ui.Node{
		Option(Value(string(domain.PriorityHigh)), SelectedIf(priority.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
		Option(Value(string(domain.PriorityMedium)), SelectedIf(priority.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
		Option(Value(string(domain.PriorityLow)), SelectedIf(priority.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
	}

	addLinkTypeOpts := linkTypeOptions(addLinkType.Get())
	curAddType := domain.RelatedType(addLinkType.Get())
	var addEntitySelect ui.Node
	if curAddType != domain.RelatedNone && curAddType != "" {
		addEntitySelect = labeledField(uistate.T("todo.linkEntity"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("todo.linkEntity")), OnChange(onAddLinkID),
				buildEntityOptions(curAddType, addLinkID.Get(), accounts, budgets, goals, txns)))
	}

	return Form(css.Class("form-grid"), Attr("data-testid", "task-add-form"), OnSubmit(add),
		Input(append([]any{css.Class("field field-wide"), Attr("id", "task-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("todo.titlePlaceholder")), Value(title.Get()), OnInput(onTitle)}, errAttrs("todo-err", errMsg.Get())...)...),
		labeledField("Priority",
			Select(css.Class("field"), Attr("aria-label", "Priority"), OnChange(onPriority), prioOptions)),
		labeledField("Due date",
			Input(css.Class("field"), Type("date"), Attr("aria-label", "Due date"), Value(dueStr.Get()), OnInput(onDue))),
		Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("todo.notesPlaceholder")), Value(notes.Get()), OnInput(onNotes)),
		labeledField(uistate.T("todo.repeat"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("todo.repeat")), Attr("data-testid", "task-add-repeat"), OnChange(onAddRecur),
				cadenceOptions(addRecur.Get()))),
		labeledField(uistate.T("todo.linkTo"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("todo.linkTo")), OnChange(onAddLinkType), addLinkTypeOpts)),
		addEntitySelect,
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		errText("todo-err", errMsg.Get()),
	)
}
