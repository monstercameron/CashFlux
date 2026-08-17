// SPDX-License-Identifier: MIT

package i18n

// dupeSafetyKeys holds the strings C652 adds to duplicate review.
//
// C571 already made the confirmation name the entry being removed. What it did
// not do was name the one being KEPT, and in a group of three near-identical
// rows that is the only question the reader actually has. Both entries now print
// with the four facts that separate them — description, date, amount, account —
// and the merge preview says what a merge does to tags and cleared status, which
// dedupe.Merge unions without asking.
var dupeSafetyKeys = Catalog{
	"duplicates.entryIdentity": "“%s” on %s for %s in %s",
	"duplicates.noAccount":     "no account",
	// ONE sentence, deliberately. The dialog renders its message as a single <p>
	// with no `white-space: pre-line`, so an earlier draft's "Removing: …\nKeeping:
	// …" collapsed into an unbroken wall of text — the exact ambiguity this ticket
	// exists to remove, with the line break that was carrying the distinction
	// silently thrown away by the browser. The two entries are separated by
	// grammar instead, which the component can actually render.
	"duplicates.deleteConfirmPair": "Remove the copy %s, and keep %s? " +
		"The kept entry is untouched, and you can undo this from the toast or from Activity.",
	"duplicates.carryTags":     "the tag %s",
	"duplicates.carryTagsMany": "the tags %s",
	"duplicates.carryCleared":  "cleared status",
}

func init() {
	for k, v := range dupeSafetyKeys {
		english[k] = v
	}
}
