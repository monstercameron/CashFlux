// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/store"
)

func solp1App(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	ds := store.SampleDataset()
	ds.Transactions = make([]domain.Transaction, 3300)
	for i := range ds.Transactions {
		ds.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("solp1-%04d", i), AccountID: ds.Accounts[0].ID,
			CategoryID: ds.Categories[0].ID, Payee: "Market", Desc: "Weekly shopping",
			Date: time.Now().UTC().Truncate(24 * time.Hour), Amount: money.New(-1200, "USD")}
	}
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old; a.Store().Close() })
	return a
}

func TestSOLP1RecategorisePersistenceWorkGuard(t *testing.T) {
	testSOLP2RecordPersistence(t)
}

func TestSOLP1PersistProfile3300(t *testing.T) {
	a := solp1App(t)
	measure := func(label string, f func()) {
		start := time.Now()
		f()
		t.Logf("SOLP1-PERF %s %.3f ms", label, float64(time.Since(start).Microseconds())/1000)
	}
	measure("dataset export", func() {
		if _, err := localDatasetExport(a); err != nil {
			t.Fatal(err)
		}
	})
	measure("undo snapshot export+decode", func() {
		if _, err := currentSnapshot(); err != nil {
			t.Fatal(err)
		}
	})
	measure("notification candidates", func() { _ = buildNotifyCandidates(a, time.Now(), notify.RuleConfig{}) })
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	data, err := localDatasetExport(a)
	if err != nil {
		t.Fatal(err)
	}
	measure("ENC3 SealRecord+envelope encoding", func() {
		env, err := atrest.SealRecord(wk, keyring.LocalScope(""), "solp1-fixture", datasetStoreKey, data)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Encode(); err != nil {
			t.Fatal(err)
		}
	})
}
