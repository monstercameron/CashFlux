// SPDX-License-Identifier: MIT

package aicred

import "testing"

func TestCredentialRoundTripAndValidation(t *testing.T) {
	tests := []struct {
		name string
		in   Credential
		want string
		bad  bool
	}{
		{name: "plaintext remembered", in: Credential{Version: 1, Provider: ProviderOpenAI, Key: "sk-secret", Remember: true}, want: "sk-secret"},
		{name: "encrypted remembered", in: Credential{Version: 1, Provider: ProviderOpenAI, Envelope: "cryptobox:v1", Remember: true}, want: "cryptobox:v1"},
		{name: "both values", in: Credential{Version: 1, Provider: ProviderOpenAI, Key: "sk-secret", Envelope: "cryptobox:v1", Remember: true}, bad: true},
		{name: "not remembered", in: Credential{Version: 1, Provider: ProviderOpenAI, Key: "sk-secret"}, bad: true},
		{name: "unsupported provider", in: Credential{Version: 1, Provider: "other", Key: "secret", Remember: true}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Marshal(tt.in)
			if tt.bad {
				if err == nil {
					t.Fatal("Marshal succeeded for invalid record")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			value := got.Key
			if value == "" {
				value = got.Envelope
			}
			if value != tt.want {
				t.Fatalf("value = %q, want %q", value, tt.want)
			}
		})
	}
	if _, err := Parse([]byte(`{"v":1,"provider":"openai","key":"x","remember":true,"extra":1}`)); err == nil {
		t.Fatal("Parse accepted an unknown field")
	}
	if _, err := Parse([]byte(`{"v":1,"provider":"openai","key":"x","remember":true} {}`)); err == nil {
		t.Fatal("Parse accepted trailing JSON")
	}
}

func TestMigrationKeyPrecedence(t *testing.T) {
	for _, tt := range []struct {
		name, dataset, legacy string
		exists                bool
		want                  string
	}{
		{"dataset first", " dataset-key ", "legacy-key", false, "dataset-key"},
		{"legacy fallback", "", " legacy-key ", false, "legacy-key"},
		{"existing credential wins", "dataset-key", "legacy-key", true, ""},
		{"empty inputs", " ", " ", false, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := MigrationKey(tt.dataset, tt.legacy, tt.exists); got != tt.want {
				t.Fatalf("MigrationKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRememberOnOff(t *testing.T) {
	if !ShouldPersist(true, " sk-key ") {
		t.Fatal("remembered key should persist")
	}
	if ShouldPersist(false, "sk-key") {
		t.Fatal("unremembered key should not persist")
	}
	if ShouldPersist(true, " \t ") {
		t.Fatal("empty key should not persist")
	}
}
