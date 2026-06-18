// Package workflow is the pure automation engine: the model for a user's
// workflow (a trigger, an optional condition, and ordered actions) plus the
// deterministic, side-effect-free planning that turns a workflow into a list of
// Effects. Matching a trigger to an event and evaluating the condition are pure
// functions over the engine variable surface, so they unit-test on native Go.
// Applying the planned Effects (the only place state changes) lives in the wasm
// app layer, which keeps the engine explainable and dry-runnable.
package workflow

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// TriggerKind is what causes a workflow to run.
type TriggerKind string

const (
	// TriggerManual runs only when the user clicks "Run now".
	TriggerManual TriggerKind = "manual"
	// TriggerTxnAdded runs after a transaction is added.
	TriggerTxnAdded TriggerKind = "txn-added"
)

// Trigger configures when a workflow runs.
type Trigger struct {
	Kind TriggerKind `json:"kind"`
}

// ActionKind is one effect a workflow can perform. The set is deliberately small
// and write-safe (no action creates transactions, so txn-added can't loop).
type ActionKind string

const (
	// ActionCreateTask creates a to-do task.
	ActionCreateTask ActionKind = "createTask"
	// ActionApplyRules categorizes uncategorized transactions via the rules engine.
	ActionApplyRules ActionKind = "applyRules"
	// ActionNotify records a message (surfaced in the run result / as a notice).
	ActionNotify ActionKind = "notify"
)

// Action is one step in a workflow. Fields are interpreted per Kind: CreateTask
// uses Title/Notes, Notify uses Message, ApplyRules uses none.
type Action struct {
	Kind    ActionKind `json:"kind"`
	Title   string     `json:"title,omitempty"`
	Notes   string     `json:"notes,omitempty"`
	Message string     `json:"message,omitempty"`
}

// Workflow is a user-defined automation: when Trigger fires and Condition holds,
// the Actions run in order. Condition is an optional sandboxed formula over the
// engine variables (empty = always). Enabled gates automatic (trigger) runs;
// a disabled workflow can still be run manually.
type Workflow struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled,omitempty"`
	Trigger   Trigger  `json:"trigger"`
	Condition string   `json:"condition,omitempty"`
	Actions   []Action `json:"actions,omitempty"`
}

// Effect is the planned result of one action: a human-readable summary (for the
// dry-run preview and the run log) plus the typed fields the apply layer needs.
type Effect struct {
	Kind    ActionKind `json:"kind"`
	Summary string     `json:"summary"`
	Title   string     `json:"title,omitempty"`
	Notes   string     `json:"notes,omitempty"`
	Message string     `json:"message,omitempty"`
}

// Run is the audit record of one workflow execution: what it did (or would do, if
// DryRun) and when. At is supplied by the caller (the engine has no clock).
type Run struct {
	ID         string   `json:"id"`
	WorkflowID string   `json:"workflowId"`
	At         string   `json:"at"`
	DryRun     bool     `json:"dryRun,omitempty"`
	Matched    bool     `json:"matched"`
	Effects    []Effect `json:"effects,omitempty"`
}

// Match reports whether a workflow's trigger fires for the given event kind.
func Match(t Trigger, event TriggerKind) bool {
	return t.Kind == event
}

// Eval evaluates a workflow condition over the variable surface and returns
// whether it holds. An empty condition always holds. A boolean result is used
// directly; a number is truthy when non-zero; a string condition is an error
// (conditions must be logical). Deterministic — a thin wrapper over the sandbox.
func Eval(condition string, vars map[string]float64) (bool, error) {
	if strings.TrimSpace(condition) == "" {
		return true, nil
	}
	v, err := formula.Eval(condition, formula.Env{Vars: vars})
	if err != nil {
		return false, err
	}
	switch n := v.(type) {
	case bool:
		return n, nil
	case float64:
		return n != 0, nil
	default:
		return false, fmt.Errorf("workflow: condition must be true/false, got %T", v)
	}
}

// Plan computes the Effects a workflow would produce given the current variables,
// without performing them. It returns (effects, matched, error): matched is false
// (and effects nil) when the condition doesn't hold. This is the engine's core —
// the same planning powers both dry-run preview and a real run (the apply layer
// just executes the returned Effects). Pure and deterministic.
func Plan(wf Workflow, vars map[string]float64) (effects []Effect, matched bool, err error) {
	ok, err := Eval(wf.Condition, vars)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	for _, a := range wf.Actions {
		effects = append(effects, planAction(a))
	}
	return effects, true, nil
}

// planAction turns one action into its Effect, including a plain-English summary.
func planAction(a Action) Effect {
	e := Effect{Kind: a.Kind, Title: a.Title, Notes: a.Notes, Message: a.Message}
	switch a.Kind {
	case ActionCreateTask:
		e.Summary = "Create task: " + fallback(a.Title, "(untitled)")
	case ActionApplyRules:
		e.Summary = "Categorize uncategorized transactions with your rules"
	case ActionNotify:
		e.Summary = "Notify: " + a.Message
	default:
		e.Summary = "Unknown action: " + string(a.Kind)
	}
	return e
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// Validate reports problems with a workflow, or nil if valid: it needs an ID, a
// name, a known trigger, and at least one action; CreateTask actions need a title.
func Validate(wf Workflow) []string {
	var errs []string
	if wf.ID == "" {
		errs = append(errs, "A workflow needs an id.")
	}
	if strings.TrimSpace(wf.Name) == "" {
		errs = append(errs, "A workflow needs a name.")
	}
	switch wf.Trigger.Kind {
	case TriggerManual, TriggerTxnAdded:
	default:
		errs = append(errs, "Unknown trigger.")
	}
	if len(wf.Actions) == 0 {
		errs = append(errs, "Add at least one action.")
	}
	for _, a := range wf.Actions {
		if a.Kind == ActionCreateTask && strings.TrimSpace(a.Title) == "" {
			errs = append(errs, "A \"create task\" action needs a title.")
		}
	}
	return errs
}
