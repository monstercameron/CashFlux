// SPDX-License-Identifier: MIT

// Package modules models which app screens (modules) are visible. Users can hide
// screens they don't use, but a few core screens are locked and can never be
// hidden. The logic is pure Go so it unit-tests on native Go; the wasm layer adds
// localStorage persistence and the settings toggles.
package modules

// locked lists route paths that must always stay visible: the dashboard (home).
// Settings is no longer a routed screen — it lives in the household-card panel.
var locked = map[string]bool{
	"/": true,
}

// IsLocked reports whether a path must always be visible.
func IsLocked(path string) bool { return locked[path] }

// Hidden is the set of hidden module paths (value true means hidden).
type Hidden map[string]bool

// IsHidden reports whether a path is currently hidden. Locked paths are never
// hidden, regardless of the stored set.
func (h Hidden) IsHidden(path string) bool {
	if IsLocked(path) {
		return false
	}
	return h[path]
}

// Toggle returns a new set with the path's visibility flipped. Toggling a locked
// path is a no-op. The result only ever contains hidden (true) entries, so it
// serializes compactly.
func (h Hidden) Toggle(path string) Hidden {
	out := h.clone()
	if IsLocked(path) {
		return out
	}
	if out[path] {
		delete(out, path)
	} else {
		out[path] = true
	}
	return out
}

// clone copies the set, dropping any false entries so the map stays minimal.
func (h Hidden) clone() Hidden {
	out := Hidden{}
	for k, v := range h {
		if v {
			out[k] = true
		}
	}
	return out
}

// Normalize drops false and locked entries, returning a clean set. Use it when
// loading possibly-stale persisted data.
func (h Hidden) Normalize() Hidden {
	out := Hidden{}
	for k, v := range h {
		if v && !IsLocked(k) {
			out[k] = true
		}
	}
	return out
}
