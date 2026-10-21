// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/store"
	"maps"
)

// settingsSnapshotCache holds a successful revision's persisted settings.
// Callers own every mutable field returned from this cache.
type settingsSnapshotCache struct {
	rev    uint64
	value  store.Settings
	loaded bool
}

// SettingsChecked returns current settings or the storage error. Readiness is
// checked even on cache hits so a closed store cannot look like a known value.
// A failed load is not cached and can be retried at the same revision.
func (a *App) SettingsChecked() (store.Settings, error) {
	if a == nil || a.store == nil {
		return store.Settings{}, fmt.Errorf("read settings: local dataset is unavailable")
	}
	if err := a.store.CheckReady(); err != nil {
		return store.Settings{}, fmt.Errorf("read settings: %w", err)
	}
	return a.settings.get(a.store.Rev(), a.store.GetSettings)
}

func (c *settingsSnapshotCache) get(rev uint64, load func() (store.Settings, error)) (store.Settings, error) {
	if !c.loaded || c.rev != rev {
		value, err := load()
		if err != nil {
			return store.Settings{}, fmt.Errorf("read settings: %w", err)
		}
		*c = settingsSnapshotCache{rev: rev, value: cloneSettingsSnapshot(value), loaded: true}
	}
	return cloneSettingsSnapshot(c.value), nil
}

func cloneSettingsSnapshot(in store.Settings) store.Settings {
	out := in
	out.FXRates = maps.Clone(in.FXRates)
	out.FXUpdatedAt = maps.Clone(in.FXUpdatedAt)
	out.FreshnessOverrides = maps.Clone(in.FreshnessOverrides)
	if in.PayoffBaseline != nil {
		value := *in.PayoffBaseline
		out.PayoffBaseline = &value
	}
	if in.Music != nil {
		value := *in.Music
		out.Music = &value
	}
	if in.AISpend != nil {
		out.AISpend = append(in.AISpend[:0:0], in.AISpend...)
	}
	return out
}
