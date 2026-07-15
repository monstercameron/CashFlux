// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/agentmemory"

// agentMemoryStore holds the assistant's transparent memory (AG19) — the durable
// facts the user asked it to remember — in the SQLite dataset's settings KV, so it
// travels with the dataset (export/import, backup, sync) and is preserved across a
// data wipe like every other setting. The value is the JSON encoding of an
// agentmemory.Store.
const agentMemoryStore = "cashflux:agent-memory"

// LoadAgentMemory reads the assistant's remembered facts, or an empty store when
// none are set.
func LoadAgentMemory() agentmemory.Store {
	return agentmemory.Load(SettingKVGet(agentMemoryStore))
}

// PersistAgentMemory saves the assistant's remembered facts (clearing the entry
// when the store is empty so a fully-cleared memory leaves no residue).
func PersistAgentMemory(s agentmemory.Store) {
	if s.Len() == 0 {
		SettingKVDelete(agentMemoryStore)
		return
	}
	SettingKVSet(agentMemoryStore, s.Marshal())
}

// RememberFact appends a fact to the assistant's memory and persists it, returning
// whether it was actually added (a blank or duplicate fact is ignored). This is the
// single "remember this" entry point the chat's remember_fact tool and the Settings
// editor both call, so capture is always explicit and inspectable — never silent.
func RememberFact(fact string) bool {
	next, added := LoadAgentMemory().Add(fact)
	if added {
		PersistAgentMemory(next)
		RequestPersist()
	}
	return added
}
