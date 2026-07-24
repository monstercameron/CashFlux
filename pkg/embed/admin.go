// SPDX-License-Identifier: MIT

package embed

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/server"
)

// PendingDevice is one entry in ListPendingDevices' result — a public,
// pkg/embed-shaped mirror of internal/server.PendingDevice, hiding the
// pairing code (only ApprovePairing's return value carries it, and only at
// the moment of approval — see its doc comment).
type PendingDevice struct {
	DeviceID    string
	Label       string
	RequestedAt time.Time
	ExpiresAt   time.Time
}

// ListPendingDevices lists every unresolved device-pairing request waiting
// on admin approval or rejection (TODOS.md C454), oldest first.
func (a *Admin) ListPendingDevices() ([]PendingDevice, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("pkg/embed: admin is not configured")
	}
	devices, err := a.store.ListPendingDevices(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	out := make([]PendingDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, PendingDevice{
			DeviceID:    d.DeviceID,
			Label:       d.Label,
			RequestedAt: d.RequestedAt,
			ExpiresAt:   d.ExpiresAt,
		})
	}
	return out, nil
}

// ApprovePairing approves a pending device request (TODOS.md C454) and
// returns the pairing code it minted, so the admin console can show it
// alongside the device's own display of the same code — a human
// cross-check that the approval landed on the device the admin actually
// meant to approve, not a different pending request.
//
// Creates a BRAND-NEW account for this approval, rather than reusing any
// existing identity: this deployment admits a small, admin-invited set of
// DISTINCT people/devices (see NewSyncAndAuthBridge's doc comment — "a host
// that wants CashFlux sync for itself and a small, admin-invited set of
// people"), and RedeemPairingCode's own invariant is that it never creates
// an account itself (C421) — so the account has to already exist by the time
// the code is minted. This is the "lazy user creation before minting" the
// plan called for: unlike SyncService.ensureUser (which materializes a row
// for an id a SESSION already claims), there is no session at all here —
// pkg/embed.Admin is called directly from the embedding host's own Go code,
// not through any RPC — so a fresh id is minted for this approval, not
// derived from anything that already exists.
//
// Returns approved=false (with no error) if the request was already
// resolved or has expired — Store.ApprovePendingDevice never overwrites a
// decision already made. In that case the freshly-created account and
// minted pairing code are simply never used and expire unread; harmless
// (an unused account with no data, no way to sign into it without the code)
// but real, so callers should not treat approved=false as an error.
func (a *Admin) ApprovePairing(deviceID string) (approved bool, pairingCode string, err error) {
	if a == nil || a.store == nil {
		return false, "", fmt.Errorf("pkg/embed: admin is not configured")
	}
	now := time.Now().UTC()
	userID, err := newDeviceUserID()
	if err != nil {
		return false, "", fmt.Errorf("pkg/embed: generate account id: %w", err)
	}
	if err := a.store.UpsertUser(server.User{ID: userID, Provider: "device", Subject: userID, CreatedAt: now}); err != nil {
		return false, "", fmt.Errorf("pkg/embed: create account: %w", err)
	}
	code, _, err := a.store.MintPairingCode(userID, now)
	if err != nil {
		return false, "", fmt.Errorf("pkg/embed: mint pairing code: %w", err)
	}
	approved, err = a.store.ApprovePendingDevice(deviceID, code, now)
	if err != nil {
		return false, "", fmt.Errorf("pkg/embed: approve pending device: %w", err)
	}
	if !approved {
		return false, "", nil
	}
	return true, code, nil
}

// RejectPairing rejects a pending device request (TODOS.md C454). Returns
// rejected=false (with no error) if the request was already resolved.
func (a *Admin) RejectPairing(deviceID string) (rejected bool, err error) {
	if a == nil || a.store == nil {
		return false, fmt.Errorf("pkg/embed: admin is not configured")
	}
	rejected, err = a.store.RejectPendingDevice(deviceID)
	if err != nil {
		return false, fmt.Errorf("pkg/embed: reject pending device: %w", err)
	}
	return rejected, nil
}

// newDeviceUserID mints a fresh, unguessable account id for a newly
// admin-approved device pairing, matching the existing "provider:subject" id
// convention (see internal/server.User).
func newDeviceUserID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "device:" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}
