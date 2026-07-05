// SPDX-License-Identifier: MIT

package workflow

import "testing"

// TestExpand covers the {{expr}} template interpolation in action free-text:
// numeric formulas (rounded to cents), string variables, booleans, multiple
// templates, unresolvable expressions left as typed, and malformed braces.
func TestExpand(t *testing.T) {
	ctx := Context{
		Vars: map[string]float64{"safe_to_spend": 1234.5678, "income": 4700, "expense": 3200},
		Strs: map[string]string{"txn_payee": "Blue Bottle"},
	}
	cases := []struct {
		name, in, want string
	}{
		{"no templates", "Check the budget", "Check the budget"},
		{"number rounds to cents", "Left: {{safe_to_spend}}", "Left: 1234.57"},
		{"formula", "Net: {{income - expense}}", "Net: 1500"},
		{"string var", "{{txn_payee}} charged you", "Blue Bottle charged you"},
		{"two templates", "{{txn_payee}}: {{income}}", "Blue Bottle: 4700"},
		{"bool", "over? {{income > expense}}", "over? true"},
		{"unresolvable stays visible", "x {{no_such_var}} y", "x {{no_such_var}} y"},
		{"empty expr stays", "a {{}} b", "a {{}} b"},
		{"unclosed braces pass through", "a {{income b", "a {{income b"},
		{"adjacent text intact", "pre{{income}}post", "pre4700post"},
	}
	for _, c := range cases {
		if got := Expand(c.in, ctx); got != c.want {
			t.Errorf("%s: Expand(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestPlanExpandsActionText proves the templates reach the planned effects (the
// same path both dry-run previews and real runs use).
func TestPlanExpandsActionText(t *testing.T) {
	wf := Workflow{
		ID: "w1", Name: "n", Enabled: true,
		Trigger: Trigger{Kind: TriggerManual},
		Actions: []Action{
			{Kind: ActionCreateTask, Title: "Review — safe to spend {{safe_to_spend}}"},
			{Kind: ActionNotify, Message: "Spent {{expense}} of {{income}}"},
		},
	}
	ctx := Context{Vars: map[string]float64{"safe_to_spend": 88.4, "income": 100, "expense": 40}}
	effects, matched, err := Plan(wf, ctx)
	if err != nil || !matched || len(effects) != 2 {
		t.Fatalf("Plan = %v effects, matched=%v, err=%v", len(effects), matched, err)
	}
	if effects[0].Title != "Review — safe to spend 88.4" {
		t.Errorf("task title = %q", effects[0].Title)
	}
	if effects[0].Summary != "Create task: Review — safe to spend 88.4" {
		t.Errorf("task summary = %q", effects[0].Summary)
	}
	if effects[1].Message != "Spent 40 of 100" {
		t.Errorf("notify message = %q", effects[1].Message)
	}
}
