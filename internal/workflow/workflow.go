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
	"math"
	"strconv"
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
//
// ActionTransfer is a sanctioned exception: it does create transactions, but is
// loop-safe because (1) it is only valid on TriggerScheduled (not TriggerTxnAdded),
// enforced by ValidateTransferAction; and (2) it carries a DedupeKey so the same
// scheduled period transfers at most once even if RunTriggeredScheduled fires
// multiple times.
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
	// ActionTransfer moves money from one account to another via CreateTransferPair.
	// Loop-safe: only valid on TriggerScheduled; DedupeKey prevents double-execution
	// within the same period.
	ActionTransfer ActionKind = "transfer"
)

// ReviewTag is the tag ActionFlagReview adds.
const ReviewTag = "needs-review"

// Action is one step in a workflow. Fields are interpreted per Kind: CreateTask
// uses Title/Notes, Notify uses Message, SetCategory uses CategoryID, AddTag uses
// Tag; ApplyRules and FlagReview use none. Transfer uses TransferFromAccountID,
// TransferToAccountID, TransferAmount (minor units), and DedupeKey.
type Action struct {
	Kind       ActionKind `json:"kind"`
	Title      string     `json:"title,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Message    string     `json:"message,omitempty"`
	CategoryID string     `json:"categoryId,omitempty"`
	Tag        string     `json:"tag,omitempty"`
	// Transfer fields — only used when Kind == ActionTransfer.
	TransferFromAccountID string `json:"transferFromAccountId,omitempty"`
	TransferToAccountID   string `json:"transferToAccountId,omitempty"`
	// TransferAmount is the transfer amount in the source account's minor units (must be > 0).
	TransferAmount int64 `json:"transferAmount,omitempty"`
	// DedupeKey is an opaque string that scopes the transfer to at most one execution
	// per period (e.g. "pyf:wf-abc:2026-06"). The apply layer skips the transfer if
	// a prior run for the same workflow already carried this key.
	DedupeKey string `json:"dedupeKey,omitempty"`
	// ResolveCondition, on an ActionCreateTask, is an optional formula-language
	// condition that makes the created task self-resolving (XC8): the task
	// auto-completes when the condition holds against a later data event. Empty
	// leaves the task manual.
	ResolveCondition string `json:"resolveCondition,omitempty"`
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
// Transfer fields are populated when Kind == ActionTransfer.
type Effect struct {
	Kind       ActionKind `json:"kind"`
	Summary    string     `json:"summary"`
	Title      string     `json:"title,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Message    string     `json:"message,omitempty"`
	TxnID      string     `json:"txnId,omitempty"`
	CategoryID string     `json:"categoryId,omitempty"`
	Tag        string     `json:"tag,omitempty"`
	// Transfer fields — only populated when Kind == ActionTransfer.
	TransferFromAccountID string `json:"transferFromAccountId,omitempty"`
	TransferToAccountID   string `json:"transferToAccountId,omitempty"`
	TransferAmount        int64  `json:"transferAmount,omitempty"`
	DedupeKey             string `json:"dedupeKey,omitempty"`
	// ResolveCondition carries an ActionCreateTask's self-resolve condition
	// through to the apply layer (XC8), which stamps it onto the created task.
	ResolveCondition string `json:"resolveCondition,omitempty"`
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

// Expand interpolates {{expr}} templates in s against the run context: each
// template is evaluated as a sandboxed formula over the engine variables (and,
// for txn-added runs, the txn_* fields), so a task title or notify message can
// carry live figures — "Safe to spend is {{safe_to_spend}}" or
// "{{txn_payee}} charged {{txn_abs}}". Numbers render rounded to cents; an
// expression that fails to evaluate is left as typed (visible, not swallowed).
func Expand(s string, ctx Context) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			b.WriteString(s)
			break
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		raw := s[i : i+j+2]
		expr := strings.TrimSpace(s[i+2 : i+j])
		out := raw
		if expr != "" {
			if v, err := formula.Eval(expr, formula.Env{Vars: ctx.Vars, Strs: ctx.Strs}); err == nil {
				switch n := v.(type) {
				case float64:
					out = strconv.FormatFloat(math.Round(n*100)/100, 'f', -1, 64)
				case bool:
					out = strconv.FormatBool(n)
				case string:
					out = n
				}
			}
		}
		b.WriteString(out)
		s = s[i+j+2:]
	}
	return b.String()
}

// planAction turns one action into its Effect, including a plain-English summary.
// Transaction-mutating effects carry the triggering transaction's id from ctx.
// The free-text fields (task title/notes, notify message) pass through Expand so
// {{expr}} templates resolve against the live context.
func planAction(a Action, ctx Context) Effect {
	e := Effect{Kind: a.Kind, Title: Expand(a.Title, ctx), Notes: Expand(a.Notes, ctx),
		Message: Expand(a.Message, ctx), CategoryID: a.CategoryID, Tag: a.Tag, TxnID: ctx.TxnID,
		ResolveCondition: a.ResolveCondition}
	switch a.Kind {
	case ActionCreateTask:
		e.Summary = "Create task: " + fallback(e.Title, "(untitled)")
	case ActionApplyRules:
		e.Summary = "Categorize uncategorized transactions with your rules"
	case ActionNotify:
		e.Summary = "Notify: " + e.Message
	case ActionSetCategory:
		e.Summary = "Set the transaction's category" + noTxnNote(ctx)
	case ActionAddTag:
		e.Summary = "Tag the transaction: " + a.Tag + noTxnNote(ctx)
	case ActionFlagReview:
		e.Tag = ReviewTag
		e.Summary = "Flag the transaction for review" + noTxnNote(ctx)
	case ActionPostRecurring:
		e.Summary = "Post all due autopost recurring transactions"
	case ActionFlagBudgetOver:
		e.Summary = "Create tasks for budgets over their limit"
	case ActionTransfer:
		e.TransferFromAccountID = a.TransferFromAccountID
		e.TransferToAccountID = a.TransferToAccountID
		e.TransferAmount = a.TransferAmount
		e.DedupeKey = a.DedupeKey
		e.Summary = fmt.Sprintf("Transfer %d minor units from %s to %s",
			a.TransferAmount, a.TransferFromAccountID, a.TransferToAccountID)
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

// ValidateTransferAction checks that a is a well-formed ActionTransfer and that
// it is safe to execute given triggerKind. It returns a non-nil error if:
//   - TransferFromAccountID is empty
//   - TransferToAccountID is empty
//   - TransferAmount is zero or negative
//   - triggerKind is TriggerTxnAdded (would cause a recursion loop because the
//     transfer legs are themselves transactions)
func ValidateTransferAction(a Action, triggerKind TriggerKind) error {
	if a.TransferFromAccountID == "" {
		return fmt.Errorf("workflow: transfer action requires a source account (TransferFromAccountID)")
	}
	if a.TransferToAccountID == "" {
		return fmt.Errorf("workflow: transfer action requires a destination account (TransferToAccountID)")
	}
	if a.TransferAmount <= 0 {
		return fmt.Errorf("workflow: transfer amount must be positive, got %d", a.TransferAmount)
	}
	if triggerKind == TriggerTxnAdded {
		return fmt.Errorf("workflow: ActionTransfer is not permitted on TriggerTxnAdded (would loop — transfer legs are transactions)")
	}
	return nil
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// noTxnNote marks a transaction-mutating effect that has no transaction in
// scope (a manual/scheduled/aggregate run) — the apply layer will no-op, and
// the summary must say so rather than reporting a mutation that never happens.
func noTxnNote(ctx Context) string {
	if ctx.TxnID == "" {
		return " — skipped: no transaction in scope for this run"
	}
	return ""
}

// txnOnlyActions are the actions that mutate the TRIGGERING transaction and
// therefore only make sense on the transaction-added trigger.
var txnOnlyActions = map[ActionKind]bool{
	ActionSetCategory: true,
	ActionAddTag:      true,
	ActionFlagReview:  true,
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
	txnOnlyFlagged := false
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
		case ActionTransfer:
			if err := ValidateTransferAction(a, wf.Trigger.Kind); err != nil {
				errs = append(errs, err.Error())
			}
		case ActionPostRecurring, ActionFlagBudgetOver: // no required fields
		}
		// Transaction-mutating actions act on the transaction that fired the
		// workflow; on any other trigger there is none and they would silently
		// no-op — refuse the combination at save instead.
		if txnOnlyActions[a.Kind] && wf.Trigger.Kind != TriggerTxnAdded && !txnOnlyFlagged {
			errs = append(errs, "Set category / add tag / flag-for-review actions change the transaction that fired the workflow — they need the \"When a transaction is added\" trigger.")
			txnOnlyFlagged = true
		}
	}
	return errs
}
