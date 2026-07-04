// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerFieldsSurface emits the bespoke /fields surface — a from-scratch
// "schema ledger", NOT the shared bento/tile/card kit: the field registry on
// the left (one ruled group per entity, spec-line rows with monospace keys and
// live cf_* formula handles) and a sticky labeled composer on the right with
// its "what this field will do" footprint. Registered after the generated
// rules so these win equal-specificity ties.
func registerFieldsSurface() {
	const mono = "ui-monospace, SFMono-Regular, Menlo, monospace"

	// ── The deck: registry column + composer rail, filling the content width. ──
	rule(".fld-deck",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 23rem"),
		prop("gap", "2.5rem"),
		prop("align-items", "start"),
	)
	ruleMedia("(max-width: 1100px)", ".fld-deck",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	ruleMedia("(max-width: 1100px)", ".fld-composer",
		prop("order", "-1"),
		prop("position", "static"),
		prop("border-left", "none"),
		prop("padding-left", "0"),
		prop("max-width", "34rem"),
	)

	// ── Registry masthead ──────────────────────────────────────────────────────
	rule(".fld-reg-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".fld-kicker",
		prop("font-size", "0.72rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.14em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--accent)"),
	)
	rule(".fld-reg-count",
		prop("font-family", mono),
		prop("font-size", "0.74rem"),
		prop("opacity", "0.6"),
	)
	rule(".fld-reg-lede",
		prop("font-size", "0.88rem"),
		prop("line-height", "1.5"),
		prop("opacity", "0.7"),
		prop("max-width", "42rem"),
		prop("margin", "0.5rem 0 0"),
	)

	// ── Ledger groups ──────────────────────────────────────────────────────────
	rule(".fld-groups",
		prop("margin-top", "1.25rem"),
	)
	rule(".fld-group",
		prop("padding", "1.05rem 0 1.15rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".fld-group:first-child",
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".fld-group-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.6rem"),
	)
	rule(".fld-group-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("margin", "0"),
	)
	rule(".fld-group-count",
		prop("font-family", mono),
		prop("font-size", "0.74rem"),
		prop("color", "var(--accent)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 35%, transparent)"),
		prop("border-radius", "999px"),
		prop("padding", "0 0.45rem"),
		prop("line-height", "1.35"),
	)
	rule(".fld-define",
		prop("margin-left", "auto"),
		prop("background", "none"),
		prop("border", "none"),
		prop("padding", "0"),
		prop("font-size", "0.78rem"),
		prop("font-weight", "600"),
		prop("color", "var(--accent)"),
		prop("cursor", "pointer"),
		prop("opacity", "0.85"),
	)
	rule(".fld-define:hover",
		prop("opacity", "1"),
		prop("text-decoration", "underline"),
	)
	// The compressed empty-entities line: dashed "open slot" chips that start a
	// definition for that entity in the composer.
	rule(".fld-undefined",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("padding", "1.05rem 0"),
	)
	rule(".fld-undef-label",
		prop("font-size", "0.82rem"),
		prop("opacity", "0.6"),
	)
	rule(".fld-undef-chip",
		prop("font-size", "0.78rem"),
		prop("font-weight", "600"),
		prop("color", "inherit"),
		prop("background", "none"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("padding", "0.2rem 0.7rem"),
		prop("cursor", "pointer"),
		prop("opacity", "0.8"),
	)
	rule(".fld-undef-chip:hover",
		prop("border-color", "var(--accent)"),
		prop("color", "var(--accent)"),
		prop("opacity", "1"),
	)

	// ── Spec-line rows ─────────────────────────────────────────────────────────
	rule(".fld-rows",
		prop("margin-top", "0.35rem"),
	)
	// The delete control is the only trailing track — the formula chip lives in
	// the sub-line — so every row's × stays ruled-column aligned regardless of
	// which rows carry a variable.
	rule(".fld-row",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(4.8rem, max-content) minmax(0, 1fr) auto"),
		prop("gap", "0.9rem"),
		prop("align-items", "center"),
		prop("padding", "0.6rem 0"),
	)
	rule(".fld-confirm",
		prop("grid-column", "1 / -1"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.6rem"),
		prop("padding", "0.15rem 0 0.25rem"),
	)
	// The warning reads in normal fg for contrast safety on both themes; the
	// danger tone lives on the destructive button itself.
	rule(".fld-confirm-msg",
		prop("font-size", "0.8rem"),
		prop("opacity", "0.85"),
	)
	rule(".fld-confirm-del",
		prop("font-size", "0.76rem"),
		prop("font-weight", "600"),
		prop("color", "var(--danger)"),
		prop("background", "none"),
		prop("border", "1px solid var(--danger)"),
		prop("border-radius", "6px"),
		prop("padding", "0.2rem 0.6rem"),
		prop("cursor", "pointer"),
	)
	rule(".fld-confirm-del:hover",
		prop("background", "color-mix(in srgb, var(--danger) 12%, transparent)"),
	)
	rule(".fld-confirm-keep",
		prop("font-size", "0.76rem"),
		prop("font-weight", "600"),
		prop("color", "inherit"),
		prop("background", "none"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "6px"),
		prop("padding", "0.2rem 0.6rem"),
		prop("cursor", "pointer"),
		prop("opacity", "0.85"),
	)
	rule(".fld-row + .fld-row",
		prop("border-top", "1px solid color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".fld-type",
		prop("font-family", mono),
		prop("font-size", "0.6rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.08em"),
		prop("text-transform", "uppercase"),
		prop("text-align", "center"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "4px"),
		prop("padding", "0.22rem 0.2rem"),
		prop("opacity", "0.8"),
		prop("white-space", "nowrap"),
	)
	rule(".fld-row-top",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
	)
	rule(".fld-label",
		prop("font-weight", "600"),
		prop("font-size", "0.92rem"),
	)
	rule(".fld-req",
		prop("font-size", "0.68rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.06em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--accent)"),
	)
	rule(".fld-row-sub",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.7rem"),
		prop("margin-top", "0.1rem"),
	)
	rule(".fld-key",
		prop("font-family", mono),
		prop("font-size", "0.76rem"),
		prop("opacity", "0.65"),
	)
	rule(".fld-opts",
		prop("font-size", "0.78rem"),
		prop("opacity", "0.55"),
	)
	rule(".fld-var",
		prop("font-family", mono),
		prop("font-size", "0.72rem"),
		prop("color", "var(--accent)"),
		prop("background", "color-mix(in srgb, var(--accent) 10%, transparent)"),
		prop("border-radius", "4px"),
		prop("padding", "0.18rem 0.4rem"),
		prop("white-space", "nowrap"),
	)

	// ── Composer rail: a labeled margin-note column, not a card ────────────────
	rule(".fld-composer",
		prop("position", "sticky"),
		prop("top", "1rem"),
		prop("border-left", "1px solid var(--border)"),
		prop("padding-left", "1.75rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".fld-comp-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("margin", "0"),
	)
	rule(".fld-comp-lede",
		prop("font-size", "0.82rem"),
		prop("line-height", "1.5"),
		prop("opacity", "0.65"),
		prop("margin", "0.4rem 0 1rem"),
	)
	rule(".fld-form",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".fld-field",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
		prop("margin-bottom", "0.85rem"),
	)
	rule(".fld-lbl",
		prop("font-size", "0.66rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.1em"),
		prop("text-transform", "uppercase"),
		prop("opacity", "0.55"),
	)
	rule(".fld-field .field",
		prop("width", "100%"),
	)
	rule(".fld-hint",
		prop("font-size", "0.68rem"),
		prop("opacity", "0.55"),
	)

	// The live footprint: what the field being composed will actually do.
	rule(".fld-foot",
		prop("border-left", "3px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
		prop("padding", "0.15rem 0 0.15rem 0.8rem"),
		prop("margin", "0.25rem 0 1.1rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
	)
	rule(".fld-foot-title",
		prop("font-size", "0.62rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.12em"),
		prop("text-transform", "uppercase"),
		prop("opacity", "0.5"),
	)
	rule(".fld-foot-line",
		prop("font-size", "0.8rem"),
		prop("line-height", "1.45"),
		prop("opacity", "0.78"),
		prop("margin", "0"),
	)
	rule(".fld-submit",
		prop("align-self", "stretch"),
	)
}
