package mermaid

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/workflow"
)

func TestEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{`Coffee`, `Coffee`},
		{`Say "hi"`, `Say 'hi'`}, // quotes can't break the label
		{`<script>alert(1)</script>`, `&lt;script&gt;alert(1)&lt;/script&gt;`}, // angle brackets entity-escaped (no raw tag)
		{"line1\nline2", "line1 line2"},                                        // newlines collapse to a space
		{"a\t\t b", "a b"},                                                     // tabs/runs collapse, trimmed
		{"  pad  ", "pad"},                                                     // trimmed
	}
	for _, c := range cases {
		if got := Escape(c.in); got != c.want {
			t.Errorf("Escape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFlowchartBuilder(t *testing.T) {
	src := NewFlowchart("").
		Node("a", "Start", ShapeRound).
		Node("b", "Decide?", ShapeDiamond).
		Edge("a", "b", "go").
		String()
	for _, want := range []string{
		"flowchart TD\n", // default direction
		`a("Start")`,     // round
		`b{"Decide?"}`,   // diamond
		`a -->|"go"| b`,  // labelled edge
	} {
		if !strings.Contains(src, want) {
			t.Errorf("flowchart missing %q in:\n%s", want, src)
		}
	}
}

func TestFromWorkflow(t *testing.T) {
	w := workflow.Workflow{
		Name:      "Tidy",
		Trigger:   workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Condition: "amount > 100",
		Actions: []workflow.Action{
			{Kind: workflow.ActionFlagReview},
			{Kind: workflow.ActionAddTag, Tag: "big"},
		},
	}
	src := FromWorkflow(w)
	for _, want := range []string{
		`trig("When a transaction is added")`, // trigger terminal
		`cond{"if amount &gt; 100"}`,          // condition diamond (operator entity-escaped)
		`trig --> cond`,                       // trigger → condition
		`cond -->|"yes"| a0`,                  // condition's yes-path to first action
		`a0["Flag for review"]`,
		`a1["Add tag: big"]`,
		`a0 --> a1`, // actions chain in order
	} {
		if !strings.Contains(src, want) {
			t.Errorf("workflow flowchart missing %q in:\n%s", want, src)
		}
	}

	// No condition → trigger links straight to the first action (no "yes" edge).
	src2 := FromWorkflow(workflow.Workflow{
		Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		Actions: []workflow.Action{{Kind: workflow.ActionApplyRules}},
	})
	if !strings.Contains(src2, `trig --> a0`) {
		t.Errorf("conditionless workflow should link trigger straight to a0:\n%s", src2)
	}
	if strings.Contains(src2, "cond") {
		t.Errorf("conditionless workflow should have no condition node:\n%s", src2)
	}
}
