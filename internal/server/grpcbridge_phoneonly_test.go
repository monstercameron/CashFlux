// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestPhoneOnlyAuthServerDisablesRegisterAndLogin proves phoneOnlyAuthServer's
// Register/Login overrides actually shadow the real authServer methods when
// dispatched through the grpc.ServiceDesc (not just at the Go interface
// level) — this is the exact bypass an adversarial review found: Register
// creates a brand-new account with no Config.SetupCode check of its own, so
// NewSyncAndAuthBridgeHandler must never let a real call reach it, regardless
// of whether SetupCode is configured.
func TestPhoneOnlyAuthServerDisablesRegisterAndLogin(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "dev-token", AppOrigin: "*", SetupCode: "invite-only"}
	bridge := httptest.NewServer(NewSyncAndAuthBridgeHandler(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	var registerOut backendrpc.TokenPairResponse
	err = conn.Invoke(ctx, backendrpc.MethodAuthRegister, backendrpc.RegisterRequest{
		Username: "attacker", Password: "password1234",
	}, &registerOut, backendrpc.JSONCallOptions()...)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("Register: err = %v, want Unimplemented — this is the exact bypass that let anyone self-register an account with no setup code", err)
	}
	if registerOut.AccessToken != "" {
		t.Fatalf("Register returned a live access token despite Unimplemented: %+v", registerOut)
	}

	var loginOut backendrpc.TokenPairResponse
	err = conn.Invoke(ctx, backendrpc.MethodAuthLogin, backendrpc.LoginRequest{
		Username: "attacker", Password: "password1234",
	}, &loginOut, backendrpc.JSONCallOptions()...)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("Login: err = %v, want Unimplemented", err)
	}

	// SyncService and AuthService's phone-verification path must still work
	// normally in this same bridge — the fix disables exactly two methods,
	// not the whole embedding.
	var verifyOut backendrpc.RequestPhoneVerificationResponse
	err = conn.Invoke(ctx, backendrpc.MethodAuthRequestPhoneVerification, backendrpc.RequestPhoneVerificationRequest{
		PhoneNumber: "+15551230099", SetupCode: "invite-only",
	}, &verifyOut, backendrpc.JSONCallOptions()...)
	// No Twilio client is configured in this test, so this is expected to fail
	// on the SMS send, not on Unimplemented/PermissionDenied — proving the
	// setup-code gate itself was satisfied and the call reached the real
	// implementation.
	if status.Code(err) == codes.Unimplemented || status.Code(err) == codes.PermissionDenied {
		t.Fatalf("RequestPhoneVerification with a valid setup code: err = %v, want it to reach the real handler", err)
	}
}
