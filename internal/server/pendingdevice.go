// SPDX-License-Identifier: MIT

package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PendingDeviceTTL is how long an unresolved pending-device request stays
// watchable before WatchPairingStatus reports it "expired" (TODOS.md C454).
// Longer than PairingCodeTTL: this leg additionally waits on a human admin
// noticing and clicking Approve, not just a device typing a code.
const PendingDeviceTTL = 10 * time.Minute

// pendingDeviceIDBytes sizes the random device id RequestDevicePairing mints.
// Unlike a pairing code, this id is never typed by a human — it is only ever
// passed by the device itself (WatchPairingStatus, CancelDevicePairing) — so
// it is sized as an unguessable bearer secret, not a short human-typeable
// code. 20 bytes -> 32 base32 chars.
const pendingDeviceIDBytes = 20

// PendingDeviceStatus values.
const (
	PendingDeviceStatusPending  = "pending"
	PendingDeviceStatusApproved = "approved"
	PendingDeviceStatusRejected = "rejected"
)

// PendingDevice is one row of the pending_devices table.
type PendingDevice struct {
	DeviceID    string
	Label       string
	Status      string
	PairingCode string
	RequestedAt time.Time
	ExpiresAt   time.Time
}

// pendingDeviceIDMintAttempts bounds retries on the (astronomically
// unlikely) event a freshly minted device id collides with one already
// outstanding.
const pendingDeviceIDMintAttempts = 5

// ErrPendingDeviceIDExhausted is returned by MintPendingDevice if it could
// not find an unused device id after pendingDeviceIDMintAttempts tries.
var ErrPendingDeviceIDExhausted = errors.New("server store: could not mint a unique pending device id")

// MintPendingDevice creates a new pending device-pairing request
// (TODOS.md C454): the first step of the admin-approved bootstrap, before any
// account is involved. label is the device's own human-readable name (e.g.
// browser/OS), shown to the admin so they can tell which real device is
// asking.
func (s *Store) MintPendingDevice(label string, now time.Time) (deviceID string, expiresAt time.Time, err error) {
	if s == nil || s.db == nil {
		return "", time.Time{}, fmt.Errorf("server store: not configured")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return "", time.Time{}, fmt.Errorf("server store: device label is required")
	}
	defer s.observeDB("MintPendingDevice", time.Now())
	now = now.UTC()
	// Best-effort opportunistic cleanup (security review finding: nothing
	// ever deleted pending_devices rows, so the table grew unbounded even
	// under devicePairingGlobalLimiter's per-minute cap — that limiter only
	// slows growth, it doesn't bound it over days/weeks). Piggybacking the
	// sweep on every real mint bounds the table's steady-state size to
	// roughly the request RATE rather than the cumulative total; a failure
	// here must never block a legitimate pairing request, so it's ignored.
	// Two steps since C700: time-out the requests nobody answered (so the
	// console can show "expired" rather than an absence), then delete the ones
	// whose retention window has passed. Deleting on expiry, as this used to,
	// bounded the table by erasing the history an operator needs to read.
	_, _ = s.ExpirePendingDevices(now)
	_, _ = s.PruneRetiredPendingDevices(now)
	expiresAt = now.Add(PendingDeviceTTL)
	for attempt := 0; attempt < pendingDeviceIDMintAttempts; attempt++ {
		id, genErr := randomPendingDeviceID()
		if genErr != nil {
			return "", time.Time{}, fmt.Errorf("server store: generate device id: %w", genErr)
		}
		_, execErr := s.db.Exec(`INSERT INTO pending_devices(device_id, label, status, pairing_code, requested_at, expires_at) VALUES(?, ?, ?, '', ?, ?)`,
			id, label, PendingDeviceStatusPending, formatTime(now), formatTime(expiresAt))
		if execErr == nil {
			return id, expiresAt, nil
		}
		if !isUniqueConstraintErr(execErr) {
			return "", time.Time{}, fmt.Errorf("server store: mint pending device: %w", execErr)
		}
		// Collision on the PRIMARY KEY(device_id) — vanishingly rare; retry with a fresh id.
	}
	return "", time.Time{}, ErrPendingDeviceIDExhausted
}

// GetPendingDevice looks up a pending device request by id. ok is false if
// no such request was ever minted.
func (s *Store) GetPendingDevice(deviceID string) (PendingDevice, bool, error) {
	if s == nil || s.db == nil {
		return PendingDevice{}, false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return PendingDevice{}, false, nil
	}
	defer s.observeDB("GetPendingDevice", time.Now())
	var pd PendingDevice
	var requestedRaw, expiresRaw string
	err := s.db.QueryRow(`SELECT device_id, label, status, pairing_code, requested_at, expires_at FROM pending_devices WHERE device_id = ?`, deviceID).
		Scan(&pd.DeviceID, &pd.Label, &pd.Status, &pd.PairingCode, &requestedRaw, &expiresRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingDevice{}, false, nil
	}
	if err != nil {
		return PendingDevice{}, false, fmt.Errorf("server store: get pending device: %w", err)
	}
	if pd.RequestedAt, err = parseTime(requestedRaw); err != nil {
		return PendingDevice{}, false, fmt.Errorf("server store: parse pending device requested_at: %w", err)
	}
	if pd.ExpiresAt, err = parseTime(expiresRaw); err != nil {
		return PendingDevice{}, false, fmt.Errorf("server store: parse pending device expires_at: %w", err)
	}
	return pd, true, nil
}

// ListPendingDevices returns every still-pending, unexpired request, oldest
// first — the admin console's approve/reject queue (TODOS.md C454). Expired
// requests are intentionally excluded rather than deleted here: they age out
// silently from the admin's perspective (nothing to act on), while
// WatchPairingStatus is what tells the waiting device it expired.
func (s *Store) ListPendingDevices(now time.Time) ([]PendingDevice, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("ListPendingDevices", time.Now())
	rows, err := s.db.Query(`SELECT device_id, label, status, pairing_code, requested_at, expires_at
FROM pending_devices WHERE status = ? AND expires_at > ? ORDER BY requested_at ASC`,
		PendingDeviceStatusPending, formatTime(now.UTC()))
	if err != nil {
		return nil, fmt.Errorf("server store: list pending devices: %w", err)
	}
	defer rows.Close()
	var out []PendingDevice
	for rows.Next() {
		var pd PendingDevice
		var requestedRaw, expiresRaw string
		if err := rows.Scan(&pd.DeviceID, &pd.Label, &pd.Status, &pd.PairingCode, &requestedRaw, &expiresRaw); err != nil {
			return nil, fmt.Errorf("server store: scan pending device: %w", err)
		}
		if pd.RequestedAt, err = parseTime(requestedRaw); err != nil {
			return nil, fmt.Errorf("server store: parse pending device requested_at: %w", err)
		}
		if pd.ExpiresAt, err = parseTime(expiresRaw); err != nil {
			return nil, fmt.Errorf("server store: parse pending device expires_at: %w", err)
		}
		out = append(out, pd)
	}
	return out, rows.Err()
}

// ApprovePendingDevice atomically moves a still-pending, unexpired request to
// approved and attaches the pairing code the admin minted for it, so
// WatchPairingStatus can push it to the waiting device. ok is false if the
// request was already resolved (approved/rejected), expired, or never
// existed — never overwrites a decision already made.
func (s *Store) ApprovePendingDevice(deviceID, pairingCode string, now time.Time) (ok bool, err error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	pairingCode = strings.TrimSpace(pairingCode)
	if deviceID == "" || pairingCode == "" {
		return false, fmt.Errorf("server store: device id and pairing code are required")
	}
	defer s.observeDB("ApprovePendingDevice", time.Now())
	res, err := s.db.Exec(`UPDATE pending_devices SET status = ?, pairing_code = ?
WHERE device_id = ? AND status = ? AND expires_at > ?`,
		PendingDeviceStatusApproved, pairingCode, deviceID, PendingDeviceStatusPending, formatTime(now.UTC()))
	if err != nil {
		return false, fmt.Errorf("server store: approve pending device: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: approve pending device: %w", err)
	}
	return affected > 0, nil
}

// RejectPendingDevice atomically moves a still-pending request to rejected, as
// an OPERATOR decision. ok is false if the request was already resolved or never
// existed.
//
// The device's own withdrawal goes through CancelPendingDevice instead. Both
// used to call this one function, so a resolved row could not say whether an
// operator had refused the request or the user had changed their mind in their
// own browser — opposite facts about whether to trust the next request from
// that device.
func (s *Store) RejectPendingDevice(deviceID string) (ok bool, err error) {
	return s.ResolvePendingDevice(deviceID, PendingDeviceStatusRejected, ResolvedByAdmin, time.Now().UTC())
}

// CancelPendingDevice records the requesting DEVICE withdrawing its own request.
// Same authority as a rejection, different meaning.
func (s *Store) CancelPendingDevice(deviceID string) (ok bool, err error) {
	return s.ResolvePendingDevice(deviceID, PendingDeviceStatusCanceled, ResolvedByDevice, time.Now().UTC())
}

// PruneExpiredPendingDevices runs the full housekeeping pass: time out the
// requests nobody answered, then delete the resolved rows whose retention
// window has passed. It returns how many rows were DELETED.
//
// It used to delete every row the moment its expiry passed, on the reasoning
// that nothing read a resolved row again. C700 established that something does:
// an operator, trying to work out which account a browser was approved onto and
// whether its code was ever used. Deleting on expiry meant a device account
// whose request had timed out appeared in the console with no request behind it
// at all — an orphan with no explanation, which is what made the duplicate
// account impossible to diagnose.
//
// The security finding it was written for still holds — the table must not grow
// without bound, since devicePairingGlobalLimiter caps the mint RATE and not the
// cumulative total. Retention satisfies both: history for as long as anyone is
// plausibly still debugging, then deletion (see PendingDeviceRetention).
func (s *Store) PruneExpiredPendingDevices(now time.Time) (deleted int64, err error) {
	if _, err := s.ExpirePendingDevices(now); err != nil {
		return 0, err
	}
	return s.PruneRetiredPendingDevices(now)
}

// randomPendingDeviceID returns a cryptographically random, URL-safe device
// id — an unguessable bearer secret, not a human-typed code (see
// pendingDeviceIDBytes's doc comment).
func randomPendingDeviceID() (string, error) {
	buf := make([]byte, pendingDeviceIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}
