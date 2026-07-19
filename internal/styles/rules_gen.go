// SPDX-License-Identifier: MIT
//
// The app design system as type-safe Go. Migrated once (losslessly) from the three
// <style> blocks that used to live in web/index.html, and now the canonical source —
// edit it directly as Go. Rules are emitted in source order so the CSS cascade is
// preserved exactly. See dsl.go for rule()/ruleMedia()/keyframes() and props_gen.go
// for the typed property constructors.

package styles

// registerGenerated emits the app design system (the former index.html <style>
// blocks) as typed rules, in source order so the cascade is preserved.
func registerGenerated() {
	rule(":root",
		customProp("--bg", "#0e0e0f"),
		customProp("--bg-elev", "#1a1a1d"),
		customProp("--bg-card", "#121214"),
		// Hover surface — a touch brighter than the card. Previously only defined in the
		// light theme, so `var(--hover)` was undefined in dark mode and every hover that
		// used it (kebab/overflow menu items, glyph toolbar buttons) had no background.
		customProp("--hover", "#26262b"),
		customProp("--border", "#2a2a2c"),
		customProp("--text", "#f4f4f5"),
		// 2026-07-17 audit: secondary text ran below comfortable contrast (account
		// metadata, task notes, notification copy, chart labels). Both tiers rise
		// one step — dim ~9.1:1, faint ~6.3:1 on the near-black base — still
		// clearly subordinate to --text, no longer straining.
		customProp("--text-dim", "#b6b6be"),
		customProp("--text-faint", "#9a9aa2"),
		customProp("--accent", "#2e8b57"),
		customProp("--accent-dim", "#1f2c24"),
		customProp("--brand", "#7c83ff"),
		customProp("--danger", "#d8716f"),
		customProp("--radius", "0px"),
		// Z-index scale — one place to reason about stacking order. Higher layer wins.
		customProp("--z-raised", "5"),     // lifted-in-card fills / figs
		customProp("--z-sticky", "20"),    // sticky headers / action bars
		customProp("--z-dropdown", "50"),  // row ⋯ menus, selects
		customProp("--z-overlay", "60"),   // in-page overlays
		customProp("--z-popover", "2000"), // floating explainers / tooltips (over content)
		customProp("--z-modal", "3000"),   // flip-panel modals + their backdrop
		customProp("--z-dialog", "3500"),  // confirm/prompt dialogs — above modals so they're answerable from within one
		customProp("--z-toast", "4000"),   // toasts / notices (above everything)
		colorScheme("dark"),
		customProp("--font-ui", "Inter"),
		customProp("--font-display", "'Fraunces'"),
		customProp("--banner-bg", "none"),
		customProp("--icon-stroke", "1.6"),
	)
	rule(".app-banner",
		display("none"),
	)
	rule(":root[data-banner=\"on\"] .app-banner",
		display("block"),
		height("132px"),
		marginBottom("1rem"),
		backgroundImage("var(--banner-bg, none)"),
		backgroundSize("cover"),
		backgroundPosition("center"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		boxShadow("inset 0 -44px 64px -34px rgba(0,0,0,0.55)"),
	)
	rule("*",
		boxSizing("border-box"),
	)
	rule("*, ::before, ::after",
		borderWidth("0"),
		borderStyle("solid"),
		borderColor("currentColor"),
	)
	rule("img, svg, video, canvas, audio, iframe, embed, object",
		display("block"),
		verticalAlign("middle"),
	)
	rule("img, video",
		maxWidth("100%"),
		height("auto"),
	)
	rule("button, input, optgroup, select, textarea",
		fontFamily("inherit"),
		fontFeatureSettings("inherit"),
		fontSize("100%"),
		fontWeight("inherit"),
		lineHeight("inherit"),
		color("inherit"),
		margin("0"),
		padding("0"),
	)
	rule("button, select",
		textTransform("none"),
	)
	rule("button, [type='button'], [type='reset'], [type='submit']",
		webkitAppearance("button"),
		backgroundColor("transparent"),
		backgroundImage("none"),
		cursor("pointer"),
	)
	rule(":-moz-focusring",
		outline("auto"),
	)
	rule("h1, h2, h3, h4, h5, h6",
		fontSize("inherit"),
		fontWeight("inherit"),
	)
	rule("a",
		color("inherit"),
		textDecoration("inherit"),
	)
	rule("ol, ul, menu",
		listStyle("none"),
		margin("0"),
		padding("0"),
	)
	rule("p, h1, h2, h3, h4, h5, h6, blockquote, figure, pre, fieldset, hr",
		margin("0"),
	)
	rule("table",
		borderCollapse("collapse"),
	)
	rule("html, body",
		margin("0"),
		height("100%"),
	)
	rule("::selection",
		background("color-mix(in srgb, var(--accent) 28%, transparent)"),
		color("var(--text)"),
	)
	rule("::-moz-selection",
		background("color-mix(in srgb, var(--accent) 28%, transparent)"),
		color("var(--text)"),
	)
	rule("body",
		fontFamily("var(--font-ui), Inter, ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", Roboto, sans-serif"),
		background("var(--bg)"),
		color("var(--text)"),
		fontSize("14.5px"),
		lineHeight("1.45"),
		webkitFontSmoothing("antialiased"),
	)
	ruleMedia("print", "html",
		customProp("--bg", "#fff !important"),
		customProp("--bg-base", "#fff !important"),
		customProp("--bg-card", "#fff !important"),
		customProp("--bg-elev", "#f6f6f6 !important"),
		customProp("--border", "#ccc !important"),
		customProp("--text", "#111 !important"),
		customProp("--text-dim", "#333 !important"),
		customProp("--text-faint", "#555 !important"),
		colorScheme("light !important"),
	)
	ruleMedia("print", "body",
		background("#fff !important"),
	)
	ruleMedia("print", ".cf-shell, main, main.cf-scroll, #cf-page-view, .bento, .card-alert, .w, .bento .w",
		background("#fff !important"),
	)
	ruleMedia("print", "aside.rail, .rail, .topbar, .mobile-tabbar, .skip-link, .sample-banner, .scope-banner,\n        .app-banner, .toast, .reso-control, .home-hero-actions, .filter-strip, .scope-selector,\n        input, select, textarea,\n        \n        button:not(.btn-link),\n        .txn-table .td-actions, .txn-table th.td-actions,\n        .txn-table .td-select, .txn-table th.td-select",
		display("none !important"),
	)
	ruleMedia("print", ".cf-shell",
		display("block !important"),
		height("auto !important"),
		overflow("visible !important"),
	)
	ruleMedia("print", "main.cf-scroll",
		overflow("visible !important"),
		height("auto !important"),
	)
	ruleMedia("print", ".card, .reports-hero, .w, .home-hero",
		boxShadow("none !important"),
		breakInside("avoid"),
		pageBreakInside("avoid"),
	)
	ruleMedia("print", ".txn-table tr",
		breakInside("avoid"),
		pageBreakInside("avoid"),
	)
	ruleMedia("print", ".txn-table thead",
		display("table-header-group"),
	)
	rawBlockMedia("print", "@page{margin: 1.5cm;}")
	rule("#boot",
		position("fixed"),
		inset("0"),
		display("grid"),
		placeItems("center"),
		background("var(--bg)"),
		zIndex("10"),
		transition("opacity var(--motion-narrative) var(--ease-exit), transform var(--motion-narrative) var(--ease-exit)"),
	)
	rule("#boot.hidden",
		opacity("0"),
		transform("scale(1.03)"),
		pointerEvents("none"),
	)
	rule(".boot-card",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap("0.9rem"),
	)
	rule(".boot-mark",
		position("relative"),
		width("64px"),
		height("64px"),
		display("grid"),
		placeItems("center"),
	)
	rule(".boot-ring",
		position("absolute"),
		inset("0"),
		width("64px"),
		height("64px"),
		animation("boot-spin 1.15s linear infinite"),
	)
	rule(".boot-ring-track",
		fill("none"),
		stroke("var(--border)"),
		strokeWidth("3"),
	)
	rule(".boot-ring-arc",
		fill("none"),
		stroke("var(--accent)"),
		strokeWidth("3"),
		strokeLinecap("round"),
		strokeDasharray("70 126"),
		strokeDashoffset("0"),
	)
	rule(".boot-c",
		fontFamily("\"Fraunces\", Georgia, serif"),
		fontWeight("600"),
		fontSize("1.6rem"),
		color("var(--text)"),
		lineHeight("1"),
		animation("boot-breathe 1.8s ease-in-out infinite"),
	)
	rule(".boot-word",
		fontFamily("\"Fraunces\", Georgia, serif"),
		fontSize("1.15rem"),
		fontWeight("600"),
		letterSpacing("0.01em"),
		color("var(--text)"),
		opacity("0"),
		animation("boot-fade-up 0.6s ease 0.1s forwards"),
	)
	rule(".boot-sub",
		fontSize("0.82rem"),
		color("var(--text-faint)"),
		opacity("0"),
		animation("boot-fade-up 0.6s ease 0.25s forwards"),
	)
	keyframes("boot-spin",
		at("to",
			transform("rotate(360deg)"),
		),
	)
	keyframes("boot-breathe",
		at("0%,100%",
			opacity("0.55"),
			transform("scale(0.97)"),
		),
		at("50%",
			opacity("1"),
			transform("scale(1)"),
		),
	)
	keyframes("boot-fade-up",
		at("from",
			opacity("0"),
			transform("translateY(6px)"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	rule("[data-theme=\"light\"] #boot",
		background("#f7f6f3"),
	)
	rule("[data-theme=\"light\"] .boot-card",
		background("transparent"),
	)
	rule("[data-theme=\"light\"] .boot-c, [data-theme=\"light\"] .boot-word",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .boot-sub",
		color("#56565c"),
	)
	rule("[data-theme=\"light\"] .boot-ring-track",
		stroke("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .boot-ring-arc",
		stroke("#56565c"),
	)
	rule("[data-theme=\"light\"] .insights-thinking",
		background("var(--border, #e4e2dd)"),
		color("var(--text, #1c1c1e)"),
	)
	rule("#app",
		zoom("var(--ui-scale, 1)"),
	)
	rule(".skip-link",
		position("absolute"),
		left("8px"),
		top("-48px"),
		zIndex("200"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		padding("0.45rem 0.8rem"),
		fontSize("13px"),
		textDecoration("none"),
		transition("top .12s ease"),
	)
	rule(".skip-link:focus",
		top("8px"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".skip-link",
		transition("none"),
	)
	rule("a:focus-visible, button:focus-visible, input:focus-visible, select:focus-visible,\n      textarea:focus-visible, [tabindex]:focus-visible, [role=\"button\"]:focus-visible,\n      [role=\"switch\"]:focus-visible, [role=\"radio\"]:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
		borderRadius("4px"),
	)
	rule("#app.app-enter",
		animation("app-settle 0.55s cubic-bezier(0.22, 1, 0.36, 1) both"),
	)
	keyframes("app-settle",
		at("from",
			opacity("0"),
			transform("translateY(10px) scale(0.992)"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "#boot, #boot.hidden",
		transition("opacity 0.2s ease"),
		transform("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".boot-ring, .boot-c, .boot-word, .boot-sub, #app.app-enter",
		animation("none"),
		opacity("1"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".flip-inner, .flip-backdrop",
		transition("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".toast",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "aside.rail",
		transition("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "*, *::before, *::after",
		transitionDuration("0.001ms !important"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ":root",
		customProp("--wonder-on", "0 !important"),
		customProp("--wonder-lift", "0px !important"),
		customProp("--wonder-press", "1 !important"),
		customProp("--wonder-shadow", "none !important"),
	)
	rule(":root",
		// Re-tuned onto the v1.2.3 motion scale: fast=120, standard=180, overlay=280.
		// Lift is capped at 1px (standalone cards only), press matches --pressed-scale,
		// and both easings resolve to the three sanctioned curves.
		customProp("--wonder-on", "1"),
		customProp("--wonder-dur-fast", "120ms"),
		customProp("--wonder-dur", "180ms"),
		customProp("--wonder-dur-slow", "280ms"),
		customProp("--wonder-ease", "cubic-bezier(0.2, 0, 0, 1)"),
		customProp("--wonder-ease-out", "cubic-bezier(0.16, 1, 0.3, 1)"),
		customProp("--wonder-lift", "1px"),
		customProp("--wonder-press", ".985"),
		customProp("--wonder-shadow", "0 3px 12px rgba(0,0,0,.12)"),
	)
	rule("[data-wonder=\"off\"]",
		customProp("--wonder-on", "0"),
		customProp("--wonder-dur-fast", "0ms"),
		customProp("--wonder-dur", "0ms"),
		customProp("--wonder-dur-slow", "0ms"),
		customProp("--wonder-lift", "0px"),
		customProp("--wonder-press", "1"),
		customProp("--wonder-shadow", "none"),
	)
	rule("[data-wonder=\"subtle\"]",
		customProp("--wonder-on", ".55"),
		customProp("--wonder-dur", "140ms"),
		customProp("--wonder-lift", "1px"),
		customProp("--wonder-press", ".99"),
		customProp("--wonder-shadow", "0 4px 14px rgba(0,0,0,.12)"),
	)
	rule(".card",
		transition("transform var(--wonder-dur) var(--wonder-ease), box-shadow var(--wonder-dur) var(--wonder-ease), border-color var(--wonder-dur-fast) var(--wonder-ease)"),
	)
	rule(".card:hover",
		transform("translateY(calc(-1 * var(--wonder-lift) * var(--wonder-on)))"),
		boxShadow("var(--wonder-shadow)"),
	)
	rule(".btn, .data-btn, .seg-btn, .add-item, .menu-btn, .icon-btn",
		transition("transform var(--wonder-dur-fast) var(--wonder-ease), background-color var(--wonder-dur-fast) ease, color var(--wonder-dur-fast) ease"),
	)
	// Press scale (spec §3): 0.985 for the micro window. Segmented controls and
	// table/list rows never scale, so .seg-btn is out and role=button is guarded.
	rule(".btn:active, .data-btn:active, .add-item:active, .menu-btn:active, .icon-btn:active,\n      [role=\"button\"]:not(.seg-btn):not(.row):not(tr):not(.nv):active",
		transform("scale(var(--wonder-press))"),
	)
	rule(".w:not(.drag)",
		transition("transform var(--wonder-dur) var(--wonder-ease), box-shadow var(--wonder-dur) var(--wonder-ease), border-color .12s ease"),
	)
	rule(".w:not(.drag):hover",
		transform("translateY(calc(-1 * var(--wonder-lift) * var(--wonder-on)))"),
		boxShadow("var(--wonder-shadow)"),
	)
	// Rows lift one surface step on hover but never translate (spec §3: "Table/list
	// rows do not translate") — background only, at the fast token.
	rule(".row:not(.txn-table .row)",
		transition("background var(--motion-fast) var(--ease-standard)"),
	)
	rule(".nv",
		transition("transform var(--wonder-dur-fast) var(--wonder-ease), background var(--wonder-dur-fast) ease, color var(--wonder-dur-fast) ease, box-shadow var(--wonder-dur-fast) ease"),
	)
	rule("aside.rail .nv",
		position("relative"),
	)
	rule("aside.rail .nv.active::before, aside.rail .nv[aria-current=\"page\"]::before",
		content("\"\""),
		position("absolute"),
		left("0"),
		top("0"),
		bottom("0"),
		width("3px"),
		background("var(--accent)"),
		borderRadius("0 2px 2px 0"),
		animation("wonder-nav-bar-in var(--wonder-dur) var(--wonder-ease-out)"),
	)
	keyframes("wonder-nav-bar-in",
		at("from",
			transform("scaleY(calc(1 - 1 * var(--wonder-on)))"),
			opacity("calc(1 - 0.6 * var(--wonder-on))"),
		),
		at("to",
			transform("scaleY(1)"),
			opacity("1"),
		),
	)
	rule("[data-wonder=\"off\"] aside.rail .nv.active::before,\n      [data-wonder=\"off\"] aside.rail .nv[aria-current=\"page\"]::before",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "aside.rail .nv.active::before, aside.rail .nv[aria-current=\"page\"]::before",
		animation("none"),
	)
	// Nav items respond with surface/color only — no movement on hover (spec §3:
	// no vertical movement for toolbar-style buttons; §1: controls never shift
	// away from the pointer).
	rule(".nav-link",
		transition("background 0.12s, color 0.12s, transform var(--wonder-dur-fast) var(--wonder-ease)"),
	)
	rule(".nav-link:hover",
		transform("translateY(calc(-1px * var(--wonder-on)))"),
	)
	rule(".nv:hover svg, .nv:hover .nav-icon",
		transform("scale(calc(1 + 0.06 * var(--wonder-on)))"),
		transition("transform var(--wonder-dur-fast) var(--wonder-ease)"),
	)
	rule(".gear-inline:hover, .gear-abs:hover",
		transform("rotate(calc(38deg * var(--wonder-on)))"),
		color("#f4f4f5"),
	)
	rule(".add-btn:hover",
		transform("scale(calc(1 + 0.12 * var(--wonder-on)))"),
	)
	rule(".notify-btn:hover",
		transform("rotate(calc(18deg * var(--wonder-on)))"),
	)
	rule(".muzak-btn:hover",
		transform("scale(calc(1 + 0.08 * var(--wonder-on)))"),
	)
	rule("a:focus-visible, button:focus-visible, input:focus-visible, select:focus-visible,\n      textarea:focus-visible, [tabindex]:focus-visible, [role=\"button\"]:focus-visible,\n      [role=\"switch\"]:focus-visible, [role=\"radio\"]:focus-visible",
		transition("outline-offset 100ms var(--wonder-ease), box-shadow 100ms var(--wonder-ease)"),
		outlineOffset("2px"),
	)
	keyframes("wonder-page-enter",
		at("from",
			opacity("0"),
			transform("translateY(calc(8px * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	rule("#cf-page-view.page-enter",
		animation("wonder-page-enter var(--wonder-dur-slow) var(--wonder-ease-out) both"),
	)
	rule("[data-wonder=\"off\"] #cf-page-view.page-enter",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "#cf-page-view.page-enter",
		animation("none"),
	)
	rule("#cf-page-view",
		viewTransitionName("cf-page"),
	)
	keyframes("wonder-xfade-out",
		at("from",
			opacity("1"),
		),
		at("to",
			opacity("0"),
		),
	)
	keyframes("wonder-xfade-in",
		at("from",
			opacity("0"),
		),
		at("to",
			opacity("1"),
		),
	)
	rule("::view-transition-old(cf-page)",
		animation("wonder-xfade-out var(--wonder-dur, 160ms) var(--wonder-ease, ease) both"),
	)
	rule("::view-transition-new(cf-page)",
		animation("wonder-xfade-in var(--wonder-dur, 160ms) var(--wonder-ease, ease) both"),
	)
	rule("[data-wonder=\"off\"] #cf-page-view",
		viewTransitionName("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "#cf-page-view",
		viewTransitionName("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "::view-transition-old(cf-page),\n        ::view-transition-new(cf-page)",
		animation("none"),
	)
	rule(".btn, .btn-primary, .set-btn.save, .data-btn, .add-item",
		position("relative"),
		overflow("hidden"),
	)
	rule(".btn::after, .btn-primary::after, .set-btn.save::after, .data-btn::after, .add-item::after",
		content("''"),
		position("absolute"),
		inset("0"),
		background("radial-gradient(circle, rgba(255,255,255,0.35) 0%, transparent 70%)"),
		transform("scale(0)"),
		opacity("0"),
		transition("transform var(--wonder-dur) var(--wonder-ease-out),\n                    opacity var(--wonder-dur) var(--wonder-ease-out)"),
		pointerEvents("none"),
		borderRadius("inherit"),
	)
	rule(".btn:active::after, .btn-primary:active::after, .set-btn.save:active::after,\n      .data-btn:active::after, .add-item:active::after",
		transform("scale(calc(2.2 * var(--wonder-on)))"),
		opacity("calc(1 * var(--wonder-on))"),
		transitionDuration("0ms"),
	)
	keyframes("wonder-success-pulse",
		at("0%",
			transform("scale(.6)"),
			opacity(".4"),
		),
		at("60%",
			transform("scale(1.15)"),
			opacity("1"),
		),
		at("100%",
			transform("scale(1)"),
			opacity("1"),
		),
	)
	rule(".toast:not(.toast-err)::before",
		animation("wonder-success-pulse calc(var(--wonder-dur-slow) * var(--wonder-on, 1)) var(--wonder-ease-out) both"),
	)
	rule("[data-wonder=\"off\"] .toast:not(.toast-err)::before",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".toast:not(.toast-err)::before",
		animation("none"),
	)
	// (v1.2.3 motion spec §2: ledger/account/task/notification rows are never
	// staggered during ordinary filtering — the old wonder-row-enter entrance
	// cascade replayed on every list re-render, so it is gone entirely. Rows
	// appear with the page; the page-level transition is the only entrance.)
	keyframes("wonder-bento-enter",
		at("from",
			opacity("0"),
			transform("scale(calc(1 - 0.02 * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	// The tile's resting state is VISIBLE (opacity:1); the entrance is purely
	// additive motion layered on top. Fill-mode is `forwards`, not `both`: `both`
	// backfills the from{opacity:0} keyframe as the pre-animation state, so a tile
	// whose entrance animation never runs to completion — e.g. a cold deep-link
	// where the main thread is saturated by wasm boot, or a re-render that restarts
	// the animation mid-flight — could settle at opacity:0 and stay invisible with
	// nothing to re-trigger it (the "top tile doesn't render on a direct URL load"
	// bug). With a visible base + `forwards`, a dropped animation degrades to
	// "shown immediately" instead of "hidden forever".
	rule(".bento .w",
		opacity("1"),
		animation("wonder-bento-enter var(--wonder-dur-slow) var(--wonder-ease-out) forwards"),
	)
	rule("[data-wonder=\"off\"] .bento .w",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".bento .w",
		animation("none"),
	)
	rule(".flip-backdrop",
		transition("opacity .28s var(--wonder-ease),\n                    backdrop-filter var(--wonder-dur-slow) var(--wonder-ease-out)"),
		backdropFilter("blur(0px)"),
	)
	rule(".flip-backdrop.show",
		backdropFilter("blur(calc(3px * var(--wonder-on)))"),
	)
	// Toasts rise and settle exactly once (spec §2: no overshoot) — the old
	// bounce midpoint + elastic bezier are gone; enter curve at the overlay token.
	keyframes("wonder-toast-in",
		at("from",
			opacity("0"),
			transform("translate(-50%, calc(0.6rem * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("translate(-50%, 0)"),
		),
	)
	keyframes("toast-in",
		at("from",
			opacity("0"),
			transform("translate(-50%, calc(0.6rem * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("translate(-50%, 0)"),
		),
	)
	rule(".toast",
		animation("toast-in var(--motion-overlay) var(--ease-enter) both"),
	)
	rule(".toast.hide",
		animation("none"),
		transition("opacity var(--wonder-dur-fast) var(--wonder-ease),\n                    transform var(--wonder-dur-fast) var(--wonder-ease)"),
		opacity("0"),
		transform("translate(-50%, 0.3rem)"),
	)
	rule("[data-wonder=\"off\"] .toast",
		animation("none"),
	)
	rule("[data-wonder=\"off\"] .toast.hide",
		transition("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".toast",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".toast.hide",
		transition("none"),
	)
	keyframes("wonder-shimmer",
		at("from",
			backgroundPosition("-200% 0"),
		),
		at("to",
			backgroundPosition("200% 0"),
		),
	)
	rule(".skeleton",
		background("var(--bg-elev, rgba(255,255,255,0.04))"),
		borderRadius("6px"),
		color("transparent !important"),
		pointerEvents("none"),
	)
	rule(".skeleton.shimmer,\n      .shimmer",
		background("linear-gradient(\n          90deg,\n          var(--bg-elev, rgba(255,255,255,0.04)) 25%,\n          rgba(255,255,255,0.08) 50%,\n          var(--bg-elev, rgba(255,255,255,0.04)) 75%\n        )"),
		backgroundSize("200% 100%"),
		animation("wonder-shimmer calc(var(--wonder-dur-slow) * 4) linear infinite"),
	)
	rule("[data-wonder=\"off\"] .shimmer,\n      [data-wonder=\"off\"] .skeleton.shimmer",
		animation("none"),
		background("var(--bg-elev, rgba(255,255,255,0.04))"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".shimmer, .skeleton.shimmer",
		animation("none"),
		background("var(--bg-elev, rgba(255,255,255,0.04))"),
	)
	rule(".wonder-reveal",
		opacity("0"),
		transform("translateY(calc(14px * var(--wonder-on, 1)))"),
		transition("opacity var(--wonder-dur-slow) var(--wonder-ease-out),\n                    transform var(--wonder-dur-slow) var(--wonder-ease-out)"),
		willChange("opacity, transform"),
	)
	rule(".wonder-reveal.in-view",
		opacity("1"),
		transform("none"),
	)
	rule("[data-wonder=\"off\"] .wonder-reveal,\n      [data-wonder=\"off\"] .wonder-reveal.in-view",
		opacity("1 !important"),
		transform("none !important"),
		transition("none !important"),
		willChange("auto !important"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".wonder-reveal,\n        .wonder-reveal.in-view",
		opacity("1 !important"),
		transform("none !important"),
		transition("none !important"),
		willChange("auto !important"),
	)
	keyframes("wonder-chart-draw",
		at("from",
			strokeDashoffset("1"),
		),
		at("to",
			strokeDashoffset("0"),
		),
	)
	keyframes("wonder-chart-fade",
		at("from",
			opacity("0"),
		),
		at("to",
			opacity("1"),
		),
	)
	rule(".wonder-chart-line",
		strokeDasharray("1"),
		animation("wonder-chart-draw var(--wonder-dur-slow) var(--wonder-ease-out) both"),
	)
	rule(".wonder-chart-area",
		animation("wonder-chart-fade var(--wonder-dur-slow) var(--wonder-ease-out) both"),
	)
	rule("[data-wonder=\"off\"] .wonder-chart-line,\n      [data-wonder=\"off\"] .wonder-chart-area",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".wonder-chart-line,\n        .wonder-chart-area",
		animation("none"),
	)
	rule("#boot-error",
		display("none"),
		position("fixed"),
		inset("auto 1rem 1rem 1rem"),
		zIndex("20"),
		background("#3b0d0d"),
		color("#fecaca"),
		border("1px solid #7f1d1d"),
		padding("0.75rem 1rem"),
		borderRadius("8px"),
		fontFamily("ui-monospace, monospace"),
		fontSize("0.8rem"),
		whiteSpace("pre-wrap"),
	)
	rule(".app",
		minHeight("100vh"),
		display("flex"),
		flexDirection("column"),
	)
	rule(".topbar",
		position("sticky"),
		top("0"),
		zIndex("5"),
		display("flex"),
		alignItems("center"),
		gap("1.25rem"),
		padding("0.75rem 1.25rem"),
		background("rgba(11,16,32,0.85)"),
		backdropFilter("blur(8px)"),
		borderBottom("1px solid var(--border)"),
		flexWrap("nowrap"),
		minHeight("3.5rem"),
	)
	rule(".topbar",
		customProp("--tb-h", "2rem"),
		gap(".85rem"),
	)
	rule(".tb-title",
		marginRight("auto"),
		minWidth("0"),
		// Shrink LAST — the page title (e.g. "Reports") was truncating to "Re…"
		// at ~1100px because the scope/period context yielded no space first.
		flexShrink("1"),
	)
	rule(".tb-context, .tb-actions",
		display("inline-flex"),
		alignItems("center"),
	)
	rule(".tb-context",
		gap(".5rem"),
		// Yield width before the page title does (its member/period controls can
		// compress); keeps short titles from over-truncating on 13" laptops.
		minWidth("0"),
		flexShrink("4"),
	)
	rule(".tb-actions",
		gap(".15rem"),
	)
	rule(".tb-actions::before",
		content("\"\""),
		alignSelf("center"),
		width("1px"),
		height("1.15rem"),
		margin("0 .35rem"),
		background("var(--border)"),
	)
	rule(".topbar .member-switcher",
		height("var(--tb-h)"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-elev, transparent)"),
		color("inherit"),
		fontSize(".82rem"),
		padding("0 1.5rem 0 .6rem"),
		maxWidth("12rem"),
	)
	rule(".topbar .cf-member-switcher-wrap",
		display("inline-flex"),
		alignItems("center"),
		gap(".4rem"),
		flex("none"),
		whiteSpace("nowrap"),
	)
	rule(".topbar .cf-member-switcher-wrap .btn",
		height("var(--tb-h)"),
		display("inline-flex"),
		alignItems("center"),
		borderRadius("8px"),
		fontSize(".82rem"),
	)
	rule(".period-control",
		position("relative"),
		display("inline-flex"),
		alignItems("center"),
		height("var(--tb-h)"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-elev, transparent)"),
		overflow("visible"),
	)
	rule(".period-step",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.7rem"),
		height("100%"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		borderRadius("7px"),
	)
	rule(".period-step:hover",
		background("var(--hover, color-mix(in srgb, var(--text) 8%, transparent))"),
		color("var(--text)"),
	)
	rule(".period-pill",
		display("inline-flex"),
		alignItems("center"),
		gap(".4rem"),
		height("100%"),
		padding("0 .55rem"),
		border("0"),
		borderInline("1px solid var(--border)"),
		background("transparent"),
		color("inherit"),
		font("inherit"),
		fontSize(".82rem"),
		fontVariantNumeric("tabular-nums"),
		cursor("pointer"),
		whiteSpace("nowrap"),
	)
	rule(".period-pill:hover",
		background("var(--hover, color-mix(in srgb, var(--text) 8%, transparent))"),
	)
	rule(".period-pill[aria-expanded=\"true\"]",
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".period-caret",
		color("var(--text-dim)"),
		transition("transform .15s ease"),
	)
	rule(".period-pill[aria-expanded=\"true\"] .period-caret",
		transform("rotate(180deg)"),
	)
	rule(".period-pop",
		minWidth("17rem"),
		padding(".65rem"),
		display("flex"),
		flexDirection("column"),
		gap(".55rem"),
	)
	rule(".period-pop .seg",
		display("flex"),
		width("100%"),
	)
	rule(".period-pop .seg-btn",
		flex("1"),
		justifyContent("center"),
	)
	rule(".period-presets",
		display("flex"),
		flexWrap("wrap"),
		gap(".35rem"),
	)
	rule(".period-preset",
		padding(".3rem .6rem"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize(".78rem"),
		cursor("pointer"),
		transition("background .12s, color .12s, border-color .12s"),
	)
	rule(".period-preset:hover",
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
		color("var(--text)"),
		borderColor("var(--accent)"),
	)
	rule(".period-rangetoggle",
		alignSelf("flex-start"),
		padding(".15rem .1rem"),
		border("0"),
		background("transparent"),
		color("var(--accent)"),
		fontSize(".78rem"),
		fontWeight("600"),
		cursor("pointer"),
	)
	rule(".period-rangetoggle:hover",
		textDecoration("underline"),
	)
	rule(".period-rangerow",
		paddingTop(".15rem"),
	)
	rule(".topbar .tb-context select.member-switcher",
		height("var(--tb-h)"),
		minHeight("0"),
		padding("0 1.5rem 0 .6rem"),
	)
	rule(".topbar .cf-member-switcher-wrap .btn",
		height("var(--tb-h)"),
		minHeight("0"),
		padding("0 .7rem"),
	)
	rule(".topbar .tb-actions .icon-btn,\n      .topbar .tb-actions .more-btn,\n      .topbar .tb-actions .notify-btn,\n      .topbar .tb-actions .muzak-btn,\n      .topbar .tb-actions .add-btn",
		width("var(--tb-h)"),
		height("var(--tb-h)"),
		borderRadius("8px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".topbar .tb-actions .add-caret",
		width("1.2rem"),
		height("var(--tb-h)"),
		borderRadius("8px"),
	)
	rule(".topbar .tb-actions .icon-btn,\n      .topbar .tb-actions .more-btn,\n      .topbar .tb-actions .notify-btn,\n      .topbar .tb-actions .muzak-btn,\n      .topbar .tb-actions .add-btn,\n      .topbar .tb-actions .add-caret",
		background("transparent !important"),
		border("0 !important"),
	)
	rule(".topbar .tb-actions .icon-btn:hover,\n      .topbar .tb-actions .more-btn:hover,\n      .topbar .tb-actions .notify-btn:hover,\n      .topbar .tb-actions .muzak-btn:hover,\n      .topbar .tb-actions .add-btn:hover,\n      .topbar .tb-actions .add-caret:hover",
		background("color-mix(in srgb, var(--text) 8%, transparent) !important"),
		color("var(--text)"),
	)
	rule(".topbar .tb-actions .add-btn",
		color("var(--accent)"),
	)
	rule(".topbar-secondary",
		display("none !important"),
	)
	rule(".topbar-more",
		display("inline-flex !important"),
	)
	rule(".topbar .tb-context > *, .topbar .tb-actions > *",
		flex("none"),
	)
	ruleMedia("(max-width: 1000px)", ".topbar [data-testid=\"profile-switch-btn\"]",
		display("none !important"),
	)
	ruleMedia("(max-width: 680px)", ".topbar .tb-context .cf-member-switcher-wrap",
		display("none !important"),
	)
	rule(".brand",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		fontWeight("800"),
		whiteSpace("nowrap"),
	)
	rule(".brand-mark",
		display("grid"),
		placeItems("center"),
		width("1.6rem"),
		height("1.6rem"),
		background("var(--accent)"),
		color("#052e13"),
		borderRadius("8px"),
		fontWeight("900"),
	)
	rule(".brand-name",
		fontSize("1.05rem"),
		letterSpacing("-0.01em"),
	)
	rule(".nav",
		display("flex"),
		gap("0.25rem"),
		flexWrap("nowrap"),
	)
	rule(".nav-link",
		appearance("none"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		font("inherit"),
		fontSize("0.9rem"),
		padding("0.4rem 0.7rem"),
		borderRadius("8px"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("background 0.12s, color 0.12s"),
	)
	rule(".nav-link:hover",
		background("var(--bg-elev)"),
		color("var(--text)"),
	)
	rule(".nav-link.active",
		background("var(--accent-dim)"),
		color("var(--accent)"),
		fontWeight("600"),
	)
	rule(".page",
		width("100%"),
		maxWidth("1040px"),
		margin("0 auto"),
		padding("1.75rem 1.25rem 4rem"),
	)
	ruleMedia("(min-width: 1441px)", "main.cf-scroll > *",
		maxWidth("1440px"),
		marginInline("auto"),
	)
	rule(".page-head",
		marginBottom("1.5rem"),
	)
	rule(".page-title",
		margin("0"),
		fontSize("1.6rem"),
		letterSpacing("-0.02em"),
	)
	rule(".page-sub",
		margin("0.25rem 0 0"),
		color("var(--text-dim)"),
	)
	rule(".card",
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		padding("1.25rem"),
		marginBottom("1rem"),
	)
	rule(".card",
		boxShadow("0 1px 1px rgba(0,0,0,0.20), 0 10px 26px -18px rgba(0,0,0,0.55),\n                    inset 0 1px 0 rgba(255,255,255,0.035)"),
	)
	rule("[data-theme=\"light\"] .card",
		boxShadow("0 1px 2px rgba(17,24,39,0.05), 0 12px 28px -20px rgba(17,24,39,0.16),\n                    inset 0 1px 0 rgba(255,255,255,0.7)"),
	)
	rule(".insight-list",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		marginTop("0.5rem"),
	)
	rule(".insight-row",
		display("flex"),
		alignItems("flex-start"),
		gap("0.6rem"),
		margin("0"),
		lineHeight("1.45"),
	)
	rule(".insight-dot",
		flex("none"),
		fontWeight("800"),
		fontSize("1rem"),
		lineHeight("1.35"),
	)
	rule(".section-divider",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		margin("1.6rem 0 0.6rem"),
		fontSize("0.74rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".section-divider::after",
		content("\"\""),
		flex("1"),
		height("1px"),
		background("var(--border)"),
	)
	rule(".section-divider:first-child",
		marginTop("0"),
	)
	rule("[data-theme=\"light\"] .section-divider",
		color("#686870"),
	)
	rule("[data-density=\"compact\"] .card",
		padding("0.7rem 0.8rem"),
		marginBottom("0.6rem"),
	)
	rule("[data-density=\"compact\"] .card-title",
		marginBottom("0.45rem"),
	)
	rule("[data-density=\"compact\"] .row",
		paddingTop("0.3rem"),
		paddingBottom("0.3rem"),
	)
	rule("[data-density=\"compact\"] .field, [data-density=\"compact\"] .btn",
		paddingTop("0.35rem"),
		paddingBottom("0.35rem"),
		minHeight("40px"),
	)
	rule("[data-theme=\"light\"]",
		customProp("--bg", "#f7f6f3"),
		customProp("--bg-base", "#f7f6f3"),
		customProp("--bg-elev", "#efede8"),
		customProp("--bg-card", "#ffffff"),
		customProp("--border", "#e4e2dd"),
		customProp("--text", "#1c1c1e"),
		customProp("--text-dim", "#4e4e55"),
		// Audit contrast raise: the light faint tier steps from ~5.2:1 to ~6.1:1.
		customProp("--text-faint", "#5e5e66"),
		customProp("--muted", "#56565c"),
		customProp("--accent-dim", "#e4f3ea"),
		customProp("--hover", "#e8e6e1"),
		colorScheme("light"),
	)
	rule(":root[data-theme=\"light\"]",
		customProp("--money-positive", "#157a43"),
	)
	rule("[data-theme=\"light\"] .bg-base",
		backgroundColor("#f7f6f3"),
	)
	rule("[data-theme=\"light\"] .bg-tile",
		backgroundColor("#ffffff"),
	)
	rule("[data-theme=\"light\"] .bg-hover, [data-theme=\"light\"] .hover\\:bg-hover:hover",
		backgroundColor("#efede8"),
	)
	rule("[data-theme=\"light\"] .text-fg",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .text-dim",
		color("#56565c"),
	)
	rule("[data-theme=\"light\"] .text-faint",
		color("#686870"),
	)
	rule("[data-theme=\"light\"] .text-warn",
		color("#8a6a16"),
	)
	rule("[data-theme=\"light\"] .cal-head",
		color("#686870"),
	)
	rule("[data-theme=\"light\"] .card-title, [data-theme=\"light\"] .page-title,\n      [data-theme=\"light\"] .row-desc, [data-theme=\"light\"] .stat-value,\n      [data-theme=\"light\"] .budget-amount,\n      [data-theme=\"light\"] .amount:not(.amount-income):not(.amount-expense),\n      [data-theme=\"light\"] .budget .row-desc",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .budget-drill, [data-theme=\"light\"] button.row-desc",
		color("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] .bar",
		background("#efede8 !important"),
		borderColor("#e4e2dd !important"),
	)
	rule("[data-theme=\"light\"] .stat",
		background("#ffffff !important"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .hero-stat-value",
		color("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] .reports-hero",
		background("#ffffff !important"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] svg[id^=\"cf-mmd\"] text, [data-theme=\"light\"] .mermaid text",
		fill("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] .cf-chart text",
		fill("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] .cf-chart .x-axis text,\n      [data-theme=\"light\"] .cf-chart .y-axis text",
		fill("#686870 !important"),
	)
	rule("[data-theme=\"light\"] .budget",
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .muted, [data-theme=\"light\"] .row-meta,\n      [data-theme=\"light\"] .budget-sub,\n      [data-theme=\"light\"] .stat-label, [data-theme=\"light\"] .page-sub,\n      [data-theme=\"light\"] .set-label, [data-theme=\"light\"] .t-caption",
		color("#3c3c43"),
	)
	rule("[data-theme=\"light\"] .border-line, [data-theme=\"light\"] .border-r, [data-theme=\"light\"] .border-b",
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .bg-\\[\\#1c1c1e\\]",
		backgroundColor("#ececec"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .bg-fg",
		backgroundColor("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .text-base",
		color("#f7f6f3"),
	)
	rule("[data-theme=\"light\"] .wh h2, [data-theme=\"light\"] .wh h3",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .wh .wh-title",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .wh .grip",
		color("#6a6a72"),
	)
	rule("[data-theme=\"light\"] .w",
		background("var(--bg-card, #ffffff) !important"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .bento > .w",
		padding("0"),
	)
	rule("[data-theme=\"light\"] .attention-item",
		background("var(--bg-card, #ffffff)"),
		color("#1c1c1e"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .attention-item.is-critical",
		background("color-mix(in srgb, var(--danger, #dc2626) 6%, #ffffff)"),
		borderColor("color-mix(in srgb, var(--danger, #dc2626) 30%, #e4e2dd)"),
	)
	rule("[data-theme=\"light\"] .attention-item.is-warning",
		background("color-mix(in srgb, var(--warn, #d97706) 6%, #ffffff)"),
		borderColor("color-mix(in srgb, var(--warn, #d97706) 30%, #e4e2dd)"),
	)
	rule("[data-theme=\"light\"] .bento, [data-theme=\"light\"] .bento + *, [data-theme=\"light\"] main > div",
		backgroundColor("var(--bg)"),
	)
	rule("[data-theme=\"light\"] .topbar",
		background("rgba(247,246,243,0.92) !important"),
		color("#1c1c1e"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] aside.rail, [data-theme=\"light\"] .rail",
		background("var(--bg-elev) !important"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] main",
		background("var(--bg) !important"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv.active, [data-theme=\"light\"] aside.rail .nv[aria-current=\"page\"]",
		background("var(--accent-dim) !important"),
		color("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv",
		color("#3c3c43"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv:hover",
		background("#efede8 !important"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] aside.rail .rail-section",
		color("#6a6a72"),
	)
	rule("[data-theme=\"light\"] .muzak-btn, [data-theme=\"light\"] .notify-btn, [data-theme=\"light\"] .add-btn",
		background("#ffffff !important"),
		color("#3c3c43"),
		border("1px solid #e4e2dd"),
	)
	rule("[data-theme=\"light\"] .muzak-btn:hover, [data-theme=\"light\"] .notify-btn:hover, [data-theme=\"light\"] .add-btn:hover",
		background("#efede8 !important"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .add-menu, [data-theme=\"light\"] .notify-menu, [data-theme=\"light\"] .muzak-menu",
		background("#ffffff !important"),
		borderColor("#e4e2dd !important"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .add-item:hover, [data-theme=\"light\"] .notify-item:hover",
		background("#efede8 !important"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .empty, [data-theme=\"light\"] .w .empty, [data-theme=\"light\"] .card .empty",
		color("#4b4b52 !important"),
	)
	rule("[data-theme=\"light\"] .nav:hover",
		background("#efede8"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .seg, [data-theme=\"light\"] .rpill",
		background("#efede8"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .seg-btn, [data-theme=\"light\"] .rstep",
		color("#56565c"),
	)
	rule("[data-theme=\"light\"] .seg-btn.active",
		background("transparent"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .seg-pill",
		background("#ffffff"),
		boxShadow("0 1px 2px rgba(0,0,0,0.08)"),
	)
	rule("[data-theme=\"light\"] .gear-inline, [data-theme=\"light\"] .gear-abs, [data-theme=\"light\"] .menu-btn, [data-theme=\"light\"] .set-close",
		color("#6a6a72"),
	)
	rule("[data-theme=\"light\"] .flip-face, [data-theme=\"light\"] .set-face",
		background("#ffffff"),
		borderColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .flip-backdrop",
		background("rgba(239,237,232,0.75)"),
	)
	rule("[data-theme=\"light\"] .toggle-row span",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .set-input, [data-theme=\"light\"] .member-chip, [data-theme=\"light\"] .rate-row .rate-in, [data-theme=\"light\"] .data-btn",
		background("#ffffff"),
		borderColor("#e4e2dd"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .badge-soon",
		background("#e8f2ff"),
		color("#1d4ed8"),
		borderColor("#bfdbfe"),
	)
	rule("[data-theme=\"light\"] .switch",
		background("#d4d2cc"),
	)
	rule("[data-theme=\"light\"] .switch::after",
		background("#ffffff"),
	)
	rule("[data-theme=\"light\"] .set-h h3",
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .set-btn.save",
		background("var(--accent,#2e8b57)"),
		borderColor("transparent"),
		color("#ffffff"),
	)
	rule("[data-theme=\"light\"] .set-btn.save:hover",
		filter("brightness(0.95)"),
	)
	rule("[data-theme=\"light\"] .set-btn.cancel",
		background("#ffffff"),
		borderColor("#e4e2dd"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .set-btn.cancel:hover",
		borderColor("#cfcdc7"),
		color("#000"),
	)
	rule("[data-theme=\"light\"] .set-h",
		borderBottomColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .set-foot",
		borderTopColor("#e4e2dd"),
	)
	rule("[data-theme=\"light\"] .add-btn",
		border("1px solid var(--border)"),
		borderRadius("6px"),
	)
	rule(".set-btn.close",
		background("transparent"),
		border("1px solid #34343a"),
		color("#a6a6ac"),
		fontWeight("500"),
	)
	rule(".set-btn.close:hover",
		color("#f4f4f5"),
		borderColor("#44444c"),
	)
	rule("[data-theme=\"light\"] .set-btn.close",
		background("#ffffff"),
		borderColor("#e4e2dd"),
		color("#1c1c1e"),
	)
	rule("[data-theme=\"light\"] .set-btn.close:hover",
		borderColor("#cfcdc7"),
		color("#000"),
	)
	rule("[data-theme=\"light\"] .set-body",
		scrollbarColor("#c5c3be transparent"),
	)
	rule("[data-theme=\"light\"] .set-body::-webkit-scrollbar-thumb",
		background("#c5c3be"),
		borderColor("#ffffff"),
	)
	rule("[data-theme=\"light\"] #cf-cmd-palette",
		background("rgba(239,237,232,0.72) !important"),
	)
	rule(".cf-dialog-scrim",
		backdropFilter("blur(4px)"),
		webkitBackdropFilter("blur(4px)"),
	)
	rule(".cf-dialog",
		boxShadow("0 12px 40px rgba(0,0,0,.35), 0 0 0 1px var(--border,#2a2a2c)"),
		padding("1.5rem 1.5rem 1.25rem"),
		minHeight("6rem"),
	)
	rule(".flip-wrap",
		maxWidth("min(760px, calc(100vw - 24px))"),
	)
	rule("[data-theme=\"light\"] main.cf-scroll::-webkit-scrollbar-thumb",
		background("#cfcdc7"),
		borderColor("#f7f6f3"),
	)
	rule(".card-title",
		margin("0 0 0.75rem"),
		fontSize("1.05rem"),
		color("var(--text)"),
		fontWeight("600"),
		letterSpacing("-0.01em"),
	)
	rule(".hero-main",
		display("flex"),
		alignItems("baseline"),
		gap("1.5rem"),
		flexWrap("wrap"),
	)
	rule(".hero-flanker-label",
		fontSize("0.75rem"),
		fontWeight("500"),
		textTransform("uppercase"),
		letterSpacing("0.05em"),
		color("var(--text-dim)"),
	)
	rule(".hero-stat",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".hero-stat-label",
		fontSize("0.73rem"),
		fontWeight("500"),
		textTransform("uppercase"),
		letterSpacing("0.05em"),
		color("var(--text-dim)"),
	)
	rule(".hero-stat-value",
		fontSize("1rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".hero-stat-value.pos",
		color("var(--money-positive)"),
	)
	rule(".hero-stat-value.neg",
		color("var(--money-negative)"),
	)
	rule("[data-theme=\"light\"] .hero-stat-value.pos",
		color("var(--money-positive) !important"),
	)
	rule("[data-theme=\"light\"] .hero-stat-value.neg",
		color("var(--money-negative) !important"),
	)
	rule(".hero-stat-sub",
		fontSize("0.72rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		opacity("0.9"),
	)
	rule(".hero-stat-sub.pos",
		color("var(--money-positive)"),
	)
	rule(".hero-stat-sub.neg",
		color("var(--money-negative)"),
	)
	rule(".home-hero",
		position("relative"),
		overflow("hidden"),
		marginBottom("1.25rem"),
		padding("1.6rem 1.8rem 1.5rem"),
		borderRadius("18px"),
		border("1px solid var(--border)"),
		background("radial-gradient(135% 130% at 0% 0%, color-mix(in srgb, var(--accent) 10%, transparent), transparent 52%),\n          radial-gradient(120% 130% at 100% 0%, color-mix(in srgb, var(--up, #54b884) 8%, transparent), transparent 48%),\n          var(--bg-card, #ffffff)"),
		boxShadow("0 1px 2px rgba(0,0,0,.04), 0 18px 42px -26px rgba(0,0,0,.30)"),
	)
	rule(".home-hero::before",
		content("\"\""),
		position("absolute"),
		inset("0 0 auto 0"),
		height("1px"),
		background("linear-gradient(90deg, transparent, color-mix(in srgb, var(--accent) 45%, transparent), transparent)"),
		opacity(".6"),
	)
	rule(".home-hero-top",
		marginBottom("1.15rem"),
	)
	rule(".home-hero-greeting",
		margin("0"),
	)
	rule(".home-hero-date",
		margin("0.2rem 0 0"),
		color("var(--text-faint)"),
		fontSize("0.82rem"),
		letterSpacing("0.01em"),
	)
	rule(".home-hero-main",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		gap("2rem"),
		flexWrap("wrap"),
	)
	rule(".home-hero-nw-block",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		minWidth("0"),
	)
	rule(".home-hero-nw-label",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		textTransform("uppercase"),
		letterSpacing("0.09em"),
		fontSize("0.68rem"),
		fontWeight("600"),
		color("var(--text-faint)"),
	)
	rule(".home-hero-nw-fig",
		lineHeight("1.04"),
		fontSize("3.1rem"),
		fontWeight("800"),
		letterSpacing("-0.025em"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)
	ruleMedia("(max-width: 720px)", ".home-hero-nw-fig",
		fontSize("2.3rem"),
	)
	rule(".home-hero-delta",
		alignSelf("flex-start"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		marginTop("0.2rem"),
		padding("0.18rem 0.6rem"),
		borderRadius("999px"),
		fontSize("0.8rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		background("color-mix(in srgb, currentColor 13%, transparent)"),
	)
	rule(".home-hero-spark",
		flex("1 1 200px"),
		maxWidth("360px"),
		minWidth("150px"),
		alignSelf("stretch"),
		display("flex"),
		alignItems("flex-end"),
		opacity("0.95"),
	)
	rule(".home-hero-spark svg",
		width("100%"),
		height("68px"),
	)
	rule(".home-hero-stats",
		display("flex"),
		flexWrap("wrap"),
		alignItems("stretch"),
		marginTop("1.35rem"),
		borderTop("1px solid var(--border)"),
		paddingTop("1rem"),
	)
	rule(".home-hero-stat",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		paddingRight("1.5rem"),
		marginRight("1.5rem"),
		borderRight("1px solid var(--border)"),
	)
	rule(".home-hero-stat:last-child",
		borderRight("0"),
		marginRight("0"),
		paddingRight("0"),
	)
	rule(".home-hero-stat-label",
		textTransform("uppercase"),
		letterSpacing("0.07em"),
		fontSize("0.66rem"),
		fontWeight("600"),
	)
	rule(".home-hero-stat-value",
		fontSize("1.15rem"),
	)
	rule(".home-hero-actions",
		display("flex"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginTop("1.25rem"),
	)
	rule(".home-hero-welcome-body",
		maxWidth("42rem"),
	)
	rule(".home-hero-quote",
		display("flex"),
		alignItems("center"),
		gap("0.65rem"),
		marginTop("1.25rem"),
		paddingTop("1rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".hero-quote-mark",
		flex("none"),
		color("var(--accent)"),
		fontSize("0.85rem"),
		opacity("0.85"),
	)
	rule(".hero-quote-text",
		flex("1"),
		minWidth("0"),
		fontFamily("var(--font-display, \"Fraunces\", serif)"),
		fontStyle("italic"),
		fontSize("1.02rem"),
		lineHeight("1.4"),
		color("var(--text)"),
	)
	rule(".hero-quote-cite",
		fontStyle("normal"),
		fontFamily("var(--font-ui, \"Inter\", sans-serif)"),
		fontSize("0.82rem"),
		fontWeight("500"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)
	rule(".hero-quote-loading",
		color("var(--text-faint)"),
		animation("quoteShimmer 1.5s ease-in-out infinite"),
	)
	keyframes("quoteShimmer",
		at("0%, 100%",
			opacity("0.5"),
		),
		at("50%",
			opacity("0.95"),
		),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".hero-quote-loading",
		animation("none"),
	)
	rule(".hero-quote-hint",
		flex("1"),
		color("var(--text-faint)"),
		fontSize("0.85rem"),
	)
	rule(".hero-quote-controls",
		flex("none"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		opacity("0.55"),
		transition("opacity 0.15s ease"),
	)
	rule(".home-hero-quote:hover .hero-quote-controls",
		opacity("1"),
	)
	rule(".hero-quote-theme",
		height("1.85rem"),
		border("1px solid var(--border)"),
		borderRadius("7px"),
		background("var(--bg-elev, transparent)"),
		color("var(--text-dim)"),
		fontSize("0.75rem"),
		padding("0 1.3rem 0 0.5rem"),
		cursor("pointer"),
	)
	rule(".hero-quote-refresh",
		width("1.85rem"),
		height("1.85rem"),
		border("1px solid var(--border)"),
		borderRadius("7px"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		fontSize("0.95rem"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".hero-quote-refresh:hover",
		color("var(--text)"),
		background("color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".hero-quote-ctx",
		padding("0.2rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.72rem"),
		fontWeight("600"),
		cursor("pointer"),
		whiteSpace("nowrap"),
		transition("color 0.12s, border-color 0.12s, background 0.12s"),
	)
	rule(".hero-quote-ctx:hover",
		color("var(--text)"),
	)
	rule(".hero-quote-ctx.is-on",
		color("var(--accent)"),
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
	)
	rule(".home-hero-quote--off",
		marginTop("1rem"),
		paddingTop("0.85rem"),
	)
	rule(".hero-quote-enable",
		display("inline-flex"),
		alignItems("center"),
		gap("0.45rem"),
		padding("0.28rem 0.7rem"),
		border("1px dashed var(--border)"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--text-faint)"),
		fontSize("0.78rem"),
		cursor("pointer"),
		transition("color 0.12s, border-color 0.12s, background 0.12s"),
	)
	rule(".hero-quote-enable:hover",
		color("var(--accent)"),
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".hero-quote-enable .hero-quote-mark",
		opacity("1"),
	)
	keyframes("heroRise",
		at("from",
			opacity("0"),
			transform("translateY(10px)"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	rule(".home-hero-top, .home-hero-main, .home-hero-stats, .home-hero-actions",
		animation("heroRise 0.55s cubic-bezier(0.2, 0.7, 0.2, 1) both"),
	)
	rule(".home-hero-main",
		animationDelay("0.06s"),
	)
	rule(".home-hero-stats",
		animationDelay("0.12s"),
	)
	rule(".home-hero-actions",
		animationDelay("0.18s"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".home-hero-top, .home-hero-main, .home-hero-stats, .home-hero-actions",
		animation("none"),
	)
	rule(".card-alert",
		borderLeft("4px solid var(--danger)"),
		background("color-mix(in srgb, var(--danger) 6%, var(--bg-card))"),
	)
	rule(".budget-over-banner",
		padding("0.6rem 0.85rem"),
		borderRadius("8px"),
		margin("0 0 0.6rem"),
		background("color-mix(in srgb, var(--danger) 10%, var(--bg-elev))"),
	)
	rule(".budget-over-icon",
		color("var(--danger)"),
		fontSize("1rem"),
		lineHeight("1"),
	)
	rule(".budget-over-text",
		fontWeight("600"),
		color("var(--text)"),
		fontSize("0.9rem"),
	)
	rule(".muted",
		color("var(--text-dim)"),
		margin("0.25rem 0"),
	)
	rule(".empty",
		color("var(--text-faint)"),
		fontStyle("italic"),
	)
	rule(".empty-cta",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap(".85rem"),
		padding("1.75rem 1rem"),
		textAlign("center"),
	)
	rule(".empty-cta .empty",
		margin("0"),
	)
	rule(".empty-cta .empty-icon",
		color("var(--accent)"),
		padding("0.7rem"),
		borderRadius("999px"),
		boxSizing("content-box"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".cal-grid",
		display("grid"),
		gridTemplateColumns("repeat(7,1fr)"),
		gap("4px"),
		marginTop(".5rem"),
	)
	rule(".cal-head",
		textAlign("center"),
		fontSize("11px"),
		textTransform("uppercase"),
		letterSpacing(".04em"),
		color("var(--text-faint)"),
		padding("2px 0"),
	)
	rule(".cal-cell",
		minHeight("46px"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		padding("4px 5px"),
		position("relative"),
	)
	rule(".cal-cell.out",
		opacity(".35"),
	)
	rule(".cal-cell.today",
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".cal-cell.today .cal-day",
		color("var(--text)"),
		fontWeight("700"),
	)
	rule(".cal-day",
		fontSize("12px"),
		color("var(--text-dim)"),
	)
	rule(".cal-dot",
		position("absolute"),
		bottom("6px"),
		left("6px"),
		width("7px"),
		height("7px"),
		borderRadius("50%"),
		background("var(--danger,#d8716f)"),
	)
	rule(".cal-dot--danger",
		background("var(--danger,#d8716f)"),
	)
	rule(".cal-dot--warn",
		background("var(--color-warn,#d6a23e)"),
	)
	rule(".cal-dot--soon",
		background("var(--accent,#2e8b57)"),
	)
	rule(".cal-dot--count",
		width("auto"),
		minWidth("14px"),
		height("14px"),
		padding("0 3px"),
		bottom("5px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		fontSize("9px"),
		fontWeight("700"),
		color("#fff"),
		lineHeight("1"),
	)
	rule(".bills-layout",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	ruleMedia("(min-width: 1024px)", ".bills-layout",
		flexDirection("row"),
		alignItems("flex-start"),
	)
	ruleMedia("(min-width: 1024px)", ".bills-layout > .card:first-child",
		flex("1 1 auto"),
		minWidth("0"),
	)
	ruleMedia("(min-width: 1024px)", ".bills-layout > .card:last-child",
		flex("0 0 320px"),
	)
	rule(".bill-sub-actions",
		flex("none"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	ruleMedia("(max-width: 760px)", ".bill-sub-actions",
		flexWrap("wrap"),
	)
	rule(".badge",
		display("inline-block"),
		fontSize("0.72rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		padding("0.2rem 0.55rem"),
		borderRadius("999px"),
	)
	rule(".badge-soon",
		background("#1e293b"),
		color("#93c5fd"),
		border("1px solid #334155"),
	)
	rule(".badge-split",
		background("#0f2a2a"),
		color("#5eead4"),
		border("1px solid #134e4a"),
		fontSize("0.68rem"),
		verticalAlign("middle"),
		marginLeft("0.35rem"),
	)
	rule("[data-theme=\"light\"] .badge-split",
		background("#f0fdfa"),
		color("#0d9488"),
		borderColor("#99f6e4"),
	)
	rule(".stat-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(160px, 1fr))"),
		gap("0.75rem"),
		marginBottom("1rem"),
	)
	rule(".stat",
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		padding("1rem 1.1rem"),
	)
	rule(".stat-label",
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		textTransform("uppercase"),
		letterSpacing("0.05em"),
	)
	rule(".stat-label .smart-tip-wrap button.btn",
		padding("0"),
		border("0"),
		minHeight("0"),
		height("1em"),
		lineHeight("1"),
	)
	rule(".stat-label .smart-tip-wrap svg",
		width("0.95em"),
		height("0.95em"),
	)
	rule(".stat-value",
		fontSize("1.5rem"),
		fontWeight("700"),
		marginTop("0.3rem"),
		whiteSpace("nowrap"),
		letterSpacing("-0.015em"),
		// Audit type system: every aligned financial value sets tabular numerals.
		fontVariantNumeric("tabular-nums"),
	)
	rule(".stat-value.is-hero",
		fontSize("2.1rem"),
		fontWeight("800"),
		letterSpacing("-0.02em"),
		minWidth("0"),
		maxWidth("100%"),
	)
	ruleMedia("(max-width: 720px)", ".stat-value.is-hero",
		fontSize("1.8rem"),
	)
	rule(".stat-value.pos",
		color("var(--money-positive)"),
	)
	rule(".stat-value.neg",
		color("var(--money-negative)"),
	)
	rule(".stat-sub",
		display("block"),
		marginTop("0.3rem"),
		fontSize("0.82rem"),
	)
	rule(".nw-summary",
		display("grid"),
		gridTemplateColumns("1.6fr 1fr"),
		gridAutoRows("1fr"),
		gap("0.75rem"),
		marginBottom("1rem"),
	)
	rule(".nw-summary .stat-hero",
		gridRow("1 / 3"),
		display("flex"),
		flexDirection("column"),
		justifyContent("center"),
	)
	rule(".nw-summary .stat-hero .stat-value",
		fontSize("2.1rem"),
	)
	ruleMedia("(max-width: 640px)", ".nw-summary",
		gridTemplateColumns("1fr"),
	)
	ruleMedia("(max-width: 640px)", ".nw-summary .stat-hero",
		gridRow("auto"),
	)
	rule(".btn-stale",
		border("1px solid #c2870b"),
		color("#d98c00"),
	)
	rule(".btn-stale:hover",
		background("rgba(245,158,11,0.12)"),
	)
	rule("[data-theme=\"light\"] .btn-stale",
		color("#92400e"),
		borderColor("#d39200"),
	)
	rule(".acct-type-icon",
		display("inline-flex"),
		alignItems("center"),
		flex("none"),
	)
	rule(".rows",
		display("flex"),
		flexDirection("column"),
	)
	rule(".row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		padding("0.6rem 0"),
		borderTop("1px solid var(--border)"),
		transition("background 0.12s ease"),
	)
	rule(".row:hover",
		background("var(--bg-elev)"),
	)
	rule("[data-theme=\"light\"] .row:hover",
		background("#efede8"),
	)
	rule(".row:first-child",
		borderTop("0"),
	)
	ruleMedia("(max-width: 760px)", ".row",
		flexWrap("wrap"),
		rowGap("0.4rem"),
	)
	ruleMedia("(max-width: 760px)", ".row .row-main",
		flex("1 1 100%"),
	)
	ruleMedia("(max-width: 760px)", ".row .acct-type-icon",
		order("-1"),
	)
	rule(".row-edit",
		padding("0.6rem 0"),
		borderTop("1px solid var(--border)"),
	)
	rule(".row-edit:first-child",
		borderTop("0"),
	)
	rule("[data-density=\"compact\"] .row-edit",
		paddingTop("0.3rem"),
		paddingBottom("0.3rem"),
	)
	rule(".row-main",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("0"),
		flex("1"),
	)
	rule(".row-desc",
		fontWeight("500"),
		color("var(--text)"),
		overflowWrap("anywhere"),
	)
	rule(".todo-summary",
		margin("0 0 0.6rem"),
		fontSize("0.86rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".row-meta",
		color("var(--text-dim)"),
		fontSize("0.82rem"),
	)
	rule(".txn-table",
		width("100%"),
		borderCollapse("collapse"),
		fontSize("0.86rem"),
	)
	rule(".txn-table thead th",
		textAlign("left"),
		padding("0.45rem 0.6rem"),
		borderBottom("1px solid var(--border)"),
		color("var(--text-dim)"),
		fontWeight("600"),
		whiteSpace("nowrap"),
		position("sticky"),
		top("0"),
		background("var(--bg)"),
		zIndex("1"),
	)
	rule(".data-table.dt-sticky thead th",
		position("sticky"),
		top("var(--dt-sticky-top, 0px)"),
		zIndex("5"),
		background("var(--bg-card)"),
		boxShadow("0 1px 0 var(--border), 0 6px 10px -8px rgba(0,0,0,0.45)"),
	)
	rule(".dt-spin",
		display("inline-block"),
		borderRadius("50%"),
		border("2px solid var(--border)"),
		borderTopColor("var(--accent)"),
		animation("dt-spin 0.65s linear infinite"),
	)
	rule(".dt-spin-sm",
		width("13px"),
		height("13px"),
		verticalAlign("middle"),
		marginLeft("0.1rem"),
	)
	keyframes("dt-spin",
		at("to",
			transform("rotate(360deg)"),
		),
	)
	rule(".txn-table .th-sort",
		background("transparent"),
		border("0"),
		color("inherit"),
		font("inherit"),
		fontWeight("600"),
		cursor("pointer"),
		padding("0.2rem 0"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.15rem"),
		minHeight("24px"),
	)
	rule(".txn-table .th-sort:hover",
		color("var(--text)"),
	)
	rule(".txn-table th[aria-sort=\"ascending\"] .th-sort, .txn-table th[aria-sort=\"descending\"] .th-sort",
		color("var(--accent)"),
	)
	rule(".txn-table tr",
		display("table-row"),
	)
	rule(".txn-table tbody tr.row",
		minHeight("44px"),
	)
	rule(".txn-table tbody td",
		padding("0.45rem 0.6rem"),
		borderBottom("1px solid var(--border)"),
		verticalAlign("middle"),
	)
	rule(".txn-table tbody tr.selected",
		background("var(--accent-dim)"),
	)
	rule(".txn-table tbody tr.row:nth-child(even)",
		background("rgba(255,255,255,0.055)"),
	)
	rule("[data-theme=\"light\"] .txn-table tbody tr.row:nth-child(even)",
		background("rgba(0,0,0,0.04)"),
	)
	rule("[data-theme=\"light\"] .txn-table thead th",
		background("#f7f6f3 !important"),
		color("#3c3c43 !important"),
	)
	rule("[data-theme=\"light\"] .txn-table tbody td, [data-theme=\"light\"] .txn-table tbody tr.row",
		color("#1c1c1e !important"),
	)
	rule("[data-theme=\"light\"] .txn-table tbody tr.row:hover",
		background("#efede8 !important"),
	)
	rule(".txn-table tbody tr.row:hover",
		background("var(--bg-elev)"),
	)
	rule(".txn-table .td-date",
		whiteSpace("nowrap"),
		color("var(--text-dim)"),
	)
	rule(".txn-table .td-cat, .txn-table .td-acct, .txn-table .td-tags",
		color("var(--text-dim)"),
	)
	rule(".txn-table .td-amount",
		textAlign("right"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
	)
	rule(".txn-table td.row-desc",
		maxWidth("280px"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".txn-table .td-tags-inline",
		color("var(--text-faint)"),
		fontSize("0.8rem"),
	)
	rule(".txn-table .td-actions .btn-icon",
		padding("0.2rem 0.35rem"),
		minHeight("0"),
	)
	rule(".txn-table .tx-2nd",
		transition("opacity 0.12s ease"),
	)
	rule(".txn-table tbody tr:not(:hover):not(:focus-within) .tx-2nd",
		opacity("0"),
		pointerEvents("none"),
	)
	ruleMedia("(pointer: coarse)", ".txn-table .tx-2nd",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".txn-table .tx-2nd:focus-visible",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".row .row-2nd",
		transition("opacity 0.12s ease"),
	)
	rule(".row:not(:hover):not(:focus-within) .row-2nd",
		opacity("0"),
		pointerEvents("none"),
	)
	ruleMedia("(pointer: coarse)", ".row .row-2nd",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".row .row-2nd:focus-visible",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".txn-table .td-actions",
		whiteSpace("nowrap"),
		textAlign("right"),
		width("96px"),
		maxWidth("96px"),
	)
	rule(".txn-table .clr-toggle",
		background("transparent"),
		border("0"),
		cursor("pointer"),
		fontSize("1.05rem"),
		lineHeight("1"),
		color("var(--text-faint)"),
		width("1.6rem"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		minHeight("1.5rem"),
	)
	rule(".txn-table .clr-toggle.is-cleared",
		color("var(--accent)"),
		fontWeight("700"),
	)
	rule(".txn-table tbody tr.row.cleared .td-date, .txn-table tbody tr.row.cleared .row-desc",
		color("var(--text-dim)"),
	)
	rule(".txn-table .td-select .check",
		background("transparent"),
		border("0"),
		cursor("pointer"),
		fontSize("1rem"),
		lineHeight("1"),
	)
	rule(".txn-table .row-edit td",
		padding("0.5rem 0.3rem"),
	)
	rule(".txn-table thead th.td-amount, .txn-table thead th.td-actions",
		textAlign("right"),
	)
	rule(".data-pager",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		padding("0.6rem 0.2rem 0.1rem"),
		fontSize("0.86rem"),
	)
	rule(".data-pager.data-pager-top",
		padding("0.1rem 0.2rem 0.55rem"),
		borderBottom("1px solid var(--border)"),
		marginBottom("0.45rem"),
	)
	rule(".data-pager .data-pos",
		fontVariantNumeric("tabular-nums"),
	)
	rule(".data-pager .data-pager-label",
		marginLeft("auto"),
	)
	rule(".data-pager .field",
		width("auto"),
		minWidth("4.5rem"),
	)
	rule(".data-pager .btn[disabled]",
		opacity("0.45"),
		cursor("default"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table, .txn-table tbody, .txn-table tr, .txn-table td",
		display("block"),
		width("100%"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table thead",
		display("none"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table tbody tr.row",
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		marginBottom("0.5rem"),
		padding("0.3rem 0.5rem"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table tbody td",
		border("0"),
		padding("0.15rem 0.2rem"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table .td-amount",
		textAlign("left"),
	)
	ruleMedia("(max-width: 1200px)", ".txn-table td.row-desc",
		maxWidth("none"),
		whiteSpace("normal"),
	)
	rule(".amount",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		whiteSpace("nowrap"),
	)
	rule(".amount-income",
		color("var(--money-positive)"),
	)
	rule(".amount-expense",
		color("var(--money-negative)"),
	)
	// Two-row standard toolbar: primary row (search fills + Filters trigger), then an
	// actions row beneath. A column so the two rows stack; each row is its own flex line.
	rule(".filter-toolbar",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		marginBottom("0.6rem"),
	)
	rule(".filter-toolbar-primary",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	// The search pill grows to fill the WHOLE line; the Filters trigger holds its natural
	// size. The base .fctrl-search caps at max-width:22rem (to avoid premature wrap in the
	// old single-row toolbar) — here it's on its own row, so lift the cap and let it fill.
	rule(".filter-toolbar-primary .fctrl-search, .filter-toolbar-primary .todo-ctrl-search",
		flex("1 1 auto"),
		maxWidth("none"),
		minWidth("0"),
	)
	rule(".filter-toolbar-primary .filters-trigger",
		flexShrink("0"),
	)
	rule(".filter-toolbar-actions",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".filter-search",
		flex("1 1 220px"),
		minWidth("160px"),
	)
	rule(".filters-trigger",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".filter-badge",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		minWidth("1.25rem"),
		height("1.25rem"),
		padding("0 0.35rem"),
		borderRadius("999px"),
		background("var(--accent)"),
		color("#fff"),
		fontSize("0.72rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".filter-chips",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
		marginBottom("0.6rem"),
	)
	rule(".filter-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		padding("0.2rem 0.3rem 0.2rem 0.6rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		fontSize("0.82rem"),
	)
	rule(".filter-chip .chip-text",
		lineHeight("1.2"),
	)
	rule(".chip-x",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.1rem"),
		height("1.1rem"),
		padding("0"),
		border("none"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--muted)"),
		fontSize("0.8rem"),
		lineHeight("1"),
		cursor("pointer"),
	)
	rule(".chip-x:hover",
		background("var(--action-danger)"),
		color("#fff"),
	)
	rule(".chip-clear-all",
		background("none"),
		border("none"),
		color("var(--accent)"),
		font("inherit"),
		cursor("pointer"),
		padding("0.2rem 0.3rem"),
	)
	rule(".chip-clear-all:hover",
		textDecoration("underline"),
	)
	// Accounts + transactions filter panels: each labeled field is a "control pill" (the
	// same language as the /todo + /goals + /budgets toolbars) — a bordered capsule with a
	// small uppercase label tag and a borderless control, flowing in a wrap row.
	rule(".filter-fields",
		display("flex"),
		flexDirection("row"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".filter-fields .field-label",
		display("flex"),
		flexDirection("row"),
		alignItems("center"),
		gap("0.45rem"),
		minHeight("38px"),
		padding("0.3rem 0.6rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("9px"),
		fontSize("0.72rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)
	rule(".filter-fields .field-label .field, .filter-fields .field-label select",
		width("auto"),
		minWidth("0"),
		minHeight("0"),
		padding("0.1rem 0.2rem"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		fontSize("0.86rem"),
		fontWeight("500"),
		prop("text-transform", "none"),
		letterSpacing("normal"),
	)
	rule(".filter-fields .field-label .field:focus, .filter-fields .field-label select:focus",
		prop("outline", "none"),
		boxShadow("none"),
	)
	// Toggle buttons that sit inside a filter pill (archived / formulas) read as the pill's
	// value — strip their own chrome so they don't look like a button-in-a-button.
	rule(".filter-fields .field-label .btn",
		minHeight("0"),
		padding("0.05rem 0.2rem"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		fontSize("0.86rem"),
		fontWeight("500"),
		prop("text-transform", "none"),
		letterSpacing("normal"),
	)
	rule(".filter-inline-panel",
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		marginBottom("0.8rem"),
		overflow("hidden"),
	)
	rule(".filter-inline-header",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		padding("0.55rem 0.9rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".filter-inline-title",
		margin("0"),
		fontSize("0.88rem"),
		fontWeight("600"),
	)
	rule(".filter-inline-body",
		padding("0.9rem"),
		display("flex"),
		flexWrap("wrap"),
		gap("0.7rem"),
	)
	rule(".filter-inline-body .filter-fields",
		flex("1 1 auto"),
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(180px, 1fr))"),
		gap("0.7rem"),
	)
	// Inside the filter flyout, keep the same inline control-pill look as the base
	// .filter-fields rule (this selector is more specific, so it must re-assert row layout).
	rule(".filter-inline-body .filter-fields .field-label",
		display("flex"),
		flexDirection("row"),
		alignItems("center"),
		gap("0.45rem"),
	)
	rule(".filter-inline-body .filter-fields .field-label .field",
		color("var(--text)"),
	)
	rule(".form-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(150px, 1fr))"),
		gap("0.6rem"),
		alignItems("end"),
	)
	// The Edit-transaction flip modal: the labeled fields pair up in two calm columns,
	// but the checkbox / attach / actions / error rows span the full width (they were
	// getting squeezed into single grid columns and misaligning). Top-aligned so the
	// paired fields line up by their labels. Scoped to .txn-edit so other .form-grid
	// forms are untouched.
	rule(".txn-edit",
		gridTemplateColumns("repeat(auto-fit, minmax(190px, 1fr))"),
		alignItems("start"),
		gap("0.7rem 0.9rem"),
	)
	rule(".txn-edit > :not(.labeled-field)",
		gridColumn("1 / -1"),
	)
	rule(".txn-edit .txn-check",
		display("flex"),
		flexDirection("row"),
		alignItems("center"),
		gap("0.5rem"),
		cursor("pointer"),
	)
	rule(".txn-edit .form-actions",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginTop("0.3rem"),
	)
	// Add-budget modal: a flex-column shell so the fields sit at the top and the action
	// bar pins to the bottom (breathing room above the CTA, never dead space below it).
	rule(".budget-add-shell",
		display("flex"),
		flexDirection("column"),
		gap("0.9rem"),
		height("100%"),
		minHeight("0"),
	)
	// Two-column field grid (wider cells than the default 150px min, so selects don't
	// truncate and the form reads calmly); identity fields + rollover span the full width.
	rule(".budget-add-grid",
		gridTemplateColumns("repeat(auto-fit, minmax(224px, 1fr))"),
		gap("0.7rem 0.9rem"),
		alignItems("end"),
	)
	rule(".budget-add-grid .ba-full",
		prop("grid-column", "1 / -1"),
	)
	// Rollover: a calm full-width toggle line, not a boxed field.
	rule(".budget-add-grid .ba-check",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		flexWrap("nowrap"),
		padding("0.35rem 0.1rem"),
		fontSize("0.9rem"),
		color("var(--text)"),
		prop("cursor", "pointer"),
	)
	// Action bar pinned to the modal's bottom: a quiet Cancel + a prominent primary. Sticky
	// so it stays visible (with a solid backing) even if a field-heavy form has to scroll;
	// margin-top:auto pins it to the bottom when the form is short.
	// Pinned custom footer button sizing (shared by every .modal-foot action bar), so
	// the primary "Add"/"Save" button reads as a comfortable, prominent target.
	rule(".modal-foot .ba-submit",
		minWidth("150px"),
	)
	// Unify semi-custom footer buttons (.modal-foot .btn / .btn-primary / .btn-del) with
	// the standard footer (.set-btn) so every modal's Cancel/Save/Delete look identical —
	// same size, the muted-green primary, the ghost secondary, the danger delete.
	// A pinned footer (alias of .modal-foot) for forms converted via FlushBody + a
	// .modal-scroll field region: a flex-shrink:0 bar so it never scrolls with the fields.
	// (position:sticky can't pin a last-child footer — its containing block has no room
	// below it — so the reliable pin is this flex sibling of the scroll region.)
	rule(".modal-sticky-foot",
		flexShrink("0"),
		marginTop("auto"),
		display("flex"),
		justifyContent("flex-end"),
		alignItems("center"),
		gap("0.5rem"),
		padding("0.75rem 1rem"),
		borderTop("1px solid #2a2a2c"),
		background("#121214"),
	)
	// A left-pushed Delete (destructive) so it sits apart from Cancel/Save on the right.
	rule(".modal-sticky-foot .btn-del, .modal-foot .btn-del",
		marginRight("auto"),
	)
	// Unify footer buttons across the standard (.set-btn), flex (.modal-foot) and sticky
	// (.modal-sticky-foot) footers so every modal's Cancel/Save/Delete look identical.
	rule(".modal-foot .btn, .modal-foot .btn-primary, .modal-foot .btn-del, .modal-sticky-foot .btn, .modal-sticky-foot .btn-primary, .modal-sticky-foot .btn-del",
		minWidth("96px"),
		minHeight("44px"),
		padding("var(--btn-py,0.5rem) 1rem"),
		borderRadius("4px"),
		fontSize("0.9rem"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".modal-foot .btn, .modal-sticky-foot .btn",
		background("transparent"),
		border("1px solid #34343a"),
		color("#a6a6ac"),
		fontWeight("500"),
	)
	rule(".modal-foot .btn:hover, .modal-sticky-foot .btn:hover",
		color("#f4f4f5"),
		borderColor("#44444c"),
	)
	rule(".modal-foot .btn-primary, .modal-sticky-foot .btn-primary",
		background("#1f2c24"),
		border("1px solid #356b50"),
		color("#7fd0a3"),
		fontWeight("600"),
	)
	rule(".modal-foot .btn-primary:hover, .modal-sticky-foot .btn-primary:hover",
		background("#26382d"),
	)
	rule(".modal-foot .btn-del, .modal-sticky-foot .btn-del",
		background("transparent"),
		border("1px solid #6b3535"),
		color("#d08a8a"),
		fontWeight("500"),
	)
	rule(".modal-foot .btn-del:hover, .modal-sticky-foot .btn-del:hover",
		background("#2c1f1f"),
		color("#f4d0d0"),
	)
	rule(".field",
		width("100%"),
		padding("0.5rem 0.6rem"),
		minHeight("44px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		color("var(--text)"),
		font("inherit"),
	)
	rule(".field",
		transition("border-color .12s ease, box-shadow .12s ease"),
	)
	rule(".field:focus",
		borderColor("var(--accent)"),
		boxShadow("0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent)"),
	)
	rule("select:not(.set-input):not(.seg-btn)",
		minHeight("44px"),
		padding("0.5rem 0.6rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		color("var(--text)"),
		font("inherit"),
		cursor("pointer"),
		transition("border-color .12s ease, box-shadow .12s ease"),
	)
	// Theme the native dropdown option list (Chromium honours these) so an opened
	// <select> — including the transparent to-do Sort/Show filter pills — reads on the
	// dark theme instead of falling back to unstyled white-on-white. Uses theme tokens so
	// it flips correctly on the light theme too.
	rule("select option",
		background("var(--bg-elev)"),
		color("var(--text)"),
	)
	rule("select optgroup",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	rule("select:not(.set-input):not(.seg-btn):focus",
		borderColor("var(--accent)"),
		boxShadow("0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent)"),
	)
	rule("select:not(.set-input):not(.seg-btn):focus-visible,\n      .field:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("1px"),
	)
	rule(".color-input",
		width("44px"),
		height("44px"),
		padding("3px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		cursor("pointer"),
	)
	rule(".color-input::-webkit-color-swatch-wrapper",
		padding("0"),
	)
	rule(".color-input::-webkit-color-swatch",
		border("none"),
		borderRadius("5px"),
	)
	rule(".cat-swatch",
		width("11px"),
		height("11px"),
		borderRadius("3px"),
		flexShrink("0"),
		display("inline-block"),
		alignSelf("center"),
	)
	rule(".btn-link",
		background("none"),
		border("none"),
		padding("0"),
		font("inherit"),
		color("var(--accent)"),
		textDecoration("underline"),
		cursor("pointer"),
	)
	rule(".btn-link:hover",
		opacity("0.8"),
	)
	rule(".cat-child-row",
		background("rgba(255,255,255,0.02)"),
	)
	rule("[data-theme=\"light\"] .cat-child-row",
		background("rgba(0,0,0,0.02)"),
	)
	rule(".cat-zero-usage",
		opacity("0.55 !important"),
	)
	rule(".cat-map",
		display("flex"),
		flexWrap("wrap"),
		gap("0.6rem"),
	)
	rule(".cat-map-group",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
		padding("0.45rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		background("var(--bg-elev)"),
	)
	rule(".cat-map-chip",
		fontSize("0.88rem"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".cat-map-sub",
		fontSize("0.78rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
		padding("0.1rem 0.5rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--text) 6%, transparent)"),
		border("1px solid color-mix(in srgb, var(--text) 14%, transparent)"),
	)
	rule(".cat-map-sub2",
		opacity("0.8"),
	)
	rule(":root",
		customProp("--btn-py", "0.5rem"),
		customProp("--btn-px", "0.8rem"),
	)
	rule(".btn",
		padding("var(--btn-py) var(--btn-px)"),
		minHeight("44px"),
		borderRadius("6px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		font("inherit"),
		fontWeight("500"),
		cursor("pointer"),
		transition("filter 0.12s"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".btn:hover",
		filter("brightness(1.12)"),
	)
	rule(".btn:disabled, .btn[aria-disabled=\"true\"]",
		opacity("0.5"),
		cursor("not-allowed"),
		filter("none"),
	)
	// .btn-tool is the standard toolbar-action-button treatment (paired with .btn): the same
	// 38px height as the pill controls next to it, a tidy gap, and a SINGLE leading glyph
	// rendered slightly grayed (like the select-tag icons) so the label leads. Applied to
	// every page's toolbar action buttons for one consistent control language.
	rule(".btn-tool",
		minHeight("38px"),
		padding("0.35rem 0.75rem"),
		gap("0.4rem"),
		borderRadius("8px"),
	)
	rule(".btn-tool svg",
		opacity("0.6"),
	)
	// Danger-tinted toolbar button (e.g. Delete selected): red text + border, still the
	// same .btn-tool footprint so it lines up with its neighbours.
	rule(".btn-tool.bt-danger",
		color("var(--danger)"),
		borderColor("color-mix(in srgb, var(--danger) 45%, var(--border))"),
	)
	rule(".btn-tool.bt-danger:hover",
		background("color-mix(in srgb, var(--danger) 10%, var(--bg-elev))"),
	)
	// Stale/amber-tinted toolbar button (e.g. Mark all updated when balances are stale).
	rule(".btn-tool.bt-stale",
		color("var(--warn, #d97706)"),
		borderColor("color-mix(in srgb, var(--warn, #d97706) 45%, var(--border))"),
	)
	rule(".btn-tool.bt-stale:hover",
		background("color-mix(in srgb, var(--warn, #d97706) 12%, var(--bg-elev))"),
	)
	// A modal-opening toolbar button stays highlighted while its flip modal is showing.
	rule(".btn-tool.is-open",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		color("var(--text)"),
	)
	// .bt-kind is the small muted trailing badge on a toolbar button that signals its
	// behaviour (⧉ opens a dialog, ↗ navigates) without a hover.
	rule(".bt-kind",
		marginLeft("0.1rem"),
		fontSize("0.85em"),
		opacity("0.45"),
		color("var(--text-dim)"),
	)
	rule(".btn-primary",
		background("linear-gradient(180deg, color-mix(in srgb, var(--accent) 90%, #000 10%), color-mix(in srgb, var(--accent) 78%, #000 22%))"),
		color("#ffffff"),
		borderColor("color-mix(in srgb, var(--accent) 78%, #000 22%)"),
		fontWeight("600"),
		boxShadow("0 1px 2px rgba(0,0,0,0.28), inset 0 1px 0 rgba(255,255,255,0.16)"),
	)
	rule(".btn-del",
		background("transparent"),
		border("0"),
		color("var(--text-faint)"),
		cursor("pointer"),
		fontSize("1rem"),
		padding("0.25rem 0.4rem"),
	)
	rule(".btn-del:hover",
		color("var(--danger)"),
	)
	rule(".btn-del, .toast-x, .rstep, .set-close",
		minWidth("24px"),
		minHeight("24px"),
		display("inline-grid"),
		placeItems("center"),
	)
	rule(".btn-del",
		minWidth("32px"),
		minHeight("32px"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", ".btn:active, .btn-primary:active, .nav-link:active, .data-btn:active,\n        .seg-btn:active, .menu-btn:active, .check:active, .btn-del:active,\n        .member-add:active, .member-chip:active, .set-btn:active, .rstep:active,\n        .toast-x:active, .nav:active, .nv:active",
		transform("translateY(1px)"),
	)
	rule(".err",
		color("#fca5a5"),
		margin("0.6rem 0 0"),
		fontSize("0.88rem"),
	)
	rule(".sr-only",
		position("absolute"),
		width("1px"),
		height("1px"),
		padding("0"),
		margin("-1px"),
		overflow("hidden"),
		clip("rect(0, 0, 0, 0)"),
		whiteSpace("nowrap"),
		border("0"),
	)
	rule(".toast",
		position("fixed"),
		left("50%"),
		bottom("1.25rem"),
		transform("translateX(-50%)"),
		zIndex("var(--z-toast)"),
		maxWidth("min(92vw, 32rem)"),
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		padding("0.7rem 0.9rem"),
		borderRadius("10px"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		border("1px solid var(--border)"),
		boxShadow("0 8px 28px rgba(0,0,0,0.35)"),
		fontSize("0.9rem"),
		animation("toast-in 160ms ease-out"),
	)
	rule(".toast-err",
		borderColor("var(--danger)"),
	)
	rule(".toast-err .toast-msg",
		color("var(--danger)"),
	)
	rule("[data-theme=\"light\"] .toast",
		background("#ffffff"),
		borderColor("#d1cfc9"),
		boxShadow("0 6px 20px rgba(0,0,0,0.16)"),
	)
	rule(".toast::before",
		content("\"\\2713\""),
		color("var(--accent)"),
		fontWeight("700"),
		fontSize("1rem"),
		lineHeight("1"),
	)
	rule(".toast-err::before",
		content("\"\\26A0\""),
		color("var(--danger)"),
	)
	rule("[data-theme=\"light\"] section.card",
		background("#ffffff !important"),
		border("1px solid #e4e2dd !important"),
	)
	rule(".toast-msg",
		flex("1"),
	)
	rule(".toast-x",
		flex("none"),
		border("0"),
		background("transparent"),
		color("inherit"),
		cursor("pointer"),
		fontSize("1.2rem"),
		lineHeight("1"),
		opacity("0.7"),
		padding("0 0.15rem"),
	)
	rule(".toast-x:hover",
		opacity("1"),
	)
	keyframes("toast-in",
		at("from",
			opacity("0"),
			transform("translate(-50%, 0.5rem)"),
		),
		at("to",
			opacity("1"),
			transform("translate(-50%, 0)"),
		),
	)
	rule(".budget",
		padding("0.7rem 0"),
		borderTop("1px solid var(--border)"),
	)
	rule(".budget:first-child",
		borderTop("0"),
	)
	rule(".budget-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.6rem"),
	)
	rule(".budget-amount",
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
		whiteSpace("nowrap"),
	)
	rule(".budget-sub",
		display("block"),
		color("var(--text-dim)"),
		fontSize("0.82rem"),
		marginTop("0.15rem"),
	)
	// Zero-based budgeting: the "To Assign" hero, its four-figure breakdown, the
	// income-basis control, and the Savings & investments section.
	rule(".zbb-hero",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		padding("0.9rem 0 0.4rem"),
	)
	rule(".zbb-label",
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-figrow",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.2rem 0.7rem"),
	)
	rule(".zbb-figure",
		fontSize("2.6rem"),
		fontWeight("800"),
		lineHeight("1.03"),
		letterSpacing("-0.01em"),
		color("var(--text)"),
	)
	rule(".zbb-figure.is-done", color("var(--money-positive)"))
	rule(".zbb-figure.is-left", color("var(--accent)"))
	rule(".zbb-figure.is-over", color("var(--money-negative)"))
	rule(".zbb-status",
		fontSize("0.9rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".zbb-status-over", color("var(--money-negative)"))
	rule(".zbb-breakdown",
		display("flex"),
		flexWrap("wrap"),
		gap("0.3rem 1.4rem"),
		marginTop("0.5rem"),
		paddingTop("0.5rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".zbb-chip",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".zbb-chip-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-chip-val",
		fontSize("1rem"),
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".zbb-basis",
		display("flex"),
		flexWrap("wrap"),
		alignItems("flex-end"),
		gap("0.6rem 0.9rem"),
		marginTop("0.6rem"),
	)
	rule(".zbb-basis-main, .zbb-basis-extra",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".zbb-basis-label, .zbb-basis-sub",
		fontSize("0.72rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".zbb-basis .field",
		minWidth("11rem"),
	)
	rule(".zbb-basis-wrap",
		display("flex"),
		flexDirection("column"),
		gap("0.55rem"),
	)
	rule(".zbb-rollover",
		display("flex"),
		alignItems("flex-start"),
		gap("0.5rem"),
	)
	rule(".zbb-savings-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.6rem"),
	)
	rule(".zbb-savings-title",
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".zbb-savings-total",
		fontSize("1.15rem"),
		fontWeight("800"),
		color("var(--money-positive)"),
	)
	rule(".zbb-savings-rows",
		display("flex"),
		flexDirection("column"),
		marginTop("0.5rem"),
	)
	rule(".zbb-savings-row",
		display("flex"),
		flexDirection("column"),
		alignItems("stretch"),
		gap("0.35rem"),
		padding("0.6rem 0"),
		borderTop("1px solid var(--border)"),
	)
	rule(".zbb-savings-row:first-child", borderTop("0"))
	rule(".zbb-savings-main",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.8rem"),
	)
	rule(".zbb-savings-id",
		display("flex"),
		flexDirection("column"),
		gap("0.05rem"),
		minWidth("0"),
	)
	rule(".zbb-savings-name",
		fontWeight("600"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".zbb-savings-type",
		fontSize("0.72rem"),
		color("var(--text-faint)"),
	)
	rule(".zbb-savings-edit",
		display("flex"),
		alignItems("center"),
		gap("0.25rem"),
		prop("flex-shrink", "0"),
	)
	rule(".zbb-savings-input",
		width("7rem"),
		textAlign("right"),
	)
	rule(".zbb-savings-per",
		fontSize("0.75rem"),
		color("var(--text-faint)"),
	)
	// Plan-vs-reality sub-line: an indented, accent-bordered strip under the account
	// row showing how this monthly rate lands against the goal's timeline. The left
	// border and the time figure take on a positive/negative tone from the delta.
	rule(".zbb-savings-goal",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginLeft("0.1rem"),
		paddingLeft("0.6rem"),
		borderLeft("2px solid var(--border)"),
		fontSize("0.75rem"),
	)
	rule(".zbb-savings-goal.is-ontrack", borderLeftColor("var(--money-positive)"))
	rule(".zbb-savings-goal.is-ahead", borderLeftColor("var(--money-positive)"))
	rule(".zbb-savings-goal.is-behind", borderLeftColor("var(--money-negative)"))
	rule(".zbb-savings-goal-name",
		fontWeight("500"),
		color("var(--text-dim)"),
	)
	rule(".zbb-savings-goal-time", color("var(--text-dim)"))
	rule(".zbb-savings-more",
		fontSize("0.7rem"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)
	rule(".zbb-savings-goal.is-ontrack .zbb-savings-goal-time",
		color("var(--money-positive)"),
		fontWeight("600"),
	)
	rule(".zbb-savings-goal.is-ahead .zbb-savings-goal-time",
		color("var(--money-positive)"),
		fontWeight("600"),
	)
	rule(".zbb-savings-goal.is-behind .zbb-savings-goal-time",
		color("var(--money-negative)"),
		fontWeight("600"),
	)
	rule(".zbb-savings-sync",
		marginLeft("auto"),
		padding("0.15rem 0.5rem"),
		fontSize("0.72rem"),
	)
	rule(".zbb-savings-synced",
		marginLeft("auto"),
		fontSize("0.72rem"),
		color("var(--money-positive)"),
		fontWeight("600"),
	)
	rule(".zbb-savings-foot",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.6rem"),
		marginTop("0.7rem"),
	)
	rule(".zbb-savings-foot-links",
		display("flex"),
		gap("0.4rem"),
		flexWrap("wrap"),
	)
	rule(".zbb-savings-spread",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		background("var(--accent)"),
		color("#fff"),
		borderColor("transparent"),
		fontWeight("600"),
	)
	// The spend-progress bar is DEMOTED in the zero-based view (spending is context,
	// not the headline), so its figures shrink and sit under a quiet caption below
	// the To-Assign hero.
	rule(".zbb-spend",
		marginTop("1rem"),
	)
	rule(".zbb-spend-cap",
		margin("0 0 0.4rem"),
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-spend .budget-loader-value",
		fontSize("0.95rem"),
	)
	rule(".zbb-spend .budget-loader-value.is-hero",
		fontSize("1.15rem"),
	)
	rule(".plan-compare-select--compact",
		maxWidth("220px"),
		fontSize("0.85rem"),
	)
	rule(".alloc-list-header",
		fontSize("0.74rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		textTransform("uppercase"),
		letterSpacing("0.04em"),
		padding("0.15rem 0 0.35rem"),
	)
	rule(".alloc-algo-summary",
		marginTop("0.4rem"),
	)
	rule(".alloc-apply-hint",
		color("var(--text-dim)"),
		fontStyle("italic"),
	)
	rule(".split-summary",
		fontWeight("600"),
		color("var(--text)"),
		marginTop("0.4rem"),
	)
	rule(".cadence-badge",
		display("inline-block"),
		fontSize("0.72rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		background("var(--bg-elev, rgba(255,255,255,0.04))"),
		padding("0.05rem 0.35rem"),
		borderRadius("6px"),
		margin("0 0.3rem"),
		verticalAlign("middle"),
	)
	rule(".subs-select-all-bar",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginBottom("0.5rem"),
	)
	rule(".artifact-thumb-wrap",
		position("relative"),
		width("2.5rem"),
		height("2.5rem"),
		flexShrink("0"),
	)
	rule(".artifact-thumb-fallback",
		display("none"),
		alignItems("center"),
		justifyContent("center"),
		width("2.5rem"),
		height("2.5rem"),
		background("var(--bg-elev, rgba(255,255,255,0.04))"),
		borderRadius("4px"),
		color("var(--text-dim)"),
	)
	rule(".artifact-list-footer",
		borderTop("1px solid var(--border)"),
		paddingTop("0.75rem"),
		marginTop("0.5rem"),
	)
	rule(".card-step-active",
		borderLeft("3px solid var(--accent)"),
	)
	rule(".doc-section-sep",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		margin("1.25rem 0 0.5rem"),
		color("var(--text-dim, #9aa0a6)"),
		fontSize("0.8rem"),
	)
	rule(".doc-section-sep::before, .doc-section-sep::after",
		content("\"\""),
		flex("1 1 auto"),
		height("1px"),
		background("var(--border, rgba(255,255,255,0.1))"),
	)
	// Statement-import review header: a sticky two-tier bar — an in/out/net summary
	// above the account picker + Import action.
	rule(".draft-actionbar",
		position("sticky"),
		top("0"),
		zIndex("5"),
		display("flex"),
		flexDirection("column"),
		gap("0.55rem"),
		padding("0.65rem 0 0.6rem"),
		margin("0 0 0.4rem"),
		background("var(--bg-card)"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".draft-actionrow",
		display("flex"),
		gap("0.5rem"),
		alignItems("center"),
	)
	rule(".draft-actionrow .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".draft-actionrow .btn",
		flex("0 0 auto"),
	)
	// The money-in / money-out / net summary strip. Net is pushed to the right so it
	// foots the amount column like a statement's bottom line.
	rule(".draft-summary",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.35rem 1.4rem"),
	)
	rule(".draft-sum-item",
		display("flex"),
		flexDirection("column"),
		gap("0.12rem"),
	)
	rule(".draft-sum-item-net",
		marginLeft("auto"),
		textAlign("right"),
	)
	rule(".draft-sum-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".draft-sum-val",
		fontSize("1.02rem"),
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".draft-sum-net",
		fontSize("1.12rem"),
	)
	// Ledger layout for the reviewed rows: date/description on the left, a
	// right-aligned amount column, then the row actions — so a statement's figures
	// scan like the statement they came from.
	rule(".draft-ledger .row",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) auto auto"),
		alignItems("center"),
		prop("column-gap", "0.9rem"),
	)
	rule(".draft-ledger .row .amount",
		justifySelf("end"),
		fontSize("0.95rem"),
	)
	rule(".draft-subline",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.35rem"),
		marginTop("0.15rem"),
	)
	rule(".draft-row-actions",
		display("flex"),
		alignItems("center"),
		gap("0.05rem"),
	)
	// Category as a chip on the row; a dashed "+ Category" ghost when the AI left it
	// blank, so unmapped rows read as an action rather than an absence.
	rule(".draft-cat-chip",
		display("inline-flex"),
		alignItems("center"),
		fontSize("0.72rem"),
		fontWeight("600"),
		lineHeight("1"),
		padding("0.2rem 0.5rem"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		border("1px solid var(--border)"),
	)
	rule(".draft-cat-add",
		display("inline-flex"),
		alignItems("center"),
		fontSize("0.72rem"),
		fontWeight("600"),
		lineHeight("1"),
		padding("0.2rem 0.5rem"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--text-faint)"),
		border("1px dashed var(--border)"),
		cursor("pointer"),
	)
	rule(".draft-cat-add:hover",
		color("var(--accent)"),
		borderColor("var(--accent)"),
	)
	// Purpose-built inline editor: one aligned row of fields, actions right-aligned
	// below — steadier than the generic auto-fit form grid in the modal width.
	rule(".draft-edit-grid",
		display("grid"),
		gridTemplateColumns("9.5rem minmax(0, 1fr) 7.5rem 11rem"),
		gap("0.5rem"),
		alignItems("end"),
	)
	rule(".draft-edit-actions",
		prop("grid-column", "1 / -1"),
		display("flex"),
		gap("0.5rem"),
		justifyContent("flex-end"),
	)
	ruleMedia("(max-width: 640px)", ".draft-edit-grid",
		gridTemplateColumns("1fr 1fr"),
	)
	rule(".badge-warn",
		display("inline-block"),
		background("#b45309"),
		color("#fff"),
		fontSize("0.7rem"),
		fontWeight("700"),
		padding("0.05rem 0.4rem"),
		borderRadius("6px"),
	)
	rule(".goal-sub",
		marginTop("0.5rem"),
	)
	rule(".goal-sub-dim",
		color("var(--text-dim)"),
		fontSize("0.8rem"),
	)
	rule(".budget",
		minHeight("100px"),
	)
	rule(".budget:not(:hover):not(:focus-within) .btn-del-hover",
		opacity("0"),
		pointerEvents("none"),
	)
	ruleMedia("(pointer: coarse)", ".budget .btn-del-hover",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".btn-del-hover:focus-visible",
		opacity("1 !important"),
		pointerEvents("auto !important"),
	)
	rule(".cover-form",
		marginTop("0.6rem"),
		padding("0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		background("var(--bg-elev, rgba(255,255,255,0.02))"),
	)
	rule(".cover-form .form-grid",
		marginTop("0.4rem"),
	)
	rule(".bar",
		height("8px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		overflow("hidden"),
		margin("0.45rem 0 0.35rem"),
	)
	rule(".bar-fill",
		height("100%"),
		background("var(--accent)"),
		borderRadius("999px"),
		boxShadow("inset 0 1px 0 rgba(255,255,255,0.28), inset 0 -1px 1px rgba(0,0,0,0.14)"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", ".bar-fill",
		transition("width 0.45s cubic-bezier(0.2, 0.75, 0.2, 1)"),
	)
	rule(".bar-fill.near",
		background("#f59e0b"),
	)
	rule(".bar-fill.over",
		background("var(--danger)"),
	)
	rule(".bar-fill.done",
		background("var(--up, #54b884)"),
	)
	rule(".bar-fill.final",
		background("linear-gradient(90deg, var(--accent), var(--up, #54b884))"),
	)
	rule(".bar-fill.overdue",
		background("var(--danger)"),
	)
	rule(".bar-fill.soon",
		background("#f59e0b"),
	)
	rule(".pill",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.74rem"),
		fontWeight("700"),
		letterSpacing("0.02em"),
		padding("0.12rem 0.5rem"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		whiteSpace("nowrap"),
	)
	rule(".pill.is-warn",
		background("color-mix(in srgb, var(--warn) 16%, var(--bg-elev))"),
		borderColor("color-mix(in srgb, var(--warn) 40%, var(--border))"),
		color("var(--text)"),
	)
	rule(".pill.is-danger",
		background("color-mix(in srgb, var(--danger) 14%, var(--bg-elev))"),
		borderColor("color-mix(in srgb, var(--danger) 40%, var(--border))"),
		color("var(--text)"),
	)
	rule(".chip-suggest",
		borderRadius("999px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
		fontSize("0.84rem"),
		padding("0.3rem 0.7rem"),
		fontWeight("500"),
	)
	rule(".chip-suggest:hover",
		background("var(--accent-dim)"),
		color("var(--accent)"),
		borderColor("var(--accent-dim)"),
	)
	rule(".chat-pill",
		borderColor("var(--border) !important"),
	)
	rule(".sub-row .row-main",
		minWidth("9rem"),
		flex("1 1 auto"),
	)
	rule(".sub-actions",
		flex("none"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".smart-highlight-row",
		outline("2px solid var(--accent)"),
		borderRadius("6px"),
		transition("outline 0.3s ease"),
	)
	rule(".btn-sm",
		padding("0.25rem 0.5rem"),
		fontSize("0.82rem"),
		minHeight("0"),
	)
	rule(".btn-ghost-danger",
		color("var(--danger)"),
		border("1px solid var(--border)"),
		background("transparent"),
	)
	rule(".btn-ghost-danger:hover",
		background("var(--action-danger)"),
		color("#fff"),
		borderColor("var(--action-danger)"),
	)
	ruleMedia("(max-width: 760px)", ".sub-row .sub-actions",
		flexWrap("wrap"),
	)
	rule(".rank-badge",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		minWidth("1.5rem"),
		height("1.5rem"),
		padding("0 0.35rem"),
		borderRadius("999px"),
		background("var(--accent)"),
		color("#ffffff"),
		fontSize("0.75rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		flex("none"),
	)
	rule(".disclosure-toggle",
		background("transparent"),
		border("0"),
		color("var(--accent)"),
		font("inherit"),
		padding("0.35rem 0"),
		cursor("pointer"),
		textAlign("left"),
	)
	rule(".disclosure-toggle:hover",
		textDecoration("underline"),
	)
	rule(".btn.fit",
		width("fit-content"),
	)
	rule(".card-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.6rem"),
		marginBottom("0.75rem"),
	)
	rule(".card-head .card-title",
		margin("0"),
	)
	rule(".pace-badge",
		display("inline-block"),
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.02em"),
		padding("0.1rem 0.45rem"),
		borderRadius("999px"),
		whiteSpace("nowrap"),
		lineHeight("1.5"),
		border("1px solid var(--border)"),
	)
	rule(".pace-final",
		background("var(--bg-elev)"),
		color("var(--accent)"),
	)
	rule(".pace-overdue",
		background("rgba(216,113,111,0.18)"),
		color("var(--danger)"),
	)
	rule(".pace-soon",
		background("rgba(245,158,11,0.18)"),
		color("#d98c00"),
	)
	rule("[data-theme=\"light\"] .pace-soon",
		color("#b45309"),
	)
	rule(".pace-ontrack",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	// "Review due" chip — an attention (amber) tone distinct from the pace states, so a
	// goal that's gone stale under its review cadence reads at a glance.
	rule(".pace-review",
		background("rgba(245,158,11,0.16)"),
		color("#d98c00"),
		borderColor("rgba(245,158,11,0.4)"),
	)
	rule("[data-theme=\"light\"] .pace-review",
		color("#b45309"),
	)
	// "Paused until …" chip (GL7) — a calm, neutral tone (never an alarm): pausing is a
	// chosen state, so it reads as a quiet status, not a warning.
	rule(".pace-paused",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		borderColor("var(--border)"),
	)
	// Goal vision image banner (GL6): a small rounded photo atop the card.
	rule(".goal-card-photo",
		marginBottom("0.5rem"),
		borderRadius("8px"),
		overflow("hidden"),
	)
	rule(".goal-card-photo.is-missing",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		height("88px"),
		border("1px dashed var(--border)"),
		background("var(--bg-elev)"),
		fontSize("0.75rem"),
	)
	rule(".sev-pill",
		display("inline-block"),
		fontSize("0.68rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		padding("0.1rem 0.45rem"),
		borderRadius("999px"),
		whiteSpace("nowrap"),
		lineHeight("1.5"),
		border("1px solid transparent"),
		verticalAlign("middle"),
	)
	rule(".sev-info",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		borderColor("var(--border)"),
	)
	rule(".sev-warning",
		background("rgba(217,140, 0,0.14)"),
		color("#a16207"),
		borderColor("rgba(217,140,0,0.35)"),
	)
	rule(".sev-critical",
		background("rgba(220, 38,38,0.12)"),
		color("#b91c1c"),
		borderColor("rgba(220,38,38,0.30)"),
	)
	rule("[data-theme=\"light\"] .sev-warning",
		color("#92400e"),
	)
	rule("[data-theme=\"light\"] .sev-critical",
		color("#991b1b"),
	)
	// The collapsed Smart "peek" bar: a slim rounded pill signalling there are alerts for
	// the page, with near-zero vertical footprint (replaces the always-open Smart card).
	rule(".smart-peek",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		minHeight("30px"),
		padding("0.25rem 0.7rem"),
		marginBottom("0.6rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		color("var(--text-dim)"),
		fontSize("0.82rem"),
		fontWeight("500"),
		cursor("pointer"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".smart-peek:hover",
		borderColor("color-mix(in srgb, var(--accent) 40%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 82%, transparent)"),
	)
	rule(".smart-peek-title",
		color("var(--text)"),
		fontWeight("600"),
	)
	rule(".smart-peek-badge",
		minWidth("1.2rem"),
		padding("0.05rem 0.35rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, currentColor 16%, transparent)"),
		fontSize("0.72rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		textAlign("center"),
	)
	rule(".smart-peek-chev",
		opacity("0.55"),
	)
	// Top-bar variant of the Smart trigger (audit P0): icon + count in the title
	// row — no bottom margin (it's not a bar above content any more) and tighter
	// padding so it sits flush with the other top-bar chips.
	rule(".smart-peek.smart-peek-tb",
		marginBottom("0"),
		minHeight("26px"),
		padding("0.15rem 0.5rem"),
		gap("0.3rem"),
	)
	// Stable wrapper for the Smart strip: keeps the component root a <div> across the
	// peek↔card swap so the reconciler updates in place (see smartStripSlot). Purely
	// structural — no box of its own.
	rule(".smart-strip-slot",
		display("block"),
	)
	ruleMedia("(max-width: 760px)", ".budget-head",
		flexWrap("wrap"),
		alignItems("baseline"),
	)
	ruleMedia("(max-width: 760px)", ".budget-head .row-desc",
		flex("1 1 100%"),
	)
	ruleMedia("(max-width: 760px)", ".budget-head .budget-amount",
		flex("0 0 auto"),
	)
	rule(".stat-value",
		color("var(--text)"),
	)
	rule(".budget .row-desc",
		color("var(--text)"),
	)
	rule(".text-up",
		color("var(--money-positive, #54b884)"),
	)
	rule(".text-down",
		color("var(--money-negative, #d8716f)"),
	)
	rule(".text-warn",
		color("#cfa14e"),
	)
	rule(".text-dim",
		color("#ababb3"),
	)
	rule(".text-faint",
		color("#7d7d85"),
	)
	rule(".text-fg",
		color("#f4f4f5"),
	)
	rule(".bg-up",
		backgroundColor("#54b884"),
	)
	rule(".bg-down",
		backgroundColor("#d8716f"),
	)
	rule(".bg-warn",
		backgroundColor("#cfa14e"),
	)
	rule(".bg-dim",
		backgroundColor("#ababb3"),
	)
	rule("[data-state=\"selected\"]",
		background("var(--surface-selected)"),
	)
	rule("[data-state=\"dirty\"]",
		borderColor("var(--severity-warn)"),
	)
	rule("[data-state=\"error\"]",
		borderColor("var(--severity-alert)"),
	)
	rule("[aria-busy=\"true\"]",
		cursor("progress"),
	)
	rule(".bg-fg",
		backgroundColor("#f4f4f5"),
	)
	rule(".bg-line",
		backgroundColor("#232325"),
	)
	rule(".notice",
		padding("0.45rem 0.7rem"),
		borderRadius("6px"),
		fontSize("0.85rem"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	rule(".notice-warn",
		borderColor("#d39200"),
		background("rgba(211,146,0,0.08)"),
		color("#cfa14e"),
	)
	rule("[data-theme=\"light\"] .notice-warn",
		color("#92620a"),
		background("#fff8e1"),
		borderColor("#d39200"),
	)
	rule(".storage-bar",
		height("6px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		overflow("hidden"),
	)
	rule(".storage-bar-fill",
		height("100%"),
		background("var(--accent)"),
		borderRadius("999px"),
		boxShadow("inset 0 1px 0 rgba(255,255,255,0.28), inset 0 -1px 1px rgba(0,0,0,0.14)"),
		transition("width 0.35s ease"),
	)
	rule(".storage-bar-fill.storage-bar-warn",
		background("#f59e0b"),
	)
	rule(".csv-preview",
		borderCollapse("collapse"),
		fontSize("0.78rem"),
		width("100%"),
	)
	rule(".csv-preview th, .csv-preview td",
		padding("0.15rem 0.5rem"),
		border("1px solid var(--border)"),
		textAlign("left"),
		whiteSpace("nowrap"),
	)
	rule(".csv-preview thead",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	rule(".ref-positive",
		color("var(--accent)"),
	)
	rule("[data-theme=\"light\"] .ref-positive",
		color("#1a7a47"),
	)
	rule(".field-wide",
		gridColumn("1 / -1"),
	)
	rule(".check",
		background("transparent"),
		border("1px solid transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		fontSize("1.15rem"),
		lineHeight("1"),
		minWidth("24px"),
		minHeight("24px"),
		display("inline-grid"),
		placeItems("center"),
		marginRight("0.4rem"),
		boxSizing("border-box"),
	)
	rule(".check:hover",
		color("var(--accent)"),
	)
	rule(".row.selected .check",
		background("var(--accent-dim)"),
		borderColor("var(--accent)"),
		color("var(--accent)"),
	)
	rule(".row.done .row-desc",
		textDecoration("line-through"),
		color("var(--text-faint)"),
	)
	rule(".task-meta",
		display("flex"),
		gap("0.6rem"),
		alignItems("center"),
		marginTop("0.15rem"),
	)
	rule(".badge-prio",
		fontSize("0.75rem"),
	)
	rule(".prio-high",
		background("#3b0d0d"),
		color("#fca5a5"),
		border("1px solid #7f1d1d"),
	)
	rule(".prio-med",
		background("#2a230c"),
		color("#fcd34d"),
		border("1px solid #854d0e"),
	)
	rule(".prio-low",
		background("#0c2a17"),
		color("#86efac"),
		border("1px solid #166534"),
	)
	rule("html .card:hover",
		transform("translateY(calc(-1 * var(--wonder-lift) * var(--wonder-on)))"),
		boxShadow("var(--wonder-shadow)"),
	)
	rule("html .w:not(.drag):hover",
		transform("translateY(calc(-1 * var(--wonder-lift) * var(--wonder-on))) !important"),
		boxShadow("var(--wonder-shadow) !important"),
	)
	rule("html .w:not(.drag)",
		boxShadow("0 1px 1px rgba(0,0,0,0.20), 0 10px 26px -18px rgba(0,0,0,0.55),\n                    inset 0 1px 0 rgba(255,255,255,0.035) !important"),
	)
	rule("html[data-theme=\"light\"] .w:not(.drag)",
		boxShadow("0 1px 2px rgba(17,24,39,0.05), 0 12px 28px -20px rgba(17,24,39,0.16),\n                    inset 0 1px 0 rgba(255,255,255,0.7) !important"),
	)
	rule("html .row:not(.txn-table .row):hover",
		transform("translateX(calc(3px * var(--wonder-on))) !important"),
	)
	rule("html .btn,\n      html .data-btn, html .seg-btn, html .add-item, html .menu-btn, html .icon-btn",
		transition("transform var(--wonder-dur-fast) var(--wonder-ease),\n                    filter 0.12s ease,\n                    background-color var(--wonder-dur-fast) ease,\n                    color var(--wonder-dur-fast) ease"),
	)
	rule("[data-wonder=\"off\"] .rows .row,\n      [data-wonder=\"off\"] .list-rows .row,\n      [data-wonder=\"off\"] .rows .row:not(.txn-table .row),\n      [data-wonder=\"off\"] .list-rows .row:not(.txn-table .row)",
		animation("none !important"),
	)
	rule(".sev-pill",
		fontSize("0.67rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		whiteSpace("nowrap"),
		lineHeight("1.4"),
		padding("0.1rem 0.42rem"),
		borderRadius("999px"),
		border("1px solid transparent"),
		display("inline-block"),
		verticalAlign("middle"),
		boxShadow("inset 0 1px 0 rgba(255,255,255,0.06)"),
	)
	rule(".sev-info",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		borderColor("var(--border)"),
	)
	rule(".sev-warning",
		background("rgba(217,140,0,0.13)"),
		color("#d4a017"),
		borderColor("rgba(217,140,0,0.30)"),
	)
	rule(".sev-critical",
		background("rgba(220,38,38,0.11)"),
		color("#f87171"),
		borderColor("rgba(220,38,38,0.28)"),
	)
	rule("[data-theme=\"light\"] .sev-warning",
		color("#92400e"),
		background("rgba(245,158,11,0.12)"),
	)
	rule("[data-theme=\"light\"] .sev-critical",
		color("#991b1b"),
		background("rgba(220,38,38,0.09)"),
	)
	rule(".notif-ctrl-btn",
		display("inline-grid"),
		placeItems("center"),
		minWidth("28px"),
		minHeight("28px"),
		padding("0.1rem 0.3rem"),
		background("transparent"),
		border("1px solid transparent"),
		borderRadius("6px"),
		color("var(--text-faint)"),
		fontSize("0.82rem"),
		lineHeight("1"),
		cursor("pointer"),
		transition("color var(--wonder-dur-fast) ease,\n                    background-color var(--wonder-dur-fast) ease,\n                    border-color var(--wonder-dur-fast) ease"),
	)
	rule(".notif-ctrl-btn:hover",
		color("var(--text)"),
		background("var(--bg-elev)"),
		borderColor("var(--border)"),
	)
	rule(".notif-ctrl-btn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("1px"),
	)
	rule(".notif-ctrl-dismiss:hover",
		color("var(--danger)"),
		background("rgba(216,113,111,0.10)"),
		borderColor("rgba(216,113,111,0.28)"),
	)
	rule("[data-theme=\"light\"] .notif-ctrl-btn:hover",
		background("#f3f2ef"),
		borderColor("#d1cfc9"),
	)
	rule("[data-theme=\"light\"] .notif-ctrl-dismiss:hover",
		background("rgba(185,28,28,0.07)"),
		borderColor("rgba(185,28,28,0.22)"),
	)
	rule("[data-testid=\"members-single-device-note\"]",
		fontSize("0.82rem"),
		color("var(--text-faint)"),
		paddingLeft("0.6rem"),
		borderLeft("2px solid var(--border)"),
		marginTop("0.3rem"),
		marginBottom("0.5rem"),
		lineHeight("1.5"),
	)
	rule("[data-theme=\"light\"] [data-testid=\"members-single-device-note\"]",
		color("#636369"),
		borderLeftColor("#d1cfc9"),
	)
	rule(".badge",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		border("1px solid var(--border)"),
	)
	rule(".badge-muted",
		background("transparent"),
		color("var(--text-faint)"),
		border("1px solid var(--border)"),
		opacity("0.8"),
	)
	rule("[data-theme=\"light\"] .badge",
		background("#f3f2ef"),
		color("#636369"),
		borderColor("#d1cfc9"),
	)
	rule("[data-theme=\"light\"] .badge-muted",
		color("#9a9a9f"),
		borderColor("#d1cfc9"),
	)
	rule("[data-testid=\"settings-notifications\"]",
		display("flex"),
		flexDirection("column"),
		gap("0"),
	)
	rule("[data-testid=\"settings-notifications\"] h4.set-label",
		marginTop("0.9rem"),
	)
	rule("[data-testid=\"settings-manage-alerts\"]",
		display("flex"),
		flexDirection("column"),
	)
	rule("[data-testid=\"settings-manage-alerts\"] .toggle-row + .toggle-row",
		borderTop("none"),
	)
	rule(".btn-ghost",
		background("transparent"),
		borderColor("transparent"),
		color("var(--text-dim)"),
		fontWeight("400"),
	)
	rule(".btn-ghost:hover",
		background("var(--bg-elev)"),
		borderColor("var(--border)"),
		color("var(--text)"),
	)
	rule("[data-theme=\"light\"] .btn-ghost",
		color("#636369"),
	)
	rule("[data-theme=\"light\"] .btn-ghost:hover",
		background("#f3f2ef"),
		borderColor("#d1cfc9"),
		color("#1c1c1e"),
	)
	rule("[role=\"tablist\"] .btn:not(.btn-ghost)",
		borderBottomColor("var(--accent)"),
		borderBottomWidth("2px"),
		fontWeight("600"),
	)
	rule(".smart-card",
		background("var(--bg-card)"),
		border("1px solid var(--border) !important"),
		borderRadius("8px !important"),
		boxShadow("0 1px 3px rgba(0,0,0,0.18), inset 0 1px 0 rgba(255,255,255,0.03)"),
		transition("box-shadow var(--wonder-dur) var(--wonder-ease)"),
	)
	rule(".smart-card:hover",
		boxShadow("0 3px 10px rgba(0,0,0,0.24), inset 0 1px 0 rgba(255,255,255,0.04)"),
	)
	rule(".smart-card[data-severity=\"alert\"]",
		borderLeft("3px solid var(--danger)  !important"),
	)
	rule(".smart-card[data-severity=\"warn\"]",
		borderLeft("3px solid #cfa14e !important"),
	)
	rule(".smart-card[data-severity=\"nudge\"]",
		borderLeft("3px solid var(--accent)  !important"),
	)
	rule("[data-theme=\"light\"] .smart-card",
		background("#ffffff"),
		boxShadow("0 1px 3px rgba(17,24,39,0.07), inset 0 1px 0 rgba(255,255,255,0.9)"),
	)
	rule("[data-theme=\"light\"] .smart-card:hover",
		boxShadow("0 3px 8px rgba(17,24,39,0.11)"),
	)
	rule(".notif-catchup-banner",
		display("flex"),
		alignItems("baseline"),
		gap("0.5rem"),
		flexWrap("wrap"),
		padding("0.55rem 0.8rem"),
		marginBottom("0.75rem"),
		background("var(--accent-dim)"),
		border("1px solid rgba(46,139,87,0.28)"),
		borderRadius("6px"),
	)
	rule("[data-theme=\"light\"] .notif-catchup-banner",
		background("#edf7f1"),
		borderColor("rgba(46,139,87,0.30)"),
	)
	rule(".notif-catchup-label",
		fontSize("0.78rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing("0.06em"),
		color("var(--accent)"),
	)
	rule("[data-theme=\"light\"] .notif-catchup-label",
		color("#1a7a47"),
	)
	rule(".notif-catchup-count",
		fontSize("0.88rem"),
		color("var(--text-dim)"),
	)
	rule("[data-theme=\"light\"] .notif-catchup-count",
		color("#3c3c43"),
	)
	rawBlockMedia("(prefers-reduced-motion: no-preference)", "@keyframes catchup-fadeslide{from { opacity: 0; transform: translateY(6px); }\n          to   { opacity: 1; transform: translateY(0); }}")
	ruleMedia("(prefers-reduced-motion: no-preference)", ".notif-catchup-banner .notif-catchup-label",
		animation("catchup-fadeslide 220ms var(--wonder-ease-out) both"),
		animationDelay("0ms"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", ".notif-catchup-banner .notif-catchup-count",
		animation("catchup-fadeslide 220ms var(--wonder-ease-out) both"),
		animationDelay("80ms"),
	)
	// The "While you were away" catch-up card reads as a full-width dashboard tile
	// (matching the .card / bento tile chrome) rather than an unstyled floating block,
	// so it sits cleanly above the bento grid instead of breaking its rhythm (C271).
	rule(".catchup-card",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1rem"),
		flexWrap("wrap"),
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
		padding("1rem 1.25rem"),
		marginBottom("1rem"),
		boxShadow("0 1px 1px rgba(0,0,0,0.20), 0 10px 26px -18px rgba(0,0,0,0.55),\n                    inset 0 1px 0 rgba(255,255,255,0.035)"),
	)
	rule("[data-theme=\"light\"] .catchup-card",
		boxShadow("0 1px 2px rgba(17,24,39,0.05), 0 12px 28px -20px rgba(17,24,39,0.16),\n                    inset 0 1px 0 rgba(255,255,255,0.7)"),
	)
	rule(".catchup-card-body",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		flex("1"),
		minWidth("0"),
	)
	rule(".catchup-card-icon",
		flex("none"),
		fontSize("1.35rem"),
		lineHeight("1"),
	)
	rule(".catchup-card-text",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("0"),
	)
	rule(".catchup-card-text strong",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".catchup-card-text p",
		margin("0"),
		fontSize("0.85rem"),
		color("var(--text-dim)"),
	)
	rule(".catchup-card-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flex("none"),
	)
	rawBlockMedia("(prefers-reduced-motion: no-preference)", "@keyframes catchup-card-in{from { opacity: 0; transform: translateY(10px); }\n          to   { opacity: 1; transform: translateY(0); }}")
	ruleMedia("(prefers-reduced-motion: no-preference)", ".catchup-card",
		animation("catchup-card-in 280ms var(--wonder-ease-out) 60ms both"),
	)
	rule(":root",
		customProp("--surface-page", "var(--bg-base)"),
		customProp("--surface-card", "var(--bg-card)"),
		customProp("--surface-raised", "var(--bg-elev)"),
		customProp("--surface-sunken", "color-mix(in srgb, var(--bg-base) 88%, #000 12%)"),
		customProp("--surface-selected", "color-mix(in srgb, var(--accent) 16%, var(--bg-card))"),
		customProp("--border-subtle", "color-mix(in srgb, var(--border) 70%, transparent)"),
		customProp("--border-strong", "color-mix(in srgb, var(--border) 72%, var(--text) 28%)"),
		customProp("--border-selected", "var(--accent)"),
		customProp("--text-primary", "var(--text)"),
		customProp("--text-secondary", "var(--text-dim)"),
		customProp("--text-tertiary", "var(--text-faint)"),
		customProp("--text-inverse", "#ffffff"),
		customProp("--interactive", "var(--accent)"),
		customProp("--interactive-hover", "color-mix(in srgb, var(--accent) 84%, var(--text) 16%)"),
		customProp("--interactive-muted", "var(--accent-dim)"),
		customProp("--focus-ring", "var(--accent)"),
		customProp("--money-positive", "var(--up)"),
		customProp("--money-negative", "var(--down)"),
		customProp("--money-neutral", "var(--text-primary)"),
		customProp("--severity-info", "var(--accent)"),
		customProp("--severity-nudge", "var(--accent)"),
		customProp("--severity-warn", "var(--warn)"),
		customProp("--severity-alert", "var(--danger)"),
		customProp("--bg", "var(--surface-page)"),
		customProp("--action-danger", "#c0392b"),
		customProp("--chart-1", "var(--accent)"),
		customProp("--chart-2", "var(--up)"),
		customProp("--chart-3", "var(--warn)"),
		customProp("--chart-4", "#52a3ff"),
		customProp("--chart-5", "#b88cff"),
		customProp("--chart-negative", "var(--down)"),
		customProp("--space-0", "0"),
		customProp("--space-1", "4px"),
		customProp("--space-2", "8px"),
		customProp("--space-3", "12px"),
		customProp("--space-4", "16px"),
		customProp("--space-5", "24px"),
		customProp("--space-6", "32px"),
		customProp("--space-7", "48px"),
		customProp("--space-8", "64px"),
		customProp("--radius-0", "0"),
		customProp("--radius-xs", "2px"),
		customProp("--radius-sm", "4px"),
		customProp("--radius-md", "6px"),
		customProp("--radius-lg", "8px"),
		customProp("--radius-pill", "999px"),
		customProp("--shadow-1", "0 1px 2px rgba(0,0,0,0.22)"),
		customProp("--shadow-2", "0 8px 24px rgba(0,0,0,0.28)"),
		customProp("--shadow-3", "0 20px 60px rgba(0,0,0,0.38)"),
		customProp("--type-11", "calc(11px * var(--ui-scale, 1))"),
		customProp("--type-12", "calc(12px * var(--ui-scale, 1))"),
		customProp("--type-13", "calc(13px * var(--ui-scale, 1))"),
		customProp("--type-14", "calc(14px * var(--ui-scale, 1))"),
		customProp("--type-16", "calc(16px * var(--ui-scale, 1))"),
		customProp("--type-18", "calc(18px * var(--ui-scale, 1))"),
		customProp("--type-20", "calc(20px * var(--ui-scale, 1))"),
		customProp("--type-24", "calc(24px * var(--ui-scale, 1))"),
		customProp("--type-32", "calc(32px * var(--ui-scale, 1))"),
		customProp("--control-h", "36px"),
		customProp("--control-h-compact", "32px"),
		customProp("--field-h", "36px"),
		customProp("--icon-button-size", "32px"),
		customProp("--focus-ring-width", "2px"),
		customProp("--focus-ring-offset", "2px"),
		customProp("--hover-tint", "color-mix(in srgb, var(--interactive) 8%, transparent)"),
		customProp("--pressed-scale", "0.985"),
		customProp("--disabled-opacity", "0.6"),
		// v1.2.3 motion spec — the only duration scale. Legacy names (base/medium/
		// slow) alias onto the spec tokens so older rules inherit the scale.
		customProp("--motion-instant", "0ms"),
		customProp("--motion-micro", "80ms"),
		customProp("--motion-fast", "120ms"),
		customProp("--motion-standard", "180ms"),
		customProp("--motion-layout", "240ms"),
		customProp("--motion-overlay", "280ms"),
		customProp("--motion-data", "320ms"),
		customProp("--motion-narrative", "450ms"),
		customProp("--motion-base", "var(--motion-standard)"),
		customProp("--motion-medium", "var(--motion-layout)"),
		customProp("--motion-slow", "var(--motion-data)"),
		// The only three easing curves: standard movement, enter, exit. No overshoot.
		customProp("--ease-standard", "cubic-bezier(0.2, 0, 0, 1)"),
		customProp("--ease-enter", "cubic-bezier(0.16, 1, 0.3, 1)"),
		customProp("--ease-exit", "cubic-bezier(0.4, 0, 1, 1)"),
		customProp("--ease-emphasized", "var(--ease-standard)"),
	)
	ruleMedia("(pointer: coarse)", ":root",
		customProp("--control-h", "44px"),
		customProp("--field-h", "44px"),
		customProp("--icon-button-size", "44px"),
	)
	rule("body.cf",
		webkitFontSmoothing("antialiased"),
	)
	rule(".fig",
		fontVariantNumeric("tabular-nums lining-nums"),
	)
	rule(".t-caption",
		fontSize("0.75rem"),
	)
	rule(".t-body",
		fontSize("0.8125rem"),
	)
	rule(".t-figure",
		fontSize("1.5rem"),
	)
	rule(".t-figure-lg",
		fontSize("2.125rem"),
		letterSpacing("-0.025em"),
	)
	rule(".nav:hover",
		background("#161617"),
		color("#f4f4f5"),
	)
	rule(".trend-body",
		display("flex"),
		flexDirection("column"),
		minHeight("0"),
		height("100%"),
		gap(".45rem"),
		overflow("hidden"),
	)
	rule(".trend-head",
		flex("0 0 auto"),
	)
	rule(".trend-figure",
		flex("0 0 auto"),
		lineHeight("1.05"),
	)
	rule(".trend-standard",
		marginTop(".18rem"),
	)
	rule(".trend-expanded",
		display("none"),
		flex("0 0 auto"),
		gridTemplateColumns("repeat(2,minmax(0,1fr))"),
		gap(".45rem"),
	)
	rule(".trend-stat",
		minWidth("0"),
		border("1px solid rgba(244,244,245,.08)"),
		background("rgba(244,244,245,.025)"),
		padding(".38rem .45rem"),
	)
	rule(".trend-stat span",
		display("block"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".trend-chart",
		flex("1 1 auto"),
		minHeight("0"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-body",
		gap(".25rem"),
		justifyContent("flex-end"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap(".5rem"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-figure",
		fontSize("1.02rem"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"1\"][data-row-span=\"1\"] .trend-head",
		display("block"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"1\"][data-row-span=\"1\"] .trend-standard,\n      .w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-expanded",
		display("none"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-chart",
		flex("0 0 54px"),
		minHeight("54px"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-chart .tick,\n      .w[data-widget=\"trend\"][data-row-span=\"1\"] .trend-chart .domain",
		display("none"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"1\"][data-row-span=\"1\"] .trend-chart",
		flexBasis("58px"),
		minHeight("58px"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"1\"][data-row-span=\"1\"] .trend-chart svg",
		overflow("visible"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"2\"] .trend-chart",
		flexBasis("0"),
		minHeight("118px"),
	)
	rule(".w[data-widget=\"trend\"][data-row-span=\"3\"] .trend-chart",
		flexBasis("0"),
		minHeight("220px"),
		marginTop("auto"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"2\"] .trend-body,\n      .w[data-widget=\"trend\"][data-col-span=\"3\"] .trend-body,\n      .w[data-widget=\"trend\"][data-col-span=\"4\"] .trend-body",
		gap(".65rem"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"2\"] .trend-figure,\n      .w[data-widget=\"trend\"][data-col-span=\"3\"] .trend-figure,\n      .w[data-widget=\"trend\"][data-col-span=\"4\"] .trend-figure",
		fontSize("1.8rem"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"1\"][data-row-span=\"3\"] .trend-figure",
		fontSize("1.55rem"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"2\"][data-row-span=\"2\"] .trend-expanded,\n      .w[data-widget=\"trend\"][data-col-span=\"2\"][data-row-span=\"3\"] .trend-expanded,\n      .w[data-widget=\"trend\"][data-col-span=\"3\"][data-row-span=\"2\"] .trend-expanded,\n      .w[data-widget=\"trend\"][data-col-span=\"3\"][data-row-span=\"3\"] .trend-expanded,\n      .w[data-widget=\"trend\"][data-col-span=\"4\"][data-row-span=\"2\"] .trend-expanded,\n      .w[data-widget=\"trend\"][data-col-span=\"4\"][data-row-span=\"3\"] .trend-expanded",
		display("grid"),
	)
	rule(".w[data-widget=\"trend\"][data-col-span=\"2\"][data-row-span=\"3\"] .trend-body,\n      .w[data-widget=\"trend\"][data-col-span=\"3\"][data-row-span=\"3\"] .trend-body,\n      .w[data-widget=\"trend\"][data-col-span=\"4\"][data-row-span=\"3\"] .trend-body",
		display("grid"),
		gridTemplateRows("auto auto minmax(0,1fr)"),
		alignContent("stretch"),
	)
	rule(".studio-design",
		marginTop("14px"),
	)
	rule(".studio-design-head",
		marginBottom("18px"),
	)
	rule(".studio-eyebrow",
		display("block"),
		fontSize(".7rem"),
		fontWeight("700"),
		letterSpacing(".14em"),
		textTransform("uppercase"),
		color("var(--accent)"),
	)
	rule(".studio-design-title",
		fontFamily("var(--font-display, Georgia, serif)"),
		fontSize("1.7rem"),
		lineHeight("1.1"),
		margin(".18rem 0 .15rem"),
		color("var(--text)"),
	)
	rule(".studio-design-sub",
		color("var(--text-dim)"),
		fontSize(".95rem"),
		maxWidth("46ch"),
	)
	rule(".studio-design-grid",
		display("grid"),
		gridTemplateColumns("minmax(0,400px) minmax(0,1fr)"),
		gap("28px"),
		alignItems("start"),
	)
	ruleMedia("(max-width: 1100px)", ".studio-design-grid",
		gridTemplateColumns("1fr"),
	)
	rule(".studio-form",
		display("flex"),
		flexDirection("column"),
		gap("18px"),
	)
	rule(".studio-section",
		borderTop("1px solid var(--border)"),
		paddingTop("16px"),
	)
	rule(".studio-section:first-of-type",
		borderTop("0"),
		paddingTop("0"),
	)
	rule(".studio-section-title",
		display("block"),
		fontSize(".78rem"),
		fontWeight("700"),
		letterSpacing(".06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		marginBottom("12px"),
	)
	rule(".studio-section-body",
		display("flex"),
		flexDirection("column"),
		gap("14px"),
	)
	rule(".field-label",
		display("flex"),
		flexDirection("column"),
		gap("6px"),
	)
	rule(".studio-label",
		fontSize(".82rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".studio-hint",
		fontSize(".76rem"),
		color("var(--text-faint)"),
		lineHeight("1.4"),
	)
	rule(".studio-formula",
		fontSize(".74rem"),
		color("var(--accent)"),
		fontFamily("var(--font-mono, ui-monospace, monospace)"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
		border("1px solid color-mix(in srgb, var(--accent) 22%, var(--border))"),
		borderRadius("7px"),
		padding(".3rem .55rem"),
		alignSelf("flex-start"),
	)
	rule(".studio-field, .field-label > .field, .field-label select, .field-label .seg",
		width("100%"),
	)
	rule(".studio-field-lg",
		fontSize("1.15rem"),
		fontWeight("600"),
		padding(".6rem .8rem"),
	)
	rule(".studio-section-body > .studio-type-grid, .studio-type-grid",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0,1fr))"),
		gap("10px"),
	)
	rule(".studio-type-card",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("3px"),
		textAlign("left"),
		padding("13px 14px"),
		border("1px solid var(--border)"),
		borderRadius("13px"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color .12s, background .12s, transform .08s"),
	)
	rule(".studio-type-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 50%, var(--border))"),
		transform("translateY(-1px)"),
	)
	rule(".studio-type-card.is-selected",
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-card))"),
		boxShadow("inset 0 0 0 1px var(--accent)"),
	)
	rule(".studio-type-icon",
		color("var(--accent)"),
		display("inline-flex"),
		marginBottom("4px"),
	)
	rule(".studio-type-icon svg",
		width("26px"),
		height("26px"),
	)
	rule(".studio-type-label",
		fontWeight("650"),
		fontSize(".98rem"),
	)
	rule(".studio-type-desc",
		fontSize(".76rem"),
		color("var(--text-faint)"),
	)
	rule(".studio-size-row",
		display("grid"),
		gridTemplateColumns("1fr 1fr"),
		gap("14px"),
	)
	rule(".studio-blocks",
		display("flex"),
		flexDirection("column"),
		gap("8px"),
	)
	rule(".studio-block-head, .studio-block-row",
		display("grid"),
		gridTemplateColumns("26px minmax(110px,1fr) minmax(150px,1.9fr) 84px auto"),
		gap("10px"),
		alignItems("center"),
	)
	rule(".studio-block-head",
		padding("0 11px"),
		fontSize(".68rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing(".05em"),
		color("var(--text-faint)"),
	)
	rule(".studio-block-row",
		padding("9px 11px"),
		border("1px solid var(--border)"),
		borderRadius("11px"),
		background("var(--bg-card)"),
	)
	rule(".studio-block-row .field-compact",
		minWidth("0"),
	)
	rule(".studio-block-row .field-compact select, .studio-block-row .field-compact .field",
		width("100%"),
	)
	rule(".studio-block-shows",
		display("flex"),
		gap("8px"),
		minWidth("0"),
		alignItems("end"),
	)
	rule(".studio-block-shows > *",
		flex("1 1 0"),
		minWidth("0"),
	)
	rule(".studio-microfield",
		display("flex"),
		flexDirection("column"),
		gap("3px"),
	)
	rule(".studio-microlabel",
		fontSize(".64rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing(".04em"),
		color("var(--text-faint)"),
	)
	rule(".studio-addblock",
		width("100%"),
		borderStyle("dashed !important"),
		color("var(--text-dim)"),
	)
	rule(".studio-addblock:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".studio-toggle",
		alignSelf("flex-start"),
		background("transparent !important"),
		border("0 !important"),
		padding(".2rem 0 !important"),
		color("var(--accent)"),
		fontSize(".82rem"),
		textDecoration("none"),
	)
	rule(".studio-toggle:hover",
		textDecoration("underline"),
	)
	rule(".studio-block-num",
		width("1.6rem"),
		height("1.6rem"),
		display("grid"),
		placeItems("center"),
		borderRadius("7px"),
		background("var(--bg-elev)"),
		color("var(--text-faint)"),
		fontSize(".72rem"),
		fontWeight("700"),
	)
	rule(".studio-block-actions",
		display("flex"),
		gap("6px"),
		alignItems("center"),
		justifySelf("end"),
	)
	rule(".studio-block-move-group",
		display("flex"),
		gap("3px"),
	)
	rule(".btn-icon, .studio-block-move",
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("7px"),
		color("var(--text-dim)"),
		cursor("pointer"),
		padding(".2rem .45rem"),
		height("2rem"),
		transition("border-color .12s, color .12s"),
	)
	rule(".btn-icon:hover, .studio-block-move:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".field-compact",
		gap("0"),
	)
	rule(".studio-list-root",
		display("flex"),
		flexDirection("column"),
		flex("1 1 auto"),
		minHeight("0"),
		height("100%"),
	)
	rule(".studio-list-body",
		flex("1 1 auto"),
		minHeight("0"),
		overflowY("auto"),
	)
	rule(".studio-list-footer",
		flex("0 0 auto"),
		display("flex"),
		flexDirection("column"),
		gap("6px"),
		paddingTop("8px"),
		marginTop("8px"),
		borderTop("1px solid var(--border)"),
	)
	rule(".dash-paged",
		display("flex"),
		flexDirection("column"),
		height("100%"),
		minHeight("0"),
	)
	rule(".dash-paged-body",
		flex("1 1 auto"),
		minHeight("0"),
		overflowY("auto"),
	)
	rule(".dash-paged .studio-list-pager",
		flex("0 0 auto"),
		paddingTop("8px"),
		marginTop("8px"),
		borderTop("1px solid var(--border)"),
	)
	rule(".studio-list-pager",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		gap("14px"),
	)
	rule(".studio-list-pager .btn-icon",
		width("1.8rem"),
		height("1.8rem"),
		display("grid"),
		placeItems("center"),
		padding("0"),
	)
	rule(".studio-list-pager .btn-icon svg",
		width("1rem"),
		height("1rem"),
	)
	rule(".studio-list-pager .t-caption",
		minWidth("4.5rem"),
		textAlign("center"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".btn-icon.is-disabled",
		opacity(".3"),
		pointerEvents("none"),
	)
	rule(".studio-list-link",
		color("var(--accent)"),
		alignSelf("flex-start"),
		fontSize(".82rem"),
		background("transparent"),
		border("0"),
		cursor("pointer"),
		padding("0"),
	)
	rule(".studio-list-link:hover",
		textDecoration("underline"),
	)
	rule(".fb-groups",
		display("flex"),
		flexDirection("column"),
		gap("16px"),
	)
	rule(".fb-group-title",
		display("block"),
		fontSize(".7rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing(".06em"),
		color("var(--accent)"),
		marginBottom("6px"),
	)
	rule(".fb-meta",
		fontFamily("var(--font-mono, ui-monospace, monospace)"),
		fontSize(".72rem"),
		color("var(--text-faint)"),
	)
	// ---- Formula workbench (the reusable FormulaBuilder / "Budget metrics" panel) ----
	// One cohesive panel: a header, the expression bar with the live result read out
	// inline, quick presets, a save row, and a dense click-to-insert variable palette.
	rule(".fb",
		display("flex"),
		flexDirection("column"),
		gap("1.1rem"),
	)
	rule(".fb-workbench",
		display("flex"),
		flexDirection("column"),
		gap("0.7rem"),
	)
	rule(".fb-head",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".fb-title",
		fontFamily("var(--font-display), 'Fraunces', serif"),
		fontSize("1.15rem"),
		fontWeight("600"),
		letterSpacing("-0.01em"),
		color("var(--text)"),
	)
	rule(".fb-sub",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	// Expression is the hero: a wide monospace input with the live result to its right.
	rule(".fb-exprbar",
		display("flex"),
		alignItems("stretch"),
		gap("0.6rem"),
		flexWrap("wrap"),
	)
	rule(".fb-expr",
		flex("1 1 260px"),
		minHeight("54px"),
		fontFamily("var(--font-mono, ui-monospace, monospace)"),
		fontSize("0.95rem"),
	)
	rule(".fb-result",
		display("flex"),
		alignItems("center"),
		gap("0.45rem"),
		flex("0 1 auto"),
		minWidth("140px"),
		maxWidth("48%"),
		padding("0 1.05rem"),
		borderRadius("10px"),
		border("1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 9%, transparent)"),
	)
	rule(".fb-result-eq",
		fontSize("1.1rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".fb-result-val",
		fontFamily("var(--font-display), 'Fraunces', serif"),
		fontSize("1.55rem"),
		fontWeight("700"),
		color("var(--accent)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".fb-result.is-empty",
		borderStyle("dashed"),
		background("transparent"),
	)
	rule(".fb-result.is-empty .fb-result-val",
		color("var(--text-faint)"),
	)
	rule(".fb-result.is-err",
		borderColor("color-mix(in srgb, var(--danger) 42%, var(--border))"),
		background("color-mix(in srgb, var(--danger) 9%, transparent)"),
	)
	rule(".fb-result-err",
		color("var(--danger)"),
		fontSize("0.8rem"),
		fontWeight("500"),
		whiteSpace("normal"),
	)
	rule(".fb-presets",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".fb-presets-lead",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		marginRight("0.15rem"),
	)
	rule(".fb-save",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	rule(".fb-save-name",
		flex("1 1 220px"),
		maxWidth("360px"),
	)
	rule(".fb-save-btn",
		flexShrink("0"),
	)
	rule(".fb-msg",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)
	// Palette: dense grid of variable chips, grouped and separated from the workbench by
	// a hairline. Click a chip to insert its variable into the expression.
	rule(".fb-palette",
		display("flex"),
		flexDirection("column"),
		gap("0.8rem"),
		paddingTop("1rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".fb-palette-lead",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	rule(".fb-pal-groups",
		display("flex"),
		flexDirection("column"),
		gap("0.9rem"),
	)
	rule(".fb-pal-group",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".fb-pal-title",
		fontSize("0.68rem"),
		fontWeight("700"),
		textTransform("uppercase"),
		letterSpacing("0.07em"),
		color("var(--accent)"),
	)
	rule(".fb-pal-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(184px, 1fr))"),
		gap("0.4rem"),
	)
	rule(".fb-chip",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.1rem"),
		textAlign("left"),
		padding("0.5rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("9px"),
		background("var(--bg-elev)"),
		prop("cursor", "pointer"),
		transition("border-color .12s ease, background .12s ease, transform .12s ease"),
	)
	rule(".fb-chip:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 9%, var(--bg-elev))"),
		transform("translateY(-1px)"),
	)
	rule(".fb-chip-label",
		fontSize("0.82rem"),
		fontWeight("600"),
		color("var(--text)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		maxWidth("100%"),
	)
	rule(".fb-chip-val",
		fontSize("0.82rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".studio-stage-wrap",
		position("sticky"),
		top("16px"),
		display("flex"),
		flexDirection("column"),
		gap("12px"),
		padding("18px"),
		border("1px solid var(--border)"),
		borderRadius("18px"),
		background("var(--bg-elev)"),
	)
	rule(".studio-stage-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
	)
	rule(".studio-stage-hint",
		fontSize(".74rem"),
		color("var(--text-faint)"),
	)
	rule(".studio-stage",
		borderRadius("14px"),
		padding("14px"),
		background("linear-gradient(0deg, color-mix(in srgb, var(--accent) 4%, transparent), transparent),\n          repeating-linear-gradient(0deg, transparent, transparent 37px, color-mix(in srgb, var(--text) 5%, transparent) 37px, color-mix(in srgb, var(--text) 5%, transparent) 38px),\n          repeating-linear-gradient(90deg, transparent, transparent 37px, color-mix(in srgb, var(--text) 5%, transparent) 37px, color-mix(in srgb, var(--text) 5%, transparent) 38px),\n          var(--bg)"),
		border("1px solid var(--border)"),
		minHeight("180px"),
		display("grid"),
		placeItems("stretch"),
	)
	rule(".studio-stage-inner",
		display("grid"),
		gridTemplateColumns("repeat(4, minmax(0,1fr))"),
		gridAutoRows("152px"),
		gap("10px"),
		alignContent("start"),
	)
	rule(".studio-stage-cell",
		display("grid"),
	)
	rule(".studio-stage-cell .w",
		minHeight("0"),
		height("100%"),
	)
	rule(".studio-publish",
		width("100%"),
		justifyContent("center"),
		padding(".7rem 1rem"),
		fontWeight("650"),
	)
	rule(".studio-status",
		fontSize(".86rem"),
		color("var(--accent)"),
		display("flex"),
		alignItems("center"),
		gap("8px"),
		flexWrap("wrap"),
	)
	rule(".studio-status a",
		color("var(--accent)"),
		textDecoration("underline"),
	)
	rule(".studio-starter-row",
		display("flex"),
		flexWrap("wrap"),
		gap("8px"),
	)
	rule(".studio-starter",
		fontSize(".82rem"),
		padding(".36rem .8rem"),
		border("1px solid color-mix(in srgb, var(--accent) 22%, var(--border))"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color .12s, color .12s, background .12s"),
	)
	rule(".studio-starter:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".studio-starter.is-active",
		borderColor("var(--accent)"),
		color("var(--text)"),
		background("color-mix(in srgb, var(--accent) 14%, var(--bg-card))"),
	)
	rule(".studio-verb",
		fontSize(".72rem"),
		padding(".2rem .5rem"),
		border("1px solid var(--border)"),
		borderRadius("7px"),
		background("var(--bg-card)"),
		color("var(--text-dim)"),
		cursor("pointer"),
		fontFamily("var(--font-mono, monospace)"),
	)
	rule(".studio-verb:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".bento",
		display("grid"),
		customProp("--cell", "152px"),
		gridTemplateColumns("repeat(4, minmax(0,1fr))"),
		// No explicit row template: rows are created by content via auto-rows.
		// A fixed repeat(8, --cell) floor kept the grid 1286px tall even when a
		// focus preset filled only 4 rows, leaving ~740px of phantom scroll
		// below the last widget (QA task #45).
		gridAutoRows("var(--cell)"),
		gap("10px"),
		overflowAnchor("none"),
	)
	rule(".bento.bento-ledger",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
		customProp("--dt-sticky-top", "3.5rem"),
	)
	rule(".bento.bento-ledger > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	rule(".bento.bento-accounts",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-accounts > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	rule(".bento.bento-budgets",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-budgets > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// Deterministic budget-surface tile order. budget-summary (the income/To-Assign banner)
	// and budget-savings self-hide by rendering nothing until data loads / the method is
	// zero-based; a keyed child that renders empty first then fills can lose its DOM anchor
	// and get appended after the later tiles, so the income summary would sometimes land
	// below the budget cards. CSS grid auto-placement uses order-modified document order, so
	// pinning `order` fixes the visual sequence no matter when each tile's node arrives.
	rule(".bento-budgets > .w[data-widget=\"budget-summary\"]", order("1"))
	rule(".bento-budgets > .w[data-widget=\"budget-toolbar\"]", order("2"))
	rule(".bento-budgets > .w[data-widget=\"budget-list\"]", order("3"))
	rule(".bento-budgets > .w[data-widget=\"budget-savings\"]", order("4"))
	rule(".bento-budgets > .w[data-widget=\"budget-formula\"]", order("5"))
	// /goals is the same widgetized surface as /budgets: full-width tiles that size to
	// their content (not the fixed dashboard bento cells).
	rule(".bento.bento-goals",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-goals > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// Same deterministic-order fix as /budgets: goal-summary self-hides until goals load,
	// so pin the surface tile order so the summary can't land below the goal list on a
	// slow/racy first paint.
	rule(".bento-goals > .w[data-widget=\"goal-summary\"]", order("1"))
	rule(".bento-goals > .w[data-widget=\"goal-toolbar\"]", order("2"))
	rule(".bento-goals > .w[data-widget=\"goal-list\"]", order("3"))
	rule(".bento-goals > .w[data-widget=\"goal-formula\"]", order("4"))
	// /debt is the widgetized payoff-ladder surface: full-width tiles that size to their
	// content, the same layout family as /goals and /budgets.
	rule(".bento.bento-debt",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-debt > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// /investments is the same widgetized surface family.
	rule(".bento.bento-invest",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-invest > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// Summary hero (mirrors the debt hero).
	rule(".inv-hero",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("1rem"),
	)
	rule(".inv-hero-main",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	rule(".inv-hero-label",
		fontSize("0.72rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.08em"),
	)
	rule(".inv-hero-value",
		fontSize("2.6rem"),
		fontWeight("700"),
		prop("line-height", "1.05"),
		fontVariantNumeric("tabular-nums"),
		prop("text-shadow", "0 0 34px color-mix(in srgb, var(--accent) 22%, transparent)"),
	)
	rule(".inv-hero-sub",
		fontSize("0.9rem"),
		prop("margin-top", "0.1rem"),
	)
	rule(".inv-hero-main .debt-owner-link",
		prop("margin-top", "0.35rem"),
	)
	// Holding / traditional cards.
	rule(".inv-list",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".inv-card",
		position("relative"),
		display("flex"),
		alignItems("center"),
		width("100%"),
		gap("1rem"),
		padding("0.9rem 1.25rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		transition("transform 0.16s ease, border-color 0.16s ease"),
	)
	rule(".inv-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 34%, var(--border))"),
		transform("translateY(-1px)"),
	)
	rule(".inv-card-body",
		flex("1 1 auto"),
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
	)
	rule(".inv-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	rule(".inv-name",
		fontWeight("700"),
		fontSize("1rem"),
	)
	rule(".inv-ticker",
		fontFamily("ui-monospace, SFMono-Regular, Menlo, monospace"),
		fontSize("0.72rem"),
		fontWeight("700"),
		padding("0.05rem 0.4rem"),
		borderRadius("6px"),
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".inv-chip",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		fontSize("0.72rem"),
		fontWeight("600"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
	)
	rule(".inv-sec-badge",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.5rem"),
		borderRadius("999px"),
		fontSize("0.66rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
	)
	rule(".inv-sec-badge.inv-sec-stock",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".inv-sec-badge.inv-sec-crypto",
		borderColor("color-mix(in srgb, #f59e0b 45%, var(--border))"),
		color("color-mix(in srgb, #f59e0b 70%, var(--text))"),
	)
	rule(".inv-meta",
		fontSize("0.8rem"),
	)
	rule(".inv-weight",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".inv-weight-track",
		position("relative"),
		flex("1 1 auto"),
		height("6px"),
		borderRadius("999px"),
		overflow("hidden"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		maxWidth("220px"),
	)
	rule(".inv-weight-fill",
		height("100%"),
		borderRadius("999px"),
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 65%, #000), var(--accent))"),
	)
	rule(".inv-weight-label",
		fontSize("0.72rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-side",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-end"),
		gap("0.2rem"),
		prop("flex", "0 0 auto"),
	)
	rule(".inv-value",
		fontSize("1.2rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-gain",
		fontSize("0.85rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-gain-pct",
		fontSize("0.78rem"),
	)
	// Allocation columns.
	rule(".inv-alloc-cols",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(240px, 1fr))"),
		gap("1rem"),
	)
	rule(".inv-alloc-title",
		fontSize("0.7rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		marginBottom("0.4rem"),
	)
	rule(".inv-alloc-list",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".inv-alloc-head",
		display("flex"),
		justifyContent("space-between"),
		gap("0.5rem"),
		fontSize("0.82rem"),
		marginBottom("0.15rem"),
	)
	rule(".inv-alloc-label",
		fontWeight("600"),
	)
	rule(".inv-alloc-val",
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-alloc-track",
		height("8px"),
		borderRadius("999px"),
		overflow("hidden"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
	)
	rule(".inv-alloc-fill",
		height("100%"),
		borderRadius("999px"),
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 60%, #000), var(--accent))"),
	)
	rule(".inv-add",
		marginBottom("0.75rem"),
	)
	// Growth chart: a header with the current value + a toned delta, a segmented window
	// toggle on the right, and the area chart below.
	rule(".inv-growth",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".inv-growth-head",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.75rem"),
	)
	rule(".inv-growth-vals",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.6rem"),
	)
	rule(".inv-growth-now",
		fontSize("1.8rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-growth-delta",
		fontSize("0.9rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	// Pools bar: chips for each custom group, each showing its value + variable name.
	rule(".inv-pools-bar",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginBottom("0.9rem"),
		paddingBottom("0.9rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".inv-pools-bar-label",
		fontSize("0.68rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.07em"),
		prop("margin-right", "0.15rem"),
	)
	rule(".inv-pool-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.5rem"),
		padding("0.3rem 0.35rem 0.3rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--accent) 35%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".inv-pool-chip-main",
		display("inline-flex"),
		alignItems("baseline"),
		gap("0.35rem"),
	)
	rule(".inv-pool-chip-name",
		fontWeight("700"),
		fontSize("0.85rem"),
	)
	rule(".inv-pool-chip-val",
		fontSize("0.75rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-pool-var",
		fontFamily("ui-monospace, SFMono-Regular, Menlo, monospace"),
		fontSize("0.68rem"),
		padding("0.05rem 0.4rem"),
		borderRadius("6px"),
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	// Per-account card header: title row + a pool selector beneath it.
	rule(".inv-acct-head",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
	)
	rule(".inv-acct-view",
		prop("margin-left", "auto"),
	)
	// A custom-chart (pool) card is accent-outlined and carries a small "Chart" tag so it
	// reads as an aggregate distinct from the single-account cards beside it.
	rule(".inv-chart-card .inv-pool-card",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 6%, var(--bg-elev))"),
	)
	rule(".inv-chart-tag",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		fontSize("0.62rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		background("var(--accent)"),
		color("var(--bg)"),
	)
	rule(".inv-acct-head .field.inv-acct-pool, .inv-acct-head select",
		prop("align-self", "flex-start"),
		width("auto"),
		minWidth("8rem"),
		fontSize("0.78rem"),
		padding("0.2rem 0.5rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
	)
	rule(".inv-pool-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(320px, 1fr))"),
		gap("0.75rem"),
	)
	rule(".inv-pool-card",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		padding("0.9rem 1rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		transition("border-color 0.15s ease"),
	)
	rule(".inv-pool-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 30%, var(--border))"),
	)
	rule(".inv-pool-card-head",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	rule(".inv-pool-title-row",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".inv-pool-name",
		fontWeight("700"),
		fontSize("0.98rem"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".inv-pool-count",
		fontSize("0.72rem"),
	)
	rule(".inv-pool-actions",
		display("inline-flex"),
		gap("0.1rem"),
		prop("margin-left", "auto"),
	)
	rule(".inv-pool-figs",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".inv-pool-val",
		fontSize("1.35rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".inv-pool-delta",
		fontSize("0.8rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	// Create/edit-pool modal: a name field + a checkable list of accounts to include.
	rule(".inv-pool-modal-form",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".pool-acct-list-label",
		fontSize("0.72rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		marginTop("0.25rem"),
	)
	rule(".pool-acct-list",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		maxHeight("15rem"),
		overflowY("auto"),
	)
	rule(".pool-acct-toggle",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		width("100%"),
		padding("0.5rem 0.65rem"),
		borderRadius("10px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		prop("text-align", "left"),
		transition("border-color 0.15s ease, background 0.15s ease"),
	)
	rule(".pool-acct-toggle:hover",
		borderColor("color-mix(in srgb, var(--accent) 30%, var(--border))"),
	)
	rule(".pool-acct-toggle.is-checked",
		borderColor("color-mix(in srgb, var(--accent) 50%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 9%, transparent)"),
	)
	rule(".pool-acct-check",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		prop("flex", "0 0 auto"),
		width("1.3rem"),
		height("1.3rem"),
		borderRadius("6px"),
		border("1px solid var(--border)"),
		color("var(--bg)"),
	)
	rule(".pool-acct-toggle.is-checked .pool-acct-check",
		background("var(--accent)"),
		borderColor("var(--accent)"),
	)
	rule(".pool-acct-name",
		flex("1 1 auto"),
		minWidth("0"),
		fontWeight("600"),
		fontSize("0.9rem"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".pool-acct-toggle:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "1px"),
	)
	// ---- /allocate: widgetized "put money to work" surface ----
	// ---- /planning: widgetized runway + forecast + afford + scenarios surface ----
	rule(".bento.bento-planning",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-planning > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// The planning tiles reuse the debt section chrome + stat grid; give the lead figures a
	// touch more presence and keep the forms tidy on the wide bento column.
	rule(".bento-planning .stat-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(auto-fit, minmax(9rem, 1fr))"),
		prop("gap", "0.9rem"),
		prop("margin-bottom", "0.5rem"),
	)
	rule(".bento-planning .stat-value.is-hero",
		prop("font-size", "2rem"),
		prop("line-height", "1.05"),
	)
	// The runway Safe-to-spend hero sits above the secondary stat grid; its bottom margin
	// matches the grid gutter so the vertical gap equals the gap between the cards.
	rule(".plan-runway-hero",
		prop("margin-bottom", "0.9rem"),
	)
	rule(".bento-planning .form-grid",
		prop("margin-top", "0.6rem"),
	)
	// A single control after results (e.g. the runway buffer / forecast trim) shouldn't claim
	// the full grid width like the multi-input forms do — cap it so it reads as one field.
	rule(".plan-inline-field",
		prop("margin-top", "0.75rem"),
		prop("max-width", "14rem"),
	)
	rule(".plan-inline-field .field",
		prop("font-variant-numeric", "tabular-nums"),
	)
	// Saved what-if scenario cards.
	rule(".bento-planning .rows",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".plan-scenario",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("padding", "0.85rem 1rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "14px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		prop("transition", "border-color 0.15s ease"),
	)
	rule(".plan-scenario:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 30%, var(--border))"),
	)
	rule(".plan-scenario-head",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
	)
	rule(".plan-scenario-title",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("min-width", "0"),
		prop("flex", "1 1 auto"),
	)
	rule(".plan-scenario-name",
		prop("font-weight", "700"),
		prop("font-size", "1rem"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".plan-scenario-meta",
		prop("font-size", "0.8rem"),
	)
	rule(".plan-scenario-figs",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.2rem"),
		prop("flex", "0 0 auto"),
	)
	rule(".plan-scenario-menu",
		prop("flex", "0 0 auto"),
		prop("margin-top", "-0.15rem"),
	)
	rule(".plan-scenario-end",
		prop("font-size", "1.35rem"),
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("line-height", "1.05"),
	)
	rule(".plan-scenario-runway",
		prop("font-size", "0.72rem"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.25rem"),
	)
	rule(".plan-scenario-runway.is-danger",
		prop("padding", "0.05rem 0.45rem"),
		prop("border-radius", "999px"),
		prop("background", "color-mix(in srgb, var(--danger) 14%, transparent)"),
	)
	// Constrain the projected sparkline so it reads as a compact card chart (the AreaChart is a
	// full-width component that otherwise renders 120px tall and swamps the row).
	rule(".plan-scenario-chart",
		prop("width", "100%"),
	)
	rule(".plan-scenario-chart svg",
		prop("height", "52px"),
	)
	rule(".bento.bento-allocate",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-allocate > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Hero: the amount to put to work + the split figures.
	rule(".alloc-hero",
		prop("display", "flex"),
		prop("align-items", "flex-end"),
		prop("justify-content", "space-between"),
		prop("flex-wrap", "wrap"),
		prop("gap", "1.25rem"),
	)
	rule(".alloc-hero-main",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("min-width", "16rem"),
		prop("flex", "1 1 auto"),
	)
	rule(".alloc-hero-label",
		prop("font-size", "0.72rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.08em"),
	)
	// The amount field is the centrepiece: a serif figure with an accent underline, sized to sit
	// comfortably beside the figure chips (a short "0.00" placeholder — the label says the rest).
	rule(".alloc-amount-field",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.2rem"),
		prop("border-bottom", "2px solid color-mix(in srgb, var(--accent) 50%, var(--border))"),
		prop("padding", "0.1rem 0.15rem 0.3rem"),
		prop("width", "min(100%, 15rem)"),
		prop("transition", "border-color 0.16s ease"),
	)
	rule(".alloc-amount-field:focus-within",
		prop("border-bottom-color", "var(--accent)"),
	)
	// Zero-state: the amount hasn't been set yet, so mute the big figure — an inert
	// bright "$0.00" otherwise reads as "there's nothing to allocate" when the income
	// nudge below is offering money to put to work.
	rule(".alloc-amount-field.is-zero .alloc-amount-affix",
		prop("color", "var(--text-faint)"),
	)
	rule(".alloc-amount-field.is-zero .alloc-amount-input",
		prop("color", "var(--text-faint)"),
	)
	rule(".alloc-amount-affix",
		prop("font-size", "1.6rem"),
		prop("font-weight", "700"),
		prop("color", "var(--text-dim)"),
		prop("line-height", "1"),
		prop("flex", "0 0 auto"),
	)
	rule(".alloc-amount-input",
		prop("font-size", "2.4rem"),
		prop("font-weight", "700"),
		prop("line-height", "1.1"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("width", "100%"),
		prop("min-width", "0"),
		prop("border", "0"),
		prop("outline", "none"),
		prop("background", "transparent"),
		prop("color", "var(--text)"),
		prop("padding", "0"),
	)
	rule(".alloc-amount-input::placeholder",
		prop("color", "var(--text-faint)"),
		prop("opacity", "1"),
	)
	rule(".alloc-income-nudge",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.35rem"),
		prop("padding", "0.7rem 0.85rem"),
		prop("border-radius", "12px"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 32%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	rule(".alloc-kept",
		prop("margin", "0.35rem 0 0"),
		prop("font-size", "0.82rem"),
	)
	// Ranked destination cards.
	rule(".alloc-plan-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".alloc-dest",
		prop("position", "relative"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "1rem"),
		prop("padding", "0.9rem 1.1rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "14px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		prop("transition", "transform 0.16s ease, border-color 0.16s ease"),
	)
	rule(".alloc-dest:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 34%, var(--border))"),
		prop("transform", "translateY(-1px)"),
	)
	// The #1 destination gets an accent focus treatment so the order reads at a glance.
	rule(".alloc-dest.is-first",
		prop("border-color", "color-mix(in srgb, var(--accent) 50%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 7%, var(--bg-elev))"),
		prop("box-shadow", "0 0 26px -10px color-mix(in srgb, var(--accent) 45%, transparent)"),
	)
	rule(".alloc-dest-rank",
		prop("flex", "0 0 auto"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "2.1rem"),
		prop("height", "2.1rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("font-size", "1.1rem"),
		prop("font-weight", "700"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".alloc-dest.is-first .alloc-dest-rank",
		prop("background", "var(--accent)"),
		prop("border-color", "var(--accent)"),
		prop("color", "var(--bg)"),
	)
	rule(".alloc-dest-body",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.45rem"),
	)
	rule(".alloc-dest-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
	)
	rule(".alloc-dest-name",
		prop("font-weight", "700"),
		prop("font-size", "1rem"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".alloc-dest-figs",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
		prop("flex", "0 0 auto"),
	)
	rule(".alloc-dest-amount",
		prop("font-size", "1.1rem"),
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "color-mix(in srgb, var(--accent) 55%, var(--text))"),
	)
	rule(".alloc-dest-score",
		prop("font-size", "0.8rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".alloc-dest-breakdown",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
	)
	rule(".alloc-dest-chip",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.25rem"),
		prop("padding", "0.05rem 0.45rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("font-size", "0.72rem"),
	)
	rule(".alloc-dest-chip-label",
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.04em"),
		prop("font-size", "0.64rem"),
	)
	rule(".alloc-dest-chip-val",
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".alloc-dest-tag",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("padding", "0.05rem 0.45rem"),
		prop("border-radius", "999px"),
		prop("font-size", "0.66rem"),
		prop("font-weight", "700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		prop("color", "color-mix(in srgb, var(--accent) 60%, var(--text))"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".alloc-dest-menu",
		prop("margin-left", "auto"),
		prop("flex", "0 0 auto"),
	)
	// Excluded / restore.
	rule(".alloc-excluded",
		prop("margin-top", "0.9rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
	)
	rule(".alloc-excluded-label",
		prop("font-size", "0.68rem"),
		prop("font-weight", "700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.07em"),
	)
	rule(".alloc-excluded-list",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.4rem"),
	)
	rule(".alloc-excluded-chip",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("padding", "0.25rem 0.35rem 0.25rem 0.7rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("background", "var(--bg-elev)"),
		prop("font-size", "0.8rem"),
	)
	rule(".alloc-hidden-note, .alloc-apply-hint",
		prop("margin-top", "0.75rem"),
		prop("font-size", "0.82rem"),
	)
	// Advanced weight tuning.
	rule(".alloc-weights",
		prop("margin-top", "0.75rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("padding-top", "0.75rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".alloc-weights-label",
		prop("font-size", "0.7rem"),
		prop("font-weight", "700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
	)
	rule(".alloc-weights-grid",
		prop("grid-template-columns", "repeat(auto-fit, minmax(6rem, 1fr))"),
	)
	rule(".alloc-save-profile",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.25rem"),
	)
	// Why / AI.
	rule(".alloc-algo",
		prop("margin", "0 0 0.6rem"),
		prop("font-size", "0.95rem"),
	)
	rule(".alloc-ai",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	rule(".alloc-ai-result",
		prop("margin", "0"),
		prop("padding", "0.75rem 0.9rem"),
		prop("border-radius", "12px"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 28%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 6%, transparent)"),
		prop("line-height", "1.55"),
	)
	// Apply / confirm.
	rule(".alloc-confirm",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	rule(".alloc-confirm-rows",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("padding", "0.6rem 0.8rem"),
		prop("border-radius", "12px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		prop("border", "1px solid var(--border)"),
	)
	// Strategy summary chips (read-only; the modal edits them).
	rule(".alloc-strategy-chips",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("margin", "0.25rem 0 0.75rem"),
	)
	rule(".alloc-strategy-chip",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.4rem"),
		prop("padding", "0.35rem 0.7rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	rule(".alloc-strategy-chip-label",
		prop("font-size", "0.66rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
	)
	rule(".alloc-strategy-chip-val",
		prop("font-weight", "700"),
		prop("font-size", "0.88rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// Strategy flip-modal body.
	rule(".alloc-profile-form",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
	)
	// Summary hero: the total owed in the display serif beside the engine ratio chips.
	rule(".debt-hero",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("1rem"),
	)
	rule(".debt-hero-main",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	rule(".debt-hero-label",
		fontSize("0.72rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.08em"),
	)
	rule(".debt-hero-value",
		fontSize("2.6rem"),
		fontWeight("700"),
		prop("line-height", "1.05"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".debt-hero-value.neg",
		color("color-mix(in srgb, var(--danger) 78%, var(--text))"),
	)
	rule(".debt-hero-sub",
		fontSize("0.95rem"),
		color("var(--text-dim)"),
		prop("margin-top", "0.15rem"),
	)
	// Engine ratio chips.
	rule(".debt-chips",
		display("flex"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".debt-stat",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("104px"),
		padding("0.5rem 0.75rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
	)
	rule(".debt-stat-label",
		fontSize("0.7rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
	)
	rule(".debt-stat-value",
		fontSize("1.15rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	// Band tinting — shared by summary stats and debt cards. Good = seagreen accent,
	// warn = amber, high = danger. The thresholds live in DebtConfig, not here.
	rule(".debt-stat.debt-band-warn",
		borderColor("color-mix(in srgb, #f59e0b 55%, var(--border))"),
	)
	rule(".debt-stat.debt-band-high",
		borderColor("color-mix(in srgb, var(--danger) 60%, var(--border))"),
		background("color-mix(in srgb, var(--danger) 10%, var(--bg-elev))"),
	)
	// The payoff ladder: full-width rows stacked in payoff order (a real ladder), so the
	// sequence reads top-to-bottom.
	rule(".debt-list",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".debt-card",
		position("relative"),
		display("flex"),
		alignItems("center"),
		width("100%"),
		gap("1rem"),
		padding("0.9rem 1.25rem 0.9rem 1.35rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		transition("transform 0.18s ease, border-color 0.18s ease, background 0.18s ease"),
	)
	rule(".debt-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 34%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 85%, transparent)"),
		transform("translateY(-1px)"),
	)
	rule(".debt-card.is-excluded",
		prop("opacity", "0.62"),
		prop("border-style", "dashed"),
	)
	// The APR/utilization-banded left rail — the at-a-glance "how hot is this debt" cue.
	rule(".debt-rail",
		position("absolute"),
		left("0"),
		top("0"),
		bottom("0"),
		width("5px"),
		prop("border-top-left-radius", "14px"),
		prop("border-bottom-left-radius", "14px"),
		background("var(--accent)"),
	)
	rule(".debt-card.debt-band-warn .debt-rail",
		background("#f59e0b"),
	)
	rule(".debt-card.debt-band-high .debt-rail",
		background("var(--danger)"),
	)
	// Payoff-rank medallion — the ladder position, emphasized: a large serif numeral so the
	// order (1, 2, 3 …) is the first thing you read down the ladder.
	rule(".debt-rank",
		prop("flex", "0 0 auto"),
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		width("2.7rem"),
		height("2.7rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--accent) 30%, var(--border))"),
		background("var(--bg)"),
		fontFamily("var(--font-display, \"Fraunces\", serif)"),
		fontWeight("700"),
		fontSize("1.25rem"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".debt-card.debt-band-high .debt-rank",
		borderColor("color-mix(in srgb, var(--danger) 55%, var(--border))"),
		color("color-mix(in srgb, var(--danger) 78%, var(--text))"),
	)
	rule(".debt-card.is-excluded .debt-rank",
		borderColor("var(--border)"),
		color("var(--text-dim)"),
	)
	// Focus treatment for the #1 debt — the one to attack first.
	rule(".debt-card.is-focus",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 9%, var(--bg-elev))"),
	)
	rule(".debt-card.is-focus .debt-rank",
		background("var(--accent)"),
		borderColor("var(--accent)"),
		color("var(--bg)"),
		fontWeight("800"),
	)
	rule(".debt-focus-tag",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.5rem"),
		borderRadius("999px"),
		fontSize("0.66rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		background("var(--accent)"),
		color("var(--bg)"),
	)
	rule(".debt-body",
		flex("1 1 auto"),
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
	)
	rule(".debt-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	rule(".debt-name",
		fontWeight("700"),
		fontSize("1rem"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		prop("max-width", "100%"),
	)
	rule(".debt-chip",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		fontSize("0.72rem"),
		fontWeight("600"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
	)
	rule(".debt-chip.debt-apr",
		borderColor("color-mix(in srgb, var(--accent) 40%, var(--border))"),
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
	)
	// Utilization meter.
	rule(".debt-util",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".debt-util-track",
		position("relative"),
		height("8px"),
		borderRadius("999px"),
		overflow("hidden"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
	)
	rule(".debt-util-fill",
		height("100%"),
		borderRadius("999px"),
		background("var(--accent)"),
		transition("width 0.25s ease"),
	)
	rule(".debt-util-fill.debt-util-warn",
		background("#f59e0b"),
	)
	rule(".debt-util-fill.debt-util-high",
		background("var(--danger)"),
	)
	rule(".debt-util-label",
		fontSize("0.72rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".debt-meta",
		fontSize("0.78rem"),
	)
	rule(".debt-side",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-end"),
		gap("0.4rem"),
		prop("flex", "0 0 auto"),
	)
	rule(".debt-owed",
		fontSize("1.2rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		color("color-mix(in srgb, var(--danger) 72%, var(--text))"),
	)
	rule(".debt-actions",
		display("flex"),
		flexWrap("wrap"),
		justifyContent("flex-end"),
		gap("0.35rem"),
	)
	// Cohesion pass — lift the reused strategy / credit / loans / payoff-calculator panels
	// into the debt page's visual language. Scoped to .bento-debt. The debt tiles drop the
	// redundant EntityListSection frame (see debtSection) so the only .card left is a
	// panel's own grouping card (the credit hero ring, the per-card breakdown, …); style
	// those as soft grouping cards — elevated surface, hairline border, no heavy shadow.
	rule(".bento-debt .card",
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		padding("0.95rem 1.1rem"),
		marginBottom("0"),
		prop("box-shadow", "none"),
	)
	rule(".bento-debt .card:hover",
		transform("none"),
		prop("box-shadow", "none"),
	)
	// A debt tile's section title (serif) + a comfortable gap above the body.
	rule(".debt-section-title",
		fontFamily("var(--font-display, \"Fraunces\", serif)"),
		fontWeight("700"),
		fontSize("1.05rem"),
		letterSpacing("-0.01em"),
		marginBottom("0.15rem"),
	)
	// Section header: the serif title on the left, the owning-page link on the right.
	rule(".debt-section-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem 0.9rem"),
	)
	// Owning-page link — a quiet "manage this on its source page" anchor.
	rule(".debt-owner-link",
		display("inline-flex"),
		alignItems("center"),
		gap("0.15rem"),
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		prop("text-decoration", "none"),
		prop("white-space", "nowrap"),
		transition("color 0.15s ease"),
	)
	rule(".debt-owner-link:hover",
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
	)
	rule(".debt-hero-main .debt-owner-link",
		prop("margin-top", "0.35rem"),
	)
	rule(".debt-owner-link svg",
		transition("transform 0.15s ease"),
	)
	rule(".debt-owner-link:hover svg",
		transform("translateX(2px)"),
	)
	rule(".debt-section > * + *",
		prop("margin-top", "0.75rem"),
	)

	// ---- A little life: depth, accent moments, and orchestrated motion (design pass) ----
	// Signature detail — a seagreen tick before every section title, tying the page together.
	rule(".debt-section-title, .bento-debt .card-title",
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
	)
	// Hero: a soft glow under the total-owed figure gives the number presence + atmosphere.
	rule(".debt-hero-value.neg",
		prop("text-shadow", "0 0 34px color-mix(in srgb, var(--danger) 30%, transparent)"),
	)
	// Ratio chips: a faint top-lit gradient for depth + a lift on hover so they feel alive.
	rule(".debt-stat",
		prop("background", "linear-gradient(180deg, color-mix(in srgb, var(--bg-elev) 60%, transparent), color-mix(in srgb, var(--bg-elev) 35%, transparent))"),
		transition("transform 0.15s ease, border-color 0.15s ease"),
	)
	rule(".debt-stat:hover",
		transform("translateY(-2px)"),
		borderColor("color-mix(in srgb, var(--accent) 35%, var(--border))"),
	)
	// Utilization meters read as lit bars (gradient) rather than flat fills.
	rule(".debt-util-fill",
		prop("background", "linear-gradient(90deg, color-mix(in srgb, var(--accent) 65%, #000) 0%, var(--accent) 100%)"),
	)
	rule(".debt-util-fill.debt-util-warn",
		prop("background", "linear-gradient(90deg, color-mix(in srgb, #f59e0b 65%, #000) 0%, #f59e0b 100%)"),
	)
	rule(".debt-util-fill.debt-util-high",
		prop("background", "linear-gradient(90deg, color-mix(in srgb, var(--danger) 65%, #000) 0%, var(--danger) 100%)"),
	)
	// The APR/utilization rail fades along its length for a softer, more crafted edge.
	rule(".debt-rail",
		prop("background", "linear-gradient(180deg, var(--accent), color-mix(in srgb, var(--accent) 55%, transparent))"),
	)
	rule(".debt-card.debt-band-warn .debt-rail",
		prop("background", "linear-gradient(180deg, #f59e0b, color-mix(in srgb, #f59e0b 55%, transparent))"),
	)
	rule(".debt-card.debt-band-high .debt-rail",
		prop("background", "linear-gradient(180deg, var(--danger), color-mix(in srgb, var(--danger) 55%, transparent))"),
	)
	// Focus (the debt to pay first) + the recommended strategy get a soft accent glow.
	rule(".debt-card.is-focus",
		boxShadow("0 0 0 1px color-mix(in srgb, var(--accent) 25%, transparent), 0 10px 34px -14px color-mix(in srgb, var(--accent) 55%, transparent)"),
	)
	rule(".strat-card.is-winner",
		boxShadow("0 10px 34px -16px color-mix(in srgb, var(--accent) 55%, transparent)"),
	)
	rule(".debt-card.is-focus .debt-rank",
		boxShadow("0 0 16px -2px color-mix(in srgb, var(--accent) 65%, transparent)"),
	)
	// Strategy comparison cards lift on hover.
	rule(".strat-card",
		transition("transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease"),
	)
	rule(".strat-card:hover",
		transform("translateY(-2px)"),
	)
	// A gentle pulse ring on the "Pay first" / "Recommended" badges draws the eye to the
	// single most important call to action without shouting.
	keyframes("debt-badge-pulse",
		at("0%", boxShadow("0 0 0 0 color-mix(in srgb, var(--accent) 55%, transparent)")),
		at("70%", boxShadow("0 0 0 6px color-mix(in srgb, var(--accent) 0%, transparent)")),
		at("100%", boxShadow("0 0 0 0 color-mix(in srgb, var(--accent) 0%, transparent)")),
	)
	rule(".debt-focus-tag, .strat-badge",
		animation("debt-badge-pulse 2.6s ease-out infinite"),
	)
	// Cards reveal in a quick staggered cascade down the ladder on load.
	keyframes("debt-card-in",
		at("from", opacity("0"), transform("translateY(9px)")),
		at("to", opacity("1"), transform("none")),
	)
	rule(".debt-list .debt-card",
		animation("debt-card-in 0.42s var(--wonder-ease-out, cubic-bezier(0.22,1,0.36,1)) both"),
	)
	rule(".debt-list .debt-card:nth-child(2)", prop("animation-delay", "0.05s"))
	rule(".debt-list .debt-card:nth-child(3)", prop("animation-delay", "0.1s"))
	rule(".debt-list .debt-card:nth-child(4)", prop("animation-delay", "0.15s"))
	rule(".debt-list .debt-card:nth-child(5)", prop("animation-delay", "0.2s"))
	rule(".debt-list .debt-card:nth-child(6)", prop("animation-delay", "0.25s"))
	rule(".debt-list .debt-card:nth-child(n+7)", prop("animation-delay", "0.3s"))
	// Respect users who prefer less motion: no cascade, no badge pulse.
	ruleMedia("(prefers-reduced-motion: reduce)", ".debt-list .debt-card",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".debt-focus-tag, .strat-badge",
		animation("none"),
	)
	rule(".bento-debt .card-title, .bento-debt h2, .bento-debt h3",
		fontFamily("var(--font-display, \"Fraunces\", serif)"),
		fontWeight("700"),
		letterSpacing("-0.01em"),
	)
	rule(".bento-debt .muted, .bento-debt .budget-sub",
		prop("max-width", "72ch"),
		prop("line-height", "1.5"),
	)
	rule(".bento-debt .stat-grid",
		gap("0.6rem"),
		marginBottom("0.75rem"),
		gridTemplateColumns("repeat(auto-fit, minmax(150px, 1fr))"),
	)
	rule(".bento-debt .stat",
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		padding("0.7rem 0.9rem"),
	)
	rule(".bento-debt .stat-value",
		fontFamily("var(--font-display, \"Fraunces\", serif)"),
		fontSize("1.5rem"),
		fontVariantNumeric("tabular-nums"),
		prop("margin-top", "0.15rem"),
	)
	// Inputs: keep single-field forms from stretching edge-to-edge; give fields the elevated
	// surface treatment so they don't read as bare boxes.
	rule(".bento-debt .form-grid",
		gridTemplateColumns("repeat(auto-fit, minmax(180px, 280px))"),
		justifyContent("start"),
		gap("0.6rem"),
	)
	rule(".bento-debt .field",
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
	)
	rule(".bento-debt .field:focus",
		borderColor("var(--accent)"),
		prop("outline", "none"),
	)
	// Progress / burn-down bars match the utilization meter.
	rule(".bento-debt .bar",
		height("10px"),
		borderRadius("999px"),
		overflow("hidden"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
	)
	rule(".bento-debt .bar-fill",
		background("var(--accent)"),
		borderRadius("999px"),
	)
	// Tables: quiet uppercase header, tabular figures, hairline rows.
	rule(".bento-debt .t-body th",
		fontSize("0.72rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		color("var(--text-dim)"),
		fontWeight("600"),
	)
	rule(".bento-debt .t-body td",
		fontVariantNumeric("tabular-nums"),
	)
	// Grouping cards inside a debt tile get vertical rhythm when stacked.
	rule(".bento-debt .card + .card",
		prop("margin-top", "0.75rem"),
	)
	// Credit-health demerits ("what's holding you back") + advice ("how to improve") lists:
	// scannable rows with a leading factor icon and a point-impact chip on the right.
	rule(".credit-list",
		display("flex"),
		flexDirection("column"),
		gap("0.55rem"),
	)
	rule(".credit-item",
		display("flex"),
		alignItems("flex-start"),
		gap("0.55rem"),
	)
	rule(".credit-item-icon",
		display("inline-flex"),
		prop("flex", "0 0 auto"),
		marginTop("0.1rem"),
	)
	rule(".credit-item-icon.is-down",
		color("color-mix(in srgb, #f59e0b 70%, var(--text))"),
	)
	rule(".credit-item-icon.is-up",
		color("var(--accent)"),
	)
	rule(".credit-item-text",
		flex("1 1 auto"),
		minWidth("0"),
		fontSize("0.9rem"),
		prop("line-height", "1.45"),
	)
	rule(".credit-pts",
		prop("flex", "0 0 auto"),
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.5rem"),
		borderRadius("999px"),
		fontSize("0.72rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		border("1px solid var(--border)"),
	)
	rule(".credit-pts-down",
		color("color-mix(in srgb, var(--danger) 78%, var(--text))"),
		borderColor("color-mix(in srgb, var(--danger) 40%, var(--border))"),
		background("color-mix(in srgb, var(--danger) 8%, transparent)"),
	)
	rule(".credit-pts-up",
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
		borderColor("color-mix(in srgb, var(--accent) 40%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
	)
	// Global "back to top" floating button — hidden until #main scrolls down, then fades in
	// at the bottom-right and smooth-scrolls to the top on click.
	rule(".cf-scrolltop",
		position("fixed"),
		prop("right", "1.25rem"),
		prop("bottom", "1.25rem"),
		prop("z-index", "45"),
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		width("2.75rem"),
		height("2.75rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--accent) 40%, var(--border))"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		cursor("pointer"),
		prop("opacity", "0"),
		prop("pointer-events", "none"),
		transform("translateY(12px)"),
		boxShadow("0 8px 22px -8px rgba(0,0,0,0.6)"),
		transition("opacity 0.2s ease, transform 0.2s ease, background 0.15s ease, border-color 0.15s ease, color 0.15s ease"),
	)
	rule(".cf-scrolltop.is-visible",
		prop("opacity", "1"),
		prop("pointer-events", "auto"),
		transform("translateY(0)"),
	)
	rule(".cf-scrolltop:hover",
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 14%, var(--bg-elev))"),
		color("color-mix(in srgb, var(--accent) 55%, var(--text))"),
	)
	rule(".cf-scrolltop:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	// Section anchors leave room under the sticky topbar when a jump-nav link scrolls to
	// them (block:start would otherwise tuck the heading right against the top edge).
	rule("#sec-overview, #sec-ladder, #sec-strategy, #sec-credit, #sec-loans, #sec-calculator",
		prop("scroll-margin-top", "1.25rem"),
	)
	// Jump-nav: a row of quick section links in the toolbar so a user can skip to a widget.
	rule(".debt-jump",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem"),
		marginBottom("0.7rem"),
		prop("padding-bottom", "0.7rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".debt-jump-label",
		fontSize("0.68rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.07em"),
		color("var(--text-dim)"),
		prop("margin-right", "0.15rem"),
	)
	rule(".debt-jump-link",
		display("inline-flex"),
		alignItems("center"),
		padding("0.2rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		color("var(--text)"),
		fontSize("0.8rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background 0.15s ease, border-color 0.15s ease, color 0.15s ease"),
	)
	rule(".debt-jump-link:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 12%, var(--bg-elev))"),
		color("color-mix(in srgb, var(--accent) 45%, var(--text))"),
	)
	rule(".debt-jump-link:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	// Strategy panel: the extra-payment control row (input + one-tap suggestion).
	rule(".strat-extra",
		display("flex"),
		alignItems("flex-end"),
		flexWrap("wrap"),
		gap("0.6rem"),
		marginBottom("0.9rem"),
	)
	rule(".strat-extra .field",
		width("14rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
	)
	// Snowball vs avalanche — a clear two-up decision with the winner badged.
	rule(".strat-compare",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(220px, 1fr))"),
		gap("0.75rem"),
	)
	rule(".strat-card",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		padding("1rem 1.15rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	rule(".strat-card.is-winner",
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 9%, var(--bg-elev))"),
	)
	rule(".strat-card-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".strat-card-name",
		fontSize("0.72rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.07em"),
		color("var(--text-dim)"),
	)
	rule(".strat-badge",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.5rem"),
		borderRadius("999px"),
		fontSize("0.62rem"),
		fontWeight("700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		background("var(--accent)"),
		color("var(--bg)"),
	)
	rule(".strat-card-months",
		fontSize("1.85rem"),
		fontWeight("700"),
		prop("line-height", "1.1"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".strat-card-stats",
		display("flex"),
		flexWrap("wrap"),
		gap("0.25rem 1.25rem"),
		prop("margin-top", "0.25rem"),
	)
	rule(".strat-card-stat",
		display("flex"),
		flexDirection("column"),
	)
	rule(".strat-card-stat-label",
		fontSize("0.66rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
	)
	rule(".strat-card-stat-value",
		fontSize("0.98rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
	)
	// Payoff order — a readable sequence line, not buried in prose.
	rule(".strat-order",
		prop("margin-top", "0.5rem"),
	)
	rule(".strat-order-label",
		fontSize("0.7rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
	)
	rule(".strat-order-seq",
		prop("margin", "0.15rem 0 0"),
		prop("line-height", "1.6"),
		color("var(--text)"),
	)
	// Per-card utilization rows read as separated line-items inside the breakdown card,
	// and the inline credit-limit editor input no longer sprawls edge-to-edge.
	rule(".bento-debt .credit-card-item + .credit-card-item",
		prop("margin-top", "0.9rem"),
		prop("padding-top", "0.9rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".bento-debt .credit-card-item .field, .bento-debt .credit-card-item input",
		prop("max-width", "13rem"),
	)
	rule(".bento-debt .credit-card-item .labeled-field",
		prop("flex-direction", "row"),
		alignItems("center"),
		gap("0.5rem"),
	)
	// To-do surface: same full-width, auto-height stacked-tile layout as /goals so the
	// summary / toolbar / list tiles flow top-to-bottom instead of into a fixed grid.
	rule(".bento.bento-todo",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-todo > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// Notifications surface: same full-width, auto-height stacked-tile layout as /todo.
	rule(".bento.bento-notif",
		gridTemplateRows("auto"),
		gridAutoRows("auto"),
	)
	rule(".bento.bento-notif > .w",
		height("auto"),
		minHeight("0"),
		overflow("visible"),
	)
	// --- Notifications "signal feed" -------------------------------------------------
	// Summary tile: a Fraunces hero count + a severity breakdown + the catch-up line.
	rule(".notif-summary",
		display("flex"),
		alignItems("center"),
		gap("1.5rem"),
		flexWrap("wrap"),
	)
	rule(".notif-summary-lead",
		display("flex"),
		alignItems("baseline"),
		gap("0.65rem"),
	)
	rule(".notif-summary-count",
		fontFamily("var(--font-display), 'Fraunces', Georgia, serif"),
		fontSize("2.3rem"),
		fontWeight("600"),
		lineHeight("1"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".notif-summary-label",
		display("flex"),
		flexDirection("column"),
	)
	rule(".notif-summary-word",
		fontSize("0.95rem"),
		color("var(--text)"),
	)
	rule(".notif-summary-sub",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)
	rule(".notif-summary-sevs",
		display("flex"),
		gap("0.5rem"),
		flexWrap("wrap"),
		marginLeft("auto"),
	)
	rule(".notif-sev-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		padding("0.3rem 0.65rem"),
		borderRadius("999px"),
		background("var(--bg)"),
		border("1px solid var(--border)"),
		fontSize("0.8rem"),
	)
	rule(".notif-sev-dot",
		width("9px"),
		height("9px"),
		borderRadius("50%"),
		flex("none"),
	)
	rule(".notif-sev-chip.sev-critical .notif-sev-dot",
		background("#ef4444"),
	)
	rule(".notif-sev-chip.sev-warning .notif-sev-dot",
		background("#f59e0b"),
	)
	rule(".notif-sev-chip.sev-info .notif-sev-dot",
		background("var(--accent)"),
	)
	rule(".notif-sev-n",
		fontWeight("600"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".notif-sev-name",
		color("var(--text-dim)"),
	)
	rule(".notif-catchup",
		display("flex"),
		alignItems("center"),
		gap("0.45rem"),
		flexBasis("100%"),
		marginTop("0.2rem"),
		fontSize("0.82rem"),
		color("var(--accent)"),
	)
	rule(".notif-catchup-dot",
		width("7px"),
		height("7px"),
		borderRadius("50%"),
		background("var(--accent)"),
		flex("none"),
	)
	// Feed list + cards. A severity-tinted left rail + a colored icon medallion; unread
	// items carry a dot and full weight, read items dim.
	rule(".notif-list",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".notif",
		display("flex"),
		alignItems("flex-start"),
		gap("0.6rem"),
		padding("0.8rem 0.9rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderLeft("3px solid var(--border-strong)"),
		borderRadius("12px"),
		transition("border-color 0.12s ease, opacity 0.12s ease"),
	)
	rule(".notif:hover",
		borderColor("var(--border-strong)"),
	)
	// The badge + body is a single clickable region that navigates to the alerting
	// resource; the title lights + a chevron appears on hover when it's linked.
	rule(".notif-main",
		flex("1"),
		minWidth("0"),
		display("flex"),
		alignItems("flex-start"),
		gap("0.85rem"),
	)
	rule(".notif-main.is-linked",
		cursor("pointer"),
		prop("outline", "none"),
	)
	rule(".notif-main.is-linked:hover .notif-title",
		color("var(--accent)"),
	)
	rule(".notif-go",
		flex("none"),
		marginLeft("auto"),
		color("var(--text-faint)"),
		transition("color 0.12s ease, transform 0.12s ease"),
	)
	rule(".notif-main.is-linked:hover .notif-go",
		color("var(--accent)"),
		transform("translateX(2px)"),
	)
	rule(".notif-main.is-linked:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
		borderRadius("8px"),
	)
	// 2026-07-17 audit: read ≠ resolved. A gentler dim for read rows, and a
	// critical alert keeps nearly full ink even after it's been seen — "read"
	// must never file away unresolved urgency.
	rule(".notif.is-read",
		opacity("0.78"),
	)
	rule(".notif.is-read.sev-critical",
		opacity("0.95"),
	)
	rule(".notif.sev-critical",
		borderLeftColor("#ef4444"),
	)
	rule(".notif.sev-warning",
		borderLeftColor("#f59e0b"),
	)
	rule(".notif.sev-info",
		borderLeftColor("var(--accent)"),
	)
	rule(".notif-badge",
		flex("none"),
		width("34px"),
		height("34px"),
		borderRadius("50%"),
		display("grid"),
		placeItems("center"),
		marginTop("0.05rem"),
	)
	rule(".notif.sev-critical .notif-badge",
		background("#3b0d0d"),
		color("#fca5a5"),
	)
	rule(".notif.sev-warning .notif-badge",
		background("#2a230c"),
		color("#fcd34d"),
	)
	rule(".notif.sev-info .notif-badge",
		background("var(--accent-dim)"),
		color("var(--accent)"),
	)
	rule(".notif-body",
		flex("1"),
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
	)
	rule(".notif-top",
		display("flex"),
		alignItems("center"),
		gap("0.45rem"),
	)
	rule(".notif-dot",
		flex("none"),
		width("8px"),
		height("8px"),
		borderRadius("50%"),
		background("var(--accent)"),
	)
	rule(".notif.sev-critical .notif-dot",
		background("#ef4444"),
	)
	rule(".notif.sev-warning .notif-dot",
		background("#f59e0b"),
	)
	rule(".notif-title",
		fontWeight("600"),
		fontSize("0.95rem"),
		color("var(--text)"),
		overflowWrap("anywhere"),
	)
	rule(".notif.is-read .notif-title",
		fontWeight("500"),
	)
	rule(".notif-text",
		margin("0"),
		fontSize("0.85rem"),
		lineHeight("1.45"),
		color("var(--text-dim)"),
		overflowWrap("anywhere"),
	)
	rule(".notif-foot",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		fontSize("0.76rem"),
		color("var(--text-faint)"),
	)
	rule(".notif-sev-tag",
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.04em"),
		prop("text-transform", "uppercase"),
	)
	rule(".notif-sev-tag.sev-critical",
		color("#fca5a5"),
	)
	rule(".notif-sev-tag.sev-warning",
		color("#fcd34d"),
	)
	rule(".notif-sev-tag.sev-info",
		color("var(--accent)"),
	)
	rule(".notif-time",
		fontVariantNumeric("tabular-nums"),
	)
	rule(".notif-clear:hover",
		borderColor("#7f1d1d"),
		color("#fca5a5"),
	)
	// Inline per-item actions (mark-read / snooze / dismiss) — always visible + one click
	// each; no ⋯ menu (a menu is an extra click for actions you take constantly). Faint by
	// default, they brighten on hover.
	rule(".notif-actions",
		flex("none"),
		alignSelf("flex-start"),
		display("flex"),
		alignItems("center"),
		gap("0.1rem"),
		marginTop("0.05rem"),
	)
	rule(".notif-icon-btn",
		background("transparent"),
		// v1.0: the resting affordance was invisible (border blended into the
		// card, glyph too faint). Give the icon full text contrast and the
		// border a perceptible presence; hover still brightens the background.
		border("1px solid var(--text-dim)"),
		color("var(--text)"),
		cursor("pointer"),
		padding("0.35rem"),
		borderRadius("8px"),
		display("grid"),
		placeItems("center"),
		transition("background 0.12s ease, color 0.12s ease, border-color 0.12s ease"),
	)
	rule(".notif-icon-btn:hover",
		background("var(--bg)"),
		color("var(--text)"),
	)
	rule(".notif-icon-btn.notif-dismiss:hover",
		background("#3b0d0d"),
		color("#fca5a5"),
	)
	// To-do line items — an EDITORIAL AGENDA, not cards. Borderless rows on a hairline
	// rhythm: a circular check-off ring whose COLOUR encodes priority (red high / accent
	// medium / faint low — so you scan urgency by the rings, no badges), the title as the
	// hero, a quiet right-aligned due date (red overdue / amber today), and a single dim
	// secondary line for repeat / linked entity / notes. A linked goal is the one accent
	// note in that line. Actions fade in on hover. Scoped to /todo so the shared .row
	// style elsewhere is untouched.
	rule(".bento-todo .rows",
		display("flex"),
		flexDirection("column"),
	)
	rule(".todo-item",
		display("flex"),
		alignItems("flex-start"),
		gap("0.85rem"),
		padding("0.8rem 0.5rem"),
		borderBottom("1px solid var(--border)"),
		borderRadius("8px"),
		transition("background 0.12s ease"),
	)
	rule(".todo-item:last-child",
		borderBottom("0"),
	)
	rule(".todo-item:hover",
		background("var(--bg-elev)"),
	)
	rule(".todo-item.is-done",
		opacity("0.5"),
	)
	// A nested sub-task reads clearly as a child: indented (inline margin-left by depth),
	// with a left guide rail, a leading ↳ connector, a smaller check ring, and a slightly
	// smaller/dimmer title.
	rule(".todo-item.is-subtask",
		borderLeft("2px solid var(--accent-dim)"),
		background("rgba(255,255,255,0.02)"),
		gap("0.6rem"),
	)
	rule(".todo-item.is-subtask:hover",
		background("var(--bg-elev)"),
	)
	rule(".todo-subarrow",
		flex("none"),
		alignSelf("center"),
		color("var(--text-faint)"),
		fontSize("1.05rem"),
		lineHeight("1"),
		marginRight("-0.15rem"),
	)
	// Disclosure chevron on parent rows (collapse/expand sub-tasks) + an equal-width
	// spacer on leaves so every checkbox lines up. The chevron rotates down when open.
	// Drag handle for "Custom order" reordering — a dim grip that brightens on hover; the
	// row it leads is the drop target.
	rule(".todo-grip",
		flex("none"),
		alignSelf("center"),
		display("grid"),
		placeItems("center"),
		width("20px"),
		height("22px"),
		marginRight("-0.15rem"),
		color("var(--text-faint)"),
		cursor("grab"),
	)
	rule(".todo-grip:hover",
		color("var(--text)"),
	)
	rule(".todo-grip:active",
		cursor("grabbing"),
	)
	rule(".bento-todo .todo-item[data-testid]:has(.todo-grip)",
		prop("scroll-margin-top", "80px"),
	)
	rule(".todo-disclose",
		flex("none"),
		alignSelf("center"),
		display("grid"),
		placeItems("center"),
		width("22px"),
		height("22px"),
		marginTop("0.05rem"),
		marginLeft("-0.35rem"),
		padding("0"),
		background("transparent"),
		border("0"),
		borderRadius("6px"),
		color("var(--text-dim)"),
		cursor("pointer"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".todo-disclose:hover",
		background("var(--bg)"),
		color("var(--text)"),
	)
	rule(".todo-disclose svg",
		transition("transform 0.15s ease"),
		transform("rotate(90deg)"),
	)
	rule(".todo-disclose.is-collapsed svg",
		transform("rotate(0deg)"),
	)
	rule(".todo-disclose-spacer",
		flex("none"),
		width("22px"),
		marginLeft("-0.35rem"),
	)
	// Sub-task summary chip on a parent ("2/3"): a quiet pill so a collapsed parent still
	// shows there's hidden work under it.
	rule(".todo-substat",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
	)
	rule(".todo-item.is-subtask .todo-check",
		width("20px"),
		height("20px"),
	)
	rule(".todo-item.is-subtask .todo-title",
		fontSize("0.92rem"),
		color("var(--text-dim)"),
	)
	// The check-off ritual: a circular ring, coloured by priority; fills accent-green
	// with a check that pops in on done.
	rule(".todo-check",
		flex("none"),
		width("24px"),
		height("24px"),
		marginTop("0.05rem"),
		borderRadius("50%"),
		border("2px solid var(--border-strong)"),
		background("transparent"),
		cursor("pointer"),
		display("grid"),
		placeItems("center"),
		color("#04140c"),
		transition("border-color 0.15s ease, background 0.15s ease, transform 0.15s ease"),
	)
	rule(".todo-check:hover",
		transform("scale(1.1)"),
	)
	rule(".todo-check.p-high",
		borderColor("#ef4444"),
	)
	rule(".todo-check.p-med",
		borderColor("var(--accent)"),
	)
	rule(".todo-check.p-low",
		borderColor("var(--border-strong)"),
	)
	rule(".todo-check.is-done",
		background("var(--accent)"),
		borderColor("var(--accent)"),
	)
	rule(".todo-check svg",
		animation("todo-check-pop 0.2s ease"),
	)
	keyframes("todo-check-pop",
		at("from",
			opacity("0"),
			transform("scale(0.3)"),
		),
		at("to",
			opacity("1"),
			transform("scale(1)"),
		),
	)
	rule(".todo-main",
		flex("1"),
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		gap("0.28rem"),
	)
	rule(".todo-headline",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("1rem"),
	)
	rule(".todo-title",
		fontWeight("500"),
		fontSize("0.98rem"),
		color("var(--text)"),
		lineHeight("1.35"),
		overflowWrap("anywhere"),
	)
	rule(".todo-item.is-done .todo-title",
		textDecoration("line-through"),
		color("var(--text-faint)"),
	)
	rule(".todo-due",
		flex("none"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	rule(".todo-due.is-overdue",
		color("#fca5a5"),
		fontWeight("600"),
	)
	rule(".todo-due.is-today",
		color("#fcd34d"),
		fontWeight("600"),
	)
	rule(".todo-meta",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)
	rule(".todo-sep",
		color("var(--text-faint)"),
	)
	rule(".todo-meta-item",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
	)
	rule(".todo-meta-note",
		color("var(--text-faint)"),
		overflow("hidden"),
		whiteSpace("nowrap"),
		maxWidth("26rem"),
		prop("text-overflow", "ellipsis"),
	)
	// Linked entity → a quiet inline text-link; a goal is the one accent note.
	rule(".todo-link",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		background("transparent"),
		border("0"),
		padding("0"),
		margin("0"),
		font("inherit"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		cursor("pointer"),
		transition("color 0.12s ease"),
	)
	rule(".todo-link:hover",
		color("var(--text)"),
		prop("text-decoration", "underline"),
		prop("text-underline-offset", "3px"),
	)
	rule(".todo-link.is-goal",
		color("var(--accent)"),
	)
	// Actions: hidden until the row is hovered / focused.
	rule(".todo-actions",
		flex("none"),
		display("flex"),
		alignItems("center"),
		gap("0.1rem"),
		opacity("0"),
		transition("opacity 0.12s ease"),
	)
	rule(".todo-item:hover .todo-actions, .todo-item:focus-within .todo-actions",
		opacity("1"),
	)
	ruleMedia("(pointer: coarse)", ".todo-actions",
		opacity("1"),
	)
	rule(".todo-icon-btn",
		background("transparent"),
		border("0"),
		color("var(--text-dim)"),
		cursor("pointer"),
		padding("0.35rem"),
		borderRadius("8px"),
		display("grid"),
		placeItems("center"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".todo-icon-btn:hover",
		background("var(--bg)"),
		color("var(--text)"),
	)
	// Pager: a quiet footer under the list — a tabular range caption on the left, and
	// Prev / "Page X of Y" / Next on the right. Matches the calm agenda tone.
	rule(".todo-pager",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1rem"),
		flexWrap("wrap"),
		marginTop("0.85rem"),
		paddingTop("0.85rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".todo-pager-range",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".todo-pager-nav",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".todo-pager-page",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		minWidth("6.5rem"),
		textAlign("center"),
	)
	rule(".todo-page-btn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.82rem"),
		padding("0.35rem 0.7rem"),
		borderRadius("8px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".todo-page-btn:hover:not(:disabled)",
		borderColor("var(--accent)"),
		background("var(--bg-elev)"),
	)
	rule(".todo-page-btn:disabled",
		opacity("0.4"),
		cursor("default"),
	)
	// --- The app-standard Pager (.std-pager): the /todo pager look, extended with a
	// rows-per-page control + a jump-to-page box, mirrored above and below every paged list. ---
	rule(".std-pager",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1rem"),
		flexWrap("wrap"),
		marginTop("0.85rem"),
		paddingTop("0.85rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".std-pager.std-pager-top",
		marginTop("0"),
		marginBottom("0.85rem"),
		paddingTop("0"),
		paddingBottom("0.85rem"),
		borderTop("0"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".std-pager-info",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem 1rem"),
	)
	rule(".std-pager-range",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)
	rule(".std-pager-sizes",
		display("flex"),
		alignItems("center"),
		gap("0.25rem"),
		flexWrap("wrap"),
	)
	rule(".std-pager-size-label",
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-faint)"),
		marginRight("0.15rem"),
	)
	rule(".std-pager .pager-size",
		minHeight("0"),
		padding("0.2rem 0.5rem"),
		fontSize("0.78rem"),
		fontWeight("600"),
		borderRadius("7px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".std-pager .pager-size:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".std-pager .pager-size.active",
		background("var(--accent)"),
		color("#04140c"),
		borderColor("var(--accent)"),
	)
	rule("[data-theme=\"light\"] .std-pager .pager-size.active",
		color("#ffffff"),
	)
	rule(".std-pager-nav",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".std-page-btn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.82rem"),
		padding("0.35rem 0.7rem"),
		borderRadius("8px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".std-page-btn:hover:not(:disabled)",
		borderColor("var(--accent)"),
		background("var(--bg-elev)"),
	)
	rule(".std-page-btn:disabled",
		opacity("0.4"),
		cursor("default"),
	)
	rule(".std-pager-jump",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)
	rule(".std-pager-jump-input",
		width("3.4rem"),
		minHeight("0"),
		padding("0.25rem 0.35rem"),
		textAlign("center"),
		borderRadius("7px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
		fontSize("0.82rem"),
	)
	rule(".std-pager-jump-total",
		whiteSpace("nowrap"),
	)
	// Toolbar row (to-do, goals, and the other list pages): ONE left-justified group of
	// controls — pill selects, toggles, and the primary action — packed together from the
	// left with a uniform gap (no split into a left cluster and a far-right cluster). The
	// primary "+ Add" button is placed last in each toolbar's markup, so it sits at the
	// right end OF THE GROUP rather than floated to the window edge.
	rule(".filter-strip",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-start"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	rule(".filter-strip-controls",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	// .fctrl is the reusable "control pill" (shared with .todo-ctrl): a bordered, auto-width
	// capsule holding an icon + uppercase label + a borderless select or search input. Used
	// on the to-do, goals, budgets, accounts, and transactions toolbars for one calm control
	// language across pages (instead of full-width field bars).
	rule(".todo-ctrl, .fctrl",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		minHeight("38px"),
		padding("0.35rem 0.6rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("9px"),
		color("var(--text-faint)"),
	)
	rule(".todo-ctrl-label, .fctrl-label",
		fontSize("0.72rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)
	// The search box (a .todo-ctrl variant): a borderless input inside the pill, an accent
	// ring when it holds a query, and a small clear (×) affordance.
	rule(".todo-ctrl-search, .fctrl-search",
		// A smaller flex-basis than the search's comfortable width: it still GROWS to fill
		// (flex-grow 1, up to max-width) on roomy toolbars, but the smaller basis stops a
		// crowded toolbar (transactions has the most actions) from wrapping the trailing
		// primary button to a second row — flex-wrap breaks at the basis, so a large basis
		// forced a premature wrap even though everything fits once the search shrinks.
		flex("1 1 10rem"),
		maxWidth("22rem"),
		minWidth("9rem"),
	)
	rule(".todo-ctrl-search.is-active, .fctrl-search.is-active",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)
	rule(".todo-search-input, .fctrl-input",
		flex("1 1 auto"),
		minWidth("0"),
		border("0"),
		outline("0"),
		background("transparent"),
		color("var(--text)"),
		fontSize("0.9rem"),
	)
	rule(".todo-search-input::placeholder, .fctrl-input::placeholder",
		color("var(--text-faint)"),
	)
	rule(".todo-search-clear, .fctrl-clear",
		display("grid"),
		placeItems("center"),
		flex("none"),
		width("18px"),
		height("18px"),
		border("0"),
		borderRadius("50%"),
		background("transparent"),
		color("var(--text-faint)"),
		cursor("pointer"),
	)
	rule(".todo-search-clear:hover, .fctrl-clear:hover",
		background("var(--bg)"),
		color("var(--text)"),
	)
	rule(".todo-ctrl select.todo-select, .fctrl select.fctrl-select",
		width("auto"),
		minWidth("0"),
		minHeight("0"),
		padding("0.1rem 0.3rem 0.1rem 0.1rem"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		font("inherit"),
		fontSize("0.86rem"),
		fontWeight("500"),
		cursor("pointer"),
	)
	rule(".todo-ctrl select.todo-select:focus, .todo-ctrl select.todo-select:focus-visible, .fctrl select.fctrl-select:focus, .fctrl select.fctrl-select:focus-visible",
		prop("outline", "none"),
		boxShadow("none"),
	)
	rule(".strip-toggle",
		display("inline-flex"),
		alignItems("center"),
		minHeight("38px"),
		padding("0.35rem 0.85rem"),
		borderRadius("9px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		fontSize("0.86rem"),
		cursor("pointer"),
		transition("border-color 0.12s ease, color 0.12s ease, background 0.12s ease"),
	)
	rule(".strip-toggle:hover",
		color("var(--text)"),
		borderColor("var(--border-strong)"),
	)
	rule(".strip-toggle.is-on",
		background("var(--accent-dim)"),
		borderColor("var(--accent)"),
		color("var(--accent)"),
	)
	// Add-task "compose slip" — a two-zone editorial modal, NOT a labelled-field stack.
	// LEFT: a writing zone with a large Fraunces title + notes, and a live priority
	// "spine" (coloured left edge) that glows faint/green/red with the chosen priority.
	// RIGHT: a compact Details rail. A footer reads back a live summary + the actions.
	// The form bleeds to the flip panel's .set-body padding (1rem sides/top, 1.5rem bottom).
	rule(".tc",
		margin("-1rem -1rem -1.5rem"),
		height("calc(100% + 2.5rem)"),
		display("flex"),
		flexDirection("column"),
		overflow("hidden"),
	)
	rule(".tc-main",
		display("grid"),
		gridTemplateColumns("1.5fr 1fr"),
		flex("1"),
		minHeight("0"),
	)
	// Writing zone + priority spine.
	rule(".tc-write",
		display("flex"),
		flexDirection("column"),
		gap("0.7rem"),
		padding("1.5rem 1.5rem 1.3rem"),
		overflowY("auto"),
		boxShadow("inset 4px 0 0 var(--border-strong)"),
		transition("box-shadow 0.25s ease"),
	)
	rule(".tc-write.p-low",
		boxShadow("inset 4px 0 0 var(--text-faint)"),
	)
	rule(".tc-write.p-med",
		boxShadow("inset 4px 0 0 var(--accent)"),
	)
	rule(".tc-write.p-high",
		boxShadow("inset 4px 0 0 #ef4444"),
	)
	rule(".tc-title",
		width("100%"),
		background("transparent"),
		border("0"),
		borderBottom("2px solid var(--border)"),
		prop("outline", "none"),
		color("var(--text)"),
		fontFamily("var(--font-display), 'Fraunces', Georgia, serif"),
		fontSize("1.5rem"),
		fontWeight("500"),
		lineHeight("1.25"),
		padding("0 0 0.5rem"),
		prop("text-overflow", "ellipsis"),
		transition("border-color 0.15s ease"),
	)
	// Kill the global focus-ring box on the borderless hero field — an underline is the
	// editorial focus indicator instead.
	rule(".tc-title:focus, .tc-title:focus-visible",
		prop("outline", "none"),
		boxShadow("none"),
		borderBottomColor("var(--accent)"),
	)
	rule(".tc-title::placeholder",
		color("var(--text-faint)"),
		fontStyle("italic"),
	)
	rule(".tc-notes",
		flex("1"),
		minHeight("6rem"),
		width("100%"),
		background("var(--bg)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		prop("outline", "none"),
		prop("resize", "none"),
		color("var(--text)"),
		fontFamily("inherit"),
		fontSize("0.92rem"),
		lineHeight("1.55"),
		padding("0.75rem 0.85rem"),
		transition("border-color 0.15s ease"),
	)
	rule(".tc-notes:focus, .tc-notes:focus-visible",
		prop("outline", "none"),
		boxShadow("none"),
		borderColor("var(--accent)"),
	)
	rule(".tc-notes::placeholder",
		color("var(--text-faint)"),
	)
	// Details rail (inspector).
	rule(".tc-rail",
		display("flex"),
		flexDirection("column"),
		gap("1.1rem"),
		padding("1.35rem 1.35rem"),
		background("var(--bg)"),
		borderLeft("1px solid var(--border)"),
		overflowY("auto"),
	)
	rule(".tc-rail-head",
		margin("0"),
		fontFamily("var(--font-display), 'Fraunces', Georgia, serif"),
		fontSize("1rem"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".tc-rail-row",
		display("flex"),
		flexDirection("column"),
		gap("0.45rem"),
		minWidth("0"),
	)
	rule(".tc-rail-label",
		fontSize("0.76rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
	)
	// Footer — live summary + actions, spanning both zones.
	rule(".tc-foot",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1rem"),
		padding("0.85rem 1.5rem"),
		borderTop("1px solid var(--border)"),
		background("var(--bg-elev)"),
	)
	rule(".tc-summary",
		// Fill the footer's free width so the live summary shows in full (it was
		// truncating to a dangling "· …" while empty space sat before the actions).
		flex("1 1 auto"),
		minWidth("0"),
		marginRight("0.75rem"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		overflow("hidden"),
		whiteSpace("nowrap"),
		prop("text-overflow", "ellipsis"),
	)
	rule(".tc-foot-actions",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flex("none"),
	)
	ruleMedia("(max-width: 620px)", ".tc-main",
		gridTemplateColumns("1fr"),
	)
	// Segmented priority control.
	rule(".task-seg",
		display("inline-flex"),
		gap("3px"),
		padding("3px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		width("fit-content"),
	)
	// Rail variant: fill the row, segments share the width equally.
	rule(".task-seg.is-rail",
		display("flex"),
		width("100%"),
	)
	rule(".task-seg.is-rail .task-seg-btn",
		flex("1"),
		justifyContent("center"),
		padding("0.4rem 0.3rem"),
	)
	rule(".task-seg-btn",
		display("inline-flex"),
		alignItems("center"),
		gap("0.45rem"),
		padding("0.4rem 0.85rem"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		borderRadius("7px"),
		cursor("pointer"),
		fontSize("0.85rem"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".task-seg-btn:hover",
		color("var(--text)"),
	)
	rule(".task-seg-btn.is-active",
		background("var(--bg-elev)"),
		color("var(--text)"),
		boxShadow("0 1px 2px rgba(0,0,0,0.25)"),
	)
	rule(".task-seg-dot",
		width("9px"),
		height("9px"),
		borderRadius("50%"),
		background("var(--border-strong)"),
		flex("none"),
	)
	rule(".task-seg-btn.p-low .task-seg-dot",
		background("var(--text-faint)"),
	)
	rule(".task-seg-btn.p-med .task-seg-dot",
		background("var(--accent)"),
	)
	rule(".task-seg-btn.p-high .task-seg-dot",
		background("#ef4444"),
	)
	// Quick-date chips.
	rule(".task-quick",
		display("flex"),
		gap("0.4rem"),
		marginTop("0.1rem"),
	)
	rule(".task-quick-chip",
		fontSize("0.76rem"),
		padding("0.24rem 0.6rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		transition("border-color 0.12s ease, color 0.12s ease, background 0.12s ease"),
	)
	rule(".task-quick-chip:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)
	rule(".task-quick-chip.is-clear",
		marginLeft("auto"),
	)
	// Goal cards: a responsive grid of compact cards (like /budgets), each a self-
	// contained card with a saved-of-target "loader" bar holding the amount + percent,
	// a pace-tinted accent stripe, and footer actions.
	rule(".bento-goals .goal-list",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(460px, 1fr))"),
		gap("0.75rem"),
		// Cards stretch to the tallest in their row so every card in a row shares one
		// height — the footer actions pin to the bottom (margin-top:auto) rather than
		// floating mid-card. Capping the steps list at 3 keeps that shared height sane.
		alignItems("stretch"),
	)
	rule(".bento-goals .goal-card",
		position("relative"),
		display("flex"),
		flexDirection("column"),
		minHeight("288px"),
		padding("0.9rem 1.15rem 0.85rem"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		boxShadow("inset 5px 0 0 var(--accent)"),
		transition("transform 0.18s ease, border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease"),
	)
	rule(".bento-goals .goal-card:hover",
		borderColor("color-mix(in srgb, var(--accent) 34%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 85%, transparent)"),
		transform("translateY(-1px)"),
	)
	// When a card's ⋯ menu is open, lift the whole card above its neighbours so the
	// popover paints over sibling cards instead of being covered by a later card in the
	// grid (the hover transform makes each card its own stacking context). The card is
	// position:relative, so z-index applies.
	rule(".bento-goals .goal-card:has(.add-wrap [aria-expanded=\"true\"])",
		zIndex("30"),
	)
	rule(".bento-goals .goal-card.is-soon",
		boxShadow("inset 5px 0 0 #f59e0b"),
	)
	rule(".bento-goals .goal-card.is-overdue",
		boxShadow("inset 5px 0 0 var(--danger)"),
		background("color-mix(in srgb, var(--danger) 10%, var(--bg-elev))"),
	)
	rule(".bento-goals .goal-card.is-done",
		boxShadow("inset 5px 0 0 color-mix(in srgb, var(--accent) 55%, var(--text-dim))"),
	)
	rule(".bento-goals .goal-card-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.45rem"),
		marginBottom("0.1rem"),
	)
	rule(".bento-goals .goal-card-title",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("700"),
		fontSize("1.05rem"),
		color("var(--text)"),
	)
	// The loader: a taller progress bar holding the amount (left) + percent (right).
	rule(".bento-goals .goal-card-loader",
		position("relative"),
		overflow("hidden"),
		height("42px"),
		margin("0.55rem 0 0.6rem"),
		borderRadius("10px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
	)
	rule(".bento-goals .goal-card-loader .bar-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		height("100%"),
		borderRadius("0"),
		boxShadow("none"),
		zIndex("0"),
	)
	// The earmark band: the reserved-but-not-moved slice, drawn as a quiet diagonal
	// hatch in the accent so it reads as "backed, pledged" rather than solid cash. It
	// runs out to COVERAGE beneath the solid saved fill; only the saved..coverage gap
	// shows through. Deliberately muted so the committed segment still leads.
	rule(".bento-goals .goal-card-loader .bar-fill.bar-earmark",
		background("repeating-linear-gradient(-45deg, color-mix(in srgb, var(--accent) 30%, transparent) 0, color-mix(in srgb, var(--accent) 30%, transparent) 5px, color-mix(in srgb, var(--accent) 12%, transparent) 5px, color-mix(in srgb, var(--accent) 12%, transparent) 10px)"),
		borderRight("1px solid color-mix(in srgb, var(--accent) 45%, transparent)"),
	)
	// Secondary goal-card action ("Log saved"): a quiet ghost next to the primary
	// "Set aside" — transparent, dim, no border weight, so it defers to the primary.
	rule(".bento-goals .goal-action-ghost",
		background("transparent"),
		borderColor("transparent"),
		color("var(--text-dim)"),
		fontWeight("500"),
	)
	rule(".bento-goals .goal-action-ghost:hover",
		background("var(--hover)"),
		color("var(--text)"),
		filter("none"),
	)
	rule(".bento-goals .goal-card-loader-figs",
		position("relative"),
		zIndex("1"),
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		height("100%"),
		padding("0 0.75rem"),
		gap("0.5rem"),
	)
	rule(".bento-goals .goal-card-loader .budget-amount",
		fontVariantNumeric("tabular-nums"),
		fontSize("0.92rem"),
		fontWeight("400"),
		color("var(--text-dim)"),
		prop("text-shadow", "0 1px 3px rgba(0,0,0,0.5)"),
	)
	rule(".bento-goals .goal-card-loader .budget-amount .budget-spent",
		color("var(--text)"),
		fontWeight("700"),
	)
	rule(".bento-goals .goal-card-loader .budget-pct",
		fontVariantNumeric("tabular-nums"),
		fontWeight("700"),
		fontSize("0.8rem"),
		color("color-mix(in srgb, var(--accent) 40%, #ffffff)"),
		prop("text-shadow", "0 1px 3px rgba(0,0,0,0.5)"),
		whiteSpace("nowrap"),
	)
	// The pale-accent-on-dark-track tint above is unreadable on light theme's
	// pale track (~1.2:1). In light mode, darken the accent and drop the dark
	// text-shadow so the percentage is legible (WCAG AA).
	rule("[data-theme=\"light\"] .bento-goals .goal-card-loader .budget-pct",
		color("color-mix(in srgb, var(--accent) 78%, #000000)"),
		prop("text-shadow", "none"),
	)
	// Footer actions pinned to the card bottom with a hairline separator.
	rule(".bento-goals .goal-card-actions",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.35rem"),
		marginTop("auto"),
		paddingTop("0.7rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
	)
	rule(".bento-goals .goal-sub",
		marginTop("0"),
	)
	// Linked to-dos ("steps") on a goal card: a compact checklist between the sub-line and
	// the footer. A hairline separates it; the list scrolls if it grows so cards stay bounded.
	rule(".goal-todos",
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".goal-todos-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".goal-todos-title",
		fontSize("0.72rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-dim)"),
	)
	rule(".goal-todos-count",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".goal-todos-list",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		maxHeight("9rem"),
		overflowY("auto"),
	)
	rule(".goal-todos-empty",
		margin("0"),
		fontSize("0.82rem"),
		color("var(--text-faint)"),
	)
	// "+N more" line under a capped (top-3) steps list — a quiet count of the remainder.
	rule(".goal-todos-more",
		margin("0.1rem 0 0 0.15rem"),
		fontSize("0.75rem"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	// GL4 contribution planner, GL5 pledge split-bar, GL3 emergency sizer: three
	// optional card sections between the sub-line and the steps, each set off by a
	// hairline and using the same quiet, uppercase section head as goal-todos.
	rule(".goal-plan, .goal-pledge, .goal-essential",
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".goal-plan-head, .goal-pledge-head, .goal-essential-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".goal-plan-title, .goal-pledge-title, .goal-essential-title",
		fontSize("0.72rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-dim)"),
	)
	rule(".goal-plan-amt",
		fontSize("0.9rem"),
		fontWeight("600"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".goal-plan-readout",
		fontSize("0.82rem"),
		color("var(--text)"),
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".goal-plan-actions, .goal-essential-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	// GL5 split bar: a rounded track of proportional member segments, tinted by the
	// accent with per-index opacity so members read apart without a hardcoded palette.
	rule(".goal-pledge-bar",
		display("flex"),
		height("0.55rem"),
		borderRadius("999px"),
		overflow("hidden"),
		background("var(--bg-elev)"),
	)
	rule(".goal-pledge-seg-0", background("var(--accent)"))
	rule(".goal-pledge-seg-1", background("color-mix(in srgb, var(--accent) 72%, var(--bg-card))"))
	rule(".goal-pledge-seg-2", background("color-mix(in srgb, var(--accent) 52%, var(--bg-card))"))
	rule(".goal-pledge-seg-3", background("color-mix(in srgb, var(--accent) 36%, var(--bg-card))"))
	rule(".goal-pledge-seg-4", background("color-mix(in srgb, var(--accent) 24%, var(--bg-card))"))
	rule(".goal-pledge-seg-5", background("color-mix(in srgb, var(--accent) 16%, var(--bg-card))"))
	rule(".goal-pledge-legend",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".goal-pledge-line",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		fontSize("0.8rem"),
	)
	rule(".goal-pledge-dot",
		width("0.6rem"),
		height("0.6rem"),
		borderRadius("999px"),
		flexShrink("0"),
	)
	rule(".goal-pledge-name",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".goal-essential-body",
		margin("0"),
		fontSize("0.82rem"),
		color("var(--text)"),
	)
	rule(".goal-essential-hint",
		margin("0"),
		fontSize("0.75rem"),
	)
	// Multi-link checklists in the goal editor (accounts / budgets). A bounded, scrolling
	// column of checkbox rows so a household with many accounts doesn't blow out the modal.
	rule(".goal-link-list",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		maxHeight("11rem"),
		overflowY("auto"),
		padding("0.15rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
	)
	rule(".goal-link-row",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		padding("0.35rem 0.4rem"),
		borderRadius("6px"),
		cursor("pointer"),
	)
	rule(".goal-link-row:hover",
		background("var(--bg-elev)"),
	)
	// Virtual-allocation modal: one row per linked account (name + free balance, then an
	// amount input), and a summary line totalling the earmark against the target.
	rule(".goal-alloc-list",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		margin("0.2rem 0"),
	)
	rule(".goal-alloc-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		padding("0.5rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("9px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	// An amount over the account's free balance: warn tint on the row + input (the
	// save button errors on it — never a silent clamp).
	rule(".goal-alloc-row.is-over",
		borderColor("color-mix(in srgb, var(--warn, #d8a24a) 55%, var(--border))"),
		background("color-mix(in srgb, var(--warn, #d8a24a) 7%, transparent)"),
	)
	rule(".goal-alloc-row.is-over .goal-alloc-input",
		borderColor("var(--warn, #d8a24a)"),
	)
	rule(".goal-alloc-over",
		fontSize("0.72rem"),
	)
	// The uncheck-all affordance sits quietly under the list.
	rule(".goal-alloc-clear",
		alignSelf("flex-start"),
		marginTop("0.35rem"),
	)
	rule(".goal-alloc-row-main",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		minWidth("0"),
	)
	rule(".goal-alloc-acct",
		fontWeight("600"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".goal-alloc-avail",
		fontSize("0.78rem"),
		fontVariantNumeric("tabular-nums"),
	)
	// Held-asset tag on non-liquid earmark rows ("Retirement · held asset").
	rule(".goal-alloc-type",
		fontSize("0.72rem"),
		whiteSpace("nowrap"),
	)
	// Specific enough (0,3,0) to beat the form's ".acct-edit-form .field { width:100% }"
	// rule, which would otherwise stretch the amount field over the account name.
	rule(".goal-allocate .goal-alloc-row .goal-alloc-input",
		width("9rem"),
		minWidth("9rem"),
		flex("0 0 9rem"),
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".goal-alloc-summary",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".goal-alloc-cover",
		fontVariantNumeric("tabular-nums"),
		color("var(--up)"),
	)
	// Allocate modal: the master enable toggle row + selectable per-account rows.
	rule(".goal-alloc-enable",
		display("flex"),
		alignItems("flex-start"),
		gap("0.6rem"),
		padding("0.6rem 0.7rem"),
		margin("0 0 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		cursor("pointer"),
	)
	rule(".goal-alloc-enable input",
		marginTop("0.15rem"),
		flex("none"),
	)
	rule(".goal-alloc-enable-txt",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".goal-alloc-enable-title",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".goal-alloc-row.is-on",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, var(--bg-elev))"),
	)
	// Smart-split control: a "total to earmark" field + even/proportional buttons.
	rule(".goal-alloc-split",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		margin("0 0 0.7rem"),
		padding("0.6rem 0.7rem"),
		border("1px dashed var(--border)"),
		borderRadius("10px"),
	)
	rule(".goal-alloc-split-row",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.6rem"),
	)
	rule(".goal-alloc-split-field",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		flex("1 1 12rem"),
		minWidth("0"),
	)
	rule(".goal-allocate .goal-alloc-split-field .goal-alloc-total",
		width("100%"),
	)
	rule(".goal-alloc-split-btns",
		display("flex"),
		gap("0.4rem"),
		flexShrink("0"),
	)
	rule(".goal-alloc-pick",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flex("1 1 auto"),
		minWidth("0"),
		cursor("pointer"),
	)
	rule(".goal-alloc-pick input",
		flex("none"),
	)
	rule(".goal-alloc-input:disabled",
		opacity("0.4"),
		cursor("not-allowed"),
	)
	// Earmark status badge tones (pace-badge base): none = quiet outline, partial = amber,
	// full = accent-green — a glanceable read of how reserved a goal is.
	rule(".earmark-none",
		background("transparent"),
		color("var(--text-faint)"),
		borderColor("var(--border)"),
	)
	rule(".earmark-partial",
		background("rgba(245,158,11,0.14)"),
		color("#d98c00"),
		borderColor("rgba(245,158,11,0.35)"),
	)
	rule("[data-theme=\"light\"] .earmark-partial",
		color("#b45309"),
	)
	rule(".earmark-full",
		background("color-mix(in srgb, var(--up) 16%, transparent)"),
		color("var(--up)"),
		borderColor("color-mix(in srgb, var(--up) 40%, var(--border))"),
	)
	// Goals-page tab strip (Goals · Earmarks) — a pill-segmented control.
	rule(".goals-tabs",
		display("inline-flex"),
		gap("0.15rem"),
		padding("0.15rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
	)
	rule(".goals-tab",
		padding("0.35rem 0.85rem"),
		border("0"),
		borderRadius("8px"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.85rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".goals-tab:hover",
		color("var(--text)"),
	)
	rule(".goals-tab.is-active",
		background("var(--accent)"),
		color("#04140c"),
	)
	rule("[data-theme=\"light\"] .goals-tab.is-active",
		color("#ffffff"),
	)
	// --- Earmarks manager (the "Earmarks" tab) ---
	rule(".earmarks-mgr",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	rule(".ea-exp-list",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".ea-exp-row",
		display("grid"),
		gridTemplateColumns("1fr auto auto"),
		alignItems("center"),
		gap("1rem"),
		padding("0.4rem 0.2rem"),
		borderBottom("1px solid color-mix(in srgb, var(--border) 55%, transparent)"),
	)
	rule(".ea-exp-head",
		fontSize("0.72rem"),
		letterSpacing("0.03em"),
		prop("text-transform", "uppercase"),
		color("var(--text-dim)"),
		fontWeight("600"),
	)
	rule(".ea-exp-name",
		fontWeight("600"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".ea-exp-earmarked",
		fontVariantNumeric("tabular-nums"),
		color("var(--up)"),
		fontWeight("600"),
	)
	rule(".ea-exp-free",
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
		fontSize("0.85rem"),
	)
	rule(".ea-goals",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".ea-goal",
		border("1px solid var(--border)"),
		borderRadius("11px"),
		padding("0.6rem 0.8rem"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
	)
	rule(".ea-goal-head",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		marginBottom("0.4rem"),
	)
	rule(".ea-goal-name",
		flex("1 1 auto"),
		minWidth("0"),
		fontWeight("700"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".ea-goal-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".ea-row",
		display("grid"),
		gridTemplateColumns("1fr auto auto"),
		alignItems("center"),
		gap("0.75rem"),
		padding("0.3rem 0.4rem"),
		borderRadius("7px"),
	)
	rule(".ea-row:hover",
		background("var(--bg)"),
	)
	rule(".ea-row-acct",
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".ea-row-amt",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".ea-row-del",
		display("grid"),
		placeItems("center"),
		width("26px"),
		height("26px"),
		border("0"),
		borderRadius("6px"),
		background("transparent"),
		color("var(--text-faint)"),
		cursor("pointer"),
		transition("background 0.12s ease, color 0.12s ease"),
	)
	rule(".ea-row-del:hover",
		background("color-mix(in srgb, var(--danger) 14%, transparent)"),
		color("var(--danger)"),
	)
	rule(".ea-empty",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap("0.4rem"),
		padding("1.5rem 1rem"),
		textAlign("center"),
	)
	rule(".ea-empty-title",
		margin("0"),
		fontWeight("700"),
		fontSize("1rem"),
		color("var(--text)"),
	)
	// Add-goal modal: a calm 2-column grid, top-aligned so a tall hint never bottom-shifts
	// its row-mates; the lead + hint-bearing fields span the full width via .fg-span.
	rule(".goal-add .form-grid",
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		alignItems("start"),
		gap("0.8rem 0.9rem"),
	)
	rule(".goal-add .fg-span",
		gridColumn("1 / -1"),
	)
	// A checkbox + inline hint row (sinking fund, contribute ledger) with proper spacing
	// and a normal-weight label — replaces the cramped bold .ba-check block.
	rule(".goal-check-row",
		display("flex"),
		alignItems("flex-start"),
		gap("0.5rem"),
		cursor("pointer"),
		fontWeight("400"),
		color("var(--text-dim)"),
		lineHeight("1.4"),
	)
	rule(".goal-check-row input",
		marginTop("0.2rem"),
		flex("none"),
	)
	// Contribute modal: a self-contained progress bar (the modal isn't under .bento-goals,
	// so the card-loader styles don't reach it), a quick-fill chip, and the amount row.
	rule(".contrib-loader",
		position("relative"),
		overflow("hidden"),
		height("40px"),
		margin("0 0 0.7rem"),
		borderRadius("10px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
	)
	rule(".contrib-loader .bar-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		height("100%"),
		background("var(--accent)"),
		zIndex("0"),
	)
	rule(".contrib-loader-figs",
		position("relative"),
		zIndex("1"),
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		height("100%"),
		padding("0 0.75rem"),
		gap("0.5rem"),
	)
	rule(".contrib-saved",
		fontVariantNumeric("tabular-nums"),
		fontSize("0.9rem"),
		color("var(--text-dim)"),
		prop("text-shadow", "0 1px 3px rgba(0,0,0,0.5)"),
	)
	rule(".contrib-saved .budget-spent",
		color("var(--text)"),
		fontWeight("700"),
	)
	rule(".contrib-pct",
		fontVariantNumeric("tabular-nums"),
		fontWeight("700"),
		fontSize("0.8rem"),
		color("color-mix(in srgb, var(--accent) 40%, #ffffff)"),
		prop("text-shadow", "0 1px 3px rgba(0,0,0,0.5)"),
	)
	rule("[data-theme=\"light\"] .contrib-pct",
		color("color-mix(in srgb, var(--accent) 78%, #000000)"),
		prop("text-shadow", "none"),
	)
	rule(".contrib-amount-row",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".contrib-amount-row .field",
		flex("1 1 auto"),
	)
	rule(".contrib-chip",
		flex("none"),
		padding("0.45rem 0.8rem"),
		border("1px solid color-mix(in srgb, var(--accent) 40%, var(--border))"),
		borderRadius("8px"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
		color("var(--accent)"),
		fontWeight("600"),
		fontSize("0.85rem"),
		whiteSpace("nowrap"),
		cursor("pointer"),
		transition("background 0.12s ease, border-color 0.12s ease"),
	)
	rule(".contrib-chip:hover",
		background("color-mix(in srgb, var(--accent) 18%, transparent)"),
		borderColor("var(--accent)"),
	)
	rule(".goal-todo",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		padding("0.25rem 0.15rem"),
		borderRadius("6px"),
	)
	rule(".goal-todo:hover",
		background("var(--bg)"),
	)
	rule(".goal-todo-check",
		flex("none"),
		width("18px"),
		height("18px"),
		borderRadius("50%"),
		border("1.5px solid var(--border-strong)"),
		background("transparent"),
		cursor("pointer"),
		display("grid"),
		placeItems("center"),
		color("#04140c"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".goal-todo-check:hover",
		borderColor("var(--accent)"),
	)
	rule(".goal-todo-check.is-done",
		background("var(--accent)"),
		borderColor("var(--accent)"),
	)
	rule(".goal-todo-title",
		fontSize("0.86rem"),
		color("var(--text)"),
		overflowWrap("anywhere"),
	)
	rule(".goal-todo-title.is-done",
		textDecoration("line-through"),
		color("var(--text-faint)"),
	)
	rule(".goal-todo-add",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		alignSelf("flex-start"),
		marginTop("0.1rem"),
		padding("0.25rem 0.5rem"),
		background("transparent"),
		border("1px dashed var(--border)"),
		borderRadius("7px"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		cursor("pointer"),
		transition("border-color 0.12s ease, color 0.12s ease"),
	)
	rule(".goal-todo-add:hover",
		borderColor("var(--accent)"),
		color("var(--accent)"),
	)
	// --- /budgets visual polish. Scoped to .bento-budgets so the shared .budget /
	// .bar / .budget-sub styles used on other screens (allocate, goals, reports) stay
	// untouched. Each budget becomes an elevated meter-card with a state-colored left
	// stripe, a prominent gradient progress bar over a visible track (so 0%/low budgets
	// no longer vanish into the background), and a tinted percent chip. ---
	// The summary "loader": a big spent-of-budgeted progress bar with the spent/budgeted/
	// left figures rendered inside it. The fill sits behind the figures (z-index) and its
	// width tracks the spent percentage; the figures stay legible over both fill and track.
	rule(".budget-loader",
		position("relative"),
		overflow("hidden"),
		borderRadius("14px"),
		border("1px solid var(--border)"),
		background("color-mix(in srgb, var(--bg-elev) 42%, transparent)"),
		minHeight("104px"),
		marginBottom("1rem"),
	)
	rule(".budget-loader.is-over",
		borderColor("color-mix(in srgb, var(--danger) 32%, var(--border))"),
	)
	rule(".budget-loader-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		width("0"),
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 30%, transparent), color-mix(in srgb, var(--accent) 15%, transparent))"),
		borderRight("2px solid color-mix(in srgb, var(--accent) 70%, transparent)"),
		transition("width 0.45s cubic-bezier(.2,.75,.2,1)"),
		zIndex("0"),
	)
	rule(".budget-loader-fill.is-near",
		background("linear-gradient(90deg, color-mix(in srgb, #f59e0b 30%, transparent), color-mix(in srgb, #f59e0b 15%, transparent))"),
		borderRight("2px solid #f59e0b"),
	)
	rule(".budget-loader-fill.is-over",
		background("linear-gradient(90deg, color-mix(in srgb, var(--danger) 34%, transparent), color-mix(in srgb, var(--danger) 18%, transparent))"),
		borderRight("2px solid var(--danger)"),
	)
	rule(".budget-loader-figs",
		position("relative"),
		zIndex("1"),
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("1rem"),
		minHeight("104px"),
		padding("0 1.5rem"),
	)
	rule(".budget-loader-fig",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		minWidth("0"),
	)
	rule(".budget-loader-fig.is-right",
		alignItems("flex-end"),
		textAlign("right"),
	)
	rule(".budget-loader-label",
		color("var(--text-dim)"),
		fontSize("0.72rem"),
		textTransform("uppercase"),
		letterSpacing("0.06em"),
	)
	rule(".budget-loader-value",
		// Serif display numerals, matching the hero figures on every other page
		// (rpt-hero-value etc.) — this is the app's clearest typographic signature,
		// so the budgets/goals summary tiles must speak it too.
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
		fontSize("1.45rem"),
		fontWeight("700"),
		letterSpacing("-0.015em"),
		prop("font-variant-numeric", "tabular-nums"),
		whiteSpace("nowrap"),
		prop("text-shadow", "0 1px 4px rgba(0,0,0,0.35)"),
	)
	rule(".budget-loader-value.is-hero",
		fontSize("2rem"),
		fontWeight("800"),
	)
	rule(".budget-loader-value.pos",
		color("var(--money-positive)"),
	)
	rule(".budget-loader-value.neg",
		color("var(--money-negative)"),
	)
	// Per-card "loader": a taller progress bar that holds the spent/limit amount (left)
	// and the percent (right) inside it, so the card header is free for just the title.
	rule(".bento-budgets .budget-card-loader",
		position("relative"),
		overflow("hidden"),
		height("42px"),
		margin("0.5rem 0 0.6rem"),
		borderRadius("10px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
	)
	rule(".bento-budgets .budget-card-loader .bar-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		height("100%"),
		borderRadius("0"),
		boxShadow("none"),
		zIndex("0"),
	)
	// XC4: the committed segment — a muted, hatched accent band sitting just past the
	// spent fill, showing the slice of remaining money already claimed by recurring
	// commitments. Deliberately quiet so the primary fill still reads at a glance.
	rule(".bento-budgets .budget-card-loader .bar-committed",
		position("absolute"),
		top("0"),
		bottom("0"),
		height("100%"),
		zIndex("0"),
		prop("background", "repeating-linear-gradient(45deg, color-mix(in srgb, var(--accent) 30%, transparent) 0, color-mix(in srgb, var(--accent) 30%, transparent) 6px, color-mix(in srgb, var(--accent) 16%, transparent) 6px, color-mix(in srgb, var(--accent) 16%, transparent) 12px)"),
	)
	rule(".bento-budgets .budget-card-loader-figs",
		position("relative"),
		zIndex("1"),
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		height("100%"),
		padding("0 0.75rem"),
		gap("0.5rem"),
	)
	rule(".bento-budgets .budget-card-loader .budget-amount",
		fontVariantNumeric("tabular-nums"),
		fontSize("0.92rem"),
		fontWeight("400"),
		color("var(--text-dim)"),
		prop("text-shadow", "0 1px 3px rgba(0,0,0,0.5)"),
	)
	rule(".bento-budgets .budget-card-loader .budget-amount .budget-spent",
		color("var(--text)"),
		fontWeight("700"),
	)
	// Budget cards lay out in a responsive grid — each a compact 1-column block (several
	// per row) instead of a full-width bar. auto-fill + minmax keeps them readable and
	// reflows to fewer columns as the surface narrows.
	rule(".bento-budgets .budget-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(320px, 1fr))"),
		gap("0.75rem"),
	)
	rule(".bento-budgets .budget",
		position("relative"),
		display("flex"),
		flexDirection("column"),
		minHeight("210px"),
		padding("0.9rem 1.15rem 0.85rem"),
		margin("0"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		boxShadow("inset 5px 0 0 var(--accent)"),
		transition("transform 0.18s ease, border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease"),
	)
	// "Last month's spend" overline tag: a small accent chip above the main bar that names
	// it as last month's — so when the overlay is on, the tile reads as last month's
	// picture (the big bar carries last period's figures) rather than this month's.
	rule(".bento-budgets .budget-lastmonth-tag",
		alignSelf("flex-start"),
		margin("0.3rem 0 0.1rem"),
		padding("0.1rem 0.45rem"),
		borderRadius("6px"),
		fontSize("0.6rem"),
		fontWeight("700"),
		letterSpacing("0.09em"),
		textTransform("uppercase"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 13%, transparent)"),
	)
	rule(".bento-budgets .budget:first-child",
		borderTop("1px solid var(--border)"),
	)
	rule(".bento-budgets .budget:hover",
		borderColor("color-mix(in srgb, var(--accent) 34%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 85%, transparent)"),
		transform("translateY(-1px)"),
		boxShadow("inset 5px 0 0 var(--accent), 0 10px 28px -16px rgba(0,0,0,0.75)"),
	)
	rule(".bento-budgets .budget.is-near, .bento-budgets .budget.is-risk",
		boxShadow("inset 5px 0 0 #f59e0b"),
	)
	rule(".bento-budgets .budget.is-near:hover, .bento-budgets .budget.is-risk:hover",
		boxShadow("inset 5px 0 0 #f59e0b, 0 10px 28px -16px rgba(0,0,0,0.75)"),
	)
	rule(".bento-budgets .budget.is-over",
		// Confine "over budget" to the left accent bar + the progress fill — a full-card
		// red wash otherwise fights the amber pace copy ("Running $X hot") layered on top.
		// This mirrors the at-risk (amber) card, which keeps a neutral body.
		boxShadow("inset 5px 0 0 var(--danger)"),
	)
	rule(".bento-budgets .budget.is-over:hover",
		borderColor("color-mix(in srgb, var(--danger) 36%, var(--border))"),
		boxShadow("inset 5px 0 0 var(--danger), 0 10px 28px -16px rgba(0,0,0,0.75)"),
	)
	rule(".bento-budgets .budget-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.4rem 0.75rem"),
		marginBottom("0.55rem"),
	)
	rule(".bento-budgets .budget-head-main",
		display("flex"),
		alignItems("baseline"),
		gap("0.55rem"),
		minWidth("0"),
		flex("1 1 auto"),
	)
	// Actions are the card's footer: pinned to the bottom (margin-top:auto) with a hairline
	// separator, wrapping within the compact card. Slightly dimmed at rest, full on hover.
	rule(".bento-budgets .budget-actions",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.35rem"),
		marginTop("auto"),
		paddingTop("0.7rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 70%, transparent)"),
		opacity("0.85"),
		transition("opacity 0.15s ease"),
	)
	rule(".bento-budgets .budget:hover .budget-actions, .bento-budgets .budget:focus-within .budget-actions",
		opacity("1"),
	)
	rule(".bento-budgets .budget-actions .btn, .bento-budgets .budget-actions .btn-del",
		flexShrink("0"),
		whiteSpace("nowrap"),
	)
	// The ⋯ overflow (destructive actions) anchors the far right of the action row.
	rule(".bento-budgets .budget-actions .add-wrap",
		marginLeft("auto"),
	)
	// Name-first hierarchy: the category name is the card title.
	rule(".bento-budgets .budget .row-desc",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("700"),
		fontSize("1.05rem"),
		color("var(--text)"),
	)
	// The spent/limit amount is secondary context: muted, tabular, pushed right of the
	// title; only the spent figure carries foreground weight.
	rule(".bento-budgets .budget-head-main .budget-amount",
		flexShrink("0"),
		marginLeft("auto"),
		fontVariantNumeric("tabular-nums"),
		fontSize("0.9rem"),
		fontWeight("400"),
		color("var(--text-dim)"),
	)
	rule(".bento-budgets .budget-amount .budget-spent",
		color("var(--text)"),
		fontWeight("600"),
	)
	rule(".bento-budgets .budget-drill",
		textDecoration("underline dotted"),
		textUnderlineOffset("3px"),
	)
	rule(".bento-budgets .budget-drill:hover",
		color("var(--accent)"),
	)
	// Percent chip: brighter, legible text on a stronger health tint (the old dark-green
	// on dark-green failed at a glance).
	rule(".bento-budgets .budget-pct",
		display("inline-flex"),
		alignItems("center"),
		fontVariantNumeric("tabular-nums"),
		fontWeight("700"),
		fontSize("0.75rem"),
		letterSpacing("0.01em"),
		padding("0.14rem 0.55rem"),
		borderRadius("999px"),
		flexShrink("0"),
		color("color-mix(in srgb, var(--accent) 45%, #ffffff)"),
		background("color-mix(in srgb, var(--accent) 26%, transparent)"),
		whiteSpace("nowrap"),
	)
	rule(".bento-budgets .budget.is-near .budget-pct, .bento-budgets .budget.is-risk .budget-pct",
		color("#fbbf24"),
		background("color-mix(in srgb, #f59e0b 26%, transparent)"),
	)
	rule(".bento-budgets .budget.is-over .budget-pct",
		color("color-mix(in srgb, var(--danger) 42%, #ffffff)"),
		background("color-mix(in srgb, var(--danger) 26%, transparent)"),
	)
	// The progress bar is the card's centerpiece: tall, rounded, over a clearly visible
	// track so even a 0%/low budget reads at a glance.
	rule(".bento-budgets .bar",
		height("16px"),
		background("color-mix(in srgb, var(--text) 9%, transparent)"),
		border("0"),
		borderRadius("8px"),
		margin("0 0 0.5rem"),
	)
	rule(".bento-budgets .bar-fill",
		borderRadius("8px"),
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 70%, #000000), var(--accent))"),
		boxShadow("0 0 12px -1px color-mix(in srgb, var(--accent) 55%, transparent)"),
	)
	rule(".bento-budgets .bar-fill.near",
		background("linear-gradient(90deg, #d97706, #f59e0b)"),
		boxShadow("0 0 12px -1px color-mix(in srgb, #f59e0b 50%, transparent)"),
	)
	rule(".bento-budgets .bar-fill.over",
		background("linear-gradient(90deg, color-mix(in srgb, var(--danger) 70%, #000000), var(--danger))"),
		boxShadow("0 0 12px -1px color-mix(in srgb, var(--danger) 55%, transparent)"),
	)
	// A single quiet metadata line below the bar (status · remaining · period); the
	// redundant "% used" line is gone (the bar + chip carry that).
	rule(".bento-budgets .budget > .budget-sub:first-of-type",
		fontSize("0.84rem"),
		color("var(--text-dim)"),
		marginTop("0"),
	)
	// Make "Left" the dominant summary figure — it's the number that matters most, so
	// it outsizes Spent/Budgeted (critique #4).
	rule(".bento-budgets .stat-value.is-hero",
		fontSize("2.6rem"),
	)
	// Toolbar: ONE left-justified group — the method + sort pickers and the actions all
	// packed together from the left with a uniform gap (not split into a left picker
	// cluster and a right action cluster). The primary "+ Add budget" is last in the
	// markup, so it sits at the right end of the group.
	rule(".bento-budgets .budgets-toolbar",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-start"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".bento-budgets .budgets-toolbar-method, .bento-goals .budgets-toolbar-method",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		minWidth("0"),
		flexShrink("1"),
	)
	rule(".bento-budgets .budgets-toolbar-label, .bento-goals .budgets-toolbar-label",
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".bento-budgets .budgets-method-select, .bento-goals .budgets-method-select",
		width("auto"),
		minWidth("0"),
		maxWidth("280px"),
	)
	rule(".bento-budgets .budgets-toolbar-actions",
		display("flex"),
		alignItems("center"),
		flexWrap("nowrap"),
		flexShrink("0"),
		gap("0.5rem"),
	)
	rule(".bento-ledger .txn-table",
		tableLayout("fixed"),
		width("100%"),
	)
	rule(".bento-ledger .txn-table th, .bento-ledger .txn-table td",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".bento-ledger .txn-table th:nth-child(1), .bento-ledger .txn-table td:nth-child(1)",
		width("40px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(2), .bento-ledger .txn-table td:nth-child(2)",
		width("116px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(3), .bento-ledger .txn-table td:nth-child(3)",
		width("120px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(5), .bento-ledger .txn-table td:nth-child(5)",
		width("184px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(6), .bento-ledger .txn-table td:nth-child(6)",
		width("150px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(7), .bento-ledger .txn-table td:nth-child(7)",
		width("118px"),
	)
	rule(".w",
		position("relative"),
		background("#121214"),
		border("1px solid #34343a"),
		overflow("hidden"),
		display("flex"),
		flexDirection("column"),
		transition("border-color .12s ease"),
		willChange("transform"),
	)
	rule(".w:hover",
		borderColor("#44444c"),
	)
	rule(".w.drag",
		opacity(".35 !important"),
		cursor("grabbing"),
	)
	rule("[data-bento-dragging] .bento",
		cursor("grabbing"),
	)
	rule("[data-bento-dragging] .bento .w:not(.drag)",
		transition("border-color .15s ease, box-shadow .15s ease"),
	)
	rule(".ghandle",
		position("absolute"),
		top("7px"),
		right("40px"),
		zIndex("3"),
		color("#5a5a64"),
		cursor("grab"),
		fontSize(".95rem"),
		lineHeight("1"),
	)
	rule(".ghandle:active",
		cursor("grabbing"),
	)
	rule(".gear-inline",
		marginLeft("auto"),
		color("#5a5a64"),
		background("transparent"),
		border("0"),
		cursor("pointer"),
		fontSize(".95rem"),
		lineHeight("1"),
		transition("color .12s ease"),
	)
	rule(".w:hover .gear-inline, .w:focus-within .gear-inline",
		color("#9a9aa2"),
	)
	rule(".gear-inline:hover",
		color("#f4f4f5"),
	)
	rule(".gear-abs",
		position("absolute"),
		top("7px"),
		right("16px"),
		zIndex("3"),
		color("#5a5a64"),
		background("transparent"),
		border("0"),
		cursor("pointer"),
		fontSize(".95rem"),
		lineHeight("1"),
	)
	rule(".gear-abs:hover",
		color("#f4f4f5"),
	)
	rule(".gear-inline, .gear-abs",
		transition("color .12s ease, transform var(--wonder-dur-fast) var(--wonder-ease)"),
	)
	rule(".seg",
		display("inline-flex"),
		background("#1a1a1d"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		padding("2px"),
		position("relative"),
	)
	rule(".seg-pill",
		position("absolute"),
		top("2px"),
		bottom("2px"),
		left("0"),
		width("0"),
		background("#2a2a30"),
		borderRadius("3px"),
		opacity("0"),
		pointerEvents("none"),
		zIndex("0"),
		transition("transform .18s ease, width .18s ease, opacity .12s ease"),
		boxShadow("0 1px 2px rgba(0,0,0,0.3), inset 0 1px 0 rgba(255,255,255,0.05)"),
	)
	rule(".seg-btn",
		padding(".32rem .72rem"),
		borderRadius("3px"),
		fontSize(".85rem"),
		color("#a6a6ac"),
		background("transparent"),
		border("0"),
		cursor("pointer"),
		position("relative"),
		zIndex("1"),
	)
	rule(".seg-btn:hover",
		color("#f4f4f5"),
	)
	rule(".seg-btn.active",
		background("transparent"),
		color("#f4f4f5"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".seg-pill",
		transition("none"),
	)
	rule(".rcap",
		fontSize(".66rem"),
		textTransform("uppercase"),
		letterSpacing(".12em"),
		color("#6c6c72"),
	)
	rule(".rpill",
		display("inline-flex"),
		alignItems("center"),
		background("#1a1a1d"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		padding(".2rem .25rem"),
	)
	rule(".rstep",
		background("transparent"),
		border("0"),
		color("#a6a6ac"),
		cursor("pointer"),
		padding("0 .3rem"),
		fontSize("1rem"),
		lineHeight("1"),
	)
	rule(".rstep:hover",
		color("#f4f4f5"),
	)
	rule(".rlabel",
		minWidth("128px"),
		textAlign("center"),
		fontSize(".8rem"),
	)
	rule(".rz",
		position("absolute"),
		zIndex("6"),
		opacity("0"),
		pointerEvents("none"),
		border("0"),
		padding("0"),
		background("transparent"),
		color("#d4d4d8"),
		transition("opacity .12s ease, background .12s ease, color .12s ease"),
	)
	rule(".w:hover .rz, .w:focus-within .rz, .w:focus-visible .rz",
		opacity(".9"),
		pointerEvents("auto"),
	)
	rule(".rz[data-dir=\"l\"], .rz[data-dir=\"r\"]",
		top("24px"),
		bottom("8px"),
		width("16px"),
		cursor("ew-resize"),
	)
	rule(".rz[data-dir=\"l\"]",
		left("0"),
	)
	rule(".rz[data-dir=\"r\"]",
		right("0"),
	)
	rule(".rz[data-dir=\"t\"], .rz[data-dir=\"b\"]",
		left("34px"),
		right("34px"),
		height("16px"),
		cursor("ns-resize"),
	)
	rule(".rz[data-dir=\"t\"]",
		top("0"),
	)
	rule(".rz[data-dir=\"b\"]",
		bottom("0"),
	)
	rule(".rz::before",
		content("\"\""),
		position("absolute"),
		borderRadius("999px"),
		background("rgba(212,212,216,.62)"),
		boxShadow("0 0 0 1px rgba(244,244,245,.12), 0 6px 18px rgba(0,0,0,.28)"),
	)
	rule(".rz[data-dir=\"l\"]::before, .rz[data-dir=\"r\"]::before",
		top("50%"),
		width("3px"),
		height("min(44px,42%)"),
		transform("translateY(-50%)"),
	)
	rule(".rz[data-dir=\"l\"]::before",
		left("4px"),
	)
	rule(".rz[data-dir=\"r\"]::before",
		right("4px"),
	)
	rule(".rz[data-dir=\"t\"]::before, .rz[data-dir=\"b\"]::before",
		left("50%"),
		width("min(48px,42%)"),
		height("3px"),
		transform("translateX(-50%)"),
	)
	rule(".rz[data-dir=\"t\"]::before",
		top("4px"),
	)
	rule(".rz[data-dir=\"b\"]::before",
		bottom("4px"),
	)
	rule(".rz[data-dir=\"l\"]::after, .rz[data-dir=\"r\"]::after",
		content("\"\""),
		position("absolute"),
		top("50%"),
		width("0"),
		height("0"),
		transform("translateY(-50%)"),
		borderTop("4px solid transparent"),
		borderBottom("4px solid transparent"),
	)
	rule(".rz[data-dir=\"l\"]::after",
		left("3px"),
		borderRight("5px solid currentColor"),
	)
	rule(".rz[data-dir=\"r\"]::after",
		right("3px"),
		borderLeft("5px solid currentColor"),
	)
	rule(".rz[data-dir=\"t\"]::after, .rz[data-dir=\"b\"]::after",
		content("\"\""),
		position("absolute"),
		left("50%"),
		width("0"),
		height("0"),
		transform("translateX(-50%)"),
		borderLeft("4px solid transparent"),
		borderRight("4px solid transparent"),
	)
	rule(".rz[data-dir=\"t\"]::after",
		top("3px"),
		borderBottom("5px solid currentColor"),
	)
	rule(".rz[data-dir=\"b\"]::after",
		bottom("3px"),
		borderTop("5px solid currentColor"),
	)
	rule(".rz:hover, .rz:focus-visible",
		opacity("1"),
		color("#fff"),
		background("rgba(244,244,245,.045)"),
	)
	rule(".rz:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".rz:hover::before, .rz:focus-visible::before",
		background("rgba(244,244,245,.78)"),
	)
	rule(".rz.off",
		opacity("0 !important"),
		pointerEvents("none !important"),
	)
	rule("main.cf-scroll",
		scrollbarWidth("thin"),
		scrollbarColor("#34343a transparent"),
		scrollBehavior("smooth"),
		// Bottom clearance for the fixed corner controls (scroll-to-top, the PWA
		// install button): without it they sit ON the last row of content —
		// e.g. covering a pager's Next button at the end of a page.
		prop("padding-bottom", "5.25rem"),
	)
	rule("main.cf-scroll::-webkit-scrollbar",
		width("11px"),
	)
	rule("main.cf-scroll::-webkit-scrollbar-track",
		background("transparent"),
	)
	rule("main.cf-scroll::-webkit-scrollbar-thumb",
		background("#2d2d33"),
		borderRadius("999px"),
		border("3px solid #0e0e0f"),
		backgroundClip("padding-box"),
	)
	rule("main.cf-scroll::-webkit-scrollbar-thumb:hover",
		background("#44444c"),
	)
	rule("main.cf-scroll::-webkit-scrollbar-thumb:active",
		background("#55555e"),
	)
	rule(".flip-backdrop",
		position("fixed"),
		inset("0"),
		background("rgba(4,4,6,.6)"),
		backdropFilter("blur(3px)"),
		display("grid"),
		placeItems("center"),
		opacity("0"),
		pointerEvents("none"),
		transition("opacity .28s"),
		zIndex("var(--z-modal)"),
	)
	rule(".flip-backdrop.show",
		opacity("1"),
		pointerEvents("auto"),
	)
	rule(".flip-wrap",
		width("384px"),
		maxWidth("92vw"),
		height("470px"),
		maxHeight("86vh"),
		perspective("1500px"),
	)
	rule(".flip-inner",
		position("relative"),
		width("100%"),
		height("100%"),
		transformStyle("preserve-3d"),
		transition("transform .55s cubic-bezier(.2,.75,.2,1)"),
		transform("rotateY(0) scale(.86)"),
	)
	rule(".flip-inner.flipped",
		transform("rotateY(180deg) scale(1)"),
	)
	rule(".flip-face",
		position("absolute"),
		inset("0"),
		backfaceVisibility("hidden"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		background("#121214"),
		overflow("hidden"),
		display("flex"),
		flexDirection("column"),
		boxShadow("0 30px 70px -20px rgba(0,0,0,.75)"),
	)
	rule(".flip-back",
		transform("rotateY(180deg)"),
	)
	rule(".set-h",
		display("flex"),
		alignItems("center"),
		gap(".5rem"),
		padding(".85rem 1rem"),
		borderBottom("1px solid #2a2a2c"),
	)
	rule(".set-h h3",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontSize("1.05rem"),
		fontWeight("600"),
		flex("1"),
		textAlign("center"),
	)
	rule(".set-close",
		background("transparent"),
		border("0"),
		color("#8a8a92"),
		cursor("pointer"),
		fontSize(".95rem"),
		width("1.5rem"),
		lineHeight("1"),
	)
	rule(".set-close:hover",
		color("#f4f4f5"),
	)
	rule(".set-body",
		flex("1"),
		overflowY("auto"),
		padding("1rem 1rem 1.5rem"),
		scrollbarWidth("thin"),
		scrollbarColor("#34343a transparent"),
	)
	rule(".set-body::-webkit-scrollbar",
		width("9px"),
	)
	rule(".set-body::-webkit-scrollbar-thumb",
		background("#2d2d33"),
		borderRadius("999px"),
		border("2px solid #121214"),
	)
	// Flush body: the scroll padding is dropped and the single form/body child fills the
	// full height as a flex column, so it can split into a scrolling field region
	// (.modal-scroll) and a pinned action bar (.modal-foot) that never scrolls off.
	rule(".set-body-flush",
		padding("0"),
		overflow("hidden"),
		display("flex"),
		flexDirection("column"),
	)
	rule(".set-body-flush > *",
		flex("1"),
		minHeight("0"),
		display("flex"),
		flexDirection("column"),
		gap("0"),
	)
	// The form roots carry their own min-height:100%/gap for the non-flush layout; inside
	// a flush body the flex chain owns sizing, so neutralize them or the fields overflow
	// and push the footer off the panel.
	rule(".set-body-flush > .acct-edit-form, .set-body-flush > form, .set-body-flush > .form-grid",
		minHeight("0"),
		gap("0"),
	)
	rule(".modal-scroll",
		flex("1"),
		minHeight("0"),
		overflowY("auto"),
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
		padding("1rem 1rem 0.9rem"),
		scrollbarWidth("thin"),
		scrollbarColor("#34343a transparent"),
	)
	rule(".modal-scroll::-webkit-scrollbar",
		width("9px"),
	)
	rule(".modal-scroll::-webkit-scrollbar-thumb",
		background("#2d2d33"),
		borderRadius("999px"),
		border("2px solid #121214"),
	)
	rule(".modal-foot",
		flexShrink("0"),
		display("flex"),
		justifyContent("flex-end"),
		gap("0.5rem"),
		padding("0.75rem 1rem"),
		borderTop("1px solid #2a2a2c"),
		background("#121214"),
	)
	// When a category/auto-budget list is the modal body's own scroll content (a direct
	// child of .modal-scroll), drop its independent max-height/scroll so .modal-scroll is
	// the single scroll region — no nested scrollbars. Nested pickers inside the add/edit
	// forms are NOT direct children, so they keep their own bounded box.
	rule(".modal-scroll > .autobudget-rows, .modal-scroll > .budgetcats-list",
		maxHeight("none"),
		overflowY("visible"),
	)
	rule(".set-label",
		fontSize(".7rem"),
		textTransform("uppercase"),
		letterSpacing(".12em"),
		color("#6c6c72"),
		margin(".7rem 0 .3rem"),
	)
	rule(".toggle-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		padding(".55rem 0"),
		// The standard hairline token — the old near-black #1f1f22 was invisible on the
		// dark surface, so long toggle lists read as one undivided wall.
		borderBottom("1px solid var(--border)"),
		fontSize(".9rem"),
	)
	// A bounded, ruled block for the settings module-visibility list (top cap; the rows
	// carry their own dividers) so it scans as a discrete group, not loose text.
	rule(".set-toggle-list",
		borderTop("1px solid var(--border)"),
		marginTop(".35rem"),
	)
	rule(".switch",
		width("40px"),
		height("24px"),
		borderRadius("999px"),
		background("#34343a"),
		position("relative"),
		cursor("pointer"),
		transition("background .2s"),
		flexShrink("0"),
	)
	rule(".switch::after",
		content("\"\""),
		position("absolute"),
		top("3px"),
		left("3px"),
		width("18px"),
		height("18px"),
		borderRadius("50%"),
		background("#9a9aa4"),
		transition(".2s"),
	)
	rule(".switch::after",
		transition("left var(--motion-standard) var(--ease-standard), background var(--motion-standard) var(--ease-standard)"),
	)
	rule(".switch.on",
		background("#3e7f5e"),
	)
	rule(".switch.on::after",
		left("19px"),
		background("#e6f3ec"),
	)
	rule(".swatch",
		width("24px"),
		height("24px"),
		borderRadius("6px"),
		cursor("pointer"),
		border("2px solid transparent"),
		transition("border-color .12s ease"),
	)
	rule(".swatch:hover",
		borderColor("var(--text-faint)"),
	)
	rule(".swatch.sel",
		borderColor("#f4f4f5"),
	)
	rule(".set-input",
		width("100%"),
		padding(".5rem .6rem"),
		background("#1a1a1d"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		color("#f4f4f5"),
		font("inherit"),
	)
	rule(".set-foot",
		padding(".75rem 1rem"),
		borderTop("1px solid #2a2a2c"),
		display("flex"),
		flexShrink("0"),
		gap(".5rem"),
		justifyContent("flex-end"),
	)
	rule(".set-btn",
		minWidth("96px"),
		padding("var(--btn-py,0.5rem) 1rem"),
		minHeight("44px"),
		borderRadius("4px"),
		cursor("pointer"),
		fontSize(".9rem"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".set-btn.save",
		background("#1f2c24"),
		border("1px solid #356b50"),
		color("#7fd0a3"),
		fontWeight("600"),
	)
	rule(".set-btn.save:hover",
		background("#26382d"),
	)
	rule(".set-btn.cancel",
		background("transparent"),
		border("1px solid #34343a"),
		color("#a6a6ac"),
		fontWeight("500"),
	)
	rule(".set-btn.cancel:hover",
		color("#f4f4f5"),
		borderColor("#44444c"),
	)
	// Account editor modal (flip modal Back body): a clean single-column form with a
	// pinned, full-width action row. Overrides the multi-column .form-grid so fields
	// stack and align in the narrow modal instead of flowing into misaligned columns.
	rule(".acct-edit-form",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
		minHeight("100%"),
	)
	// Edit-budget modal: pair the compact fields into two columns, and give the tracked-
	// category picker a bounded, bordered scroll box so it can't balloon the form.
	rule(".budget-edit-row",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(150px, 1fr))"),
		gap("0.75rem 0.9rem"),
	)
	rule(".acct-edit-form.budget-edit .budgetcats-list",
		maxHeight("184px"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-elev)"),
		padding("0.15rem 0.35rem"),
	)
	rule(".acct-edit-form .labeled-field",
		width("100%"),
	)
	rule(".acct-edit-form .field, .acct-edit-form .cf-suggest, .acct-edit-form select",
		width("100%"),
	)
	rule(".acct-edit-form .cf-adv-toggle",
		alignSelf("flex-start"),
		marginTop("0.1rem"),
	)
	// Action row at the bottom of the modal body: margin-top:auto pushes it down when
	// the form is short (no dead space); when the form is taller than the modal it sits
	// at the natural end and scrolls with the content. NOT sticky — a sticky row
	// overlaps the flowing fields (the Notes textarea / disclosure) once the form
	// overflows, which looked broken.
	rule(".acct-edit-actions",
		display("flex"),
		justifyContent("flex-end"),
		gap("0.5rem"),
		marginTop("auto"),
		paddingTop("0.9rem"),
		borderTop("1px solid #2a2a2c"),
	)
	// Cover editor: amount + full-shortfall button on one line, then a checkbox list of
	// source budgets, each with a ratio input and its computed share.
	rule(".cover-amount-row",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".cover-amount-row .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".cover-spread-label",
		display("block"),
		fontSize("0.9rem"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".cover-spread-sub",
		display("block"),
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		marginTop("0.05rem"),
	)
	// Spread header: the "Spread across" titles on the left, the ƒx (ratio-by-formula)
	// toggle on the right, mirroring the amount block's fixed/formula switch.
	rule(".cover-spread-head",
		display("flex"),
		alignItems("flex-start"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".cover-spread-titles",
		display("flex"),
		flexDirection("column"),
		minWidth("0"),
	)
	// The shared ratio formula input + its hint, shown when ƒx weighting is on.
	rule(".cover-weight-fx",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		margin("0.35rem 0 0.1rem"),
	)
	// A running caption above the source list — "Selected N · splitting $X" — so the
	// split total reads at a glance without scanning the rows.
	rule(".cover-selected-cap",
		display("block"),
		fontSize("0.75rem"),
		fontWeight("600"),
		color("var(--accent)"),
		textTransform("uppercase"),
		letterSpacing("0.03em"),
		margin("0.1rem 0 0"),
	)
	rule(".cover-selected-cap.is-empty",
		display("none"),
	)
	// The source list is bounded and scrolls internally, so the recurring toggle and
	// the Cancel/Cover actions below it stay visible without scrolling the whole modal.
	rule(".cover-sources",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		maxHeight("440px"),
		overflowY("auto"),
		paddingRight("0.25rem"),
		margin("0.15rem 0 0.1rem"),
	)
	// "Use full $X" reads as the accent shortcut it is, not a dead dark label (critic #4).
	rule(".cover-amount-row .btn",
		flexShrink("0"),
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		color("var(--accent)"),
		whiteSpace("nowrap"),
	)
	rule(".cover-amount-row .btn:hover",
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	// Amount block: the number/formula row, then a live evaluated preview or hint.
	rule(".cover-amount-block",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
	)
	rule(".cover-fx-toggle",
		flexShrink("0"),
		fontWeight("700"),
		minWidth("42px"),
	)
	rule(".cover-fx-toggle[aria-pressed=\"true\"]",
		borderColor("var(--accent)"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".cover-fx-preview",
		fontSize("0.85rem"),
		fontWeight("600"),
		color("var(--accent)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".cover-fx-err",
		fontSize("0.8rem"),
		color("var(--danger)"),
	)
	rule(".cover-fx-hint",
		fontSize("0.74rem"),
		color("var(--text-dim)"),
	)
	// The ratio-formula toggle carries a small "ratios" label before the ƒx glyph, so it
	// reads as a distinct control from the amount ƒx button (not a twin).
	rule(".cover-fx-ratio",
		gap("0.3rem"),
	)
	rule(".cover-fx-ratio-label",
		fontSize("0.72rem"),
		fontWeight("600"),
		letterSpacing("0.01em"),
		textTransform("lowercase"),
	)
	// Cover modal fills the panel height and only the SOURCE LIST scrolls — the amount,
	// spread controls, recurring toggle and the Cover/Cancel actions stay put (no
	// full-body scroll). The list flexes to absorb the overflow.
	rule(".acct-edit-form.cover-form",
		height("100%"),
		minHeight("0"),
	)
	rule(".cover-form .cover-sources",
		flex("1 1 auto"),
		minHeight("120px"),
		maxHeight("none"),
	)
	// Variable-name editor: the input, then a live "Generates budget_<slug>" chip so the
	// user can see exactly what handle their budget produces as they type.
	rule(".entity-var-block",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
	)
	rule(".entity-var-preview",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem"),
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	rule(".entity-var-preview-lead",
		color("var(--text-dim)"),
	)
	rule(".entity-var-chip",
		fontFamily("ui-monospace, SFMono-Regular, Menlo, monospace"),
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
		border("1px solid color-mix(in srgb, var(--accent) 35%, var(--border))"),
		borderRadius("0.4rem"),
		padding("0.1rem 0.4rem"),
	)
	rule(".entity-var-preview-fields",
		color("var(--text-faint, var(--text-dim))"),
		fontFamily("ui-monospace, SFMono-Regular, Menlo, monospace"),
		fontSize("0.72rem"),
	)
	rule(".cover-src-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem"),
		padding("0.55rem 0.7rem"),
		border("1px solid var(--border)"),
		borderLeft("3px solid transparent"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		transition("background 0.15s ease, border-color 0.15s ease"),
	)
	// Checked source: tinted with an accent left-stripe so the selection reads at a
	// glance (critic #5 + highest-leverage change). The checked rows sort to the top, so
	// a stronger tint + a heavier accent border makes them read as one clustered group.
	rule(".cover-src-row.is-checked",
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent) 35%, var(--border))"),
		borderLeftColor("var(--accent)"),
	)
	// Consecutive checked rows sit flush (tight cluster); trim the doubled gap between
	// them so the picked budgets read as one block rather than separate cards.
	rule(".cover-src-row.is-checked + .cover-src-row.is-checked",
		marginTop("-0.25rem"),
	)
	// The first unchecked row after the checked cluster gets extra breathing room,
	// separating "the split" from "add another source".
	rule(".cover-src-row.is-checked + .cover-src-row:not(.is-checked)",
		marginTop("0.6rem"),
	)
	rule(".cover-src-main",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		minWidth("0"),
		flex("1 1 auto"),
		cursor("pointer"),
	)
	// Branded checkbox (critic #1): no more native OS-blue box.
	rule(".cf-check",
		appearance("none"),
		width("17px"),
		height("17px"),
		flexShrink("0"),
		margin("0"),
		border("1.5px solid color-mix(in srgb, var(--text) 34%, transparent)"),
		borderRadius("4px"),
		background("transparent"),
		cursor("pointer"),
		position("relative"),
		transition("background 0.12s ease, border-color 0.12s ease"),
	)
	rule(".cf-check:checked",
		background("var(--accent)"),
		borderColor("var(--accent)"),
	)
	rule(".cf-check:checked::after",
		content("\"\""),
		position("absolute"),
		left("5px"),
		top("2px"),
		width("4px"),
		height("8px"),
		border("solid #06210f"),
		borderWidth("0 2px 2px 0"),
		transform("rotate(45deg)"),
	)
	rule(".cf-check:focus-visible",
		outline("2px solid color-mix(in srgb, var(--accent) 60%, transparent)"),
		outlineOffset("2px"),
	)
	// Payment-link flip modal (the transactions row ⋯ → "Mark as bill/subscription").
	// A summary card leading with the payment amount, then the picker, a live "links to"
	// preview, and a right-aligned Cancel / Link footer.
	rule(".txnlink-summary",
		display("flex"),
		alignItems("center"),
		gap(".75rem"),
		padding(".65rem .8rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius, 10px)"),
		background("var(--bg-elev)"),
	)
	rule(".txnlink-summary-main",
		display("flex"),
		flexDirection("column"),
		gap("1px"),
		minWidth("0"),
		flex("1 1 auto"),
	)
	rule(".txnlink-summary-desc",
		fontWeight("600"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".txnlink-summary-meta",
		fontSize("0.82rem"),
	)
	rule(".txnlink-summary-amount",
		fontSize("1.35rem"),
		fontWeight("600"),
		lineHeight("1"),
		whiteSpace("nowrap"),
		flexShrink("0"),
	)
	// "Links to" preview: an accent-tinted strip of chips echoing every pending link, so
	// Save's effect is visible before committing (both links save together).
	rule(".txnlink-preview",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap(".4rem"),
		padding(".5rem .65rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid color-mix(in srgb, var(--accent) 30%, transparent)"),
		background("color-mix(in srgb, var(--accent) 9%, transparent)"),
	)
	rule(".txnlink-preview-label",
		fontSize("0.8rem"),
		marginRight(".1rem"),
	)
	rule(".txnlink-chip",
		display("inline-flex"),
		alignItems("center"),
		padding(".15rem .5rem"),
		borderRadius("999px"),
		fontSize("0.8rem"),
		fontWeight("500"),
		border("1px solid color-mix(in srgb, var(--accent) 40%, transparent)"),
		background("var(--bg-card)"),
		color("var(--text)"),
	)
	rule(".txnlink-footer",
		display("flex"),
		justifyContent("flex-end"),
		gap(".5rem"),
		marginTop("auto"),
		paddingTop(".5rem"),
	)
	// "Count as a liability" toggle on the account add/edit forms (shown for the Other
	// type): a checkbox + a labelled two-line explanation.
	rule(".acct-liab-toggle",
		margin(".15rem 0 .35rem"),
		gap(".55rem"),
	)
	// Statement-import modal: the choose-file row.
	rule(".statement-drop",
		display("flex"),
		alignItems("center"),
		gap(".7rem"),
		flexWrap("wrap"),
	)
	rule(".statement-file",
		fontSize(".85rem"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	// Statement-import modal: the opt-in "keep a copy in this browser" checkbox row.
	rule(".statement-savedoc",
		marginTop(".35rem"),
		padding(".55rem .7rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-card)"),
	)
	// Auto-budget modal: a scrollable list of tunable per-category suggestions, each a
	// checkbox + name/slider + a live monthly figure, then a Cancel/Save footer.
	rule(".autobudget-rows",
		display("flex"),
		flexDirection("column"),
		gap(".2rem"),
		overflowY("auto"),
		maxHeight("46vh"),
		margin("0 -.25rem"),
		padding("0 .25rem"),
	)
	rule(".autobudget-row",
		display("grid"),
		gridTemplateColumns("auto 1fr auto"),
		alignItems("center"),
		gap(".7rem"),
		padding(".5rem .3rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".autobudget-row.is-off",
		opacity(".5"),
	)
	rule(".autobudget-pick",
		display("flex"),
		alignItems("center"),
	)
	rule(".autobudget-main",
		display("flex"),
		flexDirection("column"),
		gap(".2rem"),
		minWidth("0"),
	)
	rule(".autobudget-head",
		display("flex"),
		alignItems("center"),
		gap(".45rem"),
	)
	rule(".autobudget-name",
		fontWeight("600"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".autobudget-tag",
		fontSize(".7rem"),
		textTransform("uppercase"),
		letterSpacing(".04em"),
		padding(".05em .4em"),
		borderRadius("4px"),
		border("1px solid var(--border)"),
	)
	rule(".autobudget-controls",
		display("flex"),
		alignItems("center"),
		gap(".6rem"),
	)
	rule(".autobudget-slider",
		flex("1 1 auto"),
		width("auto"),
		minWidth("0"),
	)
	rule(".autobudget-readout",
		fontSize(".78rem"),
		whiteSpace("nowrap"),
		flexShrink("0"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		minWidth("8.5rem"),
		textAlign("right"),
	)
	rule(".autobudget-readout.is-tuned",
		color("var(--accent)"),
		fontWeight("600"),
	)
	rule(".autobudget-amt",
		fontSize("1.05rem"),
		fontWeight("600"),
		whiteSpace("nowrap"),
		justifySelf("end"),
	)
	rule(".autobudget-footer",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-end"),
		gap(".5rem"),
		marginTop("auto"),
		paddingTop(".6rem"),
	)
	rule(".autobudget-total",
		marginRight("auto"),
		fontSize(".9rem"),
	)
	// Budget category picker (multi-category budgets): a clean, scannable one-line
	// checklist — checkbox + name, with a subtle right-aligned "in <budget>" overlap tag.
	rule(".budgetcats-list",
		display("flex"),
		flexDirection("column"),
		overflowY("auto"),
		maxHeight("42vh"),
		margin("0 -.25rem"),
		padding("0 .25rem"),
	)
	rule(".budgetcat-row",
		display("flex"),
		alignItems("center"),
		gap(".6rem"),
		padding(".4rem .35rem"),
		borderRadius("8px"),
		cursor("pointer"),
	)
	rule(".budgetcat-row:hover",
		background("var(--hover)"),
	)
	rule(".budgetcat-name",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".budgetcat-row.is-on .budgetcat-name",
		fontWeight("600"),
	)
	rule(".budgetcat-also",
		flexShrink("0"),
		fontSize(".75rem"),
		color("var(--text-faint)"),
		fontStyle("italic"),
	)
	rule(".cover-src-name",
		fontWeight("600"),
	)
	rule(".cover-src-remain",
		fontSize("0.78rem"),
		color("var(--text-faint, #666)"),
		fontWeight("400"),
		whiteSpace("nowrap"),
	)
	rule(".cover-src-ratio",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem 0.5rem"),
		flexShrink("0"),
	)
	rule(".cover-src-ratio-label",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	// Keep the native number spinners on the ratio input so it can be stepped with the
	// browser up/down arrows (each step fires input → the shares recompute live). Fixed
	// width so it doesn't stretch to fill the wrapped flex row.
	rule(".cover-src-ratio .cover-src-weight",
		width("68px"),
		flex("0 0 68px"),
	)
	rule(".cover-src-weight:disabled",
		opacity("0.45"),
	)
	// "Use all remaining" toggle.
	rule(".cover-src-maxlabel",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.76rem"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		cursor("pointer"),
	)
	rule(".cover-src-shares",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-end"),
		minWidth("72px"),
	)
	rule(".cover-src-share",
		fontVariantNumeric("tabular-nums"),
		fontWeight("700"),
		fontSize("0.95rem"),
		color("var(--accent)"),
		whiteSpace("nowrap"),
	)
	// Over-allocated source: amber, not the same green as a valid share (critic UX bug).
	rule(".cover-src-share.is-over",
		color("#f59e0b"),
	)
	rule(".cover-src-avail",
		fontSize("0.72rem"),
		color("#f59e0b"),
		whiteSpace("nowrap"),
	)
	// Recurring toggle in the cover editor + the row badge.
	rule(".cover-recurring-toggle",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginTop("0.25rem"),
		fontSize("0.88rem"),
		cursor("pointer"),
	)
	rule(".budget-recurring",
		color("var(--accent)"),
		fontWeight("600"),
	)
	// One-time "Covered" flag — distinct from the accent recurring badge: a calmer
	// up/positive tone so continual vs 1x coverage read differently at a glance.
	rule(".budget-covered",
		color("var(--up, #54b884)"),
		fontWeight("600"),
	)
	rule(".acct-edit-actions .btn",
		minWidth("104px"),
		justifyContent("center"),
	)
	rule(".bento [class*=\"rounded-full\"][class*=\"overflow-hidden\"]",
		borderRadius("2px"),
	)
	rule("aside.rail nav",
		scrollbarWidth("none"),
	)
	rule("aside.rail nav::-webkit-scrollbar",
		width("0"),
		height("0"),
		display("none"),
	)
	rule("aside.rail",
		transition("width .34s cubic-bezier(.22,.61,.36,1)"),
		willChange("width"),
	)
	rule("aside.rail.collapsed",
		width("58px"),
	)
	rule("aside.rail .nv > span, aside.rail .brand-name",
		overflow("hidden"),
		whiteSpace("nowrap"),
		maxWidth("14rem"),
		opacity("1"),
		transform("none"),
		transition("max-width .34s cubic-bezier(.22,.61,.36,1), opacity .2s ease, transform .3s cubic-bezier(.22,.61,.36,1)"),
	)
	rule("aside.rail .nv",
		transition("gap .34s cubic-bezier(.22,.61,.36,1), padding .3s cubic-bezier(.22,.61,.36,1)"),
	)
	rule("aside.rail.collapsed .nv > span, aside.rail.collapsed .brand-name",
		maxWidth("0"),
		opacity("0"),
		transform("translateX(-10px) scale(.82)"),
		pointerEvents("none"),
	)
	rule("aside.rail.collapsed nav .rail-section,\n      aside.rail.collapsed .hh-text",
		display("none"),
	)
	rule("aside.rail.collapsed .nv",
		justifyContent("center"),
		gap("0"),
		paddingLeft("0"),
		paddingRight("0"),
		position("relative"),
	)
	rule("aside.rail.collapsed .cloud-mention,\n      aside.rail.collapsed .hh-quiet,\n      aside.rail.collapsed .rail-foot > span,\n      aside.rail.collapsed .rail-foot > a",
		display("none"),
	)
	rule(".nav-alt-hint",
		marginLeft("auto"),
		fontSize("0.7rem"),
		lineHeight("1"),
		color("var(--muted)"),
		opacity("0.7"),
		border("1px solid var(--border)"),
		borderRadius("4px"),
		padding("1px 4px"),
	)
	rule("aside.rail.collapsed .nav-alt-hint",
		display("none"),
	)
	rule("aside.rail.collapsed .railhead",
		paddingLeft("0"),
		paddingRight("0"),
		justifyContent("center"),
	)
	rule("aside.rail.collapsed .hh",
		justifyContent("center"),
		paddingLeft("0"),
		paddingRight("0"),
	)
	rule("aside.rail.collapsed .nv:hover > span,\n      aside.rail.collapsed .nv:focus-visible > span,\n      aside.rail.collapsed .nv:focus-within > span",
		display("block"),
		position("absolute"),
		left("calc(100% + 8px)"),
		top("50%"),
		transform("translateY(-50%)"),
		maxWidth("none"),
		opacity("1"),
		pointerEvents("auto"),
		whiteSpace("nowrap"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		padding("0.25rem 0.55rem"),
		fontSize("13px"),
		lineHeight("1"),
		boxShadow("0 6px 18px rgba(0,0,0,.28)"),
		zIndex("60"),
		pointerEvents("auto"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", "aside.rail.collapsed .nv:hover > span,\n        aside.rail.collapsed .nv:focus-visible > span,\n        aside.rail.collapsed .nv:focus-within > span",
		animation("rail-flyout .12s ease both"),
	)
	keyframes("rail-flyout",
		at("from",
			opacity("0"),
			transform("translate(-4px,-50%)"),
		),
		at("to",
			opacity("1"),
			transform("translateY(-50%)"),
		),
	)
	keyframes("rail-page-settle",
		at("from",
			transform("scale(.985)"),
		),
		at("to",
			transform("none"),
		),
	)
	rule("html.cf-rail-anim:not([data-wonder=\"off\"]) #cf-page-view",
		animation("rail-page-settle .4s cubic-bezier(.22,.61,.36,1)"),
		transformOrigin("left center"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", "html.cf-rail-anim #cf-page-view",
		animation("none !important"),
	)
	ruleMedia("(max-width: 767px)", "html, body, #app",
		overflowX("hidden"),
		maxWidth("100vw"),
	)
	ruleMedia("(max-width: 767px)", "aside.rail",
		width("56px"),
	)
	ruleMedia("(max-width: 767px)", "aside.rail .brand-name,\n        aside.rail .nv span,\n        aside.rail nav .rail-section,\n        aside.rail .hh-text",
		display("none"),
	)
	ruleMedia("(max-width: 767px)", "aside.rail .nv",
		justifyContent("center"),
		gap("0"),
		paddingLeft("0"),
		paddingRight("0"),
	)
	ruleMedia("(max-width: 767px)", "aside.rail .railhead",
		paddingLeft("0"),
		paddingRight("0"),
		justifyContent("center"),
	)
	ruleMedia("(max-width: 767px)", "aside.rail .hh",
		justifyContent("center"),
		paddingLeft("0"),
		paddingRight("0"),
	)
	ruleMedia("(max-width: 767px)", ".bento",
		gridTemplateColumns("1fr !important"),
		gridTemplateRows("none !important"),
		gridAutoRows("minmax(var(--cell), auto) !important"),
	)
	ruleMedia("(max-width: 767px)", ".bento > *",
		gridColumn("1 / -1 !important"),
		gridRow("auto !important"),
	)
	ruleMedia("(max-width: 1024px)", ".row",
		flexWrap("wrap"),
		rowGap(".4rem"),
	)
	ruleMedia("(min-width: 768px) and (max-width: 1024px)", ".bento",
		gridTemplateColumns("repeat(2, 1fr) !important"),
		gridTemplateRows("none !important"),
		gridAutoRows("minmax(var(--cell), auto) !important"),
	)
	ruleMedia("(min-width: 768px) and (max-width: 1024px)", ".bento > *",
		gridColumn("auto !important"),
		gridRow("auto !important"),
	)
	ruleMedia("(min-width: 768px) and (max-width: 1024px)", ".bento > *:first-child",
		gridColumn("1 / -1 !important"),
	)
	ruleMedia("(max-width: 767px)", ".page .stat-grid",
		gridTemplateColumns("repeat(auto-fit, minmax(120px, 1fr)) !important"),
	)
	rule(".menu-btn",
		background("transparent"),
		border("0"),
		color("#a6a6ac"),
		cursor("pointer"),
		display("grid"),
		placeItems("center"),
	)
	rule(".add-wrap",
		position("relative"),
		display("inline-flex"),
		alignItems("center"),
		gap("1px"),
	)
	rule(".add-caret",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("20px"),
		height("30px"),
		background("transparent"),
		border("none"),
		color("var(--text-dim)"),
		cursor("pointer"),
		borderRadius("6px"),
		transition("background .12s ease, color .12s ease"),
	)
	rule(".add-caret:hover",
		color("var(--text)"),
		background("var(--bg-elev)"),
	)
	rule("[data-theme=\"light\"] .add-caret",
		border("1px solid var(--border)"),
	)
	rule(".add-menu",
		position("absolute"),
		left("0"),
		right("auto"),
		top("calc(100% + 6px)"),
		zIndex("var(--z-dropdown)"),
		minWidth("210px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		padding("4px"),
		boxShadow("0 8px 24px rgba(0,0,0,.3)"),
		display("flex"),
		flexDirection("column"),
		gap("2px"),
	)
	// The smart explainer reuses the .add-menu overlay (absolute + edge-flip) but holds
	// prose, not a button list — so it's a fixed-width block with comfortable padding.
	rule(".smart-tip-pop.add-menu",
		position("fixed"),
		zIndex("var(--z-popover)"),
		top("auto"),
		left("auto"),
		display("block"),
		width("264px"),
		maxWidth("min(264px, 80vw)"),
		padding("0.6rem 0.75rem"),
		whiteSpace("normal"),
		cursor("default"),
		textTransform("none"),
		letterSpacing("normal"),
		textAlign("left"),
		color("var(--text)"),
	)
	rule(".smart-tip-pop-title",
		fontWeight("600"),
		fontSize("0.82rem"),
		color("var(--text)"),
	)
	rule(".smart-tip-pop-text",
		margin("0.25rem 0 0"),
		fontSize("0.78rem"),
		lineHeight("1.4"),
		color("var(--text-dim)"),
	)
	rule(".add-menu.open-left",
		left("auto"),
		right("0"),
	)
	rule(".add-menu.open-up",
		top("auto"),
		bottom("calc(100% + 6px)"),
	)
	rule(".add-item",
		// Flex row so an optional leading icon sits INLINE with the label — without
		// this the icon svg breaks onto its own line and every icon-bearing overflow
		// menu (e.g. the transactions "⋯ More") renders as stacked two-line items.
		display("flex"),
		alignItems("center"),
		gap(".45rem"),
		textAlign("left"),
		padding(".5rem .7rem"),
		borderRadius("5px"),
		background("transparent"),
		border("0"),
		color("var(--text)"),
		cursor("pointer"),
		font("inherit"),
		fontSize("13px"),
	)
	rule(".add-item:hover, .add-item:focus-visible",
		background("var(--hover)"),
	)
	// A bento tile whose overflow ("⋯ More") menu is open must rise above its neighbours,
	// or the dropdown is painted UNDER the next tile and its items can't be clicked (each
	// tile is its own stacking context). Scope to the open state via the trigger's
	// aria-expanded so closed tiles keep their normal stacking.
	rule(".w:has(.add-wrap button[aria-expanded=\"true\"])",
		position("relative"),
		zIndex("40"),
	)
	// The open menu itself sits above sibling content within the raised tile.
	rule(".add-wrap button[aria-expanded=\"true\"] + .add-menu",
		zIndex("41"),
	)
	// Destructive overflow-menu item (e.g. Delete account): red text, red hover.
	rule(".add-item.danger",
		color("var(--cf-neg,#e5807e)"),
	)
	rule(".add-item.danger:hover, .add-item.danger:focus-visible",
		background("#3b1414"),
		color("#f4a3a1"),
	)
	rule(".add-backdrop",
		position("fixed"),
		inset("0"),
		zIndex("40"),
	)
	rule(".hidden-menu",
		display("none"),
	)
	rule(".add-wrap:has(> .add-menu:not(.hidden-menu):not(.hidden))",
		zIndex("51"),
	)
	rule(".row:has(.add-menu:not(.hidden-menu):not(.hidden)),\n      .budget:has(.add-menu:not(.hidden-menu):not(.hidden)),\n      li:has(> .add-wrap > .add-menu:not(.hidden-menu):not(.hidden))",
		position("relative"),
		zIndex("51"),
	)
	// Table rows (e.g. the transactions DataTable) render as <tr>/<td>, which the
	// selectors above don't match. When a row ⋯ menu is open the cell holding it must
	// (a) rise above the sibling cells so its popover wins the hit-test, and (b) NOT
	// clip the popover — the actions cell computes `overflow:hidden` (fixed 96px width),
	// which clips the wider open-left menu so its item paints nowhere and the cell
	// underneath (e.g. td-user) swallows the click. Lift + un-clip the open cell.
	rule("td:has(> .add-wrap > .add-menu:not(.hidden-menu):not(.hidden))",
		position("relative"),
		zIndex("51"),
		overflow("visible"),
	)
	rule(".menu-btn:hover",
		color("#f4f4f5"),
	)
	rule(".member-chip",
		display("inline-flex"),
		alignItems("center"),
		gap(".4rem"),
		padding(".25rem .6rem"),
		background("#1a1a1d"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		fontSize(".8rem"),
	)
	rule(".member-add",
		padding(".25rem .6rem"),
		border("1px dashed #3a3a40"),
		borderRadius("4px"),
		color("#a6a6ac"),
		fontSize(".8rem"),
		cursor("pointer"),
		background("transparent"),
	)
	rule(".member-add:hover",
		color("#f4f4f5"),
	)
	rule(".rate-row",
		display("flex"),
		alignItems("center"),
		gap(".6rem"),
		padding(".25rem 0"),
		fontSize(".85rem"),
	)
	// Both the freshness rows (.rate-row) and the Alerts threshold rows
	// (.toggle-row) put a .rate-in number input on the right; share the 90px
	// width so the two tabs don't drift to different input sizes.
	rule(".rate-row .rate-in, .toggle-row .rate-in",
		width("90px"),
		padding(".35rem .5rem"),
		background("#1a1a1d"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		color("#f4f4f5"),
		font("inherit"),
		textAlign("right"),
	)
	rule(".data-btn",
		padding(".45rem .7rem"),
		border("1px solid #34343a"),
		borderRadius("4px"),
		color("#d6d6da"),
		background("#1a1a1d"),
		cursor("pointer"),
		fontSize(".82rem"),
	)
	rule(".data-btn:hover",
		borderColor("#44444c"),
		color("#f4f4f5"),
	)
	rule(".data-btn-danger",
		color("var(--danger, #d8716f)"),
		borderColor("var(--danger-muted, #5a2a2a)"),
	)
	rule(".data-btn-danger:hover",
		color("var(--danger, #d8716f)"),
		borderColor("var(--danger, #d8716f)"),
	)
	ruleMedia("(max-width: 768px)", ".flip-wrap div[style*=\"grid-template-columns\"]",
		gridTemplateColumns("1fr !important"),
	)
	rule(".wh",
		display("flex"),
		alignItems("center"),
		gap(".55rem"),
		padding(".7rem .85rem .55rem"),
	)
	rule(".wh h2, .wh h3, .wh .wh-title",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontSize("1rem"),
		fontWeight("600"),
		letterSpacing("-0.01em"),
		flex("1"),
		textAlign("center"),
	)
	rule(".wh .wh-title",
		background("none"),
		border("0"),
		padding("0"),
		margin("0"),
		color("inherit"),
		cursor("pointer"),
		lineHeight("inherit"),
	)
	rule(".wh .wh-title:hover, .wh .wh-title:focus-visible",
		color("var(--accent)"),
		textDecoration("underline"),
		textUnderlineOffset("3px"),
	)
	rule(".wh .grip",
		width("1.1rem"),
		marginRight(".15rem"),
	)
	rule(".grip",
		cursor("grab"),
		color("#4f4f55"),
		display("inline-flex"),
		alignItems("center"),
		lineHeight("1"),
	)
	// Sample-data chip (audit P0): a top-bar status chip, not a banner row — so
	// it must stay on one line and never add vertical chrome above the content.
	rule(".sample-banner",
		display("inline-flex"),
		flexWrap("nowrap"),
		whiteSpace("nowrap"),
		alignItems("center"),
		gap(".45rem"),
		padding(".2rem .45rem .2rem .5rem"),
		maxWidth("100%"),
		background("rgba(211,146,0,0.10)"),
		border("1px solid #d39200"),
		borderRadius("999px"),
		fontSize(".75rem"),
	)
	rule("[data-theme=\"light\"] .sample-banner",
		background("#fff8e1"),
		borderColor("#d39200"),
	)
	rule(".sample-banner-icon",
		width("14px"),
		height("14px"),
		flex("0 0 auto"),
		color("#cfa14e"),
		strokeWidth("2"),
	)
	rule("[data-theme=\"light\"] .sample-banner-icon",
		color("#92400e"),
	)
	rule(".sample-banner-text",
		color("var(--text)"),
		fontWeight("500"),
	)
	rule("[data-theme=\"light\"] .sample-banner-text",
		color("#92400e"),
	)
	rule(".sample-banner-actions",
		display("flex"),
		alignItems("center"),
		gap(".6rem"),
	)
	rule(".sample-banner-btn",
		background("transparent"),
		border("0"),
		padding(".15rem .2rem"),
		font("inherit"),
		color("#cfa14e"),
		cursor("pointer"),
		textDecoration("underline"),
		textUnderlineOffset("3px"),
		fontWeight("500"),
	)
	rule(".sample-banner-btn:hover",
		textDecorationThickness("2px"),
		color("var(--text)"),
	)
	rule("[data-theme=\"light\"] .sample-banner-btn",
		color("#92400e"),
	)
	rule("[data-theme=\"light\"] .sample-banner-btn:hover",
		color("#1c1c1e"),
	)
	// At phone widths the top bar has no room for the labeled chip: it squeezed
	// into an unreadable amber sliver between the menu and period controls. The
	// icon+label keep a readable floor and the actions drop; the full banner
	// behavior remains available at desktop widths. (Desktop-first per the
	// standing directive — this is a don't-look-broken guard, not a mobile flow.)
	ruleMedia("(max-width: 640px)", ".sample-banner",
		flexShrink("0"),
	)
	ruleMedia("(max-width: 640px)", ".sample-banner-actions",
		display("none"),
	)
	// The chip's session-dismiss ✕: quiet icon button, same amber family.
	rule(".sample-banner-x",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		background("transparent"),
		border("0"),
		padding(".1rem"),
		borderRadius("999px"),
		color("#cfa14e"),
		cursor("pointer"),
	)
	rule(".sample-banner-x:hover",
		color("var(--text)"),
	)
	rule("[data-theme=\"light\"] .sample-banner-x",
		color("#92400e"),
	)
	rule("[data-theme=\"light\"] .sample-banner-x:hover",
		color("#1c1c1e"),
	)
	rule(".scope-banner",
		display("inline-flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap(".5rem"),
		padding(".25rem .5rem .25rem .65rem"),
		marginBottom(".6rem"),
		maxWidth("100%"),
		background("var(--accent-dim, rgba(0,122,255,0.12))"),
		border("1px solid var(--accent, #007aff)"),
		borderRadius("999px"),
		fontSize(".8rem"),
	)
	rule(".scope-banner-text",
		color("var(--text)"),
		fontWeight("500"),
	)
	rule(".scope-banner-btn",
		background("transparent"),
		border("0"),
		padding(".15rem .25rem"),
		font("inherit"),
		color("var(--accent, #007aff)"),
		cursor("pointer"),
		textDecoration("underline"),
		textUnderlineOffset("3px"),
		fontWeight("500"),
	)
	rule(".scope-banner-btn:hover",
		textDecorationThickness("2px"),
		color("var(--text)"),
	)
	rule("[data-theme=\"light\"] .scope-banner",
		background("#e8f0fe"),
		borderColor("#1a56db"),
	)
	rule("[data-theme=\"light\"] .scope-banner-text",
		color("#1a3a6c"),
	)
	rule("[data-theme=\"light\"] .scope-banner-btn",
		color("#1a56db"),
	)
	rule("[data-theme=\"light\"] .scope-banner-btn:hover",
		color("#1c1c1e"),
	)
	rule(".wbody",
		padding("0 .7rem .7rem"),
		flex("1"),
		minHeight("0"),
	)
	rule(".kpi",
		padding(".95rem 1rem"),
	)
	rule(".card.smart-strip-bento",
		borderRadius("0 !important"),
		borderColor("#34343a"),
		boxShadow("none"),
		margin("10px 0 0"),
	)
	rule("[data-theme=\"light\"] .card.smart-strip-bento",
		borderColor("#e4e2dd"),
	)
	rule(".w.chrome-hover",
		background("transparent !important"),
		borderColor("transparent !important"),
		boxShadow("none !important"),
	)
	rule(".w.chrome-hover > .wbody",
		padding("0"),
	)
	rule(".w.chrome-hover > .wh",
		position("absolute"),
		top(".45rem"),
		right(".55rem"),
		left("auto"),
		width("auto"),
		padding("0"),
		margin("0"),
		zIndex("2"),
		opacity("0"),
		transition("opacity .15s var(--wonder-ease, ease)"),
	)
	rule(".w.chrome-hover > .wh h2",
		display("none"),
	)
	rule(".w.chrome-hover > .wh .grip",
		display("none"),
	)
	rule(".w.chrome-hover .rz",
		opacity("0"),
		transition("opacity .15s var(--wonder-ease, ease)"),
	)
	rule(".w.chrome-hover:hover",
		background("var(--bg-card) !important"),
		borderColor("var(--border) !important"),
	)
	rule(".w.chrome-hover:hover > .wh",
		opacity("1"),
	)
	rule(".w.chrome-hover:hover .rz",
		opacity("1"),
	)
	ruleMedia("(hover:none)", ".w.chrome-hover > .wh",
		opacity("1"),
	)
	rule(".insights-answer",
		fontSize("14px"),
		lineHeight("1.55"),
		wordWrap("break-word"),
		overflowWrap("anywhere"),
	)
	rule(".insights-answer > :first-child",
		marginTop("0"),
	)
	rule(".insights-answer > :last-child",
		marginBottom("0"),
	)
	rule(".insights-answer p",
		margin(".5rem 0"),
	)
	rule(".insights-answer h1,.insights-answer h2,.insights-answer h3,.insights-answer h4",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontWeight("600"),
		lineHeight("1.25"),
		margin(".9rem 0 .35rem"),
	)
	rule(".insights-answer h1",
		fontSize("1.25rem"),
	)
	rule(".insights-answer h2",
		fontSize("1.1rem"),
	)
	rule(".insights-answer h3",
		fontSize("1rem"),
	)
	rule(".insights-answer h4",
		fontSize(".95rem"),
	)
	rule(".insights-answer ul,.insights-answer ol",
		margin(".5rem 0"),
		paddingLeft("1.25rem"),
	)
	rule(".insights-answer ul",
		listStyle("disc"),
	)
	rule(".insights-answer ol",
		listStyle("decimal"),
	)
	rule(".insights-answer li",
		margin(".2rem 0"),
	)
	rule(".insights-answer li > ul,.insights-answer li > ol",
		margin(".15rem 0"),
	)
	rule(".insights-answer strong",
		fontWeight("700"),
	)
	rule(".insights-answer em",
		fontStyle("italic"),
	)
	rule(".insights-answer a",
		color("var(--accent,#3b82f6)"),
		textDecoration("underline"),
		textUnderlineOffset("2px"),
	)
	rule(".insights-answer code",
		fontFamily("ui-monospace,SFMono-Regular,Menlo,monospace"),
		fontSize(".85em"),
		background("rgba(120,120,130,.16)"),
		padding(".1em .35em"),
		borderRadius("4px"),
	)
	rule(".insights-answer pre",
		background("rgba(120,120,130,.16)"),
		padding(".7rem .85rem"),
		borderRadius("8px"),
		overflowX("auto"),
		margin(".6rem 0"),
	)
	rule(".insights-answer pre code",
		background("none"),
		padding("0"),
		fontSize(".82em"),
	)
	rule(".insights-answer blockquote",
		borderLeft("3px solid var(--accent,#3b82f6)"),
		margin(".6rem 0"),
		padding(".1rem .8rem"),
		opacity(".85"),
	)
	rule(".insights-answer hr",
		border("0"),
		borderTop("1px solid rgba(120,120,130,.3)"),
		margin(".8rem 0"),
	)
	rule(".insights-answer table",
		borderCollapse("collapse"),
		margin(".6rem 0"),
		fontSize(".9em"),
	)
	rule(".insights-answer th,.insights-answer td",
		border("1px solid rgba(120,120,130,.3)"),
		padding(".3rem .55rem"),
		textAlign("left"),
	)
	rule(".insights-answer th",
		background("rgba(120,120,130,.12)"),
		fontWeight("600"),
	)
	keyframes("cf-jump-flash-kf",
		at("0%",
			background("var(--accent,#3b82f6)"),
		),
		at("100%",
			background("transparent"),
		),
	)
	rule(".cf-jump-flash",
		animation("cf-jump-flash-kf 1.6s ease-out"),
		borderRadius("8px"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".cf-jump-flash",
		animation("none"),
	)
	rule(".attention-list",
		display("flex"),
		flexDirection("column"),
		gap(".4rem"),
	)
	rule(".attention-chips",
		display("flex"),
		flexWrap("wrap"),
		gap(".5rem"),
		alignItems("stretch"),
	)
	// Let the pills grow to fill each row (two per row on a wide card) so a lone trailing
	// item stretches to fill instead of stranding a half-empty row — on a card whose whole
	// job is to signal urgency, a dead row undercuts it.
	rule(".attention-chips .attention-item",
		flex("1 1 40%"),
		minWidth("280px"),
	)
	rule(".attention-item",
		display("inline-flex"),
		alignItems("center"),
		gap(".5rem"),
		textAlign("left"),
		background("transparent"),
		border("1px solid var(--border)"),
		borderRadius("9px"),
		padding(".3rem .6rem"),
		cursor("pointer"),
		fontSize("13.5px"),
		color("inherit"),
		lineHeight("1.3"),
		transition("background .12s ease, border-color .12s ease"),
	)
	rule(".attention-list .attention-item",
		width("100%"),
	)
	rule(".attention-item:hover",
		background("color-mix(in srgb, var(--accent,#3b82f6) 8%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 35%, var(--border))"),
	)
	rule(".attention-item:focus-visible",
		outline("2px solid var(--accent,#3b82f6)"),
		outlineOffset("1px"),
	)
	rule(".attention-dot",
		fontSize("11px"),
		lineHeight("1"),
		flex("none"),
	)
	rule(".attention-text",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".attention-item.is-critical",
		borderLeft("3px solid var(--danger,#dc2626)"),
	)
	rule(".attention-item.is-critical .attention-dot",
		color("var(--danger,#dc2626)"),
	)
	rule(".attention-item.is-warning",
		borderLeft("3px solid var(--warn,#d97706)"),
	)
	rule(".attention-item.is-warning .attention-dot",
		color("var(--warn,#d97706)"),
	)
	rule(".attention-item.is-info .attention-dot",
		color("var(--text-dim)"),
	)
	rule(".wm .card + .card",
		marginTop("14px"),
	)
	rule(".wm-toolbar",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap(".6rem"),
	)
	rule(".wm-sep",
		width("1px"),
		alignSelf("stretch"),
		minHeight("20px"),
		background("var(--border)"),
		margin("0 .2rem"),
	)
	rule(".wm-table-wrap",
		overflowX("auto"),
		webkitOverflowScrolling("touch"),
	)
	rule(".wm-table",
		width("100%"),
		maxWidth("780px"),
	)
	rule(".wm-table th, .wm-table td",
		verticalAlign("middle"),
		paddingTop(".45rem"),
		paddingBottom(".45rem"),
	)
	rule(".wm-table th.wm-col-vis, .wm-table th.wm-col-size, .wm-table th.wm-col-order",
		textAlign("center"),
	)
	rule(".wm-col-vis",
		width("4.5rem"),
		textAlign("center"),
	)
	rule(".wm-col-size",
		width("1%"),
		whiteSpace("nowrap"),
	)
	rule(".wm-col-order",
		width("1%"),
		whiteSpace("nowrap"),
	)
	rule(".wm-name",
		fontWeight("600"),
	)
	rule(".wm-row.is-hidden .wm-name",
		color("var(--text-dim)"),
		textDecoration("line-through"),
	)
	rule(".wm-row.is-hidden td:not(.wm-cell-name)",
		opacity(".5"),
	)
	rule(".wm-col-vis .switch",
		margin("0 auto"),
	)
	rule(".wm-size",
		display("inline-flex"),
		alignItems("center"),
		gap(".5rem"),
	)
	rule(".wm-step",
		display("inline-flex"),
		alignItems("stretch"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		overflow("hidden"),
	)
	rule(".wm-step-btn",
		width("26px"),
		height("28px"),
		border("0"),
		background("transparent"),
		cursor("pointer"),
		color("inherit"),
		fontSize("15px"),
		lineHeight("1"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		transition("background .12s ease"),
	)
	rule(".wm-step-btn:hover",
		background("color-mix(in srgb, var(--accent,#3b82f6) 10%, transparent)"),
	)
	rule(".wm-step-val",
		minWidth("2.6rem"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		fontSize("12px"),
		color("var(--text-dim)"),
		borderLeft("1px solid var(--border)"),
		borderRight("1px solid var(--border)"),
		padding("0 .2rem"),
	)
	rule(".wm-reorder",
		display("inline-flex"),
		gap(".3rem"),
		justifyContent("center"),
	)
	rule(".wm-stack",
		display("grid"),
		alignItems("center"),
		justifyItems("start"),
	)
	rule(".wm-stack > *",
		gridArea("1 / 1"),
	)
	rule(".wm-col-order .wm-stack",
		justifyItems("center"),
	)
	rule(".wm-static",
		opacity("1"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
		transition("opacity var(--motion-fast, 120ms) var(--ease-standard, ease)"),
	)
	rule(".wm-size, .wm-reorder",
		opacity("0"),
		transition("opacity var(--motion-fast, 120ms) var(--ease-standard, ease)"),
	)
	rule(".wm-row:hover .wm-static, .wm-row:focus-within .wm-static",
		opacity("0"),
	)
	rule(".wm-row:hover .wm-size, .wm-row:hover .wm-reorder,\n      .wm-row:focus-within .wm-size, .wm-row:focus-within .wm-reorder",
		opacity("1"),
	)
	rule(".wm-arrow",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("28px"),
		height("28px"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("transparent"),
		cursor("pointer"),
		color("inherit"),
		lineHeight("1"),
		transition("background .12s ease, border-color .12s ease"),
	)
	rule(".wm-arrow:hover:not(:disabled)",
		background("color-mix(in srgb, var(--accent,#3b82f6) 10%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 35%, var(--border))"),
	)
	rule(".wm-arrow:disabled",
		opacity(".3"),
		cursor("not-allowed"),
	)
	rule(".wm-style",
		display("grid"),
		gridTemplateColumns("minmax(0,1fr) 300px"),
		gap("1.75rem"),
		alignItems("start"),
	)
	ruleMedia("(max-width:820px)", ".wm-style",
		gridTemplateColumns("1fr"),
	)
	rule(".wm-style-grid",
		display("grid"),
		gridTemplateColumns("repeat(2,minmax(0,1fr))"),
		gap(".5rem 1.25rem"),
		marginTop(".75rem"),
	)
	ruleMedia("(max-width:560px)", ".wm-style-grid",
		gridTemplateColumns("1fr"),
	)
	rule(".wm-style-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap(".75rem"),
		minHeight("34px"),
	)
	rule(".wm-style-label",
		fontSize("13px"),
		color("var(--text-dim)"),
	)
	rule(".wm-style-color",
		display("inline-flex"),
		alignItems("center"),
		gap(".4rem"),
	)
	rule(".wm-color",
		width("2rem"),
		height("1.7rem"),
		padding("0"),
		border("1px solid var(--border)"),
		borderRadius("6px"),
		background("none"),
		cursor("pointer"),
	)
	rule(".wm-clear",
		width("1.4rem"),
		height("1.4rem"),
		border("1px solid var(--border)"),
		borderRadius("5px"),
		background("transparent"),
		cursor("pointer"),
		color("var(--text-dim)"),
		lineHeight("1"),
		fontSize("13px"),
	)
	rule(".wm-clear:hover",
		color("var(--down,#dc2626)"),
		borderColor("var(--down,#dc2626)"),
	)
	rule(".wm-style-select",
		minWidth("8.5rem"),
	)
	rule(".wm-style-preview",
		position("sticky"),
		top("1rem"),
		display("flex"),
		flexDirection("column"),
		gap(".5rem"),
	)
	rule(".wm-preview-label",
		fontSize("11px"),
		textTransform("uppercase"),
		letterSpacing(".06em"),
		color("var(--text-faint)"),
	)
	rule(".wm-preview-tile",
		position("static !important"),
		minHeight("120px"),
	)
	rule(".wb",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
	)
	rule(".wb-stage",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		minHeight("200px"),
		padding("1.5rem"),
		borderRadius("12px"),
		background("var(--bg,#0e0e10)"),
		backgroundImage("radial-gradient(circle, color-mix(in srgb, var(--text-dim) 22%, transparent) 1px, transparent 1px)"),
		backgroundSize("16px 16px"),
		overflow("auto"),
	)
	rule(".wb-tile",
		position("static !important"),
		transition("width .15s ease, height .15s ease"),
	)
	rule(".wb-size",
		display("flex"),
		flexWrap("wrap"),
		gap(".75rem"),
	)
	rule(".wb-canvas",
		display("flex"),
		alignItems("center"),
		gap("0"),
		minHeight("128px"),
		padding("1.5rem 1rem"),
		borderRadius("12px"),
		border("1px solid var(--border)"),
		background("var(--bg,#0e0e10)"),
		backgroundImage("radial-gradient(circle, color-mix(in srgb, var(--text-dim) 22%, transparent) 1px, transparent 1px)"),
		backgroundSize("16px 16px"),
		overflowX("auto"),
	)
	rule(".wb-node",
		position("relative"),
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap(".15rem"),
		minWidth("8.5rem"),
		padding(".7rem .9rem"),
		borderRadius("10px"),
		cursor("pointer"),
		textAlign("left"),
		color("inherit"),
		background("var(--bg-elev,#1a1a1d)"),
		border("1.5px solid var(--border)"),
		transition("border-color .12s ease, box-shadow .12s ease, transform .12s ease"),
	)
	rule(".wb-node:hover",
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 45%, var(--border))"),
	)
	rule(".wb-node.is-active",
		borderColor("var(--accent,#3b82f6)"),
		boxShadow("0 0 0 3px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)"),
	)
	rule(".wb-node-kind",
		fontSize("11px"),
		textTransform("uppercase"),
		letterSpacing(".06em"),
		color("var(--text-faint)"),
	)
	rule(".wb-node-val",
		fontSize("14px"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".wb-port",
		position("absolute"),
		top("50%"),
		width("9px"),
		height("9px"),
		borderRadius("999px"),
		background("var(--bg,#0e0e10)"),
		border("1.5px solid var(--text-dim)"),
		transform("translateY(-50%)"),
	)
	rule(".wb-port-in",
		left("-5px"),
	)
	rule(".wb-port-out",
		right("-5px"),
	)
	rule(".wb-node.is-active .wb-port",
		borderColor("var(--accent,#3b82f6)"),
	)
	rule(".wb-edge",
		position("relative"),
		flex("0 0 2.75rem"),
		height("2px"),
		background("var(--border)"),
	)
	rule(".wb-edge::after",
		content("\"\""),
		position("absolute"),
		right("-1px"),
		top("50%"),
		transform("translateY(-50%)"),
		borderTop("5px solid transparent"),
		borderBottom("5px solid transparent"),
		borderLeft("7px solid var(--border)"),
	)
	rule(".muzak-btn, .notify-btn, .add-btn",
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("30px"),
		height("30px"),
		border("0"),
		borderRadius("8px"),
		background("var(--bg-elev,#1a1a1d)"),
		cursor("pointer"),
		color("var(--accent,#3b82f6)"),
		fontSize("15px"),
		lineHeight("1"),
		transition("background .12s ease, color .12s ease, opacity .12s ease"),
	)
	rule(".muzak-btn:hover, .notify-btn:hover, .add-btn:hover",
		background("color-mix(in srgb, var(--accent,#3b82f6) 18%, var(--bg-elev,#1a1a1d))"),
	)
	rule(".muzak-btn, .notify-btn, .add-btn",
		transition("background .12s ease, color .12s ease, opacity .12s ease, transform var(--wonder-dur-fast) var(--wonder-ease)"),
	)
	rule(".muzak-btn.is-off",
		color("var(--text-faint)"),
		opacity(".7"),
	)
	rule(".muzak-btn.relative",
		position("relative"),
	)
	rule(".notify-badge",
		position("absolute"),
		top("-3px"),
		right("-3px"),
		minWidth("15px"),
		height("15px"),
		padding("0 3px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		fontSize("9px"),
		fontWeight("700"),
		lineHeight("1"),
		color("#fff"),
		background("var(--action-danger,#c0392b)"),
		borderRadius("999px"),
	)
	rule(".set-range",
		width("11rem"),
		accentColor("var(--accent,#3b82f6)"),
		cursor("pointer"),
	)
	rule(".dash-check",
		flex("none"),
		width("18px"),
		height("18px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		background("transparent"),
		border("0"),
		cursor("pointer"),
		color("var(--text-dim)"),
		fontSize("15px"),
		lineHeight("1"),
	)
	rule(".dash-check[aria-checked=\"true\"]",
		color("var(--up,#16a34a)"),
	)
	rule(".dash-check:hover",
		color("var(--accent,#3b82f6)"),
	)
	rule(".cf-dialog-backdrop",
		position("fixed"),
		inset("0"),
		// Above flip-panel modals (--z-modal) so a confirm/prompt triggered from inside a
		// modal — e.g. Merge/Delete in the Review-duplicates modal — is reachable, not
		// trapped behind the modal's backdrop.
		zIndex("var(--z-dialog)"),
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		padding("1rem"),
	)
	rule(".cf-dialog-scrim",
		position("absolute"),
		inset("0"),
		background("rgba(0,0,0,.45)"),
	)
	rule(".cf-dialog",
		position("relative"),
		zIndex("1"),
		width("min(28rem,100%)"),
		background("var(--bg-card,#fff)"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		boxShadow("0 12px 40px rgba(0,0,0,.3)"),
		padding("1.25rem 1.25rem 1rem"),
	)
	rule(".cf-dialog-title",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontSize("1.05rem"),
		fontWeight("600"),
		margin("0 0 .35rem"),
	)
	rule(".cf-dialog-msg",
		color("var(--text,inherit)"),
		margin("0 0 .9rem"),
		lineHeight("1.5"),
	)
	rule(".cf-dialog-input",
		width("100%"),
		marginBottom("1rem"),
	)
	rule(".cf-dialog-actions",
		display("flex"),
		justifyContent("flex-end"),
		gap(".5rem"),
	)
	rule(".btn-danger",
		background("linear-gradient(180deg, var(--action-danger), color-mix(in srgb, var(--action-danger) 85%, #000 15%))"),
		borderColor("color-mix(in srgb, var(--action-danger) 82%, #000 18%)"),
		color("#fff"),
		boxShadow("0 1px 2px rgba(0,0,0,0.28), inset 0 1px 0 rgba(255,255,255,0.16)"),
	)
	rule(".btn-danger:hover",
		filter("brightness(1.08)"),
	)
	rule(".row.subtask",
		borderLeft("2px solid var(--border)"),
	)
	rule(".rule-grip",
		display("inline-flex"),
		alignItems("center"),
		color("var(--text-faint)"),
		cursor("grab"),
		flex("none"),
		marginRight(".25rem"),
	)
	rule(".row[draggable=\"true\"]:active",
		cursor("grabbing"),
		background("var(--bg-elev)"),
		opacity(".85 !important"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".btn",
		minHeight("44px"),
		paddingTop("clamp(0.5rem, 1.5vw, 0.75rem)"),
		paddingBottom("clamp(0.5rem, 1.5vw, 0.75rem)"),
		paddingLeft("clamp(0.6rem, 2vw, 1rem)"),
		paddingRight("clamp(0.6rem, 2vw, 1rem)"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".field",
		minHeight("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".nav-link",
		minHeight("44px"),
		paddingLeft("0.75rem"),
		paddingRight("0.75rem"),
		display("inline-flex"),
		alignItems("center"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".btn-icon, .btn-del, .td-actions .btn",
		minWidth("44px"),
		minHeight("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".txn-table .clr-toggle,\n        .txn-table .td-select .check",
		minWidth("44px"),
		minHeight("44px"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".data-pager .btn",
		minHeight("44px"),
		minWidth("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".seg-btn",
		minHeight("44px"),
		paddingLeft("0.85rem"),
		paddingRight("0.85rem"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".rstep",
		minWidth("44px"),
		minHeight("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".gear-inline, .gear-abs",
		minWidth("44px"),
		minHeight("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".notify-btn, .muzak-btn, .add-btn, .add-caret, .menu-btn,\n        .icon-btn, .rail-collapse-btn",
		minWidth("44px"),
		minHeight("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".topbar button, .rail button",
		minHeight("44px"),
		minWidth("44px"),
	)
	ruleMedia("(max-width: 640px), (pointer: coarse)", ".btn",
		minWidth("44px"),
	)
	ruleMedia("(max-width: 640px)", ".seg-btn",
		fontSize("0.8rem"),
	)
	ruleMedia("(max-width: 640px)", ".rlabel",
		minWidth("0"),
		maxWidth("8rem"),
		textOverflow("ellipsis"),
		overflow("hidden"),
		whiteSpace("nowrap"),
		fontSize("0.75rem"),
	)
	ruleMedia("(max-width: 640px)", ".topbar",
		rowGap("0.5rem"),
	)
	ruleMedia("(max-width: 640px)", ".topbar-controls",
		rowGap("0.5rem"),
	)
	ruleMedia("(max-width: 640px)", ".reso-control",
		flexWrap("wrap"),
		gap("0.4rem"),
		width("100%"),
	)
	ruleMedia("(max-width: 640px)", ".rcap",
		display("none"),
	)
	rule(".mobile-tabbar",
		display("none"),
	)
	ruleMedia("(max-width: 640px)", "main.cf-scroll",
		paddingBottom("calc(56px + env(safe-area-inset-bottom, 0px) + 12px) !important"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar",
		display("flex"),
		position("fixed"),
		bottom("0"),
		left("0"),
		right("0"),
		zIndex("20"),
		background("var(--bg-elev, #1a1a1d)"),
		borderTop("1px solid var(--border, #2a2a2c)"),
		paddingBottom("env(safe-area-inset-bottom, 0px)"),
		height("calc(56px + env(safe-area-inset-bottom, 0px))"),
		alignItems("stretch"),
		justifyContent("space-around"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar .mobile-tab-item",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		justifyContent("center"),
		gap("0.18rem"),
		flex("1"),
		minWidth("44px"),
		minHeight("44px"),
		background("transparent"),
		border("0"),
		color("var(--text-dim, #ababb3)"),
		font("inherit"),
		fontSize("0.65rem"),
		letterSpacing("0.02em"),
		cursor("pointer"),
		transition("color 0.12s ease"),
		padding("0"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar .mobile-tab-item.active",
		color("var(--accent, #2e8b57)"),
		boxShadow("inset 0 2px 0 var(--accent, #2e8b57)"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar .mobile-tab-item:hover",
		color("var(--text, #f4f4f5)"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar .mobile-tab-label",
		fontSize("0.65rem"),
		lineHeight("1"),
	)
	ruleMedia("(max-width: 640px)", ".app",
		paddingBottom("calc(56px + env(safe-area-inset-bottom, 0px))"),
	)
	ruleMedia("(hover: none), (max-width: 640px)", ".rz",
		display("none !important"),
	)
	ruleMedia("(hover: none), (max-width: 640px)", ".ghandle, .grip",
		display("none !important"),
	)
	ruleMedia("(hover: none), (max-width: 640px)", ".bento-reset-btn",
		display("none !important"),
	)
	rule("#boot.hidden",
		pointerEvents("none !important"),
		zIndex("-1 !important"),
	)
	rule("#boot[style*=\"display: none\"],\n      #boot[style*=\"display:none\"]",
		display("none !important"),
		pointerEvents("none !important"),
		zIndex("-1 !important"),
	)
	rule("#app:not(:empty) ~ #boot,\n      body:has(#app:not(:empty)) #boot:not(.hidden)",
		pointerEvents("none !important"),
		zIndex("-1 !important"),
	)
	rule(".labeled-field",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		width("100%"),
		minWidth("0"),
	)
	rule(".labeled-field .t-caption",
		whiteSpace("normal"),
		overflowWrap("break-word"),
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		lineHeight("1.3"),
	)
	rule(".labeled-field .field",
		width("100%"),
		minWidth("0"),
	)
	rule(".card > .form-grid",
		maxWidth("660px"),
	)
	rule(".card > .form-grid + *",
		maxWidth("none"),
	)
	rule(".page",
		paddingLeft("clamp(0.75rem, 3vw, 1.25rem)"),
		paddingRight("clamp(0.75rem, 3vw, 1.25rem)"),
		paddingTop("clamp(1rem, 3dvh, 1.75rem)"),
		paddingBottom("clamp(2rem, 5dvh, 4rem)"),
	)
	rule(".page-title",
		fontSize("clamp(1.25rem, 4vw, 1.6rem)"),
	)
	rule(".card",
		padding("clamp(0.75rem, 2.5vw, 1.25rem)"),
	)
	rule(".topbar",
		minHeight("clamp(3rem, 6svh, 3.5rem)"),
	)
	rule(".card",
		borderRadius("12px"),
	)
	rule(".stat",
		borderRadius("10px"),
	)
	rule("[data-density=\"compact\"] .card",
		borderRadius("8px"),
	)
	rule(".card-title",
		fontWeight("600"),
	)
	rule(".stat-value.neg",
		color("var(--money-negative) !important"),
	)
	rule(".stat-value.pos",
		color("var(--money-positive) !important"),
	)
	rule(".budget-amount",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
	)
	rule(".card .budget-amount",
		color("var(--text)"),
	)
	rule("[data-theme=\"light\"] .card .budget-amount",
		color("#1c1c1e"),
	)
	rule(".section-divider",
		marginTop("2.25rem"),
		marginBottom("0.85rem"),
	)
	rule(".section-divider:first-child",
		marginTop("0"),
	)
	rule(".card",
		marginBottom("1.25rem"),
	)
	rule(".mermaid",
		fontFamily("inherit"),
	)
	rule(".mermaid svg text",
		fontFamily("inherit !important"),
		fontSize("13px !important"),
	)
	rule("main .t-caption:first-child",
		fontSize("0.825rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
		marginBottom("0.85rem"),
	)
	rule(".card .btn[title*=\"Download\"], .card .btn[title*=\"Tax\"]",
		fontSize("0.78rem"),
		padding("0.2rem 0.55rem"),
		opacity("0.65"),
	)
	rule(".card .btn[title*=\"Download\"]:hover, .card .btn[title*=\"Tax\"]:hover",
		opacity("1"),
	)
	rule("[data-theme=\"light\"] .topbar",
		background("rgba(247,246,243,0.92) !important"),
		borderBottomColor("#e4e2dd !important"),
	)
	rule("[data-theme=\"light\"] aside.rail,\n      [data-theme=\"light\"] .rail",
		background("var(--bg-elev) !important"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv.active,\n      [data-theme=\"light\"] aside.rail a[aria-current]",
		backgroundColor("var(--accent-dim, #e4f3ea) !important"),
		color("var(--text, #1c1c1e) !important"),
		fontWeight("600 !important"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv",
		color("var(--text-dim, #56565c)"),
	)
	rule("[data-theme=\"light\"] aside.rail .nv:hover",
		backgroundColor("var(--hover, #e8e6e1)"),
		color("var(--text, #1c1c1e)"),
	)
	rule("[data-theme=\"light\"] .muzak-btn,\n      [data-theme=\"light\"] .notify-btn,\n      [data-theme=\"light\"] .add-btn",
		backgroundColor("var(--bg-card, #ffffff) !important"),
		border("1px solid var(--border, #e4e2dd) !important"),
		color("var(--accent, #2e8b57) !important"),
	)
	rule("[data-theme=\"light\"] .muzak-btn:hover,\n      [data-theme=\"light\"] .notify-btn:hover,\n      [data-theme=\"light\"] .add-btn:hover",
		backgroundColor("var(--hover, #e8e6e1) !important"),
	)
	rule("[data-theme=\"light\"] .add-menu",
		backgroundColor("var(--bg-card, #ffffff) !important"),
		borderColor("var(--border, #e4e2dd) !important"),
		boxShadow("0 8px 24px rgba(0,0,0,.12) !important"),
	)
	rule("[data-theme=\"light\"] .add-item",
		color("var(--text, #1c1c1e) !important"),
	)
	ruleMedia("(max-width: 1024px)", ".add-menu",
		right("auto"),
		left("0"),
		minWidth("200px"),
	)
	ruleMedia("(max-width: 768px)", ".topbar-controls",
		alignItems("center"),
		justifyContent("space-between"),
	)
	ruleMedia("(max-width: 768px)", ".reso-control",
		flex("1"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 768px)", "html, body, #app",
		overflowX("hidden"),
		maxWidth("100vw"),
	)
	ruleMedia("(max-width: 768px)", "aside.rail",
		width("56px"),
	)
	ruleMedia("(max-width: 768px)", "aside.rail .brand-name,\n        aside.rail .nv span,\n        aside.rail nav .rail-section,\n        aside.rail .hh-text",
		display("none"),
	)
	ruleMedia("(max-width: 768px)", "aside.rail .nv",
		justifyContent("center"),
		gap("0"),
		paddingLeft("0"),
		paddingRight("0"),
	)
	ruleMedia("(max-width: 768px)", "aside.rail .railhead",
		paddingLeft("0"),
		paddingRight("0"),
		justifyContent("center"),
	)
	ruleMedia("(max-width: 768px)", "aside.rail .hh",
		justifyContent("center"),
		paddingLeft("0"),
		paddingRight("0"),
	)
	rule("[data-theme=\"light\"] .hh",
		backgroundColor("var(--bg-card, #ffffff)"),
		borderTopColor("var(--border, #e4e2dd)"),
	)
	rule(".add-backdrop",
		pointerEvents("none"),
	)
	rule(".add-backdrop:not(.hidden-menu)",
		pointerEvents("auto"),
	)
	rule("#cf-applock-input:focus, #cf-applock-setup input:focus",
		outline("2px solid var(--accent, #2e8b57)"),
		outlineOffset("2px"),
	)
	rule("#cf-applock-gate button, #cf-applock-setup button",
		transition("filter .12s ease, transform .08s ease"),
	)
	rule("#cf-applock-gate button:hover, #cf-applock-setup button:hover",
		filter("brightness(1.08)"),
	)
	rule("#cf-applock-gate button:active, #cf-applock-setup button:active",
		transform("scale(.97)"),
	)
	keyframes("cf-applock-shake",
		at("0%,100%",
			transform("translateX(0)"),
		),
		at("20%,60%",
			transform("translateX(-6px)"),
		),
		at("40%,80%",
			transform("translateX(6px)"),
		),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", "#cf-applock-input.cf-applock-shake",
		animation("cf-applock-shake .35s ease"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", "#cf-applock-setup",
		animation("cf-applock-fade .25s ease both"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", "#cf-applock-gate",
		animation("cf-applock-fade .25s ease both"),
	)
	keyframes("cf-applock-fade",
		at("from",
			opacity("0"),
		),
		at("to",
			opacity("1"),
		),
	)
	rule(".sync-chip",
		margin("0 0.75rem 0.4rem"),
		padding("0.2rem 0.5rem"),
		borderRadius("999px"),
		fontSize("0.72rem"),
		fontWeight("600"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
		cursor("pointer"),
		width("calc(100% - 1.5rem)"),
		justifyContent("flex-start"),
	)
	rule(".sync-chip:hover",
		color("var(--text)"),
		borderColor("var(--accent)"),
	)
	rule(".sync-chip .sync-dot",
		width("7px"),
		height("7px"),
		borderRadius("50%"),
		background("currentColor"),
		flex("none"),
	)
	rule(".sync-chip .sync-pending",
		marginLeft("auto"),
		fontVariantNumeric("tabular-nums"),
		opacity("0.8"),
	)
	rule(".sync-ok",
		color("var(--up, #54b884)"),
	)
	rule(".sync-busy .sync-dot",
		animation("cf-pulse 1s ease-in-out infinite"),
	)
	rule(".sync-off",
		color("var(--text-faint)"),
	)
	rule(".sync-warn",
		color("var(--warn, #f59e0b)"),
	)
	rule(".sync-err",
		color("var(--danger)"),
	)
	rule("aside.rail.collapsed .sync-chip",
		width("34px"),
		margin("0 auto 0.45rem"),
		padding("0.3rem 0"),
		justifyContent("center"),
		gap("0"),
	)
	rule("aside.rail.collapsed .sync-chip span:not(.sync-dot)",
		display("none"),
	)
	rule("aside.rail.collapsed .sync-chip .sync-dot",
		width("10px"),
		height("10px"),
	)
	keyframes("cf-pulse",
		at("0%,100%",
			opacity("1"),
		),
		at("50%",
			opacity("0.35"),
		),
	)
	// Compact one-line Cloud rail row (2026-07-17 audit P0): icon + label + ✕.
	// Deliberately a quiet row, not a promo card, so it never competes with the
	// primary navigation for rail space at short viewport heights.
	rule(".cloud-mention",
		margin("0 0.75rem 0.5rem"),
		padding("0.15rem 0.25rem"),
		borderRadius("6px"),
	)
	rule(".cloud-mention-link",
		padding("0.3rem 0.45rem"),
		borderRadius("6px"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		textDecoration("none"),
	)
	rule(".cloud-mention-link:hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	rule(".cloud-mention-x",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.5rem"),
		height("1.5rem"),
		borderRadius("6px"),
		color("var(--text-faint)"),
		flexShrink("0"),
	)
	rule(".cloud-mention-x:hover",
		background("var(--hover)"),
		color("var(--text)"),
	)
	// The narrow (icon-only) mobile rail has no room for a labeled row, and a
	// stripped icon-only promo reads as mystery chrome — hide it outright.
	// Two-class selector: the row's tw.Flex utility (whose sheet loads AFTER the
	// design system and wins ties by order) would otherwise out-cascade a bare
	// .cloud-mention display:none.
	ruleMedia("(max-width: 768px)", "aside.rail .cloud-mention",
		display("none !important"),
	)
	// At very short desktop heights the rail's nav needs every row; the Cloud
	// mention is the first thing to yield.
	ruleMedia("(max-height: 560px)", "aside.rail .cloud-mention",
		display("none !important"),
	)
	rule(".upsheet-backdrop",
		position("fixed"),
		inset("0"),
		zIndex("60"),
		display("flex"),
		alignItems("flex-end"),
		justifyContent("center"),
		background("rgba(4,4,6,.45)"),
	)
	rule(".upsheet",
		width("100%"),
		maxWidth("420px"),
		margin("0 0.75rem 0.75rem"),
		padding("1.1rem 1.2rem 1.25rem"),
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("14px"),
		boxShadow("0 20px 50px -16px rgba(0,0,0,.6)"),
	)
	rule(".upsheet-title",
		fontFamily("var(--font-display),'Fraunces',serif"),
		fontSize("1.15rem"),
		fontWeight("600"),
		margin("0 0 0.5rem"),
	)
	rule(".upsheet-benefits",
		margin("0 0 0.6rem"),
		paddingLeft("1.1rem"),
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".upsheet-price",
		margin("0.2rem 0"),
	)
	rule(".upsheet-trust",
		margin("0 0 0.2rem"),
	)
	rule(".sub-banner",
		display("block"),
		padding("0.45rem 1rem"),
		fontSize("0.8rem"),
		fontWeight("500"),
		border("0"),
		borderBottom("1px solid var(--border)"),
		cursor("pointer"),
	)
	rule(".sub-trial",
		background("var(--accent-dim, #e4f3ea)"),
		color("var(--accent, #2e8b57)"),
	)
	rule(".sub-pastdue",
		background("#5a3a1a"),
		color("#fcd9a8"),
	)
	rule(".sub-canceled",
		background("var(--bg-elev)"),
		color("var(--text-dim)"),
	)
	rule("[data-theme=\"light\"] .sub-pastdue",
		background("#fff4e5"),
		color("#92510a"),
	)
	ruleMedia("(prefers-reduced-motion: no-preference)", ".upsheet",
		animation("cf-applock-fade .2s ease both"),
	)
}
