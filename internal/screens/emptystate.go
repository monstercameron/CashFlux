//go:build js && wasm

package screens

import (
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
		return P(Class("empty"), props.Message)
	}
	return Div(Class("empty-cta"),
		P(Class("empty"), props.Message),
		Button(Class("btn btn-primary"), Type("button"), OnClick(onClick), props.CTALabel),
	)
}
