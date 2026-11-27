// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

// notifyRoutesKey is the app-KV key holding the notification→resource route config.
const notifyRoutesKey = "cashflux:notify:routes"

var notifyRoutesCache struct {
	app    *appstate.App
	rev    uint64
	raw    string
	loaded bool
	routes []NotifyRoute
}

// NotifyRoutes returns the configured notification→resource route table: the persisted
// config if the user has set one, otherwise the seed defaults. Persisted as JSON in the
// SQLite-backed app KV, so it's editable data rather than a source literal.
func NotifyRoutes() []NotifyRoute {
	a := appstate.Default
	if a != nil {
		rev := a.Rev()
		if notifyRoutesCache.loaded && notifyRoutesCache.app == a && notifyRoutesCache.rev == rev {
			return notifyRoutesCache.routes
		}
		raw := kvGet(notifyRoutesKey)
		rs, err := ParseNotifyRoutes(raw)
		if err != nil {
			rs = defaultNotifyRoutes()
		}
		notifyRoutesCache.app = a
		notifyRoutesCache.rev = rev
		notifyRoutesCache.raw = raw
		notifyRoutesCache.routes = rs
		notifyRoutesCache.loaded = true
		return rs
	}
	raw := kvGet(notifyRoutesKey)
	if notifyRoutesCache.loaded && notifyRoutesCache.app == nil && notifyRoutesCache.raw == raw {
		return notifyRoutesCache.routes
	}
	rs, err := ParseNotifyRoutes(raw)
	if err != nil {
		rs = defaultNotifyRoutes()
	}
	notifyRoutesCache.app = nil
	notifyRoutesCache.rev = 0
	notifyRoutesCache.raw = raw
	notifyRoutesCache.routes = rs
	notifyRoutesCache.loaded = true
	return rs
}

// SetNotifyRoutes persists a custom notification route config (pass nil/empty to fall
// back to the defaults on the next read).
func SetNotifyRoutes(rs []NotifyRoute) {
	notifyRoutesCache.loaded = false
	if len(rs) == 0 {
		kvSet(notifyRoutesKey, "")
		return
	}
	if data, err := json.Marshal(rs); err == nil {
		kvSet(notifyRoutesKey, string(data))
	}
}

// RouteForNotifyIDFrom resolves a notification's link against a preloaded route table.
func RouteForNotifyIDFrom(routes []NotifyRoute, id string) string {
	for _, r := range routes {
		if r.Route != "" && strings.HasPrefix(id, r.Prefix) {
			return r.Route
		}
	}
	return ""
}

// RouteForNotifyID resolves a notification's link from the configured route table by
// matching the item's ID prefix. Returns "" (not clickable) when nothing matches.
func RouteForNotifyID(id string) string {
	return RouteForNotifyIDFrom(NotifyRoutes(), id)
}
