// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestAuthServer(t *testing.T) *authServer {
	t.Helper()
	store := openTestStore(t)
	return newAuthService(store, Config{SessionKey: "test-session-key-0123456789"})
}

func TestAuthServerRegister(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()

	resp, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("Register: expected a token pair, got %+v", resp)
	}
	if resp.RecoveryCode == "" {
		t.Fatal("Register: expected a one-time recovery code")
	}

	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "another-password"}); err == nil {
		t.Fatal("Register: expected an error for a duplicate username")
	} else if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("Register: expected AlreadyExists for duplicate username, got %v", err)
	}
}

func TestAuthServerRegisterRejectsShortPassword(t *testing.T) {
	s := newTestAuthServer(t)
	if _, err := s.Register(context.Background(), backendrpc.RegisterRequest{Username: "cam", Password: "short"}); err == nil {
		t.Fatal("Register: expected an error for a too-short password")
	} else if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Register: expected InvalidArgument, got %v", err)
	}
}

func TestAuthServerLogin(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	resp, err := s.Login(ctx, backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("Login: expected a token pair, got %+v", resp)
	}
	if resp.RecoveryCode != "" {
		t.Fatal("Login: should never return a recovery code")
	}

	if _, err := s.Login(ctx, backendrpc.LoginRequest{Username: "cam", Password: "wrong-password"}); err == nil {
		t.Fatal("Login: expected an error for the wrong password")
	} else if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Login: expected Unauthenticated for wrong password, got %v", err)
	}

	if _, err := s.Login(ctx, backendrpc.LoginRequest{Username: "nobody", Password: "whatever1"}); err == nil {
		t.Fatal("Login: expected an error for an unknown username")
	} else if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Login: expected Unauthenticated for unknown username, got %v", err)
	}
}

func TestAuthServerRedeemPairingCode(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	now := time.Now().UTC()

	regResp, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	userID := localUserID("cam")

	t.Run("happy path", func(t *testing.T) {
		code, _, err := s.store.MintPairingCode(userID, now)
		if err != nil {
			t.Fatalf("MintPairingCode: %v", err)
		}
		resp, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: code})
		if err != nil {
			t.Fatalf("RedeemPairingCode: %v", err)
		}
		if resp.AccessToken == "" || resp.RefreshToken == "" {
			t.Fatalf("RedeemPairingCode: expected a token pair, got %+v", resp)
		}
		if resp.RecoveryCode != "" {
			t.Fatal("RedeemPairingCode: should never return a recovery code")
		}
	})

	t.Run("already consumed", func(t *testing.T) {
		code, _, err := s.store.MintPairingCode(userID, now)
		if err != nil {
			t.Fatalf("MintPairingCode: %v", err)
		}
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: code}); err != nil {
			t.Fatalf("first redeem: %v", err)
		}
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: code}); err == nil {
			t.Fatal("RedeemPairingCode: expected an error redeeming an already-consumed code")
		} else if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("RedeemPairingCode: expected Unauthenticated for a consumed code, got %v", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		code, _, err := s.store.MintPairingCode(userID, now.Add(-2*PairingCodeTTL))
		if err != nil {
			t.Fatalf("MintPairingCode: %v", err)
		}
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: code}); err == nil {
			t.Fatal("RedeemPairingCode: expected an error for an expired code")
		} else if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("RedeemPairingCode: expected Unauthenticated for an expired code, got %v", err)
		}
	})

	t.Run("user no longer exists", func(t *testing.T) {
		code, _, err := s.store.MintPairingCode("local:ghost", now)
		if err != nil {
			t.Fatalf("MintPairingCode: %v", err)
		}
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: code}); err == nil {
			t.Fatal("RedeemPairingCode: expected an error for a deleted account")
		}
	})

	t.Run("unknown code", func(t *testing.T) {
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: "000000"}); err == nil {
			t.Fatal("RedeemPairingCode: expected an error for a code that was never minted")
		} else if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("RedeemPairingCode: expected Unauthenticated for an unknown code, got %v", err)
		}
	})

	_ = regResp
}

// TestAuthServerLoginIdempotentRetryReturnsSameTokenPair proves a client
// retry of Login (e.g. after a timeout where it can't know whether the first
// attempt landed) replays the SAME token pair rather than minting a second
// device session (TODOS.md C443).
func TestAuthServerLoginIdempotentRetryReturnsSameTokenPair(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	req := backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery", DeviceLabel: "retry-device", IdempotencyKey: "login-key-1"}
	first, err := s.Login(ctx, req)
	if err != nil {
		t.Fatalf("first Login: %v", err)
	}
	second, err := s.Login(ctx, req)
	if err != nil {
		t.Fatalf("retried Login: %v", err)
	}
	if second != first {
		t.Fatalf("retried token pair = %+v, want identical to first %+v", second, first)
	}
}

func TestAuthServerLoginIdempotencyKeyReusedForDifferentRequestFails(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	first := backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery", IdempotencyKey: "shared-key"}
	if _, err := s.Login(ctx, first); err != nil {
		t.Fatalf("first Login: %v", err)
	}
	second := backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery", DeviceLabel: "a-different-device", IdempotencyKey: "shared-key"}
	_, err := s.Login(ctx, second)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Login with reused key for a different request: err = %v, want InvalidArgument", err)
	}
}

// TestAuthServerRedeemPairingCodeIdempotentRetryReturnsSameTokenPair proves a
// retry of RedeemPairingCode with the same idempotency key replays the first
// attempt's token pair rather than failing "already used" on the code it
// itself just consumed (TODOS.md C443).
func TestAuthServerRedeemPairingCodeIdempotentRetryReturnsSameTokenPair(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	userID := localUserID("cam")
	code, _, err := s.store.MintPairingCode(userID, now)
	if err != nil {
		t.Fatalf("MintPairingCode: %v", err)
	}
	req := backendrpc.RedeemPairingCodeRequest{PairingCode: code, DeviceLabel: "retry-device", IdempotencyKey: "pair-key-1"}
	first, err := s.RedeemPairingCode(ctx, req)
	if err != nil {
		t.Fatalf("first RedeemPairingCode: %v", err)
	}
	second, err := s.RedeemPairingCode(ctx, req)
	if err != nil {
		t.Fatalf("retried RedeemPairingCode: %v", err)
	}
	if second != first {
		t.Fatalf("retried token pair = %+v, want identical to first %+v", second, first)
	}
}

// TestAuthServerRedeemPairingCodeRateLimited proves a device guessing pairing
// codes (a 6-digit, 5-minute-TTL secret with no other brute-force defense)
// gets throttled rather than allowed unlimited attempts.
func TestAuthServerRedeemPairingCodeRateLimited(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	for i := 0; i < pairingLimitPerMinute; i++ {
		if _, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: "000000", DeviceLabel: "guessing-device"}); err == nil {
			t.Fatalf("call %d: expected an error for an unknown code", i)
		} else if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("call %d: expected Unauthenticated below the limit, got %v", i, err)
		}
	}
	_, err := s.RedeemPairingCode(ctx, backendrpc.RedeemPairingCodeRequest{PairingCode: "000000", DeviceLabel: "guessing-device"})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("call over the limit: err = %v, want ResourceExhausted", err)
	}
}

// TestAuthServerLoginRateLimited proves repeated wrong-password attempts
// against one username get throttled rather than allowed unlimited guesses.
func TestAuthServerLoginRateLimited(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	for i := 0; i < loginLimitPerMinute; i++ {
		if _, err := s.Login(ctx, backendrpc.LoginRequest{Username: "cam", Password: "wrong-password"}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("call %d: err = %v, want Unauthenticated below the limit", i, err)
		}
	}
	_, err := s.Login(ctx, backendrpc.LoginRequest{Username: "cam", Password: "wrong-password"})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("call over the limit: err = %v, want ResourceExhausted", err)
	}
}

// TestAuthServerRegisterRateLimited proves repeated Register calls from one
// device get throttled rather than allowed to spam accounts unbounded.
func TestAuthServerRegisterRateLimited(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	for i := 0; i < registerLimitPerMinute; i++ {
		req := backendrpc.RegisterRequest{Username: "user" + strconv.Itoa(i), Password: "correct-horse-battery", DeviceLabel: "spammer-device"}
		if _, err := s.Register(ctx, req); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	req := backendrpc.RegisterRequest{Username: "user-over-limit", Password: "correct-horse-battery", DeviceLabel: "spammer-device"}
	_, err := s.Register(ctx, req)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("call over the limit: err = %v, want ResourceExhausted", err)
	}
}
