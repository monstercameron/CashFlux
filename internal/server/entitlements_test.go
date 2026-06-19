package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsCloudActiveWhenBillingDisabled(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: false}, AuthUser{ID: "u1"})
	if err != nil || !active {
		t.Fatalf("IsCloudActive billing disabled = %v/%v, want true nil", active, err)
	}
}

func TestIsCloudActiveRequiresUser(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: false}, AuthUser{})
	if status.Code(err) != codes.Unauthenticated || active {
		t.Fatalf("IsCloudActive missing user = %v/%v, want unauthenticated false", active, err)
	}
}

func TestIsCloudActiveWhenBillingEnabledDefaultsInactive(t *testing.T) {
	active, err := IsCloudActive(context.Background(), Config{Billing: true}, AuthUser{ID: "u1"})
	if err != nil || active {
		t.Fatalf("IsCloudActive billing enabled = %v/%v, want false nil until subscriptions land", active, err)
	}
}
