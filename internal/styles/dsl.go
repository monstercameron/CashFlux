// SPDX-License-Identifier: MIT

// Package styles is the app's design system authored as type-safe Go.
//
// It replaces the hand-written CSS that used to live in web/index.html's <style>
// blocks. Every rule is a typed Go expression: a typed selector plus typed property
// declarations built from typed value constructors (props.go) and design tokens
// (tokens.go). The package accumulates these into an ordered stylesheet and renders
// it to CSS text, which Go injects into the document at boot (inject.go). No CSS text
// lives outside Go, and the authoring surface is compile-checked — changing a style
// is a Go edit, not a string edit.
//
// It is deliberately independent of the css/tw utility engine (which keeps emitting
// the element-level utility classes into <style id="gwc-css">): the two coexist, the
// design system in <style id="cf-app-css"> and the utilities in <style id="gwc-css">.
// A dedicated builder is used here because the design system is *global, literal*
// CSS (semantic class selectors, element/id selectors, literal-named @keyframes,
// @media blocks) — none of which the hashed-class utility engine can express.
//
// Layering:
//   - tokens.go    — the design tokens (palette/fonts) as typed values + the :root block.
//   - props.go     — typed property constructors (one per CSS property; no raw property
//     names at call sites). Each returns a decl.
//   - <section>.go — the component rules, authored with rule()/ruleMedia()/keyframes().
//   - install.go   — Register() runs every section's emit at boot; Build() renders;
//     Inject() (wasm) writes the result into <style id="cf-app-css">.
package styles

import "strings"

// decl is one typed declaration: a property name and its rendered value. Built only
// through the typed constructors in props.go / tokens.go, never inline.
type decl struct {
	prop  string
	value string
}

// cssRule is one rule in the ordered stylesheet: either a normal selector rule
// (optionally wrapped in an at-rule like @media), or a verbatim raw block (used for
// @keyframes, which are referenced by literal name and so can't be hashed).
type cssRule struct {
	atRule   string // e.g. "@media print" or "@media (max-width:768px)"; "" for none
	selector string // the literal CSS selector (".btn", "body", ".bento > .w", …)
	decls    []decl
	raw      string // a complete verbatim block (e.g. an @keyframes); when set, the rest is ignored
}

// sheet is the ordered accumulation of every emitted rule. Source order is preserved
// exactly, so the CSS cascade matches the original hand-written stylesheet.
var sheet []cssRule

// rule appends one global rule — `selector { declarations }` — to the stylesheet.
func rule(selector string, decls ...decl) {
	if len(decls) == 0 {
		return
	}
	sheet = append(sheet, cssRule{selector: selector, decls: decls})
}

// ruleMedia appends a rule wrapped in an @media at-rule:
// `@media <query> { selector { declarations } }`. query is the bare query
// (e.g. "print", "(max-width:768px)", "(prefers-reduced-motion:reduce)").
func ruleMedia(query, selector string, decls ...decl) {
	if len(decls) == 0 {
		return
	}
	sheet = append(sheet, cssRule{atRule: "@media " + query, selector: selector, decls: decls})
}

// rawBlock appends a verbatim CSS block (the escape hatch for constructs that have
// no typed form — literal-named @keyframes via keyframes(), and standalone at-rules
// like @page/@font-face that the transpiler keeps verbatim).
func rawBlock(css string) {
	if css == "" {
		return
	}
	sheet = append(sheet, cssRule{raw: css})
}

// rawBlockMedia appends a verbatim block wrapped in an @media at-rule (e.g. an
// @page rule nested inside @media print).
func rawBlockMedia(query, css string) {
	if css == "" {
		return
	}
	sheet = append(sheet, cssRule{atRule: "@media " + query, raw: css})
}

// customProp declares a CSS custom property (CSS variable): customProp("--bg", v).
func customProp(name, v string) decl { return decl{name, v} }

// prop is the typed wrapper for any property the generated layer doesn't name (kept
// for hand-authored rules and forward-compat). It still types the call site.
func prop(name, v string) decl { return decl{name, v} }

// Frame is one keyframe step: an offset ("0%", "from", "to", "100%") and its decls.
type Frame struct {
	Offset string
	Decls  []decl
}

// at builds a keyframe Frame.
func at(offset string, decls ...decl) Frame { return Frame{Offset: offset, Decls: decls} }

// keyframes appends an @keyframes animation with a LITERAL name (so component rules
// can reference it by `animation-name: <name>`), built from typed frames.
func keyframes(name string, frames ...Frame) {
	var b strings.Builder
	b.WriteString("@keyframes ")
	b.WriteString(name)
	b.WriteByte('{')
	for _, f := range frames {
		b.WriteString(f.Offset)
		b.WriteByte('{')
		for _, d := range f.Decls {
			b.WriteString(d.prop)
			b.WriteByte(':')
			b.WriteString(d.value)
			b.WriteByte(';')
		}
		b.WriteByte('}')
	}
	b.WriteByte('}')
	rawBlock(b.String())
}

// important marks a declaration !important (the typed analog of the CSS modifier).
func important(d decl) decl {
	if !strings.Contains(d.value, "!important") {
		d.value += " !important"
	}
	return d
}

// resetSheet clears the accumulated stylesheet — used by tests between cases.
func resetSheet() { sheet = nil }

// Build renders the accumulated stylesheet to CSS text in source order.
func Build() string {
	var b strings.Builder
	for _, r := range sheet {
		if r.raw != "" {
			if r.atRule != "" {
				b.WriteString(r.atRule + "{" + r.raw + "}")
			} else {
				b.WriteString(r.raw)
			}
			continue
		}
		var body strings.Builder
		for _, d := range r.decls {
			body.WriteString(d.prop)
			body.WriteByte(':')
			body.WriteString(d.value)
			body.WriteByte(';')
		}
		block := r.selector + "{" + body.String() + "}"
		if r.atRule != "" {
			block = r.atRule + "{" + block + "}"
		}
		b.WriteString(block)
	}
	return b.String()
}
