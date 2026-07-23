// SPDX-License-Identifier: MIT

package styles

// registerPlanSurface emits the /plan ("Fix My Finances") roadmap design: the
// playbook segmented control, the yes/no/not-sure question chips, the numbered
// step ladder with its status pills, and the free-credit-score link grid. Hero,
// chip, and section chrome reuse the shared rpt-*/debt-* rules; everything here
// is token-based so it tracks both themes. Registered from Register().
func registerPlanSurface() {
	// The bento host: let the tiles size to content rather than a fixed row height.
	rule(".bento.bento-sys#plan-page > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)

	// Beginner intro line under the hero eyebrow.
	rule(".plan-intro",
		prop("max-width", "52ch"),
		prop("margin", "0.35rem 0 0.9rem"),
		prop("line-height", "1.5"),
	)

	// The plain-English one-liner is the primary text on each step; the "why" detail
	// sits under it, quieter.
	rule(".plan-step-plain",
		prop("color", "var(--text)"),
		prop("font-weight", "500"),
		prop("line-height", "1.45"),
		prop("margin-bottom", "0.2rem"),
	)

	// ── Playbook segmented control. ─────────────────────────────────────────────
	rule(".plan-seg",
		prop("display", "inline-flex"),
		prop("padding", "3px"),
		prop("gap", "3px"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("background", "var(--bg)"),
	)
	rule(".plan-seg-btn",
		prop("appearance", "none"),
		prop("border", "0"),
		prop("cursor", "pointer"),
		prop("padding", "0.4rem 0.9rem"),
		prop("border-radius", "calc(var(--radius) - 3px)"),
		prop("font", "inherit"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("background", "transparent"),
	)
	rule(".plan-seg-btn.is-on",
		prop("background", "var(--bg-card)"),
		prop("color", "var(--text)"),
		prop("box-shadow", "0 1px 2px rgba(0,0,0,0.08)"),
	)

	// ── Onboarding questions. ───────────────────────────────────────────────────
	rule(".plan-q",
		prop("padding", "0.75rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".plan-q:first-of-type", prop("border-top", "0"))
	rule(".plan-q-label",
		prop("font-weight", "600"),
		prop("color", "var(--text)"),
		prop("margin-bottom", "0.15rem"),
	)
	rule(".plan-choices",
		prop("display", "flex"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.5rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".plan-choice-btn",
		prop("appearance", "none"),
		prop("cursor", "pointer"),
		prop("padding", "0.35rem 0.85rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("font", "inherit"),
		prop("color", "var(--text-dim)"),
		prop("background", "var(--bg-card)"),
	)
	rule(".plan-choice-btn.is-on",
		prop("background", "var(--accent)"),
		prop("border-color", "var(--accent)"),
		prop("color", "#fff"),
		prop("font-weight", "600"),
	)

	// ── The step ladder. ────────────────────────────────────────────────────────
	rule(".plan-ladder",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".plan-step",
		prop("display", "flex"),
		prop("gap", "0.85rem"),
		prop("align-items", "flex-start"),
		prop("padding", "0.85rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("background", "var(--bg-card)"),
	)
	rule(".plan-step.is-current",
		prop("border-color", "var(--accent)"),
		prop("box-shadow", "inset 3px 0 0 0 var(--accent)"),
		prop("background", "color-mix(in srgb, var(--accent) 5%, var(--bg-card))"),
	)
	rule(".plan-step-num",
		prop("flex", "0 0 auto"),
		prop("width", "1.9rem"),
		prop("height", "1.9rem"),
		prop("display", "grid"),
		prop("place-items", "center"),
		prop("border-radius", "999px"),
		prop("background", "var(--bg)"),
		prop("border", "1px solid var(--border)"),
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "var(--text-dim)"),
	)
	rule(".plan-step.is-current .plan-step-num",
		prop("background", "var(--accent)"),
		prop("border-color", "var(--accent)"),
		prop("color", "#fff"),
	)
	rule(".plan-step-body", prop("flex", "1 1 auto"), prop("min-width", "0"))
	rule(".plan-step-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("margin-bottom", "0.2rem"),
	)
	rule(".plan-step-title",
		prop("font-size", "1.05rem"),
		prop("font-weight", "700"),
		prop("color", "var(--text)"),
	)

	// ── Status pills. ───────────────────────────────────────────────────────────
	rule(".plan-pill",
		prop("flex", "0 0 auto"),
		prop("font-size", "0.72rem"),
		prop("font-weight", "700"),
		prop("letter-spacing", "0.02em"),
		prop("text-transform", "uppercase"),
		prop("padding", "0.15rem 0.55rem"),
		prop("border-radius", "999px"),
		prop("white-space", "nowrap"),
	)
	rule(".plan-pill.is-done",
		prop("background", "color-mix(in srgb, var(--up) 16%, transparent)"),
		prop("color", "var(--up)"),
	)
	rule(".plan-pill.is-now",
		prop("background", "var(--accent)"),
		prop("color", "#fff"),
	)
	rule(".plan-pill.is-todo",
		prop("background", "var(--bg)"),
		prop("color", "var(--text-dim)"),
		prop("border", "1px solid var(--border)"),
	)
	rule(".plan-pill.is-ask",
		prop("background", "color-mix(in srgb, var(--warn) 16%, transparent)"),
		prop("color", "var(--warn)"),
	)

	// ── Free-credit link grid. ──────────────────────────────────────────────────
	rule(".plan-credit-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(auto-fit, minmax(220px, 1fr))"),
		prop("gap", "0.75rem"),
	)
	rule(".plan-credit-card",
		prop("display", "block"),
		prop("padding", "0.85rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("background", "var(--bg-card)"),
		prop("text-decoration", "none"),
		prop("transition", "border-color 120ms ease, transform 120ms ease"),
	)
	rule(".plan-credit-card:hover",
		prop("border-color", "var(--accent)"),
		prop("transform", "translateY(-1px)"),
	)
	rule(".plan-credit-name",
		prop("font-weight", "700"),
		prop("color", "var(--text)"),
		prop("margin-bottom", "0.2rem"),
	)
}
