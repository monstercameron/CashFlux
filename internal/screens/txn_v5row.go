// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// txnV5RowTransfers keeps one representative for each transfer movement when
// both legs are in the current result set. An account-scoped view keeps its
// matching leg so the register remains useful.
func txnV5RowTransfers(rows []domain.Transaction, accountScoped bool) []domain.Transaction {
	if accountScoped || len(rows) < 2 {
		return rows
	}
	summary := txnclassify.SummarizeVisibleTransfers(rows)
	kept := make([]domain.Transaction, 0, len(rows))
	for i, row := range rows {
		// The legacy matcher is intentionally not trusted for row hiding: a
		// look-alike amount/date match can otherwise make a real ledger row vanish.
		if !row.IsTransfer() || row.TransferGroupID == "" || summary.CountedRows[i] {
			kept = append(kept, row)
		}
	}
	return kept
}

// txnV5RowExpandedSelection adds reciprocal transfer legs to a selection.
func txnV5RowExpandedSelection(selected map[string]bool, expandedIDs []string) map[string]bool {
	result := make(map[string]bool, len(expandedIDs))
	for _, id := range expandedIDs {
		result[id] = true
	}
	for id := range selected {
		result[id] = true
	}
	return result
}

func txnV5RowAccountScoped(criteriaAccount, criteriaAccounts string) bool {
	return strings.TrimSpace(criteriaAccount) != "" || strings.TrimSpace(criteriaAccounts) != ""
}

func txnV5RowCountTotal(rows []domain.Transaction, criteriaAccount, criteriaAccounts string) int {
	if txnV5RowAccountScoped(criteriaAccount, criteriaAccounts) {
		return len(rows)
	}
	return len(txnV5RowTransfers(rows, false))
}

func txnEditBalanceAccountCount(before, after domain.Transaction) int {
	if before.AccountID != after.AccountID || before.IsTransfer() || after.IsTransfer() {
		return 2
	}
	return 1
}

// txnV5RowTransferLabel returns the household-facing direction between accounts.
func txnV5RowTransferLabel(from, to string) string {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return ""
	}
	return from + " → " + to
}

func txnV5RowTransferDisplay(category, counterparty string, outgoing bool) string {
	if strings.Contains(category, " → ") {
		return category
	}
	if outgoing {
		return uistate.T("r4c.transferTo", counterparty)
	}
	return uistate.T("r4c.transferFrom", counterparty)
}

func txnV5RowControlName(control, value, description string) string {
	return control + ": " + value + ", " + description
}
