// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

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
