// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/store"
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

// ready returns ErrDurabilityUnavailable when the App has no local dataset behind it, so public
// methods return an error instead of dereferencing a missing App or store.
func (a *App) ready() error {
	if a == nil || a.store == nil {
		return ErrDurabilityUnavailable
	}
	return nil
}

// roleGuard returns ErrReadOnly when the active identity is not permitted to
// edit financial entities. It is called at the top of every PutX / DeleteX
// that should respect the role guardrail.
func (a *App) roleGuard() error {
	if err := a.ready(); err != nil {
		return err
	}
	if !a.CanEdit() {
		a.log.Warn("mutation blocked: active identity is read-only",
			"role", a.ActiveRole())
		return ErrReadOnly
	}
	return nil
}

// memberRoleGuard returns ErrReadOnly when the active identity may not manage
// members (only RoleOwner is permitted). Used in PutMember / DeleteMember /
// DeleteMemberAfterReassign. SetDefaultMember deliberately uses roleGuard instead: picking which
// existing member new forms default to is an editing action open to Editors, while adding,
// changing and removing members stays with Owners.
func (a *App) memberRoleGuard() error {
	if err := a.ready(); err != nil {
		return err
	}
	if !a.CanManageMembers() {
		a.log.Warn("member mutation blocked: active identity cannot manage members",
			"role", a.ActiveRole())
		return ErrReadOnly
	}
	return nil
}

// householdSettingsChanged reports whether next differs from current in any household-wide
// setting: the default account, base currency, exchange rates and their timestamps, freshness
// overrides, budgeting method, flex budget, payoff baseline and the household AI spend cap and
// spend record. The per-person fields (music resume point, AI key, model, base URL and reasoning
// effort) are blanked before the comparison, so they never count. A value that cannot be encoded counts as
// a change, which keeps the role check on the safe side.
func householdSettingsChanged(current, next store.Settings) bool {
	a, errA := json.Marshal(householdSettings(current))
	b, errB := json.Marshal(householdSettings(next))
	return errA != nil || errB != nil || !bytes.Equal(a, b)
}

// householdSettings returns s with every per-person field cleared, leaving the household-wide
// fields (including the AI spend cap and record) in place.
func householdSettings(s store.Settings) store.Settings {
	s.OpenAIKey, s.OpenAIModel, s.OpenAIReasoningEffort, s.OpenAIBaseURL = "", "", "", ""
	s.Music = nil
	return s
}
