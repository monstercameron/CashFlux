// SPDX-License-Identifier: MIT

// Package appstate — saved-scope-view persistence.
//
// Saved views are stored as a JSON map (id → JSON-of-SavedView) under the
// settings KV key cashflux:saved-scopes so they survive a dataset wipe (same
// lifecycle as user preferences).
package appstate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/scope"
)

const savedScopesKVKey = "cashflux:saved-scopes"

// loadSavedScopesMap reads the persisted id→JSON map from settings KV storage.
// It returns an empty (non-nil) map when no data has been written yet.
func (a *App) loadSavedScopesMap() map[string]string {
	raw, ok := a.GetSettingKV(savedScopesKVKey)
	if !ok || raw == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		a.log.Error("savedview: corrupt saved-scopes map; resetting", "err", err)
		return map[string]string{}
	}
	return m
}

// saveSavedScopesMap writes the id→JSON map back to settings KV storage.
func (a *App) saveSavedScopesMap(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return a.SetSettingKV(savedScopesKVKey, string(b))
}

// SavedViews returns all persisted saved views, sorted by name
// (case-insensitively). Corrupt entries are silently skipped.
func (a *App) SavedViews() []scope.SavedView {
	return scope.ListSavedViews(a.loadSavedScopesMap())
}

// PutSavedView inserts or replaces the given saved view in the persisted map.
// The view is keyed by v.ID; an existing entry with the same ID is overwritten.
func (a *App) PutSavedView(v scope.SavedView) error {
	m := a.loadSavedScopesMap()
	m = scope.PutSavedView(m, v)
	if err := a.saveSavedScopesMap(m); err != nil {
		return err
	}
	a.log.Info("saved view stored", "id", v.ID, "name", v.Name)
	return nil
}

// DeleteSavedView removes the saved view identified by id. It is a no-op when
// no view with that id exists.
func (a *App) DeleteSavedView(id string) error {
	m := a.loadSavedScopesMap()
	m = scope.DeleteSavedView(m, id)
	if err := a.saveSavedScopesMap(m); err != nil {
		return err
	}
	a.log.Info("saved view deleted", "id", id)
	return nil
}
