// SPDX-License-Identifier: MIT

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestGenerateAccessTokenReturnsDigest(t *testing.T) {
	token, err := GenerateAccessToken()
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if len(token.Token) < 32 {
		t.Fatalf("token too short: %q", token.Token)
	}
	sum := sha256.Sum256([]byte(token.Token))
	if token.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("token sha256 = %q, want digest of token", token.SHA256)
	}
}
