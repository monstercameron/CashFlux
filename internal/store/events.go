// SPDX-License-Identifier: MIT

package store

import "github.com/monstercameron/CashFlux/internal/domain"

// --- Events (first-class spending events — TX10) ---

// PutEvent inserts or updates an event row by id.
func (s *SQLiteStore) PutEvent(e domain.Event) error {
	return putJSON(s.db, "events", e.ID, e)
}

// GetEvent returns the event with the given id.
func (s *SQLiteStore) GetEvent(id string) (domain.Event, bool, error) {
	return getJSON[domain.Event](s.db, "events", id)
}

// DeleteEvent removes an event row by id. Callers unmap the event's transactions
// (delete its event-member links) separately — the transactions are never
// touched.
func (s *SQLiteStore) DeleteEvent(id string) (bool, error) {
	return deleteRow(s.db, "events", id)
}

// ListEvents returns all event rows.
func (s *SQLiteStore) ListEvents() ([]domain.Event, error) {
	return loadRows[domain.Event](s.db, "events")
}
