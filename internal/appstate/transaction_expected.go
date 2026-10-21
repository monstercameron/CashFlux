// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// TransactionUpdatePreview contains the complete validated before/after rows.
// Its private immutable transitions, rather than caller-edited public fields,
// are the only records UpdateTransactionExpected can commit.
type TransactionUpdatePreview struct {
	Before, After   []domain.Transaction
	before, after   []domain.Transaction
	changes         []store.RecordChange
	target          domain.Transaction
	group           []domain.Transaction
	balanceRows     []domain.Transaction
	balanceAccounts []string
}

// planPairedTransactionEdit is the shared transfer-edit calculation used by the
// existing editor and approval previews. It never writes or duplicates FX math.
func (a *App) planPairedTransactionEdit(prior, next domain.Transaction, receivedMinor int64) ([]domain.Transaction, error) {
	rows := []domain.Transaction{next}
	amountChanged := prior.Amount != next.Amount
	dateChanged := !prior.Date.Equal(next.Date)
	if !next.IsTransfer() || !amountChanged && !dateChanged {
		return rows, nil
	}
	pair, found := a.transferPartner(prior)
	if !found {
		return rows, nil
	}
	if amountChanged {
		amount, err := a.mirroredLegAmount(next, pair, receivedMinor)
		if err != nil {
			return nil, err
		}
		pair.Amount = amount
	}
	if dateChanged {
		pair.Date = next.Date
	}
	return append(rows, pair), nil
}

func updateGroupRows(rows []domain.Transaction, groupID string) []domain.Transaction {
	var group []domain.Transaction
	for _, row := range rows {
		if row.TransferGroupID == groupID {
			group = append(group, row)
		}
	}
	sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	return group
}

// updateRelatedTransferRows includes explicit group members and the fee from a
// proven canonical operation ID family. Fees intentionally remain ordinary
// spending rows and therefore do not carry TransferGroupID.
func updateRelatedTransferRows(rows []domain.Transaction, target domain.Transaction) []domain.Transaction {
	group := updateGroupRows(rows, target.TransferGroupID)
	stem := strings.TrimSuffix(strings.TrimSuffix(target.ID, "-out"), "-in")
	hash := strings.TrimPrefix(stem, "transfer-")
	if len(hash) != 32 || stem == target.ID {
		return group
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return group
	}
	out, in := false, false
	for _, row := range group {
		out = out || row.IsTransfer() && row.ID == stem+"-out"
		in = in || row.IsTransfer() && row.ID == stem+"-in"
	}
	if !out || !in {
		return group
	}
	for _, row := range rows {
		if row.ID == stem+"-fee" && row.TransferGroupID != target.TransferGroupID {
			group = append(group, row)
		}
	}
	sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	return group
}

// PreviewTransactionUpdate validates the complete row and canonical paired edit
// before approval. Group fees remain unchanged but are bound to the same preview.
func (a *App) PreviewTransactionUpdate(prior, draft domain.Transaction) (TransactionUpdatePreview, error) {
	var p TransactionUpdatePreview
	current, found, err := a.store.GetTransaction(prior.ID)
	if err != nil {
		return p, err
	}
	if !found || !sameTransaction(current, prior) || draft.ID != prior.ID {
		return p, fmt.Errorf("transaction changed before preview")
	}
	if draft.AccountID != prior.AccountID || draft.TransferAccountID != prior.TransferAccountID || draft.TransferGroupID != prior.TransferGroupID {
		return p, fmt.Errorf("change account or transfer classification with the transaction editor")
	}
	draft.Date = normalizeTransactionDate(draft.Date)
	all, err := a.store.ListTransactions()
	if err != nil {
		return p, err
	}
	p.target = prior
	before := []domain.Transaction{prior}
	if prior.TransferGroupID != "" && !prior.IsTransfer() {
		return p, fmt.Errorf("edit transfer fees with the transfer-aware editor")
	}
	if prior.IsTransfer() {
		pair, found := a.transferPartner(prior)
		if !found {
			return p, fmt.Errorf("transfer partner is unavailable; repair the transfer before editing")
		}
		if prior.Amount.Amount != 0 && draft.Amount.Amount != 0 && (prior.Amount.Amount < 0) != (draft.Amount.Amount < 0) {
			return p, fmt.Errorf("reverse a transfer with the transfer-aware editor")
		}
		if prior.TransferGroupID != "" {
			p.group = updateRelatedTransferRows(all, prior)
			legs := 0
			for _, row := range p.group {
				if row.IsTransfer() {
					legs++
					if row.ID != prior.ID && row.ID != pair.ID {
						return p, fmt.Errorf("transfer group has ambiguous extra legs")
					}
				}
			}
			if legs != 2 {
				return p, fmt.Errorf("transfer group must have exactly two legs")
			}
			before = p.group
		} else {
			if prior.Amount.Currency != pair.Amount.Currency {
				return p, fmt.Errorf("legacy cross-currency transfers need an explicit group before editing")
			}
			if txnclassify.AmbiguousLeg(prior, prior.TransferAccountID, all, a.Accounts()...) {
				return p, fmt.Errorf("legacy transfer partner is ambiguous")
			}
			before = append(before, pair)
		}
	}
	writes, err := a.planPairedTransactionEdit(prior, draft, 0)
	if err != nil {
		return p, err
	}
	after := cloneTransactions(before)
	for _, write := range writes {
		if issues := validate.ValidateTransaction(write, a.Accounts()...); !issues.OK() {
			return p, issues
		}
		if err := a.validateCustom("transaction", write.Custom); err != nil {
			return p, err
		}
		for i := range after {
			if after[i].ID == write.ID {
				after[i] = write
			}
		}
	}
	if prior.IsTransfer() {
		var directions []bool
		for _, row := range after {
			if row.IsTransfer() && row.Amount.Amount != 0 {
				account, found := domain.AccountByID(a.Accounts(), row.AccountID)
				if !found {
					return p, fmt.Errorf("transfer account is unavailable")
				}
				directions = append(directions, (row.Amount.Amount > 0) == (account.InflowSign() > 0))
			}
		}
		if len(directions) == 2 && directions[0] == directions[1] {
			return p, fmt.Errorf("transfer legs do not have reciprocal booking directions")
		}
	}
	for _, write := range writes {
		old, _ := findTransaction(before, write.ID)
		if old.Amount == write.Amount {
			continue
		}
		account, found := domain.AccountByID(a.Accounts(), write.AccountID)
		if !found {
			return p, fmt.Errorf("edited account is unavailable")
		}
		planned := accountTransactions(all, account.ID)
		for i := range planned {
			for _, replacement := range writes {
				if planned[i].ID == replacement.ID {
					planned[i] = replacement
				}
			}
		}
		if !containsID(p.balanceAccounts, account.ID) {
			p.balanceAccounts = append(p.balanceAccounts, account.ID)
			p.balanceRows = append(p.balanceRows, accountTransactions(all, account.ID)...)
		}
		if _, err := ledger.Balance(account, planned); err != nil {
			return p, fmt.Errorf("edited balance is unavailable: %w", err)
		}
	}
	refs := map[string]bool{}
	addGuard := func(table, id string, value any) error {
		key := table + "/" + id
		if id == "" || refs[key] {
			return nil
		}
		refs[key] = true
		c, err := creationRecord(table, id, value, value)
		if err == nil {
			p.changes = append(p.changes, c)
		}
		return err
	}
	for _, row := range append(append([]domain.Transaction(nil), before...), after...) {
		for _, id := range []string{row.AccountID, row.TransferAccountID} {
			if id != "" {
				account, found, err := a.store.GetAccount(id)
				if err != nil {
					return p, err
				}
				if !found {
					return p, fmt.Errorf("account ID %s is unavailable", id)
				}
				if err := addGuard("accounts", id, account); err != nil {
					return p, err
				}
			}
		}
		categoryIDs := []string{row.CategoryID}
		for _, split := range row.Splits {
			categoryIDs = append(categoryIDs, split.CategoryID)
		}
		for _, categoryID := range categoryIDs {
			if categoryID == "" {
				continue
			}
			category, found, err := a.store.GetCategory(categoryID)
			if err != nil {
				return p, err
			}
			var value any
			if found {
				value = category
				if row.ID == draft.ID && categoryID == draft.CategoryID && draft.CategoryID != prior.CategoryID {
					if issues := validate.ValidateCategory(category); !issues.OK() {
						return p, issues
					}
				}
			} else if row.ID == draft.ID && categoryID == draft.CategoryID && draft.CategoryID != prior.CategoryID {
				return p, fmt.Errorf("new category ID %s is unavailable", row.CategoryID)
			}
			if err := addGuard("categories", categoryID, value); err != nil {
				return p, err
			}
		}
		for _, id := range row.Owners() {
			member, found, err := a.store.GetMember(id)
			if err != nil {
				return p, err
			}
			var value any
			if found {
				value = member
			} else if row.ID == draft.ID && !containsID(prior.Owners(), id) {
				return p, fmt.Errorf("new member ID %s is unavailable", id)
			}
			if err := addGuard("members", id, value); err != nil {
				return p, err
			}
		}
	}
	if prior.IsTransfer() {
		settings, err := a.store.GetSettings()
		if err != nil {
			return p, err
		}
		settings.OpenAIKey = ""
		var value any
		if !sameJSON(settings, store.Settings{}) {
			value = settings
		}
		if err := addGuard("settings", "app", value); err != nil {
			return p, err
		}
	}
	sort.Slice(before, func(i, j int) bool { return before[i].ID < before[j].ID })
	sort.Slice(after, func(i, j int) bool { return after[i].ID < after[j].ID })
	for i := range before {
		change, err := creationRecord("transactions", before[i].ID, before[i], after[i])
		if err != nil {
			return p, err
		}
		p.changes = append(p.changes, change)
	}
	p.before, p.after = cloneTransactions(before), cloneTransactions(after)
	p.Before, p.After = cloneTransactions(before), cloneTransactions(after)
	return p, nil
}

// UpdateTransactionExpected commits only the exact validated preview. Every
// related record is checked in one SQL transaction; failures leave all rows intact.
func (a *App) UpdateTransactionExpected(p TransactionUpdatePreview) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if len(p.before) == 0 || !sameTransactionSet(p.Before, p.before) || !sameTransactionSet(p.After, p.after) {
		return fmt.Errorf("transaction preview is missing or was modified")
	}
	if p.target.TransferGroupID != "" {
		all, err := a.store.ListTransactions()
		if err != nil {
			return err
		}
		if !sameTransactionSet(updateRelatedTransferRows(all, p.target), p.group) {
			return fmt.Errorf("transfer group changed after preview")
		}
	}
	if p.target.IsTransfer() && p.target.TransferGroupID == "" {
		pair, found := a.transferPartner(p.target)
		if !found || !containsID(expectedIDs(p.before), pair.ID) || txnclassify.AmbiguousLeg(p.target, p.target.TransferAccountID, a.Transactions(), a.Accounts()...) {
			return fmt.Errorf("legacy transfer partner changed after preview")
		}
	}
	if len(p.balanceAccounts) > 0 {
		all, err := a.store.ListTransactions()
		if err != nil {
			return err
		}
		var live []domain.Transaction
		for _, id := range p.balanceAccounts {
			live = append(live, accountTransactions(all, id)...)
		}
		if !sameTransactionSet(live, p.balanceRows) {
			return fmt.Errorf("account ledger changed after preview")
		}
	}

	// Revalidate caller-independent custom-field constraints at the commit seam.
	// The immutable rows themselves never change after approval.
	for _, row := range p.after {
		if issues := validate.ValidateTransaction(row, a.Accounts()...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", row.Custom); err != nil {
			return err
		}
	}
	if sameTransactionSet(p.before, p.after) {
		return nil
	}
	if err := a.store.ReplayChangesExpected(p.changes); err != nil {
		return fmt.Errorf("update transaction: %w", err)
	}
	a.fireTxnMutated()
	return nil
}

// DeleteTransactionExpected removes the transaction and its current transfer
// partner, if any, only when expected contains the exact previewed deletion set.
// Every row and affected relationship is checked in the same store transaction.
func (a *App) DeleteTransactionExpected(targetID string, expected []domain.Transaction) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if targetID == "" || len(expected) == 0 {
		return fmt.Errorf("delete transaction: missing expected rows")
	}
	byID, err := uniqueExpectedTransactions(expected)
	if err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	if _, ok := byID[targetID]; !ok {
		return fmt.Errorf("delete transaction: target was not in the preview")
	}
	live := a.Transactions()
	ids := a.ExpandTransferPairs([]string{targetID})
	if !sameStringSet(ids, expectedIDs(expected)) {
		return fmt.Errorf("delete transaction: transfer group changed since preview")
	}
	for id, prior := range byID {
		current, ok := findTransaction(live, id)
		if !ok || !sameTransaction(current, prior) {
			return fmt.Errorf("delete transaction: %s changed since preview", id)
		}
	}
	changes, deleted, err := a.transactionDeleteChanges(ids)
	if err != nil {
		return fmt.Errorf("delete transaction: plan deletion: %w", err)
	}
	if err := setExpectedTransactionBefores(changes, byID); err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	a.afterTransactionsDeleted(changes, deleted)
	return nil
}

// MergeDuplicateTransactionsExpected merges one complete canonical duplicate
// group only while every previewed row and affected relationship remains
// unchanged. The merge result is computed here so callers cannot alter identity
// or financial fields in the surviving row.
func (a *App) MergeDuplicateTransactionsExpected(expected []domain.Transaction, survivorID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if survivorID == "" || len(expected) < 2 {
		return fmt.Errorf("merge duplicates: expected a complete group and survivor")
	}
	expectedByID, err := uniqueExpectedTransactions(expected)
	if err != nil {
		return fmt.Errorf("merge duplicates: %w", err)
	}
	survivor, ok := expectedByID[survivorID]
	if !ok {
		return fmt.Errorf("merge duplicates: survivor was not in the preview")
	}
	live := a.Transactions()
	liveByID := make(map[string]domain.Transaction, len(live))
	for _, t := range live {
		liveByID[t.ID] = t
	}
	for id, prior := range expectedByID {
		current, ok := liveByID[id]
		if !ok || !sameTransaction(current, prior) {
			return fmt.Errorf("merge duplicates: %s changed since preview", id)
		}
	}
	var canonical []string
	for _, group := range dedupe.FindDuplicates(live) {
		if containsID(group.IDs, survivorID) {
			canonical = group.IDs
			break
		}
	}
	if !sameStringSet(canonical, expectedIDs(expected)) {
		return fmt.Errorf("merge duplicates: duplicate group changed since preview")
	}
	others := make([]domain.Transaction, 0, len(expected)-1)
	for _, id := range canonical {
		if id != survivorID {
			others = append(others, expectedByID[id])
		}
	}
	merged := dedupe.Merge(survivor, others)
	deleteIDs := make([]string, 0, len(others))
	for _, other := range others {
		deleteIDs = append(deleteIDs, other.ID)
	}
	changes, deleted, err := a.transactionDeleteChanges(deleteIDs)
	if err != nil {
		return fmt.Errorf("merge duplicates: plan deletion: %w", err)
	}
	if len(deleted) != len(deleteIDs) {
		return fmt.Errorf("merge duplicates: a group member is no longer present")
	}
	deleteExpected := make(map[string]domain.Transaction, len(others))
	for _, other := range others {
		deleteExpected[other.ID] = other
	}
	if err := setExpectedTransactionBefores(changes, deleteExpected); err != nil {
		return fmt.Errorf("merge duplicates: %w", err)
	}
	before, err := json.Marshal(survivor)
	if err != nil {
		return fmt.Errorf("merge duplicates: encode survivor: %w", err)
	}
	after, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("merge duplicates: encode merged survivor: %w", err)
	}
	changes = append(changes, store.RecordChange{Table: "transactions", ID: survivorID, Before: before, After: after})
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("merge duplicates: %w", err)
	}
	a.afterTransactionsDeleted(changes, deleted)
	return nil
}

func uniqueExpectedTransactions(rows []domain.Transaction) (map[string]domain.Transaction, error) {
	byID := make(map[string]domain.Transaction, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			return nil, fmt.Errorf("expected row has no id")
		}
		if _, exists := byID[row.ID]; exists {
			return nil, fmt.Errorf("duplicate expected row %q", row.ID)
		}
		byID[row.ID] = row
	}
	return byID, nil
}

func setExpectedTransactionBefores(changes []store.RecordChange, expected map[string]domain.Transaction) error {
	seen := make(map[string]bool, len(expected))
	for i := range changes {
		if changes[i].Table != "transactions" {
			continue
		}
		prior, ok := expected[changes[i].ID]
		if !ok {
			continue
		}
		before, err := json.Marshal(prior)
		if err != nil {
			return fmt.Errorf("encode expected transaction %s: %w", prior.ID, err)
		}
		changes[i].Before = before
		seen[prior.ID] = true
	}
	for id := range expected {
		if !seen[id] {
			return fmt.Errorf("expected transaction %s is absent from the write plan", id)
		}
	}
	return nil
}

func expectedIDs(rows []domain.Transaction) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	sort.Strings(ids)
	return ids
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func findTransaction(rows []domain.Transaction, id string) (domain.Transaction, bool) {
	for _, row := range rows {
		if row.ID == id {
			return row, true
		}
	}
	return domain.Transaction{}, false
}

func sameTransaction(a, b domain.Transaction) bool {
	left, err1 := json.Marshal(a)
	right, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(left) == string(right)
}
