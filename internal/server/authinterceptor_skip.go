// SPDX-License-Identifier: MIT

package server

import "github.com/monstercameron/CashFlux/internal/backendrpc"

// authInterceptorSkipMethods lists the AuthService full method names that run
// with neither an authenticated caller nor a cloud-entitlement check
// (TODOS.md C418-C422): a device enrolling, redeeming a pairing code,
// registering, or logging in cannot present a bearer token yet, and
// refreshing/logging out must succeed even for a billing-gated account so the
// token lifecycle itself never depends on entitlement (see
// internal/server/entitlements.go — entitlement is checked per-Sync/Blob-call,
// never on AuthService). ListDevices and RevokeDevice are deliberately NOT
// listed here: those manage an already-authenticated session and require a
// caller identity, same as every other service. RequestDevicePairing/
// WatchPairingStatus/CancelDevicePairing (TODOS.md C454) are the same shape
// as RedeemPairingCode — a brand-new device with no session yet cannot
// present a bearer token — but SetPassword is deliberately NOT listed here:
// it authenticates the CALLER's existing session (AuthUserFromContext) and
// must keep going through the normal bearer-token check.
var authInterceptorSkipMethods = map[string]bool{
	backendrpc.MethodAuthEnroll:               true,
	backendrpc.MethodAuthRedeemPairingCode:    true,
	backendrpc.MethodAuthRegister:             true,
	backendrpc.MethodAuthLogin:                true,
	backendrpc.MethodAuthRefreshToken:         true,
	backendrpc.MethodAuthLogout:               true,
	backendrpc.MethodAuthRequestDevicePairing: true,
	backendrpc.MethodAuthWatchPairingStatus:   true,
	backendrpc.MethodAuthCancelDevicePairing:  true,
}

// skipsAuthAndEntitlement reports whether fullMethod must bypass both
// AuthUnaryInterceptor/AuthStreamInterceptor's bearer-token check and
// CloudEntitlementUnaryInterceptor/CloudEntitlementStreamInterceptor's
// entitlement check.
func skipsAuthAndEntitlement(fullMethod string) bool {
	return authInterceptorSkipMethods[fullMethod]
}

// entitlementOnlySkipMethods lists the additional full method(s) that must
// bypass ONLY the entitlement check, while still requiring normal
// authentication (unlike authInterceptorSkipMethods, which bypasses both).
//
// AccountService.GetEntitlement (TODOS.md C431) is the pre-flight check a
// client calls to learn WHY its cloud entitlement is inactive — its own doc
// comment says as much ("this is the pre-flight check ... it never itself
// rejects on an inactive entitlement — Active=false with a Reason is the
// whole point of the call"). Before this map existed, GetEntitlement shared
// authInterceptorSkipMethods' verdict with every other interceptor, so it was
// entitlement-gated exactly like SyncService/BlobService: an inactive account
// calling GetEntitlement got a bare PermissionDenied from
// CloudEntitlementUnaryInterceptor before ever reaching the handler that was
// supposed to explain why — the one caller who most needs to observe
// Active:false could never see it. It must still require an authenticated
// caller (GetEntitlement reports one specific user's subscription state), so
// it stays OFF authInterceptorSkipMethods and AuthUnaryInterceptor keeps
// enforcing a valid bearer token for it as normal.
var entitlementOnlySkipMethods = map[string]bool{
	backendrpc.MethodAccountGetEntitlement: true,
}

// skipsEntitlementCheck reports whether fullMethod must bypass ONLY
// CloudEntitlementUnaryInterceptor/CloudEntitlementStreamInterceptor's
// entitlement check — either because it's already exempt from both checks
// (skipsAuthAndEntitlement), or because it's the entitlement pre-flight check
// itself (entitlementOnlySkipMethods).
func skipsEntitlementCheck(fullMethod string) bool {
	return authInterceptorSkipMethods[fullMethod] || entitlementOnlySkipMethods[fullMethod]
}
