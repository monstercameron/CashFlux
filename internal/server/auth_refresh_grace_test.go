// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRefreshTokenReuseGraceIsBoundAndRevokes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		age           time.Duration
		deviceID      string
		presentedHash bool
		wantGrace     bool
	}{
		{name: "bound within grace", age: 0, deviceID: "device-1", presentedHash: true, wantGrace: true},
		{name: "unbound replay", age: 0, presentedHash: true},
		{name: "after grace", age: 11 * time.Second, deviceID: "device-1", presentedHash: true},
		{name: "token hash mismatch", age: 0, deviceID: "device-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestAuthServer(t)
			registered, err := s.Register(context.Background(), backendrpc.RegisterRequest{Username: "refresh-user", Password: "correct-horse-battery"})
			if err != nil {
				t.Fatalf("register: %v", err)
			}
			ctx := context.Background()
			if tc.deviceID != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-cashflux-device-id", tc.deviceID))
			}
			first, err := s.RefreshToken(ctx, backendrpc.RefreshTokenRequest{RefreshToken: registered.RefreshToken})
			if err != nil {
				t.Fatalf("first refresh: %v", err)
			}
			claims, ok := verifySessionClaims(s.cfg, registered.RefreshToken, "refresh", time.Now().UTC())
			if !ok {
				t.Fatal("original refresh token did not verify")
			}
			if _, err := s.store.db.Exec(`UPDATE refresh_tokens SET used_at = ? WHERE jti = ?`, formatTime(time.Now().UTC().Add(-tc.age)), claims.JTI); err != nil {
				t.Fatalf("age consumed token: %v", err)
			}

			presented := registered.RefreshToken
			if !tc.presentedHash {
				claims.Exp++
				presented, err = issueSessionTokenWithClaims(s.cfg, claims)
				if err != nil {
					t.Fatalf("issue hash-mismatched token: %v", err)
				}
			}
			second, err := s.RefreshToken(ctx, backendrpc.RefreshTokenRequest{RefreshToken: presented})
			if tc.wantGrace {
				if err != nil {
					t.Fatalf("duplicate inside grace: %v", err)
				}
				if second.AccessToken == first.AccessToken || second.RefreshToken != first.RefreshToken {
					t.Fatalf("grace replay must mint fresh access and keep successor refresh: access_equal=%v refresh_equal=%v", second.AccessToken == first.AccessToken, second.RefreshToken == first.RefreshToken)
				}
				return
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("late reuse error = %v, want Unauthenticated", err)
			}
			if second.AccessToken != "" {
				t.Fatalf("late reuse returned a token pair: %+v", second)
			}
			if _, err := s.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: first.RefreshToken}); status.Code(err) != codes.Unauthenticated {
				t.Fatalf("late reuse did not revoke successor: %v", err)
			}
		})
	}
}
