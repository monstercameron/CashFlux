// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

func seedAIKeyRecord(t *testing.T, key, passcode string) {
	t.Helper()
	var record aicred.Credential
	var err error
	if passcode == "" {
		record, err = aicred.New(key, "", true)
	} else {
		var envelope []byte
		envelope, err = awaitAICredTest(t, func(done func([]byte, error)) {
			encryptDataset([]byte(key), passcode, done)
		})
		if err == nil {
			record, err = aicred.New("", string(envelope), true)
		}
	}
	if err != nil {
		t.Fatalf("create credential record: %v", err)
	}
	raw, err := aicred.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	browserstore.Set(aiCredentialStoreKey, string(raw))
	t.Cleanup(func() { browserstore.Remove(aiCredentialStoreKey) })
}

func loadAIKeyRecord(t *testing.T) aicred.Credential {
	t.Helper()
	record, err := aicred.Parse([]byte(browserstore.GetString(aiCredentialStoreKey)))
	if err != nil {
		t.Fatalf("parse saved credential: %v", err)
	}
	return record
}

func TestAppLockEnableEncryptsExistingAICredential(t *testing.T) {
	requireVaultWebCrypto(t)
	const apiKey, newPasscode = "sk-existing-plain-key", "passcode-new"
	seedAIKeyRecord(t, apiKey, "")
	if got := loadAIKeyRecord(t); got.Key != apiKey || got.Envelope != "" {
		t.Fatalf("before enabling lock, record = %#v; want plaintext device credential", got)
	}
	if _, err := awaitAICredTest(t, func(done func([]byte, error)) {
		migrateAICredentialAtRest("", newPasscode, func(err error) { done(nil, err) })
	}); err != nil {
		t.Fatalf("enable-lock credential migration: %v", err)
	}
	got := loadAIKeyRecord(t)
	if got.Key != "" || got.Envelope == "" || strings.Contains(browserstore.GetString(aiCredentialStoreKey), apiKey) {
		t.Fatalf("after enabling lock, record is not envelope-only: %#v", got)
	}
	parsed, ok := cryptobox.Parse([]byte(got.Envelope))
	if !ok {
		t.Fatal("enabled-lock envelope is invalid")
	}
	plain, err := awaitAICredTest(t, func(done func([]byte, error)) { decryptDataset(parsed, newPasscode, done) })
	if err != nil || string(plain) != apiKey {
		t.Fatalf("new passcode decrypt = %q, %v; want original key", plain, err)
	}
}

func TestAppLockPasscodeChangeRekeysAICredential(t *testing.T) {
	requireVaultWebCrypto(t)
	const apiKey, oldPasscode, newPasscode = "sk-passcode-change-key", "passcode-old", "passcode-new"
	seedAIKeyRecord(t, apiKey, oldPasscode)
	if _, err := awaitAICredTest(t, func(done func([]byte, error)) {
		migrateAICredentialAtRest(oldPasscode, newPasscode, func(err error) { done(nil, err) })
	}); err != nil {
		t.Fatalf("passcode-change credential migration: %v", err)
	}
	got := loadAIKeyRecord(t)
	parsed, ok := cryptobox.Parse([]byte(got.Envelope))
	if !ok {
		t.Fatal("re-keyed credential envelope is invalid")
	}
	plain, err := awaitAICredTest(t, func(done func([]byte, error)) { decryptDataset(parsed, newPasscode, done) })
	if err != nil || string(plain) != apiKey {
		t.Fatalf("new passcode decrypt = %q, %v; want original key", plain, err)
	}
	if _, err := awaitAICredTest(t, func(done func([]byte, error)) { decryptDataset(parsed, oldPasscode, done) }); err == nil {
		t.Fatal("old passcode still decrypts the re-keyed credential")
	}
}

func TestAppLockDisableRestoresPlainAICredential(t *testing.T) {
	requireVaultWebCrypto(t)
	const apiKey, oldPasscode = "sk-disable-lock-key", "passcode-old"
	seedAIKeyRecord(t, apiKey, oldPasscode)
	if _, err := awaitAICredTest(t, func(done func([]byte, error)) {
		migrateAICredentialAtRest(oldPasscode, "", func(err error) { done(nil, err) })
	}); err != nil {
		t.Fatalf("disable-lock credential migration: %v", err)
	}
	got := loadAIKeyRecord(t)
	if got.Key != apiKey || got.Envelope != "" {
		t.Fatalf("after disabling lock, record = %#v; want plaintext device credential", got)
	}
	if strings.Contains(got.Envelope, apiKey) {
		t.Fatal("unexpected envelope data remains after disabling App Lock")
	}
}
