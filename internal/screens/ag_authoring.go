// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsAuthoring(app, base, rates)...) in buildChatTools

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/nlauthor"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// assistantObjectArgs requires one object and rejects unknown or mistyped fields.
func assistantObjectArgs(raw json.RawMessage, out any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("arguments must be one JSON object")
	}
	if err := assistantDataArgs(raw, out); err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return assistantArgumentShape(value, reflect.TypeOf(out).Elem())
}

// JSON's null and case-insensitive struct field matching are more permissive
// than these tool schemas. Check their shape after typed decoding as well.
func assistantArgumentShape(value any, target reflect.Type) error {
	if value == nil {
		return fmt.Errorf("null is not allowed in these tool arguments")
	}
	switch v := value.(type) {
	case map[string]any:
		if target.Kind() != reflect.Struct {
			return nil // RawMessage fields perform their own value validation.
		}
		fields := map[string]reflect.Type{}
		for i := range target.NumField() {
			field := target.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			fields[name] = field.Type
		}
		for name, child := range v {
			fieldType, ok := fields[name]
			if !ok {
				return fmt.Errorf("unknown argument %q", name)
			}
			if err := assistantArgumentShape(child, fieldType); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	case []any:
		if target.Kind() == reflect.Slice {
			for _, child := range v {
				if err := assistantArgumentShape(child, target.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// agToolsAuthoring exposes natural-language rule authoring (AG3) and workflow
// authoring (AG4) as mutating assistant tools. The model turns the user's plain
// English into structured arguments; these tools resolve names to IDs, compile the
// typed domain object through the pure nlauthor helpers (a workflow condition is
// validated through the SAME formula language the engine runs, before saving), and
// preview the compiled result and current match count for approval. Saving a rule
// leaves existing transactions unchanged.
func agToolsAuthoring(app *appstate.App, base string, rates currency.Rates) []chatTool {
	resolveCategory := func(name string) (domain.Category, bool) {
		q := strings.ToLower(strings.TrimSpace(name))
		var exact, partial []domain.Category
		for _, c := range app.Categories() {
			if strings.ToLower(c.Name) == q {
				exact = append(exact, c)
			} else if q != "" && strings.Contains(strings.ToLower(c.Name), q) {
				partial = append(partial, c)
			}
		}
		if len(exact) == 1 {
			return exact[0], true
		}
		if len(exact) == 0 && len(partial) == 1 {
			return partial[0], true
		}
		return domain.Category{}, false
	}
	catNames := func() string {
		cats := app.Categories()
		ns := make([]string, 0, len(cats))
		for _, c := range cats {
			ns = append(ns, c.Name)
		}
		return strings.Join(ns, ", ")
	}
	txnCtxs := func() []rules.TxnCtx {
		txns := app.Transactions()
		out := make([]rules.TxnCtx, 0, len(txns))
		for _, t := range txns {
			out = append(out, rules.TxnCtx{
				Payee: t.Payee, Desc: t.Desc, AmountMinor: t.Amount.Amount,
				AccountID: t.AccountID, Date: rules.NewTxnDate(t.Date),
			})
		}
		return out
	}

	type ruleArgs struct {
		Match      string   `json:"match"`
		Category   string   `json:"category"`
		Tags       []string `json:"tags"`
		RenameDesc string   `json:"rename_desc"`
	}
	parseRule := func(raw json.RawMessage, ruleID string, order int) (ruleArgs, rules.Rule, []string, bool) {
		var a ruleArgs
		if err := assistantObjectArgs(raw, &a); err != nil {
			return a, rules.Rule{}, []string{err.Error()}, false
		}
		catID := ""
		if a.Category != "" {
			if c, ok := resolveCategory(a.Category); ok {
				catID = c.ID
			} else {
				return a, rules.Rule{}, []string{"No unique category matching “" + a.Category + "”. Use an unambiguous existing name: " + catNames() + "."}, false
			}
		}
		r, errs := nlauthor.CompileRule(nlauthor.RuleInput{
			ID: ruleID, Match: a.Match, SetCategoryID: catID,
			SetTags: a.Tags, RenameDesc: a.RenameDesc, Order: order,
		})
		return a, r, errs, len(errs) == 0
	}

	type wfActionArg struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Message  string `json:"message"`
		Category string `json:"category"`
		Tag      string `json:"tag"`
	}
	type wfArgs struct {
		Name      string        `json:"name"`
		Trigger   string        `json:"trigger"`
		Condition string        `json:"condition"`
		Actions   []wfActionArg `json:"actions"`
	}
	buildActions := func(in []wfActionArg) ([]workflow.Action, []string) {
		var out []workflow.Action
		var errs []string
		for _, a := range in {
			act := workflow.Action{Kind: workflow.ActionKind(strings.TrimSpace(a.Kind))}
			unused := false
			switch act.Kind {
			case workflow.ActionCreateTask:
				act.Title, act.Notes = strings.TrimSpace(a.Title), strings.TrimSpace(a.Message)
				unused = a.Category != "" || a.Tag != ""
			case workflow.ActionNotify:
				act.Message = strings.TrimSpace(a.Message)
				if act.Message == "" {
					errs = append(errs, "A notification action needs a message.")
				}
				unused = a.Title != "" || a.Category != "" || a.Tag != ""
			case workflow.ActionSetCategory:
				if c, ok := resolveCategory(a.Category); ok {
					act.CategoryID = c.ID
				} else {
					errs = append(errs, "No unique category matching “"+a.Category+"” for the set-category action.")
				}
				unused = a.Title != "" || a.Message != "" || a.Tag != ""
			case workflow.ActionAddTag:
				act.Tag = strings.TrimSpace(a.Tag)
				unused = a.Title != "" || a.Message != "" || a.Category != ""
			case workflow.ActionFlagReview:
				unused = a.Title != "" || a.Message != "" || a.Category != "" || a.Tag != ""
			default:
				errs = append(errs, "Unsupported action kind “"+a.Kind+"”.")
			}
			if unused {
				errs = append(errs, "Action “"+a.Kind+"” includes fields that it does not use.")
			}
			out = append(out, act)
		}
		return out, errs
	}
	parseWorkflow := func(raw json.RawMessage, workflowID string) (wfArgs, workflow.Workflow, []string, bool) {
		var a wfArgs
		if err := assistantObjectArgs(raw, &a); err != nil {
			return a, workflow.Workflow{}, []string{err.Error()}, false
		}
		acts, actErrs := buildActions(a.Actions)
		trig := workflow.Trigger{Kind: workflow.TriggerKind(strings.TrimSpace(a.Trigger))}
		known := func(name string) bool { return false }
		names := app.WorkflowVariableNamesFor(trig.Kind)
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		known = func(name string) bool { return set[name] }
		wf, errs := nlauthor.CompileWorkflow(nlauthor.WorkflowInput{
			ID: workflowID, Name: a.Name, Trigger: trig, Condition: a.Condition, Actions: acts, Enabled: true,
		}, known)
		errs = append(actErrs, errs...)
		return a, wf, errs, len(errs) == 0
	}

	describeRule := func(r rules.Rule, catName string) string {
		var parts []string
		if r.SetCategoryID != "" {
			for _, category := range app.Categories() {
				if category.ID == r.SetCategoryID {
					catName = category.Name
				}
			}
			parts = append(parts, fmt.Sprintf("categorize as %q (category ID %s)", catName, r.SetCategoryID))
		}
		if len(r.SetTags) > 0 {
			parts = append(parts, "tag "+strings.Join(r.SetTags, ", "))
		}
		if r.RenameDesc != "" {
			parts = append(parts, "rename to “"+r.RenameDesc+"”")
		}
		match := r.Match
		if match == "" {
			match = fmt.Sprintf("%d condition(s)", len(r.Conditions))
		}
		return fmt.Sprintf("When a transaction matches “%s”: %s", match, strings.Join(parts, "; "))
	}
	type rulePlan struct {
		args ruleArgs
		rule rules.Rule
		errs []string
		ok   bool
	}
	type workflowPlan struct {
		wf   workflow.Workflow
		errs []string
		ok   bool
	}
	rulePlans := map[string]rulePlan{}
	prepareRule := func(raw json.RawMessage) rulePlan {
		previous := rulePlans[string(raw)]
		ruleID, order := previous.rule.ID, previous.rule.Order
		if ruleID == "" {
			ruleID, order = id.New(), app.NextRuleOrder()
		}
		a, r, errs, ok := parseRule(raw, ruleID, order)
		value := rulePlan{a, r, errs, ok}
		rulePlans[string(raw)] = value
		return value
	}
	workflowPlans := map[string]workflowPlan{}
	prepareWorkflow := func(raw json.RawMessage) workflowPlan {
		workflowID := workflowPlans[string(raw)].wf.ID
		if workflowID == "" {
			workflowID = id.New()
		}
		_, wf, errs, ok := parseWorkflow(raw, workflowID)
		value := workflowPlan{wf, errs, ok}
		workflowPlans[string(raw)] = value
		return value
	}

	return []chatTool{
		{
			spec: ai.FunctionTool("create_rule",
				"Save a categorization rule for future rule application. Give a payee/description match phrase, a category name, optional tags, and an optional description rewrite. Approval saves only the rule; existing transactions are not recategorized.",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string","description":"case-insensitive phrase in the payee/description, e.g. Trader Joe's"},"category":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"rename_desc":{"type":"string","description":"optional: rewrite the matched transaction's description"}},"required":["match"],"additionalProperties":false}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				value := prepareRule(raw)
				if !value.ok {
					return "Can't create that rule: " + strings.Join(value.errs, " ")
				}
				n := nlauthor.RuleMatchCount(value.rule, txnCtxs())
				return fmt.Sprintf("Save rule ID %s: %s.\nMatches %d existing transaction(s); existing transactions stay unchanged.", value.rule.ID, describeRule(value.rule, strings.TrimSpace(value.args.Category)), n)
			},
			run: func(raw json.RawMessage) string {
				value, exists := rulePlans[string(raw)]
				if !exists {
					value = prepareRule(raw)
				}
				if !value.ok {
					return "Can't create that rule: " + strings.Join(value.errs, " ")
				}
				for _, existing := range app.Rules() {
					if existing.ID == value.rule.ID {
						if reflect.DeepEqual(existing, value.rule) {
							return "That rule is already saved; nothing changed." + openLink("/rules", existing.ID)
						}
						return "That rule changed after the preview; review it again. Nothing changed."
					}
				}
				_, live, _, valid := parseRule(raw, value.rule.ID, value.rule.Order)
				if !valid || !reflect.DeepEqual(live, value.rule) {
					return "The rule's inputs changed after the preview; review it again. Nothing changed."
				}
				if err := app.PutRule(value.rule); err != nil {
					return "Couldn't save the rule: " + err.Error()
				}
				return fmt.Sprintf("Created the rule: %s. Existing transactions stay unchanged.%s", describeRule(value.rule, strings.TrimSpace(value.args.Category)), openLink("/rules", value.rule.ID))
			},
		},
		{
			spec: ai.FunctionTool("create_workflow",
				"Author an automation WORKFLOW from a natural-language description. Choose a trigger (txn-added, scheduled, budget-exceeded, goal-reached, bill-due, manual), an optional condition in the formula language (e.g. txn_abs > 500 — validated before saving), and one or more actions (createTask, notify, setCategory, addTag, flagReview). Transaction-changing actions (setCategory/addTag/flagReview) require the txn-added trigger. Previews the compiled workflow before applying.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"trigger":{"type":"string","enum":["txn-added","scheduled","budget-exceeded","goal-reached","bill-due","manual"]},"condition":{"type":"string","description":"optional formula over engine variables + txn_* fields, e.g. txn_abs > 500"},"actions":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["createTask","notify","setCategory","addTag","flagReview"]},"title":{"type":"string"},"message":{"type":"string"},"category":{"type":"string"},"tag":{"type":"string"}},"required":["kind"],"additionalProperties":false}}},"required":["name","trigger","actions"],"additionalProperties":false}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				value := prepareWorkflow(raw)
				if !value.ok {
					return "Can't create that workflow: " + strings.Join(value.errs, " ")
				}
				return describeWorkflow(value.wf, app.Categories())
			},
			run: func(raw json.RawMessage) string {
				value, exists := workflowPlans[string(raw)]
				if !exists {
					value = prepareWorkflow(raw)
				}
				if !value.ok {
					return "Can't create that workflow: " + strings.Join(value.errs, " ")
				}
				for _, existing := range app.Workflows() {
					if existing.ID == value.wf.ID {
						if reflect.DeepEqual(existing, value.wf) {
							return "That workflow is already saved; nothing changed." + openLink("/workflows", existing.ID)
						}
						return "That workflow changed after the preview; review it again. Nothing changed."
					}
				}
				_, live, _, valid := parseWorkflow(raw, value.wf.ID)
				if !valid || !reflect.DeepEqual(live, value.wf) {
					return "The workflow's inputs changed after the preview; review it again. Nothing changed."
				}
				if err := app.PutWorkflow(value.wf); err != nil {
					return "Couldn't save the workflow: " + err.Error()
				}
				return "Created the workflow: " + describeWorkflow(value.wf, app.Categories()) + openLink("/workflows", value.wf.ID)
			},
		},
	}
}

// describeWorkflow renders a compiled workflow in the editor's own vocabulary, so
// the preview teaches the DSL by example (AG4).
func describeWorkflow(wf workflow.Workflow, categories []domain.Category) string {
	s := fmt.Sprintf("Save enabled workflow %q (ID %s) — WHEN %s", wf.Name, wf.ID, wf.Trigger.Kind)
	if wf.Condition != "" {
		s += " AND (" + wf.Condition + ")"
	}
	for i, action := range wf.Actions {
		detail := ""
		switch action.Kind {
		case workflow.ActionCreateTask:
			detail = fmt.Sprintf("create to-do %q; notes %q", action.Title, action.Notes)
		case workflow.ActionNotify:
			detail = fmt.Sprintf("notify with message %q", action.Message)
		case workflow.ActionSetCategory:
			name := action.CategoryID
			for _, category := range categories {
				if category.ID == action.CategoryID {
					name = category.Name
				}
			}
			detail = fmt.Sprintf("set the triggering transaction's category to %q (category ID %s)", name, action.CategoryID)
		case workflow.ActionAddTag:
			detail = fmt.Sprintf("add tag %q to the triggering transaction", action.Tag)
		case workflow.ActionFlagReview:
			detail = fmt.Sprintf("add review tag %q to the triggering transaction", workflow.ReviewTag)
		}
		s += fmt.Sprintf("\n%d. %s", i+1, detail)
	}
	return s
}
