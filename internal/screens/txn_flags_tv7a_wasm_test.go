// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestTV7CompatibilityFlagsKeyAllSettingsAndWorkspace(t *testing.T) {
	app := solp3FlagsApp(t, 2)
	settings := smart.Settings{ExplicitOff: map[string]bool{spikeFlagCode: true}}
	first := txnRowFlagsMemo(app, 0, settings, time.Monday)
	same := txnRowFlagsMemo(app, 0, settings, time.Monday)
	mapID := func(flags txnRowFlags) uintptr { return reflect.ValueOf(flags.Duplicate).Pointer() }
	if mapID(first) == 0 || mapID(first) != mapID(same) {
		t.Fatal("identical complete-ledger inputs did not reuse flags")
	}
	settings.Dismissed = map[string]bool{"dismissed-insight": true}
	changed := txnRowFlagsMemo(app, 0, settings, time.Monday)
	if mapID(first) == mapID(changed) {
		t.Error("changed detector settings reused the old flag cache")
	}
	raw, had := browserstore.Get("cashflux:workspaces")
	t.Cleanup(func() {
		if had {
			browserstore.Set("cashflux:workspaces", raw)
		} else {
			browserstore.Remove("cashflux:workspaces")
		}
	})
	registry, _ := json.Marshal(map[string]string{"activeId": "tv7a-another-workspace"})
	browserstore.Set("cashflux:workspaces", string(registry))
	other := txnRowFlagsMemo(app, 0, settings, time.Monday)
	if mapID(changed) == mapID(other) {
		t.Error("workspace change reused another workspace's flags")
	}
}

func TestTV7SlicedFlagsUseWholeLedgerAcrossSearchAndClear(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	duplicate := rows[1]
	duplicate.ID = "second-market"
	if err := app.Store().PutTransaction(duplicate); err != nil {
		t.Fatal(err)
	}
	settings := smart.Settings{ExplicitOff: map[string]bool{spikeFlagCode: true}}
	for _, query := range []string{"coffee", ""} {
		filter := uistate.CurrentTxFilter()
		filter.Text = query
		uistate.PersistTxFilter(filter)
		key := makeTxnFlagsKey(app, 0, settings, time.Monday)
		var result txnRowFlags
		published := false
		cancel := scheduleTxnFlags(key, settings, func() bool { return true }, func(flags txnRowFlags) {
			result, published = flags, true
		})
		solp3Wait(t, func() bool { return published })
		cancel()
		if !result.Duplicate[rows[1].ID] || !result.Duplicate[duplicate.ID] || len(result.Duplicate) != 2 {
			t.Fatalf("query %q shrank whole-ledger flags: %+v", query, result)
		}
	}
}
