// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// explainSeedKey is the localStorage slot holding a pending "explain this figure"
// prompt (AG7). localStorage (not an in-memory atom) so the seed survives the
// navigation from a KPI surface to /assistant, and even a hard reload of the
// assistant route.
const explainSeedKey = "app:explainSeed"

// SeedExplain stores a pre-seeded assistant prompt for the chat host to consume
// on mount. The Explain affordance on a KPI/figure builds the derivation text and
// calls this, then navigates to /assistant. An empty text clears any pending seed.
func SeedExplain(text string) {
	if text == "" {
		KVDelete(explainSeedKey)
		return
	}
	KVSet(explainSeedKey, text)
}

// ConsumeExplainSeed returns any pending explain seed and clears it (one-shot), so
// a later visit to the assistant starts blank. ok is false when nothing is
// pending. The chat host calls this on mount (a coordinator one-liner) and, when
// ok, submits the text as the opening user turn.
func ConsumeExplainSeed() (string, bool) {
	v := KVGet(explainSeedKey)
	if v == "" {
		return "", false
	}
	KVDelete(explainSeedKey)
	return v, true
}
