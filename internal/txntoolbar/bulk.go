// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"reflect"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnreview"
)

// EditKind identifies a bulk draft operation; persistence remains with the caller.
type EditKind uint8

// Bulk draft operations.
const (
	Clear EditKind = iota
	Review
	Exclude
	Category
	Owners
	Tag
)

// Edit describes a bulk change without mutating the original ledger snapshot.
type Edit struct {
	Kind   EditKind
	Value  bool
	Text   string
	Owners []string
	Now    time.Time
}

// Draft returns only changed, selected rows. Transfers cannot receive categories.
// Ownership and tags are copied so prior snapshots remain safe for undo.
func Draft(rows []domain.Transaction, selected map[string]bool, edit Edit) []domain.Transaction {
	var writes []domain.Transaction
	for _, row := range rows {
		if !selected[row.ID] {
			continue
		}
		updated := row
		switch edit.Kind {
		case Clear:
			if row.Cleared == edit.Value {
				continue
			}
			updated = row.MarkCleared(edit.Value, edit.Now)
		case Review:
			updated = txnreview.Confirm(row, row.CategoryID)
		case Exclude:
			updated.ExcludeFromReports = edit.Value
		case Category:
			if row.IsTransfer() {
				continue
			}
			updated = txnreview.Confirm(row, edit.Text)
		case Owners:
			if SameOwners(row.Owners(), edit.Owners) {
				continue
			}
			updated.SetOwners(append([]string(nil), edit.Owners...))
		case Tag:
			if strings.TrimSpace(edit.Text) == "" || HasTag(row.Tags, edit.Text) {
				continue
			}
			updated.Tags = AddTag(row.Tags, edit.Text)
		default:
			continue
		}
		if !reflect.DeepEqual(row, updated) {
			updated.Tags = append([]string(nil), updated.Tags...)
			updated.MemberIDs = append([]string(nil), updated.MemberIDs...)
			writes = append(writes, updated)
		}
	}
	return writes
}

// SelectedRows returns ledger-order rows, optionally excluding transfers.
func SelectedRows(rows []domain.Transaction, selected map[string]bool, ordinaryOnly bool) []domain.Transaction {
	result := make([]domain.Transaction, 0, len(selected))
	for _, row := range rows {
		if selected[row.ID] && (!ordinaryOnly || !row.IsTransfer()) {
			result = append(result, row)
		}
	}
	return result
}

// IDs returns row IDs in the same deterministic ledger order.
func IDs(rows []domain.Transaction) []string {
	result := make([]string, len(rows))
	for i, row := range rows {
		result[i] = row.ID
	}
	return result
}

// CategoryKind defaults to expense and follows the first selected ordinary row.
func CategoryKind(rows []domain.Transaction, selected map[string]bool) domain.CategoryKind {
	for _, row := range rows {
		if selected[row.ID] && !row.IsTransfer() {
			if row.Amount.Amount > 0 {
				return domain.KindIncome
			}
			break
		}
	}
	return domain.KindExpense
}
