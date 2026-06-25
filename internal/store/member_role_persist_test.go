// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
)

// TestMemberRoleRoundTrip verifies that each MemberRole value survives a
// PutMember → GetMember round-trip through the SQLite JSON store.
func TestMemberRoleRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		m    domain.Member
	}{
		{
			name: "owner role",
			m:    domain.Member{ID: "r1", Name: "Alice", IsDefault: true, Role: domain.RoleOwner},
		},
		{
			name: "admin role",
			m:    domain.Member{ID: "r2", Name: "Bob", Role: domain.RoleAdmin},
		},
		{
			name: "viewer role",
			m:    domain.Member{ID: "r3", Name: "Carol", Role: domain.RoleViewer},
		},
	}
	s := newStore(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.PutMember(tc.m); err != nil {
				t.Fatalf("PutMember: %v", err)
			}
			got, ok, err := s.GetMember(tc.m.ID)
			if err != nil || !ok {
				t.Fatalf("GetMember: ok=%v err=%v", ok, err)
			}
			if got.Role != tc.m.Role {
				t.Errorf("Role = %q, want %q", got.Role, tc.m.Role)
			}
		})
	}
}

// TestMemberRoleLegacyDefault verifies that a row persisted without a role
// field (simulating a legacy dataset) resolves correctly via memberrole.Resolve:
// IsDefault=true → RoleOwner, IsDefault=false → RoleAdmin.
func TestMemberRoleLegacyDefault(t *testing.T) {
	s := newStore(t)

	// Persist a member without a Role field by marshalling a legacy struct that
	// lacks the field, then inserting the raw JSON directly.
	type legacyMember struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"isDefault,omitempty"`
	}
	for _, tc := range []struct {
		id        string
		isDefault bool
		want      domain.MemberRole
	}{
		{"leg-owner", true, domain.RoleOwner},
		{"leg-admin", false, domain.RoleAdmin},
	} {
		data, _ := json.Marshal(legacyMember{ID: tc.id, Name: tc.id, IsDefault: tc.isDefault})
		if _, err := s.db.Exec(
			"INSERT INTO members(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data",
			tc.id, string(data),
		); err != nil {
			t.Fatalf("raw insert: %v", err)
		}
		got, ok, err := s.GetMember(tc.id)
		if err != nil || !ok {
			t.Fatalf("GetMember %q: ok=%v err=%v", tc.id, ok, err)
		}
		// Role field is empty in the stored JSON.
		if got.Role != "" {
			t.Errorf("legacy row %q: stored Role = %q, want empty", tc.id, got.Role)
		}
		// Resolve applies the migration default.
		effective := memberrole.Resolve(got)
		if effective != tc.want {
			t.Errorf("Resolve(legacy %q) = %q, want %q", tc.id, effective, tc.want)
		}
	}
}

// TestMemberRoleDatasetRoundTrip verifies that Role survives an export (Snapshot)
// → import (Load) cycle through the Dataset JSON representation.
func TestMemberRoleDatasetRoundTrip(t *testing.T) {
	s := newStore(t)
	members := []domain.Member{
		{ID: "d1", Name: "Alice", IsDefault: true, Role: domain.RoleOwner},
		{ID: "d2", Name: "Bob", Role: domain.RoleAdmin},
		{ID: "d3", Name: "Carol", Role: domain.RoleViewer},
	}
	for _, m := range members {
		if err := s.PutMember(m); err != nil {
			t.Fatalf("PutMember %q: %v", m.ID, err)
		}
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Import into a fresh store.
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, want := range members {
		got, ok, err := s2.GetMember(want.ID)
		if err != nil || !ok {
			t.Fatalf("GetMember %q after Load: ok=%v err=%v", want.ID, ok, err)
		}
		if got.Role != want.Role {
			t.Errorf("after Load: member %q Role = %q, want %q", want.ID, got.Role, want.Role)
		}
	}
}
