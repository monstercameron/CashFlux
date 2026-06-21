//go:build js && wasm

package uistate

import "syscall/js"

// chatPromptStore is the localStorage entry holding the user's custom Insights-chat
// system prompt (persona/instructions). Empty/unset means use the app default. The
// live financial context is appended by the screen, so a custom prompt never loses
// it.
const chatPromptStore = "cashflux:chat-system-prompt"

// PersistSystemPrompt saves the user's custom chat system prompt (or clears it when
// blank).
func PersistSystemPrompt(prompt string) {
	if prompt == "" {
		js.Global().Get("localStorage").Call("removeItem", chatPromptStore)
		return
	}
	js.Global().Get("localStorage").Call("setItem", chatPromptStore, prompt)
}

// LoadSystemPrompt reads the user's custom chat system prompt, or "" when none is set.
func LoadSystemPrompt() string {
	v := js.Global().Get("localStorage").Call("getItem", chatPromptStore)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
