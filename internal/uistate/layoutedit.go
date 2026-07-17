// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const layoutEditAtomID = "dashboard:layout-edit"

// UseLayoutEdit returns the shared "edit layout" mode atom for the dashboard
// bento (QA task #76): while false — the default — the per-tile drag grips and
// resize handles stay hidden and pointer drag-reorder is off, so the everyday
// dashboard reads as content, not as a construction site. The toggle in the
// hero actions flips it on for deliberate rearranging. Session-only by design:
// edit mode is a moment, not a preference, so a reload always returns to the
// calm surface. (Keyboard grab/move stays available regardless — it requires
// an explicit grab and never fires by accident.)
func UseLayoutEdit() state.Atom[bool] { return state.UseAtom(layoutEditAtomID, false) }
