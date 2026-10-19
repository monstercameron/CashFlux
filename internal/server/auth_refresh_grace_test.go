// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRefreshTokenReuseGraceReturnsSamePairThenRevokes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		age       time.Duration
		wantGrace bool
	}{
		{name: "within grace", age: 0, wantGrace: true},
		{name: "after grace", age: 31 * time.Second, wantGrace: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestAuthServer(t)
			registered, err := s.Register(context.Background(), backendrpc.RegisterRequest{Username: "refresh-user", Password: "correct-horse-battery"})
			if err != nil {
				t.Fatalf("register: %v", err)
			}
			first, err := s.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: registered.RefreshToken})
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

			second, err := s.RefreshToken(context.Background(), backendrpc.RefreshTokenRequest{RefreshToken: registered.RefreshToken})
			if tc.wantGrace {
				if err != nil {
					t.Fatalf("duplicate inside grace: %v", err)
				}
				if second.AccessToken != first.AccessToken || second.RefreshToken != first.RefreshToken {
					t.Fatalf("grace retry changed successor pair: access_equal=%v refresh_equal=%v access_lengths=%d/%d refresh_lengths=%d/%d", second.AccessToken == first.AccessToken, second.RefreshToken == first.RefreshToken, len(second.AccessToken), len(first.AccessToken), len(second.RefreshToken), len(first.RefreshToken))
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
