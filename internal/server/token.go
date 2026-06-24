// SPDX-License-Identifier: MIT

package server

import (
	"crypto/sha256"
	"encoding/hex"
)

type AccessToken struct {
	Token  string
	SHA256 string
}

func GenerateAccessToken() (AccessToken, error) {
	token, err := randomURLToken(32)
	if err != nil {
		return AccessToken{}, err
	}
	sum := sha256.Sum256([]byte(token))
	return AccessToken{Token: token, SHA256: hex.EncodeToString(sum[:])}, nil
}
