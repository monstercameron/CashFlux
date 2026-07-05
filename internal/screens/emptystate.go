// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// emptyCTAProps configures an EmptyStateCTA block.
type emptyCTAProps struct {
	// Message is the friendly line explaining the list is empty.
	Message string
	// CTALabel is the button text (e.g. "Add your first goal").
	CTALabel string
	// FocusID is the id of the add form's first field; clicking the button moves
	// the cursor there (see focusByID). Empty FocusID renders just the message.
	FocusID string
	// AddTarget, when set and FocusID is empty, makes the CTA open the matching
	// entity's add modal via uistate.SetAddTarget (e.g. "goal", "account",
	// "budget"). Takes precedence over Href when both are set.
	AddTarget string
	// Href, when set, makes the CTA navigate to this logical route instead of
	// focusing a field or opening a modal — for derived screens (Bills,
	// Subscriptions, Reports) whose data is created elsewhere. Takes effect only
	// when FocusID and AddTarget are both empty.
	Href string
	// Icon is the muted glyph shown above a first-run empty state (CTA variant).
	// Unset falls back to a neutral box glyph (C46). Ignored for the bare-message
	// variant, where a glyph would clutter transient "no match" / "all done" lines.
	Icon icon.Name
	// ImportLink, when true, adds a secondary "Import from a file" link below the
	// primary CTA that navigates to /documents (C14) — so a user staring at an
	// empty accounts/transactions list discovers the bulk-import path instead of
	// only the one-at-a-time add flow.
	ImportLink bool
}

// EmptyStateCTA renders a friendly empty-state block: a short message plus a
// button that opens the entity's add modal (AddTarget), jumps the cursor to an
// inline add form (FocusID), or navigates to a route (Href). It is its own
// component so its click handler hooks stay stable as the list toggles between
// empty and non-empty (mounting/unmounting a whole component is safe; reordering
// hooks inside a stable one is not).
func EmptyStateCTA(props emptyCTAProps) ui.Node {
	nav := router.UseNavigate()
	onClick := ui.UseEvent(Prevent(func() { focusByID(props.FocusID) }))
	onNav := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath(props.Href)) }))
	onAddTarget := ui.UseEvent(Prevent(func() { uistate.SetAddTarget(props.AddTarget) }))
	// C14: secondary "Import from a file" navigation, used when ImportLink is set.
	// Registered unconditionally so the hook order is stable regardless of props.
	onImport := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/documents")) }))
	importLink := func() ui.Node {
		if !props.ImportLink {
			return Fragment()
		}
		return Button(css.Class("btn-link", tw.Text12, tw.Mt1), Type("button"),
			Attr("data-testid", "empty-import-link"), OnClick(onImport),
			uistate.T("common.importFromFile"))
	}

	if props.FocusID == "" {
		glyph := props.Icon
		if !glyph.Valid() {
			glyph = icon.Box
		}
		// AddTarget: open the entity's add modal.
		if props.AddTarget != "" {
			return Div(css.Class("empty-cta"),
				uiw.Icon(glyph, css.Class(tw.W8, tw.H8, "empty-icon")),
				P(css.Class("empty"), props.Message),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(onAddTarget), props.CTALabel),
				importLink(),
			)
		}
		// Href: navigate to where the data is created.
		if props.Href != "" {
			return Div(css.Class("empty-cta"),
				uiw.Icon(glyph, css.Class(tw.W8, tw.H8, "empty-icon")),
				P(css.Class("empty"), props.Message),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(onNav), props.CTALabel),
			)
		}
		// No action: plain transient line ("no match" / "all done").
		return P(css.Class("empty"), props.Message)
	}
	// FocusID: jump cursor to the inline add form's first field.
	glyph := props.Icon
	if !glyph.Valid() {
		glyph = icon.Box
	}
	return Div(css.Class("empty-cta"),
		uiw.Icon(glyph, css.Class(tw.W8, tw.H8, tw.TextFaint)),
		P(css.Class("empty"), props.Message),
		Button(css.Class("btn btn-primary"), Type("button"), OnClick(onClick), props.CTALabel),
		importLink(),
	)
}
