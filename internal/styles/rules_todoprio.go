// SPDX-License-Identifier: MIT

package styles

// registerTodoPrioSurface styles the To-do row's neutral priority tag and pins the
// headline lead group. Priority USED to be encoded in the checkbox ring colour (red
// high / green medium), which collided with the app's semantic colours — green is the
// done-fill and red is "Overdue" — so an incomplete medium task showed a green
// "looks-done" ring right next to red overdue text. The ring is now reserved for the
// done state; priority moved to this off-scale (neutral) tag. Registered after
// registerGenerated() so it wins the cascade.
func registerTodoPrioSurface() {
	// Keep the priority tag + title together on the left; the due chip stays pinned right
	// by the headline's space-between. min-width:0 lets a long title truncate instead of
	// shoving the due date off the row.
	rule(".todo-headline-lead",
		display("flex"),
		alignItems("baseline"),
		gap("0.5rem"),
		minWidth("0"),
	)
	// Fixed-width priority slot: reserves room for the High/Low tag so titles align in a
	// column even on the (majority) unmarked Medium rows. Sized to fit the tag snugly.
	rule(".todo-prio-slot",
		flex("none"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("flex-start"),
		minWidth("2.6rem"),
	)

	// A quiet uppercase tag — legible as text (not colour-coded), so it never fights the
	// overdue-red / done-green signals. High reads a touch stronger; Low sits fainter.
	rule(".todo-prio",
		flex("none"),
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.4rem"),
		borderRadius("var(--radius-sm)"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.6rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		lineHeight("1.5"),
		whiteSpace("nowrap"),
	)
	rule(".todo-prio.is-high",
		color("var(--text)"),
		borderColor("color-mix(in srgb, var(--text-dim) 70%, transparent)"),
		background("color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".todo-prio.is-low",
		opacity("0.6"),
	)
	// A done row's priority tag is past-tense — mute it so the completed title leads.
	rule(".todo-item.is-done .todo-prio",
		opacity("0.5"),
	)
}
