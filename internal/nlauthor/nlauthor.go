// SPDX-License-Identifier: MIT

// Package nlauthor compiles natural-language authoring requests into the app's
// validated domain objects — a categorization Rule (AG3) or an automation
// Workflow (AG4). The assistant's tools parse the model's structured arguments,
// resolve names to IDs, and hand the resolved pieces here; this package assembles
// the typed object and validates it (a workflow condition through the SAME
// formula language the engine evaluates, so an invalid condition is rejected
// before it can ever be saved).
//
// Pure Go, no syscall/js — unit-tested natively. It never persists anything and
// never assigns identity beyond what the caller passes in; the wasm layer owns id
// generation, name resolution, and the approval/preview loop.
package nlauthor

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// RuleInput is the resolved shape of a natural-language rule request: the match
// phrase (or structured conditions), the category the caller already resolved to
// an ID, optional tags and a description rewrite, and the rule's precedence order.
type RuleInput struct {
	ID            string
	Match         string
	Conditions    []rules.RuleCondition
	SetCategoryID string
	SetTags       []string
	RenameDesc    string
	Order         int
}

// CompileRule assembles a rules.Rule from resolved input and validates it,
// returning the rule and a list of plain-English problems (empty when valid). A
// rule needs a way to MATCH (a phrase or at least one structured condition) and
// at least one EFFECT (set a category, add tags, or rewrite the description);
// otherwise it would silently do nothing.
func CompileRule(in RuleInput) (rules.Rule, []string) {
	r := rules.Rule{
		ID:            strings.TrimSpace(in.ID),
		Match:         strings.TrimSpace(in.Match),
		SetCategoryID: strings.TrimSpace(in.SetCategoryID),
		SetTags:       cleanTags(in.SetTags),
		RenameDesc:    strings.TrimSpace(in.RenameDesc),
		Order:         in.Order,
		Conditions:    in.Conditions,
	}
	var errs []string
	if r.ID == "" {
		errs = append(errs, "A rule needs an id.")
	}
	if r.Match == "" && len(r.Conditions) == 0 {
		errs = append(errs, "A rule needs a phrase to match (e.g. \"Trader Joe's\") or at least one condition.")
	}
	if r.SetCategoryID == "" && len(r.SetTags) == 0 && r.RenameDesc == "" {
		errs = append(errs, "A rule needs an action: set a category, add a tag, or rewrite the description.")
	}
	return r, errs
}

// RuleMatchCount reports how many of the given transaction contexts the compiled
// rule would catch — the "would affect N existing transactions" preview (AG3).
// It reuses the engine's own matcher so the count can never disagree with live
// application.
func RuleMatchCount(r rules.Rule, txns []rules.TxnCtx) int { return r.MatchCountFull(txns) }

// WorkflowInput is the resolved shape of a natural-language workflow request.
// Condition is an optional formula-language expression (the engine's condition
// language); Actions are already resolved to typed workflow.Action values.
type WorkflowInput struct {
	ID        string
	Name      string
	Trigger   workflow.Trigger
	Condition string
	Actions   []workflow.Action
	Enabled   bool
}

// CompileWorkflow assembles a workflow.Workflow from resolved input, validating
// its condition through formula.Validate (against the known-variable predicate)
// BEFORE building — the engine's conditions ARE the formula language, so an
// unknown variable or a syntax error is caught here rather than at run time — and
// then running the workflow engine's own structural Validate. known reports
// whether a variable name is part of the live engine surface (pass
// app.WorkflowVariableNames as a set); a nil known skips the reference check but
// still parses the expression. Returns the workflow and any plain-English
// problems (empty when valid and safe to save).
func CompileWorkflow(in WorkflowInput, known func(name string) bool) (workflow.Workflow, []string) {
	wf := workflow.Workflow{
		ID:        strings.TrimSpace(in.ID),
		Name:      strings.TrimSpace(in.Name),
		Enabled:   in.Enabled,
		Trigger:   in.Trigger,
		Condition: strings.TrimSpace(in.Condition),
		Actions:   in.Actions,
	}
	var errs []string
	if cond := wf.Condition; cond != "" {
		if err := formula.Validate(cond, known); err != nil {
			errs = append(errs, "The condition isn't valid: "+err.Error())
		}
	}
	errs = append(errs, workflow.Validate(wf)...)
	return wf, errs
}

// cleanTags trims, drops blanks, and de-duplicates a tag list, preserving order.
func cleanTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}
