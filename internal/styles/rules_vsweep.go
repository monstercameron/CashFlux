// SPDX-License-Identifier: MIT

package styles

// registerVSweep holds the CSS for the 2026-07-03 world-class visual/UX sweep's
// remaining tickets (C340–C362). Registered last so these overrides win over the
// base rules they refine.
func registerVSweep() {
	// ── C342/C343: name the window over the hero's stat row ─────────────────
	// The four figures (income / spending / net / savings rate) cover the
	// SELECTED period, not "this month" — and the savings rate among them is the
	// one most often read against /health's three-month average. The caption
	// carries the spacing the row used to own so the two read as one block.
	rule(".home-hero-stats-block",
		marginTop("1rem"),
		paddingTop("0.85rem"),
	)
	rule(".home-hero-stats-block .home-hero-stats",
		marginTop("0"),
		paddingTop("0.35rem"),
	)
	// The always-on period chip reads quieter than the "Today" exception chip it
	// shares a slot with: one is context, the other is a warning that a figure
	// ignores the picker.
	rule(".w-today.w-window",
		borderStyle("dashed"),
		textTransform("none"),
		letterSpacing("0.02em"),
	)

	// ── C353: the real rate sits beside the abstract score ──────────────────
	rule(".alloc-dest-chip-real",
		marginLeft("0.3rem"),
		fontVariantNumeric("tabular-nums"),
	)

	// ── C348: /subscriptions keeps its actions until you reach for them ─────
	// A list of twenty rows carrying two resting buttons each is forty controls
	// competing with the twenty numbers the page is actually about. "Remind me"
	// is a rare action; it fades in on hover or keyboard focus, the way the
	// transactions ledger already does it. It stays visible whenever the row is
	// focused within, so it is never keyboard-unreachable, and stays visible
	// unconditionally on touch, where there is no hover to reveal it on.
	rule(".sub-row .sub-actions .btn-reveal",
		opacity("0"),
		transition("opacity var(--motion-quick, 120ms) ease"),
	)
	rule(".sub-row:hover .sub-actions .btn-reveal, .sub-row:focus-within .sub-actions .btn-reveal",
		opacity("1"),
	)
	ruleMedia("(hover: none)", ".sub-row .sub-actions .btn-reveal",
		opacity("1"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".sub-row .sub-actions .btn-reveal",
		transition("none"),
	)

	// Cross-links between the page's three views of the same subscriptions.
	rule(".subs-xlink",
		marginTop("0.35rem"),
		fontSize("var(--type-12)"),
	)

	// ── C357: the rules list IS the ordering surface ────────────────────────
	// The precedence number moved onto the row when the duplicate "Rule order"
	// section was retired. Quiet and tabular so it reads as an index, not a
	// figure competing with the match count on the right.
	rule(".rule-prec",
		flex("none"),
		minWidth("1.25rem"),
		fontVariantNumeric("tabular-nums"),
		fontSize("var(--type-12)"),
		textAlign("right"),
	)

	// ── C358: the plan's arc, quiet under its ending figure ─────────────────
	rule(".plan-scenario-arc",
		display("block"),
		fontSize("var(--type-12)"),
		marginTop("0.1rem"),
	)

	// ── C360: the category map is navigation, so its chips look reachable ───
	rule("a.cat-map-chip, a.cat-map-sub",
		textDecoration("none"),
		color("inherit"),
		cursor("pointer"),
	)
	rule("a.cat-map-chip:hover, a.cat-map-sub:hover, a.cat-map-chip:focus-visible, a.cat-map-sub:focus-visible",
		borderColor("var(--accent)"),
		color("var(--text)"),
	)

	// ── C379: the target tick on a drift bar ────────────────────────────────
	// The bar's LENGTH is the current weight and the tick marks the target, so
	// "on target" reads as a full-looking bar with the tick at its end rather
	// than as an empty bar — the same value-on-a-scale reading the health
	// factors use.
	rule(".inv-alloc-track",
		position("relative"),
	)
	rule(".inv-drift-tick",
		position("absolute"),
		top("-2px"),
		bottom("-2px"),
		width("2px"),
		background("var(--text)"),
		opacity("0.55"),
	)
	rule(".inv-drift-over",
		color("var(--warn, var(--text))"),
	)
	rule(".inv-drift-under",
		color("var(--text-dim)"),
	)

	rule(".home-hero-stats-window",
		margin("0"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		fontSize("0.66rem"),
	)
	// ─── C402: to-do bulk selection ──────────────────────────────────────────
	// The bar sits above the list and stays put while you scroll a long page —
	// a selection you built over three screenfuls with the action bar scrolled
	// off the top is a selection you cannot act on.
	rule(".todo-bulk-entry",
		display("flex"),
		justifyContent("flex-end"),
		marginBottom("0.5rem"),
	)
	rule(".todo-bulkbar",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
		position("sticky"),
		top("0"),
		zIndex("3"),
		padding("0.5rem 0.65rem"),
		marginBottom("0.6rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface-2, var(--surface))"),
	)
	rule(".todo-select",
		flex("0 0 auto"),
		width("1rem"),
		height("1rem"),
		marginRight("0.15rem"),
		accentColor("var(--accent)"),
		cursor("pointer"),
	)

	// ─── C405: the ready-made automation gallery ─────────────────────────────
	rule(".wf-preset-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fill, minmax(15rem, 1fr))"),
		gap("0.75rem"),
	)
	rule(".wf-preset-card",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.4rem"),
		padding("0.85rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface)"),
	)
	rule(".wf-preset-name",
		margin("0"),
		fontSize("0.95rem"),
		lineHeight("1.3"),
	)
	rule(".wf-preset-desc",
		margin("0"),
		flex("1 1 auto"),
		fontSize("0.82rem"),
		lineHeight("1.45"),
		color("var(--text-dim)"),
	)
	rule(".wf-preset-meta",
		display("flex"),
		flexWrap("wrap"),
		gap("0.35rem"),
	)

	// ─── C404: one adaptive to-do toolbar ────────────────────────────────────
	// The row holds the two controls every session touches (which view, which
	// lens) and hands the rest to the FilterToolbar, which grows into the slack.
	rule(".todo-cmdbar .cmdbar-single",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.6rem"),
	)
	rule(".cmdbar-grow",
		flex("1 1 22rem"),
		minWidth("0"),
	)
	// Inside the popover the controls stack: they were cramped side by side in the
	// old bar, which is a large part of why they read as clutter.
	rule(".todo-filter-fields",
		display("flex"),
		flexDirection("column"),
		gap("0.55rem"),
	)
	rule(".todo-filter-fields .todo-ctrl",
		width("100%"),
		justifyContent("space-between"),
	)
	rule(".todo-filter-row",
		display("flex"),
		gap("0.4rem"),
	)

	// ─── C399: the goal contribution-history rail ────────────────────────────
	rule(".goal-hist",
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".goal-hist-head",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.4rem"),
		marginBottom("0.4rem"),
	)
	rule(".goal-hist-bars",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
	)
	rule(".goal-hist-row",
		display("grid"),
		gridTemplateColumns("4.5rem 1fr auto"),
		alignItems("center"),
		gap("0.45rem"),
		fontSize("0.72rem"),
	)
	rule(".goal-hist-month, .goal-hist-amt",
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".goal-hist-amt",
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".goal-hist-track",
		position("relative"),
		height("0.6rem"),
		borderRadius("999px"),
		background("var(--surface-2, var(--border))"),
		overflow("visible"),
	)
	rule(".goal-hist-fill",
		display("block"),
		height("100%"),
		borderRadius("999px"),
		background("var(--accent)"),
	)
	// A month that missed its plan is toned down rather than alarmed: this is a
	// record of what happened, not a warning about what will.
	rule(".goal-hist-fill.is-short",
		background("var(--text-faint)"),
	)
	// The plan marker sits ON the track, so clearing the line is a glance instead
	// of a comparison between two bar lengths.
	rule(".goal-hist-plan",
		position("absolute"),
		top("-2px"),
		bottom("-2px"),
		width("2px"),
		background("var(--text)"),
		opacity("0.5"),
	)
	rule(".goal-hist-legend",
		margin("0.35rem 0 0"),
		fontSize("0.7rem"),
		color("var(--text-faint)"),
	)

	// ─── C383: the report window picker ──────────────────────────────────────
	rule(".rpta-range",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		margin("0.5rem 0 0.75rem"),
	)
	rule(".rpta-range-row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("flex-end"),
		gap("0.6rem"),
	)
	rule(".rpta-range-ctrl",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	rule(".rpta-range-custom",
		display("flex"),
		flexWrap("wrap"),
		alignItems("flex-end"),
		gap("0.6rem"),
	)
	rule(".rpta-range-hint, .rpta-range-resolved",
		margin("0"),
		fontSize("0.72rem"),
		color("var(--text-faint)"),
	)
	// The resolved-window line is the answer to "what am I looking at", so it
	// reads a step stronger than the hint beside it.
	rule(".rpta-range-resolved",
		color("var(--text-dim)"),
	)

	// ─── C385: the per-section methodology drawer ────────────────────────────
	// Quiet by default and quiet when open: reference material sits under the
	// section it explains without competing with it.
	rule(".rpta-method",
		marginTop("0.75rem"),
		paddingTop("0.6rem"),
		borderTop("1px dashed var(--border)"),
		fontSize("0.78rem"),
	)
	rule(".rpta-method-sum",
		cursor("pointer"),
		color("var(--text-dim)"),
		listStyle("none"),
	)
	rule(".rpta-method-sum:hover",
		color("var(--text)"),
	)
	rule(".rpta-method-body",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(16rem, 1fr))"),
		gap("0.9rem"),
		marginTop("0.6rem"),
	)
	rule(".rpta-method-list",
		margin("0.3rem 0 0"),
		paddingLeft("1.1rem"),
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		color("var(--text-dim)"),
		lineHeight("1.5"),
	)
	rule(".rpta-method-bmks > li",
		display("flex"),
		flexDirection("column"),
	)
	rule(".rpta-method-bmk",
		color("var(--text)"),
	)
	// The source sits directly under the value it attributes, never as a
	// footnote — an unsourced-looking benchmark is what this drawer exists to fix.
	rule(".rpta-method-src",
		color("var(--text-faint)"),
		fontSize("0.72rem"),
	)

	// ─── C384: printing the report ───────────────────────────────────────────
	// "Save as PDF" is the browser's print dialog, so print IS the export format
	// for the whole narrative. The base sheet already strips the app shell; what
	// it did not know about is the report's own navigation chrome and its chapter
	// structure.
	//
	// Each numbered section starts a fresh page. A twelve-chapter review whose
	// chapters straddle page breaks is a document nobody can hand to anyone, and
	// the sections are already self-contained — that is what the numbering means.
	ruleMedia("print", ".rpta-sec",
		prop("break-before", "page"),
		prop("page-break-before", "always"),
	)
	// …except the first, which would otherwise leave a blank sheet behind the
	// masthead.
	ruleMedia("print", ".rpta-sec:first-of-type",
		prop("break-before", "auto"),
		prop("page-break-before", "auto"),
	)
	// Report navigation is worthless on paper: jump links point nowhere, the
	// window picker cannot be operated, and the ask-the-assistant buttons are
	// affordances for a screen.
	ruleMedia("print", ".rpta-index, .rpta-range, .rpta-sec-actions, .rpta-drill, "+
		".rpta-scope-toggle, .rpta-toolbar, .rpta-srclink, .rpta-partial-chip",
		display("none !important"),
	)
	// A section heading must never be the last thing on a page.
	ruleMedia("print", ".rpta-sec-head",
		prop("break-after", "avoid"),
		prop("page-break-after", "avoid"),
	)
	ruleMedia("print", ".rpta-cat-row, .rpta-flow-row, .goal-hist-row",
		breakInside("avoid"),
		pageBreakInside("avoid"),
	)
	// The methodology drawer prints in whatever state the reader left it, which
	// is the point — but an open one must not split across a page break.
	ruleMedia("print", ".rpta-method",
		breakInside("avoid"),
		pageBreakInside("avoid"),
	)
	// A <details> the reader opened stays open on paper; native print behaviour
	// already does this, and forcing them all open would print eleven appendices
	// nobody asked for.
	ruleMedia("print", ".rpta-masthead",
		prop("break-after", "page"),
		prop("page-break-after", "always"),
	)

	// ─── C386: a drillable mark inside a quiet row ───────────────────────────
	// The row label becomes a button, so it must read as the label it replaced
	// until it is hovered — a list of six link-blue payee names would shout.
	rule(".rpta-row-drill",
		padding("0"),
		border("0"),
		background("none"),
		font("inherit"),
		color("inherit"),
		textAlign("left"),
		cursor("pointer"),
	)
	rule(".rpta-row-drill:hover, .rpta-row-drill:focus-visible",
		color("var(--accent)"),
		textDecoration("underline"),
	)

	// ─── C408: the evidence drawer on a notification row ─────────────────────
	rule(".notif-why",
		marginTop("0.4rem"),
		fontSize("0.75rem"),
	)
	rule(".notif-why-sum",
		cursor("pointer"),
		color("var(--text-faint)"),
		listStyle("none"),
	)
	rule(".notif-why-sum:hover",
		color("var(--text-dim)"),
	)
	rule(".notif-why-body",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		marginTop("0.35rem"),
		paddingLeft("0.6rem"),
		borderLeft("2px solid var(--border)"),
	)
	rule(".notif-why-row",
		display("grid"),
		gridTemplateColumns("6.5rem 1fr"),
		gap("0.4rem"),
	)
	rule(".notif-why-label",
		color("var(--text-faint)"),
	)
	rule(".notif-why-val",
		color("var(--text-dim)"),
	)
	rule(".notif-why-link",
		marginTop("0.25rem"),
		color("var(--accent)"),
	)

	// ─── C407: whose alert this is ───────────────────────────────────────────
	// A quiet chip, not a badge: it identifies, it does not rank. The severity
	// tag beside it is the thing that should carry colour.
	rule(".notif-member-chip",
		marginLeft("0.35rem"),
		padding("0.05rem 0.4rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		fontSize("0.68rem"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".notif-member-lens",
		maxWidth("11rem"),
	)

	// ─── C381: the forward projection under the balance chart ────────────────
	// A projection sits beside a record of what actually happened, so it must not
	// look equally certain: faint stroke, dashed line, muted label.
	rule(".acct-fwd",
		marginTop("0.4rem"),
		paddingTop("0.4rem"),
		borderTop("1px dashed var(--border)"),
	)
	rule(".acct-fwd-head",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.4rem"),
	)
	rule(".acct-fwd-label",
		color("var(--text-faint)"),
		textTransform("uppercase"),
		letterSpacing("0.04em"),
		fontSize("0.65rem"),
	)
	// Running out of money is the one reading here that should raise its voice.
	rule(".acct-fwd-warn",
		color("var(--warn, var(--text))"),
	)
	rule(".acct-fwd-note",
		display("block"),
		marginTop("0.15rem"),
		fontSize("0.68rem"),
		color("var(--text-faint)"),
	)

	// ─── C376: the holdings paste importer ───────────────────────────────────
	rule(".hld-import",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
		padding("0.85rem"),
		marginTop("0.6rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface)"),
	)
	rule(".hld-import-head",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".hld-import-ctrl",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		maxWidth("18rem"),
	)
	rule(".hld-import-text",
		width("100%"),
		fontFamily("var(--font-mono, ui-monospace, monospace)"),
		fontSize("0.78rem"),
	)
	rule(".hld-import-table",
		width("100%"),
		borderCollapse("collapse"),
		fontSize("0.78rem"),
	)
	rule(".hld-import-table th, .hld-import-table td",
		textAlign("left"),
		padding("0.25rem 0.4rem"),
		borderBottom("1px solid var(--border)"),
	)
	// The action tag is the column a reader scans, so it is the only coloured
	// thing in the table — an add and an update must be distinguishable at a
	// glance, and a skip must not shout.
	rule(".hld-import-tag",
		padding("0.05rem 0.4rem"),
		borderRadius("999px"),
		fontSize("0.68rem"),
		whiteSpace("nowrap"),
		border("1px solid var(--border)"),
	)
	rule(".hld-import-add",
		color("var(--accent)"),
		borderColor("var(--accent)"),
	)
	rule(".hld-import-update",
		color("var(--text)"),
	)
	rule(".hld-import-skip",
		color("var(--text-faint)"),
	)
	rule(".hld-import-map",
		margin("0"),
		fontSize("0.7rem"),
		color("var(--text-faint)"),
	)
	rule(".hld-import-entry",
		marginTop("0.4rem"),
	)

	// ─── C380: the imported benchmark comparison ─────────────────────────────
	rule(".invest-bench",
		marginTop("0.6rem"),
		paddingTop("0.6rem"),
		borderTop("1px dashed var(--border)"),
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	// The verdict is the point of the panel, so it is the only line with weight.
	rule(".invest-bench-read",
		margin("0"),
		fontSize("0.95rem"),
	)
	rule(".invest-bench-ahead",
		color("var(--accent)"),
	)
	rule(".invest-bench-behind",
		color("var(--text-dim)"),
	)
	rule(".invest-bench-actions",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	rule(".invest-bench-form",
		display("flex"),
		flexDirection("column"),
		gap("0.45rem"),
		padding("0.7rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface)"),
	)
	rule(".invest-bench-ctrl",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		maxWidth("18rem"),
	)
	rule(".invest-bench-text",
		width("100%"),
		fontFamily("var(--font-mono, ui-monospace, monospace)"),
		fontSize("0.78rem"),
	)

	// ─── E3: the contradiction strip ─────────────────────────────────────────
	// It appears only when something is actually wrong, so it is allowed to be
	// visible — but it is a report, not an alarm: the severity dot carries the
	// tone and the text stays readable.
	rule(".contra-strip",
		marginTop("0.6rem"),
		padding("0.7rem 0.85rem"),
		borderRadius("var(--radius, 10px)"),
		border("1px solid var(--border)"),
		background("var(--surface)"),
	)
	rule(".contra-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.5rem"),
		marginBottom("0.4rem"),
	)
	rule(".contra-title",
		fontSize("0.72rem"),
		textTransform("uppercase"),
		letterSpacing("0.04em"),
		color("var(--text-dim)"),
	)
	rule(".contra-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
	)
	rule(".contra-row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.4rem"),
		paddingLeft("0.6rem"),
		borderLeft("3px solid var(--border)"),
		fontSize("0.8rem"),
	)
	rule(".contra-critical",
		borderLeftColor("var(--danger, var(--warn, var(--text)))"),
	)
	rule(".contra-warning",
		borderLeftColor("var(--warn, var(--text-dim))"),
	)
	rule(".contra-notice",
		borderLeftColor("var(--border)"),
	)
	rule(".contra-text",
		color("var(--text)"),
	)
	// "How wrong is it" is the reader's first question, so the delta reads as a
	// figure rather than as part of the sentence.
	rule(".contra-delta",
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
	)

	// ─── LF-8: the unfinished-work line ──────────────────────────────────────
	// Chores, not errors, so it reads quietly beside the integrity findings.
	rule(".data-hygiene",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.5rem"),
		margin("0.35rem 0 0.5rem"),
	)

	// ─── LF-7: bills + recurring as a month grid ─────────────────────────────
	rule(".rhy-cal-head",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginBottom("0.5rem"),
	)
	rule(".rhy-cal-entry",
		display("flex"),
	)
	// The day marker carries STATE, not just presence: an overdue day and a
	// settled day are opposite readings, and one neutral dot for both costs a
	// click to interpret.
	rule(".rhy-cal-dot",
		display("block"),
		marginTop("0.1rem"),
		fontSize("0.6rem"),
		lineHeight("1.2"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".rhy-cal-dot.is-paid",
		color("var(--text-faint)"),
		textDecoration("line-through"),
	)
	rule(".rhy-cal-dot.is-overdue",
		color("var(--warn, var(--text))"),
		fontWeight("600"),
	)

	// ─── DP1: the overdue break in the agenda ────────────────────────────────
	// It is a heading like the month labels beside it, not an alarm — the rows
	// under it already carry their own OVERDUE pills, and a loud divider would
	// double the shouting.
	rule(".rhy-ag-month.rhy-ag-overdue",
		color("var(--warn, var(--text-dim))"),
	)

	// ─── DP-F5a/b: empty-lane add, and the seeded-day subtitle ───────────────
	rule(".tdb-empty",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap("0.3rem"),
	)
	// The seeded day is a statement of context, not a field label, so it reads
	// above the form rather than inside it.
	rule(".tc-seeded-day",
		margin("0 0 0.5rem"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)

	// ─── DP-F5c: what "Save as template" actually snapshots ──────────────────
	rule(".txt-save-row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.4rem"),
	)

	// ─── FP-T1a/T1b: the retirement card ─────────────────────────────────────
	rule(".retire-inputs",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(9rem, 1fr))"),
		gap("0.6rem"),
		margin("0.6rem 0 0.8rem"),
	)
	rule(".retire-field",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
	)
	// The real-dollar figure is the headline; everything else supports it.
	rule(".retire-headline",
		margin("0"),
		fontSize("1.1rem"),
	)
	rule(".retire-read",
		margin("0.4rem 0 0"),
		fontSize("0.9rem"),
	)
	// Running out is the one reading here that should raise its voice; lasting is
	// stated calmly, because a good answer does not need emphasis to be believed.
	rule(".retire-short",
		color("var(--warn, var(--text))"),
	)
	rule(".retire-ok",
		color("var(--text)"),
	)

	// ─── FP-T1c: the return reading under the growth chart ───────────────────
	rule(".inv-returns",
		margin("0.75rem 0 0"),
		paddingTop("0.6rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".inv-return-line",
		margin("0.2rem 0"),
		fontSize("0.95rem"),
	)

	// ─── FP-T2a: the amortization schedule table ─────────────────────────────
	// Capped and scrollable: a 30-year mortgage is 360 rows, and a table that
	// pushes everything below it off the page is not a disclosure, it is a
	// takeover.
	rule(".loan-sched-wrap",
		marginTop("0.6rem"),
		maxHeight("22rem"),
		overflowY("auto"),
		overflowX("auto"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md, 8px)"),
		padding("0.5rem 0.6rem"),
	)
	rule(".loan-sched",
		width("100%"),
		borderCollapse("collapse"),
		fontSize("var(--type-13)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".loan-sched th",
		textAlign("right"),
		position("sticky"),
		top("0"),
		background("var(--surface, var(--bg))"),
		padding("0.25rem 0.5rem"),
		color("var(--text-dim)"),
		fontWeight("500"),
	)
	rule(".loan-sched td",
		textAlign("right"),
		padding("0.2rem 0.5rem"),
		whiteSpace("nowrap"),
	)
	rule(".loan-sched th:first-child, .loan-sched td:first-child",
		textAlign("left"),
	)

	// ─── FP-T1d: purchase history + the sale form ────────────────────────────
	rule(".lot-wrap",
		marginTop("0.5rem"),
	)
	rule(".lot-panel",
		marginTop("0.4rem"),
		padding("0.5rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md, 8px)"),
	)
	rule(".lot-row",
		display("grid"),
		gridTemplateColumns("7rem 5rem 7rem 1fr auto"),
		alignItems("center"),
		gap("0.4rem"),
		padding("0.15rem 0"),
		fontSize("var(--type-13)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".lot-add",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(8rem, 1fr))"),
		alignItems("end"),
		gap("0.5rem"),
		marginTop("0.5rem"),
	)
	rule(".sell-form",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(9rem, 1fr))"),
		gap("0.5rem"),
		marginTop("0.6rem"),
		padding("0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md, 8px)"),
	)
	// The preview and the actions span the whole form: the consequence of the
	// choice above should not sit in a column beside it.
	rule(".sell-preview, .sell-actions",
		gridColumn("1 / -1"),
	)
	rule(".sell-actions",
		display("flex"),
		gap("0.5rem"),
	)
	rule(".sell-gain",
		margin("0"),
		fontSize("1rem"),
	)

	// ─── FP-T1f: the income recorder ─────────────────────────────────────────
	rule(".income-wrap",
		marginTop("0.4rem"),
	)
	rule(".income-form",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit, minmax(8rem, 1fr))"),
		alignItems("end"),
		gap("0.5rem"),
		marginTop("0.4rem"),
		padding("0.5rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-md, 8px)"),
	)

}
