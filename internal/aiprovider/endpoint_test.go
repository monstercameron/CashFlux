// SPDX-License-Identifier: MIT

package aiprovider

import "testing"

func TestResolveBaseURL(t *testing.T) {
	const def = "https://api.openai.com/v1"
	cases := []struct {
		override, fallback, want string
	}{
		{"", def, def},                                              // no override -> default
		{"   ", def, def},                                           // blank override -> default
		{"http://localhost:11434/v1", def, "http://localhost:11434/v1"}, // override wins
		{"http://localhost:1234/v1/", def, "http://localhost:1234/v1"},  // trailing slash trimmed
		{"  https://proxy.example/v1  ", def, "https://proxy.example/v1"}, // trimmed
		{"http://x/v1", "", "http://x/v1"},                          // override wins over blank fallback
		{"", "", ""},                                                // both blank -> blank
	}
	for _, c := range cases {
		if got := ResolveBaseURL(c.override, c.fallback); got != c.want {
			t.Errorf("ResolveBaseURL(%q,%q) = %q, want %q", c.override, c.fallback, got, c.want)
		}
	}
}

func TestIsLocalEndpoint(t *testing.T) {
	local := []string{
		"http://localhost:11434/v1",
		"http://127.0.0.1:1234/v1",
		"http://[::1]:8080",
		"http://mymac.local/v1",
		"https://host.docker.internal:1234",
	}
	for _, u := range local {
		if !IsLocalEndpoint(u) {
			t.Errorf("IsLocalEndpoint(%q) = false, want true", u)
		}
	}
	remote := []string{
		"https://api.openai.com/v1",
		"https://proxy.example.com/v1",
		"https://openrouter.ai/api/v1",
	}
	for _, u := range remote {
		if IsLocalEndpoint(u) {
			t.Errorf("IsLocalEndpoint(%q) = true, want false", u)
		}
	}
}
