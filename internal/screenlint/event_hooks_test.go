// SPDX-License-Identifier: MIT

package screenlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestEventOptionsDoNotRegisterHooksInsideBranches protects GWC's hook order.
func TestEventOptionsDoNotRegisterHooksInsideBranches(t *testing.T) {
	for rel, source := range readInternal(t) {
		if !strings.HasPrefix(rel, "screens/") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, rel, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if call, ok := node.(*ast.CallExpr); ok && isRawEventOption(call) && insideControlBlock(stack) {
				pos := fset.Position(call.Pos())
				t.Errorf("%s:%d: event option registers a hook inside a conditional block", filepath.ToSlash(rel), pos.Line)
			}
			stack = append(stack, node)
			return true
		})
	}
}

func isRawEventOption(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.Ident)
	if !ok {
		if sel, yes := call.Fun.(*ast.SelectorExpr); yes {
			selector, ok = sel.Sel, true
		}
	}
	if !ok || len(selector.Name) < 3 || !strings.HasPrefix(selector.Name, "On") || selector.Name[2] < 'A' || selector.Name[2] > 'Z' || len(call.Args) == 0 {
		return false
	}
	return isFuncOrPrevent(call.Args[0])
}

func isFuncOrPrevent(expr ast.Expr) bool {
	if _, ok := expr.(*ast.FuncLit); ok {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "Prevent" && len(call.Args) > 0 && isFuncOrPrevent(call.Args[0])
}

func insideControlBlock(stack []ast.Node) bool {
	for _, node := range stack {
		switch node.(type) {
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt:
			return true
		}
	}
	return false
}
