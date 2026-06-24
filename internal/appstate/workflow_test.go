// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/workflow"
)

// TestRunWorkflowApplies proves the apply path: a matching workflow's actions
// actually change state (a task is created) and the run is recorded, while a dry
// run plans the same effects without changing anything.
func TestRunWorkflowApplies(t *testing.T) {
	a := newApp(t, false)
	wf := workflow.Workflow{
		ID: "wf1", Name: "Always create a task", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		// No condition → always matches.
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Do the thing"}},
	}

	// Dry run: plans the effect but creates nothing and records nothing.
	dry, err := a.RunWorkflow(wf, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dry.Matched || len(dry.Effects) != 1 || !dry.DryRun {
		t.Errorf("dry run wrong: %+v", dry)
	}
	if len(a.Tasks()) != 0 {
		t.Errorf("dry run created tasks: %d", len(a.Tasks()))
	}
	if len(a.WorkflowRuns()) != 0 {
		t.Errorf("dry run recorded a run: %d", len(a.WorkflowRuns()))
	}

	// Real run: creates the task and records the run.
	got, err := a.RunWorkflow(wf, false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !got.Matched || len(got.Effects) != 1 {
		t.Errorf("run wrong: %+v", got)
	}
	tasks := a.Tasks()
	if len(tasks) != 1 || tasks[0].Title != "Do the thing" {
		t.Fatalf("expected one created task, got %+v", tasks)
	}
	if len(a.WorkflowRuns()) != 1 {
		t.Errorf("expected one recorded run, got %d", len(a.WorkflowRuns()))
	}
}

// TestRunWorkflowCondition verifies a condition gates execution.
func TestRunWorkflowCondition(t *testing.T) {
	a := newApp(t, false)
	// With no data, income and expense are both 0, so "expense > income" is false.
	wf := workflow.Workflow{
		ID: "wf2", Name: "Conditional", Enabled: true,
		Trigger:   workflow.Trigger{Kind: workflow.TriggerManual},
		Condition: "expense > income",
		Actions:   []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Nope"}},
	}
	run, err := a.RunWorkflow(wf, false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Matched {
		t.Error("condition should not have matched on empty data")
	}
	if len(a.Tasks()) != 0 {
		t.Error("no task should be created when the condition fails")
	}
}
