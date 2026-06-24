// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"strconv"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// Background-music persistence bridge. The live position lives in localStorage
// (fast, per-device); this layer mirrors the music state into the dataset at
// coarse checkpoints so it travels with export/import + backups and resumes on a
// fresh device — without re-serializing the whole dataset on every position tick.

const (
	muzakEnabledKey = "cashflux:muzak"
	muzakVolKey     = "cashflux:muzak-volume"
	muzakPosKey     = "cashflux:muzak-pos"
)

func muzakLSGet(k string) (string, bool) { return browserstore.Get(k) }

func muzakLSPut(k, v string) { browserstore.Set(k, v) }

// seedMusicFromDataset copies the dataset's saved music state into localStorage
// when this device has none yet (a fresh load or just-imported workspace), so the
// player resumes from the durable checkpoint.
func seedMusicFromDataset() {
	app := appstate.Default
	if app == nil {
		return
	}
	if _, ok := muzakLSGet(muzakPosKey); ok {
		return // local state already present — it's the freshest
	}
	m, ok := app.MusicState()
	if !ok {
		return
	}
	if m.Enabled {
		muzakLSPut(muzakEnabledKey, "1")
	} else {
		muzakLSPut(muzakEnabledKey, "0")
	}
	muzakLSPut(muzakVolKey, strconv.FormatFloat(m.Volume, 'f', 3, 64))
	if pos, err := json.Marshal(struct {
		I int     `json:"i"`
		T float64 `json:"t"`
	}{m.Index, m.Position}); err == nil {
		muzakLSPut(muzakPosKey, string(pos))
	}
}

// checkpointMusic reads the live localStorage music state and writes it into the
// dataset. Called from the JS player at coarse moments (via the bridge) and from
// the Go toggle / volume controls.
func checkpointMusic() {
	app := appstate.Default
	if app == nil {
		return
	}
	m := store.MusicState{Enabled: true, Volume: uistate.DefaultMuzakVolume}
	if v, ok := muzakLSGet(muzakEnabledKey); ok {
		m.Enabled = v != "0"
	}
	if v, ok := muzakLSGet(muzakVolKey); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Volume = f
		}
	}
	if v, ok := muzakLSGet(muzakPosKey); ok {
		var p struct {
			I int     `json:"i"`
			T float64 `json:"t"`
		}
		if json.Unmarshal([]byte(v), &p) == nil {
			m.Index = p.I
			m.Position = p.T
		}
	}
	_ = app.PutMusicState(m)
}

// registerMusicBridge exposes window.cashfluxMusicSave so the JS player can ask Go
// to checkpoint the music state into the dataset.
func registerMusicBridge() {
	js.Global().Set("cashfluxMusicSave", js.FuncOf(func(js.Value, []js.Value) any {
		checkpointMusic()
		return nil
	}))
}
