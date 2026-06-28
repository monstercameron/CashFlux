// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app contains the application shell: Chrome, routing, and global
// UI wiring. This file provides the MemberSwitcher top-bar control (L21).
package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// MemberSwitcher is the top-bar member-perspective picker (L21). When there
// are two or more household members it renders a <select> that lets the user
// scope the app's figures to one member or back to "Everyone" (the default
// full-household view). The selection is persisted to localStorage via the
// shared ActiveMember atom so the chosen view survives reloads.
//
// Its own component ensures the OnChange hook registers at a stable render
// position (never inside a variable-length list) per the framework rules.
func MemberSwitcher() uic.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	members := app.Members()
	// Only shown when the household has more than one member.
	if len(members) < 2 {
		return Fragment()
	}

	// MIA-extend (#445-8): read the active scope so the switcher preserves all
	// non-owner dimensions (Institutions, Types, AccountIDs) when changing members.
	scopeAtom := uistate.UseActiveScope()
	cur := scopeAtom.Get()
	current := ""
	if len(cur.Owners) == 1 {
		current = cur.Owners[0]
	}

	onChange := OnChange(func(v string) {
		var owners []string
		if v != "" {
			owners = []string{v}
		}
		uistate.SetActiveScope(scope.ReportScope{
			Institutions: cur.Institutions,
			Owners:       owners,
			Types:        cur.Types,
			AccountIDs:   cur.AccountIDs,
		})
	})

	// Build <option> elements: "Everyone" first, then each member in store
	// order (which matches the household's defined member sequence).
	opts := []any{
		Option(Value(""), SelectedIf(current == ""), uistate.T("member.everyone")),
	}
	for _, m := range members {
		m := m
		role := memberrole.Label(memberrole.Resolve(m))
		opts = append(opts, Option(
			Value(m.ID),
			SelectedIf(current == m.ID),
			m.Name+" · "+role,
		))
	}

	args := []any{
		css.Class("member-switcher", tw.Text13, tw.TextDim),
		Attr("aria-label", uistate.T("member.switcherLabel")),
		Attr("data-testid", "member-switcher"),
		Attr("title", uistate.T("member.switcherLabel")),
		onChange,
	}
	args = append(args, opts...)
	return Select(args...)
}
