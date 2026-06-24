// SPDX-License-Identifier: MIT

package backendauth

import "strings"

const (
	ModeToken = "token"
	ModeOAuth = "oauth"
)

type Discovery struct {
	AuthMode      string
	AuthProviders []string
}

func (d Discovery) Normalize() Discovery {
	mode := strings.ToLower(strings.TrimSpace(d.AuthMode))
	switch mode {
	case ModeOAuth, ModeToken:
	default:
		mode = ModeToken
	}
	out := Discovery{AuthMode: mode}
	seen := map[string]bool{}
	for _, provider := range d.AuthProviders {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		out.AuthProviders = append(out.AuthProviders, provider)
	}
	return out
}

func (d Discovery) UsesToken() bool {
	return d.Normalize().AuthMode == ModeToken
}

func (d Discovery) OAuthProvidersOrFallback(fallback []string) []string {
	d = d.Normalize()
	if d.AuthMode != ModeOAuth {
		return nil
	}
	if len(d.AuthProviders) > 0 {
		return append([]string(nil), d.AuthProviders...)
	}
	out := make([]string, 0, len(fallback))
	seen := map[string]bool{}
	for _, provider := range fallback {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		out = append(out, provider)
	}
	return out
}
