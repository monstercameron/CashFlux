// SPDX-License-Identifier: MIT

// Native helper for the "Custom Sync" pre-flight entitlement check (TODOS.md
// C431). No build tag: this file has no syscall/js so it unit-tests on native
// Go; the wasm view code (customsync.go) resolves the returned i18n key via
// uistate.T.
package app

import "github.com/monstercameron/CashFlux/internal/backendrpc"

// customSyncGateMessageKey maps a GetEntitlement rejection reason
// (backendrpc.EntitlementReasonXxx) to the i18n key for its upgrade-prompt
// copy, falling back to a generic message for a reason string the client
// doesn't recognize (e.g. a future server-side reason this build predates).
func customSyncGateMessageKey(reason string) string {
	switch reason {
	case backendrpc.EntitlementReasonAdminSuspended:
		return "customSync.gatedAdminSuspended"
	case backendrpc.EntitlementReasonPlanTierInsufficient:
		return "customSync.gatedPlanTier"
	case backendrpc.EntitlementReasonBillingLapsed:
		return "customSync.gatedBillingLapsed"
	default:
		return "customSync.gatedGeneric"
	}
}
