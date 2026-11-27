// SPDX-License-Identifier: MIT

package main

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

// endpointPoolKey normalizes a backend URL into a single per-endpoint pool key.
// Connections are deliberately NOT keyed by rotating access token so 15-minute
// token refreshes reuse the existing WebSocket tunnel instead of leaking
// connections until GRPCMaxConnectionsPerClient is exhausted.
func endpointPoolKey(endpoint string) string {
	return strings.TrimRight(strings.TrimSpace(endpoint), "/")
}

// connReusable reports whether a pooled gRPC ClientConn is healthy enough to
// reuse rather than closing and redialing.
func connReusable(state connectivity.State) bool {
	return state != connectivity.Shutdown && state != connectivity.TransientFailure
}

// rpcError converts a gRPC error into a transport-neutral rpcprotocol.Error,
// preserving structured ErrorInfo reason/metadata instead of stripping details.
func rpcError(err error, ambiguous bool) *rpcprotocol.Error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &rpcprotocol.Error{
			Code:           "Unavailable",
			Message:        err.Error(),
			MayHaveApplied: ambiguous,
		}
	}
	out := &rpcprotocol.Error{
		Code:           st.Code().String(),
		Message:        st.Message(),
		MayHaveApplied: ambiguous,
	}
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info != nil {
			out.Reason = strings.TrimSpace(info.GetReason())
			if len(info.GetMetadata()) > 0 {
				out.Metadata = make(map[string]string, len(info.GetMetadata()))
				for k, v := range info.GetMetadata() {
					out.Metadata[k] = v
				}
			}
			break
		}
	}
	return out
}

func mayHaveApplied(err error) bool {
	switch status.Code(err) {
	case codes.Canceled, codes.DeadlineExceeded, codes.Unknown, codes.Internal, codes.Unavailable:
		return true
	default:
		return false
	}
}
