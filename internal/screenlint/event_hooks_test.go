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

type hookFinding struct {
	file     string
	function string
	line     int
	why      string
}

type hookFunction struct {
	file string
	pkg  string
	body *ast.BlockStmt
	fset *token.FileSet
}

// TestHooksKeepStableOrder scans screen and reusable UI code. Hooks may not be
// registered conditionally, after an early return, or through a helper called
// conditionally. Event props are included because GWC converts func values to
// UseEvent hooks while rendering.
func TestHooksKeepStableOrder(t *testing.T) {
	files := readInternal(t)
	funcs := make(map[string]hookFunction)
	parsed := make(map[string]*ast.File)
	fsets := make(map[string]*token.FileSet)
	for rel, source := range files {
		if !isHookLintScope(rel) || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, rel, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		parsed[rel], fsets[rel] = file, fset
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				funcs[file.Name.Name+"."+fn.Name.Name] = hookFunction{file: rel, pkg: file.Name.Name, body: fn.Body, fset: fset}
			}
		}
		// Function-valued local helpers (for example `dateField := func(...)`) are
		// part of the same component body and participate in the package call graph.
		ast.Inspect(file, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range assign.Rhs {
				lit, ok := rhs.(*ast.FuncLit)
				if !ok || i >= len(assign.Lhs) {
					continue
				}
				id, ok := assign.Lhs[i].(*ast.Ident)
				if ok {
					funcs[file.Name.Name+"."+id.Name] = hookFunction{file: rel, pkg: file.Name.Name, body: lit.Body, fset: fset}
				}
			}
			return true
		})
	}

	hasHooks := hookFunctionClosure(funcs)
	var findings []hookFinding
	for rel, file := range parsed {
		if strings.HasPrefix(rel, "app/") && rel != "app/toast.go" && rel != "app/save_failure_banner.go" {
			continue
		}
		fset := fsets[rel]
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			findings = append(findings, inspectHookBody(rel, file.Name.Name, fn.Name.Name, fn.Body, fset, hasHooks)...)
		}
		// Analyze assigned function literals as independent function bodies. They
		// are skipped by the outer function walk to avoid treating their returns
		// as returns from the enclosing component.
		ast.Inspect(file, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range assign.Rhs {
				lit, ok := rhs.(*ast.FuncLit)
				if !ok || i >= len(assign.Lhs) {
					continue
				}
				id, ok := assign.Lhs[i].(*ast.Ident)
				if !ok || !isCalledLocalHelper(file, id.Name) {
					continue
				}
				name := id.Name
				findings = append(findings, inspectHookBody(rel, file.Name.Name, name, lit.Body, fset, hasHooks)...)
			}
			return true
		})
	}

	baseline, err := os.ReadFile("testdata/hook_baseline.txt")
	if err != nil {
		t.Fatal(err)
	}
	baselineKeys := make(map[string]bool)
	for _, line := range strings.Split(string(baseline), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			baselineKeys[line] = true
		}
	}
	current := make(map[string]bool)
	for _, finding := range findings {
		key := filepath.ToSlash(finding.file) + ":" + finding.function
		current[key] = true
		if !baselineKeys[key] {
			t.Errorf("new hook-order finding %s:%d: %s", key, finding.line, finding.why)
		}
	}
	t.Logf("hook order inventory: %d call sites across %d functions; baseline has %d functions", len(findings), len(current), len(baselineKeys))
	for key := range baselineKeys {
		if !current[key] {
			t.Errorf("stale hook baseline entry %s; remove it", key)
		}
	}
}

func isCalledLocalHelper(file *ast.File, name string) bool {
	called := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if ok && id.Name == name {
			called = true
			return false
		}
		return !called
	})
	return called
}

func TestHookAnalysisFindsConditionalValuesHelpersAndEarlyReturns(t *testing.T) {
	const source = `package sample
func component(ready bool, callback func()) {
	if ready { ui.UseState(0) }
	if ready { OnClick(callback) }
	if !ready { return }
	ui.UseMemo(func() int { return 1 })
}
func hookHelper() { uistate.UsePrefs() }
func callsHelper(ready bool) { if ready { hookHelper() } }
func stable(callback func()) { OnClick(callback); ui.UseState(0) }
func constantFalse() { if false { return }; ui.UseState(0) }
func constantTrue() { if true { return }; ui.UseState(0) }
func loop(items []int) { for range items { ui.UseState(0) } }
type props struct { OnPage func(int) }
func callbackMethod(p props, page int) { p.OnPage(page) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	funcs := make(map[string]hookFunction)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			funcs[file.Name.Name+"."+fn.Name.Name] = hookFunction{file: "screens/fixture.go", pkg: file.Name.Name, body: fn.Body, fset: fset}
		}
	}
	hasHooks := hookFunctionClosure(funcs)
	var findings []hookFinding
	for name, fn := range funcs {
		findings = append(findings, inspectHookBody(fn.file, fn.pkg, strings.TrimPrefix(name, file.Name.Name+"."), fn.body, fn.fset, hasHooks)...)
	}
	got := make(map[string]bool)
	for _, finding := range findings {
		got[finding.function+":"+finding.why] = true
	}
	for _, want := range []string{"component:hook is registered inside a branch or loop", "component:hook is registered after an early return", "callsHelper:hook-calling helper hookHelper is called conditionally", "loop:hook is registered inside a branch or loop"} {
		if !got[want] {
			t.Errorf("missing finding %q; got %#v", want, findings)
		}
	}
	for _, safe := range []string{"stable", "constantFalse", "constantTrue", "callbackMethod"} {
		for _, finding := range findings {
			if finding.function == safe {
				t.Errorf("unexpected finding in %s: %#v", safe, finding)
			}
		}
	}
}

func isHookLintScope(rel string) bool {
	return strings.HasPrefix(rel, "screens/") || strings.HasPrefix(rel, "ui/") || strings.HasPrefix(rel, "app/")
}

func hookFunctionClosure(funcs map[string]hookFunction) map[string]bool {
	has := make(map[string]bool, len(funcs))
	for name, fn := range funcs {
		has[name] = bodyHasDirectHook(fn.body)
	}
	for changed := true; changed; {
		changed = false
		for name, fn := range funcs {
			if has[name] {
				continue
			}
			ast.Inspect(fn.body, func(node ast.Node) bool {
				if _, ok := node.(*ast.FuncLit); ok {
					return false
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if ok && has[fn.pkg+"."+id.Name] {
					has[name], changed = true, true
					return false
				}
				return true
			})
		}
	}
	return has
}

func bodyHasDirectHook(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if call, ok := node.(*ast.CallExpr); ok && isHookCall(call) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func inspectHookBody(rel, pkg, function string, body *ast.BlockStmt, fset *token.FileSet, hasHooks map[string]bool) []hookFinding {
	var findings []hookFinding
	var walk func(ast.Node, bool, bool)
	walk = func(node ast.Node, conditional, afterReturn bool) {
		if node == nil {
			return
		}
		switch n := node.(type) {
		case *ast.FuncLit:
			return // nested callbacks are separate render functions
		case *ast.BlockStmt:
			returned := afterReturn
			for _, stmt := range n.List {
				walk(stmt, conditional, returned)
				mayReturn, mustReturn := returnEffect(stmt)
				if mustReturn {
					return
				}
				if mayReturn {
					returned = true
				}
			}
			return
		case *ast.IfStmt:
			walk(n.Init, conditional, afterReturn)
			walk(n.Cond, conditional, afterReturn)
			if value, known := constantBool(n.Cond); known {
				if value {
					walk(n.Body, conditional, afterReturn)
				} else {
					walk(n.Else, conditional, afterReturn)
				}
				return
			}
			walk(n.Body, true, afterReturn)
			walk(n.Else, true, afterReturn)
			return
		case *ast.SwitchStmt:
			walk(n.Init, conditional, afterReturn)
			walk(n.Tag, conditional, afterReturn)
			for _, stmt := range n.Body.List {
				walk(stmt, true, afterReturn)
			}
			return
		case *ast.TypeSwitchStmt:
			walk(n.Init, conditional, afterReturn)
			walk(n.Assign, conditional, afterReturn)
			for _, stmt := range n.Body.List {
				walk(stmt, true, afterReturn)
			}
			return
		case *ast.ForStmt:
			walk(n.Init, conditional, afterReturn)
			walk(n.Cond, conditional, afterReturn)
			walk(n.Post, conditional, afterReturn)
			walk(n.Body, true, afterReturn)
			return
		case *ast.RangeStmt:
			walk(n.X, conditional, afterReturn)
			walk(n.Body, true, afterReturn)
			return
		case *ast.SelectStmt:
			for _, stmt := range n.Body.List {
				walk(stmt, true, afterReturn)
			}
			return
		case *ast.CallExpr:
			if isHookCall(n) || isEventHookCall(n) {
				why := "hook is registered inside a branch or loop"
				if afterReturn {
					why = "hook is registered after an early return"
				}
				if conditional || afterReturn {
					pos := fset.Position(n.Pos())
					findings = append(findings, hookFinding{file: rel, function: function, line: pos.Line, why: why})
				}
			}
			if id, ok := n.Fun.(*ast.Ident); ok && hasHooks[pkg+"."+id.Name] && (conditional || afterReturn) {
				pos := fset.Position(n.Pos())
				findings = append(findings, hookFinding{file: rel, function: function, line: pos.Line, why: "hook-calling helper " + id.Name + " is called conditionally"})
			}
			for _, arg := range n.Args {
				walk(arg, conditional, afterReturn)
			}
			walk(n.Fun, conditional, afterReturn)
			return
		}
		ast.Inspect(node, func(child ast.Node) bool {
			if child == nil || child == node {
				return true
			}
			// Preserve lexical control flow through the custom walker.
			walk(child, conditional, afterReturn)
			return false
		})
	}
	walk(body, false, false)
	return findings
}

// returnEffect reports whether a statement may return from the current
// function and whether every path through it returns. Constant if conditions
// keep dead and guaranteed exits from making later hooks look conditional.
func returnEffect(stmt ast.Stmt) (mayReturn, mustReturn bool) {
	switch n := stmt.(type) {
	case *ast.ReturnStmt:
		return true, true
	case *ast.BlockStmt:
		for _, child := range n.List {
			may, must := returnEffect(child)
			if must {
				return true, true
			}
			mayReturn = mayReturn || may
		}
		return mayReturn, false
	case *ast.IfStmt:
		condition, known := constantBool(n.Cond)
		if known {
			if condition {
				return returnEffect(n.Body)
			}
			if n.Else != nil {
				return returnEffect(n.Else)
			}
			return false, false
		}
		bodyMay, bodyMust := returnEffect(n.Body)
		elseMay, elseMust := false, false
		if n.Else != nil {
			elseMay, elseMust = returnEffect(n.Else)
		}
		return bodyMay || elseMay, bodyMust && elseMust
	case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		// These constructs can choose a path that does not return (or run zero
		// times), so they never make following statements unreachable by themselves.
		return false, false
	}
	return false, false
}

func constantBool(expr ast.Expr) (bool, bool) {
	if value, ok := expr.(*ast.Ident); ok {
		if value.Name == "true" {
			return true, true
		}
		if value.Name == "false" {
			return false, true
		}
	}
	return false, false
}

func isHookCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !strings.HasPrefix(sel.Sel.Name, "Use") {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "ui" || pkg.Name == "uistate" || pkg.Name == "router")
}

func isEventHookCall(call *ast.CallExpr) bool {
	// GWC's shorthand event prop constructors are bare identifiers (OnClick,
	// OnInput, ...). A selector such as props.OnChange(value) is an ordinary
	// callback invocation, not a prop option that registers UseEvent.
	selector, ok := call.Fun.(*ast.Ident)
	if !ok || len(selector.Name) < 3 || !strings.HasPrefix(selector.Name, "On") || selector.Name[2] < 'A' || selector.Name[2] > 'Z' || len(call.Args) == 0 {
		return false
	}
	return isFuncValue(call.Args[0])
}

func isFuncValue(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		// Event handlers are normally function-valued identifiers. A Handler
		// interface is safe, but the AST-only lint cannot resolve aliases; the
		// explicit conversion makes that distinction visible and intentional.
		return value.Name != "nil" && value.Name != "Handler"
	case *ast.CallExpr:
		if id, ok := value.Fun.(*ast.Ident); ok && id.Name == "Prevent" && len(value.Args) > 0 {
			return isFuncValue(value.Args[0])
		}
		if _, ok := value.Fun.(*ast.FuncLit); ok {
			return true
		}
	case *ast.ParenExpr:
		return isFuncValue(value.X)
	}
	return false
}
