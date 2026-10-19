// SPDX-License-Identifier: MIT

// Package aicred defines the device-local OpenAI credential record and its
// migration decisions. It contains no browser or cryptography dependencies.
package aicred

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ProviderOpenAI identifies the OpenAI API credential provider.
const ProviderOpenAI = "openai"

// Credential is a device-global OpenAI credential. Exactly one of Key or
// Envelope may be set; Envelope contains encrypted credential bytes encoded by
// the app's existing dataset encryption format.
type Credential struct {
	Version  int    `json:"v"`
	Provider string `json:"provider"`
	Key      string `json:"key,omitempty"`
	Envelope string `json:"envelope,omitempty"`
	Remember bool   `json:"remember"`
}

// New creates a normalized credential record for a plaintext or encrypted key.
func New(key, envelope string, remember bool) (Credential, error) {
	key, envelope = strings.TrimSpace(key), strings.TrimSpace(envelope)
	if (key == "") == (envelope == "") {
		return Credential{}, errors.New("aicred: exactly one of key or envelope is required")
	}
	if !remember {
		return Credential{}, errors.New("aicred: a stored credential must be remembered")
	}
	return Credential{Version: 1, Provider: ProviderOpenAI, Key: key, Envelope: envelope, Remember: true}, nil
}

// Marshal validates and encodes a credential record.
func Marshal(c Credential) ([]byte, error) {
	if c.Version != 1 || c.Provider != ProviderOpenAI || !c.Remember ||
		(strings.TrimSpace(c.Key) == "") == (strings.TrimSpace(c.Envelope) == "") {
		return nil, errors.New("aicred: invalid credential record")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("aicred: encode credential: %w", err)
	}
	return b, nil
}

// Parse validates and decodes a credential record, rejecting unknown fields.
func Parse(data []byte) (Credential, error) {
	var c Credential
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return Credential{}, fmt.Errorf("aicred: decode credential: %w", err)
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Credential{}, errors.New("aicred: trailing credential data")
		}
		return Credential{}, fmt.Errorf("aicred: trailing credential data: %w", err)
	}
	if _, err := Marshal(c); err != nil {
		return Credential{}, err
	}
	return c, nil
}

// MigrationKey chooses the key to adopt when no credential record exists.
// The dataset value has precedence over the legacy standalone entry.
func MigrationKey(datasetKey, legacyKey string, credentialExists bool) string {
	if credentialExists {
		return ""
	}
	if key := strings.TrimSpace(datasetKey); key != "" {
		return key
	}
	return strings.TrimSpace(legacyKey)
}

// ShouldPersist reports whether a key should be stored across reloads.
func ShouldPersist(remember bool, key string) bool {
	return remember && strings.TrimSpace(key) != ""
}
