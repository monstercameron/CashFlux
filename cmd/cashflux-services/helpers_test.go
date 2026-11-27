// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

func TestEndpointPoolKeyIgnoresTrailingSlashAndWhitespace(t *testing.T) {
	a := endpointPoolKey(" https://sync.example.com/ ")
	b := endpointPoolKey("https://sync.example.com")
	if a != b || a != "https://sync.example.com" {
		t.Fatalf("pool keys = %q vs %q", a, b)
	}
}

func TestConnReusableEvictsDeadAndTransientFailureStates(t *testing.T) {
	cases := []struct {
		state connectivity.State
		want  bool
	}{
		{connectivity.Idle, true},
		{connectivity.Connecting, true},
		{connectivity.Ready, true},
		{connectivity.TransientFailure, false},
		{connectivity.Shutdown, false},
	}
	for _, tc := range cases {
		if got := connReusable(tc.state); got != tc.want {
			t.Fatalf("connReusable(%v) = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func TestRPCErrorPreservesStructuredErrorInfo(t *testing.T) {
	st, err := status.New(codes.PermissionDenied, "billing lapsed").WithDetails(&errdetails.ErrorInfo{
		Reason:   "BILLING_LAPSED",
		Domain:   "cashflux.app",
		Metadata: map[string]string{"plan": "pro"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rpcError(st.Err(), false)
	if got.Code != "PermissionDenied" || got.Reason != "BILLING_LAPSED" || got.Metadata["plan"] != "pro" {
		t.Fatalf("rpcError = %+v", got)
	}
	if !rpcprotocol.HasReason(got, "BILLING_LAPSED") {
		t.Fatal("HasReason did not match BILLING_LAPSED")
	}
}
