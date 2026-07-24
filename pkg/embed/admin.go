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

// OwnerAccountID is the single CashFlux account every activation code binds
// to. Deliberately fixed and well-known rather than random: an activation
// code can only be minted from behind the embedding host's own admin login,
// so the access control is that login — not the secrecy of an account id —
// and a stable id is what makes every activated device land in the SAME
// account and therefore sync with each other. Contrast ApprovePairing, which
// mints a fresh account per approval because that flow admits distinct
// people.
const OwnerAccountID = "device:owner"

// ownerAccountProvider/ownerAccountSubject are OwnerAccountID's (provider,
// subject) pair — the unique key Store.UpsertUser conflicts on, which is what
// makes MintActivationCode idempotent about account creation.
const (
	ownerAccountProvider = "device"
	ownerAccountSubject  = "owner"
)

// MintActivationCode mints a short-lived (server.PairingCodeTTL), single-use
// activation code for the deployment owner's account, creating that account
// on the first call and reusing it forever after.
//
// This is the admin-initiated half of the pairing flow: the host's admin
// console mints a code, the owner types it into any CashFlux client's
// Settings → Cloud, and AuthService.RedeemPairingCode turns it into a real
// session. Unlike RequestDevicePairing → ApprovePairing there is no pending
// request to approve first — the code IS the credential, and minting it
// requires nothing but access to the embedding host's admin console.
//
// Every code binds to OwnerAccountID, so activating a second device joins the
// same account and the two devices sync with each other. Codes are single-use
// and expire, so minting one repeatedly is safe; an unredeemed code simply
// expires unread.
func (a *Admin) MintActivationCode() (code string, expiresAt time.Time, err error) {
	if a == nil || a.store == nil {
		return "", time.Time{}, fmt.Errorf("pkg/embed: admin is not configured")
	}
	now := time.Now().UTC()
	if err := a.store.UpsertUser(server.User{
		ID:        OwnerAccountID,
		Provider:  ownerAccountProvider,
		Subject:   ownerAccountSubject,
		CreatedAt: now,
	}); err != nil {
		return "", time.Time{}, fmt.Errorf("pkg/embed: create owner account: %w", err)
	}
	code, expiresAt, err = a.store.MintPairingCode(OwnerAccountID, now)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("pkg/embed: mint activation code: %w", err)
	}
	return code, expiresAt, nil
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

// User is one enrolled account, for the admin console's user list.
type User struct {
	ID                 string
	Provider           string
	Email              string
	CreatedAt          time.Time
	SubscriptionPlan   string
	SubscriptionStatus string
	// RequestsThisMonth is the summed request count across this account's
	// usage rows for the current calendar month (UTC) — the admin console's
	// "request volume" column.
	RequestsThisMonth int64
}

// maxListUsersPage caps a single ListUsers call, matching
// internal/server.Store.ListUsersFiltered's own safety ceiling.
const maxListUsersPage = 500

// ListUsers returns a page of enrolled accounts, newest first, each carrying
// its current calendar month's request volume — the admin console's user
// list. limit is clamped to [1, 500]; offset below 0 is treated as 0.
func (a *Admin) ListUsers(limit, offset int) ([]User, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("pkg/embed: admin is not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > maxListUsersPage {
		limit = maxListUsersPage
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := a.store.ListUsers(limit, offset)
	if err != nil {
		return nil, fmt.Errorf("pkg/embed: list users: %w", err)
	}
	monthStart := currentMonthStartUTC()
	out := make([]User, 0, len(rows))
	for _, r := range rows {
		createdAt, _ := time.Parse(time.RFC3339Nano, r.CreatedAt) // zero value on a malformed/legacy row — never fatal to the whole list
		requests, err := a.monthlyRequestTotal(r.ID, monthStart)
		if err != nil {
			return nil, fmt.Errorf("pkg/embed: usage for %s: %w", r.ID, err)
		}
		out = append(out, User{
			ID:                 r.ID,
			Provider:           r.Provider,
			Email:              r.Email,
			CreatedAt:          createdAt,
			SubscriptionPlan:   r.SubscriptionPlan,
			SubscriptionStatus: r.SubscriptionStatus,
			RequestsThisMonth:  requests,
		})
	}
	return out, nil
}

// monthlyRequestTotal sums a user's daily request counts from monthStart
// onward — a small (at most 31-row) query per user, acceptable at this
// package's target scale (a single embedding host's own small, admin-invited
// user set — see NewSyncAndAuthBridge's doc comment — not a
// multi-tenant SaaS user list).
func (a *Admin) monthlyRequestTotal(userID string, monthStart time.Time) (int64, error) {
	usage, err := a.store.ListUserUsage(userID, monthStart)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, u := range usage {
		total += u.Requests
	}
	return total, nil
}

// currentMonthStartUTC returns 00:00:00 UTC on the 1st of the current
// calendar month.
func currentMonthStartUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// StorageStats reports on-disk storage usage for the admin console's storage
// panel: the SQLite database's own size and the total size of every stored
// artifact blob.
func (a *Admin) StorageStats() (dbBytes, blobBytes int64, err error) {
	if a == nil || a.store == nil {
		return 0, 0, fmt.Errorf("pkg/embed: admin is not configured")
	}
	dbBytes, blobBytes, err = a.store.StorageStats()
	if err != nil {
		return 0, 0, fmt.Errorf("pkg/embed: storage stats: %w", err)
	}
	return dbBytes, blobBytes, nil
}
