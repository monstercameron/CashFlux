// SPDX-License-Identifier: MIT

package server

import (
	"fmt"
	"time"
)

// PhoneClientRow is one enrolled phone/SMS account, for admin listing
// (pkg/embed.Admin.ListClients).
type PhoneClientRow struct {
	ID              string
	PhoneNumber     string
	CreatedAt       time.Time
	PhoneVerifiedAt time.Time
	Suspended       bool
}

// ListPhoneClients returns enrolled phone/SMS accounts, newest first, capped
// at limit. Filtered to a non-empty phone_verified_at — ensurePhoneUser
// upserts a user row the moment someone merely REQUESTS a verification code,
// before they ever complete it, so without this filter an abandoned attempt
// would show up as a "client" alongside genuinely enrolled accounts.
func (s *Store) ListPhoneClients(limit int) ([]PhoneClientRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("server store: not configured")
	}
	if limit <= 0 {
		limit = 200
	}
	defer s.observeDB("ListPhoneClients", time.Now())
	rows, err := s.db.Query(`
SELECT id, subject, created_at, phone_verified_at, suspended_at
FROM users
WHERE provider = 'phone' AND phone_verified_at != ''
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("server store: list phone clients: %w", err)
	}
	defer rows.Close()
	var out []PhoneClientRow
	for rows.Next() {
		var (
			id, phone                        string
			createdRaw, verifiedRaw, suspRaw string
		)
		if err := rows.Scan(&id, &phone, &createdRaw, &verifiedRaw, &suspRaw); err != nil {
			return nil, fmt.Errorf("server store: scan phone client: %w", err)
		}
		createdAt, err := parseTime(createdRaw)
		if err != nil {
			return nil, fmt.Errorf("server store: parse phone client created_at: %w", err)
		}
		verifiedAt, err := parseTime(verifiedRaw)
		if err != nil {
			return nil, fmt.Errorf("server store: parse phone client phone_verified_at: %w", err)
		}
		out = append(out, PhoneClientRow{
			ID: id, PhoneNumber: phone, CreatedAt: createdAt, PhoneVerifiedAt: verifiedAt,
			Suspended: suspRaw != "",
		})
	}
	return out, rows.Err()
}
