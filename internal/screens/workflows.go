//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workflow"
	"github.com/monstercameron/GoWebComponents/css"
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
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	rev := ui.UseState(0)
	_ = rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }

	wfs := app.Workflows()
	var rows []ui.Node
	for _, w := range wfs {
		rows = append(rows, ui.CreateElement(workflowRow, workflowRowProps{Workflow: w, Refresh: refresh}))
	}
	listBody := P(css.Class("empty"), uistate.T("workflows.empty"))
	if len(rows) > 0 {
		listBody = Div(css.Class("rows"), rows)
	}
	// Count badge on the "Your workflows" heading so Raj can see the list size at a
	// glance, consistent with Accounts / Goals / Transactions (G19).
	yoursTitle := uistate.T("workflows.yours")
	if n := len(wfs); n > 0 {
		yoursTitle += "  " + strconv.Itoa(n)
	}

	return Div(
		ui.CreateElement(addWorkflowForm, addWorkflowFormProps{Refresh: refresh}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: yoursTitle,
			Body:  listBody,
		}),
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
	cadence := ui.UseState(string(domain.CadenceMonthly))
	condition := ui.UseState("")
	actions := ui.UseState([]workflow.Action(nil))
	draftKind := ui.UseState(string(workflow.ActionCreateTask))
	draftText := ui.UseState("")
	draftCat := ui.UseState("")
	msg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTrigger := ui.UseEvent(func(v string) { trigger.Set(v) })
	onCadence := ui.UseEvent(func(v string) { cadence.Set(v) })
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
		case workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
			return a, true
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
		trig := workflow.Trigger{Kind: workflow.TriggerKind(trigger.Get())}
		if trig.Kind == workflow.TriggerScheduled {
			trig.Cadence = domain.RecurringCadence(cadence.Get())
			trig.NextRun = trig.Cadence.Next(time.Now())
		}
		w := workflow.Workflow{
			ID: id.New(), Name: name.Get(), Enabled: true,
			Trigger:   trig,
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
		paramControl = Select(css.Class("field"), OnChange(onDraftCat), opts)
	case workflow.ActionApplyRules, workflow.ActionFlagReview, workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
		paramControl = P(css.Class("muted"), uistate.T("workflows.noParam"))
	default: // createTask / notify / addTag
		paramControl = Input(css.Class("field"), Attr("placeholder", uistate.T("workflows.actionText")),
			Attr("aria-label", uistate.T("workflows.actionText")),
			Value(draftText.Get()), OnInput(onDraftText))
	}

	// Rendered list of staged actions.
	var staged []ui.Node
	for i, a := range actions.Get() {
		staged = append(staged, ui.CreateElement(stagedActionRow, stagedActionRowProps{
			Label: actionLabel(a), Index: i, OnRemove: removeAction,
		}))
	}

	// insertVar appends a workflow condition variable token to the condition
	// input. Each variable gets its own component (condVarButton) so the handler
	// hook lives at a stable position — never called inside a loop (framework
	// gotcha). The insert callback is a plain func passed as a prop.
	insertVar := func(v string) {
		cur := condition.Get()
		if cur != "" {
			condition.Set(cur + " " + v)
		} else {
			condition.Set(v)
		}
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("workflows.create"),
		Body: Fragment(
			Div(css.Class("form-grid"),
				Input(css.Class("field"), Attr("placeholder", uistate.T("workflows.name")), Attr("aria-label", uistate.T("workflows.name")), Value(name.Get()), OnInput(onName)),
				Select(css.Class("field"), Attr("aria-label", "Trigger"), OnChange(onTrigger),
					Option(Value(string(workflow.TriggerManual)), SelectedIf(trigger.Get() == string(workflow.TriggerManual)), uistate.T("workflows.triggerManual")),
					Option(Value(string(workflow.TriggerTxnAdded)), SelectedIf(trigger.Get() == string(workflow.TriggerTxnAdded)), uistate.T("workflows.triggerTxn")),
					Option(Value(string(workflow.TriggerScheduled)), SelectedIf(trigger.Get() == string(workflow.TriggerScheduled)), uistate.T("workflows.triggerScheduled")),
					Option(Value(string(workflow.TriggerBudgetExceeded)), SelectedIf(trigger.Get() == string(workflow.TriggerBudgetExceeded)), uistate.T("workflows.triggerBudgetExceeded")),
					Option(Value(string(workflow.TriggerGoalReached)), SelectedIf(trigger.Get() == string(workflow.TriggerGoalReached)), uistate.T("workflows.triggerGoalReached")),
					Option(Value(string(workflow.TriggerBillDue)), SelectedIf(trigger.Get() == string(workflow.TriggerBillDue)), uistate.T("workflows.triggerBillDue")),
				),
				If(trigger.Get() == string(workflow.TriggerScheduled),
					Select(css.Class("field"), OnChange(onCadence),
						Option(Value(string(domain.CadenceWeekly)), SelectedIf(cadence.Get() == string(domain.CadenceWeekly)), uistate.T("workflows.cadenceWeekly")),
						Option(Value(string(domain.CadenceMonthly)), SelectedIf(cadence.Get() == string(domain.CadenceMonthly)), uistate.T("workflows.cadenceMonthly")),
						Option(Value(string(domain.CadenceQuarterly)), SelectedIf(cadence.Get() == string(domain.CadenceQuarterly)), uistate.T("workflows.cadenceQuarterly")),
						Option(Value(string(domain.CadenceYearly)), SelectedIf(cadence.Get() == string(domain.CadenceYearly)), uistate.T("workflows.cadenceYearly")),
					),
				),
			),
			// Condition input in its own full-width row so it has room to breathe and
			// isn't truncated to ~10 chars inside a 3-column form-grid cell (GI3).
			Div(css.Class("form-grid", tw.Mt1),
				Input(css.Class("field", "field-wide"), Attr("placeholder", uistate.T("workflows.condition")), Attr("aria-label", uistate.T("workflows.conditionLabel")), Value(condition.Get()), OnInput(onCondition)),
			),
			// Inline variable reference for the condition formula (C65). Lists every
			// available variable with a click-to-insert button so users don't need to
			// memorise the names. Each variable is its own component (condVarButton) so
			// its OnClick hook never runs inside a variable-length loop (framework rule).
			// Transaction-context variables are only injected when the trigger is
			// "transaction added"; all four are always shown here as reference since users
			// can test conditions with "run now" on any trigger.
			Div(css.Class(tw.Mt2),
				P(css.Class("muted"), uistate.T("workflows.conditionHint")),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap15, tw.Mt1),
					ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_abs", Desc: uistate.T("workflows.varTxnAbs"), OnInsert: insertVar}),
					ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_amount", Desc: uistate.T("workflows.varTxnAmount"), OnInsert: insertVar}),
					ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_payee", Desc: uistate.T("workflows.varTxnPayee"), OnInsert: insertVar}),
					ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_category", Desc: uistate.T("workflows.varTxnCategory"), OnInsert: insertVar}),
				),
				P(css.Class("muted", tw.Mt1), uistate.T("workflows.conditionExamples")),
			),
			// Action builder. A small "Actions" sub-label and a light divider visually
			// separate the workflow identity section (name/trigger/condition) from the
			// action-builder section (what it will do), so Raj doesn't process them as
			// one undifferentiated block (G19 spacing fix).
			Hr(css.Class(tw.Mt2)),
			P(css.Class("muted"), "Actions"),
			Div(css.Class("form-grid", tw.Mt1),
				Select(css.Class("field"), Attr("aria-label", "Action type"), OnChange(onDraftKind),
					Option(Value(string(workflow.ActionCreateTask)), SelectedIf(draftKind.Get() == string(workflow.ActionCreateTask)), uistate.T("workflows.actCreateTask")),
					Option(Value(string(workflow.ActionSetCategory)), SelectedIf(draftKind.Get() == string(workflow.ActionSetCategory)), uistate.T("workflows.actSetCategory")),
					Option(Value(string(workflow.ActionAddTag)), SelectedIf(draftKind.Get() == string(workflow.ActionAddTag)), uistate.T("workflows.actAddTag")),
					Option(Value(string(workflow.ActionFlagReview)), SelectedIf(draftKind.Get() == string(workflow.ActionFlagReview)), uistate.T("workflows.actFlagReview")),
					Option(Value(string(workflow.ActionApplyRules)), SelectedIf(draftKind.Get() == string(workflow.ActionApplyRules)), uistate.T("workflows.actApplyRules")),
					Option(Value(string(workflow.ActionNotify)), SelectedIf(draftKind.Get() == string(workflow.ActionNotify)), uistate.T("workflows.actNotify")),
					Option(Value(string(workflow.ActionPostRecurring)), SelectedIf(draftKind.Get() == string(workflow.ActionPostRecurring)), uistate.T("workflows.actPostRecurring")),
					Option(Value(string(workflow.ActionFlagBudgetOver)), SelectedIf(draftKind.Get() == string(workflow.ActionFlagBudgetOver)), uistate.T("workflows.actFlagBudgetOver")),
				),
				paramControl,
				// btn-sm: "Add action" is a staging step, not the primary save — giving it
				// full-column width made it visually heavier than "Save workflow" (G19).
				Button(css.Class("btn btn-sm"), Type("button"), OnClick(addAction), uistate.T("workflows.addAction")),
			),
			// mt-3 + "rows" wrapper gives the staged list a clear visual gap so it reads
			// as "what will be done" rather than a continuation of the action inputs (G19).
			If(len(staged) > 0, Div(css.Class("rows", tw.Mt3), staged)),
			If(msg.Get() != "", P(css.Class("err"), Attr("role", "alert"), msg.Get())),
			Div(css.Class(tw.Mt2),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(save), uistate.T("workflows.save")),
			),
		),
	})
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
	showDiagram := ui.UseState(false)
	editing := ui.UseState(false)
	editName := ui.UseState(w.Name)
	editCond := ui.UseState(w.Condition)
	onEditName := ui.UseEvent(func(v string) { editName.Set(v) })
	onEditCond := ui.UseEvent(func(v string) { editCond.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		editName.Set(w.Name)
		editCond.Set(w.Condition)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		n := strings.TrimSpace(editName.Get())
		if n == "" {
			return
		}
		w2 := w
		w2.Name = n
		w2.Condition = strings.TrimSpace(editCond.Get())
		if err := app.PutWorkflow(w2); err == nil {
			editing.Set(false)
			if props.Refresh != nil {
				props.Refresh()
			}
		}
	}))

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

	if editing.Get() {
		return Div(css.Class("row-edit"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("workflows.name"),
					Input(css.Class("field"), Attr("aria-label", uistate.T("workflows.name")), Value(editName.Get()), OnInput(onEditName))),
				labeledField(uistate.T("workflows.conditionLabel"),
					Input(css.Class("field"), Attr("aria-label", uistate.T("workflows.conditionLabel")), Placeholder(uistate.T("workflows.condition")), Value(editCond.Get()), OnInput(onEditCond))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	enableLabel := uistate.T("workflows.enable")
	if w.Enabled {
		enableLabel = uistate.T("workflows.disable")
	}

	var result ui.Node = Fragment()
	if r := last.Get(); r != nil {
		if !r.Matched && !r.DryRun {
			result = P(css.Class("muted", tw.Mt1), uistate.T("workflows.noMatch"))
		} else if !r.Matched && r.DryRun {
			result = P(css.Class("muted", tw.Mt1), uistate.T("workflows.dryNoMatch"))
		} else {
			var lines []ui.Node
			for _, e := range r.Effects {
				lines = append(lines, Div(css.Class("row-meta"), "• "+e.Summary))
			}
			head := uistate.T("workflows.applied")
			if r.DryRun {
				head = uistate.T("workflows.wouldDo")
			}
			result = Div(css.Class(tw.Mt1), P(css.Class("row-meta"), head), Div(lines))
		}
	}

	return Div(css.Class("row-edit"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.FlexWrap),
			Div(css.Class("row-main"),
				Div(css.Class("row-desc"), w.Name),
				Div(css.Class("row-meta"), triggerLabel(w.Trigger.Kind)+conditionSuffix(w.Condition)+" · "+actionsLabel(len(w.Actions))),
			),
			Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap),
				// "Dry run" is the safe exploratory action and gets the primary accent so
				// Raj's first instinct is simulation, not live execution (G19 hierarchy fix).
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(func() { run(true) }), uistate.T("workflows.dryRun")),
				// "Run now" is the live-execution action; neutral weight signals it is the
				// deliberate, secondary step after previewing the dry run.
				Button(css.Class("btn"), Type("button"), OnClick(func() { run(false) }), uistate.T("workflows.runNow")),
				Button(css.Class("btn"), Type("button"), OnClick(toggle), enableLabel),
				Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("action.edit")), Title(uistate.T("action.edit")), OnClick(startEdit), uistate.T("action.edit")),
				Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("action.delete")), Title(uistate.T("action.delete")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
			),
		),
		result,
		// A Mermaid flowchart of this workflow: trigger → condition → actions (C70).
		// Collapsed by default (GI3): 4 workflows rendered ~2000px of diagrams and
		// buried the run-history card. Each row owns its own showDiagram state so the
		// toggle never runs inside a loop (framework hook rule).
		Div(css.Class(tw.Mt1),
			Button(css.Class("btn"), Type("button"), OnClick(func() { showDiagram.Set(!showDiagram.Get()) }),
				IfElse(showDiagram.Get(), Text("Hide diagram"), Text("Show diagram")),
			),
			If(showDiagram.Get(),
				uiw.Mermaid(uiw.MermaidProps{
					Source: mermaid.FromWorkflow(w),
					Class:  tw.Fold(tw.Mt2),
					Label:  "Flowchart of " + w.Name,
				}),
			),
		),
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
		rows = append(rows, Div(css.Class("row"),
			Span(css.Class("row-desc"), name),
			Span(css.Class("row-meta"), r.At+" · "+strconv.Itoa(len(r.Effects))+" "+uistate.T("workflows.effectsWord")),
		))
	}
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("workflows.history"),
		Rows:  rows,
	})
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
	case workflow.ActionPostRecurring:
		return uistate.T("workflows.actPostRecurring")
	case workflow.ActionFlagBudgetOver:
		return uistate.T("workflows.actFlagBudgetOver")
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
	switch k {
	case workflow.TriggerTxnAdded:
		return uistate.T("workflows.triggerTxn")
	case workflow.TriggerScheduled:
		return uistate.T("workflows.triggerScheduled")
	case workflow.TriggerBudgetExceeded:
		return uistate.T("workflows.triggerBudgetExceeded")
	case workflow.TriggerGoalReached:
		return uistate.T("workflows.triggerGoalReached")
	case workflow.TriggerBillDue:
		return uistate.T("workflows.triggerBillDue")
	default:
		return uistate.T("workflows.triggerManual")
	}
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
	return Div(css.Class("row"),
		Span(css.Class("row-desc"), props.Label),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", "Remove action"), Title("Remove action"),
			OnClick(func() { props.OnRemove(props.Index) }), "✕"),
	)
}

type condVarButtonProps struct {
	Token    string       // variable name, e.g. "txn_abs"
	Desc     string       // short description shown as a tooltip
	OnInsert func(string) // called with Token when the user clicks
}

// condVarButton renders one workflow-condition variable as a clickable pill that
// appends the variable name to the condition input (C65). It is its own component
// (not rendered inside a loop) so its OnClick hook always runs at a stable render
// position — satisfying the framework's no-hooks-in-loops rule.
func condVarButton(props condVarButtonProps) ui.Node {
	ins := ui.UseEvent(Prevent(func() { props.OnInsert(props.Token) }))
	return Button(css.Class("data-btn"), Type("button"),
		Attr("data-testid", "cond-var-"+props.Token),
		Title(props.Desc),
		Attr("aria-label", "Insert "+props.Token+" into condition"),
		OnClick(ins), props.Token)
}
