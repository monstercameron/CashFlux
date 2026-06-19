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
