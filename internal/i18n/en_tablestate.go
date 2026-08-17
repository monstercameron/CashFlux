// SPDX-License-Identifier: MIT

package i18n

// tableStateKeys holds what a data table says about ITSELF: whether it is
// mid-update, what order it settled into, and — when its body is windowed — how
// much of itself it is really rendering.
//
//   - C625 — a sort changed the announced state about a second before the rows
//     caught up, so a fast reader could act on the old order under the new label.
//   - C626 — "All" announced 1–3284 of 3284 while ~40 rows existed in the DOM,
//     with nothing telling assistive tech that the other 3,244 were not there to
//     be tabbed through.
//   - C628 — the same shape again, from the search debounce.
var tableStateKeys = Catalog{
	"ui.table.sorting":  "Re-sorting the table…",
	"ui.table.updating": "Updating the table…",
	"ui.table.sortedBy": "Sorted by %s, %s.",
	// Direction as a word, so the announcement does not depend on the caret.
	"ui.table.ascending":  "ascending",
	"ui.table.descending": "descending",
	// The honest version of the "1–3284 of 3284" the pager shows: that count is
	// about the matches, not about the rows that exist to be reached. Plain words,
	// not "viewport"/"rendered" — this is read aloud to a person, and the only half
	// of it they can act on is the page controls, so that is the half that gives an
	// instruction. The count arrives pre-pluralized from the caller.
	"ui.table.virtualCaption": "%s in total. Only the ones near where you are reading " +
		"are loaded — use the page controls to reach the rest.",

	// C628: the way out of a no-match state belongs where the disappointment is,
	// and it has to say how much of the view it takes with it. "Your other filters"
	// rather than a list: naming three of the ten dimensions leaves the reader
	// wondering about the seven it skipped, which is the opposite of reassurance.
	"transactions.noMatchSearch":       "Nothing matches “%s”.",
	"transactions.noMatchKeepsFilters": "This clears only the search — your other filters stay as they are.",
}

func init() {
	for k, v := range tableStateKeys {
		english[k] = v
	}
}
