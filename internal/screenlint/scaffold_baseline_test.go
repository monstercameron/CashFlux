// SPDX-License-Identifier: MIT

// Package screenlint holds native-runnable guard checks over the wasm-only
// internal/screens package source. The screens package is built with
// GOOS=js GOARCH=wasm (it imports syscall/js via the framework), so a normal
// _test.go living inside it cannot run under `go test ./...` on the host. This
// package carries no build constraint and inspects the screen sources as text,
// so the ratchet runs in the standard native test lane.
package screenlint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Markup the C73 component-ization epic governs. Screens reach for a primitive
// (Card / EntityListSection / DataTable) instead of hand-rolling these. See
// docs/COMPONENTS.md for the porting guide.
const (
	// cardScaffold is the bespoke card container `Section(css.Class("card"...`.
	// As of 2026-06-23 every screen card renders through the Card/EntityListSection
	// primitive, so this MUST stay at ZERO — a new occurrence means someone
	// hand-rolled a card instead of using the primitive.
	cardScaffold = `Section(css.Class("card`

	// rowsScaffold is the list-row container `Div(css.Class("rows"...`. This is the
	// exact markup EntityListSection.Rows itself emits; the remaining occurrences are
	// list bodies INSIDE ported primitive cards (a description + list, or an
	// empty-or-list branch) — not bespoke card scaffolds. It is a one-way ratchet:
	// the count may only fall as those bodies adopt the Rows slot. Introduced at 48,
	// now 38.
	rowsScaffold = `Div(css.Class("rows`

	// rowsBaseline caps the list-container count (one-way; only lower it).
	// Bumped 39→40: the dormant-WIP integration (617ccb86) added one legitimate
	// new list container in planning.go (detected recurring rows), warranting a raw
	// Div(.rows) body inside an EntityListSection rather than a nested primitive.
	rowsBaseline = 40
)

// countMatches returns the occurrences of substr across every non-test .go file
// in internal/screens.
func countMatches(t *testing.T, substr string) int {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	screensDir := filepath.Join(wd, "..", "screens")
	entries, err := os.ReadDir(screensDir)
	if err != nil {
		t.Fatalf("read screens dir %q: %v", screensDir, err)
	}
	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(screensDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		total += strings.Count(string(b), substr)
	}
	return total
}

// TestNoBespokeCardScaffold is the hard invariant: every card in internal/screens
// renders through the Card/EntityListSection primitive, so there must be ZERO
// hand-rolled `Section(css.Class("card"...` scaffolds. A failure means a new card
// was hand-rolled — use the primitive (see docs/COMPONENTS.md) instead.
func TestNoBespokeCardScaffold(t *testing.T) {
	if n := countMatches(t, cardScaffold); n != 0 {
		t.Fatalf("found %d bespoke Section(css.Class(\"card\")) scaffold(s) in internal/screens — "+
			"every card must render through uiw.Card / uiw.EntityListSection (docs/COMPONENTS.md)", n)
	}
}

// TestRowsContainerRatchet caps the list-row containers (Div(.rows)) that live
// inside ported primitive cards. One-way: the count may fall as those bodies adopt
// the EntityListSection.Rows slot, never rise.
func TestRowsContainerRatchet(t *testing.T) {
	n := countMatches(t, rowsScaffold)
	if n > rowsBaseline {
		t.Fatalf("Div(css.Class(\"rows\")) list containers rose to %d, above the baseline of %d — "+
			"use EntityListSection.Rows for new lists instead of a hand-rolled Div(.rows)", n, rowsBaseline)
	}
	if n < rowsBaseline {
		t.Logf("list containers down to %d (baseline %d) — lower rowsBaseline to lock in the win", n, rowsBaseline)
	}
}
