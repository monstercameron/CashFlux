// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
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
		// crypto/rand failing means no randomness is available at all. Returning
		// a FIXED marker here was worse than returning nothing: every client in
		// that state would send the same key, so two unrelated actions collide on
		// one idempotency record — and on ResetPassword the replacement recovery
		// code is derived from the key, making it identical for everyone. An
		// empty key is the honest answer: the server treats it as "no idempotency
		// requested" and simply processes the call, losing retry-dedup for one
		// action instead of sharing a secret across every device.
		return ""
	}
	return hex.EncodeToString(buf)
}

// customSyncErrorMessage extracts a gRPC status message for display, falling
// back to fallback when err carries none — mirroring the status.FromError
// pattern already used in backend.go's uploadOpenAIKeyToBackend.
func customSyncErrorMessage(err error, fallback string) string {
	var rpcErr *rpcprotocol.Error
	if errors.As(err, &rpcErr) && strings.TrimSpace(rpcErr.Message) != "" {
		return rpcErr.Message
	}
	if err != nil {
		return fmt.Sprintf("%s: %v", fallback, err)
	}
	return fallback
}
