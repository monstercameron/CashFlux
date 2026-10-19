// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestAutosaveRefusesQuarantinedData(t *testing.T) {
	oldQuarantined, oldPending, oldHydrated := datasetQuarantined, quarantinePending, hydrationSucceeded
	t.Cleanup(func() {
		datasetQuarantined, quarantinePending, hydrationSucceeded = oldQuarantined, oldPending, oldHydrated
	})
	datasetQuarantined, quarantinePending, hydrationSucceeded = true, false, true
	if !autosaveBlocked() {
		t.Fatal("autosave should be blocked while unreadable data is quarantined")
	}
	datasetQuarantined, quarantinePending, hydrationSucceeded = false, true, true
	if !autosaveBlocked() {
		t.Fatal("autosave should be blocked while quarantine write is pending")
	}
	datasetQuarantined, quarantinePending, hydrationSucceeded = false, false, false
	if !autosaveBlocked() {
		t.Fatal("autosave should be blocked until hydration succeeds")
	}
}

func TestBootRestoresExistingQuarantineCopyForBannerAndDownload(t *testing.T) {
	const key = quarantinePrefix + "1720000000"
	const raw = `{"broken":true}`
	browserstore.Set(key, raw)
	t.Cleanup(func() { browserstore.Remove(key) })

	oldRaw, oldAvailable, oldQuarantined := quarantineRaw, quarantineAvailable, datasetQuarantined
	t.Cleanup(func() {
		quarantineRaw, quarantineAvailable, datasetQuarantined = oldRaw, oldAvailable, oldQuarantined
	})
	quarantineRaw, quarantineAvailable, datasetQuarantined = "", false, false

	if !loadQuarantineRecord() {
		t.Fatal("boot did not restore an existing quarantine record")
	}
	if !quarantineAvailable || quarantineRaw != raw {
		t.Fatalf("restored recovery state = available %v, raw %q", quarantineAvailable, quarantineRaw)
	}
	if got := string(quarantinedDownloadBytes()); got != raw {
		t.Fatalf("download bytes = %q, want %q", got, raw)
	}
}

func TestQuarantineUsesExistingCopyForRepeatedBytes(t *testing.T) {
	const key = quarantinePrefix + "1720000001"
	const raw = `{"broken":true}`
	browserstore.Set(key, raw)
	t.Cleanup(func() { browserstore.Remove(key) })

	got, ok := findQuarantineRecord([]byte(raw))
	if !ok || got != key {
		t.Fatalf("findQuarantineRecord = (%q, %v), want existing %q", got, ok, key)
	}
}
