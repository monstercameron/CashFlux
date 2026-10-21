// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TransactionsChecked returns the immutable ledger snapshot or its storage
// error. Transactions is the best-effort convenience accessor for UI paths
// that intentionally treat an unreadable table as empty.
func (a *App) TransactionsChecked() ([]domain.Transaction, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read transactions: local dataset is unavailable")
	}
	snapshot, err := a.reads.transactions.get(a.store)
	if err != nil {
		return nil, fmt.Errorf("read transactions: %w", err)
	}
	if snapshot == nil {
		return nil, fmt.Errorf("read transactions: local dataset is unavailable")
	}
	return snapshot.rows, nil
}

// AccountsChecked reads accounts without hiding a storage failure behind an
// empty slice. Use it when an empty result would be presented as a valid plan.
func (a *App) AccountsChecked() ([]domain.Account, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read accounts: local dataset is unavailable")
	}
	if err := a.store.CheckReady(); err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	rows, err := a.reads.accounts.getChecked(a.store.Rev(), a.store.ListAccounts)
	if err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	return rows, nil
}

// GoalsChecked reads goals without hiding a storage failure behind an empty
// slice. Use it when an empty result would be presented as a valid plan.
func (a *App) GoalsChecked() ([]domain.Goal, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read goals: local dataset is unavailable")
	}
	if err := a.store.CheckReady(); err != nil {
		return nil, fmt.Errorf("read goals: %w", err)
	}
	rows, err := a.reads.goals.getChecked(a.store.Rev(), a.store.ListGoals)
	if err != nil {
		return nil, fmt.Errorf("read goals: %w", err)
	}
	return rows, nil
}

// BudgetsChecked returns budgets without turning a storage failure into a valid
// empty collection. Use it when category usage or a prepared view depends on the
// complete set of budget references.
func (a *App) BudgetsChecked() ([]domain.Budget, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read budgets: local dataset is unavailable")
	}
	if err := a.store.CheckReady(); err != nil {
		return nil, fmt.Errorf("read budgets: %w", err)
	}
	rows, err := a.reads.budgets.getChecked(a.store.Rev(), a.store.ListBudgets)
	if err != nil {
		return nil, fmt.Errorf("read budgets: %w", err)
	}
	return rows, nil
}

// TxnLinksChecked reads current transaction links without hiding storage
// failures, allowing reports to distinguish no refund links from unknown links.
func (a *App) TxnLinksChecked() ([]domain.TxnLink, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read transaction links: local dataset is unavailable")
	}
	links, err := a.store.ListTxnLinks()
	if err != nil {
		return nil, fmt.Errorf("read transaction links: %w", err)
	}
	return links, nil
}
