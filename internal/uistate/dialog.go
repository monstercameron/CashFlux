// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strings"

	"github.com/monstercameron/GoWebComponents/v7/state"
)

// DialogKind distinguishes a yes/no confirm from a text prompt.
type DialogKind string

const (
	DialogConfirm DialogKind = "confirm"
	DialogPrompt  DialogKind = "prompt"
)

// DialogRequest is a pending in-app modal dialog that replaces the native prompt and
// confirm. OnResult is called once with the person's choice: (true, text) on confirm,
// where text is the entry for a prompt and "" for a confirm, or (false, "") on cancel.
type DialogRequest struct {
	Kind         DialogKind
	Title        string
	Message      string
	Default      string // prompt initial value
	ConfirmLabel string // optional; defaults to OK/Confirm
	Destructive  bool   // tint the confirm button as a destructive action
	Secret       bool   // mask a prompt's entry, for a passcode
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
	ConfirmModalLabeled(message, "", destructive, onResult)
}

// ConfirmModalLabeled is ConfirmModal with a custom confirm-button label, so a
// destructive action can name itself (for example "Erase everything") instead of the
// generic "Confirm". An empty label uses the default.
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
// (or "" on cancel/empty). Use PromptModalResult when cancel must be told apart from
// an empty entry.
func PromptModal(message, def string, onResult func(string)) {
	PromptModalResult(message, def, func(ok bool, v string) {
		if onResult == nil {
			return
		}
		if ok {
			onResult(strings.TrimSpace(v))
		} else {
			onResult("")
		}
	})
}

// PromptModalResult opens an in-app text-entry dialog and reports whether the person
// confirmed it. onResult(false, "") means they cancelled, so a caller can leave its
// value alone. onResult(true, v) carries the trimmed entry, which may be empty when the
// person deliberately cleared the field.
func PromptModalResult(message, def string, onResult func(ok bool, value string)) {
	openPrompt(&DialogRequest{Kind: DialogPrompt, Message: message, Default: def}, func(ok bool, v string) {
		if ok {
			v = strings.TrimSpace(v)
		}
		onResult(ok, v)
	})
}

// PromptSecret opens an in-app dialog that asks for a passcode and masks the entry. onResult
// gets the entry as typed, with no trimming, and ok is false when the person cancelled.
func PromptSecret(message string, onResult func(ok bool, value string)) {
	openPrompt(&DialogRequest{Kind: DialogPrompt, Message: message, Secret: true}, onResult)
}

// openPrompt puts a prompt request on the shared dialog atom, if the host is mounted.
func openPrompt(req *DialogRequest, onResult func(ok bool, value string)) {
	if !dialogReady {
		return
	}
	req.OnResult = func(ok bool, v string) {
		if onResult != nil {
			onResult(ok, v)
		}
	}
	dialogAtom.Set(req)
}
