// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTV7EMemberRoleIsSecondaryToMemberName(t *testing.T) {
	members := []domain.Member{
		{ID: "marcus", Name: "Marcus Hartley", Role: domain.RoleOwner},
	}
	if got := memberSwitcherRole("marcus", members); got != "Owner" {
		t.Fatalf("selected member role = %q, want secondary role label", got)
	}
	if got := memberSwitcherRole("", members); got != "" {
		t.Fatalf("everyone role = %q, want no secondary role", got)
	}
}
