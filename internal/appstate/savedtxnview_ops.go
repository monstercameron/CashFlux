// SPDX-License-Identifier: MIT

// Package appstate — saved transaction-view (TX3) persistence and CRUD.
//
// Saved transaction views are stored as a JSON map (id → JSON-of-SavedTxnView)
// under the settings KV key cashflux:saved-txn-views, so they survive a dataset
// wipe (the same lifecycle as user preferences and scope saved-views). This file
// is the App-level CRUD seam over the pure savedtxnview package: it enforces the
// "name required + unique" invariant and owns id/timestamp assignment.
package appstate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/savedtxnview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

const savedTxnViewsKVKey = "cashflux:saved-txn-views"

// loadSavedTxnViewsMap reads the persisted id→JSON map from settings KV storage.
// It returns an empty (non-nil) map when nothing has been written yet.
func (a *App) loadSavedTxnViewsMap() map[string]string {
	raw, ok := a.GetSettingKV(savedTxnViewsKVKey)
	if !ok || raw == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		a.log.Error("savedtxnview: corrupt map; resetting", "err", err)
		return map[string]string{}
	}
	return m
}

// saveSavedTxnViewsMap writes the id→JSON map back to settings KV storage.
func (a *App) saveSavedTxnViewsMap(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return a.SetSettingKV(savedTxnViewsKVKey, string(b))
}

// SavedTxnViews returns every persisted saved transaction view, ordered by name.
func (a *App) SavedTxnViews() []savedtxnview.SavedTxnView {
	return savedtxnview.List(a.loadSavedTxnViewsMap())
}

// SaveTxnView creates a new saved view from the current filter criteria under the
// given name and optional threshold, assigning a fresh id and creation time. It
// validates the name (required, unique) before persisting and returns the stored
// view. A blank or duplicate name is rejected.
func (a *App) SaveTxnView(name string, criteria txnfilter.Criteria, threshold int64) (savedtxnview.SavedTxnView, error) {
	v := savedtxnview.SavedTxnView{
		ID:        id.New(),
		Name:      name,
		Criteria:  criteria,
		Threshold: threshold,
		CreatedAt: a.clock(),
	}
	if err := v.Validate(); err != nil {
		return savedtxnview.SavedTxnView{}, err
	}
	if savedtxnview.NameTaken(a.SavedTxnViews(), name, "") {
		return savedtxnview.SavedTxnView{}, savedtxnview.ErrNameTaken
	}
	m := savedtxnview.Put(a.loadSavedTxnViewsMap(), v)
	if err := a.saveSavedTxnViewsMap(m); err != nil {
		return savedtxnview.SavedTxnView{}, err
	}
	a.log.Info("saved txn view stored", "id", v.ID, "name", v.Name)
	return v, nil
}

// UpdateTxnView replaces an existing saved view by v.ID (keeping its identity),
// re-validating name and uniqueness (excluding itself). Used to rename a view or
// change its threshold. A view whose ID is not already stored is rejected so an
// update never silently creates a record.
func (a *App) UpdateTxnView(v savedtxnview.SavedTxnView) error {
	if err := v.Validate(); err != nil {
		return err
	}
	m := a.loadSavedTxnViewsMap()
	if _, ok := m[v.ID]; !ok {
		return savedtxnview.ErrNotFound
	}
	if savedtxnview.NameTaken(savedtxnview.List(m), v.Name, v.ID) {
		return savedtxnview.ErrNameTaken
	}
	m = savedtxnview.Put(m, v)
	if err := a.saveSavedTxnViewsMap(m); err != nil {
		return err
	}
	a.log.Info("saved txn view updated", "id", v.ID, "name", v.Name)
	return nil
}

// DeleteTxnView removes the saved view identified by id (a no-op if absent).
func (a *App) DeleteTxnView(viewID string) error {
	m := savedtxnview.Delete(a.loadSavedTxnViewsMap(), viewID)
	if err := a.saveSavedTxnViewsMap(m); err != nil {
		return err
	}
	a.log.Info("saved txn view deleted", "id", viewID)
	return nil
}

// SavedTxnView returns the stored view with the given id, if present.
func (a *App) SavedTxnView(viewID string) (savedtxnview.SavedTxnView, bool) {
	for _, v := range a.SavedTxnViews() {
		if v.ID == viewID {
			return v, true
		}
	}
	return savedtxnview.SavedTxnView{}, false
}
