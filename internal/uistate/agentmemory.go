// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/agentmemory"
	"github.com/monstercameron/CashFlux/internal/store"
)

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

// ReadAgentMemoryFromStore reads transparent memory from a caller-bound dataset.
// The raw value and existence bit let write previews detect edits made between
// approval and execution; storage failures remain visible to the caller.
func ReadAgentMemoryFromStore(s *store.SQLiteStore) (agentmemory.Store, string, bool, error) {
	if s == nil {
		return agentmemory.Store{}, "", false, fmt.Errorf("agent memory store is unavailable")
	}
	raw, exists, err := s.GetSettingKV(agentMemoryStore)
	if err != nil {
		return agentmemory.Store{}, "", false, err
	}
	return agentmemory.Load(raw), raw, exists, nil
}

// WriteAgentMemoryToStore persists transparent memory to a caller-bound dataset.
// The caller owns identity checks and the persist request so a stale screen cannot
// redirect a write through the globally active app.
func WriteAgentMemoryToStore(s *store.SQLiteStore, memory agentmemory.Store) error {
	if s == nil {
		return fmt.Errorf("agent memory store is unavailable")
	}
	if memory.Len() == 0 {
		return s.DeleteSettingKV(agentMemoryStore)
	}
	return s.SetSettingKV(agentMemoryStore, memory.Marshal())
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
