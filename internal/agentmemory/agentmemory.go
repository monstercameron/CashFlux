// SPDX-License-Identifier: MIT

// Package agentmemory is the pure model of the assistant's transparent, durable
// memory (AG19): the short list of facts the user has explicitly asked the agent
// to remember ("paid biweekly", "don't suggest cutting eating out"). It holds no
// storage and no platform code — the wasm/state layer marshals a Store to and from
// the dataset's settings KV, and the prompt layer renders it into the system
// context. Kept pure so add/edit/delete, de-duplication, capping, and rendering
// all unit-test on native Go.
//
// The design rule (from the ticket): memory is inspectable and edited by the user,
// captured only on an explicit "remember this" — never silently. This package
// enforces the mechanical invariants (trim, dedupe, cap); the "explicit" part is a
// UI/tool concern the caller owns.
package agentmemory

import (
	"encoding/json"
	"strings"
)

// MaxFacts caps how many facts are retained so the memory can never bloat the
// system prompt without bound. Adding past the cap drops the oldest fact.
const MaxFacts = 50

// maxFactLen bounds a single fact so one runaway entry can't dominate the prompt.
const maxFactLen = 280

// Store is the ordered list of remembered facts, oldest first. The zero value is a
// valid empty store.
type Store struct {
	Facts []string `json:"facts,omitempty"`
}

// Load parses a Store from its JSON encoding. An empty or malformed string yields
// an empty store (memory fails safe to "nothing remembered" rather than erroring).
func Load(raw string) Store {
	var s Store
	if strings.TrimSpace(raw) == "" {
		return s
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Store{}
	}
	s.Facts = normalize(s.Facts)
	return s
}

// Marshal encodes the Store to JSON for persistence. An empty store encodes to a
// compact object so the round-trip is lossless.
func (s Store) Marshal() string {
	b, err := json.Marshal(Store{Facts: normalize(s.Facts)})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Add appends a fact, trimming whitespace and clamping length, ignoring a blank
// entry and skipping a case-insensitive duplicate of an existing fact. When the
// store is at MaxFacts the oldest fact is dropped to make room. It returns the new
// Store (value semantics; the receiver is unchanged) and whether a fact was added.
func (s Store) Add(fact string) (Store, bool) {
	f := clamp(strings.TrimSpace(fact))
	if f == "" {
		return s, false
	}
	for _, ex := range s.Facts {
		if strings.EqualFold(ex, f) {
			return s, false
		}
	}
	next := append(append([]string{}, s.Facts...), f)
	if len(next) > MaxFacts {
		next = next[len(next)-MaxFacts:]
	}
	return Store{Facts: next}, true
}

// Edit replaces the fact at index i with a new value (trimmed/clamped). An
// out-of-range index or a blank new value leaves the store unchanged. Editing to a
// value that duplicates another fact removes the duplicate by keeping the edit.
func (s Store) Edit(i int, fact string) Store {
	if i < 0 || i >= len(s.Facts) {
		return s
	}
	f := clamp(strings.TrimSpace(fact))
	if f == "" {
		return s
	}
	next := append([]string{}, s.Facts...)
	next[i] = f
	return Store{Facts: normalize(next)}
}

// Delete removes the fact at index i. An out-of-range index leaves the store
// unchanged.
func (s Store) Delete(i int) Store {
	if i < 0 || i >= len(s.Facts) {
		return s
	}
	next := append(append([]string{}, s.Facts[:i]...), s.Facts[i+1:]...)
	return Store{Facts: next}
}

// Len reports how many facts are remembered.
func (s Store) Len() int { return len(s.Facts) }

// Prompt renders the memory as a compact Markdown block for the system prompt, or
// "" when nothing is remembered so an empty memory adds nothing to the context.
// The heading names the facts as user-provided and durable so the model treats
// them as standing instructions.
func (s Store) Prompt() string {
	facts := normalize(s.Facts)
	if len(facts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Remembered about the user\n")
	b.WriteString("Durable facts the user asked you to remember. Honor them; do not contradict them.\n")
	for _, f := range facts {
		b.WriteString("- " + f + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// clamp bounds a single fact to maxFactLen runes.
func clamp(s string) string {
	if r := []rune(s); len(r) > maxFactLen {
		return strings.TrimSpace(string(r[:maxFactLen]))
	}
	return s
}

// normalize trims, drops blanks, and removes case-insensitive duplicates (keeping
// the first occurrence), preserving order.
func normalize(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, f := range in {
		f = clamp(strings.TrimSpace(f))
		if f == "" {
			continue
		}
		key := strings.ToLower(f)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
