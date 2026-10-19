// SPDX-License-Identifier: MIT

package store

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/auditlog"
)

// ListAuditEntriesByEntity returns audit entries for one entity, newest first.
// A non-positive limit returns every matching entry.
func (s *SQLiteStore) ListAuditEntriesByEntity(entityType, entityID string, limit int) ([]auditlog.Entry, error) {
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS audit_log_entity_idx ON audit_log (
		json_extract(data, '$.EntityType'), json_extract(data, '$.EntityID'),
		json_extract(data, '$.At') DESC
	)`); err != nil {
		return nil, fmt.Errorf("store: ensure audit entity index: %w", err)
	}

	query := `SELECT data FROM audit_log
		WHERE json_extract(data, '$.EntityType') = ? AND json_extract(data, '$.EntityID') = ?
		ORDER BY json_extract(data, '$.At') DESC, id DESC`
	args := []any{entityType, entityID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	entries, err := queryRows[auditlog.Entry](s.db, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list audit entries for %s %q: %w", entityType, entityID, err)
	}
	return entries, nil
}
