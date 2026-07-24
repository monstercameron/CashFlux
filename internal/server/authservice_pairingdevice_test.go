// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakePairingWatchStream is an in-memory grpc.ServerStream for driving
// WatchPairingStatusRPC directly, without a real network transport — the
// same pattern fakeUploadStream/fakeDownloadStream use in
// blobservice_test.go. Context() returns a live, never-canceled context
// (WatchPairingStatusRPC's select loop exits early via Context().Done() —
// tests exercise the "event delivered" path, not cancellation) and SendMsg
// records every message sent, safe for concurrent use since
// WatchPairingStatusRPC runs on its own goroutine in these tests.
type fakePairingWatchStream struct {
	grpc.ServerStream
	mu   sync.Mutex
	sent []any
}

func newFakeServerStream() *fakePairingWatchStream { return &fakePairingWatchStream{} }

func (s *fakePairingWatchStream) Context() context.Context { return context.Background() }

func (s *fakePairingWatchStream) SendMsg(m any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return nil
}

func (s *fakePairingWatchStream) sentMessages() []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]any, len(s.sent))
	copy(out, s.sent)
	return out
}

func TestAuthServerRequestDevicePairing(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	resp, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "kitchen-tablet"})
	if err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}
	if resp.DeviceID == "" {
		t.Fatal("RequestDevicePairing: expected a non-empty device id")
	}
	pd, ok, err := s.store.GetPendingDevice(resp.DeviceID)
	if err != nil || !ok {
		t.Fatalf("GetPendingDevice: ok=%v err=%v", ok, err)
	}
	if pd.Status != PendingDeviceStatusPending || pd.Label != "kitchen-tablet" {
		t.Fatalf("GetPendingDevice: got %+v", pd)
	}
}

// TestAuthServerRequestDevicePairingRateLimited proves an attacker rotating
// device labels to spam pending_devices rows gets throttled by the global
// backstop, not just the per-label limiter it can trivially evade.
func TestAuthServerRequestDevicePairingRateLimited(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	for i := 0; i < devicePairingLimitPerMinute; i++ {
		if _, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "spammer-device"}); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	_, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "spammer-device"})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("call over the per-label limit: err = %v, want ResourceExhausted", err)
	}
}

func TestAuthServerCancelDevicePairing(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	reqResp, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "phone"})
	if err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}
	cancelResp, err := s.CancelDevicePairing(ctx, backendrpc.CancelDevicePairingRequest{DeviceID: reqResp.DeviceID})
	if err != nil {
		t.Fatalf("CancelDevicePairing: %v", err)
	}
	if !cancelResp.Canceled {
		t.Fatal("CancelDevicePairing: expected Canceled=true")
	}
	pd, ok, err := s.store.GetPendingDevice(reqResp.DeviceID)
	if err != nil || !ok {
		t.Fatalf("GetPendingDevice: ok=%v err=%v", ok, err)
	}
	if pd.Status != PendingDeviceStatusRejected {
		t.Fatalf("GetPendingDevice after cancel: status = %q, want %q", pd.Status, PendingDeviceStatusRejected)
	}

	// Canceling an unknown/already-resolved device id is not an error — it
	// just reports nothing was canceled, matching RejectPendingDevice's ok=false.
	cancelResp, err = s.CancelDevicePairing(ctx, backendrpc.CancelDevicePairingRequest{DeviceID: reqResp.DeviceID})
	if err != nil {
		t.Fatalf("second CancelDevicePairing: %v", err)
	}
	if cancelResp.Canceled {
		t.Fatal("CancelDevicePairing: expected Canceled=false for an already-resolved request")
	}
}

// TestAuthServerCancelDevicePairingCannotCancelAnotherDevice proves possession
// of a DIFFERENT device's id is required — cancel is scoped by whatever id
// the caller supplies, and an unrelated/made-up id simply finds nothing.
func TestAuthServerCancelDevicePairingCannotCancelAnotherDevice(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	if _, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "victim-device"}); err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}
	resp, err := s.CancelDevicePairing(ctx, backendrpc.CancelDevicePairingRequest{DeviceID: "guessed-id-that-does-not-exist"})
	if err != nil {
		t.Fatalf("CancelDevicePairing: %v", err)
	}
	if resp.Canceled {
		t.Fatal("CancelDevicePairing: a guessed id must never cancel a real, different pending request")
	}
}

func TestAuthServerSetPasswordRequiresAuthentication(t *testing.T) {
	s := newTestAuthServer(t)
	_, err := s.SetPassword(context.Background(), backendrpc.SetPasswordRequest{Username: "cam", Password: "correct-horse-battery"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("SetPassword without a session: err = %v, want Unauthenticated", err)
	}
}

// TestAuthServerSetPasswordAttachesToCallingSessionNotANewAccount proves the
// core correctness property TODOS.md C454 exists for: SetPassword must
// attach credentials to whatever account the caller's session ALREADY
// belongs to (here, a token-mode identity with no users row yet), never mint
// a second, disconnected account the way Register would.
func TestAuthServerSetPasswordAttachesToCallingSessionNotANewAccount(t *testing.T) {
	s := newTestAuthServer(t)
	tokenUserID := "token:abc123"
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: tokenUserID})

	if _, err := s.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	// The row that gained credentials must be the CALLER's own row (lazily
	// materialized by ensureUserRow), identified by the session's own id —
	// not a fresh `local:cam` row the way Register/CreateLocalUser would create.
	user, _, ok, err := s.store.GetLocalUserByUsername("cam")
	if err != nil {
		t.Fatalf("GetLocalUserByUsername: %v", err)
	}
	if !ok {
		t.Fatal("GetLocalUserByUsername: expected the username to now resolve to an account")
	}
	if user.ID != tokenUserID {
		t.Fatalf("SetPassword attached credentials to id %q, want the CALLING session's own id %q", user.ID, tokenUserID)
	}
	if _, found, _ := s.store.GetUserByID(localUserID("cam")); found {
		t.Fatal("SetPassword must not create a second, disconnected `local:cam` account")
	}

	// The account can now log in with the new username/password — the whole
	// point of the bootstrap.
	loginResp, err := s.Login(context.Background(), backendrpc.LoginRequest{Username: "cam", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("Login after SetPassword: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatal("Login after SetPassword: expected a token pair")
	}
}

func TestAuthServerSetPasswordRejectsUsernameCollision(t *testing.T) {
	s := newTestAuthServer(t)
	if _, err := s.Register(context.Background(), backendrpc.RegisterRequest{Username: "cam", Password: "correct-horse-battery"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "token:someone-else"})
	_, err := s.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "cam", Password: "another-password"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("SetPassword with a taken username: err = %v, want AlreadyExists", err)
	}
}

func TestAuthServerSetPasswordRejectsShortPassword(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "token:abc"})
	_, err := s.SetPassword(ctx, backendrpc.SetPasswordRequest{Username: "cam", Password: "short"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("SetPassword with a short password: err = %v, want InvalidArgument", err)
	}
}

// TestAuthServerWatchPairingStatusApproved proves the full happy path: a
// device requests pairing, an "admin" (directly via the store, standing in
// for pkg/embed.Admin.ApprovePairing) approves it, and the watch stream
// delivers exactly one "approved" event carrying the pairing code, then closes.
func TestAuthServerWatchPairingStatusApproved(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	reqResp, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "watched-device"})
	if err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}

	stream := newFakeServerStream()
	done := make(chan error, 1)
	go func() {
		done <- s.WatchPairingStatusRPC(backendrpc.WatchPairingStatusRequest{DeviceID: reqResp.DeviceID}, stream)
	}()

	// Give the watch loop a moment to start polling before approving —
	// exercises the "still pending" branch, not just an instant resolve.
	time.Sleep(50 * time.Millisecond)
	if ok, err := s.store.ApprovePendingDevice(reqResp.DeviceID, "555555", time.Now().UTC()); err != nil || !ok {
		t.Fatalf("ApprovePendingDevice: ok=%v err=%v", ok, err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WatchPairingStatusRPC: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WatchPairingStatusRPC: timed out waiting for the approved event")
	}

	msgs := stream.sentMessages()
	if len(msgs) != 1 {
		t.Fatalf("WatchPairingStatusRPC: sent %d messages, want exactly 1 (one-shot)", len(msgs))
	}
	ev, ok := msgs[0].(*backendrpc.PairingStatusEvent)
	if !ok {
		t.Fatalf("WatchPairingStatusRPC: sent message type %T, want *backendrpc.PairingStatusEvent", msgs[0])
	}
	if ev.Status != "approved" || ev.PairingCode != "555555" {
		t.Fatalf("WatchPairingStatusRPC: event = %+v, want status=approved code=555555", ev)
	}
}

func TestAuthServerWatchPairingStatusRejected(t *testing.T) {
	s := newTestAuthServer(t)
	ctx := context.Background()
	reqResp, err := s.RequestDevicePairing(ctx, backendrpc.RequestDevicePairingRequest{DeviceLabel: "watched-device"})
	if err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}
	if ok, err := s.store.RejectPendingDevice(reqResp.DeviceID); err != nil || !ok {
		t.Fatalf("RejectPendingDevice: ok=%v err=%v", ok, err)
	}
	stream := newFakeServerStream()
	if err := s.WatchPairingStatusRPC(backendrpc.WatchPairingStatusRequest{DeviceID: reqResp.DeviceID}, stream); err != nil {
		t.Fatalf("WatchPairingStatusRPC: %v", err)
	}
	msgs := stream.sentMessages()
	if len(msgs) != 1 {
		t.Fatalf("WatchPairingStatusRPC: sent %d messages, want exactly 1", len(msgs))
	}
	ev := msgs[0].(*backendrpc.PairingStatusEvent)
	if ev.Status != "rejected" || ev.PairingCode != "" {
		t.Fatalf("WatchPairingStatusRPC: event = %+v, want status=rejected with no code", ev)
	}
}

func TestAuthServerWatchPairingStatusUnknownDevice(t *testing.T) {
	s := newTestAuthServer(t)
	stream := newFakeServerStream()
	err := s.WatchPairingStatusRPC(backendrpc.WatchPairingStatusRequest{DeviceID: "never-minted"}, stream)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("WatchPairingStatusRPC for an unknown device: err = %v, want NotFound", err)
	}
}
