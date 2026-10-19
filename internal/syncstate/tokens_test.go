// SPDX-License-Identifier: MIT

package syncstate

import (
	"reflect"
	"testing"
)

func TestDecideRefresh(t *testing.T) {
	tests := []struct {
		name            string
		startingRefresh string
		storedRefresh   string
		startingAccess  string
		storedAccess    string
		want            RefreshDecision
	}{
		{
			name:            "unchanged pair still needs this tab to refresh",
			startingRefresh: "r1", storedRefresh: "r1",
			startingAccess: "a1", storedAccess: "a1",
			want: RefreshNeeded,
		},
		{
			name:            "another tab rotated the refresh token",
			startingRefresh: "r1", storedRefresh: "r2",
			startingAccess: "a1", storedAccess: "a2",
			want: RefreshReuseStored,
		},
		{
			name:            "cookie session: changed access token is the rotation signal",
			startingRefresh: "", storedRefresh: "",
			startingAccess: "a1", storedAccess: "a2",
			want: RefreshReuseStored,
		},
		{
			name:            "cookie session: unchanged access token still needs a refresh",
			startingRefresh: "", storedRefresh: "",
			startingAccess: "a1", storedAccess: "a1",
			want: RefreshNeeded,
		},
		{
			name:            "session signed out in another tab while waiting",
			startingRefresh: "r1", storedRefresh: "",
			startingAccess: "a1", storedAccess: "",
			want: RefreshSessionGone,
		},
		{
			name:            "cookie session cleared while waiting",
			startingRefresh: "", storedRefresh: "",
			startingAccess: "a1", storedAccess: "",
			want: RefreshSessionGone,
		},
		{
			name:            "credential appeared while waiting is reused, not refreshed over",
			startingRefresh: "", storedRefresh: "r9",
			startingAccess: "", storedAccess: "a9",
			want: RefreshReuseStored,
		},
		{
			name:            "no session before or after is not a vanished session",
			startingRefresh: "", storedRefresh: "",
			startingAccess: "", storedAccess: "",
			want: RefreshNeeded,
		},
		{
			name:            "whitespace differences are not a rotation",
			startingRefresh: " r1 ", storedRefresh: "r1",
			startingAccess: "a1", storedAccess: " a1 ",
			want: RefreshNeeded,
		},
		{
			// The pair rotates together, so this state should not occur; if
			// it ever does (a torn read between the two writes) the changed
			// refresh token is the stronger signal and wins.
			name:            "refresh changed but access not yet visible",
			startingRefresh: "r1", storedRefresh: "r2",
			startingAccess: "a1", storedAccess: "a1",
			want: RefreshReuseStored,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRefresh(tc.startingRefresh, tc.storedRefresh, tc.startingAccess, tc.storedAccess)
			if got != tc.want {
				t.Fatalf("DecideRefresh(%q, %q, %q, %q) = %v, want %v",
					tc.startingRefresh, tc.storedRefresh, tc.startingAccess, tc.storedAccess, got, tc.want)
			}
		})
	}
}

func TestKeysToClearOnServerChange(t *testing.T) {
	// The exact/prefix lists mirror what internal/app passes for a server
	// host change (SV0.2): the rotated pair, its expiry and refresh mode,
	// the CSRF token, the signed-in user ID, the sync queue, the dataset
	// owner, the cloud-AI-key flag, and every per-workspace sync bookmark.
	exact := []string{
		"cashflux:auth:access-token",
		"cashflux:auth:refresh-token",
		"cashflux:auth:expires-in-seconds",
		"cashflux:auth:refresh-mode",
		"cashflux:auth:csrf",
		"cashflux:auth:user-id",
		"cashflux:sync-queue",
		"cashflux:dataset-owner",
		"cashflux:cloud-ai-key-set",
	}
	prefixes := []string{"cashflux:sync-meta:"}

	tests := []struct {
		name     string
		present  []string
		exact    []string
		prefixes []string
		want     []string
	}{
		{
			name: "clears the whole old scope and nothing else",
			present: []string{
				"cashflux:auth:access-token",
				"cashflux:auth:refresh-token",
				"cashflux:auth:expires-in-seconds",
				"cashflux:auth:refresh-mode",
				"cashflux:auth:csrf",
				"cashflux:auth:user-id",
				"cashflux:sync-queue",
				"cashflux:sync-meta:default",
				"cashflux:sync-meta:ws-2",
				"cashflux:dataset-owner",
				"cashflux:cloud-ai-key-set",
				// Not server scope: the dataset itself, device identity,
				// prefs and unrelated state must survive a server change.
				"cashflux:dataset",
				"cashflux:sync-device-id",
				"cashflux:prefs",
				"cashflux:theme",
				"cashflux:sync-conflict:default",
				"cashflux:workspaces",
			},
			exact:    exact,
			prefixes: prefixes,
			want: []string{
				"cashflux:auth:access-token",
				"cashflux:auth:csrf",
				"cashflux:auth:expires-in-seconds",
				"cashflux:auth:refresh-mode",
				"cashflux:auth:refresh-token",
				"cashflux:auth:user-id",
				"cashflux:cloud-ai-key-set",
				"cashflux:dataset-owner",
				"cashflux:sync-meta:default",
				"cashflux:sync-meta:ws-2",
				"cashflux:sync-queue",
			},
		},
		{
			name:     "no old scope present clears nothing",
			present:  []string{"cashflux:dataset", "cashflux:prefs"},
			exact:    exact,
			prefixes: prefixes,
			want:     nil,
		},
		{
			name:     "only bookmarks present",
			present:  []string{"cashflux:sync-meta:default"},
			exact:    exact,
			prefixes: prefixes,
			want:     []string{"cashflux:sync-meta:default"},
		},
		{
			name:     "duplicate present keys are returned once",
			present:  []string{"cashflux:sync-queue", "cashflux:sync-queue"},
			exact:    exact,
			prefixes: prefixes,
			want:     []string{"cashflux:sync-queue"},
		},
		{
			name:     "prefix does not match a merely similar key",
			present:  []string{"cashflux:sync-metadata", "cashflux:sync-meta"},
			exact:    exact,
			prefixes: prefixes,
			want:     nil,
		},
		{
			name:     "empty present list",
			present:  nil,
			exact:    exact,
			prefixes: prefixes,
			want:     nil,
		},
		{
			name:     "empty exact and prefix lists clear nothing",
			present:  []string{"cashflux:sync-queue", "cashflux:sync-meta:default"},
			exact:    nil,
			prefixes: nil,
			want:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := KeysToClearOnServerChange(tc.present, tc.exact, tc.prefixes)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("KeysToClearOnServerChange = %q, want %q", got, tc.want)
			}
		})
	}
}
