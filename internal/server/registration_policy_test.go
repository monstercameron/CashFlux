// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegistrationPolicyDefaultsClosedAndAllowsExplicitOpen(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_REGISTRATION_OPEN", "")
	t.Setenv("CASHFLUX_SERVER_DATA_DIR", t.TempDir())
	closed, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv closed: %v", err)
	}
	if closed.RegistrationOpen {
		t.Fatal("registration must default closed")
	}

	t.Setenv("CASHFLUX_SERVER_REGISTRATION_OPEN", "true")
	open, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv open: %v", err)
	}
	if !open.RegistrationOpen {
		t.Fatal("explicit registration-open override was ignored")
	}
}

func TestFullServerAuthServiceHonorsRegistrationPolicy(t *testing.T) {
	store := openTestStore(t)
	req := backendrpc.RegisterRequest{
		Username:    "approval-test",
		Password:    "correct-horse-battery",
		DeviceLabel: "policy-test-device",
	}

	closed := authServiceForRegistrationPolicy(store, Config{})
	if _, err := closed.Register(context.Background(), req); status.Code(err) != codes.Unimplemented {
		t.Fatalf("closed Register error = %v, want Unimplemented", err)
	}

	open := authServiceForRegistrationPolicy(store, Config{RegistrationOpen: true, Token: "policy-test-token"})
	if _, err := open.Register(context.Background(), req); err != nil {
		t.Fatalf("open Register: %v", err)
	}
}

func TestVersionReportsRegistrationPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		open bool
	}{
		{name: "approval required", open: false},
		{name: "open signup", open: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewMux(Config{
				AuthMode:         "token",
				AppOrigin:        "http://127.0.0.1:8080",
				RegistrationOpen: tc.open,
			}, openTestStore(t))
			req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
			req.Header.Set("Origin", "http://127.0.0.1:8080")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
			}
			var got VersionResponse
			if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.RegistrationOpen != tc.open {
				t.Fatalf("RegistrationOpen = %v, want %v", got.RegistrationOpen, tc.open)
			}
		})
	}
}
