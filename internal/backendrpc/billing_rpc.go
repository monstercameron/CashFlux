// SPDX-License-Identifier: MIT

package backendrpc

// MethodBillingCreateCheckoutSession is the BillingService.CreateCheckoutSession
// full method name (TODOS.md C440 — replaces the REST createBillingSession call
// from internal/app/backend.go). It must stay in lockstep with the generated
// backendrpcpb.BillingService_CreateCheckoutSession_FullMethodName constant;
// proto_contract_test.go asserts the correspondence.
const MethodBillingCreateCheckoutSession = "/cashflux.v1.BillingService/CreateCheckoutSession"

// CreateCheckoutSessionRequest asks the server to start a hosted checkout
// session with a payment provider for the given plan.
type CreateCheckoutSessionRequest struct {
	Plan     string `json:"plan"`
	Provider string `json:"provider,omitempty"`
}

// CreateCheckoutSessionResponse carries the URL the client redirects the
// browser to via window.location — the one network hop this migration cannot
// move onto gRPC (see proto/README.md and TODOS.md C442).
type CreateCheckoutSessionResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
}
