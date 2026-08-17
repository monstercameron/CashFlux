// SPDX-License-Identifier: MIT

package screenlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── C553 review-commit guards ───────────────────────────────────────────────
//
// "Categorize & next" advanced the card while the transaction stayed
// Uncategorized after a reload. Two distinct properties have to hold, and both
// are structural, so they are asserted against the source rather than through a
// browser (the review surfaces are wasm-only and their commit path is a closure
// over app state that no native test can reach).
//
//  1. The commit must not advance unless the write LANDED. applyReviewChoice
//     reports that as a bool; every caller must branch on it. A queue that moves
//     on from an item it did not save is indistinguishable from one that saved —
//     which is exactly why the defect went unnoticed.
//
//  2. The write must be flushed. The dataset autosaves on a 4-second ticker and
//     on pagehide, so a missing RequestPersist is not always fatal — but the
//     review loop is the documented case FOR it: many rapid writes that a user
//     may close or reload straight after, which is the race RequestPersist
//     exists to remove.

// reviewCommitFiles are the two surfaces that own a review commit path.
var reviewCommitFiles = []string{
	"screens/review_inbox.go",
	"screens/review_suggest.go",
	"screens/review_surface.go",
}

func parseInternalFile(t *testing.T, rel string) (*token.FileSet, *ast.File) {
	t.Helper()
	path := filepath.Join(internalRoot(t), filepath.FromSlash(rel))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %q: %v", rel, err)
	}
	return fset, f
}

// TestApplyReviewChoiceReportsWhetherTheWriteLanded pins the signature the
// callers depend on. Reverting it to a bare func() would make every call site
// below silently unconditional again.
func TestApplyReviewChoiceReportsWhetherTheWriteLanded(t *testing.T) {
	_, file := parseInternalFile(t, "screens/review_suggest.go")
	var found bool
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "applyReviewChoice" {
			continue
		}
		found = true
		res := fn.Type.Results
		if res == nil || len(res.List) != 1 {
			t.Fatalf("applyReviewChoice must return exactly one value (whether the write landed); got %v", res)
		}
		id, ok := res.List[0].Type.(*ast.Ident)
		if !ok || id.Name != "bool" {
			t.Fatalf("applyReviewChoice must return bool; got %v", res.List[0].Type)
		}
	}
	if !found {
		t.Fatal("applyReviewChoice not found in screens/review_suggest.go")
	}
}

// TestEveryReviewCommitBranchesOnTheResult is the core C553 guard: no call to
// applyReviewChoice may be a bare statement. A bare call is a call whose result
// is thrown away, which is how the card advanced past a write that failed.
func TestEveryReviewCommitBranchesOnTheResult(t *testing.T) {
	var bare []string
	for _, rel := range reviewCommitFiles {
		fset, file := parseInternalFile(t, rel)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := stmt.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "applyReviewChoice" {
				return true
			}
			bare = append(bare, rel+":"+fset.Position(call.Pos()).String())
			return true
		})
	}
	if len(bare) > 0 {
		t.Errorf("applyReviewChoice called as a bare statement (result discarded) at:\n  %s\n\n"+
			"Branch on the returned bool. Discarding it advances the review queue past a write "+
			"that did not land, so the item looks resolved, the count drops, and a reload brings "+
			"it straight back (C553).", strings.Join(bare, "\n  "))
	}
}

// TestReviewWritesRequestPersist keeps the flush in place on both write helpers.
func TestReviewWritesRequestPersist(t *testing.T) {
	path := filepath.Join(internalRoot(t), "screens", "review_inbox.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read review_inbox.go: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		t.Fatalf("parse review_inbox.go: %v", err)
	}
	// C653 collapsed the two write helpers (one keyed on a transaction id, one that
	// re-derived a merchant key and swept the ledger) into a single one that takes
	// the exact ids the surface drew. The flush requirement this test exists for is
	// unchanged; there is just one place left to enforce it.
	want := map[string]bool{
		"assignReviewToCharges": false,
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if _, tracked := want[fn.Name.Name]; !tracked || fn.Body == nil {
			continue
		}
		names := callNamesIn(fn.Body)
		want[fn.Name.Name] = names["uistate.RequestPersist"] || names["RequestPersist"]
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("%s does not call uistate.RequestPersist — the review loop writes fast and is "+
				"often closed or reloaded straight after, which is the exact race RequestPersist "+
				"removes (C553).", name)
		}
	}
}

// callNamesIn collects every called function name in a body, both bare (Foo())
// and qualified (pkg.Foo()).
func callNamesIn(body ast.Node) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			out[fn.Name] = true
		case *ast.SelectorExpr:
			out[fn.Sel.Name] = true
			if x, ok := fn.X.(*ast.Ident); ok {
				out[x.Name+"."+fn.Sel.Name] = true
			}
		}
		return true
	})
	return out
}
