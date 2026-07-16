// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app contains the application shell: Chrome, routing, and global
// UI wiring. This file provides the MemberSwitcher top-bar control (L21).
package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
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
	// C274: "Switch profile…" button opens the "Who's using CashFlux?" modal.
	// UseEvent registered here (at a stable position, not inside a loop) so
	// the hook depth is always constant regardless of the member count.
	openSwitch := uic.UseEvent(func() { openProfileSwitch() })
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

	// The <select> is a VIEW LENS (scope the figures to one member's perspective),
	// not an identity switch — label it "View as" so it no longer reads as a second
	// "profile" control competing with the device-profile button beside it (task:
	// consolidate the two look-alike header controls).
	args := []any{
		css.Class("member-switcher", tw.Text13, tw.TextDim),
		Attr("aria-label", uistate.T("member.viewAsLabel")),
		Attr("data-testid", "member-switcher"),
		Attr("title", uistate.T("member.viewAsLabel")),
		onChange,
	}
	args = append(args, opts...)
	// C274: the "Switch profile…" (device-user) affordance is now an ICON-ONLY button
	// so it stops presenting as a second text dropdown about "profiles". It's a
	// distinct identity action (who's using this device), visually subordinate to the
	// view-scope select, with its accessible name preserved on the icon button.
	return Span(css.Class("cf-member-switcher-wrap"),
		Span(css.Class("cf-viewas-label", tw.Text13, tw.TextFaint), uistate.T("member.viewAsPrefix")),
		Select(args...),
		Button(css.Class("icon-btn", tw.W7, tw.H7, tw.TextDim, tw.HoverTextFg),
			Type("button"),
			Attr("aria-label", uistate.T("profileSwitch.switchBtn")),
			Attr("data-testid", "profile-switch-btn"),
			Title(uistate.T("profileSwitch.switchBtn")),
			OnClick(openSwitch),
			ui.Icon(icon.Users, css.Class(tw.W5, tw.H5)),
		),
	)
}
