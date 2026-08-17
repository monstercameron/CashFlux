// SPDX-License-Identifier: MIT

package screenlint

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sourceOf reads a file under internal/ as text, for the checks that are about
// what the source SAYS rather than its AST shape.
func sourceOf(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(internalRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %q: %v", rel, err)
	}
	return string(data)
}

// ─── C555: the selected period has to survive a reload ───────────────────────
//
// Selecting August while working in the transaction ledger held for the session
// and reverted after a reload. The window WAS persisted — from exactly two
// places, both on /reports, both on RENDER. Every control that actually CHANGES
// the period (the top-bar stepper, the quick-jump presets, the resolution
// picker, the range editor, the week-start preference) set the atom and saved
// nothing, so a reload restored whatever /reports had last happened to write.
//
// The fix is uistate.SetPeriod, which sets and persists together. This guard is
// what keeps it the only way through: the setters are spread across the shell,
// settings and two screens, and a rule every caller has to remember is a rule
// the next control will not follow. A bare Set on a period-window atom fails
// here rather than shipping as a period that silently forgets itself.
//
// It is a source guard because the controls are wasm-only click handlers over
// shared atom state, which no native test can reach.

// periodSetterFiles are the files that hold a shared period-window atom.
var periodSetterFiles = []string{
	"app/shell.go",
	"app/settings.go",
	"screens/reports_saved.go",
}

// periodAtomNames are the local identifiers those files bind the shared window
// atom to. A new file binding it under a different name should be added here
// along with the file.
var periodAtomNames = map[string]bool{"periodAtom": true, "atom": true}

// TestPeriodChangesGoThroughSetPeriod fails on a bare `<periodAtom>.Set(...)` in
// any file that owns a period control. Setting without persisting is the exact
// defect C555 reported, and it is invisible in a session — the window is correct
// until the next reload.
func TestPeriodChangesGoThroughSetPeriod(t *testing.T) {
	for _, rel := range periodSetterFiles {
		fset, f := parseInternalFile(t, rel)
		// Only guard files that actually hold the shared window atom; otherwise a
		// generic local named "atom" in an unrelated file would trip this.
		if !strings.Contains(sourceOf(t, rel), "uistate.UsePeriod()") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Set" {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || !periodAtomNames[ident.Name] {
				return true
			}
			t.Errorf("%s: %s.Set(...) sets the period without persisting it — "+
				"use uistate.SetPeriod(%s, w) so the selection survives a reload (C555)",
				fset.Position(call.Pos()), ident.Name, ident.Name)
			return true
		})
	}
}

// TestSetPeriodPersists pins the seam itself. Dropping the PersistPeriodWindow
// call inside SetPeriod would satisfy every guard above while restoring the
// original bug in one line.
func TestSetPeriodPersists(t *testing.T) {
	src := sourceOf(t, "uistate/periodpersist.go")
	i := strings.Index(src, "func SetPeriod(")
	if i < 0 {
		t.Fatal("uistate.SetPeriod is gone — the period-persistence seam C555 depends on")
	}
	body := src[i:]
	if j := strings.Index(body[1:], "\nfunc "); j >= 0 {
		body = body[:j+1]
	}
	if !strings.Contains(body, "PersistPeriodWindow(") {
		t.Error("SetPeriod no longer persists the window — a period change would " +
			"again be lost on reload with nothing on screen admitting the reset (C555)")
	}
	if !strings.Contains(body, "atom.Set(") {
		t.Error("SetPeriod no longer applies the window to the atom")
	}
}
