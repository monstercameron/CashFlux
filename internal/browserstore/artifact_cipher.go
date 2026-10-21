// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// LocalArtifactCipher uses the same workspace key and authenticated scope as
// protected browser records. Artifact IDs enter the authenticated record key,
// so moving a sealed file to a different ID or account cannot open it.
type LocalArtifactCipher struct{}

func (LocalArtifactCipher) SealArtifact(id string, plaintext []byte) ([]byte, error) {
	if secureKeys == nil {
		return nil, atrest.ErrLocked
	}
	env, err := secureKeys.Seal("artifact:"+id, plaintext)
	if err != nil {
		return nil, err
	}
	return env.Encode()
}

func (LocalArtifactCipher) OpenArtifact(id string, sealed []byte) ([]byte, error) {
	if secureKeys == nil {
		return nil, atrest.ErrLocked
	}
	var env atrest.Envelope
	if err := json.Unmarshal(sealed, &env); err != nil {
		return nil, fmt.Errorf("decode encrypted artifact: %w", err)
	}
	return secureKeys.Open("artifact:"+id, env)
}
