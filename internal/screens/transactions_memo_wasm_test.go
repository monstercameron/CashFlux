// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestTxnFrameRowPropsCacheReusesOnlyUnchangedRows(t *testing.T) {
	cache := &txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)}
	first := cache.reuse(txnFrameRowProps{ID: "tx-a", Desc: "Market", Selected: false})
	unchanged := cache.reuse(txnFrameRowProps{ID: "tx-a", Desc: "Market", Selected: false})
	if first != unchanged {
		t.Fatal("unchanged transaction props did not keep their identity")
	}
	changed := cache.reuse(txnFrameRowProps{ID: "tx-a", Desc: "Market", Selected: true})
	if changed == unchanged {
		t.Fatal("selection change reused stale row props")
	}
	updated := cache.reuse(txnFrameRowProps{ID: "tx-a", Desc: "Updated Market", Selected: true})
	if updated == changed {
		t.Fatal("transaction value change reused stale row props")
	}
}

func TestTxnFrameRowPropsCacheStaysBounded(t *testing.T) {
	cache := &txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)}
	for i := 0; i < txnRowPropsCacheLimit+20; i++ {
		cache.reuse(txnFrameRowProps{ID: string(rune(i + 1))})
	}
	if len(cache.rows) > txnRowPropsCacheLimit || len(cache.order) > txnRowPropsCacheLimit {
		t.Fatalf("cache grew beyond its bound: rows=%d order=%d", len(cache.rows), len(cache.order))
	}
}
