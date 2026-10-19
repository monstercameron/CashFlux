// SPDX-License-Identifier: MIT

// Package screenlint contains static checks for user-facing screen code.
package screenlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/i18n"
)

// FormatArgOffender describes a literal uistate.T call whose argument count
// does not match the format verbs in its English catalog entry.
type FormatArgOffender struct {
	File string
	Line int
	Key  string
	Got  int
	Want int
}

// FindTranslationArgCountOffenders scans Go files beneath screens for calls to
// uistate.T with literal keys and compares their arguments with the catalog.
func FindTranslationArgCountOffenders(root string) ([]FormatArgOffender, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	bundle := i18n.DefaultBundle()
	catalog := bundle.Langs[i18n.English]
	fset := token.NewFileSet()
	var offenders []FormatArgOffender
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		formattedTranslations := make(map[*ast.CallExpr]int)
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isFmtSprintf(call.Fun) || len(call.Args) < 1 {
				return true
			}
			translation, ok := call.Args[0].(*ast.CallExpr)
			if ok && isUistateT(translation.Fun) {
				formattedTranslations[translation] = len(call.Args) - 1
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isUistateT(call.Fun) || len(call.Args) == 0 {
				return true
			}
			keyLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || keyLit.Kind != token.STRING {
				return true
			}
			key, unquoteErr := strconv.Unquote(keyLit.Value)
			if unquoteErr != nil {
				return true
			}
			template, exists := catalog[key]
			if !exists {
				return true
			}
			want := countFormatVerbs(template)
			got, formatted := formattedTranslations[call]
			if !formatted {
				got = len(call.Args) - 1
			}
			if got != want {
				pos := fset.Position(call.Pos())
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				offenders = append(offenders, FormatArgOffender{File: filepath.ToSlash(rel), Line: pos.Line, Key: key, Got: got, Want: want})
			}
			return true
		})
		return nil
	})
	return offenders, err
}

func isUistateT(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "uistate"
}

func isFmtSprintf(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "fmt"
}

func countFormatVerbs(format string) int {
	count := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		if format[i] == ' ' || format[i] == '\t' || format[i] == '\n' || format[i] == '\r' {
			// Catalog prose sometimes uses an unescaped percentage before a
			// word ("50% needs"). It is not a conversion in these templates.
			continue
		}
		if format[i] == '%' {
			continue
		}
		if format[i] == ' ' || format[i] == '\t' || format[i] == '\n' || format[i] == '\r' {
			// A percent followed by whitespace is prose (for example "30% of").
			continue
		}
		// Parse a printf directive rather than scanning prose for the next
		// letter. This keeps percentages such as "80%. Counts" literal.
		start := i
		for i < len(format) && strings.ContainsRune("#0+- ", rune(format[i])) {
			i++
		}
		if i < len(format) && format[i] == '[' {
			if end := strings.IndexByte(format[i:], ']'); end >= 0 {
				i += end + 1
			}
		}
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		if i < len(format) && format[i] == '.' {
			i++
			if i < len(format) && format[i] == '[' {
				if end := strings.IndexByte(format[i:], ']'); end >= 0 {
					i += end + 1
				}
			}
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		if i < len(format) && isFormatVerb(format[i]) {
			count++
		} else {
			i = start
		}
	}
	return count
}

func isFormatVerb(char byte) bool {
	return strings.ContainsRune("bcdoxXUeEfgGstTqvwp%", rune(char))
}

func screensRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "screens")
}
