//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
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
		return Section(ClassStr("card"), P(ClassStr("empty"), uistate.T("common.notReady")))
	}
	rev := ui.UseState(0)
	_ = rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }

	wfs := app.Workflows()
	var rows []ui.Node
	for _, w := range wfs {
		rows = append(rows, ui.CreateElement(workflowRow, workflowRowProps{Workflow: w, Refresh: refresh}))
	}
	listBody := P(ClassStr("empty"), uistate.T("workflows.empty"))
	if len(rows) > 0 {
		listBody = Div(ClassStr("rows"), rows)
	}

	return Div(
		ui.CreateElement(addWorkflowForm, addWorkflowFormProps{Refresh: refresh}),
		Section(ClassStr("card"),
			H3(ClassStr("card-title"), uistate.T("workflows.yours")),
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
	draftCat := ui.UseState("")
	msg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTrigger := ui.UseEvent(func(v string) { trigger.Set(v) })
	onCondition := ui.UseEvent(func(v string) { condition.Set(v) })
	onDraftKind := ui.UseEvent(func(v string) { draftKind.Set(v) })
	onDraftText := ui.UseEvent(func(v string) { draftText.Set(v) })
	onDraftCat := ui.UseEvent(func(e ui.Event) { draftCat.Set(e.GetValue()) })

	// buildDraft turns the current action-builder fields into an Action, reporting
	// whether it's complete enough to add (so an empty draft isn't staged, and a
	// half-filled one gives a reason rather than failing silently).
	buildDraft := func() (workflow.Action, bool) {
		a := workflow.Action{Kind: workflow.ActionKind(draftKind.Get())}
		switch a.Kind {
		case workflow.ActionCreateTask:
			a.Title = strings.TrimSpace(draftText.Get())
			return a, a.Title != ""
		case workflow.ActionNotify:
			a.Message = strings.TrimSpace(draftText.Get())
			return a, a.Message != ""
		case workflow.ActionAddTag:
			a.Tag = strings.TrimSpace(draftText.Get())
			return a, a.Tag != ""
		case workflow.ActionSetCategory:
			a.CategoryID = draftCat.Get()
			return a, a.CategoryID != ""
		default: // applyRules / flagReview need no parameter
			return a, true
		}
	}
	addAction := func() {
		a, ok := buildDraft()
		if !ok {
			msg.Set(uistate.T("workflows.actionNeedsValue"))
			return
		}
		actions.Set(append(append([]workflow.Action(nil), actions.Get()...), a))
		draftText.Set("")
		draftCat.Set("")
		msg.Set("")
	}
	// Drop a staged action before saving, so a mistaken one doesn't force starting
	// the whole workflow over (C65).
	removeAction := func(i int) {
		cur := actions.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := append([]workflow.Action(nil), cur[:i]...)
		next = append(next, cur[i+1:]...)
		actions.Set(next)
	}
	save := func() {
		app := appstate.Default
		// Fold in a still-pending action the user typed but didn't click "Add
		// action" for, so a filled-but-unstaged action isn't silently lost (C37).
		acts := append([]workflow.Action(nil), actions.Get()...)
		if a, ok := buildDraft(); ok {
			acts = append(acts, a)
		}
		w := workflow.Workflow{
			ID: id.New(), Name: name.Get(), Enabled: true,
			Trigger:   workflow.Trigger{Kind: workflow.TriggerKind(trigger.Get())},
			Condition: condition.Get(), Actions: acts,
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
		draftText.Set("")
		draftCat.Set("")
		msg.Set("")
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	// The action parameter control depends on the chosen action kind.
	var paramControl ui.Node
	switch workflow.ActionKind(draftKind.Get()) {
	case workflow.ActionSetCategory:
		opts := []ui.Node{Option(Value(""), uistate.T("workflows.chooseCategory"))}
		if appstate.Default != nil {
			for _, c := range appstate.Default.Categories() {
				opts = append(opts, Option(Value(c.ID), SelectedIf(draftCat.Get() == c.ID), c.Name))
			}
		}
		paramControl = Select(ClassStr("field"), OnChange(onDraftCat), opts)
	case workflow.ActionApplyRules, workflow.ActionFlagReview:
		paramControl = P(ClassStr("muted"), uistate.T("workflows.noParam"))
	default: // createTask / notify / addTag
		paramControl = Input(ClassStr("field"), Attr("placeholder", uistate.T("workflows.actionText")),
			Value(draftText.Get()), OnInput(onDraftText))
	}

	// Rendered list of staged actions.
	var staged []ui.Node
	for i, a := range actions.Get() {
		staged = append(staged, ui.CreateElement(stagedActionRow, stagedActionRowProps{
			Label: actionLabel(a), Index: i, OnRemove: removeAction,
		}))
	}

	return Section(ClassStr("card"),
		H3(ClassStr("card-title"), uistate.T("workflows.create")),
		Div(ClassStr("form-grid"),
			Input(ClassStr("field"), Attr("placeholder", uistate.T("workflows.name")), Value(name.Get()), OnInput(onName)),
			Select(ClassStr("field"), OnChange(onTrigger),
				Option(Value(string(workflow.TriggerManual)), SelectedIf(trigger.Get() == string(workflow.TriggerManual)), uistate.T("workflows.triggerManual")),
				Option(Value(string(workflow.TriggerTxnAdded)), SelectedIf(trigger.Get() == string(workflow.TriggerTxnAdded)), uistate.T("workflows.triggerTxn")),
			),
			Input(ClassStr("field"), Attr("placeholder", uistate.T("workflows.condition")), Value(condition.Get()), OnInput(onCondition)),
		),
		// Action builder. The parameter control depends on the chosen action:
		// a category picker for "set category", a text field for create-task /
		// notify / add-tag, and nothing for apply-rules / flag-for-review.
		Div(ClassStr("form-grid mt-2"),
			Select(ClassStr("field"), OnChange(onDraftKind),
				Option(Value(string(workflow.ActionCreateTask)), SelectedIf(draftKind.Get() == string(workflow.ActionCreateTask)), uistate.T("workflows.actCreateTask")),
				Option(Value(string(workflow.ActionSetCategory)), SelectedIf(draftKind.Get() == string(workflow.ActionSetCategory)), uistate.T("workflows.actSetCategory")),
				Option(Value(string(workflow.ActionAddTag)), SelectedIf(draftKind.Get() == string(workflow.ActionAddTag)), uistate.T("workflows.actAddTag")),
				Option(Value(string(workflow.ActionFlagReview)), SelectedIf(draftKind.Get() == string(workflow.ActionFlagReview)), uistate.T("workflows.actFlagReview")),
				Option(Value(string(workflow.ActionApplyRules)), SelectedIf(draftKind.Get() == string(workflow.ActionApplyRules)), uistate.T("workflows.actApplyRules")),
				Option(Value(string(workflow.ActionNotify)), SelectedIf(draftKind.Get() == string(workflow.ActionNotify)), uistate.T("workflows.actNotify")),
			),
			paramControl,
			Button(ClassStr("btn"), Type("button"), OnClick(addAction), uistate.T("workflows.addAction")),
		),
		If(len(staged) > 0, Div(ClassStr("rows"), staged)),
		If(msg.Get() != "", P(ClassStr("err"), Attr("role", "alert"), msg.Get())),
		Div(ClassStr("mt-2"),
			Button(ClassStr("btn btn-primary"), Type("button"), OnClick(save), uistate.T("workflows.save")),
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
			result = P(ClassStr("muted mt-1"), uistate.T("workflows.noMatch"))
		} else if !r.Matched && r.DryRun {
			result = P(ClassStr("muted mt-1"), uistate.T("workflows.dryNoMatch"))
		} else {
			var lines []ui.Node
			for _, e := range r.Effects {
				lines = append(lines, Div(ClassStr("row-meta"), "• "+e.Summary))
			}
			head := uistate.T("workflows.applied")
			if r.DryRun {
				head = uistate.T("workflows.wouldDo")
			}
			result = Div(ClassStr("mt-1"), P(ClassStr("row-meta"), head), Div(lines))
		}
	}

	return Div(ClassStr("row-edit"),
		Div(ClassStr("flex items-center justify-between gap-2 flex-wrap"),
			Div(ClassStr("row-main"),
				Div(ClassStr("row-desc"), w.Name),
				Div(ClassStr("row-meta"), triggerLabel(w.Trigger.Kind)+conditionSuffix(w.Condition)+" · "+actionsLabel(len(w.Actions))),
			),
			Div(ClassStr("flex gap-2 flex-wrap"),
				Button(ClassStr("btn"), Type("button"), OnClick(func() { run(true) }), uistate.T("workflows.dryRun")),
				Button(ClassStr("btn btn-primary"), Type("button"), OnClick(func() { run(false) }), uistate.T("workflows.runNow")),
				Button(ClassStr("btn"), Type("button"), OnClick(toggle), enableLabel),
				Button(ClassStr("btn-del"), Type("button"), Attr("aria-label", uistate.T("action.delete")), Title(uistate.T("action.delete")), OnClick(del), uiw.Icon(icon.Close, ClassStr("w-4 h-4"))),
			),
		),
		result,
		// A Mermaid flowchart of this workflow: trigger → condition → actions (C70).
		uiw.Mermaid(uiw.MermaidProps{
			Source: mermaid.FromWorkflow(w),
			Class:  "mt-2",
			Label:  "Flowchart of " + w.Name,
		}),
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
		rows = append(rows, Div(ClassStr("row"),
			Span(ClassStr("row-desc"), name),
			Span(ClassStr("row-meta"), r.At+" · "+strconv.Itoa(len(r.Effects))+" "+uistate.T("workflows.effectsWord")),
		))
	}
	return Section(ClassStr("card"),
		H3(ClassStr("card-title"), uistate.T("workflows.history")),
		Div(ClassStr("rows"), rows),
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
	case workflow.ActionSetCategory:
		return uistate.T("workflows.actSetCategory")
	case workflow.ActionAddTag:
		return uistate.T("workflows.actAddTag") + ": " + a.Tag
	case workflow.ActionFlagReview:
		return uistate.T("workflows.actFlagReview")
	default:
		return string(a.Kind)
	}
}

// actionsLabel renders the action count with correct singular/plural wording
// ("1 action" vs "2 actions") instead of always-plural "N actions" (C54).
func actionsLabel(n int) string {
	word := uistate.T("workflows.actionsWord")
	if n == 1 {
		word = uistate.T("workflows.actionWord")
	}
	return strconv.Itoa(n) + " " + word
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

type stagedActionRowProps struct {
	Label    string
	Index    int
	OnRemove func(int)
}

// stagedActionRow renders one staged (not-yet-saved) workflow action with a remove
// button, so a mistaken action can be dropped before saving (C65). It is its own
// component so the remove button's OnClick hook sits at a stable render position —
// the staged list is variable-length (the framework loop-hook gotcha).
func stagedActionRow(props stagedActionRowProps) ui.Node {
	return Div(ClassStr("row"),
		Span(ClassStr("row-desc"), props.Label),
		Button(ClassStr("btn-del"), Type("button"), Attr("aria-label", "Remove action"), Title("Remove action"),
			OnClick(func() { props.OnRemove(props.Index) }), "✕"),
	)
}
