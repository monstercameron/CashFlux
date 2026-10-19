// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

func awaitAICredTest(t *testing.T, start func(func([]byte, error))) ([]byte, error) {
	return awaitVaultTest(t, start)
}

func TestSV08AppLockCredentialRecordContainsOnlyEnvelope(t *testing.T) {
	requireVaultWebCrypto(t)
	const apiKey, passcode = "sk-test-device-secret", "test-passcode"
	envelope, err := awaitAICredTest(t, func(done func([]byte, error)) { encryptDataset([]byte(apiKey), passcode, done) })
	if err != nil {
		t.Fatalf("encrypt device credential: %v", err)
	}
	if _, ok := cryptobox.Parse(envelope); !ok {
		t.Fatal("encrypted credential is not a cryptobox envelope")
	}
	record, err := aicred.New("", string(envelope), true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := aicred.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), apiKey) {
		t.Fatal("stored record contains the plaintext API key")
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["key"] != nil || fields["envelope"] == nil {
		t.Fatalf("record fields = %#v, expected envelope only", fields)
	}
	parsed, ok := cryptobox.Parse([]byte(record.Envelope))
	if !ok {
		t.Fatal("record envelope could not be parsed")
	}
	plain, err := awaitAICredTest(t, func(done func([]byte, error)) { decryptDataset(parsed, passcode, done) })
	if err != nil {
		t.Fatalf("decrypt stored credential: %v", err)
	}
	if string(plain) != apiKey {
		t.Fatalf("decrypted key = %q", plain)
	}
}
