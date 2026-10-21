// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"
)

// notifyRoutesKey is the app-KV key holding the notification→resource route config.
const notifyRoutesKey = "cashflux:notify:routes"

// NotifyRoutes returns the configured notification→resource route table: the persisted
// config if the user has set one, otherwise the seed defaults. Persisted as JSON in the
// SQLite-backed app KV, so it's editable data rather than a source literal.
func NotifyRoutes() []NotifyRoute {
	rs, err := ParseNotifyRoutes(kvGet(notifyRoutesKey))
	if err != nil {
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
