// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"
)

// DefaultAccountID returns the saved household fallback account for new
// transactions. Missing or archived IDs are returned as stored; selection
// ignores them without rewriting the setting.
func (a *App) DefaultAccountID() string {
	return a.Settings().DefaultAccountID
}

// SetDefaultAccountID saves a valid, active account as the household fallback
// for new transactions. The write completes through the store's settings path
// before this method returns successfully.
func (a *App) SetDefaultAccountID(accountID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("appstate: default account id is required")
	}
	account, found, err := a.store.GetAccount(accountID)
	if err != nil {
		return fmt.Errorf("appstate: validate default account %q: %w", accountID, err)
	}
	if !found {
		return fmt.Errorf("appstate: default account %q not found", accountID)
	}
	if account.Archived {
		return fmt.Errorf("appstate: archived account %q cannot be the default", accountID)
	}
	settings := a.Settings()
	settings.DefaultAccountID = accountID
	if err := a.PutSettings(settings); err != nil {
		return fmt.Errorf("appstate: save default account %q: %w", accountID, err)
	}
	return nil
}

// ClearDefaultAccountID removes the household fallback for new transactions.
func (a *App) ClearDefaultAccountID() error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	settings := a.Settings()
	settings.DefaultAccountID = ""
	if err := a.PutSettings(settings); err != nil {
		return fmt.Errorf("appstate: clear default account: %w", err)
	}
	return nil
}
