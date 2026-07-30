// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
)

// checkCloudEntitlement calls AccountService.GetEntitlement (TODOS.md C431) —
// the pre-flight check the "Custom Sync" enrollment toggle must call BEFORE
// any enrollment RPC, so a lapsed/suspended/tier-insufficient account sees
// why it can't enroll instead of a raw RPC failure partway through.
//
// NOTE for the lane that owns the Custom Sync toggle UI: this file only adds
// the client-side call: wire onResult/onError into whatever component renders
// the toggle (see internal/app/syncpage.go or wherever "Custom Sync" lives),
// calling checkCloudEntitlement before starting enrollment, and branching the
// UI on resp.Active / resp.Reason (backendrpc.EntitlementReasonXxx).
func checkCloudEntitlement(endpoint, token string, onResult func(backendrpc.GetEntitlementResponse), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if token == "" {
		onError("Sign in before checking cloud sync eligibility.")
		return
	}
	go func() {
		ctx := context.Background()
		var out backendrpc.GetEntitlementResponse
		err := invokeWorkerRPC(ctx, endpoint, token, backendrpc.MethodAccountGetEntitlement, backendrpc.GetEntitlementRequest{}, &out)
		if err == nil {
			onResult(out)
			return
		}
		var rpcErr *rpcprotocol.Error
		if errors.As(err, &rpcErr) && strings.TrimSpace(rpcErr.Message) != "" {
			onError(rpcErr.Message)
			return
		}
		onError(err.Error())
	}()
}
