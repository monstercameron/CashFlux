// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

const (
	widgetCfgAtomID  = "widgets:config"
	widgetCfgStoreID = "cashflux:widget-config"
)

// UseWidgetConfigs returns the shared widget-settings atom, seeded from
// localStorage. The flip panel writes it; widgets read their slice via For.
func UseWidgetConfigs() state.Atom[WidgetConfigs] {
	return useWidgetConfigsWithLoader(loadWidgetConfigs)
}

// Each mounted reader loads one seed; later renders use the live fixed atom.
// A supported workspace reload creates a fresh runtime and initializer.
func useWidgetConfigsWithLoader(load func() WidgetConfigs) state.Atom[WidgetConfigs] {
	initial := ui.UseMemo(load, appstate.Default)
	return state.UseAtom(widgetCfgAtomID, initial)
}

// PersistWidgetConfigs saves all widget settings to localStorage.
func PersistWidgetConfigs(c WidgetConfigs) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	kvSet(widgetCfgStoreID, string(data))
}

// loadWidgetConfigs reads saved widget settings from localStorage, defaulting to
// empty when absent or invalid.
func loadWidgetConfigs() WidgetConfigs {
	raw := kvGet(widgetCfgStoreID)
	if strings.TrimSpace(raw) == "null" {
		return nil
	}
	c, err := ParseWidgetConfigs(raw)
	if err != nil {
		return WidgetConfigs{}
	}
	return c
}
