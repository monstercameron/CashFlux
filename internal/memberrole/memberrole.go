// SPDX-License-Identifier: MIT

// Package memberrole provides pure permission logic for household member roles.
// It has no syscall/js dependency and is safe to unit-test on native Go.
//
// The three roles form a strict hierarchy:
//
//	owner > admin > viewer
//
// RoleOwner can do everything including managing members.
// RoleAdmin can read and edit all financial entities but cannot manage members.
// RoleViewer can only read — no creates, edits, or deletes.
//
// Legacy rows (Role == "") are treated as RoleAdmin so existing members retain
// full access after the schema migration that introduced this field.
package memberrole

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// DefaultRole returns the role that should be assigned to a member when none
// is explicitly provided. The default member (IsDefault=true) is the household
// owner; all other members default to admin.
func DefaultRole(isDefault bool) domain.MemberRole {
	if isDefault {
		return domain.RoleOwner
	}
	return domain.RoleAdmin
}

// Resolve returns the effective role for a member, applying the migration
// default for legacy rows whose Role field is the zero value.
func Resolve(m domain.Member) domain.MemberRole {
	if m.Role == "" {
		return DefaultRole(m.IsDefault)
	}
	return m.Role
}

// Valid reports whether r is one of the three defined role constants.
// The zero value ("") is not valid — callers should call Resolve first.
func Valid(r domain.MemberRole) bool {
	switch r {
	case domain.RoleOwner, domain.RoleAdmin, domain.RoleViewer:
		return true
	default:
		return false
	}
}

// ParseRole converts a raw string (e.g. from a form input or import) to a
// MemberRole. It returns an error for any string that is not a known role.
func ParseRole(s string) (domain.MemberRole, error) {
	r := domain.MemberRole(s)
	if !Valid(r) {
		return "", fmt.Errorf("memberrole: unknown role %q (want owner|admin|viewer)", s)
	}
	return r, nil
}

// Label returns a short human-readable label for r, suitable for display in a
// select control or a member list. It panics on an invalid role — callers
// should validate with Valid or ParseRole before calling Label.
func Label(r domain.MemberRole) string {
	switch r {
	case domain.RoleOwner:
		return "Owner"
	case domain.RoleAdmin:
		return "Admin"
	case domain.RoleViewer:
		return "Viewer"
	default:
		panic(fmt.Sprintf("memberrole: Label called with invalid role %q", r))
	}
}

// CanManageMembers reports whether a member with role r may add, edit, or
// remove other members. Only RoleOwner has this permission.
func CanManageMembers(r domain.MemberRole) bool {
	return r == domain.RoleOwner
}

// CanEditEntities reports whether a member with role r may create, edit, or
// delete financial entities (accounts, transactions, budgets, goals, etc.).
// RoleOwner and RoleAdmin both have this permission; RoleViewer does not.
func CanEditEntities(r domain.MemberRole) bool {
	return r == domain.RoleOwner || r == domain.RoleAdmin
}

// CanViewOnly reports whether the member is restricted to read-only access.
// This is the inverse of CanEditEntities (and implies CanManageMembers is false).
func CanViewOnly(r domain.MemberRole) bool {
	return r == domain.RoleViewer
}
