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
	ctx := Context{Vars: map[string]float64{"expense": 3000, "income": 2000}}
	if ok, err := Eval("", ctx); !ok || err != nil {
		t.Errorf("empty condition should hold: %v %v", ok, err)
	}
	if ok, err := Eval("expense > income", ctx); !ok || err != nil {
		t.Errorf("expense>income should hold: %v %v", ok, err)
	}
	if ok, _ := Eval("income > expense", ctx); ok {
		t.Error("income>expense should not hold")
	}
	if ok, _ := Eval("expense", ctx); !ok {
		t.Error("non-zero number should be truthy")
	}
	if ok, _ := Eval("expense - expense", ctx); ok {
		t.Error("zero should be falsy")
	}
	if _, err := Eval("unknownvar > 1", ctx); err == nil {
		t.Error("unknown var should error")
	}
}

// Per-transaction conditions: amount + string matching on payee/category — the
// thing that makes txn-added genuinely useful.
func TestEvalPerTransaction(t *testing.T) {
	ctx := Context{
		Vars: map[string]float64{"txn_amount": -250, "txn_abs": 250},
		Strs: map[string]string{"txn_payee": "BISTRO ROMA", "txn_category": "Dining"},
	}
	cases := []struct {
		cond string
		want bool
	}{
		{"txn_abs > 200", true},
		{"txn_abs > 500", false},
		{`txn_category == "Dining"`, true},
		{`txn_category == "Groceries"`, false},
		{`contains(txn_payee, "bistro")`, true}, // case-insensitive
		{`txn_amount < 0`, true},                // it's an expense
	}
	for _, c := range cases {
		got, err := Eval(c.cond, ctx)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", c.cond, err)
			continue
		}
		if got != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.cond, got, c.want)
		}
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
	effs, matched, err := Plan(wf, Context{Vars: map[string]float64{"expense": 3000, "income": 2000}})
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
	effs, matched, err = Plan(wf, Context{Vars: map[string]float64{"expense": 1000, "income": 2000}})
	if err != nil || matched || effs != nil {
		t.Errorf("expected no match: %v %v %v", matched, err, effs)
	}
	// Bad condition → error.
	if _, _, err := Plan(Workflow{Condition: "income +"}, Context{Vars: map[string]float64{"income": 1}}); err == nil {
		t.Error("bad condition should error")
	}
}

// Transaction-mutating actions carry the triggering transaction id and resolved
// fields from the context.
func TestPlanTxnActions(t *testing.T) {
	wf := Workflow{
		ID: "w", Name: "Route dining", Trigger: Trigger{Kind: TriggerTxnAdded},
		Actions: []Action{
			{Kind: ActionSetCategory, CategoryID: "cat-dining"},
			{Kind: ActionAddTag, Tag: "eating-out"},
			{Kind: ActionFlagReview},
		},
	}
	effs, matched, err := Plan(wf, Context{TxnID: "txn-1", Vars: map[string]float64{}})
	if err != nil || !matched || len(effs) != 3 {
		t.Fatalf("plan: %v %v %d", err, matched, len(effs))
	}
	if effs[0].Kind != ActionSetCategory || effs[0].TxnID != "txn-1" || effs[0].CategoryID != "cat-dining" {
		t.Errorf("setCategory effect wrong: %+v", effs[0])
	}
	if effs[1].TxnID != "txn-1" || effs[1].Tag != "eating-out" {
		t.Errorf("addTag effect wrong: %+v", effs[1])
	}
	if effs[2].Tag != ReviewTag || effs[2].TxnID != "txn-1" {
		t.Errorf("flagReview effect wrong: %+v", effs[2])
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
