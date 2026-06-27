// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

const imageDraftAtomID = "doc:imageDraft"

// UseImageDraft returns the shared atom holding the data-URL of the last image
// the user picked in the Documents screen. It survives navigation so that if
// the user is sent to Settings to add an OpenAI key and then navigates back,
// the chosen image is still available and shown ready to process (C98).
//
// The atom holds a plain string (the base64 data: URL). It is cleared when the
// image is successfully imported so it does not persist stale data across
// unrelated sessions.
func UseImageDraft() state.Atom[string] {
	return state.UseAtom(imageDraftAtomID, "")
}
