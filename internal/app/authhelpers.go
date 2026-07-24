// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"syscall/js"

	"google.golang.org/grpc/status"
)

// customSyncDeviceLabel returns a short, human-readable label for this
// browser/device — shown back to the user in the device list (ListDevices).
// It reads navigator.platform/userAgentData when available and falls back to
// a generic label off the browser, matching the js.Global() interop style
// backend.go already uses (appOrigin) rather than adding a new pattern.
func customSyncDeviceLabel() string {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return "This device"
	}
	if uaData := nav.Get("userAgentData"); uaData.Truthy() {
		if platform := uaData.Get("platform"); platform.Truthy() && strings.TrimSpace(platform.String()) != "" {
			return strings.TrimSpace(platform.String()) + " browser"
		}
	}
	if platform := nav.Get("platform"); platform.Truthy() && strings.TrimSpace(platform.String()) != "" {
		return strings.TrimSpace(platform.String()) + " browser"
	}
	return "This device"
}

// newIdempotencyKey returns a fresh random hex token for an AuthService
// request's IdempotencyKey (TODOS.md C443): distinct per attempt, reused
// across retries of the same logical action.
func newIdempotencyKey() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing would mean no source of randomness is available at
		// all (fatal for the whole app, not just this flow); fall back to a
		// fixed marker rather than panicking mid-render.
		return "idempotency-key-unavailable"
	}
	return hex.EncodeToString(buf)
}

// customSyncErrorMessage extracts a gRPC status message for display, falling
// back to fallback when err carries none — mirroring the status.FromError
// pattern already used in backend.go's uploadOpenAIKeyToBackend.
func customSyncErrorMessage(err error, fallback string) string {
	if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
		return st.Message()
	}
	if err != nil {
		return fmt.Sprintf("%s: %v", fallback, err)
	}
	return fallback
}
