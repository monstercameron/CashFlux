// SPDX-License-Identifier: MIT

package atrest

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

func TestCatalogueClassifiesInventoryAndFailsClosed(t *testing.T) {
	tests := []struct {
		key   string
		class Class
	}{
		{"cashflux:dataset", MustEncrypt},
		{"cashflux:dataset:gen", MustEncrypt},
		{"cashflux:seeded", MayStayPlain},
		{"cashflux:checkpoints", MustEncrypt},
		{"cashflux:checkpoint-data:cp-1", MustEncrypt},
		{"cashflux:sync-queue", MustEncrypt},
		{"cashflux:sync-conflict:ws-1", MustEncrypt},
		{"cashflux:quarantine:123:reason", MustEncrypt},
		{"cashflux:workspaces", MustEncrypt},
		{"cashflux:ws-data:ws-1", MustEncrypt},
		{"cashflux:sync-meta:ws-1", MustEncrypt},
		{"cashflux:sync-device-id", MustEncrypt},
		{"cashflux:sync-status", MustEncrypt},
		{"cashflux:auth:access-token", ShouldNotPersist},
		{"cashflux:auth:refresh-token", ShouldNotPersist},
		{"cashflux:prefs", MustEncrypt},
		{"cashflux:cred:openai", MustEncrypt},
		{"cashflux:credvault", MustEncrypt},
		{"cashflux:applock", MustEncrypt},
		{"cashflux:webauthn-credid", MustEncrypt},
		{"cashflux:webauthn-salt", MustEncrypt},
		{"cashflux:webauthn-vault", MustEncrypt},
		{"cashflux:member-pins", MustEncrypt},
		{"cf.artifactSalt", MayStayPlain},
		{"artifact:receipt-1", MustEncrypt},
		{"cashflux:notify:feed", MustEncrypt},
		{"cashflux:tx-filter", MustEncrypt},
		{"cashflux:theme", MayStayPlain},
		{"cashflux:active-lang", MayStayPlain},
		{"cashflux:txn-cols", MayStayPlain},
		{"cashflux:layout", MayStayPlain},
		{"cashflux:layout-mode", MayStayPlain},
		{"cashflux:rail-collapsed", MayStayPlain},
		{"cashflux:muzak", MayStayPlain},
		{"cashflux:allocate:config", MustEncrypt},
		{"cashflux:reconcile:draft:acct-1", MustEncrypt},
		{"cashflux:wb-graph", MustEncrypt},
		{"cashflux:networth-view", MustEncrypt},
		{"cashflux:auth:csrf", ShouldNotPersist},
		{"cashflux:recovery-code", ShouldNotPersist},
		{"cfv2:log:change-1", MustEncrypt},
		{"cfv2:snapshot:head", MustEncrypt},
		{"cfv2:meta:cursor", MayStayPlain},
		{"cache:cashflux-build-a", MayStayPlain},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if got := Classify(tc.key); got != tc.class {
				t.Fatalf("Classify(%q) = %s, want %s", tc.key, got, tc.class)
			}
			entry := ClassifyEntry(tc.key)
			if entry.Reason == "" {
				t.Fatalf("ClassifyEntry(%q) has no reason", tc.key)
			}
		})
	}
	if got := Classify("cashflux:future-secret:record"); got != MustEncrypt {
		t.Fatalf("unknown key class = %s, want %s", got, MustEncrypt)
	}
	for _, entry := range Catalog() {
		key := entry.Key
		if got := Classify(key); got != entry.Class {
			t.Errorf("catalogue entry %q classifies as %s, want %s", key, got, entry.Class)
		}
	}
}

func TestCatalogReturnsCopy(t *testing.T) {
	entries := Catalog()
	entries[0].Key = "changed"
	if Catalog()[0].Key == "changed" {
		t.Fatal("Catalog returned its mutable backing slice")
	}
}

func TestEveryENC0InventoriedKeyHasCatalogueClassification(t *testing.T) {
	keys := strings.Fields(`
	theme fonts banner prefs active-lang languages applock openai-key cred:openai websearch-key cloud-ai-key-set
	cloud-mention-snoozed chat-system-prompt rail-collapsed rail-tool-groups backupCadence lastBackupAt muzak
	muzak-pos muzak-volume muzak-noticed notify:browser notify:delivered seeded sampleActive sync-device-id
	sync-status sync-queue sync-meta:<workspace> sync-conflict:<workspace> dataset-owner auth:access-token
	auth:refresh-token auth:expires-in-seconds auth:refresh-mode auth:csrf auth:user-id workspaces ws-data:<id>
	webauthn-credid webauthn-salt webauthn-vault member-pins checkpoints checkpoint-data:<id>
	quarantine:<unix>[:reason] tx-filter txn-cols layout layout-mode nav-order nav-favorites nav-favorites-set
	widget-config hidden-widgets hidden-modules period-res period-window active-identity active-member
	active-scope accounts-class allocate:config debt:config networth:config networth-view planning:config
	reports:config report-range report-scope saved-reports report-snapshots retirement-plan tax-plan
	rebalance-targets invest:pools goals:roundupconfig goals:rounduprompt budgets:density budgets:sweepconfig
	budgets:sweepprompt bills:smartconfig health:trend benchmark-series notify:feed notifhistory notify:lastSeen
	notify:routes smart-digest:delivered smart-settings subs-detect-prefs subs:confirmed rhythm-agenda-view
	rhythm-detect-pins learn-tally learn-threshold freshness-dismissals review-resolutions flag-verdicts
	agent-memory assistant-privacy-default import-wip reconcile:draft:<accountID> txntemplates todo-views
	tasksuggest:dismissed wins-seen onboard-dismissed setup:currencyConfirmed setup:wizardDone visit:baseline
	visit:lastOpen wb-graph wb-canvas-pos wb-canvas-view wb-cards wb-toggle:<id> dash-preset dash-daily-nudge
	standing-instructions saved-view-threshold-dismissals app:explainSeed inflation-pct cashflux:dataset
	cashflux:dataset:gen cashflux:checkpoints cashflux:sync-conflict:<workspace> cashflux:quarantine:<unix>
	cashflux:ws-data:<id> cashflux:credvault cf.artifactSalt artifact:<id>
	`)
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			entry := ClassifyEntry(key)
			if strings.Contains(entry.Reason, "Unknown storage keys") {
				t.Fatalf("inventory key %q fell through to fail-closed default", key)
			}
		})
	}
}

func TestRecordEnvelopeRoundTripAndEncode(t *testing.T) {
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	scope := keyring.Scope{ServerID: "server-1", AccountID: "account-1"}
	plain := []byte(`{"merchant":"Example Market","amountMinor":-2799}`)
	e, err := SealRecord(wk, scope, "workspace-1", "cashflux:dataset", plain)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Example Market")) {
		t.Fatal("encoded envelope contains plaintext")
	}
	decoded, err := DecodeStored(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind != StoredEnvelope || decoded.Envelope == nil {
		t.Fatalf("decoded kind = %v, want StoredEnvelope", decoded.Kind)
	}
	got, err := OpenRecord(wk, scope, "workspace-1", "cashflux:dataset", *decoded.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("opened plaintext = %q, want %q", got, plain)
	}
}

func TestRecordEnvelopeRejectsTamperAndWrongBinding(t *testing.T) {
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	scope := keyring.Scope{ServerID: "server-1", AccountID: "account-1"}
	e, err := SealRecord(wk, scope, "workspace-1", "record-a", []byte("private payload"))
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer otherKey.Destroy()
	tests := []struct {
		name      string
		wk        *keyring.WorkspaceKey
		scope     keyring.Scope
		workspace string
		recordKey string
		mutate    func(*Envelope)
	}{
		{name: "different server", wk: wk, scope: keyring.Scope{ServerID: "server-2", AccountID: "account-1"}, workspace: "workspace-1", recordKey: "record-a"},
		{name: "different account", wk: wk, scope: keyring.Scope{ServerID: "server-1", AccountID: "account-2"}, workspace: "workspace-1", recordKey: "record-a"},
		{name: "different workspace", wk: wk, scope: scope, workspace: "workspace-2", recordKey: "record-a"},
		{name: "different record key", wk: wk, scope: scope, workspace: "workspace-1", recordKey: "record-b"},
		{name: "different key", wk: otherKey, scope: scope, workspace: "workspace-1", recordKey: "record-a"},
		{name: "tampered ciphertext", wk: wk, scope: scope, workspace: "workspace-1", recordKey: "record-a", mutate: func(e *Envelope) { e.Ciphertext[len(e.Ciphertext)-1] ^= 0x80 }},
		{name: "tampered nonce", wk: wk, scope: scope, workspace: "workspace-1", recordKey: "record-a", mutate: func(e *Envelope) { e.Nonce[0] ^= 0x80 }},
		{name: "tampered key version", wk: wk, scope: scope, workspace: "workspace-1", recordKey: "record-a", mutate: func(e *Envelope) { e.KeyVersion++ }},
		{name: "tampered key id", wk: wk, scope: scope, workspace: "workspace-1", recordKey: "record-a", mutate: func(e *Envelope) { e.KeyID = otherKey.KeyID() }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copy := cloneEnvelope(e)
			if tc.mutate != nil {
				tc.mutate(&copy)
			}
			if _, err := OpenRecord(tc.wk, tc.scope, tc.workspace, tc.recordKey, copy); err == nil {
				t.Fatal("OpenRecord succeeded with tampered or mismatched input")
			}
		})
	}
}

func TestDecodeStoredDistinguishesLegacyJSONAndEnvelope(t *testing.T) {
	legacy := []byte(`{"old":"plaintext"}`)
	decoded, err := DecodeStored(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind != StoredLegacyJSON || !bytes.Equal(decoded.Legacy, legacy) || decoded.Envelope != nil {
		t.Fatalf("legacy decode = %#v", decoded)
	}

	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	e, err := SealRecord(wk, keyring.LocalScope("account"), "workspace", "record", []byte("value"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeStored(raw)
	if err != nil || decoded.Kind != StoredEnvelope {
		t.Fatalf("envelope decode kind=%v err=%v", decoded.Kind, err)
	}
	if _, err := DecodeStored([]byte("not-json")); !errors.Is(err, ErrInvalidStoredValue) {
		t.Fatalf("invalid JSON err = %v, want ErrInvalidStoredValue", err)
	}
	badEnvelope := []byte(`{"format":"cashflux-atrest","version":1}`)
	if _, err := DecodeStored(badEnvelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("malformed envelope err = %v, want ErrInvalidEnvelope", err)
	}
}

func cloneEnvelope(e Envelope) Envelope {
	e.Nonce = bytes.Clone(e.Nonce)
	e.Ciphertext = bytes.Clone(e.Ciphertext)
	return e
}
