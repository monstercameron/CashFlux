// SPDX-License-Identifier: MIT

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// deriveTxnRows applies the ledger criteria and then orders the Description column
// by the text it displays. The input is not mutated; the returned slice is owned by
// the caller and can be retained by a render memo until its dependencies change.
func deriveTxnRows(txns []domain.Transaction, criteria txnfilter.Criteria, labels txnfilter.Labels, hasLearned func(string) bool, resolve func(string) string) []domain.Transaction {
	defer txnPerfStart("filter-sort")()
	labels.Payee = memoTxnLabel(labels.Payee)
	rows, _ := txnfilter.ApplyWithLabelsChecked(txns, criteria, labels)
	if criteria.Normalize().Sort == "payee" {
		sortTxnDescriptions(rows, criteria.Normalize().Dir, hasLearned, resolve)
	}
	return rows
}

func memoTxnLabel(resolve func(string) string) func(string) string {
	if resolve == nil {
		return nil
	}
	values := make(map[string]string)
	return func(raw string) string {
		if value, ok := values[raw]; ok {
			return value
		}
		value := resolve(raw)
		values[raw] = value
		return value
	}
}

// txnDisplayDescription returns the exact description text the ledger row shows.
func txnDisplayDescription(desc, payee string, hasLearned func(string) bool, resolve func(string) string) string {
	if strings.TrimSpace(payee) != "" && hasLearned != nil && hasLearned(payee) {
		return resolve(payee)
	}
	if strings.TrimSpace(desc) == "" {
		return resolve(payee)
	}
	return desc
}

// sortTxnDescriptions orders transactions by their rendered Description cell.
// Ties always use ID ascending, independent of sort direction.
func sortTxnDescriptions(txns []domain.Transaction, dir string, hasLearned func(string) bool, resolve func(string) string) {
	keys := make(map[string]string, len(txns))
	for _, txn := range txns {
		keys[txn.ID] = strings.ToLower(txnDisplayDescription(txn.Desc, txn.Payee, hasLearned, resolve))
	}
	sort.Slice(txns, func(i, j int) bool {
		cmp := strings.Compare(keys[txns[i].ID], keys[txns[j].ID])
		if cmp == 0 {
			return txns[i].ID < txns[j].ID
		}
		if dir == txnfilter.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}
