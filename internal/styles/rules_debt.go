// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerDebtCoachSurface emits the /debt coaching-surface rules added in the
// interactive redesign: the Watch-outs alert cards, the strategy tuner (segmented
// method picker + extra-payment stepper + live readout), and the teaching
// accordion. It reuses the page's existing visual grammar — the colored left rail
// that says "how hot", the serif display numerals, the config-driven band colors —
// so the new tiles read as one system with the ladder and summary above them.
//
// Chained from install.go's Register(). The band colors (danger / #f59e0b amber /
// accent) match the debt-card rails in rules_gen.go on purpose.
func registerDebtCoachSurface() {
	// Jump-nav targets must land BELOW the fixed header AND the sticky plan bar, not
	// behind them. An ID-list rule in rules_gen.go set only 1.25rem (pre-plan-bar, and
	// missing the new sections); this equal-specificity ID rule is emitted later (this
	// surface registers last), so it wins and covers every section with the measured
	// header height plus the plan bar's height and gaps.
	rule("#sec-overview, #sec-watchouts, #sec-ladder, #sec-tuner, #sec-strategy, #sec-credit, #sec-loans, #sec-calculator, #sec-learn",
		prop("scroll-margin-top", "calc(var(--debt-header, 101px) + 4.5rem)"),
	)

	// Sticky plan-summary bar — pinned below the header so the active plan stays in
	// view down the long page. Solid background so scrolled tiles never show through;
	// z-index below the app topbar (which is 5). grid-column spans the whole bento.
	rule(".debt-planbar",
		position("sticky"),
		// Sit flush below the measured sticky header (published by the tile) plus a
		// small gap; fall back to a sane desktop value before the measure lands.
		prop("top", "calc(var(--debt-header, 101px) + 0.4rem)"),
		zIndex("4"),
		prop("grid-column", "1 / -1"),
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("1.75rem"),
		padding("0.6rem 1.1rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-card)"),
		boxShadow("0 6px 18px -8px rgba(0,0,0,0.45)"),
	)
	rule(".debt-planbar-item",
		display("flex"),
		flexDirection("column"),
		gap("0.05rem"),
	)
	rule(".debt-planbar-label",
		fontSize("var(--type-11)"),
		prop("text-transform", "uppercase"),
		letterSpacing("0.06em"),
	)
	rule(".debt-planbar-value",
		fontSize("1.05rem"),
		fontWeight("650"),
		fontVariantNumeric("tabular-nums"),
	)

	// The plan-scope note under the hero debt-free line (e.g. "excludes $153,720").
	rule(".debt-hero-note",
		fontSize("0.82rem"),
		prop("margin", "0.1rem 0 0"),
	)

	// --- Watch-outs: severity-railed alert cards ---
	rule(".debt-alerts",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".debt-alert",
		position("relative"),
		display("flex"),
		alignItems("flex-start"),
		gap("0.75rem"),
		padding("0.85rem 1rem 0.85rem 1.25rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
	)
	// A colored left rail per severity — the same "how hot" cue as the ladder cards.
	rule(".debt-alert-rail",
		position("absolute"),
		left("0"),
		top("0"),
		bottom("0"),
		width("5px"),
		prop("border-top-left-radius", "14px"),
		prop("border-bottom-left-radius", "14px"),
		background("var(--accent)"),
	)
	rule(".debt-alert-critical",
		borderColor("color-mix(in srgb, var(--danger) 45%, var(--border))"),
		background("color-mix(in srgb, var(--danger) 7%, var(--bg-elev))"),
	)
	rule(".debt-alert-critical .debt-alert-rail",
		background("var(--danger)"),
	)
	rule(".debt-alert-critical .debt-alert-icon",
		color("var(--danger)"),
	)
	rule(".debt-alert-watch",
		borderColor("color-mix(in srgb, #f59e0b 45%, var(--border))"),
		background("color-mix(in srgb, #f59e0b 6%, var(--bg-elev))"),
	)
	rule(".debt-alert-watch .debt-alert-rail",
		background("#f59e0b"),
	)
	rule(".debt-alert-watch .debt-alert-icon",
		color("#f59e0b"),
	)
	rule(".debt-alert-info .debt-alert-rail",
		background("var(--accent)"),
	)
	rule(".debt-alert-info .debt-alert-icon",
		color("var(--accent)"),
	)
	rule(".debt-alert-icon",
		prop("flex", "0 0 auto"),
		display("inline-flex"),
		prop("margin-top", "0.1rem"),
	)
	rule(".debt-alert-body",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
	)
	rule(".debt-alert-title",
		fontWeight("650"),
		fontSize("0.98rem"),
		color("var(--text)"),
	)
	rule(".debt-alert-text",
		fontSize("0.9rem"),
		lineHeight("1.5"),
		margin("0"),
	)
	// The all-clear state — calm, not empty.
	rule(".debt-allclear",
		display("flex"),
		alignItems("center"),
		gap("0.85rem"),
		padding("1rem 1.15rem"),
		border("1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--accent) 6%, var(--bg-elev))"),
	)
	rule(".debt-allclear-icon",
		color("var(--accent)"),
		display("inline-flex"),
		prop("flex", "0 0 auto"),
	)
	rule(".debt-allclear-title",
		fontWeight("650"),
		fontSize("1.05rem"),
	)
	rule(".debt-allclear-text",
		fontSize("0.9rem"),
		lineHeight("1.5"),
		margin("0.1rem 0 0"),
	)

	// --- Strategy tuner ---
	rule(".debt-tuner-grid",
		display("grid"),
		gridTemplateColumns("1fr 1fr"),
		gap("1.25rem"),
		prop("margin-top", "0.75rem"),
	)
	rule(".debt-tuner-block",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		minWidth("0"),
	)
	rule(".debt-tuner-label",
		fontSize("var(--type-11)"),
		prop("text-transform", "uppercase"),
		letterSpacing("0.06em"),
	)
	// Segmented method picker — two tactile buttons, active one accent-tinted.
	rule(".seg",
		display("flex"),
		gap("0.4rem"),
	)
	rule(".seg-btn",
		prop("flex", "1 1 0"),
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		alignItems("flex-start"),
		padding("0.6rem 0.75rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		cursor("pointer"),
		prop("text-align", "left"),
		transition("border-color 0.16s ease, background 0.16s ease, transform 0.16s ease"),
	)
	rule(".seg-btn:hover",
		borderColor("color-mix(in srgb, var(--accent) 40%, var(--border))"),
		transform("translateY(-1px)"),
	)
	rule(".seg-btn.is-active",
		borderColor("color-mix(in srgb, var(--accent) 70%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 14%, var(--bg-elev))"),
	)
	rule(".seg-btn-title",
		fontWeight("650"),
		fontSize("0.98rem"),
		color("var(--text)"),
	)
	rule(".seg-btn-sub",
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		lineHeight("1.3"),
	)
	// Extra-payment stepper.
	rule(".debt-stepper",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".debt-step-btn",
		prop("flex", "0 0 auto"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("2.5rem"),
		height("2.5rem"),
		fontSize("1.35rem"),
		fontWeight("500"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 55%, transparent)"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color 0.16s ease, background 0.16s ease"),
	)
	rule(".debt-step-btn:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 10%, var(--bg-elev))"),
	)
	rule(".debt-step-input",
		width("7.5rem"),
		prop("text-align", "center"),
		fontVariantNumeric("tabular-nums"),
		fontSize("1.1rem"),
		fontWeight("600"),
	)
	rule(".debt-tuner-chips",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
	)
	// Small pill buttons beside the stepper (Suggest / Clear).
	rule(".chip-btn",
		display("inline-flex"),
		alignItems("center"),
		padding("0.3rem 0.7rem"),
		fontSize("var(--type-12)"),
		fontWeight("550"),
		color("var(--text-dim)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		cursor("pointer"),
		transition("border-color 0.16s ease, color 0.16s ease, background 0.16s ease"),
	)
	rule(".chip-btn:hover",
		color("var(--text)"),
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 10%, var(--bg-elev))"),
	)
	// Live readout row + impact callout.
	rule(".debt-tuner-stats",
		display("flex"),
		flexWrap("wrap"),
		gap("0.5rem"),
		prop("margin-top", "1rem"),
	)
	rule(".debt-tuner-impact",
		prop("margin", "0.75rem 0 0"),
		padding("0.7rem 0.9rem"),
		borderRadius("var(--radius-lg)"),
		border("1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		borderLeft("3px solid var(--accent)"),
		background("color-mix(in srgb, var(--accent) 7%, var(--bg-elev))"),
		fontSize("0.92rem"),
		lineHeight("1.5"),
	)
	rule(".debt-tuner-impact.muted",
		border("1px dashed var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
	)

	// --- Teaching accordion ---
	rule(".debt-learn",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		prop("margin-top", "0.5rem"),
	)
	rule(".debt-learn-item",
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		overflow("hidden"),
	)
	rule(".debt-learn-q",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		padding("0.8rem 1rem"),
		cursor("pointer"),
		fontWeight("600"),
		fontSize("0.98rem"),
		color("var(--text)"),
		prop("list-style", "none"),
	)
	rule(".debt-learn-q::-webkit-details-marker",
		display("none"),
	)
	// A rotating chevron marker built from a border triangle.
	rule(".debt-learn-q::after",
		prop("content", "''"),
		prop("flex", "0 0 auto"),
		width("0.5rem"),
		height("0.5rem"),
		prop("border-right", "2px solid var(--text-dim)"),
		prop("border-bottom", "2px solid var(--text-dim)"),
		transform("rotate(45deg)"),
		transition("transform 0.2s ease"),
	)
	rule("details[open] > .debt-learn-q::after",
		transform("rotate(-135deg)"),
	)
	rule("details[open] > .debt-learn-q",
		color("var(--accent)"),
	)
	rule(".debt-learn-a",
		prop("margin", "0"),
		padding("0 1rem 0.9rem"),
		fontSize("0.9rem"),
		lineHeight("1.65"),
	)

	// Stack the tuner columns on narrow content widths (pane-not-viewport, so keyed
	// off the content column like the rest of the surface — see breakpoints.go).
	ruleContentMax(contentGrid4, ".debt-tuner-grid",
		gridTemplateColumns("1fr"),
		gap("1rem"),
	)
}
