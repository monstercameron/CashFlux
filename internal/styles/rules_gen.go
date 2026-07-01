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
		customProp("--border", "#2a2a2c"),
		customProp("--text", "#f4f4f5"),
		customProp("--text-dim", "#ababb3"),
		customProp("--text-faint", "#888890"),
		customProp("--accent", "#2e8b57"),
		customProp("--accent-dim", "#1f2c24"),
		customProp("--brand", "#7c83ff"),
		customProp("--danger", "#d8716f"),
		customProp("--radius", "0px"),
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
	ruleMedia("print", "aside.rail, .rail, .topbar, .mobile-tabbar, .skip-link, .sample-banner, .scope-banner,\n        .app-banner, .toast, .reso-control, .home-hero-actions, .reports-export,\n        input, select, textarea,\n        \n        button:not(.btn-link),\n        .txn-table .td-actions, .txn-table th.td-actions,\n        .txn-table .td-select, .txn-table th.td-select",
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
		transition("opacity 0.45s ease, transform 0.45s ease"),
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
		customProp("--wonder-on", "1"),
		customProp("--wonder-dur-fast", "110ms"),
		customProp("--wonder-dur", "170ms"),
		customProp("--wonder-dur-slow", "300ms"),
		customProp("--wonder-ease", "cubic-bezier(.2,.75,.2,1)"),
		customProp("--wonder-ease-out", "cubic-bezier(.16,1,.3,1)"),
		customProp("--wonder-lift", "5px"),
		customProp("--wonder-press", ".975"),
		customProp("--wonder-shadow", "0 6px 22px rgba(0,0,0,.16)"),
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
	rule(".btn:active, .data-btn:active, .seg-btn:active, .add-item:active, .menu-btn:active, .icon-btn:active, [role=\"button\"]:active",
		transform("scale(var(--wonder-press))"),
	)
	rule(".w:not(.drag)",
		transition("transform var(--wonder-dur) var(--wonder-ease), box-shadow var(--wonder-dur) var(--wonder-ease), border-color .12s ease"),
	)
	rule(".w:not(.drag):hover",
		transform("translateY(calc(-1 * var(--wonder-lift) * var(--wonder-on)))"),
		boxShadow("var(--wonder-shadow)"),
	)
	rule(".row:not(.txn-table .row):hover",
		transform("translateX(calc(2px * var(--wonder-on)))"),
	)
	rule(".row:not(.txn-table .row)",
		transition("background 0.12s ease, transform var(--wonder-dur-fast) var(--wonder-ease)"),
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
	rule(".nv:hover",
		transform("translateY(calc(-1px * var(--wonder-on)))"),
	)
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
	keyframes("wonder-row-enter",
		at("from",
			opacity("0"),
			transform("translateY(calc(6px * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("none"),
		),
	)
	rule(".rows .row:not(.txn-table .row),\n      .list-rows .row:not(.txn-table .row)",
		animation("wonder-row-enter var(--wonder-dur) var(--wonder-ease-out) both"),
	)
	rule(".rows .row:nth-child(1):not(.txn-table .row),\n      .list-rows .row:nth-child(1):not(.txn-table .row)",
		animationDelay("calc(0ms  + 0ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(2):not(.txn-table .row),\n      .list-rows .row:nth-child(2):not(.txn-table .row)",
		animationDelay("calc(40ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(3):not(.txn-table .row),\n      .list-rows .row:nth-child(3):not(.txn-table .row)",
		animationDelay("calc(80ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(4):not(.txn-table .row),\n      .list-rows .row:nth-child(4):not(.txn-table .row)",
		animationDelay("calc(110ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(5):not(.txn-table .row),\n      .list-rows .row:nth-child(5):not(.txn-table .row)",
		animationDelay("calc(140ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(6):not(.txn-table .row),\n      .list-rows .row:nth-child(6):not(.txn-table .row)",
		animationDelay("calc(165ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(7):not(.txn-table .row),\n      .list-rows .row:nth-child(7):not(.txn-table .row)",
		animationDelay("calc(190ms * var(--wonder-on))"),
	)
	rule(".rows .row:nth-child(8):not(.txn-table .row),\n      .list-rows .row:nth-child(8):not(.txn-table .row)",
		animationDelay("calc(210ms * var(--wonder-on))"),
	)
	rule("[data-wonder=\"off\"] .rows .row,\n      [data-wonder=\"off\"] .list-rows .row",
		animation("none"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".rows .row, .list-rows .row",
		animation("none"),
	)
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
	rule(".bento .w",
		animation("wonder-bento-enter var(--wonder-dur-slow) var(--wonder-ease-out) both"),
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
	keyframes("wonder-toast-in",
		at("from",
			opacity("0"),
			transform("translate(-50%, calc(0.6rem * var(--wonder-on)))"),
		),
		at("50%",
			transform("translate(-50%, calc(-0.15rem * var(--wonder-on)))"),
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
		at("50%",
			transform("translate(-50%, calc(-0.15rem * var(--wonder-on)))"),
		),
		at("to",
			opacity("1"),
			transform("translate(-50%, 0)"),
		),
	)
	rule(".toast",
		animation("toast-in var(--wonder-dur-slow) cubic-bezier(.34,1.56,.64,1) both"),
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
	)
	rule(".tb-context, .tb-actions",
		display("inline-flex"),
		alignItems("center"),
	)
	rule(".tb-context",
		gap(".5rem"),
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
		customProp("--text-dim", "#56565c"),
		customProp("--text-faint", "#686870"),
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
	rule(".reports-hero",
		marginBottom("1rem"),
		padding("1.25rem"),
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
	)
	rule(".hero-period",
		fontSize("0.825rem"),
		fontWeight("500"),
		color("var(--text-dim)"),
		margin("0 0 0.6rem"),
	)
	rule(".hero-main",
		display("flex"),
		alignItems("baseline"),
		gap("1.5rem"),
		flexWrap("wrap"),
	)
	rule(".hero-net",
		fontSize("2.5rem"),
		fontWeight("800"),
		lineHeight("1.1"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		letterSpacing("-0.025em"),
	)
	rule(".hero-net.pos",
		color("var(--money-positive)"),
	)
	rule(".hero-net.neg",
		color("var(--money-negative)"),
	)
	rule(".hero-net-delta",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		marginTop("0.2rem"),
		fontSize("0.8rem"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".hero-net-delta.pos",
		color("var(--money-positive)"),
	)
	rule(".hero-net-delta.neg",
		color("var(--money-negative)"),
	)
	rule(".hero-flankers",
		display("flex"),
		gap("1.25rem"),
		alignItems("baseline"),
		flexWrap("wrap"),
	)
	rule(".hero-flanker",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".hero-flanker-label",
		fontSize("0.75rem"),
		fontWeight("500"),
		textTransform("uppercase"),
		letterSpacing("0.05em"),
		color("var(--text-dim)"),
	)
	rule(".hero-flanker-value",
		fontSize("1.75rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
		letterSpacing("-0.015em"),
	)
	rule(".hero-flanker-value.pos",
		color("var(--money-positive)"),
	)
	rule(".hero-flanker-value.neg",
		color("var(--money-negative)"),
	)
	rule(".hero-secondary",
		display("flex"),
		gap("1.25rem"),
		flexWrap("wrap"),
		marginTop("0.75rem"),
		paddingTop("0.75rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".reports-grid",
		display("grid"),
		gridTemplateColumns("1fr"),
		gap("1rem"),
		alignItems("start"),
	)
	ruleMedia("(min-width: 1100px)", ".reports-grid",
		gridTemplateColumns("1fr 1fr"),
	)
	rule(".reports-chart-pair",
		display("grid"),
		gridTemplateColumns("1fr"),
		gap("1rem"),
		alignItems("start"),
	)
	ruleMedia("(min-width: 900px)", ".reports-chart-pair",
		gridTemplateColumns("1fr 1fr"),
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
		padding("0.32rem 0.75rem"),
		border("1px dashed var(--border)"),
		borderRadius("999px"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.82rem"),
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
		background("color-mix(in srgb, var(--danger) 6%, var(--card-bg))"),
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
	rule(".filter-toolbar",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
		marginBottom("0.6rem"),
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
	rule(".filter-fields",
		display("flex"),
		flexDirection("column"),
		gap("0.7rem"),
	)
	rule(".filter-fields .field-label",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		fontSize("0.8rem"),
		color("var(--muted)"),
	)
	rule(".filter-fields .field-label .field",
		color("var(--text)"),
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
	rule(".filter-inline-body .filter-fields .field-label",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		fontSize("0.8rem"),
		color("var(--muted)"),
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
		zIndex("60"),
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
		color("var(--fg, var(--text))"),
		whiteSpace("nowrap"),
	)
	rule(".budget-sub",
		display("block"),
		color("var(--text-dim)"),
		fontSize("0.82rem"),
		marginTop("0.15rem"),
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
		color("var(--fg, var(--text))"),
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
	rule(".draft-actionbar",
		position("sticky"),
		top("0"),
		zIndex("5"),
		display("flex"),
		gap("0.5rem"),
		alignItems("center"),
		padding("0.5rem 0"),
		margin("0.25rem 0 0.5rem"),
		background("var(--bg-elev, #1a1a1d)"),
		borderBottom("1px solid var(--border, rgba(255,255,255,0.08))"),
	)
	rule(".draft-actionbar .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".draft-actionbar .btn",
		flex("0 0 auto"),
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
	rawBlockMedia("(prefers-reduced-motion: no-preference)", "@keyframes catchup-card-in{from { opacity: 0; transform: translateY(10px); }\n          to   { opacity: 1; transform: translateY(0); }}")
	ruleMedia("(prefers-reduced-motion: no-preference)", "[data-testid=\"dash-catchup-card\"]",
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
		customProp("--disabled-opacity", "0.48"),
		customProp("--motion-instant", "0ms"),
		customProp("--motion-fast", "100ms"),
		customProp("--motion-base", "160ms"),
		customProp("--motion-medium", "220ms"),
		customProp("--motion-slow", "320ms"),
		customProp("--ease-standard", "cubic-bezier(0.2, 0, 0, 1)"),
		customProp("--ease-enter", "cubic-bezier(0, 0, 0, 1)"),
		customProp("--ease-exit", "cubic-bezier(0.4, 0, 1, 1)"),
		customProp("--ease-emphasized", "cubic-bezier(0.2, 0, 0, 1)"),
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
		background("#121214"),
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
		background("color-mix(in srgb, var(--accent) 12%, #121214)"),
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
		background("#121214"),
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
		background("#1a1a1d"),
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
		background("#1a1a1d"),
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
		background("linear-gradient(0deg, color-mix(in srgb, var(--accent) 4%, transparent), transparent),\n          repeating-linear-gradient(0deg, transparent, transparent 37px, color-mix(in srgb, var(--text) 5%, transparent) 37px, color-mix(in srgb, var(--text) 5%, transparent) 38px),\n          repeating-linear-gradient(90deg, transparent, transparent 37px, color-mix(in srgb, var(--text) 5%, transparent) 37px, color-mix(in srgb, var(--text) 5%, transparent) 38px),\n          #0e0e10"),
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
		background("#15151a"),
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
		background("color-mix(in srgb, var(--accent) 14%, #121214)"),
	)
	rule(".studio-verb",
		fontSize(".72rem"),
		padding(".2rem .5rem"),
		border("1px solid var(--border)"),
		borderRadius("7px"),
		background("#121214"),
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
		gridTemplateRows("repeat(8, var(--cell))"),
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
	// --- /budgets visual polish. Scoped to .bento-budgets so the shared .budget /
	// .bar / .budget-sub styles used on other screens (allocate, goals, reports) stay
	// untouched. Each budget becomes an elevated meter-card with a state-colored left
	// stripe, a prominent gradient progress bar over a visible track (so 0%/low budgets
	// no longer vanish into the background), and a tinted percent chip. ---
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
		background("color-mix(in srgb, var(--danger) 11%, var(--bg-elev))"),
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
	// Name-first hierarchy: the category name is the card title.
	rule(".bento-budgets .budget .row-desc",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("700"),
		fontSize("1.05rem"),
		color("var(--fg, var(--text))"),
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
		color("var(--fg, var(--text))"),
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
		background("color-mix(in srgb, var(--fg, #ffffff) 9%, transparent)"),
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
	// Toolbar: a clean single row — a compact labelled method picker on the left and
	// right-aligned, uniform-height actions. The method select is no longer .field's
	// full width (which read like a giant search bar and wrapped the buttons below).
	rule(".bento-budgets .budgets-toolbar",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("nowrap"),
		gap("1rem"),
	)
	rule(".bento-budgets .budgets-toolbar-method",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		minWidth("0"),
		flexShrink("1"),
	)
	rule(".bento-budgets .budgets-toolbar-label",
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".bento-budgets .budgets-method-select",
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
		zIndex("50"),
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
		borderBottom("1px solid #1f1f22"),
		fontSize(".9rem"),
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
		transition("left var(--wonder-dur) cubic-bezier(.34,1.56,.64,1), background var(--wonder-dur) var(--wonder-ease-out)"),
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
		color("var(--fg, var(--text))"),
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
		border("1.5px solid color-mix(in srgb, var(--fg, #ffffff) 34%, transparent)"),
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
	rule("aside.rail.collapsed .cloud-mention,\n      aside.rail.collapsed .rail-foot > span,\n      aside.rail.collapsed .rail-foot > a",
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
		zIndex("50"),
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
	rule(".add-menu.open-left",
		left("auto"),
		right("0"),
	)
	rule(".add-menu.open-up",
		top("auto"),
		bottom("calc(100% + 6px)"),
	)
	rule(".add-item",
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
	rule(".rate-row .rate-in",
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
	rule(".sample-banner",
		display("inline-flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap(".5rem"),
		padding(".25rem .5rem .25rem .55rem"),
		marginBottom("1rem"),
		maxWidth("100%"),
		background("rgba(211,146,0,0.10)"),
		border("1px solid #d39200"),
		borderRadius("999px"),
		fontSize(".8rem"),
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
		alignItems("center"),
	)
	rule(".attention-item",
		display("inline-flex"),
		alignItems("center"),
		gap(".5rem"),
		textAlign("left"),
		background("transparent"),
		border("1px solid var(--line,#e5e7eb)"),
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
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 35%, var(--line,#e5e7eb))"),
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
		color("var(--dim,#6b7280)"),
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
		background("var(--line,#e5e7eb)"),
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
		color("var(--dim,#6b7280)"),
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
		border("1px solid var(--line,#e5e7eb)"),
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
		color("var(--dim,#6b7280)"),
		borderLeft("1px solid var(--line,#e5e7eb)"),
		borderRight("1px solid var(--line,#e5e7eb)"),
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
		border("1px solid var(--line,#e5e7eb)"),
		borderRadius("8px"),
		background("transparent"),
		cursor("pointer"),
		color("inherit"),
		lineHeight("1"),
		transition("background .12s ease, border-color .12s ease"),
	)
	rule(".wm-arrow:hover:not(:disabled)",
		background("color-mix(in srgb, var(--accent,#3b82f6) 10%, transparent)"),
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 35%, var(--line,#e5e7eb))"),
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
		color("var(--dim,#6b7280)"),
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
		border("1px solid var(--line,#e5e7eb)"),
		borderRadius("6px"),
		background("none"),
		cursor("pointer"),
	)
	rule(".wm-clear",
		width("1.4rem"),
		height("1.4rem"),
		border("1px solid var(--line,#e5e7eb)"),
		borderRadius("5px"),
		background("transparent"),
		cursor("pointer"),
		color("var(--dim,#6b7280)"),
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
		color("var(--faint,#9ca3af)"),
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
		backgroundImage("radial-gradient(circle, color-mix(in srgb, var(--dim,#6b7280) 22%, transparent) 1px, transparent 1px)"),
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
		border("1px solid var(--line,#e5e7eb)"),
		background("var(--bg,#0e0e10)"),
		backgroundImage("radial-gradient(circle, color-mix(in srgb, var(--dim,#6b7280) 22%, transparent) 1px, transparent 1px)"),
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
		border("1.5px solid var(--line,#e5e7eb)"),
		transition("border-color .12s ease, box-shadow .12s ease, transform .12s ease"),
	)
	rule(".wb-node:hover",
		borderColor("color-mix(in srgb, var(--accent,#3b82f6) 45%, var(--line,#e5e7eb))"),
	)
	rule(".wb-node.is-active",
		borderColor("var(--accent,#3b82f6)"),
		boxShadow("0 0 0 3px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)"),
	)
	rule(".wb-node-kind",
		fontSize("11px"),
		textTransform("uppercase"),
		letterSpacing(".06em"),
		color("var(--faint,#9ca3af)"),
	)
	rule(".wb-node-val",
		fontSize("14px"),
		fontWeight("600"),
		color("var(--fg,#e5e7eb)"),
	)
	rule(".wb-port",
		position("absolute"),
		top("50%"),
		width("9px"),
		height("9px"),
		borderRadius("999px"),
		background("var(--bg,#0e0e10)"),
		border("1.5px solid var(--dim,#6b7280)"),
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
		background("var(--line,#e5e7eb)"),
	)
	rule(".wb-edge::after",
		content("\"\""),
		position("absolute"),
		right("-1px"),
		top("50%"),
		transform("translateY(-50%)"),
		borderTop("5px solid transparent"),
		borderBottom("5px solid transparent"),
		borderLeft("7px solid var(--line,#e5e7eb)"),
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
		color("var(--faint,#9ca3af)"),
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
		color("var(--dim,#6b7280)"),
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
		zIndex("90"),
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
		border("1px solid var(--line,#e5e7eb)"),
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
		borderLeft("2px solid var(--line,#e5e7eb)"),
	)
	rule(".rule-grip",
		display("inline-flex"),
		alignItems("center"),
		color("var(--faint,#9ca3af)"),
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
	rule(".share-bar",
		height("8px !important"),
	)
	rule(".share-bar > div",
		height("8px !important"),
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
	rule(".cloud-mention",
		margin("0 0.75rem 0.6rem"),
		padding("0.6rem 0.7rem"),
		borderRadius("8px"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
	)
	rule(".cloud-mention-title",
		fontSize("0.8rem"),
		fontWeight("600"),
		color("var(--text)"),
		margin("0"),
	)
	rule(".cloud-mention-body",
		margin("0.15rem 0 0"),
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
