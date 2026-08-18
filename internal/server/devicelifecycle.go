// SPDX-License-Identifier: MIT

package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// This file models what happens to a device access request after somebody
// decides about it (TODOS.md C700).
//
// The original design stored one word — pending, approved, rejected — and
// deleted the row the moment it expired. That is enough to run the approval
// queue and not enough to answer any question an operator actually asks when
// something has gone wrong: which account did this browser get? was the code
// ever used? did the admin refuse this, or did the user cancel it? has it
// simply timed out? With no answers, a device account with no pending request
// is an orphan in the console, which is precisely the state that made the
// duplicate-account incident impossible to read.

// Additional PendingDeviceStatus values (see pendingdevice.go for the original
// three). Together they form the request's full life story.
const (
	// PendingDeviceStatusRedeemed marks an approved request whose pairing code
	// the device actually used. Distinct from approved: an approval nobody
	// redeemed is a dead end, and it should not look like a working device.
	PendingDeviceStatusRedeemed = "redeemed"
	// PendingDeviceStatusCanceled marks a request the DEVICE withdrew.
	// Previously indistinguishable from an admin refusal, though the two mean
	// opposite things about whether to trust the next request from that device.
	PendingDeviceStatusCanceled = "canceled"
	// PendingDeviceStatusExpired marks a request nobody answered in time. It is
	// now an explicit state rather than the absence of a row, so "we ignored
	// this" and "this never happened" stop looking the same.
	PendingDeviceStatusExpired = "expired"
)

// Resolver values for pending_devices.resolved_by.
const (
	// ResolvedByAdmin means an operator decided.
	ResolvedByAdmin = "admin"
	// ResolvedByDevice means the requesting device withdrew its own request.
	ResolvedByDevice = "device"
	// ResolvedBySystem means the clock decided — nobody answered in time.
	ResolvedBySystem = "system"
	// ResolvedByUnknown is the honest backfill for rows resolved before this
	// was recorded. It is not "admin": guessing would put a decision in a real
	// operator's name.
	ResolvedByUnknown = "unknown"
)

// PendingDeviceRetention is how long a RESOLVED request stays readable in the
// console after it expires.
//
// It exists because C700's requirement (show operators the lifecycle) and the
// original security finding (the table must not grow without bound) pull in
// opposite directions. A fixed window satisfies both: history is available for
// as long as anyone is plausibly still debugging an incident, and the table's
// steady-state size stays proportional to the request rate over one month
// rather than to all time.
const PendingDeviceRetention = 30 * 24 * time.Hour

// DeviceLifecycle is one request's full story, as the console shows it.
type DeviceLifecycle struct {
	DeviceID    string
	Label       string
	Status      string
	UserID      string
	RequestedAt time.Time
	ExpiresAt   time.Time
	ResolvedAt  time.Time
	ResolvedBy  string
	RedeemedAt  time.Time
	// PairingCodeIssued reports whether a code is currently attached. The code
	// itself is never returned here: it is shown once, at the moment of
	// approval, and reading it back later would turn the audit view into a
	// credential store.
	PairingCodeIssued bool
}

// Terminal reports whether the request can no longer change on its own.
func (d DeviceLifecycle) Terminal() bool {
	switch d.Status {
	case PendingDeviceStatusRejected, PendingDeviceStatusCanceled,
		PendingDeviceStatusExpired, PendingDeviceStatusRedeemed:
		return true
	}
	return false
}

// EffectiveStatus reports the status a request has RIGHT NOW, folding in the
// clock. A row still marked pending past its expiry is expired in every sense
// that matters to a reader; the stored value catches up when ExpirePendingDevices
// next runs, and this makes the console agree with reality in the meantime
// rather than showing a request as actionable when approving it would fail.
func (d DeviceLifecycle) EffectiveStatus(now time.Time) string {
	if d.Status == PendingDeviceStatusPending && !now.Before(d.ExpiresAt) {
		return PendingDeviceStatusExpired
	}
	return d.Status
}

const deviceLifecycleColumns = `device_id, label, status, pairing_code, requested_at, expires_at, user_id, resolved_at, resolved_by, redeemed_at`

// scanDeviceLifecycle reads one row in deviceLifecycleColumns order.
func scanDeviceLifecycle(scan func(dest ...any) error) (DeviceLifecycle, error) {
	var (
		d                                                  DeviceLifecycle
		pairingCode                                        string
		requestedRaw, expiresRaw, resolvedRaw, redeemedRaw string
	)
	if err := scan(&d.DeviceID, &d.Label, &d.Status, &pairingCode, &requestedRaw, &expiresRaw,
		&d.UserID, &resolvedRaw, &d.ResolvedBy, &redeemedRaw); err != nil {
		return DeviceLifecycle{}, err
	}
	d.PairingCodeIssued = strings.TrimSpace(pairingCode) != ""
	var err error
	if d.RequestedAt, err = parseTime(requestedRaw); err != nil {
		return DeviceLifecycle{}, fmt.Errorf("parse requested_at: %w", err)
	}
	if d.ExpiresAt, err = parseTime(expiresRaw); err != nil {
		return DeviceLifecycle{}, fmt.Errorf("parse expires_at: %w", err)
	}
	if d.ResolvedAt, err = parseOptionalTime(resolvedRaw); err != nil {
		return DeviceLifecycle{}, fmt.Errorf("parse resolved_at: %w", err)
	}
	if d.RedeemedAt, err = parseOptionalTime(redeemedRaw); err != nil {
		return DeviceLifecycle{}, fmt.Errorf("parse redeemed_at: %w", err)
	}
	return d, nil
}

// logRedeemBookkeeping notes a failure to close the loop on a redeemed request.
// It is deliberately not an error to the caller: the redemption itself already
// succeeded, and refusing the session the user just legitimately earned because
// an audit column would not update is the wrong trade. The console shows the
// request as still "approved", which is a smaller lie than a failed sign-in.
func (s *authServer) logRedeemBookkeeping(err error) {
	slog.Warn("pairing redemption bookkeeping failed", "err", err)
}

// ListDeviceLifecycles returns device requests newest first, for the console's
// device view. Unlike ListPendingDevices it includes resolved and expired rows —
// that history is the point. limit <= 0 defaults to 100.
func (s *Store) ListDeviceLifecycles(limit int) ([]DeviceLifecycle, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("server store: not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	defer s.observeDB("ListDeviceLifecycles", time.Now())
	rows, err := s.db.Query(`SELECT `+deviceLifecycleColumns+`
FROM pending_devices ORDER BY requested_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("server store: list device lifecycles: %w", err)
	}
	defer rows.Close()
	var out []DeviceLifecycle
	for rows.Next() {
		d, err := scanDeviceLifecycle(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("server store: scan device lifecycle: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeviceLifecyclesForUser returns the device requests approved onto one account —
// the console's "which browsers are signed in as this user?" answer.
func (s *Store) DeviceLifecyclesForUser(userID string) ([]DeviceLifecycle, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}
	defer s.observeDB("DeviceLifecyclesForUser", time.Now())
	rows, err := s.db.Query(`SELECT `+deviceLifecycleColumns+`
FROM pending_devices WHERE user_id = ? ORDER BY requested_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: list device lifecycles for user: %w", err)
	}
	defer rows.Close()
	var out []DeviceLifecycle
	for rows.Next() {
		d, err := scanDeviceLifecycle(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("server store: scan device lifecycle: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDeviceLifecycle reads one request's full story.
func (s *Store) GetDeviceLifecycle(deviceID string) (DeviceLifecycle, bool, error) {
	if s == nil || s.db == nil {
		return DeviceLifecycle{}, false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceLifecycle{}, false, nil
	}
	defer s.observeDB("GetDeviceLifecycle", time.Now())
	row := s.db.QueryRow(`SELECT `+deviceLifecycleColumns+` FROM pending_devices WHERE device_id = ?`, deviceID)
	d, err := scanDeviceLifecycle(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceLifecycle{}, false, nil
	}
	if err != nil {
		return DeviceLifecycle{}, false, fmt.Errorf("server store: get device lifecycle: %w", err)
	}
	return d, true, nil
}

// MarkPendingDeviceRedeemed records that an approved request's pairing code was
// actually used, and by which account. ok is false when the row is not in the
// approved state — a redemption of a code whose request was already resolved
// some other way is not something to quietly overwrite.
func (s *Store) MarkPendingDeviceRedeemed(deviceID string, now time.Time) (ok bool, err error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false, nil
	}
	defer s.observeDB("MarkPendingDeviceRedeemed", time.Now())
	res, err := s.db.Exec(`UPDATE pending_devices SET status = ?, redeemed_at = ?
WHERE device_id = ? AND status = ?`,
		PendingDeviceStatusRedeemed, formatTime(now.UTC()), deviceID, PendingDeviceStatusApproved)
	if err != nil {
		return false, fmt.Errorf("server store: mark device redeemed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: mark device redeemed: %w", err)
	}
	return affected > 0, nil
}

// MarkPendingDeviceRedeemedByCode records redemption for whichever request
// carries this pairing code. RedeemPairingCode knows the code, not the device
// id — the device id never leaves the requesting browser — so this is the only
// join available at the moment redemption happens.
//
// A code is unique and single-use, so at most one row can match.
func (s *Store) MarkPendingDeviceRedeemedByCode(code string, now time.Time) (ok bool, err error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return false, nil
	}
	defer s.observeDB("MarkPendingDeviceRedeemedByCode", time.Now())
	res, err := s.db.Exec(`UPDATE pending_devices SET status = ?, redeemed_at = ?
WHERE pairing_code = ? AND status = ?`,
		PendingDeviceStatusRedeemed, formatTime(now.UTC()), code, PendingDeviceStatusApproved)
	if err != nil {
		return false, fmt.Errorf("server store: mark device redeemed by code: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: mark device redeemed by code: %w", err)
	}
	return affected > 0, nil
}

// ExpirePendingDevices moves timed-out requests to the expired state instead of
// leaving them looking actionable. It is the transition PruneExpiredPendingDevices
// used to perform by deletion — the difference is that the row survives to be
// read, and is deleted later by retention.
func (s *Store) ExpirePendingDevices(now time.Time) (expired int64, err error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("ExpirePendingDevices", time.Now())
	res, err := s.db.Exec(`UPDATE pending_devices SET status = ?, resolved_at = ?, resolved_by = ?
WHERE status = ? AND expires_at <= ?`,
		PendingDeviceStatusExpired, formatTime(now.UTC()), ResolvedBySystem,
		PendingDeviceStatusPending, formatTime(now.UTC()))
	if err != nil {
		return 0, fmt.Errorf("server store: expire pending devices: %w", err)
	}
	expired, err = res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("server store: expire pending devices: %w", err)
	}
	return expired, nil
}

// SetPendingDeviceAccount records which account an approval produced or attached
// to, and stamps who resolved it.
func (s *Store) SetPendingDeviceAccount(deviceID, userID, resolvedBy string, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("server store: device id is required")
	}
	defer s.observeDB("SetPendingDeviceAccount", time.Now())
	if _, err := s.db.Exec(`UPDATE pending_devices SET user_id = ?, resolved_at = ?, resolved_by = ?
WHERE device_id = ?`, strings.TrimSpace(userID), formatTime(now.UTC()), resolvedBy, deviceID); err != nil {
		return fmt.Errorf("server store: set pending device account: %w", err)
	}
	return nil
}

// ResolvePendingDevice moves a still-pending request to a terminal state,
// recording who moved it. It replaces the shared RejectPendingDevice for callers
// that know whether the admin or the device is acting — the two were previously
// the same row update, so the console could not tell a refusal from a
// withdrawal.
func (s *Store) ResolvePendingDevice(deviceID, status, resolvedBy string, now time.Time) (ok bool, err error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false, nil
	}
	switch status {
	case PendingDeviceStatusRejected, PendingDeviceStatusCanceled:
	default:
		return false, fmt.Errorf("server store: %q is not a resolution a caller may set", status)
	}
	defer s.observeDB("ResolvePendingDevice", time.Now())
	res, err := s.db.Exec(`UPDATE pending_devices SET status = ?, resolved_at = ?, resolved_by = ?
WHERE device_id = ? AND status = ?`,
		status, formatTime(now.UTC()), resolvedBy, deviceID, PendingDeviceStatusPending)
	if err != nil {
		return false, fmt.Errorf("server store: resolve pending device: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: resolve pending device: %w", err)
	}
	return affected > 0, nil
}

// ReissuePairingCode attaches a fresh code to a request whose approval was never
// redeemed, and pushes its expiry out so the waiting device can pick it up.
//
// This is the recovery path for the commonest real failure: an approval whose
// five-minute code lapsed before the person got back to the other browser. The
// alternative today is to reject and re-request, which mints ANOTHER device
// account — the exact behaviour that produced the duplicate in the first place.
//
// It refuses a request that was already redeemed: that code did its job, and
// issuing a second one for the same request would be a way to hand out a second
// credential for an account without an approval decision behind it.
func (s *Store) ReissuePairingCode(deviceID string, now time.Time) (code string, ok bool, err error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("server store: not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", false, nil
	}
	life, found, err := s.GetDeviceLifecycle(deviceID)
	if err != nil {
		return "", false, err
	}
	if !found || strings.TrimSpace(life.UserID) == "" {
		return "", false, nil
	}
	if life.Status != PendingDeviceStatusApproved && life.Status != PendingDeviceStatusExpired {
		return "", false, nil
	}
	code, _, err = s.MintPairingCode(life.UserID, now)
	if err != nil {
		return "", false, fmt.Errorf("server store: reissue pairing code: %w", err)
	}
	defer s.observeDB("ReissuePairingCode", time.Now())
	if _, err := s.db.Exec(`UPDATE pending_devices SET status = ?, pairing_code = ?, expires_at = ?
WHERE device_id = ?`,
		PendingDeviceStatusApproved, code, formatTime(now.UTC().Add(PendingDeviceTTL)), deviceID); err != nil {
		return "", false, fmt.Errorf("server store: reissue pairing code: %w", err)
	}
	return code, true, nil
}

// PruneRetiredPendingDevices deletes resolved rows once they are older than
// PendingDeviceRetention, keeping the table bounded now that expiry no longer
// deletes on the spot. Still-pending rows are never deleted here — they are
// expired first (ExpirePendingDevices), which is what starts their retention
// clock.
func (s *Store) PruneRetiredPendingDevices(now time.Time) (deleted int64, err error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("server store: not configured")
	}
	defer s.observeDB("PruneRetiredPendingDevices", time.Now())
	cutoff := now.UTC().Add(-PendingDeviceRetention)
	res, err := s.db.Exec(`DELETE FROM pending_devices WHERE status <> ? AND expires_at < ?`,
		PendingDeviceStatusPending, formatTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("server store: prune retired pending devices: %w", err)
	}
	deleted, err = res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("server store: prune retired pending devices: %w", err)
	}
	return deleted, nil
}
