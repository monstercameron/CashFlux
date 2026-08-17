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

// CatSuggestHost mounts the per-transaction "Categorize this" modal (SM-2) at the
// shell root. It reads the categorize atom (the target transaction id) and renders
// the flip modal when set. The body is a <form> (screens.CatSuggestFormID); the
// FlipPanel's pinned footer supplies Cancel + Save (Save = a native submit for
// that form), so the buttons stay fixed while the body scrolls. Mounting at the
// shell root keeps the fixed panel clear of the tile transforms that would clip it.
func CatSuggestHost() uic.Node {
	open := uistate.UseCatSuggest()
	if open.Get() == "" {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        uistate.T("catSuggest.title"),
		Width:        uiw.FlipSmallW,
		Height:       "min(90vh, 520px)",
		FormID:       screens.CatSuggestFormID,
		SaveLabel:    uistate.T("catSuggest.save"),
		SaveTestID:   "catsuggest-save",
		CancelTestID: "catsuggest-cancel",
		OnClose:      func() { uistate.CloseCatSuggest() },
		Back:         uic.CreateElement(screens.CatSuggestBody, struct{}{}),
	})
}
