// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/store"
	"testing"
	"time"
)

func TestSettingsCheckedReturnsOwnedSnapshotAndInvalidatesOnStoreWrite(t *testing.T) {
	a := newApp(t, false)
	initial := store.Settings{
		BaseCurrency:       "USD",
		FXRates:            map[string]float64{"EUR": 1.1},
		FXUpdatedAt:        map[string]time.Time{"EUR": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		FreshnessOverrides: map[string]int{"cash": 9},
		PayoffBaseline:     &store.PayoffBaseline{TotalOwed: 1234, Currency: "USD"},
		Music:              &store.MusicState{Enabled: true, Volume: .4, Index: 2, Position: 8.5},
		AISpend:            []aispend.Bucket{{Feature: "assistant", Calls: 3, Tokens: 80}},
	}
	if err := a.Store().PutSettings(initial); err != nil {
		t.Fatal(err)
	}
	first, err := a.SettingsChecked()
	if err != nil {
		t.Fatal(err)
	}
	first.FXRates["EUR"] = 9
	first.FXUpdatedAt["EUR"] = time.Time{}
	first.FreshnessOverrides["cash"] = 99
	first.PayoffBaseline.TotalOwed = 0
	first.Music.Volume = 1
	first.AISpend[0].Tokens = 0
	second, err := a.SettingsChecked()
	if err != nil {
		t.Fatal(err)
	}
	if second.FXRates["EUR"] != 1.1 || second.FXUpdatedAt["EUR"].IsZero() || second.FreshnessOverrides["cash"] != 9 {
		t.Fatalf("caller mutation changed maps: %+v", second)
	}
	if second.PayoffBaseline.TotalOwed != 1234 || second.Music.Volume != .4 || second.AISpend[0].Tokens != 80 {
		t.Fatal("caller mutation changed nested settings")
	}
	updated := initial
	updated.BaseCurrency = "CAD"
	updated.FXRates = map[string]float64{"EUR": 1.2}
	if err := a.Store().PutSettings(updated); err != nil {
		t.Fatal(err)
	}
	third, err := a.SettingsChecked()
	if err != nil {
		t.Fatal(err)
	}
	if third.BaseCurrency != "CAD" || third.FXRates["EUR"] != 1.2 {
		t.Fatalf("direct write did not invalidate snapshot: %+v", third)
	}
}

func TestSettingsCheckedRejectsWarmClosedStore(t *testing.T) {
	a := newApp(t, false)
	if _, err := a.SettingsChecked(); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SettingsChecked(); err == nil {
		t.Fatal("warm cache hid a closed store")
	}
}

func TestSettingsSnapshotRetriesFailureAndLoadsOncePerRevision(t *testing.T) {
	var cache settingsSnapshotCache
	attempts := 0
	load := func() (store.Settings, error) {
		attempts++
		if attempts == 1 {
			return store.Settings{}, errors.New("temporary read failure")
		}
		return store.Settings{BaseCurrency: "USD"}, nil
	}
	if _, err := cache.get(42, load); err == nil || cache.loaded {
		t.Fatal("failed read was retained")
	}
	if value, err := cache.get(42, load); err != nil || value.BaseCurrency != "USD" || attempts != 2 {
		t.Fatalf("retry failed: attempts=%d error=%v", attempts, err)
	}
	if _, err := cache.get(42, load); err != nil || attempts != 2 {
		t.Fatal("warm snapshot queried source again")
	}
	if _, err := cache.get(43, load); err != nil || attempts != 3 {
		t.Fatal("new revision did not reload source")
	}
	for _, value := range []store.Settings{{}, {FXRates: map[string]float64{}, AISpend: []aispend.Bucket{}}} {
		cloned := cloneSettingsSnapshot(value)
		if (cloned.FXRates == nil) != (value.FXRates == nil) || (cloned.AISpend == nil) != (value.AISpend == nil) {
			t.Fatal("clone changed nil/empty shape")
		}
	}
}
