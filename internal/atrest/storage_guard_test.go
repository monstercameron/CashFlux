//go:build !js

package atrest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRawSensitiveStorageWrites prevents a new writer bypassing the encrypted
// boundary. Set and SetDurable in browserstore are themselves secure aliases;
// there is intentionally no exported raw browserstore setter to allowlist.
func TestNoRawSensitiveStorageWrites(t *testing.T) {
	root := filepath.Join("..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "browserstore" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Include JavaScript embedded in Go raw strings (the widget canvas uses
		// one). A fallback is still a write path even if normally unreachable.
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, "localStorage.setItem(") || strings.Contains(line, "sessionStorage.setItem(") {
				t.Errorf("%s:%d: web storage writer bypasses browserstore", path, i+1)
			}
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, raw, 0)
		if err != nil {
			return err
		}
		// Detect syscall/js storage aliases in each function. Both local and
		// session storage writers must route through the shared boundary.
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			aliases := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, rhs := range assign.Rhs {
					call, ok := rhs.(*ast.CallExpr)
					if !ok || len(call.Args) == 0 || i >= len(assign.Lhs) {
						continue
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Get" {
						continue
					}
					arg, ok := call.Args[0].(*ast.BasicLit)
					if !ok || (arg.Value != `"localStorage"` && arg.Value != `"sessionStorage"`) {
						continue
					}
					if name, ok := assign.Lhs[i].(*ast.Ident); ok {
						aliases[name.Name] = true
					}
				}
				return true
			})
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Call" {
					return true
				}
				arg, ok := call.Args[0].(*ast.BasicLit)
				if !ok || arg.Value != `"setItem"` {
					return true
				}
				if name, ok := sel.X.(*ast.Ident); ok && aliases[name.Name] {
					// Whole-device erasure coordinates tabs through a nonpersonal
					// localStorage marker before deleting browserstore itself.
					if filepath.Base(path) == "settings.go" && fn.Name.Name == "markDeviceErasePending" && len(call.Args) == 3 {
						key, keyOK := call.Args[1].(*ast.BasicLit)
						value, valueOK := call.Args[2].(*ast.BasicLit)
						if keyOK && valueOK && key.Value == `"cashflux:erase-in-progress"` && value.Value == `"pending"` {
							return true
						}
					}
					t.Errorf("%s: web storage writer bypasses browserstore", fset.Position(call.Pos()))
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(filepath.Join("..", "..", "web"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".html") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, "localStorage.setItem(") || strings.Contains(line, "sessionStorage.setItem(") {
				// This standalone page cannot call browserstore: it runs after
				// Wasm exits. These exact writes carry no household data.
				if filepath.Base(path) == "erase-device.html" {
					trimmed := strings.TrimSpace(line)
					if trimmed == `localStorage.setItem(api.marker, ticket);` ||
						trimmed == `localStorage.setItem(api.erasedFlag, "1");` {
						continue
					}
				}
				t.Errorf("%s:%d: web storage writer bypasses browserstore", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
