// SPDX-License-Identifier: MIT

// Package grpccodec registers and selects CashFlux's strict JSON gRPC codec.
//
// It is intentionally separate from backendrpc so browser render code can use
// the pure request/response contracts without linking google.golang.org/grpc.
// Import this package only from servers, native clients, or services.wasm.
package grpccodec

import (
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

func init() {
	encoding.RegisterCodec(backendrpc.JSONCodec{})
}

// CallOptions selects CashFlux's strict JSON gRPC codec.
func CallOptions() []grpc.CallOption {
	return []grpc.CallOption{grpc.ForceCodec(backendrpc.JSONCodec{})}
}
