// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// AccountReconciliationPreview is a read-only, state-bound balance checkpoint
// preview. Balances use the account card's stated convention (positive means
// money held, negative means debt owed).
type AccountReconciliationPreview struct {
	AccountID             string
	AccountName           string
	Currency              string
	CurrentStatedMinor    int64
	TargetStatedMinor     int64
	AdjustmentStatedMinor int64
	AsOf                  time.Time
	NeedsAdjustment       bool
	CheckpointAmountMinor int64
	CheckpointCount       int
	CheckpointAction      string

	account              domain.Account
	currentStated        int64
	targetStated         int64
	adjustmentStated     int64
	currency             string
	accountName          string
	targetStoredMinor    int64
	checkpointAmount     int64
	checkpointCount      int
	checkpointAction     string
	asOf                 time.Time
	needsAdjustment      bool
	expectedTransactions []domain.Transaction
}

// PreviewAccountReconciliation reads the booked balance and builds a preview
// whose commit will fail if the account or any transaction on it changes.
func (a *App) PreviewAccountReconciliation(accountID string, targetStatedMinor int64, asOf time.Time) (AccountReconciliationPreview, error) {
	if accountID == "" {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: account ID is required")
	}
	account, ok, err := a.store.GetAccount(accountID)
	if err != nil || !ok {
		if err != nil {
			return AccountReconciliationPreview{}, fmt.Errorf("balance update: read account: %w", err)
		}
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: account no longer exists")
	}
	if account.Archived {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: archived accounts cannot be updated")
	}
	allRows, err := a.store.ListTransactions()
	if err != nil {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: read ledger: %w", err)
	}
	rows := accountTransactions(allRows, accountID)
	booked, err := ledger.Balance(account, rows)
	if err != nil {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: read booked balance: %w", err)
	}
	convention, _ := reconcile.ConventionOf(account, booked.Amount)
	currentStated, err := statedBalance(booked.Amount, convention)
	if err != nil {
		return AccountReconciliationPreview{}, err
	}
	targetStored, err := storedBalance(targetStatedMinor, convention)
	if err != nil {
		return AccountReconciliationPreview{}, err
	}
	adjustment, err := checkedSubtract(targetStatedMinor, currentStated)
	if err != nil {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: adjustment: %w", err)
	}
	if asOf.IsZero() {
		asOf = a.clock()
	}
	checkpoints := provisional.Of(accountID, rows)
	rawRows := rowsWithoutCheckpoints(rows)
	rawBooked, err := ledger.Balance(account, rawRows)
	if err != nil {
		return AccountReconciliationPreview{}, fmt.Errorf("balance update: read balance without prior checkpoints: %w", err)
	}
	var checkpointAmount int64
	if adjustment != 0 {
		checkpointAmount, err = checkedSubtract(targetStored, rawBooked.Amount)
		if err != nil {
			return AccountReconciliationPreview{}, fmt.Errorf("balance update: checkpoint amount: %w", err)
		}
	}
	action := "add"
	if len(checkpoints) > 0 {
		action = "replace"
	}
	if checkpointAmount == 0 && len(checkpoints) > 0 {
		action = "remove"
	}
	if adjustment == 0 {
		action = "none"
	}
	return AccountReconciliationPreview{
		AccountID: account.ID, AccountName: account.Name, Currency: account.Currency,
		CurrentStatedMinor: currentStated, TargetStatedMinor: targetStatedMinor,
		AdjustmentStatedMinor: adjustment, AsOf: normalizeTransactionDate(asOf),
		NeedsAdjustment: adjustment != 0, account: account, targetStoredMinor: targetStored,
		CheckpointAmountMinor: checkpointAmount, CheckpointCount: len(checkpoints), CheckpointAction: action,
		currentStated: currentStated, targetStated: targetStatedMinor,
		adjustmentStated: adjustment, currency: account.Currency, accountName: account.Name,
		checkpointAmount: checkpointAmount, checkpointCount: len(checkpoints), checkpointAction: action,
		asOf: normalizeTransactionDate(asOf), needsAdjustment: adjustment != 0,
		expectedTransactions: cloneTransactions(rows),
	}, nil
}

// CommitAccountReconciliation atomically replaces balance checkpoints and
// updates BalanceAsOf only if the previewed account and its complete ledger
// slice remain unchanged. A zero-delta confirmation is a no-op.
func (a *App) CommitAccountReconciliation(preview AccountReconciliationPreview, desc string) (domain.Transaction, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Transaction{}, err
	}
	if preview.AccountID == "" || preview.account.ID != preview.AccountID ||
		preview.AccountName != preview.accountName || preview.Currency != preview.currency ||
		preview.CurrentStatedMinor != preview.currentStated || preview.TargetStatedMinor != preview.targetStated ||
		preview.AdjustmentStatedMinor != preview.adjustmentStated || preview.AsOf != preview.asOf ||
		preview.NeedsAdjustment != preview.needsAdjustment || preview.CheckpointAmountMinor != preview.checkpointAmount ||
		preview.CheckpointCount != preview.checkpointCount || preview.CheckpointAction != preview.checkpointAction {
		return domain.Transaction{}, fmt.Errorf("balance update: invalid preview")
	}
	account, ok, err := a.store.GetAccount(preview.AccountID)
	if err != nil || !ok || !sameJSON(account, preview.account) {
		return domain.Transaction{}, fmt.Errorf("balance update: account changed since preview")
	}
	allRows, err := a.store.ListTransactions()
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("balance update: read ledger: %w", err)
	}
	rows := accountTransactions(allRows, preview.AccountID)
	if !sameTransactionSet(rows, preview.expectedTransactions) {
		return domain.Transaction{}, fmt.Errorf("balance update: ledger changed since preview")
	}
	if !preview.needsAdjustment {
		return domain.Transaction{}, nil
	}
	rawRows := rowsWithoutCheckpoints(rows)
	booked, err := ledger.Balance(account, rawRows)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("balance update: read booked balance: %w", err)
	}
	checkpoint, create, err := provisional.NewChecked(id.New(), account.ID, desc, booked.Amount,
		preview.targetStoredMinor, account.Currency, preview.asOf, preview.asOf)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("balance update: %w", err)
	}
	if create {
		accounts, err := a.store.ListAccounts()
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("balance update: read accounts: %w", err)
		}
		if issues := validate.ValidateTransaction(checkpoint, accounts...); !issues.OK() {
			return domain.Transaction{}, issues
		}
	} else {
		checkpoint = domain.Transaction{}
	}
	changes, err := a.reconciliationChanges(account, rows, checkpoint, preview.asOf)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return domain.Transaction{}, fmt.Errorf("balance update: commit: %w", err)
	}
	a.fireTxnMutated()
	return checkpoint, nil
}

func (a *App) reconciliationChanges(account domain.Account, rows []domain.Transaction, checkpoint domain.Transaction, asOf time.Time) ([]store.RecordChange, error) {
	changes := make([]store.RecordChange, 0, len(rows)+2)
	for _, old := range provisional.Of(account.ID, rows) {
		before, err := json.Marshal(old)
		if err != nil {
			return nil, fmt.Errorf("balance update: encode old checkpoint: %w", err)
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: old.ID, Before: before})
	}
	checkpoints := make(map[string]bool)
	for _, old := range provisional.Of(account.ID, rows) {
		checkpoints[old.ID] = true
	}
	for _, row := range rows {
		if checkpoints[row.ID] {
			continue
		}
		data, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("balance update: encode expected transaction %s: %w", row.ID, err)
		}
		// A no-op transition makes the row part of the same expected-state write.
		changes = append(changes, store.RecordChange{Table: "transactions", ID: row.ID, Before: data, After: data})
	}
	updated := account
	updated.BalanceAsOf = asOf
	beforeAccount, err := json.Marshal(account)
	if err != nil {
		return nil, fmt.Errorf("balance update: encode account: %w", err)
	}
	afterAccount, err := json.Marshal(updated)
	if err != nil {
		return nil, fmt.Errorf("balance update: encode updated account: %w", err)
	}
	changes = append(changes, store.RecordChange{Table: "accounts", ID: account.ID, Before: beforeAccount, After: afterAccount})
	if checkpoint.ID != "" {
		checkpointData, err := json.Marshal(checkpoint)
		if err != nil {
			return nil, fmt.Errorf("balance update: encode checkpoint: %w", err)
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: checkpoint.ID, After: checkpointData})
	}
	return changes, nil
}

// ExpectedTransferPreview binds the ordinary transfer preview to the accounts,
// saved FX settings and account ledger rows used to calculate it.
type ExpectedTransferPreview struct {
	Params          TransferParams
	Balances        TransferPreview
	FromAccountName string
	ToAccountName   string
	FromAmount      money.Money
	ToAmount        money.Money

	params      TransferParams
	balances    TransferPreview
	fromAccount domain.Account
	toAccount   domain.Account
	settings    store.Settings
	fromRows    []domain.Transaction
	toRows      []domain.Transaction
	fromAmount  money.Money
	toAmount    money.Money
}

// PreviewExpectedTransfer normalizes defaults, then records all financial
// inputs used by the shared transfer preview math.
func (a *App) PreviewExpectedTransfer(p TransferParams) (ExpectedTransferPreview, error) {
	if p.FromAccountID == "" || p.ToAccountID == "" || p.FromAccountID == p.ToAccountID || p.AmountMinor <= 0 || p.ReceivedMinor < 0 || p.FeeMinor < 0 {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer preview: invalid account or amount")
	}
	if p.OperationID == "" {
		p.OperationID = id.New()
	}
	if p.Date.IsZero() {
		p.Date = a.clock()
	}
	p.Date = normalizeTransactionDate(p.Date)
	if p.Desc == "" {
		p.Desc = "Transfer"
	}
	if p.Source == "" {
		p.Source = domain.TxnSourceManual
	}
	from, to, err := a.expectedTransferAccounts(p.FromAccountID, p.ToAccountID)
	if err != nil {
		return ExpectedTransferPreview{}, err
	}
	if from.Archived || to.Archived {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer: archived accounts cannot be used")
	}
	settings, err := a.store.GetSettings()
	if err != nil {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer preview: read settings: %w", err)
	}
	settings.OpenAIKey = ""
	allRows, err := a.store.ListTransactions()
	if err != nil {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer preview: read ledger: %w", err)
	}
	rows := allRows
	fromRows, toRows := accountTransactions(rows, from.ID), accountTransactions(rows, to.ID)
	fromBal, err := ledger.Balance(from, fromRows)
	if err != nil {
		return ExpectedTransferPreview{}, err
	}
	toBal, err := ledger.Balance(to, toRows)
	if err != nil {
		return ExpectedTransferPreview{}, err
	}
	rates := settingsRates(settings)
	fromLeg, toLeg, err := transferLegsWithRates(from, to, p.AmountMinor, p.ReceivedMinor, rates)
	if err != nil {
		return ExpectedTransferPreview{}, err
	}
	feeDelta := int64(0)
	if p.FeeMinor > 0 {
		feeDelta = -from.InflowSign() * p.FeeMinor
	}
	fromLegAndFee, err := checkedAdd(fromLeg.Amount, feeDelta)
	var fromAfter int64
	if err == nil {
		fromAfter, err = checkedAdd(fromBal.Amount, fromLegAndFee)
	}
	if err != nil {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer preview: source balance overflows")
	}
	toAfter, err := checkedAdd(toBal.Amount, toLeg.Amount)
	if err != nil {
		return ExpectedTransferPreview{}, fmt.Errorf("transfer preview: destination balance overflows")
	}
	pv := TransferPreview{FromBefore: fromBal, FromAfter: money.New(fromAfter, from.Currency),
		ToBefore: toBal, ToAfter: money.New(toAfter, to.Currency)}
	return ExpectedTransferPreview{Params: p, Balances: pv, FromAccountName: from.Name, ToAccountName: to.Name,
		FromAmount: fromLeg, ToAmount: toLeg, params: p, balances: pv, fromAccount: from, toAccount: to,
		settings: cloneSettings(settings), fromRows: cloneTransactions(fromRows), toRows: cloneTransactions(toRows),
		fromAmount: fromLeg, toAmount: toLeg}, nil
}

// CreateTransferPairExpected commits only if both previewed accounts, their
// ledger rows and saved FX settings are still current. It uses the same transfer
// legs and row construction as CreateTransferPair.
func (a *App) CreateTransferPairExpected(preview ExpectedTransferPreview) (string, string, error) {
	if err := a.roleGuard(); err != nil {
		return "", "", err
	}
	p := preview.params
	if p.OperationID == "" || !sameJSON(preview.Params, preview.params) || !sameJSON(preview.Balances, preview.balances) ||
		preview.FromAccountName != preview.fromAccount.Name || preview.ToAccountName != preview.toAccount.Name ||
		!sameJSON(preview.FromAmount, preview.fromAmount) || !sameJSON(preview.ToAmount, preview.toAmount) ||
		preview.fromAccount.ID != p.FromAccountID || preview.toAccount.ID != p.ToAccountID {
		return "", "", fmt.Errorf("transfer: invalid preview")
	}
	legs, outID, inID, err := buildTransferRows(p, preview.fromAccount, preview.toAccount, settingsRates(preview.settings))
	if err != nil {
		return "", "", err
	}
	exists, err := a.expectedTransferAlreadyCommitted(legs)
	if err != nil {
		return "", "", err
	}
	if exists {
		return outID, inID, nil
	}
	from, to, err := a.expectedTransferAccounts(p.FromAccountID, p.ToAccountID)
	if err != nil || from.Archived || to.Archived || !sameJSON(from, preview.fromAccount) || !sameJSON(to, preview.toAccount) {
		return "", "", fmt.Errorf("transfer: account changed since preview")
	}
	settings, err := a.store.GetSettings()
	if err != nil {
		return "", "", fmt.Errorf("transfer: read settings: %w", err)
	}
	settings.OpenAIKey = ""
	if !sameJSON(settings, preview.settings) {
		return "", "", fmt.Errorf("transfer: saved currency settings changed since preview")
	}
	rows, err := a.store.ListTransactions()
	if err != nil {
		return "", "", fmt.Errorf("transfer: read ledger: %w", err)
	}
	if !sameTransactionSet(accountTransactions(rows, from.ID), preview.fromRows) ||
		!sameTransactionSet(accountTransactions(rows, to.ID), preview.toRows) {
		return "", "", fmt.Errorf("transfer: account ledger changed since preview")
	}
	if err := a.writeExpectedTransfer(preview, legs); err != nil {
		return "", "", err
	}
	return outID, inID, nil
}

func (a *App) expectedTransferAlreadyCommitted(rows []domain.Transaction) (bool, error) {
	found := 0
	for _, expected := range rows {
		current, ok, err := a.store.GetTransaction(expected.ID)
		if err != nil {
			return false, fmt.Errorf("transfer: check operation ID: %w", err)
		}
		if !ok {
			continue
		}
		found++
		if !sameTransaction(current, expected) {
			return false, fmt.Errorf("transfer: operation ID already belongs to a different transfer")
		}
	}
	if found == 0 {
		return false, nil
	}
	if found != len(rows) {
		return false, fmt.Errorf("transfer: a previous attempt left an incomplete transfer; no additional rows were written")
	}
	return true, nil
}

func buildTransferRows(p TransferParams, from, to domain.Account, rates currency.Rates) ([]domain.Transaction, string, string, error) {
	if p.AmountMinor <= 0 || p.ReceivedMinor < 0 || p.FeeMinor < 0 {
		return nil, "", "", fmt.Errorf("transfer: invalid amount")
	}
	fromMoney, toMoney, err := transferLegsWithRates(from, to, p.AmountMinor, p.ReceivedMinor, rates)
	if err != nil {
		return nil, "", "", err
	}
	when := normalizeTransactionDate(p.Date)
	desc := p.Desc
	if desc == "" {
		desc = "Transfer"
	}
	source := p.Source
	if source == "" {
		source = domain.TxnSourceManual
	}
	outID, inID := transferID(p.OperationID, "out"), transferID(p.OperationID, "in")
	groupID := transferGroupID(p, when, desc, source)
	out := domain.Transaction{ID: outID, AccountID: from.ID, TransferAccountID: to.ID,
		TransferGroupID: groupID, Amount: fromMoney, Date: when, Desc: desc, Payee: to.Name,
		Reviewed: true, Source: source}
	in := domain.Transaction{ID: inID, AccountID: to.ID, TransferAccountID: from.ID,
		TransferGroupID: groupID, Amount: toMoney, Date: when, Desc: desc, Payee: from.Name,
		Reviewed: true, Source: source}
	legs := []domain.Transaction{out, in}
	if p.FeeMinor > 0 {
		legs = append(legs, domain.Transaction{ID: transferID(p.OperationID, "fee"), AccountID: from.ID,
			Amount: money.New(-from.InflowSign()*p.FeeMinor, from.Currency), Date: when,
			Desc: desc + " — fee", Payee: to.Name, Reviewed: true, Source: source})
	}
	return legs, outID, inID, nil
}

func (a *App) writeExpectedTransfer(preview ExpectedTransferPreview, legs []domain.Transaction) error {
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return fmt.Errorf("transfer: read accounts: %w", err)
	}
	for _, row := range legs {
		if issues := validate.ValidateTransaction(row, accounts...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", row.Custom); err != nil {
			return err
		}
	}
	var settingsData json.RawMessage
	if !sameJSON(preview.settings, store.Settings{}) {
		encoded, err := json.Marshal(preview.settings)
		if err != nil {
			return fmt.Errorf("transfer: encode expected settings: %w", err)
		}
		settingsData = encoded
	}
	fromData, err := json.Marshal(preview.fromAccount)
	if err != nil {
		return fmt.Errorf("transfer: encode expected source account: %w", err)
	}
	toData, err := json.Marshal(preview.toAccount)
	if err != nil {
		return fmt.Errorf("transfer: encode expected destination account: %w", err)
	}
	changes := []store.RecordChange{
		{Table: "settings", ID: "app", Before: settingsData, After: settingsData},
		{Table: "accounts", ID: preview.fromAccount.ID, Before: fromData, After: fromData},
		{Table: "accounts", ID: preview.toAccount.ID, Before: toData, After: toData},
	}
	for _, row := range append(append([]domain.Transaction(nil), preview.fromRows...), preview.toRows...) {
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("transfer: encode expected transaction %s: %w", row.ID, err)
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: row.ID, Before: data, After: data})
	}
	for _, row := range legs {
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("transfer: encode leg: %w", err)
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: row.ID, After: data})
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("transfer: commit expected preview: %w", err)
	}
	a.fireTxnMutated()
	return nil
}

func accountTransactions(rows []domain.Transaction, accountID string) []domain.Transaction {
	out := make([]domain.Transaction, 0)
	for _, row := range rows {
		if row.AccountID == accountID {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func rowsWithoutCheckpoints(rows []domain.Transaction) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		if !row.IsBalanceCheckpoint() {
			out = append(out, row)
		}
	}
	return out
}

func (a *App) expectedTransferAccounts(fromID, toID string) (domain.Account, domain.Account, error) {
	if fromID == "" || toID == "" || fromID == toID {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: source and destination accounts must differ")
	}
	from, ok, err := a.store.GetAccount(fromID)
	if err != nil {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: read source account: %w", err)
	}
	if !ok {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: source account no longer exists")
	}
	to, ok, err := a.store.GetAccount(toID)
	if err != nil {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: read destination account: %w", err)
	}
	if !ok {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: destination account no longer exists")
	}
	return from, to, nil
}

func settingsRates(settings store.Settings) currency.Rates {
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: settings.FXRates}
}

func cloneTransactions(rows []domain.Transaction) []domain.Transaction {
	encoded, _ := json.Marshal(rows)
	var out []domain.Transaction
	_ = json.Unmarshal(encoded, &out)
	return out
}

func cloneSettings(s store.Settings) store.Settings {
	encoded, _ := json.Marshal(s)
	var out store.Settings
	_ = json.Unmarshal(encoded, &out)
	return out
}

func sameTransactionSet(a, b []domain.Transaction) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || !sameTransaction(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameJSON(a, b any) bool {
	left, err1 := json.Marshal(a)
	right, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(left) == string(right)
}

func statedBalance(stored int64, convention reconcile.Convention) (int64, error) {
	if convention == reconcile.PositiveOwed {
		return checkedNegate(stored)
	}
	return stored, nil
}

func storedBalance(stated int64, convention reconcile.Convention) (int64, error) {
	if convention == reconcile.PositiveOwed {
		return checkedNegate(stated)
	}
	return stated, nil
}

func checkedNegate(v int64) (int64, error) {
	if v == math.MinInt64 {
		return 0, fmt.Errorf("balance update: amount is outside the supported range")
	}
	return -v, nil
}

func checkedSubtract(a, b int64) (int64, error) {
	if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		return 0, fmt.Errorf("amount is outside the supported range")
	}
	return a - b, nil
}

func checkedAdd(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, fmt.Errorf("amount is outside the supported range")
	}
	return a + b, nil
}
