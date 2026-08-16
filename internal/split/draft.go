// SPDX-License-Identifier: MIT

package split

import "strings"

// Line is one row of a split draft as the editor holds it: a category id and a
// value string the user typed (an amount, or a percentage in percent mode).
// Both are raw — classification deliberately happens before parsing, because a
// row's COMPLETENESS is a different question from whether its number is valid.
type Line struct {
	CategoryID string
	Value      string
}

// Draft summarises a split draft's shape: how many rows are finished, how many
// are untouched placeholders, and how many are half-written.
//
// C566: the split editor seeds a blank second row so "carve a piece out" is one
// tap away, and the balance arithmetic skipped blank values — so an untouched
// draft summed to exactly the transaction amount and the footer reported
// "Balanced" beside an enabled Save, on a draft that could only fail validation.
// The remainder was telling the truth about the money and lying about the split.
//
// Classifying rows is therefore its own question, answered once, here, in pure
// Go — so the footer text and the Save button read the same verdict and cannot
// contradict each other again.
type Draft struct {
	// Complete counts rows carrying BOTH a category and a value: the only rows
	// that will survive into the saved breakdown.
	Complete int
	// Blank counts wholly untouched rows. They are drafts, not errors — harmless
	// and ignored on save — but they are also what stops a one-line draft from
	// being a split, so the UI names them rather than silently skipping them.
	Blank int
	// Incomplete counts half-written rows: a category with no amount, or an
	// amount with no category. These block a save and are worth pointing at.
	Incomplete int
}

// Classify summarises the draft's rows. A value is considered present when it
// holds anything other than whitespace, matching what the user sees in the field.
func Classify(lines []Line) Draft {
	var d Draft
	for _, l := range lines {
		hasCat := strings.TrimSpace(l.CategoryID) != ""
		hasVal := strings.TrimSpace(l.Value) != ""
		switch {
		case hasCat && hasVal:
			d.Complete++
		case !hasCat && !hasVal:
			d.Blank++
		default:
			d.Incomplete++
		}
	}
	return d
}

// MinSplitLines is how many complete rows a breakdown needs to be a split at
// all. One line is not a split — it is just the transaction's own category.
const MinSplitLines = 2

// Saveable reports whether a draft may be committed: every started row finished,
// at least MinSplitLines of them, every value parseable, and the money fully
// accounted for.
//
// It takes the arithmetic verdicts (parseOK, balanced) rather than computing
// them, because the totals differ by entry mode — minor units against the
// transaction amount, or basis points against 100% — and that arithmetic already
// lives with the mode. What this owns is the RULE, so one expression decides the
// Save button's state in the inline editor and in the modal footer alike.
func (d Draft) Saveable(parseOK, balanced bool) bool {
	return parseOK && balanced && d.Incomplete == 0 && d.Complete >= MinSplitLines
}
