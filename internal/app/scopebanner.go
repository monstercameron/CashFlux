// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ScopeBanner renders a persistent chip below the top bar announcing which
// household member's perspective the app is currently scoped to (C281). When
// no member is active (the atom holds "") it renders nothing. The "View all"
// action resets the atom to "" (everyone / full-household view) and clears the
// persisted choice.
//
// ScopeBanner is its own component so its click hook occupies a stable
// position in the hook chain regardless of whether the banner is visible,
// satisfying the framework's On*-hooks-must-not-be-in-conditionals rule.
func ScopeBanner() uic.Node {
	activeMember := uistate.UseActiveMember()
	activeID := activeMember.Get()

	clearScope := uic.UseEvent(func() {
		uistate.SetActiveMember("")
	})

	if activeID == "" {
		return Fragment()
	}

	// Resolve the active member's display name from the live store. Fall back
	// to the raw ID if the member can't be found (defensive; should not happen
	// in normal operation).
	name := activeID
	if app := appstate.Default; app != nil {
		for _, m := range app.Members() {
			if m.ID == activeID {
				name = m.Name
				break
			}
		}
	}

	label := uistate.T("scope.viewingAs", name)
	viewAllTitle := uistate.T("scope.viewAllTitle")

	return Div(
		css.Class("scope-banner"),
		Attr("role", "status"),
		Attr("aria-label", uistate.T("scope.bannerLabel")),
		Attr("data-testid", "scope-banner"),
		Span(css.Class("scope-banner-text"), label),
		Button(
			css.Class("scope-banner-btn"),
			Type("button"),
			Attr("title", viewAllTitle),
			Attr("aria-label", viewAllTitle),
			Attr("data-testid", "scope-banner-clear"),
			OnClick(clearScope),
			uistate.T("scope.viewAll"),
		),
	)
}
