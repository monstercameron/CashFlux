package styles

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRegisterCallsEveryPackageRuleRegistrar(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source file")
	}
	dir := filepath.Dir(currentFile)
	fset := token.NewFileSet()

	registrars := make(map[string]struct{})
	calls := make(map[string]map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read styles package directory: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "register") {
				registrars[fn.Name.Name] = struct{}{}
			}
			if fn.Body != nil {
				calls[fn.Name.Name] = make(map[string]struct{})
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					ident, ok := call.Fun.(*ast.Ident)
					if ok {
						calls[fn.Name.Name][ident.Name] = struct{}{}
					}
					return true
				})
			}
		}
	}

	reachable := make(map[string]struct{})
	var visit func(string)
	visit = func(name string) {
		if _, seen := reachable[name]; seen {
			return
		}
		reachable[name] = struct{}{}
		for called := range calls[name] {
			visit(called)
		}
	}
	visit("Register")

	var missing []string
	for name := range registrars {
		if _, ok := reachable[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		t.Errorf("Register does not call rule registrars: %s", strings.Join(missing, ", "))
	}
}
