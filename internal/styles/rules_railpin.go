// SPDX-License-Identifier: MIT

package styles

// registerRailPin styles the pinned section and the per-row pin toggle.
func registerRailPin() {
	// The row is a flex pair: the link takes the space, the pin sits at the end.
	// Deliberately NOT position:relative — the active indicator measures the link's
	// offsetTop against the nav, and a positioned ancestor here would reparent that
	// measurement and park the highlight at the top of the rail.
	rule(".nav-row",
		display("flex"),
		alignItems("center"),
		gap("0.15rem"),
	)
	rule(".nav-row > a", flex("1 1 auto"), minWidth("0"))

	// The pin is quiet until you go looking for it. Thirty always-visible stars
	// would make the rail read as a settings screen rather than a menu — but it
	// stays in the DOM and reveals on focus, so the keyboard path is never hidden.
	rule(".nav-pin",
		flex("0 0 auto"),
		display("grid"),
		placeItems("center"),
		width("1.5rem"),
		height("1.5rem"),
		borderRadius("var(--radius-sm)"),
		color("var(--text-faint)"),
		cursor("pointer"),
		opacity("0"),
		transition("opacity var(--motion-micro) var(--ease-exit), color var(--motion-micro) var(--ease-exit)"),
	)
	rule(".nav-row:hover .nav-pin, .nav-row:focus-within .nav-pin, .nav-pin:focus-visible",
		opacity("1"),
	)
	// A pinned row keeps its star lit IN A FOLDER, where that is the only way to
	// tell which rows are pinned without hovering each one.
	rule(".nav-pin.is-pinned", opacity("1"), color("var(--accent)"))
	// Inside the Pinned section it goes quiet again: every row there is pinned by
	// definition, so a column of lit stars states something the heading already
	// said, and the control is still one hover or Tab away when you want to unpin.
	rule(".rail-pinned .nav-pin.is-pinned", opacity("0"), color("var(--accent)"))
	rule(".rail-pinned .nav-row:hover .nav-pin, .rail-pinned .nav-row:focus-within .nav-pin",
		opacity("1"),
	)
	rule(".nav-pin:hover", color("var(--text)"))
	rule(".nav-pin.is-pinned:hover", color("var(--accent)"))
	// No disabled state remains: a full list makes the control ask a question
	// rather than refuse, so there is nothing to grey out.
	// Reduced motion: the reveal is instant rather than faded.
	ruleMedia("(prefers-reduced-motion: reduce)", ".nav-pin", transition("none"))

	// The pinned block sits above the folders and never collapses.
	rule(".rail-sec-head",
		padding("0.35rem 0.75rem 0.25rem"),
		color("var(--text-faint)"),
		fontSize("0.6875rem"),
		fontWeight("600"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
	)
	rule(".rail-pinned",
		display("flex"),
		flexDirection("column"),
		gap("0.125rem"),
		marginBottom("0.35rem"),
	)
	rule(".rail-pinned-empty",
		padding("0.25rem 0.75rem 0.6rem"),
		color("var(--text-faint)"),
		fontSize("0.78rem"),
		lineHeight("1.45"),
	)
	// The collapsed rail is an icon strip: section headings and the empty-state
	// sentence have no room, and the pinned rows themselves stay as icons.
	rule(".rail.collapsed .rail-sec-head, .rail.collapsed .rail-pinned-empty",
		display("none"),
	)
	rule(".rail.collapsed .nav-pin", display("none"))

	// ── the swap question ────────────────────────────────────────────────────
	//
	// The prompt sits above the pinned rows it is asking about, so the question and
	// the answers read in that order rather than the reader hunting for what
	// changed.
	rule(".rail-swap",
		margin("0 0.5rem 0.4rem"),
		padding("0.5rem 0.6rem"),
		borderRadius("var(--radius-sm)"),
		background("var(--bg-elev)"),
		border("1px solid var(--accent)"),
	)
	rule(".rail-swap-q",
		color("var(--text)"),
		fontSize("0.8125rem"),
		fontWeight("600"),
		lineHeight("1.35"),
	)
	rule(".rail-swap-hint",
		marginTop("0.15rem"),
		color("var(--text-dim)"),
		fontSize("0.75rem"),
		lineHeight("1.4"),
	)
	rule(".rail-swap-cancel",
		marginTop("0.4rem"),
		color("var(--text-dim)"),
		fontSize("0.75rem"),
		textDecoration("underline"),
		cursor("pointer"),
	)
	rule(".rail-swap-cancel:hover", color("var(--text)"))
	// Every pinned row is an offer while the question stands, so they read as one
	// set of choices rather than as the menu they were a moment ago.
	// The dashed hairline was --border on a dark ground and effectively invisible,
	// so the rows still read as the menu they had just stopped being. --border-strong
	// is the token that survives that background.
	rule(".nav-swap-target",
		width("100%"),
		textAlign("left"),
		border("1px dashed var(--border-strong)"),
		background("transparent"),
		color("var(--text-dim)"),
	)
	rule(".nav-swap-target:hover",
		borderColor("var(--accent)"),
		color("var(--text)"),
		background("var(--hover)"),
	)
}

// registerWsIdentity styles the merged workspace + household control.
func registerWsIdentity() {
	// The trigger is now two lines: the workspace it switches, and the household
	// glance that used to live in the footer. The uppercase heading it replaces was
	// labelling a control whose value already named itself.
	rule(".ws-switch-sub",
		display("block"),
		marginTop("0.15rem"),
		color("var(--text-faint)"),
		fontSize("0.6875rem"),
		lineHeight("1.3"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".ws-menu-privacy",
		display("flex"),
		alignItems("flex-start"),
		gap("0.35rem"),
		padding("0.25rem 0.5rem"),
		color("var(--text-faint)"),
		fontSize("0.6875rem"),
		lineHeight("1.4"),
	)
	rule(".ws-menu-meta",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
		padding("0.35rem 0.5rem 0.15rem"),
		fontSize("0.6875rem"),
		color("var(--text-faint)"),
	)
	// The collapsed rail shows an icon-only trigger, so the second line has no room.
	rule(".rail.collapsed .ws-switch-sub", display("none"))
}

// registerRailDrag styles the pinned list's drag-to-reorder.
func registerRailDrag() {
	// A grab cursor is the only standing hint that these rows move; the rest of
	// the affordance appears while dragging, where it is actually useful.
	rule(".rail-pinned .nav-row > a", cursor("grab"))
	rule(".rail-pinned .nav-row > a:active", cursor("grabbing"))
	// The row being dragged fades so the gap it will leave is readable.
	rule(".rail-pinned .nav-row > a[draggable=\"true\"]:active", opacity("0.55"))
}
