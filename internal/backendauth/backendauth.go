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
	// CustomAuthEnabled mirrors VersionResponse.CustomAuthEnabled: whether this
	// backend has AuthServiceServer registered (phone/SMS, username/password,
	// pairing-code sign-in) at all. False on a SyncService-only embedding.
	CustomAuthEnabled bool
	// BillingEnabled/PaymentProviders mirror the matching VersionResponse
	// fields — whether this backend has a billing/subscription concept, and
	// which payment providers are actually configured and ready to use.
	BillingEnabled   bool
	PaymentProviders []string
}

func (d Discovery) Normalize() Discovery {
	mode := strings.ToLower(strings.TrimSpace(d.AuthMode))
	switch mode {
	case ModeOAuth, ModeToken:
	default:
		mode = ModeToken
	}
	out := Discovery{AuthMode: mode, CustomAuthEnabled: d.CustomAuthEnabled, BillingEnabled: d.BillingEnabled}
	seen := map[string]bool{}
	for _, provider := range d.AuthProviders {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		out.AuthProviders = append(out.AuthProviders, provider)
	}
	seenPayment := map[string]bool{}
	for _, provider := range d.PaymentProviders {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || seenPayment[provider] {
			continue
		}
		seenPayment[provider] = true
		out.PaymentProviders = append(out.PaymentProviders, provider)
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
