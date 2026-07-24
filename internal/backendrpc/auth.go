// SPDX-License-Identifier: MIT

package backendrpc

// Method name constants for AuthService — the shared device/session identity
// core (TODOS.md C418). These must stay in lockstep with the generated
// backendrpcpb.AuthService_*_FullMethodName constants; proto_contract_test.go
// asserts the correspondence.
const (
	MethodAuthEnroll            = "/cashflux.v1.AuthService/Enroll"
	MethodAuthRedeemPairingCode = "/cashflux.v1.AuthService/RedeemPairingCode"
	MethodAuthRegister          = "/cashflux.v1.AuthService/Register"
	MethodAuthLogin             = "/cashflux.v1.AuthService/Login"
	MethodAuthRefreshToken      = "/cashflux.v1.AuthService/RefreshToken"
	MethodAuthLogout            = "/cashflux.v1.AuthService/Logout"
	MethodAuthListDevices       = "/cashflux.v1.AuthService/ListDevices"
	MethodAuthRevokeDevice      = "/cashflux.v1.AuthService/RevokeDevice"

	// Admin-approved device pairing bootstrap (2026-07-24 unification): a
	// device with no working credentials requests pairing, an admin approves
	// it from the portfolio console, and the device exchanges the resulting
	// pairing code for a session then sets a password — see TODOS.md C454.
	MethodAuthRequestDevicePairing = "/cashflux.v1.AuthService/RequestDevicePairing"
	MethodAuthWatchPairingStatus   = "/cashflux.v1.AuthService/WatchPairingStatus"
	MethodAuthCancelDevicePairing  = "/cashflux.v1.AuthService/CancelDevicePairing"
	MethodAuthSetPassword          = "/cashflux.v1.AuthService/SetPassword"
)

// EnrollRequest starts a brand-new, never-before-seen device/account pairing.
// Lane C/D own the body; this pass only carries the shape.
type EnrollRequest struct {
	DeviceLabel string `json:"deviceLabel,omitempty"`
}

// TokenPairResponse is the common success shape returned by every AuthService
// method that mints or rotates a session: a fresh access/refresh pair plus the
// device identity the refresh token is now scoped to.
type TokenPairResponse struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresInSeconds int64  `json:"expiresInSeconds"`
	DeviceID         string `json:"deviceId,omitempty"`
	// RecoveryCode is set ONLY by Register's response: a one-time account
	// recovery code the caller must save now, since password accounts have
	// no email/SMS-backed recovery path (TODOS.md C422). Every other method
	// leaves this empty.
	RecoveryCode string `json:"recoveryCode,omitempty"`
}

// RedeemPairingCodeRequest links a new device to an existing account using a
// short-lived, single-use code minted by the portal (TODOS.md C421).
type RedeemPairingCodeRequest struct {
	PairingCode    string `json:"pairingCode"`
	DeviceLabel    string `json:"deviceLabel,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// RegisterRequest creates a username/password account for users who won't
// share a phone number (TODOS.md C422).
type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceLabel string `json:"deviceLabel,omitempty"`
}

// LoginRequest authenticates an existing username/password account.
type LoginRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	DeviceLabel    string `json:"deviceLabel,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// RefreshTokenRequest rotates a refresh token for a new access/refresh pair.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// LogoutRequest revokes the session family the given refresh token belongs to.
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// LogoutResponse reports whether a session family was revoked.
type LogoutResponse struct {
	Revoked bool `json:"revoked"`
}

// ListDevicesRequest lists the caller's active device sessions.
type ListDevicesRequest struct{}

// DeviceSession is one entry in a user's per-device session list.
type DeviceSession struct {
	FamilyID    string `json:"familyId"`
	DeviceLabel string `json:"deviceLabel,omitempty"`
	ExpiresAt   string `json:"expiresAt"`
	Current     bool   `json:"current,omitempty"`
}

// ListDevicesResponse is the caller's active device/session list.
type ListDevicesResponse struct {
	Devices []DeviceSession `json:"devices"`
}

// RevokeDeviceRequest revokes one session family by id (signs that device out).
type RevokeDeviceRequest struct {
	FamilyID string `json:"familyId"`
}

// RevokeDeviceResponse reports whether the session family was revoked.
type RevokeDeviceResponse struct {
	Revoked bool `json:"revoked"`
}

// RequestDevicePairingRequest starts a pending device-pairing request: an
// unauthenticated device asks to be paired, and waits for an admin to
// approve or reject it via WatchPairingStatus (TODOS.md C454).
type RequestDevicePairingRequest struct {
	DeviceLabel string `json:"deviceLabel,omitempty"`
}

// RequestDevicePairingResponse carries the opaque device id the caller
// watches (WatchPairingStatus) and can cancel (CancelDevicePairing).
// Possession of this id is the only credential this flow requires — it is
// never guessable and never displayed to anyone but the requesting device.
type RequestDevicePairingResponse struct {
	DeviceID string `json:"deviceId"`
}

// WatchPairingStatusRequest opens a one-shot watch on a pending device
// request: the stream delivers exactly one PairingStatusEvent (approved,
// rejected, or expired) and then closes.
type WatchPairingStatusRequest struct {
	DeviceID string `json:"deviceId"`
}

// PairingStatusEvent is the single event WatchPairingStatus's stream
// delivers before closing.
type PairingStatusEvent struct {
	// Status is one of "approved", "rejected", or "expired".
	Status string `json:"status"`
	// PairingCode is set only when Status is "approved" — the caller
	// exchanges it via RedeemPairingCode for a real session.
	PairingCode string `json:"pairingCode,omitempty"`
}

// CancelDevicePairingRequest lets the requesting device withdraw its own
// pending request (e.g. the user declines, or the displayed pairing code
// doesn't match what the admin console shows).
type CancelDevicePairingRequest struct {
	DeviceID string `json:"deviceId"`
}

// CancelDevicePairingResponse reports whether a pending request was canceled.
type CancelDevicePairingResponse struct {
	Canceled bool `json:"canceled"`
}

// SetPasswordRequest attaches a username/password to the CALLER's own
// authenticated session (AuthUserFromContext) — never a new account.
// Authenticated-only; see authServer.SetPassword's doc comment for why this
// is a distinct RPC from Register.
type SetPasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SetPasswordResponse is empty — success is the absence of an error.
type SetPasswordResponse struct{}
