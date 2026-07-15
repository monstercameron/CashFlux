// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// institutionsManagerAtomID keys the shell-root flip modal for the institution
// directory (AC10): false = closed, true = open. The modal itself lists every
// institution and toggles into an inline add/edit sub-form, so unlike the account
// group editor it does not need to carry a target id.
const institutionsManagerAtomID = "accounts:institutionsManager"

// UseInstitutionsManager returns the atom driving the institution-directory flip
// modal. Read by the shell-root InstitutionsManagerHost and set by the /accounts
// toolbar's "Institutions" trigger.
func UseInstitutionsManager() state.Atom[bool] {
	return state.UseAtom(institutionsManagerAtomID, false)
}
