// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// accountGroupEditAtomID keys the shell-root flip modal for creating or editing an
// account group (AC1): "" = closed, "new" = create, or a group id = edit that group.
const accountGroupEditAtomID = "accounts:groupEdit"

// UseAccountGroupEdit returns the atom driving the create/edit account-group flip
// modal: "" = closed, "new" = create, or a group id = edit that group. Read by the
// shell-root AccountGroupsEditHost and set by the /accounts group triggers.
func UseAccountGroupEdit() state.Atom[string] {
	return state.UseAtom(accountGroupEditAtomID, "")
}
