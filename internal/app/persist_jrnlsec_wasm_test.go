// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestJRNLSECEraseDataErasesRecoveryCopiesAndMemory(t *testing.T) {
	f := jrnlFixture(t)
	const quarantineKey = "cashflux:quarantine:1792418369"
	const journalKey = journalRecoveryPrefix + "erase-everything"
	copy := snapshotRecoveryCopy(jrnlRead(t))
	solp22Batch(t, map[string]string{quarantineKey: copy, journalKey: copy}, nil)
	oldQuarantined, oldPending, oldRaw, oldAvailable := datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable
	oldBannerHidden := quarantineBannerHidden
	oldSample := uistate.SampleActive()
	t.Cleanup(func() {
		datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = oldQuarantined, oldPending, oldRaw, oldAvailable
		quarantineBannerHidden = oldBannerHidden
		uistate.SetSampleActive(oldSample)
	})
	datasetQuarantined, quarantinePending, quarantineRaw, quarantineAvailable = true, true, copy, true
	quarantineBannerHidden = false
	if err := f.app.Wipe(); err != nil {
		t.Fatal(err)
	}
	if err := enc31Wait(t, func(done func(error)) {
		wipeFinancialLocalState(func() { done(nil) }, wipeEraseEverything)
	}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{quarantineKey, journalKey} {
		got, _, err := browserstore.GetSecure(key)
		if err != nil || got != "" {
			t.Fatalf("erase data retained recovery data at %s: err=%v", key, err)
		}
	}
	if datasetQuarantined || quarantinePending || quarantineRaw != "" || quarantineAvailable {
		t.Fatal("erase data left in-memory recovery state available")
	}

	// A later sample boot must not recover the erased household from either key.
	if err := f.app.LoadSample(); err != nil {
		t.Fatal(err)
	}
	uistate.SetSampleActive(true)
	if err := enc31Wait(t, func(done func(error)) {
		wipeFinancialLocalState(func() { done(nil) })
	}); err != nil {
		t.Fatal(err)
	}
	// This is the durable sample state a reload hydrates before attempting the
	// normal sample-only quarantine recovery path.
	loaded := jrnlApp(t)
	appstate.Default = loaded
	hydrateDataset()
	if datasetQuarantined || quarantineAvailable || quarantineRaw != "" {
		t.Fatalf("reload restored an erased recovery copy while sample data was showing: quarantined=%v available=%v rawBytes=%d", datasetQuarantined, quarantineAvailable, len(quarantineRaw))
	}
	if len(loaded.Transactions()) == 0 {
		t.Fatal("simulated reload did not keep the sample dataset")
	}
	for _, row := range loaded.Transactions() {
		if row.ID == f.row.ID && row.Desc == f.row.Desc {
			t.Fatal("simulated reload restored the erased household transaction")
		}
	}
}

func TestJRNLSECSampleWipesKeepRecoveryCopies(t *testing.T) {
	for _, action := range []string{"load sample", "clear sample"} {
		t.Run(action, func(t *testing.T) {
			f := jrnlFixture(t)
			const quarantineKey = "cashflux:quarantine:1792418369"
			const journalKey = journalRecoveryPrefix + "sample-keeps"
			quarantine, journal := "original unreadable bytes", snapshotRecoveryCopy(jrnlRead(t))
			solp22Batch(t, map[string]string{quarantineKey: quarantine, journalKey: journal}, nil)
			if action == "load sample" {
				if err := f.app.LoadSample(); err != nil {
					t.Fatal(err)
				}
			} else if err := f.app.Wipe(); err != nil {
				t.Fatal(err)
			}
			if err := enc31Wait(t, func(done func(error)) {
				wipeFinancialLocalState(func() { done(nil) })
			}); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{quarantineKey: quarantine, journalKey: journal} {
				got, have, err := browserstore.GetSecure(key)
				if err != nil || !have || got != want {
					t.Fatalf("%s did not preserve %s: have=%v err=%v", action, key, have, err)
				}
			}
		})
	}
}
