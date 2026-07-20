// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerSmartSurface emits the bespoke /assistant Smart tab (and /smart route)
// surface — a from-scratch editorial layout, NOT the shared bento/tile/card kit:
// a masthead that leads with the findings count and the agent's voice, then the
// proven sections stacked as bespoke blocks whose legacy card chrome is dissolved
// so they read as editorial sections rather than stacked tiles. Registered after
// the generated rules so these win equal-specificity ties (and beat the
// !important .smart-card box via higher specificity).
func registerSmartSurface() {
	// ── The deck: one editorial column that fills the content width like every
	// other page (reports/health .bento are max-width:none). No arbitrary cap. ───
	rule(".smt-deck",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "2rem"),
	)

	// ── Masthead ─────────────────────────────────────────────────────────────────
	rule(".smt-masthead",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
		prop("padding-bottom", "1.5rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".smt-kicker",
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.14em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--accent)"),
	)
	rule(".smt-headline",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-top", "0.15rem"),
	)
	rule(".smt-count",
		prop("font-size", "3.6rem"),
		prop("line-height", "1"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.02em"),
	)
	rule(".smt-count-label",
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.08em"),
		prop("text-transform", "uppercase"),
	)
	// The agent's voice — a serif editorial line ruled with a soft accent spine.
	rule(".smt-voice",
		prop("font-size", "var(--type-20)"),
		prop("font-style", "italic"),
		prop("line-height", "1.4"),
		prop("margin", "0.5rem 0 0"),
		prop("padding-left", "0.75rem"),
		prop("border-left", "2px solid color-mix(in srgb, var(--accent) 55%, transparent)"),
	)
	// Posture metrics: quiet overline labels over display-serif values.
	rule(".smt-metrics",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "2.25rem"),
		prop("margin-top", "1.1rem"),
	)
	rule(".smt-metric",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
	)
	rule(".smt-metric-label",
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.1em"),
		prop("text-transform", "uppercase"),
		prop("opacity", "0.55"),
	)
	rule(".smt-metric-value",
		prop("font-size", "1.3rem"),
		prop("font-weight", "600"),
		prop("line-height", "1.1"),
	)
	rule(".smt-fine",
		prop("font-size", "0.76rem"),
		prop("opacity", "0.6"),
		prop("margin", "1.1rem 0 0"),
		prop("max-width", "44rem"),
	)

	// ── Sections: dissolve the legacy card chrome so each block reads as a bespoke
	// editorial section, and speak the same serif accent-tick title language. ─────
	rule(".smt-deck .card",
		prop("background", "none"),
		prop("border", "none"),
		prop("box-shadow", "none"),
		prop("border-radius", "0"),
		prop("padding", "0"),
	)
	rule(".smt-deck .card-head",
		prop("padding", "0"),
		prop("margin-bottom", "0.9rem"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("row-gap", "0.4rem"),
	)
	rule(".smt-deck .card-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "var(--type-20)"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
	)

	// ── Findings feed: the bordered/rounded/shadowed .smart-card box dissolves into
	// an editorial row — a severity-colored left tick + a bottom hairline, no box.
	// Higher specificity than the generated .smart-card !important box (0,2,0 vs
	// 0,1,0; the severity variants use 0,3,0 to keep their color). ────────────────
	rule(".smt-deck .smart-card",
		prop("background", "none !important"),
		prop("border-top", "none !important"),
		prop("border-right", "none !important"),
		prop("border-bottom", "1px solid var(--border) !important"),
		prop("border-left", "3px solid var(--border) !important"),
		prop("border-radius", "0 !important"),
		prop("box-shadow", "none !important"),
		prop("padding", "0.85rem 0.6rem 0.85rem 0.95rem !important"),
	)
	rule(".smt-deck .smart-card:hover",
		prop("box-shadow", "none !important"),
	)
	rule(".smt-deck .smart-card[data-severity=\"alert\"]",
		prop("border-left-color", "var(--danger) !important"),
	)
	rule(".smt-deck .smart-card[data-severity=\"warn\"]",
		prop("border-left-color", "#cfa14e !important"),
	)
	rule(".smt-deck .smart-card[data-severity=\"nudge\"]",
		prop("border-left-color", "var(--accent) !important"),
	)
	// The feed list loses its inter-card gap (rows share hairlines) and its last
	// row drops the trailing rule.
	rule(".smt-deck [data-testid=\"smart-insights\"] .smart-card:last-child",
		prop("border-bottom", "none !important"),
	)
}
