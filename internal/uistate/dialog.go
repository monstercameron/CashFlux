// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strings"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

// DialogKind distinguishes a yes/no confirm from a text prompt.
type DialogKind string

const (
	DialogConfirm DialogKind = "confirm"
	DialogPrompt  DialogKind = "prompt"
)

// DialogRequest is a pending in-app modal dialog (replacing native
// prompt/confirm). OnResult is invoked once with the user's choice: (true, text)
// on confirm — text is the entry for a prompt, "" for a confirm — or (false, "")
// on cancel.
type DialogRequest struct {
	Kind         DialogKind
	Title        string
	Message      string
	Default      string // prompt initial value
	ConfirmLabel string // optional; defaults to OK/Confirm
	Destructive  bool   // tint the confirm button as a destructive action
	OnResult     func(ok bool, value string)
}

// UseDialog returns the shared atom holding the current modal dialog request
// (nil when none is open). The DialogHost renders it; helpers set it.
func UseDialog() state.Atom[*DialogRequest] {
	return state.UseAtom("app:dialog", (*DialogRequest)(nil))
}

// dialogAtom is captured from DialogHost's render so the helpers Set the exact
// instance the host subscribed with (Set from a freshly looked-up atom doesn't
// notify the subscriber). Shared so any package (app, screens) can open a dialog.
var (
	dialogAtom  state.Atom[*DialogRequest]
	dialogReady bool
)

// CaptureDialog registers the dialog atom the host renders with. Call from the
// DialogHost component each render.
func CaptureDialog(a state.Atom[*DialogRequest]) {
	dialogAtom, dialogReady = a, true
}

// ConfirmModal opens an in-app yes/no dialog; onResult(true) when confirmed.
func ConfirmModal(message string, destructive bool, onResult func(bool)) {
	if !dialogReady {
		return
	}
	dialogAtom.Set(&DialogRequest{
		Kind: DialogConfirm, Message: message, Destructive: destructive,
		OnResult: func(ok bool, _ string) {
			if onResult != nil {
				onResult(ok)
			}
		},
	})
}

// ConfirmModalLabeled is ConfirmModal with a custom confirm-button label, so a
// destructive action can name itself (e.g. "Erase everything") instead of the
// generic "Confirm" (C298).
func ConfirmModalLabeled(message, confirmLabel string, destructive bool, onResult func(bool)) {
	if !dialogReady {
		return
	}
	dialogAtom.Set(&DialogRequest{
		Kind: DialogConfirm, Message: message, Destructive: destructive, ConfirmLabel: confirmLabel,
		OnResult: func(ok bool, _ string) {
			if onResult != nil {
				onResult(ok)
			}
		},
	})
}

// PromptModal opens an in-app text-entry dialog; onResult gets the trimmed entry
// (or "" on cancel/empty).
func PromptModal(message, def string, onResult func(string)) {
	if !dialogReady {
		return
	}
	dialogAtom.Set(&DialogRequest{
		Kind: DialogPrompt, Message: message, Default: def,
		OnResult: func(ok bool, v string) {
			if onResult == nil {
				return
			}
			if ok {
				onResult(strings.TrimSpace(v))
			} else {
				onResult("")
			}
		},
	})
}
