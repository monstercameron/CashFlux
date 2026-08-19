// SPDX-License-Identifier: MIT

// Native unit tests for the connection-credential stripping applied to every
// workspace bundle that leaves this device (prefsredact.go).
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// A workspace export is a file the user downloads and shares. cashflux:prefs is
// one of perWorkspaceKeys, so it was bundled in verbatim — carrying a live
// access token in plain text, while wsExport's own comment claimed the envelope
// "carries no secrets".
func TestStripConnectionPrefsRemovesCredentials(t *testing.T) {
	bundle := map[string]string{
		"cashflux:dataset": `{"transactions":[]}`,
		prefsStoreKey: `{"theme":"dark","weekStart":1,"serverUrl":"https://budget.example",` +
			`"serverToken":"eyJhbGciOiJIUzI1NiJ9.live-access-token","serverCsrf":"csrf-secret"}`,
	}
	out := stripConnectionPrefs(bundle)

	raw := out[prefsStoreKey]
	for _, secret := range []string{"live-access-token", "csrf-secret", "budget.example"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("stripped prefs still contain %q: %s", secret, raw)
		}
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("stripped prefs are not valid JSON: %v", err)
	}
	for _, key := range connectionPrefKeys {
		if _, present := fields[key]; present {
			t.Fatalf("connection field %q survived", key)
		}
	}
}

// Losing a user's settings to a security fix would be its own bug, so
// everything that is not a connection field must come through untouched —
// including preferences this package has never heard of.
func TestStripConnectionPrefsKeepsEverythingElse(t *testing.T) {
	bundle := map[string]string{
		"cashflux:dataset": `{"transactions":[1,2,3]}`,
		"cashflux:layout":  `{"tiles":["a","b"]}`,
		prefsStoreKey: `{"theme":"dark","weekStart":1,"accent":"#abc","scale":110,` +
			`"serverToken":"secret","someFuturePreference":{"nested":true}}`,
	}
	out := stripConnectionPrefs(bundle)

	if out["cashflux:dataset"] != bundle["cashflux:dataset"] {
		t.Fatal("the dataset was altered")
	}
	if out["cashflux:layout"] != bundle["cashflux:layout"] {
		t.Fatal("the layout was altered")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(out[prefsStoreKey]), &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"theme", "weekStart", "accent", "scale", "someFuturePreference"} {
		if _, present := fields[key]; !present {
			t.Fatalf("preference %q was dropped", key)
		}
	}
	if nested, ok := fields["someFuturePreference"].(map[string]any); !ok || nested["nested"] != true {
		t.Fatalf("an unknown nested preference was not preserved: %#v", fields["someFuturePreference"])
	}
}

// The input is untrusted on the way in as well as out: an older export, or one
// hand-edited to put the fields back, must not inject an endpoint or a bearer
// token into this device.
func TestStripConnectionPrefsEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bundle map[string]string
		check  func(t *testing.T, out map[string]string)
	}{
		{
			name:   "no prefs entry at all",
			bundle: map[string]string{"cashflux:dataset": "{}"},
			check: func(t *testing.T, out map[string]string) {
				if _, present := out[prefsStoreKey]; present {
					t.Fatal("a prefs entry was invented")
				}
				if out["cashflux:dataset"] != "{}" {
					t.Fatal("the dataset was altered")
				}
			},
		},
		{
			name:   "empty prefs entry",
			bundle: map[string]string{prefsStoreKey: ""},
			check: func(t *testing.T, out map[string]string) {
				if out[prefsStoreKey] != "" {
					t.Fatalf("empty prefs became %q", out[prefsStoreKey])
				}
			},
		},
		{
			name:   "unparseable prefs are dropped, never passed through",
			bundle: map[string]string{prefsStoreKey: `{"serverToken":"secret", TRUNCATED`},
			check: func(t *testing.T, out map[string]string) {
				if raw, present := out[prefsStoreKey]; present {
					t.Fatalf("prefs we could not read were kept: %q", raw)
				}
			},
		},
		{
			name:   "a JSON array where prefs should be is dropped",
			bundle: map[string]string{prefsStoreKey: `["serverToken","secret"]`},
			check: func(t *testing.T, out map[string]string) {
				if raw, present := out[prefsStoreKey]; present {
					t.Fatalf("a non-object prefs entry was kept: %q", raw)
				}
			},
		},
		{
			name:   "nil bundle",
			bundle: nil,
			check: func(t *testing.T, out map[string]string) {
				if len(out) != 0 {
					t.Fatalf("nil bundle produced %#v", out)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, stripConnectionPrefs(tc.bundle))
		})
	}
}

// The caller's own map must not be mutated: exportWorkspace strips a bundle that
// came from bundleCurrent(), and switchWorkspace saves that same shape back into
// this device's storage. Editing in place would strip the LIVE prefs.
func TestStripConnectionPrefsDoesNotMutateItsInput(t *testing.T) {
	original := `{"theme":"dark","serverToken":"live-token"}`
	bundle := map[string]string{prefsStoreKey: original}

	out := stripConnectionPrefs(bundle)

	if bundle[prefsStoreKey] != original {
		t.Fatalf("the caller's bundle was modified: %q", bundle[prefsStoreKey])
	}
	if strings.Contains(out[prefsStoreKey], "live-token") {
		t.Fatal("the returned copy still holds the token")
	}
}

// Stripping twice must be identical to stripping once — export and import both
// apply it, and a round trip through a file goes through both.
func TestStripConnectionPrefsIsIdempotent(t *testing.T) {
	bundle := map[string]string{
		prefsStoreKey: `{"theme":"dark","serverUrl":"https://budget.example","serverToken":"tok"}`,
	}
	once := stripConnectionPrefs(bundle)
	twice := stripConnectionPrefs(once)
	if once[prefsStoreKey] != twice[prefsStoreKey] {
		t.Fatalf("not idempotent:\n once: %s\ntwice: %s", once[prefsStoreKey], twice[prefsStoreKey])
	}
}

// Every field the connection list names must actually be a field prefs
// serializes, or the strip would silently protect nothing. This pins the two
// lists together across a rename.
func TestConnectionPrefKeysMatchPrefsJSONTags(t *testing.T) {
	// Marshalled from a prefs value carrying all three, so the tags come from
	// the real struct rather than from a copy of its field names.
	raw, err := json.Marshal(map[string]string{
		"serverUrl": "u", "serverToken": "t", "serverCsrf": "c",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range connectionPrefKeys {
		if _, present := fields[key]; !present {
			t.Fatalf("connectionPrefKeys names %q, which is not a serialized prefs field", key)
		}
	}
	if len(connectionPrefKeys) != len(fields) {
		t.Fatalf("connectionPrefKeys = %v, expected exactly the three connection fields", connectionPrefKeys)
	}
}

// The full backup is the file most likely to end up in a sync folder, and it
// carried a live access token in the clear.
func TestStripConnectionPrefsJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want func(t *testing.T, got string)
	}{
		{
			name: "credentials removed, settings kept",
			raw:  `{"theme":"dark","serverUrl":"https://budget.example","serverToken":"live","serverCsrf":"csrf"}`,
			want: func(t *testing.T, got string) {
				for _, secret := range []string{"live", "csrf", "budget.example"} {
					if strings.Contains(got, secret) {
						t.Fatalf("still contains %q: %s", secret, got)
					}
				}
				if !strings.Contains(got, "dark") {
					t.Fatalf("theme was dropped: %s", got)
				}
			},
		},
		{
			name: "empty in, empty out",
			raw:  "",
			want: func(t *testing.T, got string) {
				if got != "" {
					t.Fatalf("got %q", got)
				}
			},
		},
		{
			name: "unreadable prefs are dropped, never passed through",
			raw:  `{"serverToken":"live", TRUNCATED`,
			want: func(t *testing.T, got string) {
				if got != "" {
					t.Fatalf("unparseable prefs came through: %q", got)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.want(t, stripConnectionPrefsJSON(tc.raw)) })
	}
}
