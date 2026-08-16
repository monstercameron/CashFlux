// SPDX-License-Identifier: MIT

// Command copyconv rewrites smartengine insight construction from baked English
// to the key + args form (C362).
//
// It exists because the conversion is 150-odd sites of prose surgery: each
// Title/Detail/Label is a `+`-concatenation chain mixing literal text with
// formatted values, and turning one into a format string plus an argument list
// by hand is both tedious and easy to get subtly wrong (an argument in the wrong
// order still compiles and still reads as English). Doing it over the AST makes
// the decomposition mechanical and total.
//
// It is a one-shot developer tool, not part of the app build.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: copyconv <file.go>...")
		os.Exit(2)
	}
	keys := map[string]string{}
	for _, path := range os.Args[1:] {
		n, err := convert(path, keys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("%s: %d converted\n", path, n)
	}
	var out []string
	for k, v := range keys {
		out = append(out, "\t"+strconv.Quote(k)+": "+strconv.Quote(v)+",")
	}
	sort.Strings(out)
	os.WriteFile("copyconv_keys.txt", []byte(strings.Join(out, "\n")), 0o644)
}

var featureRe = regexp.MustCompile(`SMART-([A-Z]+)(\d+)`)

// keyFor turns a feature code and a field into a catalog key.
func keyFor(feature, field string, seq int) string {
	m := featureRe.FindStringSubmatch(feature)
	if m == nil {
		return ""
	}
	k := "smart." + strings.ToLower(m[1]) + m[2] + "." + field
	if seq > 0 {
		k += strconv.Itoa(seq + 1)
	}
	return k
}

// decompose turns a `+`-concatenation expression into a printf format string
// and the non-literal operands, in order. Literal text is escaped so a stray %
// in the copy cannot become a verb.
func decompose(fset *token.FileSet, e ast.Expr) (format string, args []ast.Expr, ok bool) {
	var walk func(ast.Expr) bool
	var b strings.Builder
	walk = func(x ast.Expr) bool {
		switch v := x.(type) {
		case *ast.BinaryExpr:
			if v.Op != token.ADD {
				return false
			}
			return walk(v.X) && walk(v.Y)
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return false
			}
			b.WriteString(strings.ReplaceAll(s, "%", "%%"))
			return true
		case *ast.ParenExpr:
			return walk(v.X)
		default:
			b.WriteString("%s")
			args = append(args, x)
			return true
		}
	}
	if !walk(e) {
		return "", nil, false
	}
	return b.String(), args, true
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var buf bytes.Buffer
	_ = format.Node(&buf, fset, e)
	return buf.String()
}

func convert(path string, keys map[string]string) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}

	type edit struct {
		start, end int    // byte offsets of the KeyValueExpr (plus trailing comma) to delete
		chain      string // ".WithTitle(...)" text to append after the literal
		litEnd     int    // byte offset just after the composite literal's closing brace
	}
	var edits []edit
	count := 0

	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		typeName := sel.Sel.Name
		if typeName != "Insight" && typeName != "Action" {
			return true
		}
		// Find the feature code: on an Insight it is a field; on an Action we
		// take it from the enclosing function's nearest Insight (handled by the
		// caller passing -feature).
		feature := ""
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok || id.Name != "Feature" {
				continue
			}
			if b, ok := kv.Value.(*ast.BasicLit); ok && b.Kind == token.STRING {
				feature, _ = strconv.Unquote(b.Value)
			}
		}
		if typeName == "Action" {
			feature = currentFeature
		} else {
			currentFeature = feature
		}
		if feature == "" {
			return true
		}

		fields := map[string]string{"Title": "title", "Detail": "detail", "Label": "action"}
		var chain strings.Builder
		var drop []ast.Node
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			field, want := fields[id.Name]
			if !want {
				continue
			}
			fstr, args, ok := decompose(fset, kv.Value)
			if !ok || fstr == "" {
				continue
			}
			key := keyFor(feature, field, seqFor(feature, field))
			if key == "" {
				continue
			}
			keys[key] = fstr
			method := map[string]string{"Title": "WithTitle", "Detail": "WithDetail", "Label": "WithLabel"}[id.Name]
			var parts []string
			parts = append(parts, strconv.Quote(key), strconv.Quote(fstr))
			for _, a := range args {
				parts = append(parts, exprString(fset, a))
			}
			chain.WriteString("." + method + "(" + strings.Join(parts, ", ") + ")")
			drop = append(drop, el)
			count++
		}
		if chain.Len() == 0 {
			return true
		}
		for _, d := range drop {
			edits = append(edits, edit{start: fset.Position(d.Pos()).Offset, end: fset.Position(d.End()).Offset})
		}
		edits = append(edits, edit{start: -1, litEnd: fset.Position(lit.End()).Offset, chain: chain.String()})
		return true
	})

	if count == 0 {
		return 0, nil
	}
	out := string(src)
	// Apply from the end so offsets stay valid.
	sort.Slice(edits, func(i, j int) bool {
		a, b := edits[i].start, edits[j].start
		if a < 0 {
			a = edits[i].litEnd
		}
		if b < 0 {
			b = edits[j].litEnd
		}
		return a > b
	})
	for _, e := range edits {
		if e.start < 0 {
			out = out[:e.litEnd] + e.chain + out[e.litEnd:]
			continue
		}
		end := e.end
		for end < len(out) && (out[end] == ',' || out[end] == ' ' || out[end] == '\t') {
			end++
		}
		if end < len(out) && out[end] == '\n' {
			end++
		}
		out = out[:e.start] + out[end:]
	}
	// Break the builder chain across lines so gofmt can indent it: a
	// forty-argument WithDetail on one line is technically correct and unreadable.
	out = chainBreak.ReplaceAllString(out, ".\n$1(")
	formatted, err := format.Source([]byte(out))
	if err != nil {
		return 0, fmt.Errorf("reformat: %w", err)
	}
	return count, os.WriteFile(path, formatted, 0o644)
}

// chainBreak matches a builder call that follows another on the same line.
var chainBreak = regexp.MustCompile(`\.(WithTitle|WithDetail|WithLabel|WithAmount|WithAction|WithConfidence)\(`)

var currentFeature string
var seqCount = map[string]int{}

func seqFor(feature, field string) int {
	k := feature + "/" + field
	n := seqCount[k]
	seqCount[k] = n + 1
	return n
}
