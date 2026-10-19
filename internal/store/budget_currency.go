// SPDX-License-Identifier: MIT

package store

import "github.com/monstercameron/CashFlux/internal/domain"

// backfillBudgetCurrencies repairs the legacy implicit-currency representation
// once on ingress. Explicit currencies and the caller's dataset are preserved.
func backfillBudgetCurrencies(ds Dataset) Dataset {
	if ds.Budgets == nil {
		return ds
	}
	budgets := make([]domain.Budget, len(ds.Budgets))
	for i, b := range ds.Budgets {
		budgets[i] = b.WithBaseCurrency(ds.Settings.BaseCurrency)
	}
	ds.Budgets = budgets
	return ds
}
