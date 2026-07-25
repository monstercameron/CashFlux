// SPDX-License-Identifier: MIT

package embed

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
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

// OwnerSession is a ready-to-use CashFlux session for the deployment owner.
type OwnerSession struct {
	AccessToken      string
	RefreshToken     string
	ExpiresInSeconds int64
	// DeviceID is the refresh-token FAMILY id, which is what the client reports as
	// its device and what an operator revokes to sign one device out.
	DeviceID string
}

// ProvisionOwnerSession issues a session for the owner account directly, creating
// that account on first use exactly as MintActivationCode does.
//
// This exists so an embedding host that has ALREADY authenticated the operator —
// its own admin login, on its own origin — can hand the client a signed-in
// CashFlux without a second credential. An activation code between a console you
// just logged into and the data behind it is a step that proves nothing new; it
// only guarantees the operator types six digits to reach their own books.
//
// The host is responsible for calling this ONLY on a request it has authenticated.
// CashFlux cannot see the host's session and does not try to: everything this
// function knows is that its caller is Go code running inside the host process.
// That is the whole trust boundary, and it is why nothing reachable over the wire
// calls it.
func (a *Admin) ProvisionOwnerSession(deviceLabel string) (OwnerSession, error) {
	if a == nil || a.store == nil {
		return OwnerSession{}, fmt.Errorf("pkg/embed: admin is not configured")
	}
	now := time.Now().UTC()
	if err := a.store.UpsertUser(server.User{
		ID:        OwnerAccountID,
		Provider:  ownerAccountProvider,
		Subject:   ownerAccountSubject,
		CreatedAt: now,
	}); err != nil {
		return OwnerSession{}, fmt.Errorf("pkg/embed: create owner account: %w", err)
	}
	// The owner account is the one account whose role is not a judgement call.
	// Re-asserted on every provision so a hand-edited database cannot leave the
	// operator unable to write to their own deployment.
	if err := a.store.SetUserRole(OwnerAccountID, RoleOwner); err != nil {
		return OwnerSession{}, fmt.Errorf("pkg/embed: set owner role: %w", err)
	}
	if strings.TrimSpace(deviceLabel) == "" {
		deviceLabel = "owner session"
	}
	pair, err := server.IssueSessionForUser(a.cfg, a.store, OwnerAccountID, deviceLabel)
	if err != nil {
		return OwnerSession{}, fmt.Errorf("pkg/embed: issue owner session: %w", err)
	}
	return OwnerSession{
		AccessToken:      pair.AccessToken,
		RefreshToken:     pair.RefreshToken,
		ExpiresInSeconds: pair.ExpiresInSeconds,
		DeviceID:         pair.DeviceID,
	}, nil
}

// ActivationCodeIsValid reports whether a code is one this server minted and has
// not yet consumed, WITHOUT consuming it.
//
// It exists for one job: letting the embedding host recognise a visitor who is
// arriving with a code the host itself just minted from an authenticated admin
// console, and let them through its own front door on that basis. The code is then
// redeemed normally by the client, once, through the usual RPC — this only peeks.
//
// Peeking is safe to expose to a request-handling path in a way that consuming
// would not be: a wrong guess here costs an attacker a lookup, while a consuming
// check would let anyone burn a legitimate code by visiting a URL.
func (a *Admin) ActivationCodeIsValid(code string) (bool, error) {
	if a == nil || a.store == nil {
		return false, fmt.Errorf("pkg/embed: admin is not configured")
	}
	if strings.TrimSpace(code) == "" {
		return false, nil
	}
	_, ok, err := a.store.PeekPairingCodeUserID(code)
	if err != nil {
		return false, fmt.Errorf("pkg/embed: peek activation code: %w", err)
	}
	return ok, nil
}

// MintActivationCodeForUser mints an activation code bound to an EXISTING account,
// so an invited person can sign in to their own data rather than the operator's.
//
// MintActivationCode is the owner's shortcut and always binds to OwnerAccountID —
// deliberately, so every device the operator activates lands in one dataset. That
// is exactly wrong for a second person, and without this function CreateUser was
// useless: it could make an account nobody could ever sign in to.
//
// The account must already exist. A code minted for a typo'd id would be
// redeemable — RedeemPairingCode resolves whatever user the code names — and would
// silently create a session for an account with no data and no owner, which is a
// worse outcome than an error.
func (a *Admin) MintActivationCodeForUser(userID string) (code string, expiresAt time.Time, err error) {
	if a == nil || a.store == nil {
		return "", time.Time{}, fmt.Errorf("pkg/embed: admin is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", time.Time{}, fmt.Errorf("pkg/embed: user id is required")
	}
	if _, ok, err := a.store.GetUserByID(userID); err != nil {
		return "", time.Time{}, fmt.Errorf("pkg/embed: look up account: %w", err)
	} else if !ok {
		return "", time.Time{}, fmt.Errorf("pkg/embed: no such account %q", userID)
	}
	code, expiresAt, err = a.store.MintPairingCode(userID, time.Now().UTC())
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

// CreateUser adds an account the operator invites by hand, without any device
// having asked for one. It has a username and a role but NO password: the person
// gets in with an activation code and sets their own credential afterwards, so a
// password never has to travel from the operator to them out of band.
//
// The generated id is random, unlike OwnerAccountID: this is one of "a small,
// admin-invited set of DISTINCT people", and a guessable id for someone else's
// account is a bad idea even when the id alone grants nothing.
func (a *Admin) CreateUser(username, role string) (userID string, err error) {
	if a == nil || a.store == nil {
		return "", fmt.Errorf("pkg/embed: admin is not configured")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return "", fmt.Errorf("pkg/embed: username is required")
	}
	if role == "" {
		role = RoleMember
	}
	if !server.ValidRole(role) {
		return "", fmt.Errorf("pkg/embed: unknown role %q", role)
	}
	id, err := newDeviceUserID()
	if err != nil {
		return "", fmt.Errorf("pkg/embed: generate account id: %w", err)
	}
	now := time.Now().UTC()
	if err := a.store.UpsertUser(server.User{ID: id, Provider: "device", Subject: id, CreatedAt: now}); err != nil {
		return "", fmt.Errorf("pkg/embed: create account: %w", err)
	}
	if err := a.store.SetUsername(id, username); err != nil {
		return "", fmt.Errorf("pkg/embed: set username: %w", err)
	}
	if err := a.store.SetUserRole(id, role); err != nil {
		return "", fmt.Errorf("pkg/embed: set role: %w", err)
	}
	return id, nil
}

// UpdateUser changes an account's username and/or role. A blank field is left
// alone, so a caller can change one without having to restate the other.
//
// Demoting the OWNER account is refused: it is the account every activation code
// binds to, and a deployment whose owner is a viewer has no way back in through
// its own console.
func (a *Admin) UpdateUser(userID, username, role string) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("pkg/embed: admin is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("pkg/embed: user id is required")
	}
	if role = strings.TrimSpace(role); role != "" {
		if !server.ValidRole(role) {
			return fmt.Errorf("pkg/embed: unknown role %q", role)
		}
		if userID == OwnerAccountID && role != RoleOwner {
			return fmt.Errorf("pkg/embed: the owner account cannot be demoted")
		}
		if err := a.store.SetUserRole(userID, role); err != nil {
			return fmt.Errorf("pkg/embed: set role: %w", err)
		}
	}
	if username = strings.TrimSpace(username); username != "" {
		if err := a.store.SetUsername(userID, username); err != nil {
			return fmt.Errorf("pkg/embed: set username: %w", err)
		}
	}
	return nil
}

// SetSuspended freezes or restores an account. A suspended account keeps every
// byte of its data and can still be listed, inspected and un-suspended — it simply
// cannot push (see SyncService.requireWriter). Suspending revokes live sessions
// too, so it takes effect on devices that are already signed in rather than only
// at their next sign-in.
//
// Refuses to suspend the owner, for the same reason UpdateUser refuses to demote
// it: locking the operator out of their own deployment through their own console.
func (a *Admin) SetSuspended(userID string, suspended bool) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("pkg/embed: admin is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("pkg/embed: user id is required")
	}
	if suspended && userID == OwnerAccountID {
		return fmt.Errorf("pkg/embed: the owner account cannot be suspended")
	}
	now := time.Now().UTC()
	if err := a.store.SetUserSuspended(userID, suspended, now); err != nil {
		return fmt.Errorf("pkg/embed: set suspended: %w", err)
	}
	if suspended {
		// Suspension has to bite devices that are ALREADY signed in: a device
		// holding a refresh token would otherwise keep rotating itself fresh
		// access indefinitely, and the suspension would be advisory.
		if err := a.store.RevokeRefreshSessionsForUser(userID, now); err != nil {
			return fmt.Errorf("pkg/embed: revoke sessions: %w", err)
		}
	}
	return nil
}

// ResetCredentials clears an account's password and logs it out everywhere,
// leaving the account and all of its data intact. The person gets back in with a
// fresh activation code and sets a new password.
//
// Both halves are required and neither is sufficient: clearing the password alone
// stops only the NEXT sign-in, while a device already holding a refresh token
// would keep rotating itself access indefinitely; revoking sessions alone leaves
// the old password working.
func (a *Admin) ResetCredentials(userID string) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("pkg/embed: admin is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("pkg/embed: user id is required")
	}
	if err := a.store.ClearLocalPassword(userID); err != nil {
		return fmt.Errorf("pkg/embed: clear password: %w", err)
	}
	if err := a.store.RevokeRefreshSessionsForUser(userID, time.Now().UTC()); err != nil {
		return fmt.Errorf("pkg/embed: revoke sessions: %w", err)
	}
	return nil
}

// DeleteUser permanently purges an enrolled account and everything it owns:
// workspaces, dataset snapshots and their history, artifact-blob links, AI keys,
// usage rows, subscriptions, idempotency keys, and — critically — every refresh
// token, so a deleted account cannot be resurrected through a session that
// outlived it. It then sweeps blob files no surviving workspace references, since
// purging the rows alone would leave the bytes on disk.
//
// Returns deleted=false (with no error) when no such account exists, so a
// double-submit or a stale admin console reads as "already gone" rather than a
// failure. This is NOT reversible and there is no tombstone: the account is
// erased, and re-admitting that person means minting them a new activation code
// and starting from an empty dataset.
//
// The audit entry is written BEFORE the purge (matching handleAccountDelete) so
// the record exists even if the delete itself fails partway. audit_events has no
// foreign key to users and is never purged, so the trail survives the account.
// The actor is recorded as "embed:admin" rather than a user id: pkg/embed.Admin is
// called directly from the embedding host's own Go code and there is no CashFlux
// session behind it — the real authorization happened at the host's admin login,
// which CashFlux cannot see.
func (a *Admin) DeleteUser(userID string) (deleted bool, err error) {
	if a == nil || a.store == nil {
		return false, fmt.Errorf("pkg/embed: admin is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, fmt.Errorf("pkg/embed: user id is required")
	}
	if _, auditErr := a.store.AppendAuditEvent(server.AuditEvent{
		Timestamp:  time.Now().UTC(),
		ActorID:    "embed:admin",
		Action:     "admin.user.delete",
		TargetType: "user",
		TargetID:   userID,
	}); auditErr != nil {
		return false, fmt.Errorf("pkg/embed: audit delete: %w", auditErr)
	}
	deleted, err = a.store.DeleteAccount(userID)
	if err != nil {
		return false, fmt.Errorf("pkg/embed: delete account: %w", err)
	}
	if !deleted {
		return false, nil
	}
	if _, err := a.store.SweepUnreferencedBlobs(a.blobRoot); err != nil {
		// The account IS gone at this point — reporting a failure would be a lie
		// that invites a retry against an id that no longer exists. Surface it as a
		// deletion that succeeded with cleanup still owed; the next delete (or the
		// periodic sweep) reclaims the same orphans.
		return true, fmt.Errorf("pkg/embed: account deleted, blob sweep failed: %w", err)
	}
	return true, nil
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
	// usage rows for the current calendar month (UTC). NOTE: usage rows are
	// written only by the metered AI proxy — nothing in the sync path touches
	// them — so this reads 0 for an account that syncs constantly and buys no
	// AI. It is a billing figure, not a liveness one; the three fields below
	// are what answer "did this person's data actually arrive?".
	RequestsThisMonth int64
	// Workspaces is how many live (non-tombstoned) workspaces the account owns.
	Workspaces int
	// DatasetBytes is the total size of those workspaces' dataset snapshots.
	DatasetBytes int64
	// LastSyncedAt is the most recent push across them. Zero means this account
	// has never synced anything — which callers should render as "never synced"
	// rather than as a date.
	LastSyncedAt time.Time
	// Role is what this account may do: RoleOwner / RoleMember / RoleViewer.
	Role string
	// Username is its local-auth login name, empty for an account that only ever
	// signs in with an activation code.
	Username string
	// Suspended accounts keep all their data but cannot push.
	Suspended bool
}

// Roles, re-exported so an embedding host can present and validate them without
// importing internal/server.
const (
	RoleOwner  = server.RoleOwner
	RoleMember = server.RoleMember
	RoleViewer = server.RoleViewer
)

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
		workspaces, datasetBytes, lastSyncedAt, err := a.store.UserSyncSummary(r.ID)
		if err != nil {
			return nil, fmt.Errorf("pkg/embed: sync summary for %s: %w", r.ID, err)
		}
		out = append(out, User{
			ID:                 r.ID,
			Provider:           r.Provider,
			Email:              r.Email,
			CreatedAt:          createdAt,
			SubscriptionPlan:   r.SubscriptionPlan,
			SubscriptionStatus: r.SubscriptionStatus,
			RequestsThisMonth:  requests,
			Workspaces:         workspaces,
			DatasetBytes:       datasetBytes,
			LastSyncedAt:       lastSyncedAt,
			Role:               r.Role,
			Username:           r.Username,
			Suspended:          r.Suspended,
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

// StorageStats reports storage usage for the admin console's storage panel: the
// SQLite database's own size, the total size of every stored artifact blob, and
// the total size of every dataset snapshot.
//
// snapshotBytes exists because the other two barely move: a 17KB dataset landing
// in a 284KB database is lost in the page-count rounding, and an account with no
// attachments never moves the blob figure off zero — so a panel showing only
// those two reads as "nothing is happening" on a server that is syncing fine.
// snapshotBytes is the figure that changes every time someone pushes.
func (a *Admin) StorageStats() (dbBytes, blobBytes, snapshotBytes int64, err error) {
	if a == nil || a.store == nil {
		return 0, 0, 0, fmt.Errorf("pkg/embed: admin is not configured")
	}
	dbBytes, blobBytes, err = a.store.StorageStats()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("pkg/embed: storage stats: %w", err)
	}
	snapshotBytes, err = a.store.SnapshotBytes()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("pkg/embed: snapshot bytes: %w", err)
	}
	return dbBytes, blobBytes, snapshotBytes, nil
}
