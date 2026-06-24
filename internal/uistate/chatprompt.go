// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// chatPromptStore holds the user's custom Insights-chat system prompt
// (persona/instructions) in the SQLite settings KV. Empty/unset means use the app
// default. The live financial context is appended by the screen, so a custom prompt
// never loses it.
const chatPromptStore = "cashflux:chat-system-prompt"

// PersistSystemPrompt saves the user's custom chat system prompt (or clears it when
// blank).
func PersistSystemPrompt(prompt string) {
	if prompt == "" {
		SettingKVDelete(chatPromptStore)
		return
	}
	SettingKVSet(chatPromptStore, prompt)
}

// LoadSystemPrompt reads the user's custom chat system prompt, or "" when none is set.
func LoadSystemPrompt() string {
	return SettingKVGet(chatPromptStore)
}
