// SPDX-License-Identifier: MIT

package backendrpc

// MethodAccountGetEntitlement is the AccountService.GetEntitlement full method
// name (TODOS.md C431). It must stay in lockstep with the generated
// backendrpcpb.AccountService_GetEntitlement_FullMethodName constant;
// proto_contract_test.go asserts the correspondence.
const MethodAccountGetEntitlement = "/cashflux.v1.AccountService/GetEntitlement"

// GetEntitlementRequest asks whether the caller's account may use cloud sync
// right now, and why not if it can't.
type GetEntitlementRequest struct{}

// GetEntitlementResponse is a thin read of the same entitlement logic
// IsCloudActive already backs (see internal/server/entitlements.go) plus the
// storage-quota figures a Custom Sync toggle screen needs before enrolling.
type GetEntitlementResponse struct {
	Active     bool   `json:"active"`
	Reason     string `json:"reason"`
	BytesUsed  int64  `json:"bytesUsed"`
	BytesLimit int64  `json:"bytesLimit"`
	PlanTier   string `json:"planTier,omitempty"`
}

// Entitlement reason strings, aligned with the existing ErrorReason-style
// taxonomy (internal/server/errors.go) so a client branches on a stable value.
const (
	EntitlementReasonOK                   = "ok"
	EntitlementReasonBillingLapsed        = "billing_lapsed"
	EntitlementReasonAdminSuspended       = "admin_suspended"
	EntitlementReasonPlanTierInsufficient = "plan_tier_insufficient"
)
