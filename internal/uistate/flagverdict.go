// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/flagverdict"
)

// flagVerdictStore holds the judgements recorded against spending flags (WF-SM1)
// in the SQLite dataset's settings KV, so they travel with the dataset
// (export/import, backup, sync) like every other setting. The value is the JSON
// encoding of a flagverdict.Memory.
//
// It lives with the DATA rather than in browser storage because a verdict is a
// statement about the household's money, not about this browser: somebody who
// tells the app an annual charge is expected should not be asked again on their
// laptop.
const flagVerdictStore = "cashflux:flag-verdicts"

// LoadFlagVerdicts reads the recorded judgements, or an empty memory when none
// are set.
func LoadFlagVerdicts() flagverdict.Memory {
	return flagverdict.Load(SettingKVGet(flagVerdictStore))
}

// PersistFlagVerdicts saves the judgements, clearing the entry when the memory is
// empty so a fully-cleared memory leaves no residue.
func PersistFlagVerdicts(m flagverdict.Memory) {
	if len(m.Records) == 0 {
		SettingKVDelete(flagVerdictStore)
		return
	}
	SettingKVSet(flagVerdictStore, m.Marshal())
}

// RecordFlagVerdict stores a judgement against a flag and persists it.
func RecordFlagVerdict(key, subject string, v flagverdict.Verdict, at time.Time) {
	PersistFlagVerdicts(LoadFlagVerdicts().Record(flagverdict.Record{
		Key: key, Subject: subject, Verdict: v, At: at,
	}))
	RequestPersist()
}

// ForgetFlagVerdict drops the judgement for a key, bringing a flag silenced by
// mistake back.
func ForgetFlagVerdict(key string) {
	PersistFlagVerdicts(LoadFlagVerdicts().Forget(key))
	RequestPersist()
}
