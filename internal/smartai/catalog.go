// SPDX-License-Identifier: MIT

package smartai

import (
	"sort"
	"strings"
)

// PathSep joins a parent and child category in the qualified path the model is
// shown and answers with ("Auto > Gas"). Bare leaf names are ambiguous the
// moment two parents own the same child — "Gas" under both Auto and Utilities —
// and a name-keyed resolver silently picks whichever landed in the map last
// (C489). Every prompt and every parse goes through a path instead.
const PathSep = " > "

// Cat is one category as the model sees it: a stable id, its qualified path, and
// whether it is an income bucket. Income is what lets a sign-mismatched
// assignment be rejected rather than written (C490).
type Cat struct {
	ID     string
	Path   string
	Income bool
}

// Catalog is the resolved category list for one request. Build it once per call
// and hand it to both the prompt builder and the parser, so the names the model
// is offered and the names it is held to can never drift apart.
type Catalog struct {
	byPath map[string]Cat
	order  []Cat
}

// NewCatalog indexes cats by their lower-cased path. Entries with an empty path
// or id are dropped; the FIRST definition of a duplicate path wins, so the
// result is deterministic regardless of map iteration order.
func NewCatalog(cats []Cat) Catalog {
	c := Catalog{byPath: make(map[string]Cat, len(cats))}
	for _, e := range cats {
		p := strings.TrimSpace(e.Path)
		if p == "" || strings.TrimSpace(e.ID) == "" {
			continue
		}
		e.Path = p
		// Index through the SAME normalizer Lookup uses, or the two spellings of
		// the separator never meet and every lookup misses.
		k := normalizePath(p)
		if k == "" {
			continue
		}
		if _, dup := c.byPath[k]; dup {
			continue
		}
		c.byPath[k] = e
		c.order = append(c.order, e)
	}
	sort.SliceStable(c.order, func(i, j int) bool { return c.order[i].Path < c.order[j].Path })
	return c
}

// Len reports how many categories the catalog holds.
func (c Catalog) Len() int { return len(c.order) }

// Lookup resolves a path the model returned, case-insensitively. It also accepts
// the separator written with tighter spacing ("Auto>Gas") or with the display
// chevron ("Auto › Gas"), because models reformat punctuation freely and a
// rejected answer here costs the user a category they would have accepted.
func (c Catalog) Lookup(path string) (Cat, bool) {
	k := normalizePath(path)
	if k == "" {
		return Cat{}, false
	}
	e, ok := c.byPath[k]
	return e, ok
}

// normalizePath lower-cases and canonicalizes separator spelling/spacing.
func normalizePath(s string) string {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "\"'`"))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "›", ">")
	s = strings.ReplaceAll(s, "»", ">")
	parts := strings.Split(s, ">")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.ToLower(strings.Join(parts, strings.TrimSpace(PathSep)))
}

// Prompt renders the catalog as the block sent to the model: one qualified path
// per line with its kind, so the model can avoid handing an expense charge an
// income category in the first place (the parser still enforces it).
func (c Catalog) Prompt() string {
	var b strings.Builder
	for _, e := range c.order {
		b.WriteString(e.Path)
		if e.Income {
			b.WriteString(" | income\n")
		} else {
			b.WriteString(" | expense\n")
		}
	}
	return b.String()
}

// Confidence is how sure the model claims to be about one assignment (C488).
// Without it there is no way to rank a queue or to offer "accept all the safe
// ones", because the previous contract expressed doubt only by staying silent.
type Confidence int

const (
	// ConfLow: the model guessed. Never apply in bulk.
	ConfLow Confidence = iota
	// ConfMedium: a reasonable answer that deserves a glance. The default when
	// the model omits the marker, so an older/terser reply degrades safely.
	ConfMedium
	// ConfHigh: the model is confident; safe to offer for bulk confirmation.
	ConfHigh
)

// String returns a stable, non-localized token for logs and test names.
func (c Confidence) String() string {
	switch c {
	case ConfLow:
		return "low"
	case ConfHigh:
		return "high"
	}
	return "medium"
}

// parseConfidence maps the model's marker onto the enum, defaulting to medium
// for anything absent or unrecognized.
func parseConfidence(s string) Confidence {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "certain", "sure":
		return ConfHigh
	case "low", "unsure", "maybe", "guess":
		return ConfLow
	}
	return ConfMedium
}
