// SPDX-License-Identifier: MIT

package styles

import (
	"fmt"
	"strings"
)

// This file defines the app's RESPONSIVE BREAKPOINT SYSTEM.
//
// CashFlux is a desktop-first shell: a fixed left rail beside one scrolling
// content pane (main.cf-scroll). A plain viewport @media query cannot describe
// the width a page actually has, because the rail eats 240px expanded but only
// 58px collapsed — the same 1280px window gives a page 1040px or 1222px
// depending on rail state. Historically every layout rule was written against
// the viewport with the collapsed rail implicitly assumed, so collapsing the
// sidebar never re-flowed anything and expanded-rail windows clipped.
//
// The system here makes layout respond to CONTENT WIDTH (viewport minus the
// live rail). Container queries would express this directly, but the app
// renders position:fixed overlays *inside* the pane (the assistant drawer, the
// ledger's action-sheet backdrops), and a container-type ancestor would become
// their containing block and pin them to the pane — so instead the shell
// mirrors the rail state onto <html> as the `cf-rail-c` class (see
// app/shell.go), and ruleContentMax / ruleContentMin compile one logical
// content-width condition into a pair of @media rules, one per rail state.
// The rail-state prefix is wrapped in :where() so both emissions keep exactly
// the specificity of the bare selector — the cascade behaves as if the browser
// supported "@media (content-width < N)" natively.
//
// THE SCALE — named content-width thresholds. Every layout cutover in the app
// uses one of these (matching the long-standing viewport idioms at their
// collapsed-rail equivalents: 768/1024/1100 viewport ≈ 710/966/1042 content):
//
//	contentGrid1  (710px) — below this the bento grid is a single column and
//	                        every tile spans the full pane.
//	contentGrid4  (966px) — below this the 4-column bento drops to 2 columns
//	                        with flow placement, and the transaction ledger
//	                        leaves table mode for stacked cards.
//	contentTwoCol (1042px) — below this side-by-side main+aside compositions
//	                        (reports chart pairs, workflows/fields/studio
//	                        composer rails, the assistant deck, the budgets
//	                        status strip) stack vertically.
//
// Viewport-based rules remain correct for things sized by the WINDOW, not the
// pane: the phone shell (≤640/767px), pointer/hover media, print, and the
// top bar's own two-row fold (≤1535px), which spans the pane at every rail
// state its thresholds cover. New page-layout rules should use the content
// helpers; new phone/input-modality rules should keep viewport queries.
const (
	// railExpandedPx is aside.rail's expanded width (tw.W60 in app/shell.go).
	railExpandedPx = 240
	// railCollapsedPx is aside.rail.collapsed's width (rules_gen.go).
	railCollapsedPx = 58

	// contentGrid1: below this content width the bento grid single-columns.
	contentGrid1 = 710
	// contentGrid4: below this content width the bento drops 4→2 columns and
	// the transaction ledger switches from table to stacked cards.
	contentGrid4 = 966
	// contentTwoCol: below this content width two-column page compositions
	// (main + aside/composer/chart-pair) stack.
	contentTwoCol = 1042
)

// railStateClass is the <html> class the shell maintains while the rail is
// collapsed. app/shell.go writes it; the helpers below key their expanded/
// collapsed emissions off it. Default (class absent) is treated as expanded —
// the conservative direction: layouts compact sooner, nothing clips.
const railStateClass = "cf-rail-c"

// ruleContentMax emits decls for selector whenever the content pane is at most
// maxContent px wide, whatever the rail state:
//
//   - viewport ≤ maxContent+58: true for BOTH rail states (an expanded rail
//     only makes the pane narrower), so the bare selector is emitted.
//   - viewport ≤ maxContent+240 with the rail expanded: emitted behind the
//     zero-specificity :where(html:not(.cf-rail-c)) prefix.
func ruleContentMax(maxContent int, selector string, decls ...decl) {
	ruleMedia(fmt.Sprintf("(max-width: %dpx)", maxContent+railCollapsedPx), selector, decls...)
	ruleMedia(fmt.Sprintf("(max-width: %dpx)", maxContent+railExpandedPx),
		prefixEachSelector(":where(html:not(."+railStateClass+")) ", selector), decls...)
}

// ruleContentMin is the inverse: decls apply whenever the content pane is at
// least minContent px wide (viewport ≥ minContent+240 unconditionally; from
// minContent+58 when the rail is collapsed).
func ruleContentMin(minContent int, selector string, decls ...decl) {
	ruleMedia(fmt.Sprintf("(min-width: %dpx)", minContent+railExpandedPx), selector, decls...)
	ruleMedia(fmt.Sprintf("(min-width: %dpx)", minContent+railCollapsedPx),
		prefixEachSelector(":where(html."+railStateClass+") ", selector), decls...)
}

// prefixEachSelector prepends prefix to every comma-separated selector in list
// (a CSS selector list is a top-level comma join; none of our selectors nest
// commas inside functional pseudo-classes with descendant parts).
func prefixEachSelector(prefix, list string) string {
	parts := strings.Split(list, ",")
	for i, p := range parts {
		parts[i] = prefix + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
