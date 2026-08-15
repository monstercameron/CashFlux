// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
		// C500: 90% of the viewport. A 460px column cannot host the bulk list, and
		// both modes share ONE size so switching mode never resizes the container.
		Width:  uiw.FlipFullW,
		Height: uiw.FlipFullH,
		// FlushBody lets the body pin its own action bar (.modal-foot) instead of
		// letting it scroll off the bottom of a tall panel.
		FlushBody: true,
		NoFooter:  true,
		OnClose:   func() { uistate.CloseReviewInbox() },
		Back:      uic.CreateElement(screens.ReviewSurfaceBody, struct{}{}),
	})
}
