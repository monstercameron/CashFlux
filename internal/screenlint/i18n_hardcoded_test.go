// SPDX-License-Identifier: MIT

package screenlint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ─── C361/C362 hardcoded-English ratchet ─────────────────────────────────────
//
// Every user-facing string must come from the i18n catalog (uistate.T /
// i18n keys), never a hardcoded literal — otherwise the language setting
// silently doesn't apply to that copy. This test AST-scans the UI layer and
// the copy-producing logic packages for English literals in display positions:
//
//   - bare string children of html/shorthand element calls (Div, Span, P, …),
//   - display prop options (Title/Placeholder/Alt) and
//     Attr("aria-label"/"title"/"placeholder"/"alt", …),
//   - display-named struct fields (Title:, Label:, Detail:, Message:, …),
//   - including literals reached through `+` concatenation and fmt.Sprintf.
//
// It is a ONE-WAY RATCHET: the per-directory baselines below may only fall.
// New hardcoded copy fails this test — route it through the catalog instead
// (see internal/i18n/en_*.go for the extension-file pattern). When you convert
// strings, lower the matching baseline to the new count.
//
// A literal counts when it looks like prose (letters + a space, or sentence
// punctuation) or is a single capitalized word ≥3 letters. i18n keys, routes,
// css utility soup, glyphs, urls, and all-lowercase identifiers are ignored.
var i18nBaselines = map[string]int{
	// UI layer (wasm screens + app chrome). Baselines set 2026-07-03 (C361);
	// first conversion tranche brought screens 211→126 and app 17→0.
	"../screens": 125,
	"../app":     0,
	// Shared component library + UI-adjacent packages (second sweep pass —
	// Cam: "every page AND component"). widgetregistry's 2 are the preset
	// content-layout templates persisted into user specs (C362 class:
	// translate at spec-creation time, not render time).
	"../ui":             0,
	"../uistate":        0,
	"../widgetrender":   0,
	"../widgetregistry": 2,
	"../pages":          0,
	"../mermaid":        0,
	"../chartspec":      0,
	// Logic packages that produce user-facing copy (C362: these need the
	// key+args architecture — insights/notifications currently bake English
	// at generation time, and notification copy is persisted pre-formatted).
	// 160→166 (2026-07-14): smartengine is pure (no uistate.T), so new detectors
	// carry their insight copy in-package like every other engine; the baseline
	// moves with them — SMART-BL16 price-creep, new-merchant/trial, and the BG6
	// seasonal auto-budget true-up.
	// 166→169 (2026-07-14): GL3 SMART-G21 emergency-fund resize re-suggest adds
	// its three in-package insight strings (Title/Detail/Label).
	// 169→171 (2026-07-14): AC14 SMART-A9 fee-bleed detector adds its in-package
	// insight Detail and the close-it-task action Label.
	"../smartengine":   171,
	"../widgetcatalog": 42,
	"../healthscore":   0,
	"../credithealth":  0,
	"../attention":     0,
	"../widgetsource":  0,
	"../notify":        0,
	"../subscriptions": 0,
	"../billsched":     0,
}

var i18nElementFuncs = map[string]bool{
	"Div": true, "Span": true, "P": true, "Button": true, "A": true, "Td": true,
	"Th": true, "Li": true, "Ul": true, "Ol": true, "H1": true, "H2": true, "H3": true,
	"H4": true, "H5": true, "H6": true, "Label": true, "Option": true, "Text": true,
	"Textf": true, "Strong": true, "Em": true, "Small": true, "Legend": true,
	"Summary": true, "Figcaption": true, "Caption": true, "Dt": true, "Dd": true,
	"Header": true, "Footer": true, "Section": true, "Article": true, "Aside": true,
	"Main": true, "Nav": true, "Blockquote": true, "Pre": true, "Code": true,
	"Tr": true, "Table": true, "Form": true, "Fieldset": true,
}

// i18nPropFuncs are calls whose FIRST string arg is user-visible: display prop
// options plus the screens' local label helpers (labeledField et al) — the
// helper-argument blind spot the second sweep pass closed.
var i18nPropFuncs = map[string]bool{
	"Title": true, "Placeholder": true, "Alt": true,
	"labeledField": true, "withFieldLabel": true, "smartBrandHeader": true,
}

var i18nDisplayFields = map[string]bool{
	"Title": true, "Subtitle": true, "Label": true, "Detail": true, "Message": true,
	"Body": true, "Text": true, "Summary": true, "Description": true, "Hint": true,
	"CTALabel": true, "SearchLabel": true, "FiltersLabel": true, "EmptyLabel": true,
	"Name": true, "Eyebrow": true, "Caption": true, "Tooltip": true, "Prefix": true,
	"Suffix": true, "Heading": true, "Sub": true, "Note": true, "Question": true,
}

var i18nAttrNames = map[string]bool{
	"aria-label": true, "title": true, "placeholder": true,
	"aria-description": true, "alt": true,
}

var (
	i18nLetterRe = regexp.MustCompile(`[A-Za-z]`)
	i18nKeyRe    = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*\.[a-zA-Z0-9_.\-]+$`)
	i18nVerbRe   = regexp.MustCompile(`%[#+\-\d.*]*[a-zA-Z%]`)
)

// i18nExempt are exact literals that are deliberately NOT translated: brand and
// product names, and universal keyboard-shortcut notation. Add here only when a
// string genuinely must read the same in every language.
var i18nExempt = map[string]bool{
	"CashFlux":     true,
	"CashFlux ":    true,
	"GPT-5.4 mini": true,
	"Alt + %d":     true,
}

func i18nLiteral(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func i18nCalleeName(c *ast.CallExpr) string {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// i18nLiteralsWithin collects string literals contributing to a display value:
// bare literals, `a + b` concatenations, and Sprintf/Sprint args. T(...) calls
// are opaque (already translated).
func i18nLiteralsWithin(e ast.Expr) []ast.Expr {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return []ast.Expr{v}
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return append(i18nLiteralsWithin(v.X), i18nLiteralsWithin(v.Y)...)
		}
	case *ast.ParenExpr:
		return i18nLiteralsWithin(v.X)
	case *ast.CallExpr:
		name := i18nCalleeName(v)
		if name == "T" {
			return nil
		}
		if name == "Sprintf" || name == "Sprint" || name == "Errorf" {
			var out []ast.Expr
			for _, a := range v.Args {
				out = append(out, i18nLiteralsWithin(a)...)
			}
			return out
		}
	}
	return nil
}

func i18nClassify(s string) bool {
	if i18nExempt[s] {
		return false
	}
	s = i18nVerbRe.ReplaceAllString(s, " ")
	if !i18nLetterRe.MatchString(s) || i18nKeyRe.MatchString(s) {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "--") ||
		strings.HasPrefix(s, "http") || strings.HasPrefix(s, "data:") {
		return false
	}
	trim := strings.TrimSpace(s)
	if trim == "" {
		return false
	}
	if strings.ToLower(trim) == trim && !strings.ContainsAny(trim, ".!?,’'") {
		return false // css utility soup / identifiers
	}
	letters := 0
	for _, r := range trim {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letters++
		}
	}
	if strings.Contains(trim, " ") && letters >= 4 {
		return true
	}
	return letters >= 3 && trim[0] >= 'A' && trim[0] <= 'Z' && strings.ToUpper(trim) != trim
}

func i18nClassifyExpr(e ast.Expr) (string, ast.Expr) {
	for _, lit := range i18nLiteralsWithin(e) {
		s, _ := i18nLiteral(lit)
		if i18nClassify(s) {
			return s, lit
		}
	}
	return "", nil
}

func i18nScanDir(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var finds []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		record := func(pos token.Pos, ctx, s string) {
			if len(s) > 60 {
				s = s[:60] + "…"
			}
			p := fset.Position(pos)
			finds = append(finds, fmt.Sprintf("%s:%d %s %q", e.Name(), p.Line, ctx, s))
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				name := i18nCalleeName(node)
				if i18nElementFuncs[name] {
					for _, arg := range node.Args {
						if s, lit := i18nClassifyExpr(arg); s != "" {
							record(lit.Pos(), name+"(child)", s)
						}
					}
				}
				if i18nPropFuncs[name] && len(node.Args) >= 1 {
					if s, lit := i18nClassifyExpr(node.Args[0]); s != "" {
						record(lit.Pos(), name+"(prop)", s)
					}
				}
				if name == "Attr" && len(node.Args) == 2 {
					if an, ok := i18nLiteral(node.Args[0]); ok && i18nAttrNames[an] {
						if s, lit := i18nClassifyExpr(node.Args[1]); s != "" {
							record(lit.Pos(), "Attr("+an+")", s)
						}
					}
				}
			case *ast.KeyValueExpr:
				if id, ok := node.Key.(*ast.Ident); ok && i18nDisplayFields[id.Name] {
					if s, lit := i18nClassifyExpr(node.Value); s != "" {
						record(lit.Pos(), id.Name+":", s)
					}
				}
			}
			return true
		})
	}
	sort.Strings(finds)
	return finds
}

// TestNoNewHardcodedEnglish enforces the C361/C362 baselines: hardcoded
// user-facing English may only decrease. Run with -v to list every finding.
func TestNoNewHardcodedEnglish(t *testing.T) {
	dirs := make([]string, 0, len(i18nBaselines))
	for d := range i18nBaselines {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		finds := i18nScanDir(t, dir)
		base := i18nBaselines[dir]
		if len(finds) > base {
			t.Errorf("%s: %d hardcoded user-facing strings (baseline %d) — new copy must go through the i18n catalog (uistate.T + internal/i18n/en_*.go).\nFindings:\n  %s",
				dir, len(finds), base, strings.Join(finds, "\n  "))
		} else if len(finds) < base {
			t.Logf("%s: %d hardcoded strings, baseline %d — lower the baseline in i18n_hardcoded_test.go to lock in the progress", dir, len(finds), base)
		}
		if testing.Verbose() {
			for _, f := range finds {
				t.Logf("%s %s", dir, f)
			}
		}
	}
}
