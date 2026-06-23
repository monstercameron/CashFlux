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

// Raw-markup scaffolds the C73 component-ization epic is migrating away from.
// Each screen should reach for a primitive (Card / EntityListSection / DataTable)
// instead of hand-rolling these. See docs/COMPONENTS.md for the porting guide.
const (
	// cardScaffold is the bespoke card container `Section(css.Class("card"...`.
	cardScaffold = `Section(css.Class("card`
	// rowsScaffold is the bespoke list container `Div(css.Class("rows"...`.
	rowsScaffold = `Div(css.Class("rows`

	// scaffoldBaseline is the TOTAL count of the two raw scaffolds across
	// internal/screens. Introduced at 165 (117 card + 48 rows) on 2026-06-23 and
	// ratcheted down as screens migrate to EntityListSection/Card/DataTable. This
	// is a ONE-WAY ratchet —
	// contributors may only ever LOWER it as screens are ported to primitives.
	// If this constant needs to go UP, you are adding bespoke markup the epic is
	// trying to delete: use a primitive instead. Lower it whenever you migrate.
	scaffoldBaseline = 146
)

// countScaffolds returns the combined occurrences of the two raw scaffolds in a
// single source file's text.
func countScaffolds(src string) int {
	return strings.Count(src, cardScaffold) + strings.Count(src, rowsScaffold)
}

// TestScaffoldBaseline asserts the number of raw card/rows scaffolds in the
// screens package never EXCEEDS the recorded baseline — preventing new bespoke
// markup while allowing the existing offenders to be migrated down over time.
func TestScaffoldBaseline(t *testing.T) {
	// Resolve internal/screens relative to this test file (../screens), so the
	// check is path-independent of the working directory.
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
		total += countScaffolds(string(b))
	}

	if total > scaffoldBaseline {
		t.Fatalf("raw card/rows scaffolds in internal/screens rose to %d, above the baseline of %d — "+
			"use a primitive (Card/EntityListSection/DataTable, see docs/COMPONENTS.md) instead of hand-rolling markup",
			total, scaffoldBaseline)
	}
	if total < scaffoldBaseline {
		t.Logf("raw scaffolds down to %d (baseline %d) — lower the scaffoldBaseline constant to lock in the win",
			total, scaffoldBaseline)
	}
}
