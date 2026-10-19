// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// RegularSpendOverrides returns the persisted baseline-only household
// corrections. They do not change reports, budgets, or transaction categories.
func (a *App) RegularSpendOverrides() domain.RegularSpendOverrides {
	overrides, err := a.store.GetRegularSpendOverrides()
	a.logErr("regular-spend overrides", err)
	return overrides
}

// SetRegularSpendEssentialCategory marks a category as essential for this
// baseline only. Unknown category IDs are rejected.
func (a *App) SetRegularSpendEssentialCategory(categoryID string, essential bool) error {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return fmt.Errorf("appstate: regular-spend category id is required")
	}
	if essential {
		found := false
		for _, category := range a.Categories() {
			if category.ID == categoryID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("appstate: regular-spend category %q does not exist", categoryID)
		}
	}
	return a.updateRegularSpendOverrides(func(overrides *domain.RegularSpendOverrides) {
		overrides.EssentialCategoryIDs = setString(overrides.EssentialCategoryIDs, categoryID, essential)
	})
}

// SetRegularSpendOneOffTransaction excludes a transaction from this baseline
// only. Reports and transaction classification remain unchanged.
func (a *App) SetRegularSpendOneOffTransaction(transactionID string, oneOff bool) error {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return fmt.Errorf("appstate: regular-spend transaction id is required")
	}
	if oneOff {
		found := false
		for _, txn := range a.Transactions() {
			if txn.ID == transactionID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("appstate: regular-spend transaction %q does not exist", transactionID)
		}
	}
	return a.updateRegularSpendOverrides(func(overrides *domain.RegularSpendOverrides) {
		overrides.OneOffTransactionIDs = setString(overrides.OneOffTransactionIDs, transactionID, oneOff)
	})
}

// SetRegularSpendMerchantRegular records or removes a normalized merchant key
// accepted as regular by the baseline.
func (a *App) SetRegularSpendMerchantRegular(merchant string, regular bool) error {
	merchant = normalizeRegularSpendMerchant(merchant)
	if merchant == "" {
		return fmt.Errorf("appstate: regular-spend merchant is required")
	}
	return a.updateRegularSpendOverrides(func(overrides *domain.RegularSpendOverrides) {
		overrides.TreatMerchantRegular = setString(overrides.TreatMerchantRegular, merchant, regular)
	})
}

func (a *App) updateRegularSpendOverrides(update func(*domain.RegularSpendOverrides)) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	overrides, err := a.store.GetRegularSpendOverrides()
	if err != nil {
		return fmt.Errorf("appstate: read regular-spend overrides before update: %w", err)
	}
	update(&overrides)
	if err := a.store.PutRegularSpendOverrides(overrides); err != nil {
		return fmt.Errorf("appstate: save regular-spend overrides: %w", err)
	}
	return nil
}

func setString(values []string, value string, enabled bool) []string {
	result := make([]string, 0, len(values)+1)
	found := false
	for _, current := range values {
		if current == value {
			if enabled && !found {
				result = append(result, current)
				found = true
			}
			continue
		}
		result = append(result, current)
	}
	if enabled && !found {
		result = append(result, value)
	}
	return result
}

func normalizeRegularSpendMerchant(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
