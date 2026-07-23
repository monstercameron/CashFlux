// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/status"
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
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: endpoint, Token: token})
		if err != nil {
			onError("Couldn't reach the backend server.")
			return
		}
		defer conn.Close()
		var out backendrpc.GetEntitlementResponse
		err = conn.Invoke(ctx, backendrpc.MethodAccountGetEntitlement, backendrpc.GetEntitlementRequest{}, &out, backendrpc.JSONCallOptions()...)
		if err == nil {
			onResult(out)
			return
		}
		if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
			onError(st.Message())
			return
		}
		onError(err.Error())
	}()
}
