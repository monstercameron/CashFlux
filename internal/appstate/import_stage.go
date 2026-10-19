// SPDX-License-Identifier: MIT

package appstate

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
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
			ruleIDs[rowID] = a.autoRuleFor(tx.Payee, tx.Desc, tx.Amount.Amount, tx.AccountID, rules.NewTxnDate(tx.Date))
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
				input = importInput(tx, rowID, i+1)
				input.Description = row.Description
				input.RawDate, input.RawAmount, input.RawCurrency, input.RawAccount = row.Date, row.Amount, row.Currency, row.AccountID
				input.Error = a.importRowError(tx, accounts)
			}
		}
		inputs = append(inputs, input)
	}
	return a.stageImport(kind, accountID, inputs)
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
	transactionIDs := map[string]bool{}
	for i := range inputs {
		if inputs[i].Error != "" || inputs[i].Transaction == nil {
			continue
		}
		transactionID := inputs[i].Transaction.ID
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
	auditview.SetSessionActor(auditview.ActorImport)
	defer func() { auditview.CaptureNow(); auditview.SetSessionActor("") }()
	if err := a.store.PutImportBatch(prepared, doc); err != nil {
		for _, i := range preparedOutcomes {
			outcome := &result.Outcomes[i]
			outcome.Status, outcome.Reason = "failed", "import batch was not saved: "+err.Error()
			result.Failed = append(result.Failed, store.CSVRowError{Line: outcome.Line, Reason: outcome.Reason})
		}
		result.DocumentID = ""
		return result, fmt.Errorf("commit import: %w", err)
	}
	result.Imported = len(prepared)
	for j, i := range preparedOutcomes {
		result.Outcomes[i].Status = "imported"
		result.Outcomes[i].TransactionID = prepared[j].ID
	}
	if result.Imported > 0 {
		a.fireTxnMutated()
	}
	for i := range prepared {
		a.applySinkingFundDrawdown(prepared[i])
		a.RunTriggered(workflow.TriggerTxnAdded, &prepared[i])
		a.resolveTasksForTxn(prepared[i])
	}
	if err := a.CreditRuleHits(batch.ruleHits); err != nil {
		a.log.Error("credit rule hits after import", "err", err)
	}
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
	result, err := a.Commit(batch, documentID, checkpointID)
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
		done(result, nil)
	})
}
