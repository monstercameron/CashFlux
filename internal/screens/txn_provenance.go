// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/txnprov"
)

// txnAutoByID returns the rows whose category is automatic and unconfirmed, keyed
// by transaction id and read O(1) while building a row.
func txnAutoByID(app *appstate.App, txns []domain.Transaction, rs []rules.Rule, dataRev int) map[string]txnprov.Provenance {
	if !provCache.valid || provCache.app != app || provCache.revision != dataRev {
		provCache = txnProvenanceCache{
			app: app, revision: dataRev,
			values: make(map[string]txnprov.Provenance),
			seen:   make(map[string]bool), valid: true,
		}
	}
	for _, t := range txns {
		if !provCache.seen[t.ID] {
			p := txnprov.Of(t, rs)
			if p.IsAutomatic() {
				provCache.values[t.ID] = p
			}
			provCache.seen[t.ID] = true
		}
	}
	return provCache.values
}

type txnProvenanceCache struct {
	app      *appstate.App
	revision int
	values   map[string]txnprov.Provenance
	seen     map[string]bool
	valid    bool
}

var provCache txnProvenanceCache
