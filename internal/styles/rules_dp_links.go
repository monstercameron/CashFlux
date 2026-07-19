// SPDX-License-Identifier: MIT

package styles

// registerDpLinks re-neutralizes GENERIC NAVIGATION links from the consistency-audit
// pass, so green stays trustworthy for FINANCIAL meaning (a positive balance / +change).
//
// Green was overloaded: positive money AND plain navigation both rendered in
// var(--accent). This pass pulls the plain-navigation links — "View all …", a widget
// list's open link, an assistant note jump, a report's plan link, a budget drill — back
// to a NEUTRAL link tone (quiet var(--text-dim), brightening to var(--text) with an
// underline on hover). It touches ONLY these anchor/link classes, using only `color`
// and `text-decoration`; it never targets money text (balances, +changes,
// .money-positive), so a positive figure keeps its green. Green links are now reserved
// for the rare link that advances the current financial task.
//
// Theme tokens only (var(--text)/var(--text-dim)), so light and dark both track.
// Registered LAST in install.go, so each override ties the original single-class
// selector's specificity and wins on source order.
func registerDpLinks() {
	// The neutral resting + hover treatment shared by every re-neutralized nav link.
	// v1.2.3 motion spec §3: on hover the color strengthens and the underline grows
	// left-to-right over the fast token (a bottom gradient, so width can animate)
	// instead of popping in; the text never moves.
	neutralLink := func(sel string) {
		rule(sel,
			color("var(--text-dim)"),
			textDecoration("none"),
			background("linear-gradient(currentColor, currentColor) no-repeat 0 100% / 0% 1px"),
			transition("background-size var(--motion-fast) var(--ease-standard), color var(--motion-fast) var(--ease-standard)"),
		)
		rule(sel+":hover", color("var(--text)"), prop("background-size", "100% 1px"))
	}

	// Report "plan" action link (reports-annual) — plain navigation to a plan/budget.
	neutralLink(".rpta-plan-link")
	// Widget-studio list open link — plain navigation into a studio item.
	neutralLink(".studio-list-link")
	// Assistant note jump link — plain navigation, not a money action.
	neutralLink(".ask-note-link")
	// Dashboard "View all N bills" link — plain navigation to the full list.
	neutralLink(".dash-view-all")

	// Budget drill ("Details" drill-down): base already inherits a neutral color and a
	// dotted underline; only its :hover turned green. Match the generated selector's
	// specificity (0,3,0) and win on source order so the hover brightens to text, not
	// accent. (Keeps the existing dotted underline from the base rule.)
	rule(".bento-budgets .budget-drill:hover", color("var(--text)"))
}
