// SPDX-License-Identifier: MIT

package store

import "github.com/monstercameron/CashFlux/internal/domain"

// PutSweepRule inserts or updates a surplus-sweep rule by id (AC7).
func (s *SQLiteStore) PutSweepRule(r domain.SweepRule) error {
	return putJSON(s.db, "sweeprules", r.ID, r)
}

// GetSweepRule returns the sweep rule with the given id.
func (s *SQLiteStore) GetSweepRule(id string) (domain.SweepRule, bool, error) {
	return getJSON[domain.SweepRule](s.db, "sweeprules", id)
}

// DeleteSweepRule removes a sweep rule by id. Returns true if a row was deleted.
func (s *SQLiteStore) DeleteSweepRule(id string) (bool, error) {
	return deleteRow(s.db, "sweeprules", id)
}

// ListSweepRules returns all sweep rules.
func (s *SQLiteStore) ListSweepRules() ([]domain.SweepRule, error) {
	return loadRows[domain.SweepRule](s.db, "sweeprules")
}
