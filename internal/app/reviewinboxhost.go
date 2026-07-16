// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// ReviewInboxHost mounts the transaction Review inbox (CG-S2) at the shell root.
// It reads the review-inbox atom and renders the flip modal when open. The body
// owns its own step controls (categorize / suggest / looks-good / skip / done),
// so the panel is NoFooter. Mounting at the shell root keeps the fixed panel
// clear of the tile transforms that would clip it.
func ReviewInboxHost() uic.Node {
	open := uistate.UseReviewInbox()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title: uistate.T("review.title"),
		Width: "min(92vw, 460px)",
		// Sized to the taller step content (card + picker + suggestion + batch row +
		// actions) so there's little dead space; the all-caught-up state centers.
		Height:   "min(88vh, 500px)",
		NoFooter: true,
		OnClose:  func() { uistate.CloseReviewInbox() },
		Back:     uic.CreateElement(screens.ReviewInboxBody, struct{}{}),
	})
}
