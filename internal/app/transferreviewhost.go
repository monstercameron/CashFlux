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

// TransferReviewHost mounts the "Review transfers" panel as a double-wide flip
// modal at the shell root (C676) — the same container the duplicates panel uses.
//
// Both surfaces answer the same shape of question: here is a pile of rows the app
// believes are wrong, resolve them in place. Giving them different containers
// would make one look like a different KIND of problem than the other.
//
// It keeps its own Cancel/Apply footer rather than the panel's, because unlike
// duplicates nothing here is written until the whole batch is confirmed — a
// CloseOnly footer would leave the primary action buried in the body.
func TransferReviewHost() uic.Node {
	open := uistate.UseTransferReviewOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("txnreview.reviewBtn"),
		Width:     "900px",
		Height:    "660px",
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.TransferReviewBody, struct{}{}),
	})
}
