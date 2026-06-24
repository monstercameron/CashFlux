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
