//go:build js && wasm

package uistate

import "syscall/js"

// aiKeyStore is the localStorage entry holding the OpenAI key when the user has
// opted into remembering it on this device (prefs.RememberAIKey). It is kept
// separate from the autosaved dataset, which always redacts the key.
const aiKeyStore = "cashflux:openai-key"

// PersistAIKey writes the OpenAI key to localStorage (only call this when the
// user has opted in via the "remember on this device" toggle).
func PersistAIKey(key string) {
	js.Global().Get("localStorage").Call("setItem", aiKeyStore, key)
}

// ClearAIKey removes any persisted OpenAI key (used when the user turns the
// remember-key toggle off).
func ClearAIKey() {
	js.Global().Get("localStorage").Call("removeItem", aiKeyStore)
}

// LoadAIKey reads the persisted OpenAI key, or "" if none is stored.
func LoadAIKey() string {
	v := js.Global().Get("localStorage").Call("getItem", aiKeyStore)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
