// SPDX-License-Identifier: MIT

// Native unit tests for the Custom Sync entitlement-gate reason mapping
// (customsync_gate.go). No build tag: runs with `go test ./internal/app/...`
// on any platform without a browser or WASM runtime.
package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
)

func TestCustomSyncGateMessageKey(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{"admin suspended", backendrpc.EntitlementReasonAdminSuspended, "customSync.gatedAdminSuspended"},
		{"plan tier insufficient", backendrpc.EntitlementReasonPlanTierInsufficient, "customSync.gatedPlanTier"},
		{"billing lapsed", backendrpc.EntitlementReasonBillingLapsed, "customSync.gatedBillingLapsed"},
		{"unrecognized reason falls back to generic", "some-future-reason", "customSync.gatedGeneric"},
		{"empty reason falls back to generic", "", "customSync.gatedGeneric"},
		{"active/ok reason (should never route to the gate, but must not panic)", backendrpc.EntitlementReasonOK, "customSync.gatedGeneric"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := customSyncGateMessageKey(tc.reason); got != tc.want {
				t.Errorf("customSyncGateMessageKey(%q) = %q, want %q", tc.reason, got, tc.want)
			}
		})
	}
}
