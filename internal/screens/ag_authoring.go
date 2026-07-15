// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsAuthoring(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
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

// agToolsAuthoring exposes natural-language rule authoring (AG3) and workflow
// authoring (AG4) as mutating assistant tools. The model turns the user's plain
// English into structured arguments; these tools resolve names to IDs, compile the
// typed domain object through the pure nlauthor helpers (a workflow condition is
// validated through the SAME formula language the engine runs, before saving), and
// preview the compiled result — with the "would affect N existing transactions"
// count for a rule — for the user's one-tap approval.
func agToolsAuthoring(app *appstate.App, base string, rates currency.Rates) []chatTool {
	cats := app.Categories()
	resolveCategory := func(name string) (domain.Category, bool) {
		q := strings.ToLower(strings.TrimSpace(name))
		for _, c := range cats {
			if strings.ToLower(c.Name) == q {
				return c, true
			}
		}
		for _, c := range cats {
			if q != "" && strings.Contains(strings.ToLower(c.Name), q) {
				return c, true
			}
		}
		return domain.Category{}, false
	}
	catNames := func() string {
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
	parseRule := func(raw json.RawMessage) (ruleArgs, rules.Rule, []string, bool) {
		var a ruleArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return a, rules.Rule{}, []string{"Couldn't read the rule details."}, false
		}
		catID := ""
		if a.Category != "" {
			if c, ok := resolveCategory(a.Category); ok {
				catID = c.ID
			} else {
				return a, rules.Rule{}, []string{"No category matching “" + a.Category + "”. Create it first. Existing: " + catNames() + "."}, false
			}
		}
		r, errs := nlauthor.CompileRule(nlauthor.RuleInput{
			ID: id.New(), Match: a.Match, SetCategoryID: catID,
			SetTags: a.Tags, RenameDesc: a.RenameDesc, Order: app.NextRuleOrder(),
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
			switch act.Kind {
			case workflow.ActionCreateTask:
				act.Title, act.Notes = strings.TrimSpace(a.Title), strings.TrimSpace(a.Message)
			case workflow.ActionNotify:
				act.Message = strings.TrimSpace(a.Message)
			case workflow.ActionSetCategory:
				if c, ok := resolveCategory(a.Category); ok {
					act.CategoryID = c.ID
				} else {
					errs = append(errs, "No category matching “"+a.Category+"” for the set-category action.")
				}
			case workflow.ActionAddTag:
				act.Tag = strings.TrimSpace(a.Tag)
			case workflow.ActionFlagReview, workflow.ActionApplyRules, workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
				// no extra fields
			default:
				errs = append(errs, "Unsupported action kind “"+a.Kind+"”.")
			}
			out = append(out, act)
		}
		return out, errs
	}
	parseWorkflow := func(raw json.RawMessage) (wfArgs, workflow.Workflow, []string, bool) {
		var a wfArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return a, workflow.Workflow{}, []string{"Couldn't read the workflow details."}, false
		}
		acts, actErrs := buildActions(a.Actions)
		trig := workflow.Trigger{Kind: workflow.TriggerKind(strings.TrimSpace(a.Trigger))}
		known := func(name string) bool { return false }
		names := app.WorkflowVariableNames()
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		known = func(name string) bool { return set[name] }
		wf, errs := nlauthor.CompileWorkflow(nlauthor.WorkflowInput{
			ID: id.New(), Name: a.Name, Trigger: trig, Condition: a.Condition, Actions: acts, Enabled: true,
		}, known)
		errs = append(actErrs, errs...)
		return a, wf, errs, len(errs) == 0
	}

	describeRule := func(r rules.Rule, catName string) string {
		var parts []string
		if r.SetCategoryID != "" {
			parts = append(parts, "categorize as "+catName)
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

	return []chatTool{
		{
			spec: ai.FunctionTool("create_rule",
				"Author a categorization RULE from a natural-language description. Give the match phrase (found in payee/description), the category to assign, optional tags, and an optional description rewrite. Previews the compiled rule and how many EXISTING transactions it would affect before applying (on approval it also files those existing matches).",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string","description":"case-insensitive phrase in the payee/description, e.g. Trader Joe's"},"category":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"rename_desc":{"type":"string","description":"optional: rewrite the matched transaction's description"}},"required":["match"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				a, r, errs, ok := parseRule(raw)
				if !ok {
					return "Can't create that rule: " + strings.Join(errs, " ")
				}
				n := nlauthor.RuleMatchCount(r, txnCtxs())
				return fmt.Sprintf("%s.\nWould affect %s existing transaction(s).", describeRule(r, strings.TrimSpace(a.Category)), plural(n, "matching"))
			},
			run: func(raw json.RawMessage) string {
				a, r, errs, ok := parseRule(raw)
				if !ok {
					return "Can't create that rule: " + strings.Join(errs, " ")
				}
				n := nlauthor.RuleMatchCount(r, txnCtxs())
				if err := app.PutRule(r); err != nil {
					return "Couldn't save the rule: " + err.Error()
				}
				return fmt.Sprintf("Created the rule: %s. It matches %d existing transaction(s). %s", describeRule(r, strings.TrimSpace(a.Category)), n, openLink("/rules", r.ID))
			},
		},
		{
			spec: ai.FunctionTool("create_workflow",
				"Author an automation WORKFLOW from a natural-language description. Choose a trigger (txn-added, scheduled, budget-exceeded, goal-reached, bill-due, manual), an optional condition in the formula language (e.g. txn_abs > 500 — validated before saving), and one or more actions (createTask, notify, setCategory, addTag, flagReview). Transaction-changing actions (setCategory/addTag/flagReview) require the txn-added trigger. Previews the compiled workflow before applying.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"trigger":{"type":"string","enum":["txn-added","scheduled","budget-exceeded","goal-reached","bill-due","manual"]},"condition":{"type":"string","description":"optional formula over engine variables + txn_* fields, e.g. txn_abs > 500"},"actions":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["createTask","notify","setCategory","addTag","flagReview"]},"title":{"type":"string"},"message":{"type":"string"},"category":{"type":"string"},"tag":{"type":"string"}},"required":["kind"]}}},"required":["name","trigger","actions"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				_, wf, errs, ok := parseWorkflow(raw)
				if !ok {
					return "Can't create that workflow: " + strings.Join(errs, " ")
				}
				return describeWorkflow(wf)
			},
			run: func(raw json.RawMessage) string {
				_, wf, errs, ok := parseWorkflow(raw)
				if !ok {
					return "Can't create that workflow: " + strings.Join(errs, " ")
				}
				if err := app.PutWorkflow(wf); err != nil {
					return "Couldn't save the workflow: " + err.Error()
				}
				return "Created the workflow: " + describeWorkflow(wf) + openLink("/workflows", wf.ID)
			},
		},
	}
}

// describeWorkflow renders a compiled workflow in the editor's own vocabulary, so
// the preview teaches the DSL by example (AG4).
func describeWorkflow(wf workflow.Workflow) string {
	kinds := make([]string, 0, len(wf.Actions))
	for _, a := range wf.Actions {
		kinds = append(kinds, string(a.Kind))
	}
	s := fmt.Sprintf("“%s” — WHEN %s", wf.Name, string(wf.Trigger.Kind))
	if wf.Condition != "" {
		s += " AND (" + wf.Condition + ")"
	}
	s += " THEN " + strings.Join(kinds, ", ")
	return s
}
