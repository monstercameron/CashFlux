// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"io"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestBudgetDensityMigratesLegacyDatasetValueOnce(t *testing.T) {
	priorApp := appstate.Default
	priorScope, priorScopeApp := chromeScope, chromeScopeApp
	priorPreferences := chromePreferences
	chromePreferences = make(map[string]*chromePreference)
	appstate.Default = nil
	chromeScope, chromeScopeApp = "", nil
	t.Cleanup(func() {
		appstate.Default = nil // Keep the scheduled secure write from touching browser storage.
		appstate.Default = priorApp
		chromePreferences = priorPreferences
		chromeScope, chromeScopeApp = priorScope, priorScopeApp
	})

	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = nil })
	if err := app.SetKV(budgetDensityStoreID, BudgetDensityCompact); err != nil {
		t.Fatal(err)
	}
	storageKey := chromePreferenceKey(budgetDensityStoreID)
	previous, hadPrevious := browserstore.Get(storageKey)
	if hadPrevious {
		browserstore.Remove(storageKey)
	}
	t.Cleanup(func() {
		browserstore.Remove(storageKey)
		if hadPrevious {
			browserstore.Set(storageKey, previous)
		}
	})

	if got := loadBudgetDensityPreference(); got != BudgetDensityCompact {
		t.Fatalf("migrated density = %q, want compact", got)
	}
	pending := chromePreferences[storageKey]
	if pending == nil || pending.value != BudgetDensityCompact {
		t.Fatalf("device preference migration = %#v, want pending compact write", pending)
	}
	version := pending.version
	if got := loadBudgetDensityPreference(); got != BudgetDensityCompact {
		t.Fatalf("density on second read = %q, want compact", got)
	}
	if pending.version != version {
		t.Fatalf("second read restarted migration: version %d, want %d", pending.version, version)
	}
}

func TestBudgetDensityDevicePreferenceWinsOverLegacyValue(t *testing.T) {
	priorApp := appstate.Default
	priorScope, priorScopeApp := chromeScope, chromeScopeApp
	priorPreferences := chromePreferences
	chromePreferences = make(map[string]*chromePreference)
	appstate.Default = nil
	chromeScope, chromeScopeApp = "", nil
	t.Cleanup(func() {
		appstate.Default = nil
		appstate.Default = priorApp
		chromePreferences = priorPreferences
		chromeScope, chromeScopeApp = priorScope, priorScopeApp
	})

	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = nil })
	if err := app.SetKV(budgetDensityStoreID, BudgetDensityCompact); err != nil {
		t.Fatal(err)
	}
	storageKey := chromePreferenceKey(budgetDensityStoreID)
	previous, hadPrevious := browserstore.Get(storageKey)
	browserstore.Set(storageKey, BudgetDensityComfortable)
	t.Cleanup(func() {
		browserstore.Remove(storageKey)
		if hadPrevious {
			browserstore.Set(storageKey, previous)
		}
	})

	if got := loadBudgetDensityPreference(); got != BudgetDensityComfortable {
		t.Fatalf("density = %q, want device preference comfortable", got)
	}
	if _, migrated := chromePreferences[storageKey]; migrated {
		t.Fatal("legacy value overwrote the existing device preference")
	}
}
