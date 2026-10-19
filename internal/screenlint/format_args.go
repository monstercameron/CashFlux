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
			got := len(call.Args) - 1
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
		// Skip flags, width, precision and positional indices until the first
		// letter. Stop there even if it is unsupported, so a literal percent
		// followed by prose cannot consume a later letter from that word.
		for i < len(format) && !isASCIIAlpha(format[i]) {
			i++
		}
		if i < len(format) && isFormatVerb(format[i]) {
			count++
		}
	}
	return count
}

func isFormatVerb(char byte) bool {
	return strings.ContainsRune("aAbBcCdDeEfFgGopqsSTUvxX%", rune(char))
}

func isASCIIAlpha(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func screensRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "screens")
}
