// SPDX-License-Identifier: MIT

package rpcprotocol

import (
	"errors"
	"testing"
)

func TestValidateRequest(t *testing.T) {
	valid := Request{
		Version:  Version,
		Kind:     KindUnary,
		ID:       "op-1",
		Endpoint: "https://cashflux.test",
		Token:    "token",
		Method:   "/cashflux.v1.SyncService/ListWorkspaces",
	}
	tests := []struct {
		name string
		req  Request
		ok   bool
	}{
		{name: "valid unary", req: valid, ok: true},
		{name: "wrong version", req: withRequest(valid, func(r *Request) { r.Version++ })},
		{name: "missing id", req: withRequest(valid, func(r *Request) { r.ID = "" })},
		{name: "missing endpoint", req: withRequest(valid, func(r *Request) { r.Endpoint = "" })},
		{name: "missing token", req: withRequest(valid, func(r *Request) { r.Token = "" })},
		{name: "missing method", req: withRequest(valid, func(r *Request) { r.Method = "" })},
		{name: "cancel only needs id", req: Request{Version: Version, Kind: KindStreamCancel, ID: "op-1"}, ok: true},
		{name: "unknown kind", req: withRequest(valid, func(r *Request) { r.Kind = "surprise" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateRequest() error = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func withRequest(in Request, change func(*Request)) Request {
	change(&in)
	return in
}

func TestValidateResponse(t *testing.T) {
	if err := ValidateResponse(Response{Version: Version, Kind: KindReady}); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := ValidateResponse(Response{Version: Version, Kind: KindFailure, ID: "op"}); err == nil {
		t.Fatal("failure without error was accepted")
	}
	if err := ValidateResponse(Response{Version: Version + 1, Kind: KindReady}); err == nil {
		t.Fatal("version skew was accepted")
	}
}

func TestStreamCursorRejectsGapsAndDuplicateTermination(t *testing.T) {
	var cursor StreamCursor
	if err := cursor.AcceptEvent(0); err != nil {
		t.Fatalf("event 0: %v", err)
	}
	if err := cursor.AcceptEvent(2); err == nil {
		t.Fatal("sequence gap was accepted")
	}
	if err := cursor.AcceptEvent(1); err != nil {
		t.Fatalf("event 1: %v", err)
	}
	if err := cursor.Terminate(); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if err := cursor.Terminate(); err == nil {
		t.Fatal("duplicate termination was accepted")
	}
	if err := cursor.AcceptEvent(2); err == nil {
		t.Fatal("post-terminal event was accepted")
	}
}

func TestErrorClassification(t *testing.T) {
	err := &Error{Code: "Unauthenticated", Message: "sign in again"}
	if !IsCode(err, "unauthenticated") {
		t.Fatal("case-insensitive code classification failed")
	}
	if !errors.Is(err, err) {
		t.Fatal("error must retain normal identity semantics")
	}
}
