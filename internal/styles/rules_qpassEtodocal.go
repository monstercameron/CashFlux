// SPDX-License-Identifier: MIT

package styles

// registerTodoCalFit makes the To-do calendar's 7-day month grid FIT the content
// pane instead of clipping at the right edge with a hard-to-notice bottom-only
// scrollbar (2026-07-19 v1.2.7 review, lane E #2).
//
// The shared calendar primitive (rules_calendar.go) lays the weekday header row
// and each week out as `grid-template-columns: repeat(7, 1fr)`. A `1fr` track has
// an implicit `min-width: auto`, so a day cell whose content has intrinsic width
// (a task chip with a long nowrap title) can push its column past 1/7 of the
// pane, blowing the grid wider than its container — which then scrolls sideways,
// clipping tasks near the right edge with only a bottom scrollbar to reveal them.
//
// The fix, scoped to the To-do calendar (`.tcal`) so the shared date-picker
// calendar is untouched, is the canonical one: `minmax(0, 1fr)` lets every column
// shrink below its content's intrinsic width. The task chips already truncate
// (`.tcal-chip-title` ellipsizes and each chip carries its full title as a
// tooltip), so the grid now always sums to 100% of the pane and never overflows —
// no sideways scroll, nothing clipped — at expanded-sidebar widths and above.
//
// Chained from registerTodoCalSurface, so it is emitted after the base
// `.uical-week` / `.uical-weekdays` rules and, with the extra `.tcal` class,
// wins on specificity. Theme tokens only (no colour rules here).
func registerTodoCalFit() {
	// minmax(0, 1fr) — the min track size of 0 (vs 1fr's default auto min) is what
	// lets the seven day columns shrink to fit the pane; the header row and every
	// week share the same template so labels stay aligned over their columns.
	rule(".tcal .uical-weekdays, .tcal .uical-week",
		gridTemplateColumns("repeat(7, minmax(0, 1fr))"),
	)
}
