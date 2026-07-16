// SPDX-License-Identifier: MIT

package styles

// registerGoalStatesWidget styles the dashboard "Goals at a glance" tile: a row of
// three count cells (Current / Missed / Completed) rendered as quiet buttons. Each is
// a large serif number over a small uppercase label. Only the Missed cell takes a
// tone (red) and only when it carries a count; Completed reads in the up/green so a
// finished goal feels earned. Theme tokens only, so it tracks light + dark.
func registerGoalStatesWidget() {
	rule(".dash-goalstates",
		display("grid"),
		gridTemplateColumns("repeat(3, 1fr)"),
		gap("0.5rem"),
		height("100%"),
		alignItems("stretch"),
	)
	// Each count is a button (opens /goals). Quiet by default — a hairline frame, no
	// fill — so the three read as one calm cluster, not three loud chips.
	rule(".dash-goalstates .dgs-cell",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		justifyContent("center"),
		gap("0.15rem"),
		padding("0.6rem 0.4rem"),
		border("1px solid var(--border-subtle)"),
		borderRadius("10px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		color("var(--text)"),
		font("inherit"),
		cursor("pointer"),
		transition("border-color 0.12s ease, background 0.12s ease, transform 0.08s ease"),
	)
	rule(".dash-goalstates .dgs-cell:hover",
		borderColor("var(--text-dim)"),
		background("color-mix(in srgb, var(--bg-elev) 75%, transparent)"),
	)
	rule(".dash-goalstates .dgs-cell:active",
		transform("translateY(1px)"),
	)
	rule(".dash-goalstates .dgs-cell:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// The number leads: big, serif, tabular so the three align.
	rule(".dash-goalstates .dgs-n",
		fontSize("1.7rem"),
		lineHeight("1.05"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".dash-goalstates .dgs-k",
		fontSize("0.62rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	// Completed: an earned green on the number.
	rule(".dash-goalstates .dgs-cell.is-done .dgs-n",
		color("var(--up, #4ea777)"),
	)
	// Missed: red tone + a faint red wash, drawn ONLY when the cell carries a count
	// (the widget adds .is-missed only for count > 0), so a clean slate stays calm.
	rule(".dash-goalstates .dgs-cell.is-missed",
		borderColor("color-mix(in srgb, var(--down, #d8716f) 45%, var(--border))"),
		background("color-mix(in srgb, var(--down, #d8716f) 8%, transparent)"),
	)
	rule(".dash-goalstates .dgs-cell.is-missed .dgs-n",
		color("var(--down, #d8716f)"),
	)
}
