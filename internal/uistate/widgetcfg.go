//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/GoWebComponents/state"
)

// WidgetConfigs maps a widget id to its saved settings (per widgetcfg.Schema).
// Persisted to localStorage so widget settings survive reloads, like the layout
// and filter atoms.
type WidgetConfigs map[string]widgetcfg.Config

const (
	widgetCfgAtomID  = "widgets:config"
	widgetCfgStoreID = "cashflux:widget-config"
)

// UseWidgetConfigs returns the shared widget-settings atom, seeded from
// localStorage. The flip panel writes it; widgets read their slice via For.
func UseWidgetConfigs() state.Atom[WidgetConfigs] {
	return state.UseAtom(widgetCfgAtomID, loadWidgetConfigs())
}

// PersistWidgetConfigs saves all widget settings to localStorage.
func PersistWidgetConfigs(c WidgetConfigs) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", widgetCfgStoreID, string(data))
}

// For returns one widget's config, never nil.
func (c WidgetConfigs) For(id string) widgetcfg.Config {
	if cfg, ok := c[id]; ok && cfg != nil {
		return cfg
	}
	return widgetcfg.Config{}
}

// WithField returns a deep copy of the configs with one widget's field set, so
// callers can Set the atom without mutating the shared map.
func (c WidgetConfigs) WithField(id, key, value string) WidgetConfigs {
	out := make(WidgetConfigs, len(c)+1)
	for wid, cfg := range c {
		nc := make(widgetcfg.Config, len(cfg))
		for k, v := range cfg {
			nc[k] = v
		}
		out[wid] = nc
	}
	if out[id] == nil {
		out[id] = widgetcfg.Config{}
	}
	out[id][key] = value
	return out
}

// loadWidgetConfigs reads saved widget settings from localStorage, defaulting to
// empty when absent or invalid.
func loadWidgetConfigs() WidgetConfigs {
	v := js.Global().Get("localStorage").Call("getItem", widgetCfgStoreID)
	if v.IsNull() || v.IsUndefined() {
		return WidgetConfigs{}
	}
	var c WidgetConfigs
	if err := json.Unmarshal([]byte(v.String()), &c); err != nil {
		return WidgetConfigs{}
	}
	return c
}
