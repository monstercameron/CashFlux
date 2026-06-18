package workflow

import "testing"

func TestMatch(t *testing.T) {
	wf := Workflow{Trigger: Trigger{Kind: TriggerTxnAdded}}
	if !Match(wf.Trigger, TriggerTxnAdded) {
		t.Error("should match txn-added")
	}
	if Match(wf.Trigger, TriggerManual) {
		t.Error("should not match manual")
	}
}

func TestEval(t *testing.T) {
	vars := map[string]float64{"expense": 3000, "income": 2000}
	if ok, err := Eval("", vars); !ok || err != nil {
		t.Errorf("empty condition should hold: %v %v", ok, err)
	}
	if ok, err := Eval("expense > income", vars); !ok || err != nil {
		t.Errorf("expense>income should hold: %v %v", ok, err)
	}
	if ok, _ := Eval("income > expense", vars); ok {
		t.Error("income>expense should not hold")
	}
	// Numeric truthiness.
	if ok, _ := Eval("expense", vars); !ok {
		t.Error("non-zero number should be truthy")
	}
	if ok, _ := Eval("expense - expense", vars); ok {
		t.Error("zero should be falsy")
	}
	// Errors surface.
	if _, err := Eval("unknownvar > 1", vars); err == nil {
		t.Error("unknown var should error")
	}
}

func TestPlan(t *testing.T) {
	wf := Workflow{
		ID: "w1", Name: "Overspend alert", Trigger: Trigger{Kind: TriggerTxnAdded},
		Condition: "expense > income",
		Actions: []Action{
			{Kind: ActionCreateTask, Title: "Review spending", Notes: "You overspent"},
			{Kind: ActionApplyRules},
			{Kind: ActionNotify, Message: "Heads up: over budget"},
		},
	}
	// Condition holds → all actions planned.
	effs, matched, err := Plan(wf, map[string]float64{"expense": 3000, "income": 2000})
	if err != nil || !matched {
		t.Fatalf("expected match: %v %v", matched, err)
	}
	if len(effs) != 3 {
		t.Fatalf("want 3 effects, got %d", len(effs))
	}
	if effs[0].Kind != ActionCreateTask || effs[0].Title != "Review spending" ||
		effs[0].Summary != "Create task: Review spending" {
		t.Errorf("createTask effect wrong: %+v", effs[0])
	}
	if effs[2].Summary != "Notify: Heads up: over budget" {
		t.Errorf("notify summary wrong: %q", effs[2].Summary)
	}
	// Condition fails → no match, no effects.
	effs, matched, err = Plan(wf, map[string]float64{"expense": 1000, "income": 2000})
	if err != nil || matched || effs != nil {
		t.Errorf("expected no match: %v %v %v", matched, err, effs)
	}
	// Bad condition → error.
	if _, _, err := Plan(Workflow{Condition: "income +"}, map[string]float64{"income": 1}); err == nil {
		t.Error("bad condition should error")
	}
}

func TestValidate(t *testing.T) {
	good := Workflow{ID: "w", Name: "n", Trigger: Trigger{Kind: TriggerManual},
		Actions: []Action{{Kind: ActionApplyRules}}}
	if errs := Validate(good); errs != nil {
		t.Errorf("valid workflow flagged: %v", errs)
	}
	if errs := Validate(Workflow{Trigger: Trigger{Kind: "bogus"}}); len(errs) == 0 {
		t.Error("bogus workflow should be invalid")
	}
	// CreateTask without a title is invalid.
	noTitle := Workflow{ID: "w", Name: "n", Trigger: Trigger{Kind: TriggerManual},
		Actions: []Action{{Kind: ActionCreateTask}}}
	if errs := Validate(noTitle); len(errs) == 0 {
		t.Error("createTask without title should be invalid")
	}
}
