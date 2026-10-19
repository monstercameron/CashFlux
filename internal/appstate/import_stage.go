// SPDX-License-Identifier: MIT

package appstate

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeebase"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
	"github.com/monstercameron/CashFlux/internal/taskresolve"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// ErrStaleImport means the reviewed batch changed or its account/ledger context
// changed. Stage again and show the new preview before asking for approval.
var ErrStaleImport = errors.New("the import preview has changed; review it again before importing")

// StagedImport is a read-only review snapshot shared by every import source.
// Rows retain unusable source records as well as duplicates. Edits, additions,
// exclusions, or account changes require staging again before Commit.
type StagedImport struct {
	staging.Batch
	// Kind identifies the source recorded in history.
	Kind domain.DocumentKind
	// AccountID is the selected fallback frozen when the preview was built.
	AccountID string
	owner     *App
	context   [32]byte
	seal      [32]byte
	ruleHits  map[string]int
}

// ImportRowOutcome identifies the disposition of each source row after Commit.
// Status is imported, duplicate, or failed. TransactionID is populated only
// after the entire SQLite import transaction has committed successfully.
type ImportRowOutcome struct {
	ID            string
	Line          int
	Status        string
	TransactionID string
	Reason        string
}

// StageCSV parses CSV with the same account resolver and currency precision as
// the file preflight and feeds the shared staged-batch model. It never writes.
func (a *App) StageCSV(data []byte, accountID string) (StagedImport, error) {
	accounts := a.Accounts()
	list := make([]store.CSVAccount, 0, len(accounts))
	for _, ac := range accounts {
		list = append(list, store.CSVAccount{ID: ac.ID, Name: ac.Name, Currency: ac.Currency})
	}
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	txns, failed, err := store.TransactionsFromCSVResilientForAccountList(data, base, accountID, list)
	if err != nil {
		return StagedImport{}, err
	}
	_, resolveCategory, resolveMember := a.importResolvers()
	resolveAccount := idResolver(accountPairs(accounts))
	inputs := make([]staging.Input, 0, len(txns)+len(failed))
	ruleIDs := make(map[string]string, len(txns))
	for i, tx := range txns {
		tx.TransferAccountID = resolveAccount(tx.TransferAccountID)
		tx.CategoryID = resolveCategory(tx.CategoryID)
		owners := tx.Owners()
		for j := range owners {
			owners[j] = resolveMember(owners[j])
		}
		tx.SetOwners(owners)
		line := csvSourceLine(i, failed)
		rowID := fmt.Sprintf("csv-row-%d", line)
		if !tx.IsTransfer() {
			ruleIDs[rowID] = a.autoRuleForTransaction(tx)
		}
		description := tx.Desc
		tx = a.AutoCategorizeTransaction(tx)
		input := importInput(tx, rowID, line)
		input.Description = description
		input.Error = a.importRowError(tx, accounts)
		inputs = append(inputs, input)
	}
	for _, issue := range failed {
		input := staging.Input{ID: "csv-row-" + fmt.Sprint(issue.Line), Line: issue.Line, Error: issue.Reason,
			Description: issue.Source.Description, RawDate: issue.Source.Date, RawAmount: issue.Source.Amount,
			RawCurrency: issue.Source.Currency, RawAccount: issue.Source.Account}
		reference := strings.TrimSpace(issue.Source.Account)
		if reference == "" {
			reference = accountID
		}
		if account, ok := domain.AccountByID(accounts, resolveAccount(reference)); ok {
			input.AccountID = account.ID
			input.Currency = account.Currency
		}
		inputs = append(inputs, input)
	}

	sort.SliceStable(inputs, func(i, j int) bool { return inputs[i].Line < inputs[j].Line })
	batch, err := a.stageImport(domain.DocCSV, accountID, inputs)
	if err != nil {
		return StagedImport{}, err
	}
	for _, row := range batch.Rows {
		if row.Approved() {
			if rid := ruleIDs[row.ID]; rid != "" {
				batch.ruleHits[rid]++
			}
		}
	}
	return batch, nil
}

// Stage normalizes reviewed statement, PDF, receipt, or AI-extracted rows into
// the same model as StageCSV. Extracted rows remain drafts until Commit is called.
// Explicit unknown accounts and currencies never fall back silently.
func (a *App) Stage(kind domain.DocumentKind, rows []extract.Row, accountID string) (StagedImport, error) {
	accounts := a.Accounts()
	resolve := idResolver(accountPairs(accounts))
	inputs := make([]staging.Input, 0, len(rows))
	ruleIDs := map[string]string{}
	for i, row := range rows {
		rowID := row.ID
		if rowID == "" {
			rowID = id.New()
		}
		selected := strings.TrimSpace(row.AccountID)
		if selected == "" {
			selected = accountID
		}
		ac, found := domain.AccountByID(accounts, resolve(selected))
		input := staging.Input{ID: rowID, Line: i + 1, Description: row.Description,
			RawDate: row.Date, RawAmount: row.Amount, RawCurrency: row.Currency, RawAccount: row.AccountID}
		if !found {
			input.Error = fmt.Sprintf("unknown account %q; choose an existing account", selected)
		} else {
			input.AccountID = ac.ID
			input.Currency = ac.Currency
			tx, err := a.transactionFromDocumentRow(ac, row)
			if err != nil {
				input.Error = err.Error()
			} else {
				ruleIDs[rowID] = a.autoRuleForTransaction(domain.Transaction{Payee: "", Desc: strings.TrimSpace(row.Description), AccountID: tx.AccountID, Date: tx.Date, Amount: tx.Amount})
				input = importInput(tx, rowID, i+1)
				input.Description = row.Description
				input.RawDate, input.RawAmount, input.RawCurrency, input.RawAccount = row.Date, row.Amount, row.Currency, row.AccountID
				input.Error = a.importRowError(tx, accounts)
			}
		}
		inputs = append(inputs, input)
	}
	batch, err := a.stageImport(kind, accountID, inputs)
	if err != nil {
		return StagedImport{}, err
	}
	for _, row := range batch.Rows {
		if row.Approved() {
			if rid := ruleIDs[row.ID]; rid != "" {
				batch.ruleHits[rid]++
			}
		}
	}
	return batch, nil
}

func accountPairs(accounts []domain.Account) [][2]string {
	pairs := make([][2]string, 0, len(accounts))
	for _, ac := range accounts {
		pairs = append(pairs, [2]string{ac.ID, ac.Name})
	}
	return pairs
}

func importInput(tx domain.Transaction, rowID string, line int) staging.Input {
	return staging.Input{ID: rowID, Line: line, Transaction: &tx, Date: tx.Date,
		Description: tx.Desc, AccountID: tx.AccountID, AmountMinor: tx.Amount.Amount, HasAmount: true, Currency: tx.Amount.Currency,
		RawDate: dateutil.FormatDate(tx.Date), RawAmount: money.FormatMinor(tx.Amount.Amount, currency.Decimals(tx.Amount.Currency)), RawCurrency: tx.Amount.Currency, RawAccount: tx.AccountID}
}

func (a *App) importRowError(tx domain.Transaction, accounts []domain.Account) string {
	if _, found := domain.AccountByID(accounts, tx.AccountID); !found {
		return "choose an existing account"
	}
	if issues := validate.ValidateTransaction(tx, accounts...); !issues.OK() {
		return issues.Error()
	}
	if err := a.validateCustom("transaction", tx.Custom); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) stageImport(kind domain.DocumentKind, accountID string, inputs []staging.Input) (StagedImport, error) {
	existing, err := a.store.ListTransactions()
	if err != nil {
		return StagedImport{}, fmt.Errorf("stage import: read ledger: %w", err)
	}
	existingIDs := make(map[string]domain.Transaction, len(existing))
	for _, txn := range existing {
		existingIDs[txn.ID] = txn
	}
	transactionIDs := map[string]bool{}
	for i := range inputs {
		if inputs[i].Error != "" || inputs[i].Transaction == nil {
			continue
		}
		transactionID := inputs[i].Transaction.ID
		if prior, exists := existingIDs[transactionID]; exists && dedupe.Key(prior) != dedupe.Key(*inputs[i].Transaction) {
			inputs[i].Error = "import row was not saved: transaction ID already belongs to a ledger row; remove the ID to import a different transaction"
		}
		if transactionIDs[transactionID] {
			inputs[i].Error = "transaction ID is repeated in this file"
		}
		transactionIDs[transactionID] = true
	}
	batch := StagedImport{Batch: staging.Stage(inputs, existing, accountID, ""), Kind: kind, AccountID: accountID, owner: a, ruleHits: map[string]int{}}
	ids := map[string]bool{}
	for i := range batch.Rows {
		row := &batch.Rows[i]
		if ids[row.ID] {
			return StagedImport{}, fmt.Errorf("stage import: draft row ID %q is repeated", row.ID)
		}
		ids[row.ID] = true
	}
	batch.context, err = a.importContext()
	if err != nil {
		return StagedImport{}, err
	}
	batch.seal, err = importSeal(batch)
	return batch, err
}

// importContext deliberately excludes checkpoints and UI preferences: creating a
// checkpoint between preview and commit must not invalidate a review. All ledger
// and account changes do invalidate it, without using wall clocks as a winner.
func (a *App) importContext() ([32]byte, error) {
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return [32]byte{}, fmt.Errorf("stage import: read accounts: %w", err)
	}
	txns, err := a.store.ListTransactions()
	if err != nil {
		return [32]byte{}, fmt.Errorf("stage import: read ledger: %w", err)
	}
	data, err := json.Marshal(struct {
		Accounts     []domain.Account
		Transactions []domain.Transaction
	}{accounts, txns})
	if err != nil {
		return [32]byte{}, fmt.Errorf("stage import: encode context: %w", err)
	}
	return sha256.Sum256(data), nil
}

func importSeal(batch StagedImport) ([32]byte, error) {
	data, err := json.Marshal(struct {
		Batch   staging.Batch
		Kind    domain.DocumentKind
		Account string
	}{batch.Batch, batch.Kind, batch.AccountID})
	if err != nil {
		return [32]byte{}, fmt.Errorf("stage import: encode review: %w", err)
	}
	return sha256.Sum256(data), nil
}

// Commit writes exactly the approved staged transactions and one history record
// atomically. documentID may be empty to omit history. checkpointID is written
// in that same transaction, avoiding a second history write. The caller must
// await its durable IndexedDB save before acknowledging success to the user.
func (a *App) Commit(batch StagedImport, documentID, checkpointID string) (CSVImportResult, error) {
	if err := a.roleGuard(); err != nil {
		return CSVImportResult{}, err
	}
	seal, err := importSeal(batch)
	if err != nil {
		return CSVImportResult{}, err
	}
	context, err := a.importContext()
	if err != nil {
		return CSVImportResult{}, err
	}
	if batch.owner != a || seal != batch.seal || context != batch.context {
		return CSVImportResult{}, ErrStaleImport
	}
	result := CSVImportResult{DocumentID: documentID}
	prepared := make([]domain.Transaction, 0, batch.Counts.New)
	extracted := make([]extract.Row, 0, batch.Counts.New)
	preparedOutcomes := []int{}
	accounts := a.Accounts()
	for _, row := range batch.Rows {
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
			// Custom-field definitions can change without changing the ledger context.
			if reason := a.importRowError(row.Transaction, accounts); reason != "" {
				return CSVImportResult{}, fmt.Errorf("review import row %d again: %s", row.Line, reason)
			}
			tx := row.Transaction
			tx.SourceDocID = documentID
			prepared = append(prepared, tx)
			extracted = append(extracted, a.extractStagedRow(row))
			preparedOutcomes = append(preparedOutcomes, len(result.Outcomes))
			if row.PossibleDuplicate {
				result.PossibleDuplicates = append(result.PossibleDuplicates, store.CSVRowError{Line: row.Line, Reason: "possible duplicate; review this separate occurrence"})
			}
		}
		result.Outcomes = append(result.Outcomes, outcome)
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
	plan, err := a.planImportAutomation(prepared, batch.ruleHits)
	if err != nil {
		return CSVImportResult{}, err
	}
	auditview.SetSessionActor(auditview.ActorImport)
	defer func() { auditview.CaptureNow(); auditview.SetSessionActor("") }()
	if err := a.store.PutImportBatchWithChanges(prepared, doc, plan.changes); err != nil {
		for _, i := range preparedOutcomes {
			outcome := &result.Outcomes[i]
			outcome.Status, outcome.Reason = "failed", "import batch was not saved: "+err.Error()
			result.Failed = append(result.Failed, store.CSVRowError{Line: outcome.Line, Reason: outcome.Reason})
		}
		result.DocumentID = ""
		return result, fmt.Errorf("commit import: %w", err)
	}
	result.Imported = len(prepared)
	if len(plan.resolveChanges) > 0 {
		lastAutoResolve = &lastAutoResolveState{app: a, changes: plan.resolveChanges, titles: plan.resolvedTitles}
	}
	for j, i := range preparedOutcomes {
		result.Outcomes[i].Status = "imported"
		result.Outcomes[i].TransactionID = prepared[j].ID
	}
	// Dataset actions are batch actions, even when many rows matched. They
	// run only after the import transaction commits, with triggers suspended.
	previous := a.triggersSuspended
	previousObservers := a.suppressTxnObservers
	a.triggersSuspended = true
	a.suppressTxnObservers = true
	for _, effect := range plan.deferred {
		a.applyEffect(effect)
	}
	a.triggersSuspended = previous
	a.suppressTxnObservers = previousObservers
	if result.Imported > 0 {
		a.fireTxnMutated()
	}
	a.publishImportNotices(plan.notices)

	return result, nil
}

func (a *App) extractStagedRow(row staging.Row) extract.Row {
	tx := row.Transaction
	status := "pending"
	if tx.Cleared {
		status = "cleared"
	}
	return extract.Row{ID: row.ID, Date: dateutil.FormatDate(tx.Date), Description: tx.Desc,
		Amount:   money.FormatMinor(tx.Amount.Amount, currency.Decimals(tx.Amount.Currency)),
		Category: a.categoryNameByID(tx.CategoryID), AccountID: tx.AccountID, Currency: tx.Amount.Currency, Status: status}
}

func (a *App) transactionFromDocumentRow(acc domain.Account, row extract.Row) (domain.Transaction, error) {
	code := strings.ToUpper(strings.TrimSpace(row.Currency))
	if code != "" && code != acc.Currency {
		return domain.Transaction{}, fmt.Errorf("currency %s does not match account currency %s", code, acc.Currency)
	}
	minor, err := staging.ParseAmount(row.Amount, currency.Decimals(acc.Currency))
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("invalid amount: %w", err)
	}
	if minor == 0 {
		return domain.Transaction{}, errors.New("amount must not be zero")
	}
	date, err := dateutil.ParseDate(strings.TrimSpace(row.Date))
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("invalid date; use YYYY-MM-DD: %w", err)
	}
	desc := strings.TrimSpace(row.Description)
	if desc == "" {
		return domain.Transaction{}, errors.New("description is required")
	}
	tx := domain.Transaction{ID: id.New(), AccountID: acc.ID, Date: date, Desc: desc,
		CategoryID: a.categoryIDForDocumentRow(row.Category), Amount: money.New(minor, acc.Currency), Source: domain.TxnSourceScanned,
		Cleared: extractStatusCleared(row.Status)}
	return a.AutoCategorizeTransaction(tx), nil
}

// CommitDurable commits a reviewed batch and calls done only after the page's
// durable storage boundary completes. An unavailable boundary prevents writes.
// On a durability error, result describes the rows still held in memory: keep
// the draft and retry RequestDurable, rather than importing those rows twice.
func (a *App) CommitDurable(batch StagedImport, documentID, checkpointID string, done func(CSVImportResult, error)) {
	if done == nil {
		return
	}
	if a.durableWriter == nil {
		done(CSVImportResult{}, ErrDurabilityUnavailable)
		return
	}
	previousNotifier := a.Notifier
	var notices []string
	a.Notifier = func(message string) { notices = append(notices, message) }
	result, err := a.Commit(batch, documentID, checkpointID)
	a.Notifier = previousNotifier
	if err != nil {
		done(result, err)
		return
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
		if previousNotifier != nil && len(notices) > 0 {
			previousNotifier(strings.Join(notices, "\n"))
		}
		done(result, nil)
	})
}

// importAutomationPlan retains all row-local writes and audit runs for one SQL
// transaction. Dataset-wide actions are deduplicated and run once afterwards.
type importAutomationPlan struct {
	importEffects
	deferred       []workflow.Effect
	resolveChanges []store.RecordChange
	resolvedTitles []string
}

// importAutomationContext belongs to one immutable batch, rather than a store
// revision. Its own planned writes cannot invalidate the aggregate values.
type importAutomationContext struct {
	vars                 map[string]float64
	defs                 []customfields.Def
	accounts, categories map[string]string
	merchants            map[string][]int64
	positions            map[string]int
}

func (a *App) newImportAutomationContext(txns []domain.Transaction) importAutomationContext {
	all := append(append([]domain.Transaction(nil), a.Transactions()...), txns...)
	accounts, categories := a.Accounts(), a.Categories()
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	c := importAutomationContext{defs: a.CustomFieldDefsFor("transaction"), accounts: map[string]string{}, categories: map[string]string{}, merchants: map[string][]int64{}, positions: map[string]int{}}
	// The engine sees the complete imported ledger once, before automation. Row
	// fields override this frozen aggregate surface in event().
	c.vars = engineenv.Vars(engineenv.Data{
		Ledger: budgeting.NewNettedLedger(all, a.TxnLinks()), Accounts: accounts,
		Transactions: all, Members: a.Members(), Budgets: a.Budgets(), Goals: a.Goals(), Tasks: a.Tasks(),
		Recurring: a.Recurring(), Categories: categories, CustomDefs: a.CustomFieldDefs(), Molecules: a.Molecules(),
		Rates: currency.Rates{Base: base, Rates: a.Settings().FXRates}, Now: a.clock(),
	})
	for _, account := range accounts {
		c.accounts[account.ID] = account.Name
	}
	for _, category := range categories {
		c.categories[category.ID] = category.Name
	}
	groups := map[string][]domain.Transaction{}
	for _, txn := range all {
		if !txn.IsExpense() || payeebase.Key(txn) == "" {
			continue
		}
		groups[importMerchantKey(txn)] = append(groups[importMerchantKey(txn)], txn)
	}
	for key, rows := range groups {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Amount.Amount > rows[j].Amount.Amount })
		for _, txn := range rows {
			magnitude, err := txn.Amount.AbsChecked()
			if err != nil {
				continue
			} // unusable history is never a baseline
			c.positions[txn.ID] = len(c.merchants[key])
			c.merchants[key] = append(c.merchants[key], magnitude.Amount)
		}
	}
	return c
}

func importMerchantKey(txn domain.Transaction) string {
	return txn.Amount.Currency + "\x00" + payeebase.Key(txn)
}

func (c importAutomationContext) event(txn domain.Transaction) workflow.Context {
	ctx := workflow.Context{Vars: taskResolveEventVars(c.vars, txn, nil, c.defs), Strs: map[string]string{}, TxnID: txn.ID}
	ctx.Strs["txn_payee"], ctx.Strs["txn_desc"] = txn.Payee, txn.Desc
	ctx.Strs["txn_tags"] = strings.Join(txn.Tags, ",")
	ctx.Strs["txn_account"], ctx.Strs["txn_category"] = c.accounts[txn.AccountID], c.categories[txn.CategoryID]
	for _, def := range c.defs {
		if def.Type != customfields.TypeNumber && def.Type != customfields.TypeBool {
			ctx.Strs["cf_txn_"+def.Key], _ = txn.Custom[def.Key].(string)
		}
	}
	values := c.merchants[importMerchantKey(txn)]
	position, present := c.positions[txn.ID]
	n := len(values)
	if present {
		n--
	}
	if n >= payeebase.MinHistory {
		at := func(i int) int64 {
			if present && i >= position {
				i++
			}
			return values[i]
		}
		median := at(n / 2)
		if n%2 == 0 {
			low := at(n/2 - 1)
			median = low + (median-low)/2
		}
		if median > 0 {
			div := math.Pow10(currency.Decimals(txn.Amount.Currency))
			ctx.Vars["txn_payee_typical"] = float64(median) / div
			ctx.Vars["txn_vs_typical"] = math.Abs(float64(txn.Amount.Amount)) / float64(median)
		}
	}
	return ctx
}

func (a *App) planImportAutomation(txns []domain.Transaction, ruleHits map[string]int) (importAutomationPlan, error) {
	plan := importAutomationPlan{}
	if len(txns) == 0 {
		return plan, nil
	}
	context := a.newImportAutomationContext(txns)
	datasetActions := map[workflow.ActionKind]bool{}
	born := map[string]int{}
	originalTxns := append([]domain.Transaction(nil), txns...)
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

	tasks := a.Tasks()
	openTaskTitles := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if task.Status == domain.StatusOpen {
			openTaskTitles[task.Title] = true
		}
	}

	for i := range txns {
		original := txns[i]
		ctx := context.event(original)
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
					if !openTaskTitles[effect.Title] {
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
						openTaskTitles[task.Title] = true
						born[task.ID] = i
						tasks = append(tasks, task)
						if err := appendChange("tasks", task.ID, nil, task); err != nil {
							return plan, err
						}
					}
				case workflow.ActionApplyRules, workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
					if !datasetActions[effect.Kind] {
						datasetActions[effect.Kind] = true
						plan.deferred = append(plan.deferred, effect)
					}
				case workflow.ActionAgentRun, workflow.ActionTransfer:
					return plan, fmt.Errorf("this workflow action requires a scheduled run, not a transaction import")
				case workflow.ActionNotify:
					plan.notices = append(plan.notices, effect.Message)
				}
			}
			run := workflow.Run{ID: id.New(), WorkflowID: w.ID, At: a.clock().Format(time.RFC3339), Matched: true, Effects: effects}
			if err := appendChange("workflowruns", run.ID, nil, run); err != nil {
				return plan, err
			}
		}

	}
	// Resolve against the same batch context used by workflows. Each task is
	// completed at most once; tasks created by row i cannot see earlier events.
	for _, task := range tasks {
		if task.Status != domain.StatusOpen || task.Resolve == nil {
			continue
		}
		for _, original := range originalTxns[born[task.ID]:] {
			ctx := context.event(original)
			ev := taskresolve.Event{Vars: ctx.Vars, Strs: ctx.Strs, Txn: &taskresolve.TxnEvent{Payee: txnPayeeOrDesc(original), AmountMinor: original.Amount.Amount, Currency: original.Amount.Currency}}
			resolved, err := taskresolve.Resolves(*task.Resolve, ev)
			if err != nil {
				a.log.Warn("task resolve condition failed", "task", task.ID, "err", err)
				break
			}
			if !resolved {
				continue
			}
			completed := task
			completed.Status = domain.StatusDone
			if err := appendChange("tasks", task.ID, task, completed); err != nil {
				return plan, err
			}
			plan.resolveChanges = append(plan.resolveChanges, plan.changes[len(plan.changes)-1])
			plan.resolvedTitles = append(plan.resolvedTitles, task.Title)
			if next, spawn := taskrecur.NextOccurrence(completed, id.New(), a.clock()); spawn {
				if issues := validate.ValidateTask(next); !issues.OK() {
					return plan, fmt.Errorf("resolve imported task: %w", issues)
				}
				if err := appendChange("tasks", next.ID, nil, next); err != nil {
					return plan, err
				}
				plan.resolveChanges = append(plan.resolveChanges, plan.changes[len(plan.changes)-1])
			}
			plan.notices = append(plan.notices, "Done for you: "+task.Title)
			break
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
			amount, err := txn.Amount.AbsChecked()
			if err != nil {
				return plan, fmt.Errorf("plan imported sinking-fund amount: %w", err)
			}
			if amount.Currency != updated.CurrentAmount.Currency {
				converted, err := rates.Convert(amount, updated.CurrentAmount.Currency)
				if err != nil {
					a.log.Warn("sinking-fund drawdown skipped: no FX rate", "goal", goal.ID, "err", err)
					continue
				}
				amount = converted
			}
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
