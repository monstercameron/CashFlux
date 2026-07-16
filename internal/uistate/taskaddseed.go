// SPDX-License-Identifier: MIT

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// TaskAddSeed pre-fills the add-task modal when it's opened from another surface
// (e.g. a transaction's "Add follow-up task…"): a suggested title and a pre-selected
// entity link. Empty fields simply leave the corresponding form field at its default.
type TaskAddSeed struct {
	Title    string
	LinkType string // domain.RelatedType value ("transaction", …); "" = no link
	LinkID   string
}

// capturedTaskAddSeed lets a click handler on another page (e.g. the transactions row
// kebab) seed the next add-task modal without calling UseAtom outside a render — the
// same captured-atom seam used for the sub-task parent and calendar due-date presets.
var (
	capturedTaskAddSeed state.Atom[TaskAddSeed]
	taskAddSeedCaptured bool
)

// UseTaskAddSeed returns the atom holding the pre-fill for the next add-task modal.
// AddHost reads it to seed TaskAddForm; calling it in a render captures the atom for
// SetTaskAddSeed.
func UseTaskAddSeed() state.Atom[TaskAddSeed] {
	a := state.UseAtom("todo:addSeed", TaskAddSeed{})
	capturedTaskAddSeed = a
	taskAddSeedCaptured = true
	return a
}

// SetTaskAddSeed sets (or clears with the zero value) the pre-fill for the next
// add-task modal. Safe from a click handler (uses the captured atom, not UseAtom).
func SetTaskAddSeed(s TaskAddSeed) {
	if taskAddSeedCaptured {
		capturedTaskAddSeed.Set(s)
	}
}
