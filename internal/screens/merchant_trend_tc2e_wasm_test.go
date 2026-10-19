// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestMerchantTrendCacheIsRevisionScopedAndBounded(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	app := appstate.Default
	previous := mtrendStatsCache
	defer func() { mtrendStatsCache = previous }()

	mtrendCachePut(app, "Costco", mtrendCached{base: "USD", ok: true})
	if _, ok := mtrendCacheGet(app, "costco"); !ok {
		t.Fatal("merchant trend cache did not normalize merchant names")
	}
	if err := app.Store().PutTransaction(domain.Transaction{ID: "cache-revision", Amount: money.New(-100, "USD")}); err != nil {
		t.Fatalf("change ledger revision: %v", err)
	}
	if _, ok := mtrendCacheGet(app, "Costco"); ok {
		t.Fatal("merchant trend cache survived a ledger revision")
	}
	for i := 0; i < mtrendStatsCacheLimit+1; i++ {
		mtrendCachePut(app, fmt.Sprintf("merchant-%03d", i), mtrendCached{base: "USD", ok: true})
	}
	if got := len(mtrendCacheEntries(app)); got > mtrendStatsCacheLimit {
		t.Fatalf("merchant trend cache has %d entries, limit is %d", got, mtrendStatsCacheLimit)
	}
}
