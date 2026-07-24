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
