// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// copyToClipboard writes text to the system clipboard via the async Clipboard API and
// posts a confirmation toast (or an error toast if the clipboard is unavailable). Used
// by copyable read-only values (e.g. the budget formulas modal).
func copyToClipboard(text, okToast string) {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		uistate.PostNotice(uistate.T("common.copyFail"), true)
		return
	}
	clip := nav.Get("clipboard")
	if !clip.Truthy() {
		uistate.PostNotice(uistate.T("common.copyFail"), true)
		return
	}
	clip.Call("writeText", text)
	if okToast == "" {
		okToast = uistate.T("common.copied")
	}
	uistate.PostNotice(okToast, false)
}
