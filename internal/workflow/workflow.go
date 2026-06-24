// SPDX-License-Identifier: MIT

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
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
)

// TriggerKind is what causes a workflow to run.
type TriggerKind string

const (
	// TriggerManual runs only when the user clicks "Run now".
	TriggerManual TriggerKind = "manual"
	// TriggerTxnAdded runs after a transaction is added.
	TriggerTxnAdded TriggerKind = "txn-added"
	// TriggerScheduled runs automatically on a recurring cadence.
	TriggerScheduled TriggerKind = "scheduled"
	// TriggerBudgetExceeded runs when a budget transitions to over-limit.
	TriggerBudgetExceeded TriggerKind = "budget-exceeded"
	// TriggerGoalReached runs when a goal is fully funded.
	TriggerGoalReached TriggerKind = "goal-reached"
	// TriggerBillDue runs when a recurring bill is on or past its due date.
	TriggerBillDue TriggerKind = "bill-due"
)

// Trigger configures when a workflow runs.
type Trigger struct {
	Kind    TriggerKind             `json:"kind"`
	Cadence domain.RecurringCadence `json:"cadence,omitempty"`
	NextRun time.Time               `json:"nextRun,omitempty"`
}

// ActionKind is one effect a workflow can perform. The set is write-safe (no
// action creates transactions, so a txn-added workflow can't loop). SetCategory,
// AddTag, and FlagReview act on the transaction that triggered the workflow.
type ActionKind string

const (
	// ActionCreateTask creates a to-do task.
	ActionCreateTask ActionKind = "createTask"
	// ActionApplyRules categorizes uncategorized transactions via the rules engine.
	ActionApplyRules ActionKind = "applyRules"
	// ActionNotify shows the user a message (in-app notice).
	ActionNotify ActionKind = "notify"
	// ActionSetCategory sets the triggering transaction's category.
	ActionSetCategory ActionKind = "setCategory"
	// ActionAddTag adds a tag to the triggering transaction.
	ActionAddTag ActionKind = "addTag"
	// ActionFlagReview tags the triggering transaction for review.
	ActionFlagReview ActionKind = "flagReview"
	// ActionPostRecurring posts all due autopost recurring transactions.
	ActionPostRecurring ActionKind = "postRecurring"
	// ActionFlagBudgetOver creates tasks for every budget currently over its limit.
	ActionFlagBudgetOver ActionKind = "flagBudgetOver"
)

// ReviewTag is the tag ActionFlagReview adds.
const ReviewTag = "needs-review"

// Action is one step in a workflow. Fields are interpreted per Kind: CreateTask
// uses Title/Notes, Notify uses Message, SetCategory uses CategoryID, AddTag uses
// Tag; ApplyRules and FlagReview use none.
type Action struct {
	Kind       ActionKind `json:"kind"`
	Title      string     `json:"title,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Message    string     `json:"message,omitempty"`
	CategoryID string     `json:"categoryId,omitempty"`
	Tag        string     `json:"tag,omitempty"`
}

// Context is what a workflow is evaluated against: numeric variables (the engine
// surface plus, for a txn-added run, the triggering transaction's amount), string
// variables (the triggering transaction's payee/description/category/account), and
// the triggering transaction's id (empty for manual/aggregate runs). Per-
// transaction variables are prefixed "txn_".
type Context struct {
	Vars  map[string]float64
	Strs  map[string]string
	TxnID string
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
// TxnID is the transaction a transaction-mutating effect targets (empty otherwise).
type Effect struct {
	Kind       ActionKind `json:"kind"`
	Summary    string     `json:"summary"`
	Title      string     `json:"title,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Message    string     `json:"message,omitempty"`
	TxnID      string     `json:"txnId,omitempty"`
	CategoryID string     `json:"categoryId,omitempty"`
	Tag        string     `json:"tag,omitempty"`
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

// Eval evaluates a workflow condition over the context and returns whether it
// holds. An empty condition always holds. A boolean result is used directly; a
// number is truthy when non-zero; a string condition is an error (conditions must
// be logical). Deterministic — a thin wrapper over the sandbox.
func Eval(condition string, ctx Context) (bool, error) {
	if strings.TrimSpace(condition) == "" {
		return true, nil
	}
	v, err := formula.Eval(condition, formula.Env{Vars: ctx.Vars, Strs: ctx.Strs})
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

// Plan computes the Effects a workflow would produce given the current context,
// without performing them. It returns (effects, matched, error): matched is false
// (and effects nil) when the condition doesn't hold. This is the engine's core —
// the same planning powers both dry-run preview and a real run (the apply layer
// just executes the returned Effects). Pure and deterministic.
func Plan(wf Workflow, ctx Context) (effects []Effect, matched bool, err error) {
	ok, err := Eval(wf.Condition, ctx)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	for _, a := range wf.Actions {
		effects = append(effects, planAction(a, ctx))
	}
	return effects, true, nil
}

// planAction turns one action into its Effect, including a plain-English summary.
// Transaction-mutating effects carry the triggering transaction's id from ctx.
func planAction(a Action, ctx Context) Effect {
	e := Effect{Kind: a.Kind, Title: a.Title, Notes: a.Notes, Message: a.Message,
		CategoryID: a.CategoryID, Tag: a.Tag, TxnID: ctx.TxnID}
	switch a.Kind {
	case ActionCreateTask:
		e.Summary = "Create task: " + fallback(a.Title, "(untitled)")
	case ActionApplyRules:
		e.Summary = "Categorize uncategorized transactions with your rules"
	case ActionNotify:
		e.Summary = "Notify: " + a.Message
	case ActionSetCategory:
		e.Summary = "Set the transaction's category"
	case ActionAddTag:
		e.Summary = "Tag the transaction: " + a.Tag
	case ActionFlagReview:
		e.Tag = ReviewTag
		e.Summary = "Flag the transaction for review"
	case ActionPostRecurring:
		e.Summary = "Post all due autopost recurring transactions"
	case ActionFlagBudgetOver:
		e.Summary = "Create tasks for budgets over their limit"
	default:
		e.Summary = "Unknown action: " + string(a.Kind)
	}
	return e
}

// IsScheduledWorkflowDue reports whether a scheduled workflow's NextRun is on or
// before now. It returns false for any non-scheduled trigger.
func IsScheduledWorkflowDue(w Workflow, now time.Time) bool {
	if w.Trigger.Kind != TriggerScheduled {
		return false
	}
	return !w.Trigger.NextRun.After(now)
}

// AdvanceScheduledNextRun bumps w.Trigger.NextRun forward by the workflow's
// cadence until it is strictly after now, catching up any missed periods. It is
// a no-op for non-scheduled triggers. The guard cap (600) prevents an infinite
// loop on a misconfigured zero-interval cadence.
func AdvanceScheduledNextRun(w *Workflow, now time.Time) {
	if w.Trigger.Kind != TriggerScheduled {
		return
	}
	for guard := 0; !w.Trigger.NextRun.After(now) && guard < 600; guard++ {
		w.Trigger.NextRun = w.Trigger.Cadence.Next(w.Trigger.NextRun)
	}
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
	case TriggerManual, TriggerTxnAdded, TriggerScheduled, TriggerBudgetExceeded, TriggerGoalReached, TriggerBillDue:
	default:
		errs = append(errs, "Unknown trigger.")
	}
	if len(wf.Actions) == 0 {
		errs = append(errs, "Add at least one action.")
	}
	for _, a := range wf.Actions {
		switch a.Kind {
		case ActionCreateTask:
			if strings.TrimSpace(a.Title) == "" {
				errs = append(errs, "A \"create task\" action needs a title.")
			}
		case ActionSetCategory:
			if strings.TrimSpace(a.CategoryID) == "" {
				errs = append(errs, "A \"set category\" action needs a category.")
			}
		case ActionAddTag:
			if strings.TrimSpace(a.Tag) == "" {
				errs = append(errs, "An \"add tag\" action needs a tag.")
			}
		case ActionPostRecurring, ActionFlagBudgetOver: // no required fields
		}
	}
	return errs
}
