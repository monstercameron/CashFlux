// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txntemplate"
)

// txnTemplatesKVKey is the single KV blob under which all transaction quick-
// templates ("favourites") are persisted, mirroring the JSON-blob-in-KV pattern
// used for occurrences and saved views. No SQL schema is involved.
const txnTemplatesKVKey = "cashflux:txntemplates"

// txnTemplatesCache lazily holds the decoded store so the picker can render
// without re-parsing the KV blob every frame. nil means "not loaded yet"; it is
// (re)populated on first read and refreshed in place whenever we persist.
var txnTemplatesCache *txntemplate.Store

// loadTxnTemplates returns the decoded store, reading + decoding the KV blob once
// and caching it. Decoding is tolerant, so a missing/corrupt blob reads as empty.
func loadTxnTemplates() *txntemplate.Store {
	if txnTemplatesCache != nil {
		return txnTemplatesCache
	}
	s, _ := txntemplate.Unmarshal(kvGet(txnTemplatesKVKey))
	txnTemplatesCache = &s
	return txnTemplatesCache
}

// persistTxnTemplates writes the current store back to KV.
func persistTxnTemplates(s *txntemplate.Store) {
	raw, err := txntemplate.Marshal(*s)
	if err != nil {
		return
	}
	kvSet(txnTemplatesKVKey, raw)
}

// TxnTemplates returns the saved quick-templates (favourites) in stored order. The
// returned slice is the live cache backing store; callers must not mutate it.
func TxnTemplates() []domain.TxnTemplate {
	return loadTxnTemplates().Items
}

// SaveTxnTemplate upserts a template (assigning an ID when blank) and persists the
// whole set. The updated template's ID is returned for callers that need it.
func SaveTxnTemplate(t domain.TxnTemplate) {
	s := loadTxnTemplates()
	s.Upsert(t)
	persistTxnTemplates(s)
}

// DeleteTxnTemplate removes the template with the given ID and persists the set.
func DeleteTxnTemplate(id string) {
	s := loadTxnTemplates()
	s.Delete(id)
	persistTxnTemplates(s)
}
