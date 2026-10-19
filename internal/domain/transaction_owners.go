// SPDX-License-Identifier: MIT

package domain

import (
	"bytes"
	"encoding/json"
)

// Owners returns the transaction's stable, deduplicated owners. It also reads
// the legacy MemberID field for transactions constructed by older callers.
func (t Transaction) Owners() []string {
	owners := t.MemberIDs
	if len(owners) == 0 && t.MemberID != "" {
		owners = []string{t.MemberID}
	}
	return uniqueOwnerIDs(owners)
}

// HasOwner reports whether id owns the transaction.
func (t Transaction) HasOwner(id string) bool {
	if id == "" {
		return false
	}
	for _, owner := range t.Owners() {
		if owner == id {
			return true
		}
	}
	return false
}

// IsUnassigned reports whether no member owns the transaction.
func (t Transaction) IsUnassigned() bool { return len(t.Owners()) == 0 }

// PrimaryOwner returns the first owner for legacy single-owner interfaces.
// Shared-owner displays should use Owners instead.
func (t Transaction) PrimaryOwner() string {
	owners := t.Owners()
	if len(owners) == 0 {
		return ""
	}
	return owners[0]
}

// NormalizeOwners migrates legacy ownership and canonicalizes the owner list.
// Call it at mutation boundaries when subsequent code needs canonical fields.
func (t *Transaction) NormalizeOwners() {
	if t == nil {
		return
	}
	t.MemberIDs = t.Owners()
	t.MemberID = ""
}

// SetOwners replaces transaction ownership with stable, deduplicated ids.
func (t *Transaction) SetOwners(owners []string) {
	if t == nil {
		return
	}
	t.MemberIDs = uniqueOwnerIDs(owners)
	t.MemberID = ""
}

func uniqueOwnerIDs(owners []string) []string {
	seen := make(map[string]struct{}, len(owners))
	out := make([]string, 0, len(owners))
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		if _, exists := seen[owner]; exists {
			continue
		}
		seen[owner] = struct{}{}
		out = append(out, owner)
	}
	return out
}

type transactionJSON Transaction

// MarshalJSON writes only the canonical MemberIDs ownership field.
func (t Transaction) MarshalJSON() ([]byte, error) {
	t.NormalizeOwners()
	return json.Marshal(transactionJSON(t))
}

// UnmarshalJSON migrates legacy single-owner transactions as they are loaded.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	var decoded transactionJSON
	if err := json.Unmarshal(bytes.TrimSpace(data), &decoded); err != nil {
		return err
	}
	*t = Transaction(decoded)
	t.NormalizeOwners()
	return nil
}
