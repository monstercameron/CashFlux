//go:build js && wasm

package screens

import (
	"sort"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workflow"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Workflows is the automation manager: create automations (a trigger, an optional
// condition formula, and write-safe actions), enable/disable them, run them now or
// preview them with a dry run, and review the run history. Planning is the pure
// internal/workflow engine; applying effects + recording runs is appstate.
func Workflows() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	rev := ui.UseState(0)
	_ = rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }

	wfs := app.Workflows()
	var rows []ui.Node
	for _, w := range wfs {
		rows = append(rows, ui.CreateElement(workflowRow, workflowRowProps{Workflow: w, Refresh: refresh}))
	}
	listBody := P(Class("empty"), uistate.T("workflows.empty"))
	if len(rows) > 0 {
		listBody = Div(Class("rows"), rows)
	}

	return Div(
		ui.CreateElement(addWorkflowForm, addWorkflowFormProps{Refresh: refresh}),
		Section(Class("card"),
			H3(Class("card-title"), uistate.T("workflows.yours")),
			listBody,
		),
		ui.CreateElement(workflowHistory, workflowHistoryProps{}),
	)
}

type addWorkflowFormProps struct{ Refresh func() }

// addWorkflowForm builds a new workflow: name, trigger, optional condition, and a
// list of actions assembled one at a time. A single stable component so its many
// form hooks never run inside a loop.
func addWorkflowForm(props addWorkflowFormProps) ui.Node {
	name := ui.UseState("")
	trigger := ui.UseState(string(workflow.TriggerManual))
	condition := ui.UseState("")
	actions := ui.UseState([]workflow.Action(nil))
	draftKind := ui.UseState(string(workflow.ActionCreateTask))
	draftText := ui.UseState("")
	msg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTrigger := ui.UseEvent(func(v string) { trigger.Set(v) })
	onCondition := ui.UseEvent(func(v string) { condition.Set(v) })
	onDraftKind := ui.UseEvent(func(v string) { draftKind.Set(v) })
	onDraftText := ui.UseEvent(func(v string) { draftText.Set(v) })

	addAction := func() {
		a := workflow.Action{Kind: workflow.ActionKind(draftKind.Get())}
		switch a.Kind {
		case workflow.ActionCreateTask:
			a.Title = draftText.Get()
		case workflow.ActionNotify:
			a.Message = draftText.Get()
		}
		actions.Set(append(append([]workflow.Action(nil), actions.Get()...), a))
		draftText.Set("")
	}
	save := func() {
		app := appstate.Default
		w := workflow.Workflow{
			ID: id.New(), Name: name.Get(), Enabled: true,
			Trigger:   workflow.Trigger{Kind: workflow.TriggerKind(trigger.Get())},
			Condition: condition.Get(), Actions: actions.Get(),
		}
		if errs := workflow.Validate(w); len(errs) > 0 {
			msg.Set(errs[0])
			return
		}
		if err := app.PutWorkflow(w); err != nil {
			msg.Set(err.Error())
			return
		}
		name.Set("")
		condition.Set("")
		actions.Set(nil)
		msg.Set("")
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	// Rendered list of staged actions.
	var staged []ui.Node
	for i, a := range actions.Get() {
		staged = append(staged, Div(Class("row"),
			Span(Class("row-desc"), actionLabel(a)),
		))
		_ = i
	}

	return Section(Class("card"),
		H3(Class("card-title"), uistate.T("workflows.create")),
		Div(Class("form-grid"),
			Input(Class("field"), Attr("placeholder", uistate.T("workflows.name")), Value(name.Get()), OnInput(onName)),
			Select(Class("field"), OnChange(onTrigger),
				Option(Value(string(workflow.TriggerManual)), SelectedIf(trigger.Get() == string(workflow.TriggerManual)), uistate.T("workflows.triggerManual")),
				Option(Value(string(workflow.TriggerTxnAdded)), SelectedIf(trigger.Get() == string(workflow.TriggerTxnAdded)), uistate.T("workflows.triggerTxn")),
			),
			Input(Class("field"), Attr("placeholder", uistate.T("workflows.condition")), Value(condition.Get()), OnInput(onCondition)),
		),
		// Action builder.
		Div(Class("form-grid mt-2"),
			Select(Class("field"), OnChange(onDraftKind),
				Option(Value(string(workflow.ActionCreateTask)), SelectedIf(draftKind.Get() == string(workflow.ActionCreateTask)), uistate.T("workflows.actCreateTask")),
				Option(Value(string(workflow.ActionApplyRules)), SelectedIf(draftKind.Get() == string(workflow.ActionApplyRules)), uistate.T("workflows.actApplyRules")),
				Option(Value(string(workflow.ActionNotify)), SelectedIf(draftKind.Get() == string(workflow.ActionNotify)), uistate.T("workflows.actNotify")),
			),
			Input(Class("field"), Attr("placeholder", uistate.T("workflows.actionText")), Value(draftText.Get()), OnInput(onDraftText)),
			Button(Class("btn"), Type("button"), OnClick(addAction), uistate.T("workflows.addAction")),
		),
		If(len(staged) > 0, Div(Class("rows"), staged)),
		If(msg.Get() != "", P(Class("err"), Attr("role", "alert"), msg.Get())),
		Div(Class("mt-2"),
			Button(Class("btn btn-primary"), Type("button"), OnClick(save), uistate.T("workflows.save")),
		),
	)
}

type workflowRowProps struct {
	Workflow workflow.Workflow
	Refresh  func()
}

// workflowRow is one workflow with its controls (enable, run, dry-run, delete) and
// an inline area showing the last run's planned/applied effects. Its own component
// so the action hooks and the result state stay stable across the list.
func workflowRow(props workflowRowProps) ui.Node {
	w := props.Workflow
	last := ui.UseState((*workflow.Run)(nil))

	run := func(dry bool) {
		app := appstate.Default
		r, err := app.RunWorkflow(w, dry)
		if err != nil {
			r = workflow.Run{Effects: []workflow.Effect{{Summary: err.Error()}}}
		}
		rr := r
		last.Set(&rr)
		if !dry && props.Refresh != nil {
			props.Refresh()
		}
	}
	toggle := func() {
		app := appstate.Default
		w2 := w
		w2.Enabled = !w2.Enabled
		if err := app.PutWorkflow(w2); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}
	del := func() {
		app := appstate.Default
		if err := app.DeleteWorkflow(w.ID); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}

	enableLabel := uistate.T("workflows.enable")
	if w.Enabled {
		enableLabel = uistate.T("workflows.disable")
	}

	var result ui.Node = Fragment()
	if r := last.Get(); r != nil {
		if !r.Matched && !r.DryRun {
			result = P(Class("muted mt-1"), uistate.T("workflows.noMatch"))
		} else if !r.Matched && r.DryRun {
			result = P(Class("muted mt-1"), uistate.T("workflows.dryNoMatch"))
		} else {
			var lines []ui.Node
			for _, e := range r.Effects {
				lines = append(lines, Div(Class("row-meta"), "• "+e.Summary))
			}
			head := uistate.T("workflows.applied")
			if r.DryRun {
				head = uistate.T("workflows.wouldDo")
			}
			result = Div(Class("mt-1"), P(Class("row-meta"), head), Div(lines))
		}
	}

	return Div(Class("row-edit"),
		Div(Class("flex items-center justify-between gap-2 flex-wrap"),
			Div(Class("row-main"),
				Div(Class("row-desc"), w.Name),
				Div(Class("row-meta"), triggerLabel(w.Trigger.Kind)+conditionSuffix(w.Condition)+" · "+strconv.Itoa(len(w.Actions))+" "+uistate.T("workflows.actionsWord")),
			),
			Div(Class("flex gap-2 flex-wrap"),
				Button(Class("btn"), Type("button"), OnClick(func() { run(true) }), uistate.T("workflows.dryRun")),
				Button(Class("btn btn-primary"), Type("button"), OnClick(func() { run(false) }), uistate.T("workflows.runNow")),
				Button(Class("btn"), Type("button"), OnClick(toggle), enableLabel),
				Button(Class("btn-del"), Type("button"), Title(uistate.T("action.delete")), OnClick(del), "✕"),
			),
		),
		result,
	)
}

type workflowHistoryProps struct{}

// workflowHistory shows the most recent applied runs (newest first).
func workflowHistory(_ workflowHistoryProps) ui.Node {
	app := appstate.Default
	runs := app.WorkflowRuns()
	if len(runs) == 0 {
		return Fragment()
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].At > runs[j].At })
	names := map[string]string{}
	for _, w := range app.Workflows() {
		names[w.ID] = w.Name
	}
	var rows []ui.Node
	for i, r := range runs {
		if i >= 12 {
			break
		}
		name := names[r.WorkflowID]
		if name == "" {
			name = uistate.T("workflows.deleted")
		}
		rows = append(rows, Div(Class("row"),
			Span(Class("row-desc"), name),
			Span(Class("row-meta"), r.At+" · "+strconv.Itoa(len(r.Effects))+" "+uistate.T("workflows.effectsWord")),
		))
	}
	return Section(Class("card"),
		H3(Class("card-title"), uistate.T("workflows.history")),
		Div(Class("rows"), rows),
	)
}

// --- label helpers ---

func actionLabel(a workflow.Action) string {
	switch a.Kind {
	case workflow.ActionCreateTask:
		return uistate.T("workflows.actCreateTask") + ": " + a.Title
	case workflow.ActionApplyRules:
		return uistate.T("workflows.actApplyRules")
	case workflow.ActionNotify:
		return uistate.T("workflows.actNotify") + ": " + a.Message
	default:
		return string(a.Kind)
	}
}

func triggerLabel(k workflow.TriggerKind) string {
	if k == workflow.TriggerTxnAdded {
		return uistate.T("workflows.triggerTxn")
	}
	return uistate.T("workflows.triggerManual")
}

func conditionSuffix(cond string) string {
	if cond == "" {
		return ""
	}
	return " · if " + cond
}
