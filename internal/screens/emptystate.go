//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
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
	// Icon is the muted glyph shown above a first-run empty state (CTA variant).
	// Unset falls back to a neutral box glyph (C46). Ignored for the bare-message
	// variant, where a glyph would clutter transient "no match" / "all done" lines.
	Icon icon.Name
}

// EmptyStateCTA renders a friendly empty-state block: a short message plus a
// button that jumps the cursor to the add form so the user can create their
// first entry without hunting for it (§6.5). It is its own component so its
// click handler hook stays stable as the list toggles between empty and
// non-empty (mounting/unmounting a whole component is safe; reordering hooks
// inside a stable one is not).
func EmptyStateCTA(props emptyCTAProps) ui.Node {
	onClick := ui.UseEvent(Prevent(func() { focusByID(props.FocusID) }))
	if props.FocusID == "" {
		return P(ClassStr("empty"), props.Message)
	}
	// A muted glyph above the first-run message makes an otherwise-blank panel feel
	// intentional and inviting (C46).
	glyph := props.Icon
	if !glyph.Valid() {
		glyph = icon.Box
	}
	return Div(ClassStr("empty-cta"),
		uiw.Icon(glyph, ClassStr("w-8 h-8 text-faint")),
		P(ClassStr("empty"), props.Message),
		Button(ClassStr("btn btn-primary"), Type("button"), OnClick(onClick), props.CTALabel),
	)
}
