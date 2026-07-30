// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package backendrpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

func init() {
	encoding.RegisterCodec(JSONCodec{})
}

// JSONCallOptions selects CashFlux's strict JSON gRPC codec.
//
// Native server, integration-test, and load-generator code keeps this
// compatibility helper. Browser code deliberately cannot see it: main.wasm
// must not link gRPC merely by importing the pure request/response contracts.
func JSONCallOptions() []grpc.CallOption {
	return []grpc.CallOption{grpc.ForceCodec(JSONCodec{})}
}
