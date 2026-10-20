// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
)

func primarySmartApp(t *testing.T, payee string) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Type: domain.TypeCash, Class: domain.ClassAsset, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "duplicate"} {
		if err := app.PutTransaction(domain.Transaction{ID: id, AccountID: "cash", Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), Payee: payee, Desc: payee, Amount: money.New(-1000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestPrimarySmartInputReusesOnlyMatchingContext(t *testing.T) {
	app := primarySmartApp(t, "Cafe")
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	first := buildSmartInputAt(app, time.Monday, now)
	second := buildSmartInputAt(app, time.Monday, now.Add(30*time.Second))
	if &first.Transactions[0] != &second.Transactions[0] || !second.Now.Equal(now) {
		t.Fatal("unchanged context recopied the ledger")
	}
	week := buildSmartInputAt(app, time.Sunday, now)
	if week.WeekStart != time.Sunday {
		t.Fatal("week preference reused stale input")
	}
	next := buildSmartInputAt(app, time.Sunday, now.Add(time.Minute))
	if !next.Now.Equal(now.Add(time.Minute)) {
		t.Fatal("clock bucket did not refresh")
	}
	txn := app.Transactions()[0]
	txn.Desc = "Changed"
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	changed := buildSmartInputAt(app, time.Sunday, now.Add(time.Minute))
	if changed.Transactions[0].Desc != "Changed" && changed.Transactions[1].Desc != "Changed" {
		t.Fatal("mutation reused stale input")
	}
	if smartInputCache.Len() != 1 {
		t.Fatal("full-ledger cache retained obsolete snapshots")
	}
}

func TestPrimarySmartFindingsSharePassAndIsolateApps(t *testing.T) {
	firstApp := primarySmartApp(t, "Cafe")
	secondApp := primarySmartApp(t, "Different merchant")
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if firstApp.Rev() != secondApp.Rev() {
		t.Fatal("fixture needs shared global revision")
	}
	for _, app := range []*appstate.App{firstApp, secondApp} {
		want := smartengine.Run(buildSmartInputRawAt(app, time.Monday, now), smart.EnableFreeOnly(smart.Settings{}))
		first := freeSmartFindings(app, time.Monday, now)
		if !reflect.DeepEqual(first, want) {
			t.Fatal("cached findings differ from engine result")
		}
		if len(first) == 0 {
			t.Fatal("duplicate fixture produced no findings")
		}
		second := freeSmartFindings(app, time.Monday, now.Add(20*time.Second))
		if &first[0] != &second[0] {
			t.Fatal("two finding sections reran the engine")
		}
	}
	if freeFindingsCache.Len() != 1 {
		t.Fatal("findings cache exceeded one context")
	}
}
