// SPDX-License-Identifier: MIT

package configlayer

import "testing"

func TestResolvePrecedence(t *testing.T) {
	l := Layers{
		Defaults:  map[string]string{"dateStyle": "iso", "weekStart": "sunday"},
		Household: map[string]string{"weekStart": "monday", "currency": "USD"},
		Member:    map[string]string{"currency": "EUR"},
		Screen:    map[string]string{"dateStyle": "us"},
	}
	cases := map[string]string{
		"dateStyle": "us",     // screen overrides default
		"weekStart": "monday", // household overrides default; no member/screen
		"currency":  "EUR",    // member overrides household
		"missing":   "",       // unset everywhere
	}
	for k, want := range cases {
		if got := l.Resolve(k); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestResolveEmptyFallsThrough(t *testing.T) {
	l := Layers{
		Household: map[string]string{"accent": "#54b884"},
		Member:    map[string]string{"accent": ""}, // empty must NOT shadow household
	}
	if got := l.Resolve("accent"); got != "#54b884" {
		t.Errorf("empty member value shadowed household: got %q", got)
	}
}

func TestResolveOrAndSource(t *testing.T) {
	l := Layers{
		Defaults:  map[string]string{"theme": "dark"},
		Member:    map[string]string{"theme": "light"},
	}
	if got := l.ResolveOr("theme", "x"); got != "light" {
		t.Errorf("ResolveOr set key = %q, want light", got)
	}
	if got := l.ResolveOr("nope", "fallback"); got != "fallback" {
		t.Errorf("ResolveOr unset = %q, want fallback", got)
	}
	if got := l.Source("theme"); got != "member" {
		t.Errorf("Source(theme) = %q, want member", got)
	}
	if got := l.Source("nope"); got != "" {
		t.Errorf("Source(unset) = %q, want empty", got)
	}
}

func TestNilLayersSafe(t *testing.T) {
	var l Layers // all nil
	if got := l.Resolve("anything"); got != "" {
		t.Errorf("nil layers Resolve = %q, want empty", got)
	}
	if got := l.ResolveOr("anything", "d"); got != "d" {
		t.Errorf("nil layers ResolveOr = %q, want d", got)
	}
}
