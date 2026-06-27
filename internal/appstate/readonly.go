// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
)

// ErrReadOnly is returned by entity-mutating methods (PutX / DeleteX for
// accounts, transactions, budgets, goals, rules, members, etc.) when the active
// identity resolves to a read-only role (RoleViewer). It is a sentinel that
// callers — UI layers, tests — can detect with errors.Is.
var ErrReadOnly = errors.New("appstate: active identity is read-only (Viewer role)")

// SetActiveRoleFunc installs a function that the App calls on every mutating
// operation to discover the current active identity's role. The function
// receives no arguments and returns the effective domain.MemberRole for
// whoever is operating the app right now.
//
// This is the injection seam that keeps appstate free of any import-cycle with
// uistate: the wasm entry point wires this once at startup by closing over the
// uistate atom; native-Go tests can inject a static function. When nil (the
// default), all mutations are permitted — the permissive fallback that keeps
// existing callers compiling without modification.
//
// Example wasm wiring (in main.go / app init):
//
//	appstate.Default.SetActiveRoleFunc(func() domain.MemberRole {
//	    id := uistate.ActiveIdentityID()
//	    for _, m := range appstate.Default.Members() {
//	        if m.ID == id {
//	            return memberrole.Resolve(m)
//	        }
//	    }
//	    return domain.RoleOwner // no match → permissive
//	})
func (a *App) SetActiveRoleFunc(fn func() domain.MemberRole) {
	a.activeRoleFn = fn
}

// ActiveRole returns the effective role for the current operator, consulting
// the injected function. Returns RoleOwner (permissive) when no function has
// been wired.
func (a *App) ActiveRole() domain.MemberRole {
	if a.activeRoleFn == nil {
		return domain.RoleOwner
	}
	return a.activeRoleFn()
}

// CanEdit reports whether the current active identity may create, edit, or
// delete financial entities (accounts, transactions, budgets, goals, rules,
// etc.). Delegates to memberrole.CanEditEntities.
func (a *App) CanEdit() bool {
	return memberrole.CanEditEntities(a.ActiveRole())
}

// CanManageMembers reports whether the current active identity may add, edit,
// or remove household members. Delegates to memberrole.CanManageMembers.
func (a *App) CanManageMembers() bool {
	return memberrole.CanManageMembers(a.ActiveRole())
}

// roleGuard returns ErrReadOnly when the active identity is not permitted to
// edit financial entities. It is called at the top of every PutX / DeleteX
// that should respect the role guardrail.
func (a *App) roleGuard() error {
	if !a.CanEdit() {
		a.log.Warn("mutation blocked: active identity is read-only",
			"role", a.ActiveRole())
		return ErrReadOnly
	}
	return nil
}

// memberRoleGuard returns ErrReadOnly when the active identity may not manage
// members (only RoleOwner is permitted). Used in PutMember / DeleteMember /
// DeleteMemberAfterReassign.
func (a *App) memberRoleGuard() error {
	if !a.CanManageMembers() {
		a.log.Warn("member mutation blocked: active identity cannot manage members",
			"role", a.ActiveRole())
		return ErrReadOnly
	}
	return nil
}
