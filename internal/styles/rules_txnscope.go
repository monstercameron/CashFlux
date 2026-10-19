// SPDX-License-Identifier: MIT

package styles

// registerTxnScope emits the styles for the transactions ledger's scope model
// (C574/C575/C578): the filter summary's lead-in, the "viewing as" member-lens
// chip, and the word a row shows beside its needs-review dot.
//
// Theme tokens only, so it works in light and dark. Registered late in install so
// its equal-specificity rules win over the earlier generic chip styles.
func registerTxnScope() {
	// --- Filter summary lead-in ---------------------------------------------------
	// The chip row is a sentence about the view ("Filtering by: Groceries, July"),
	// so the lead-in is quiet, uppercase and non-shrinking: a label, not a chip.
	rule(".filter-chips-lead",
		flexShrink("0"),
		fontSize("var(--fs-meta)"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)

	// --- The member-lens chip -----------------------------------------------------
	// It comes from the top bar, not the filter panel, and its ✕ clears the top bar.
	// An accent left edge marks it as a different KIND of chip so the difference is
	// visible before the click, not discovered after it.
	rule(".filter-chip.filter-chip-lens",
		borderColor("var(--accent)"),
		boxShadow("inset 2px 0 0 0 var(--accent)"),
	)

	// --- The pending band, collapsed by default (C582) ----------------------------
	// It stops being one big navigating button: the band is a summary you read, and
	// the two things you might want to do with it — see the items, go to the
	// schedule — are named controls inside it. Collapsed it is a single line.
	rule(".txn-upcoming",
		cursor("default"),
		padding("0.4rem 0.85rem"),
		marginBottom("0.5rem"),
	)
	rule(".txn-upcoming:hover, .txn-upcoming:focus-visible",
		borderColor("var(--border)"),
	)
	rule(".txn-upcoming.is-collapsed .txn-upcoming-head",
		marginBottom("0"),
	)
	// The disclosure sits at the far end of the summary line, where the eye lands
	// after reading the fact — not before it.
	rule(".txn-upcoming-disc",
		marginLeft("auto"),
		flexShrink("0"),
		fontSize("var(--fs-body)"),
	)
	rule(".txn-upcoming-link",
		marginTop("0.35rem"),
		fontSize("var(--fs-body)"),
		alignSelf("flex-start"),
	)

	// --- Count line + legend share one row (C575 / C582) --------------------------
	// Both answer "how do I read the rows below", and every row of chrome above the
	// ledger is a row the user scrolls past to reach what they came for. The count
	// takes the left, the legend the right; they stack on narrow widths.
	rule(".txn-toolbar-foot",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.25rem 1rem"),
		padding("0.35rem 0.1rem 0.1rem"),
	)
	// The count is the sentence that gives every other number on the page its
	// denominator, so it is normal reading weight, not the dim caption the hidden
	// summary used to be.
	rule(".txn-countline",
		display("flex"),
		flexWrap("wrap"),
		alignItems("baseline"),
		gap("0.35rem"),
		fontSize("var(--fs-body)"),
		color("var(--text)"),
	)
	rule(".txn-countline .txn-count-sep, .txn-countline .txn-count-net, .txn-countline .txn-count-lens",
		color("var(--text-dim)"),
	)
	rule(".txn-toolbar-foot .txn-legend",
		padding("0"),
	)

	// --- The edit modal's destructive action is set apart (C580) ------------------
	// Delete ends the form, below a hairline, with room around it — so it is not one
	// more control in the run of fields, and a mis-aimed click on the last field
	// cannot land on it.
	//
	// It is NOT sticky. Pinning it to the foot of the scrolling body was tried and
	// reverted: `position:sticky; bottom:0` keeps the element in flow, so the band
	// floated over the Note field and clipped the two settings after it. Making it a
	// genuine footer control needs the FlipPanel footer itself, which means the
	// modal HOST owning the delete handler — a larger change than this ticket, and
	// one worth doing properly rather than approximating with an overlay.
	rule(".txn-edit-danger",
		display("flex"),
		marginTop("1rem"),
		paddingTop("0.75rem"),
		borderTop("1px solid var(--border)"),
	)

	// --- "Recategorize the similar ones too?" reads as a question (C580) ----------
	// Question, then evidence, then answers — down the panel, with the answers free
	// to wrap onto a second line. As a form-grid this was three auto-fit columns and
	// the three answer buttons shared the last one, clipped mid-word.
	rule(".txn-recat-offer",
		display("flex"),
		flexDirection("column"),
		gap("0.6rem"),
	)
	rule(".txn-recat-actions",
		flexWrap("wrap"),
	)

	// --- The category picker gives the SELECT the width (C580) --------------------
	// "New category" is a secondary action beside the control the field is actually
	// for, and at the modal's two-column width it was taking more than half the row
	// — leaving the select clipped to "— No categ". The select now claims a real
	// minimum and the button wraps beneath it when there is not room for both.
	rule(".txn-cat-row",
		flexWrap("wrap"),
	)
	rule(".txn-cat-row .field",
		flex("1 1 12rem"),
		minWidth("12rem"),
	)

	// --- The review header names its own arithmetic (C602) ------------------------
	// "249 charges left → 9 merchants to decide" with the joining words on the line,
	// not hidden in the arrow's tooltip. Quiet and small so it joins the two figures
	// rather than competing with them.
	rule(".rvs-joiner",
		fontSize("var(--fs-meta)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)

	// --- The review card's queue reason reads as a phrase (C600) ------------------
	// Uppercase + 700 weight made "QUEUED: NO CATEGORY ON FILE" shout like a status
	// stamp, which is exactly the reading that put it in competition with the
	// Category control below it. It is a note about why the charge is here, so it is
	// set as one: sentence case, normal tracking, still tinted.
	rule(".rvw-reason",
		prop("text-transform", "none"),
		fontWeight("600"),
		letterSpacing("0"),
		fontSize("var(--fs-meta)"),
	)

	// --- The review card's pending-decision note (C600) ---------------------------
	// Quiet, directly under the category control it qualifies: the card now states
	// three different things about one charge (why it is queued, what would be
	// assigned, and that nothing has been), so the third has to read as a footnote
	// to the second rather than a fourth competing label.
	rule(".rvs-pending",
		margin("0.25rem 0 0"),
		fontSize("var(--fs-meta)"),
		color("var(--text-dim)"),
	)

	// --- The one-hop return crumb (C581) ------------------------------------------
	// A thin band under the top bar on the page a side trip landed on. Quiet — it is
	// a thread back, not an alert — but the back control is a real button, because a
	// link styled as prose is the thing users do not find when they want out.
	rule(".return-crumb",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		padding("0.4rem 1.25rem"),
		borderBottom("1px solid var(--border)"),
		background("var(--bg-elev)"),
		fontSize("var(--fs-body)"),
	)
	rule(".return-crumb-note",
		fontSize("var(--fs-meta)"),
	)
	rule(".return-crumb-x",
		marginLeft("auto"),
	)

	// --- The primary Add, split (C573) --------------------------------------------
	// One control, two targets: press the label to add, press the caret to say what
	// kind first. They read as one object — a seam rather than a gap — so the caret
	// is unmistakably a modifier of the button beside it and not a fifth toolbar
	// action competing with it.
	rule(".btn-split",
		display("inline-flex"),
		alignItems("stretch"),
	)
	rule(".btn-split > .btn:first-child",
		prop("border-top-right-radius", "0"),
		prop("border-bottom-right-radius", "0"),
	)
	rule(".btn-split .btn-split-caret",
		prop("border-top-left-radius", "0"),
		prop("border-bottom-left-radius", "0"),
		borderLeft("1px solid rgba(0,0,0,0.22)"),
		paddingLeft("0.5rem"),
		paddingRight("0.5rem"),
	)

	// Category text and its optional suggestion/provenance marks remain on one line.
	rule(".td-cat-inner",
		display("flex"),
		alignItems("center"),
		gap("0.3rem"),
		minWidth("0"),
		flexWrap("nowrap"),
	)
	rule(".td-cat-name",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		minWidth("0"),
		flex("0 1 auto"),
	)
	rule(".td-cat-inner .txn-auto-mark, .td-cat-inner .txn-catsuggest",
		flex("0 0 auto"),
		whiteSpace("nowrap"),
	)
	rule(".td-cat-inner .txn-auto-mark",
		fontSize("var(--fs-meta)"),
		lineHeight("1"),
		color("var(--text-dim)"),
		border("0"),
		padding("0"),
	)
	rule(".td-cat-inner .txn-catsuggest",
		color("var(--accent)"),
		background("transparent"),
		border("0"),
		padding("0"),
		font("inherit"),
		cursor("pointer"),
	)
	rule("#txn-ledger-anchor",
		prop("scroll-margin-top", "calc(var(--dt-sticky-top, 3.5rem) + 0.5rem)"),
	)
	rule(".txn-skip-link",
		position("absolute"),
		left("-10000px"),
		top("0"),
		zIndex("100"),
	)
	rule(".txn-skip-link:focus",
		left("0.75rem"),
		top("0.75rem"),
		padding("0.5rem 0.75rem"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		border("2px solid var(--accent)"),
		borderRadius("var(--radius-md)"),
	)
	// Lower-case, dim and small: this marks the ORDINARY case (most imported rows
	// are unconfirmed), so it must not shout. It earns its place by being a word
	// rather than a colour, and by explaining itself on hover and to a screen reader.
	rule(".txn-auto-mark",
		fontSize("var(--fs-meta)"),
		lineHeight("1"),
		color("var(--text-dim)"),
		cursor("help"),
	)
}
