// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/standing"
)

// standingStore holds the household's standing instructions (WF-SM4) in the
// SQLite dataset's settings KV, so they travel with the dataset like every other
// setting.
//
// With the DATA rather than the browser, for the same reason the flag verdicts
// are: "keep at least $15,000 liquid" is a decision about the household's money,
// not a preference of the machine it was typed on.
const standingStore = "cashflux:standing-instructions"

// LoadStanding reads the household's standing instructions.
func LoadStanding() standing.Book { return standing.Load(SettingKVGet(standingStore)) }

// PersistStanding saves them, clearing the entry when the book is empty so a
// fully-cleared book leaves no residue.
func PersistStanding(b standing.Book) {
	if b.Len() == 0 {
		SettingKVDelete(standingStore)
		return
	}
	SettingKVSet(standingStore, b.Marshal())
}

// SetStandingInstruction records one instruction and persists.
func SetStandingInstruction(i standing.Instruction) {
	if i.At.IsZero() {
		i.At = time.Now()
	}
	PersistStanding(LoadStanding().Set(i))
	RequestPersist()
}

// ForgetStandingInstruction lifts one instruction and persists.
func ForgetStandingInstruction(id string) {
	PersistStanding(LoadStanding().Forget(id))
	RequestPersist()
}
