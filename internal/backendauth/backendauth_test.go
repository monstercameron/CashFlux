// SPDX-License-Identifier: MIT

package backendauth

import (
	"reflect"
	"testing"
)

func TestDiscoveryNormalize(t *testing.T) {
	got := (Discovery{
		AuthMode:      " OAuth ",
		AuthProviders: []string{"Google", "github", "google", " "},
	}).Normalize()
	want := Discovery{AuthMode: ModeOAuth, AuthProviders: []string{"google", "github"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Normalize() = %#v, want %#v", got, want)
	}
}

func TestDiscoveryDefaultsUnknownModeToToken(t *testing.T) {
	got := (Discovery{AuthMode: "magic", AuthProviders: []string{"google"}}).Normalize()
	if got.AuthMode != ModeToken {
		t.Fatalf("unknown mode normalized to %q, want %q", got.AuthMode, ModeToken)
	}
	if len(got.AuthProviders) != 1 {
		t.Fatalf("providers should still be discoverable, got %#v", got.AuthProviders)
	}
}

func TestOAuthProvidersOrFallback(t *testing.T) {
	if got := (Discovery{AuthMode: ModeToken, AuthProviders: []string{"google"}}).OAuthProvidersOrFallback([]string{"github"}); len(got) != 0 {
		t.Fatalf("token mode providers = %#v, want none", got)
	}
	if got := (Discovery{AuthMode: ModeOAuth}).OAuthProvidersOrFallback([]string{"Google", "github", "google"}); !reflect.DeepEqual(got, []string{"google", "github"}) {
		t.Fatalf("fallback providers = %#v", got)
	}
	if got := (Discovery{AuthMode: ModeOAuth, AuthProviders: []string{"github"}}).OAuthProvidersOrFallback([]string{"google"}); !reflect.DeepEqual(got, []string{"github"}) {
		t.Fatalf("advertised providers = %#v", got)
	}
}

// TestDiscoveryNormalizePreservesCapabilityFields proves CustomAuthEnabled,
// BillingEnabled, and PaymentProviders survive Normalize() untouched (they
// aren't part of the auth-mode/provider dedupe logic Normalize exists for).
func TestDiscoveryNormalizePreservesCapabilityFields(t *testing.T) {
	got := (Discovery{
		AuthMode:          ModeToken,
		CustomAuthEnabled: true,
		BillingEnabled:    true,
		PaymentProviders:  []string{"Stripe", "paypal", "stripe"},
	}).Normalize()
	if !got.CustomAuthEnabled {
		t.Fatalf("CustomAuthEnabled = false, want true")
	}
	if !got.BillingEnabled {
		t.Fatalf("BillingEnabled = false, want true")
	}
	want := []string{"stripe", "paypal"}
	if !reflect.DeepEqual(got.PaymentProviders, want) {
		t.Fatalf("PaymentProviders = %#v, want %#v", got.PaymentProviders, want)
	}
}

func TestDiscoveryNormalizeDefaultsCapabilityFieldsFalse(t *testing.T) {
	got := (Discovery{AuthMode: ModeToken}).Normalize()
	if got.CustomAuthEnabled {
		t.Fatalf("CustomAuthEnabled = true, want false (zero value)")
	}
	if got.BillingEnabled {
		t.Fatalf("BillingEnabled = true, want false (zero value)")
	}
	if len(got.PaymentProviders) != 0 {
		t.Fatalf("PaymentProviders = %#v, want empty", got.PaymentProviders)
	}
}
