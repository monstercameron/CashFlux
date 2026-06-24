// SPDX-License-Identifier: MIT

// Package configlayer resolves a configuration value through the app's layered
// precedence: defaults → household → member → screen, most-specific wins (§1.19).
//
// Each layer is a string→string map of set keys. A later (more specific) layer
// overrides an earlier one for the same key; an absent or empty value in a layer
// falls through to the next-broader layer, and ultimately to the built-in default.
// The app's persisted household Settings is the "household" layer; member- and
// screen-scoped overrides plug in as the later layers when present.
package configlayer

// Layers holds the four precedence tiers, broadest first. Any tier may be nil.
type Layers struct {
	Defaults  map[string]string
	Household map[string]string
	Member    map[string]string
	Screen    map[string]string
}

// order returns the layers from broadest to most-specific.
func (l Layers) order() []map[string]string {
	return []map[string]string{l.Defaults, l.Household, l.Member, l.Screen}
}

// Resolve returns the effective value of key: the value from the most-specific
// layer that sets it to a non-empty string, or "" if no layer sets it.
func (l Layers) Resolve(key string) string {
	out := ""
	for _, layer := range l.order() {
		if layer == nil {
			continue
		}
		if v, ok := layer[key]; ok && v != "" {
			out = v // more-specific layers come later, so they win
		}
	}
	return out
}

// ResolveOr returns Resolve(key) or fallback when no layer sets the key.
func (l Layers) ResolveOr(key, fallback string) string {
	if v := l.Resolve(key); v != "" {
		return v
	}
	return fallback
}

// Source reports which tier supplied the effective value for key
// ("default"|"household"|"member"|"screen"), or "" when unset. Useful for an
// "inherited from …" affordance in a future settings UI.
func (l Layers) Source(key string) string {
	names := []string{"default", "household", "member", "screen"}
	src := ""
	for i, layer := range l.order() {
		if layer == nil {
			continue
		}
		if v, ok := layer[key]; ok && v != "" {
			src = names[i]
		}
	}
	return src
}
