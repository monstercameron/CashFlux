// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

// This bridge routes app/UI persistence through the SQLite-backed key-value stores
// (appstate → store.appkv / settingskv) so all data lives in the one SQLite dataset:
// it exports/imports as a unit and a wipe clears it (appkv) or preserves it
// (settingskv, for config). Reads transparently migrate any value still sitting in
// the browser store (IndexedDB, ex-localStorage) into SQLite on first access, so
// existing installs carry their state forward once. No localStorage is touched —
// the only non-SQLite store is browserstore (IndexedDB), used purely for migration.
//
// Intentional single-source exemptions
// ------------------------------------
// The dataset is the ONE shareable/hydratable state blob, so nearly everything routes
// through this bridge (or the store.Settings struct / Settings.Music checkpoint). A
// small, deliberate set of keys stays OUTSIDE the dataset in the browser store because
// they cannot or should not live inside the very thing they gate, secure, or index:
//
//   - Seed bootstrap gate — "cashflux:seeded". Read in app.hydrateDataset BEFORE the
//     dataset is imported to decide whether an absent/empty dataset means first-run
//     (seed the sample) or an intentional wipe (stay empty). It gates dataset creation,
//     so it can't be read from the dataset. (Contrast cashflux:sampleActive, which
//     merely DESCRIBES a loaded dataset and so lives in the dataset app KV.)
//   - Workspace registry — "cashflux:workspaces" and the "cashflux:ws-data:" /
//     "cashflux:sync-meta:" families. This is the INDEX of all datasets plus each
//     inactive workspace's bundled blob; a single dataset can't contain the registry
//     of every dataset (including itself).
//   - Lock gate config — "cashflux:applock". Must be readable before the (possibly
//     encrypted) dataset can be decrypted, to decide whether to show the passcode gate
//     at all. A lock config sealed inside the thing it locks is unreachable.
//   - Device-bound security material — "cashflux:webauthn-credid/salt/vault" (a passkey
//     bound to THIS authenticator), "cashflux:member-pins" (device access control), the
//     encrypted credential vault, and the per-install artifact salt ("cf.artifactSalt").
//     These are scoped to this device/authenticator and must not travel in a shared blob.
//   - Sync identity/transport — "cashflux:sync-device-id/status/queue". Per-device
//     transport bookkeeping, not user content; sharing it across clients is meaningless.
//
// Two more categories are NOT exemptions — the state DOES live in the dataset, with a
// browser-store copy kept only as a fast device-local cache:
//   - Music playback — the high-frequency live position streams to browserstore
//     ("cashflux:muzak-pos", …) but is folded into Settings.Music at coarse checkpoints
//     (app.checkpointMusic), so the durable state travels with the dataset.
//   - Artifact bytes — binary blobs live in the "cashflux-artifacts" IndexedDB store to
//     keep the JSON dataset under the storage quota, but they are referenced by the
//     dataset and round-trip through export/import.

// kvGet returns the persisted value for key (empty when absent), preferring SQLite
// and migrating a legacy browser-store value into it on first read.
func kvGet(key string) string {
	if app := appstate.Default; app != nil {
		if v, ok := app.GetKV(key); ok {
			return v
		}
		if val, ok := browserstore.Get(key); ok {
			_ = app.SetKV(key, val)
			browserstore.Remove(key)
			return val
		}
		return ""
	}
	return browserstore.GetString(key)
}

// kvSet persists key→val into SQLite. Before the store is ready (very early boot)
// it falls back to the browser store; the next read migrates it in.
func kvSet(key, val string) {
	if app := appstate.Default; app != nil {
		_ = app.SetKV(key, val)
		return
	}
	browserstore.Set(key, val)
}

func kvDelete(key string) {
	if app := appstate.Default; app != nil {
		_ = app.DeleteKV(key)
	}
	browserstore.Remove(key)
}

// KVGet/KVSet/KVDelete are the exported bridge for other wasm packages (app, screens).
func KVGet(key string) string { return kvGet(key) }
func KVSet(key, val string)   { kvSet(key, val) }
func KVDelete(key string)     { kvDelete(key) }

// SettingKVGet/SettingKVSet/SettingKVDelete persist config & preferences into the
// SQLite dataset's PRESERVED settings KV (theme, fonts, language, prefs, muzak, …):
// in SQLite like everything else, but survives a wipe.
func SettingKVGet(key string) string {
	if app := appstate.Default; app != nil {
		if v, ok := app.GetSettingKV(key); ok {
			return v
		}
		if val, ok := browserstore.Get(key); ok {
			_ = app.SetSettingKV(key, val)
			browserstore.Remove(key)
			return val
		}
		return ""
	}
	return browserstore.GetString(key)
}

func SettingKVSet(key, val string) {
	if app := appstate.Default; app != nil {
		_ = app.SetSettingKV(key, val)
		return
	}
	browserstore.Set(key, val)
}

func SettingKVDelete(key string) {
	if app := appstate.Default; app != nil {
		_ = app.DeleteSettingKV(key)
	}
	browserstore.Remove(key)
}
