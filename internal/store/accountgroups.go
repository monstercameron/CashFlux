// SPDX-License-Identifier: MIT

package store

import "github.com/monstercameron/CashFlux/internal/domain"

// --- Account groups (user-defined /accounts groupings — AC1) ---

// PutAccountGroup inserts or updates an account group by id.
func (s *SQLiteStore) PutAccountGroup(g domain.AccountGroup) error {
	return putJSON(s.db, "accountgroups", g.ID, g)
}

// GetAccountGroup returns the account group with the given id.
func (s *SQLiteStore) GetAccountGroup(id string) (domain.AccountGroup, bool, error) {
	return getJSON[domain.AccountGroup](s.db, "accountgroups", id)
}

// DeleteAccountGroup removes an account group by id. Deleting a group only
// ungroups its accounts (reassign-on-delete); the accounts are never touched.
func (s *SQLiteStore) DeleteAccountGroup(id string) (bool, error) {
	return deleteRow(s.db, "accountgroups", id)
}

// ListAccountGroups returns all account-group rows.
func (s *SQLiteStore) ListAccountGroups() ([]domain.AccountGroup, error) {
	return loadRows[domain.AccountGroup](s.db, "accountgroups")
}
