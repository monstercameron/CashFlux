// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// UseRecurringEditID drives the add/edit-recurring flip modal (RecurringEditHost):
// "" = closed, "new" = create, or a recurring ID to edit. Only the shell-root host
// and the tiny trigger buttons subscribe, so opening/closing never re-renders the
// recurring surface (the flip-modal isolation lesson from the investments pools).
func UseRecurringEditID() state.Atom[string] { return state.UseAtom("recurring:editID", "") }

// UseSubsPrefsOpen drives the subscription-detection preferences flip modal
// (SubsPrefsHost): true = open. Same isolation rationale as UseRecurringEditID.
func UseSubsPrefsOpen() state.Atom[bool] { return state.UseAtom("subs:prefsOpen", false) }

// UseBillsSmartOpen drives the smart-pay-schedule flip modal (BillsSmartHost):
// true = open. Same isolation rationale as the other modal atoms.
func UseBillsSmartOpen() state.Atom[bool] { return state.UseAtom("bills:smartOpen", false) }
