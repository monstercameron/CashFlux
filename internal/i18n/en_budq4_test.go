// SPDX-License-Identifier: MIT

package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestBudgetTopUpActionHasOneCatalogDefinition(t *testing.T) {
	files, err := filepath.Glob("en_*.go")
	if err != nil {
		t.Fatal(err)
	}
	definitions := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, expr := range value.Values {
					lit, ok := expr.(*ast.CompositeLit)
					if !ok {
						continue
					}
					for _, elt := range lit.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, ok := kv.Key.(*ast.BasicLit)
						if ok && strings.Trim(key.Value, "\"") == "budgets.topupBtn" {
							definitions++
						}
					}
				}
			}
		}
	}
	if definitions != 1 {
		t.Fatalf("budgets.topupBtn catalog definitions = %d, want exactly one", definitions)
	}
	if got := DefaultBundle().T(English, "budgets.topupBtn"); got != "Raise limit…" {
		t.Fatalf("budgets.topupBtn = %q", got)
	}
}
