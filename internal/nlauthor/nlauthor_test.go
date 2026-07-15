// SPDX-License-Identifier: MIT

package nlauthor

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func TestCompileRuleValid(t *testing.T) {
	r, errs := CompileRule(RuleInput{
		ID: "r1", Match: "Trader Joe's", SetCategoryID: "cat-groceries",
		SetTags: []string{"errands", "errands", " "}, Order: 3,
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if r.Match != "Trader Joe's" || r.SetCategoryID != "cat-groceries" || r.Order != 3 {
		t.Fatalf("rule = %+v", r)
	}
	if len(r.SetTags) != 1 || r.SetTags[0] != "errands" {
		t.Fatalf("tags not cleaned: %v", r.SetTags)
	}
}

func TestCompileRuleRejectsNoMatchAndNoAction(t *testing.T) {
	_, errs := CompileRule(RuleInput{ID: "r1"})
	joined := strings.Join(errs, " | ")
	if !strings.Contains(joined, "phrase to match") || !strings.Contains(joined, "needs an action") {
		t.Fatalf("expected match+action errors, got: %v", errs)
	}
}

func TestRuleMatchCount(t *testing.T) {
	r, _ := CompileRule(RuleInput{ID: "r1", Match: "coffee", SetCategoryID: "c"})
	txns := []rules.TxnCtx{
		{Payee: "Blue Bottle Coffee", Desc: "latte"},
		{Payee: "Shell", Desc: "gas"},
		{Payee: "Coffee shop", Desc: ""},
	}
	if n := RuleMatchCount(r, txns); n != 2 {
		t.Fatalf("match count = %d, want 2", n)
	}
}

func TestCompileWorkflowValidatesCondition(t *testing.T) {
	known := func(name string) bool { return name == "txn_abs" }
	in := WorkflowInput{
		ID: "w1", Name: "Big spend review", Enabled: true,
		Trigger:   workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Condition: "txn_abs > 500",
		Actions:   []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Review large charge"}},
	}
	wf, errs := CompileWorkflow(in, known)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Condition != "txn_abs > 500" || wf.Trigger.Kind != workflow.TriggerTxnAdded {
		t.Fatalf("workflow = %+v", wf)
	}
}

func TestCompileWorkflowRejectsUnknownVariable(t *testing.T) {
	known := func(name string) bool { return false }
	in := WorkflowInput{
		ID: "w1", Name: "x",
		Trigger:   workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Condition: "nonsense_var > 1",
		Actions:   []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "t"}},
	}
	_, errs := CompileWorkflow(in, known)
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, " "), "condition isn't valid") {
		t.Fatalf("expected condition error, got: %v", errs)
	}
}

func TestCompileWorkflowRejectsSyntaxError(t *testing.T) {
	in := WorkflowInput{
		ID: "w1", Name: "x",
		Trigger:   workflow.Trigger{Kind: workflow.TriggerScheduled},
		Condition: "income >",
		Actions:   []workflow.Action{{Kind: workflow.ActionNotify, Message: "hi"}},
	}
	_, errs := CompileWorkflow(in, nil) // nil known still parses
	if len(errs) == 0 {
		t.Fatalf("expected syntax error")
	}
}

func TestCompileWorkflowStructuralValidation(t *testing.T) {
	in := WorkflowInput{
		ID: "w1", Name: "x",
		Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		// no actions -> workflow.Validate should complain
	}
	_, errs := CompileWorkflow(in, nil)
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, " "), "at least one action") {
		t.Fatalf("expected structural error, got: %v", errs)
	}
}
