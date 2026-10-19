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

	"github.com/monstercameron/CashFlux/internal/i18n"
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
// The prose ratchet counts strings with prose or sentence punctuation, plus
// single capitalized words. A separate ratchet catches lowercase display words
// while excluding lowercase schema values such as field names. i18n keys,
// routes, css utility soup, glyphs and urls are ignored.
var i18nBaselines = map[string]int{
	// UI layer (wasm screens + app chrome). Baselines set 2026-07-03 (C361);
	// first conversion tranche brought screens 211→126, later sweeps to 115, and
	// the 2026-07-19 third tranche (en_i18nsweep3.go: theme editor, widget
	// builder, studio designer, txn table columns, and the long tail) to 0.
	// The whole UI layer is now at zero — keep it there.
	"../screens": 0,
	"../app":     0,
	// Shared component library + UI-adjacent packages (second sweep pass —
	// Cam: "every page AND component"). widgetregistry's 2 are the preset
	// content-layout templates persisted into user specs (C362 class:
	// translate at spec-creation time, not render time).
	"../ui":           0,
	"../uistate":      0,
	"../widgetrender": 0,
	// 2→0 (2026-08-16, C362): the spotlight preset's text blocks carry a TextKey
	// beside their English, so the copy baked into a user's persisted spec
	// resolves through the catalog at render time instead of being frozen in
	// whatever language they placed the widget in.
	"../widgetregistry": 0,
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
	// 163→0 (2026-08-16, C362): every detector pairs its English with a catalog
	// key through Insight.WithTitle/WithDetail and Action.WithLabel, and
	// smartengine.localize resolves them at the one door insights leave the
	// package through.
	"../smartengine": 0,
	// 42→0 (2026-08-16, C362): every widget, column, chart and preset label now
	// carries its catalog key beside its English, resolved through the translator
	// the app installs at boot (widgetcatalog.SetTranslator).
	"../widgetcatalog": 0,
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
	// C361: help.go's own section builder. Its body lines were plain string
	// arguments — a display position the scanner did not look at — so the entire
	// help centre sat outside the language setting while every screen around it
	// was at zero. A blind spot in the guard is indistinguishable from compliance.
	"helpTopicBody": true,
}

// i18nAllArgStringFuncs are calls where EVERY string argument is user-visible,
// not just the first. helpTopicBody takes a variadic list of paragraphs; checking
// only its first would have left the second line of every help topic unguarded.
var i18nAllArgStringFuncs = map[string]bool{
	"helpTopicBody":    true,
	"PostNotice":       true,
	"setTxnSaveStatus": true,
	"txnCountFigure":   true,
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
	i18nLetterRe  = regexp.MustCompile(`[A-Za-z]`)
	i18nKeyRe     = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*\.[a-zA-Z0-9_.\-]+$`)
	i18nVerbRe    = regexp.MustCompile(`%[#+\-\d.*]*[a-zA-Z%]`)
	i18nLowerWord = regexp.MustCompile(`^[a-z]{2,}$`)
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
		return false // css utility soup / identifiers; lowercase copy has its own ratchet
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

func i18nClassifyLowerExpr(e ast.Expr) (string, ast.Expr) {
	for _, lit := range i18nLiteralsWithin(e) {
		s, _ := i18nLiteral(lit)
		trim := strings.TrimSpace(s)
		if i18nLowerWord.MatchString(trim) {
			return trim, lit
		}
	}
	return "", nil
}

func budgetCountKeyIssues(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	bundle := i18n.DefaultBundle()
	var issues []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || i18nCalleeName(call) != "budgetCountKey" || len(call.Args) < 1 {
				return true
			}
			prefix, ok := i18nLiteral(call.Args[0])
			if !ok {
				issues = append(issues, entry.Name()+": budgetCountKey prefix must be a string literal")
				return true
			}
			for _, suffix := range []string{"one", "many"} {
				key := prefix + "." + suffix
				if bundle.T(i18n.English, key) == key {
					issues = append(issues, entry.Name()+": missing budgetCountKey catalog key "+key)
				}
			}
			return true
		})
	}
	sort.Strings(issues)
	return issues
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
	return i18nScanDirMode(t, dir, false)
}

func i18nScanLowerDir(t *testing.T, dir string) []string {
	return i18nScanDirMode(t, dir, true)
}

func i18nScanDirMode(t *testing.T, dir string, lowercase bool) []string {
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
		// Values already reachable through a sibling <Field>Key (C362).
		hasCatalogKey := map[token.Pos]bool{}
		classifyExpr := i18nClassifyExpr
		if lowercase {
			classifyExpr = i18nClassifyLowerExpr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				name := i18nCalleeName(node)
				if i18nElementFuncs[name] {
					for _, arg := range node.Args {
						if s, lit := classifyExpr(arg); s != "" {
							record(lit.Pos(), name+"(child)", s)
						}
					}
				}
				if i18nPropFuncs[name] && len(node.Args) >= 1 {
					if s, lit := classifyExpr(node.Args[0]); s != "" {
						record(lit.Pos(), name+"(prop)", s)
					}
				}
				if i18nAllArgStringFuncs[name] {
					for _, arg := range node.Args {
						if s, lit := classifyExpr(arg); s != "" {
							record(lit.Pos(), name+"(arg)", s)
						}
					}
				}
				if name == "Attr" && len(node.Args) == 2 {
					if an, ok := i18nLiteral(node.Args[0]); ok && i18nAttrNames[an] {
						if s, lit := classifyExpr(node.Args[1]); s != "" {
							record(lit.Pos(), "Attr("+an+")", s)
						}
					}
				}
			case *ast.CompositeLit:
				if lowercase {
					break // field names such as "name" and "currency" are schema values
				}
				// A display field paired with a <Field>Key sibling IS reachable by
				// the catalog — the literal is that key's fallback, not the only
				// copy of the sentence (C362). Mark its value so the KeyValueExpr
				// case below leaves it alone. Inspect visits a composite literal
				// before its elements, so the marks are always in place in time.
				keyed := map[string]bool{}
				for _, el := range node.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok && strings.HasSuffix(id.Name, "Key") {
							keyed[strings.TrimSuffix(id.Name, "Key")] = true
						}
					}
				}
				for _, el := range node.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok && keyed[id.Name] {
							hasCatalogKey[kv.Value.Pos()] = true
						}
					}
				}
			case *ast.KeyValueExpr:
				if id, ok := node.Key.(*ast.Ident); ok && i18nDisplayFields[id.Name] && !hasCatalogKey[node.Value.Pos()] {
					if s, lit := classifyExpr(node.Value); s != "" {
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
	if issues := budgetCountKeyIssues(t, "../screens"); len(issues) > 0 {
		t.Errorf("budgetCountKey prefix has no catalog entry:\n  %s", strings.Join(issues, "\n  "))
	}
	for dir, baseline := range map[string]int{"../screens": 33, "../widgetsource": 41} {
		if findings := i18nScanLowerDir(t, dir); len(findings) > baseline {
			t.Errorf("%s: %d lowercase user-facing words (baseline %d) — route new copy through the catalog:\n  %s",
				dir, len(findings), baseline, strings.Join(findings, "\n  "))
		}
	}
	budgetFindings := make([]string, 0)
	for _, finding := range i18nScanLowerDir(t, "../screens") {
		if strings.HasPrefix(finding, "budgets") {
			budgetFindings = append(budgetFindings, finding)
		}
	}
	if len(budgetFindings) != 0 {
		t.Errorf("budget screens contain lowercase user-facing literals:\n  %s", strings.Join(budgetFindings, "\n  "))
	}
}

func TestHardcodedEnglishScannerChecksNoticeStatusAndLabelHelpers(t *testing.T) {
	dir := t.TempDir()
	source := `package screens
func sample() {
	PostNotice("These changes are not saved. Try again.", true)
	setTxnSaveStatus("txn", "Saving...")
	txnCountFigure("spent", "Spent", 10)
}`
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	findings := strings.Join(i18nScanDir(t, dir), "\n")
	for _, want := range []string{"These changes are not saved", "Saving...", "Spent"} {
		if !strings.Contains(findings, want) {
			t.Errorf("scanner missed %q; findings:\n%s", want, findings)
		}
	}
}

func TestHardcodedEnglishScannerFindsLowercaseDisplayWords(t *testing.T) {
	dir := t.TempDir()
	source := `package screens
func sample() { Span("of") }`
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if findings := strings.Join(i18nScanLowerDir(t, dir), "\n"); !strings.Contains(findings, `"of"`) {
		t.Fatalf("scanner missed lowercase display word; findings:\n%s", findings)
	}
}

func TestBudgetCountKeyRatchetResolvesBothPluralKeys(t *testing.T) {
	dir := t.TempDir()
	source := `package screens
func sample(n int) { budgetCountKey("missing.prefix", n) }`
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if issues := budgetCountKeyIssues(t, dir); len(issues) != 2 {
		t.Fatalf("invalid plural prefix produced %d issues, want one/many failures: %v", len(issues), issues)
	}
}
