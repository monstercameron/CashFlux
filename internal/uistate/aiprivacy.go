// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

// The assistant's per-conversation privacy tier (AG17). The active tier is a shared
// atom so the chip and the send path read the same value and re-render in step; the
// user's preferred default is persisted in the settings KV so a new conversation
// opens at their chosen level ("default rememberable").
const (
	privacyTierAtomID  = "assistant:privacyTier"
	privacyTierDefault = "cashflux:assistant-privacy-default"
)

// DefaultPrivacyTier reads the user's saved default conversation tier (AG17),
// falling back to full detail when unset.
func DefaultPrivacyTier() aicontext.ConversationTier {
	return aicontext.ParseConversationTier(SettingKVGet(privacyTierDefault))
}

// PersistDefaultPrivacyTier saves the user's preferred default conversation tier so
// future chats open at it.
func PersistDefaultPrivacyTier(t aicontext.ConversationTier) {
	SettingKVSet(privacyTierDefault, string(t))
}

// UsePrivacyTier is the shared atom for the active conversation's privacy tier,
// seeded from the saved default. Set it to switch the running conversation between
// full detail and aggregates-only.
func UsePrivacyTier() state.Atom[aicontext.ConversationTier] {
	return state.UseAtom(privacyTierAtomID, DefaultPrivacyTier())
}
