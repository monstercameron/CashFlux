// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"
)

// notifyRoutesKey is the app-KV key holding the notification→resource route config.
const notifyRoutesKey = "cashflux:notify:routes"

// NotifyRoute is one entry of the notification link config: a notification ID prefix (the
// rule that produced it, e.g. "default-bill-due") and the in-app path it links to.
type NotifyRoute struct {
	Prefix string `json:"prefix"`
	Route  string `json:"route"`
}

// defaultNotifyRoutes is the seed config used until the user overrides it. It is DATA
// (persisted + overridable at runtime via SetNotifyRoutes), not compiled-in behaviour —
// changing where a notification links needs no code change, just a new stored config.
func defaultNotifyRoutes() []NotifyRoute {
	return []NotifyRoute{
		{"default-bill-due", "/bills"},
		{"default-budget", "/budgets"},
		{"default-stale", "/accounts"},
		{"default-low", "/accounts"},
		{"default-goal", "/goals"},
		{"default-large", "/transactions"},
		{"default-unusual", "/transactions"},
		{"default-paycheck", "/transactions"},
		{"default-digest", "/reports"},
	}
}

// NotifyRoutes returns the configured notification→resource route table: the persisted
// config if the user has set one, otherwise the seed defaults. Persisted as JSON in the
// SQLite-backed app KV, so it's editable data rather than a source literal.
func NotifyRoutes() []NotifyRoute {
	raw := kvGet(notifyRoutesKey)
	if raw == "" {
		return defaultNotifyRoutes()
	}
	var rs []NotifyRoute
	if err := json.Unmarshal([]byte(raw), &rs); err != nil || len(rs) == 0 {
		return defaultNotifyRoutes()
	}
	return rs
}

// SetNotifyRoutes persists a custom notification route config (pass nil/empty to fall
// back to the defaults on the next read).
func SetNotifyRoutes(rs []NotifyRoute) {
	if len(rs) == 0 {
		kvSet(notifyRoutesKey, "")
		return
	}
	if data, err := json.Marshal(rs); err == nil {
		kvSet(notifyRoutesKey, string(data))
	}
}

// RouteForNotifyID resolves a notification's link from the configured route table by
// matching the item's ID prefix. Returns "" (not clickable) when nothing matches.
func RouteForNotifyID(id string) string {
	for _, r := range NotifyRoutes() {
		if r.Route != "" && strings.HasPrefix(id, r.Prefix) {
			return r.Route
		}
	}
	return ""
}
