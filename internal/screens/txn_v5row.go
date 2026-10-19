// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// txnV5RowTransfers keeps one representative for each transfer movement when
// both legs are in the current result set. An account-scoped view keeps its
// matching leg so the register remains useful.
func txnV5RowTransfers(rows []domain.Transaction, accountScoped bool, rateTables ...currency.Rates) []domain.Transaction {
	if accountScoped || len(rows) < 2 {
		return rows
	}
	summary := txnclassify.SummarizeVisibleTransfers(rows, rateTables...)
	kept := make([]domain.Transaction, 0, len(rows))
	for i, row := range rows {
		if !row.IsTransfer() || summary.CountedRows[i] {
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
	_, single := (txnfilter.Criteria{Account: strings.TrimSpace(criteriaAccount), Accounts: criteriaAccounts}).SingleAccount()
	return single
}

func txnEditBalanceAccountCount(before, after domain.Transaction) int {
	if before.AccountID != after.AccountID || before.IsTransfer() || after.IsTransfer() {
		return 2
	}
	return 1
}

// txnV5RowTransferLabel returns the household-facing direction between accounts.
func txnV5RowTransferLabel(from, to string, outgoing ...bool) string {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return ""
	}
	if len(outgoing) > 0 && !outgoing[0] {
		from, to = to, from
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
