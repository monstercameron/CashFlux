// SPDX-License-Identifier: MIT

package appstate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// ImportCommitProgress describes preparation and the durable save separately.
// Completed counts reviewed source rows; saving is not a success acknowledgement.
type ImportCommitProgress struct {
	Phase     string
	Completed int
	Total     int
}

// CommitImportBatch writes the approved staged model through the bulk path. Like
// Commit, its return means SQLite committed; browser callers must use
// CommitImportBatchDurable to wait for the generation-checked strict save.
func (a *App) CommitImportBatch(batch StagedImport, documentID, checkpointID string) (CSVImportResult, error) {
	return a.commitImportBatch(batch, documentID, checkpointID, nil, nil, a.publishImportNotices)
}

func (a *App) publishImportNotices(notices []string) {
	if a.Notifier != nil && len(notices) > 0 {
		a.Notifier(strings.Join(notices, "\n"))
	}
}

// CommitImportBatchDurable yields before preparing the batch, reports progress,
// and acknowledges success only after RequestTransactionsDurable completes.
// A failed durable save retains the SQL rows: retry RequestDurable, not import.
func (a *App) CommitImportBatchDurable(batch StagedImport, documentID, checkpointID string, progress func(ImportCommitProgress), done func(CSVImportResult, error)) {
	if done == nil {
		return
	}
	if a.durableWriter == nil {
		done(CSVImportResult{}, ErrDurabilityUnavailable)
		return
	}
	if progress != nil {
		progress(ImportCommitProgress{Phase: "preparing", Total: len(batch.Rows)})
	}
	go func() {
		time.Sleep(time.Millisecond) // paint the pending state before any work
		var notices []string
		result, err := a.commitImportBatch(batch, documentID, checkpointID, progress, func() { time.Sleep(time.Millisecond) }, func(messages []string) { notices = messages })
		if err != nil {
			done(result, err)
			return
		}
		if progress != nil {
			progress(ImportCommitProgress{Phase: "saving", Completed: len(batch.Rows), Total: len(batch.Rows)})
		}
		ids := make([]string, 0, result.Imported)
		for _, outcome := range result.Outcomes {
			if outcome.Status == "imported" {
				ids = append(ids, outcome.TransactionID)
			}
		}
		a.RequestTransactionsDurable(ids, func(durable DurableResult) {
			if durable.Err != nil {
				done(result, fmt.Errorf("save imported rows: %w", durable.Err))
				return
			}
			if progress != nil {
				progress(ImportCommitProgress{Phase: "saved", Completed: len(batch.Rows), Total: len(batch.Rows)})
			}
			a.publishImportNotices(notices)
			done(result, nil)
		})
	}()
}

func (a *App) checkImportBatch(batch StagedImport) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	seal, err := importSeal(batch)
	if err != nil {
		return err
	}
	ledger, err := a.reads.transactions.get(a.store)
	if err != nil {
		return fmt.Errorf("read import ledger: %w", err)
	}
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return fmt.Errorf("read import accounts: %w", err)
	}
	data, err := json.Marshal(struct {
		Accounts     []domain.Account
		Transactions []domain.Transaction
	}{accounts, ledger.rows})
	if err != nil {
		return fmt.Errorf("encode import context: %w", err)
	}
	context := sha256.Sum256(data)
	if batch.owner != a || seal != batch.seal || context != batch.context {
		return ErrStaleImport
	}
	return nil
}

func (a *App) commitImportBatch(batch StagedImport, documentID, checkpointID string, progress func(ImportCommitProgress), yield func(), publishNotices func([]string)) (CSVImportResult, error) {
	startRevision := a.Rev()
	if err := a.checkImportBatch(batch); err != nil {
		return CSVImportResult{}, err
	}
	// Dataset-wide automation and self-resolving tasks can depend on preceding
	// effects. Preserve the established ordered runner for those configurations.
	// The bulk planner below is only used when row-local equivalence is proven.
	legacy := false
	for _, w := range a.Workflows() {
		if w.Enabled && workflow.Match(w.Trigger, workflow.TriggerTxnAdded) && !localImportWorkflow(w) {
			legacy = true
		}
	}
	if legacy || hasOpenResolveTask(a.Tasks()) {
		previousNotifier := a.Notifier
		var messages []string
		a.Notifier = func(message string) { messages = append(messages, message) }
		defer func() { a.Notifier = previousNotifier }()
		result, err := a.Commit(batch, documentID, checkpointID)
		a.Notifier = previousNotifier
		if err == nil && publishNotices != nil {
			publishNotices(messages)
		}
		return result, err
	}
	result := CSVImportResult{DocumentID: documentID}
	prepared := make([]domain.Transaction, 0, batch.Counts.New)
	extracted := make([]extract.Row, 0, batch.Counts.New)
	var outcomeIndexes []int
	accounts := a.Accounts()
	sliceStart := time.Now()
	for i, row := range batch.Rows {
		outcome := ImportRowOutcome{ID: row.ID, Line: row.Line}
		switch row.Verdict {
		case staging.Unusable:
			outcome.Status, outcome.Reason = "failed", row.Reason
			result.Failed = append(result.Failed, store.CSVRowError{Line: row.Line, Reason: row.Reason})
		case staging.Duplicate:
			outcome.Status, outcome.Reason = "duplicate", "already in the ledger"
			result.Duplicates++
		default:
			if !row.Approved() {
				return CSVImportResult{}, ErrStaleImport
			}
			if reason := a.importRowError(row.Transaction, accounts); reason != "" {
				return CSVImportResult{}, fmt.Errorf("review import row %d again: %s", row.Line, reason)
			}
			txn := row.Transaction
			txn.SourceDocID = documentID
			// The stage and its nested values remain owned by the draft.
			txn.Tags = slices.Clone(txn.Tags)
			prepared = append(prepared, txn)
			extracted = append(extracted, a.extractStagedRow(row))
			outcomeIndexes = append(outcomeIndexes, len(result.Outcomes))
			if row.PossibleDuplicate {
				result.PossibleDuplicates = append(result.PossibleDuplicates, store.CSVRowError{Line: row.Line, Reason: "possible duplicate; review this separate occurrence"})
			}
		}
		result.Outcomes = append(result.Outcomes, outcome)
		if yield != nil && i+1 < len(batch.Rows) && time.Since(sliceStart) >= 12*time.Millisecond {
			if progress != nil {
				progress(ImportCommitProgress{Phase: "preparing", Completed: i + 1, Total: len(batch.Rows)})
			}
			yield()
			sliceStart = time.Now()
		}
	}
	doc := domain.Document{}
	if documentID != "" {
		status := domain.DocImported
		if len(prepared) == 0 && len(result.Failed) > 0 {
			status = domain.DocFailed
		}
		doc = domain.Document{ID: documentID, Kind: batch.Kind, AccountID: batch.AccountID,
			UploadedAt: a.now(), Status: status, CheckpointID: checkpointID, RowsLinked: true,
			Extracted: documentRowsFromExtract(extracted), RowCount: len(prepared),
			SkippedCount: result.Duplicates + len(result.Failed), DuplicateCount: result.Duplicates, FailedCount: len(result.Failed)}
	}
	plan, err := a.planImportEffects(prepared, batch.ruleHits, yield)
	if err != nil {
		return CSVImportResult{}, err
	}
	// Every timer/progress callback above can admit another edit. Revalidate at
	// the final boundary, then never yield until all SQLite effects are complete.
	if a.Rev() != startRevision {
		return CSVImportResult{}, ErrStaleImport
	}
	if err := a.roleGuard(); err != nil {
		return CSVImportResult{}, err
	}
	seal, err := importSeal(batch)
	if err != nil {
		return CSVImportResult{}, err
	}
	if seal != batch.seal {
		return CSVImportResult{}, ErrStaleImport
	}
	beforeRev := a.store.Rev()
	auditview.SetSessionActor(auditview.ActorImport)
	defer func() { auditview.CaptureNow(); auditview.SetSessionActor("") }()
	if err := a.store.PutImportBatchWithChanges(prepared, doc, plan.changes); err != nil {
		for _, index := range outcomeIndexes {
			outcome := &result.Outcomes[index]
			outcome.Status, outcome.Reason = "failed", "import batch was not saved: "+err.Error()
			result.Failed = append(result.Failed, store.CSVRowError{Line: outcome.Line, Reason: outcome.Reason})
		}
		result.DocumentID = ""
		return result, fmt.Errorf("commit import: %w", err)
	}
	if err := a.reads.transactions.addImported(a.store, beforeRev, prepared); err != nil {
		a.logErr("refresh imported ledger", err)
	}
	result.Imported = len(prepared)
	for i, index := range outcomeIndexes {
		result.Outcomes[index].Status, result.Outcomes[index].TransactionID = "imported", prepared[i].ID
	}
	if result.Imported > 0 {
		a.fireTxnMutated()
	}
	if publishNotices != nil {
		publishNotices(plan.notices)
	}
	return result, nil
}

type importEffects struct {
	changes []store.RecordChange
	notices []string
}

func importChange(table, recordID string, before, after any) (store.RecordChange, error) {
	change := store.RecordChange{Table: table, ID: recordID}
	var err error
	if before != nil {
		change.Before, err = json.Marshal(before)
		if err != nil {
			return change, fmt.Errorf("encode import before-image: %w", err)
		}
	}
	change.After, err = json.Marshal(after)
	if err != nil {
		return change, fmt.Errorf("encode import effect: %w", err)
	}
	return change, nil
}

func (a *App) planImportEffects(txns []domain.Transaction, ruleHits map[string]int, yield func()) (importEffects, error) {
	plan := importEffects{}
	originalTxns := slices.Clone(txns)
	appendChange := func(table, recordID string, before, after any) error {
		change, err := importChange(table, recordID, before, after)
		if err == nil {
			plan.changes = append(plan.changes, change)
		}
		return err
	}
	var local []workflow.Workflow
	for _, w := range a.Workflows() {
		if !w.Enabled || !workflow.Match(w.Trigger, workflow.TriggerTxnAdded) {
			continue
		}
		local = append(local, w)
	}
	defs := a.CustomFieldDefsFor("transaction")
	tasks := a.Tasks()
	sliceStart := time.Now()
	for i := range txns {
		original := txns[i]
		ctx := workflow.Context{Vars: taskResolveEventVars(nil, original, nil, defs), Strs: map[string]string{}, TxnID: original.ID}
		addTaskResolveTransactionStrings(a, ctx.Strs, original, defs)
		for _, w := range local {
			effects, matched, err := workflow.Plan(w, ctx)
			if err != nil {
				a.logErr("workflowTriggered", err)
				continue
			}
			if !matched {
				continue
			}
			for _, effect := range effects {
				switch effect.Kind {
				case workflow.ActionSetCategory:
					txns[i].CategoryID = effect.CategoryID
				case workflow.ActionAddTag:
					txns[i].Tags = addTagUnique(txns[i].Tags, effect.Tag)
				case workflow.ActionFlagReview:
					if !txns[i].Reviewed {
						txns[i].Tags = addTagUnique(txns[i].Tags, effect.Tag)
					}
				case workflow.ActionCreateTask:
					found := false
					for _, task := range tasks {
						found = found || task.Status == domain.StatusOpen && task.Title == effect.Title
					}
					if !found {
						task := domain.Task{ID: id.New(), Title: effect.Title, Notes: effect.Notes, Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceManual}
						if effect.ResolveCondition != "" {
							task.Resolve = &domain.TaskResolve{Condition: effect.ResolveCondition}
						}
						if issues := validate.ValidateTask(task); !issues.OK() {
							a.logErr("workflowCreateTask", issues)
							continue
						}
						if err := a.validateCustom("task", task.Custom); err != nil {
							a.logErr("workflowCreateTask", err)
							continue
						}
						tasks = append(tasks, task)
						if err := appendChange("tasks", task.ID, nil, task); err != nil {
							return plan, err
						}
					}
				case workflow.ActionNotify:
					plan.notices = append(plan.notices, effect.Message)
				}
			}
			run := workflow.Run{ID: id.New(), WorkflowID: w.ID, At: a.clock().Format(time.RFC3339), Matched: true, Effects: effects}
			if err := appendChange("workflowruns", run.ID, nil, run); err != nil {
				return plan, err
			}
		}
		if yield != nil && i+1 < len(txns) && time.Since(sliceStart) >= 12*time.Millisecond {
			yield()
			sliceStart = time.Now()
		}
	}
	// Drawdown uses the staged category, before transaction-added workflows,
	// matching the ordinary transaction write path. Aggregate each goal once.
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	for _, goal := range a.Goals() {
		if !goal.IsSinkingFund || goal.Archived {
			continue
		}
		updated := goal
		for _, txn := range originalTxns {
			if txn.IsTransfer() || !txn.Amount.IsNegative() || txn.CategoryID != goal.CategoryID || txn.CategoryID == "" {
				continue
			}
			amount := txn.Amount.Abs()
			if amount.Currency != updated.CurrentAmount.Currency {
				converted, err := rates.Convert(amount, updated.CurrentAmount.Currency)
				if err != nil {
					a.log.Warn("sinking-fund drawdown skipped: no FX rate", "goal", goal.ID, "err", err)
					continue
				}
				amount = converted
			}
			var err error
			updated, err = goals.DrawDownFund(updated, amount.Amount)
			if err != nil {
				return plan, fmt.Errorf("plan imported sinking-fund drawdown: %w", err)
			}
		}
		if updated.CurrentAmount != goal.CurrentAmount {
			if err := appendChange("goals", goal.ID, goal, updated); err != nil {
				return plan, err
			}
		}
	}
	if len(ruleHits) > 0 {
		for _, rule := range a.Rules() {
			if count := ruleHits[rule.ID]; count > 0 {
				if err := appendChange("rules", rule.ID, rule, rule.RecordHits(count, a.clock())); err != nil {
					return plan, err
				}
			}
		}
	}
	return plan, nil
}

// localImportWorkflow proves from parsed references that a workflow needs only
// row fields. Complex and aggregate workflows retain the established runner.
func localImportWorkflow(w workflow.Workflow) bool {
	localExpression := func(expression string) bool {
		if strings.TrimSpace(expression) == "" {
			return true
		}
		names, err := formula.References(expression)
		if err != nil {
			return false
		}
		for _, name := range names {
			if strings.HasPrefix(name, "cf_txn_") {
				continue
			}
			switch name {
			case "txn_amount", "txn_abs", "txn_payee", "txn_desc", "txn_tags", "txn_account", "txn_category":
			default:
				return false
			}
		}
		return true
	}
	if !localExpression(w.Condition) {
		return false
	}
	for _, action := range w.Actions {
		if action.Kind == workflow.ActionCreateTask && action.ResolveCondition != "" {
			return false // the newly created task can resolve on this same posting
		}
		switch action.Kind {
		case workflow.ActionSetCategory, workflow.ActionAddTag, workflow.ActionFlagReview, workflow.ActionCreateTask, workflow.ActionNotify:
		default:
			return false
		}
		for _, template := range []string{action.Title, action.Notes, action.Message} {
			for {
				start := strings.Index(template, "{{")
				if start < 0 {
					break
				}
				end := strings.Index(template[start+2:], "}}")
				if end < 0 {
					break
				}
				if !localExpression(template[start+2 : start+2+end]) {
					return false
				}
				template = template[start+2+end+2:]
			}
		}
	}
	return true
}
