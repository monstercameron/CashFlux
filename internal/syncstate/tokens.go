// SPDX-License-Identifier: MIT

package syncstate

import (
	"sort"
	"strings"
)

// RefreshDecision says what a tab that has just acquired the cross-tab
// token-refresh lock should do next (SV0.2).
//
// The decision is pure so it can be tested without a browser: the caller
// supplies the credential it started with (captured before waiting for the
// lock) and the credential it re-read from shared storage after acquiring
// the lock. Everything that makes the cross-tab refresh dangerous lives in
// the gap between those two reads — a refresh token is single-use, so
// replaying a copy another tab has already consumed trips the server's
// reuse detection and revokes the whole session family.
type RefreshDecision int

const (
	// RefreshNeeded means nobody else refreshed while this tab waited: the
	// stored credential is still the one this tab started with, so this tab
	// is the one that must call the server.
	RefreshNeeded RefreshDecision = iota
	// RefreshReuseStored means another tab already refreshed while this tab
	// waited for the lock. The stored credential has moved on; reuse it and
	// do not call the server.
	RefreshReuseStored
	// RefreshSessionGone means the session this tab meant to refresh no
	// longer exists in storage (signed out, or scope reset in another tab).
	// There is nothing to refresh and nothing to reuse.
	RefreshSessionGone
)

// DecideRefresh compares the credential a tab started with against the one
// stored now, and returns the [RefreshDecision] for the tab holding the
// refresh lock.
//
// Both halves of the pair are compared because the two session types expose
// different evidence. A token-pair session stores its rotating refresh
// token locally, so a changed refresh token is the direct proof another tab
// rotated. An OAuth cookie session keeps its refresh token in an HttpOnly
// cookie the page cannot read, so the only observable rotation signal is a
// changed access token — callers pass empty refresh values there and the
// access comparison carries the decision.
//
// A stored value that merely appeared (starting empty, stored non-empty)
// also counts as reuse: whatever wrote it is newer than anything this tab
// holds, and refreshing on top of it would at best waste a rotation.
func DecideRefresh(startingRefresh, storedRefresh, startingAccess, storedAccess string) RefreshDecision {
	startingRefresh = strings.TrimSpace(startingRefresh)
	storedRefresh = strings.TrimSpace(storedRefresh)
	startingAccess = strings.TrimSpace(startingAccess)
	storedAccess = strings.TrimSpace(storedAccess)

	if storedRefresh == "" && storedAccess == "" && (startingRefresh != "" || startingAccess != "") {
		return RefreshSessionGone
	}
	if storedRefresh != "" && storedRefresh != startingRefresh {
		return RefreshReuseStored
	}
	if storedAccess != "" && storedAccess != startingAccess {
		return RefreshReuseStored
	}
	return RefreshNeeded
}

// KeysToClearOnServerChange returns the subset of present storage keys that
// belong to the old server's scope and must be cleared when the configured
// server host changes (SV0.2): every key in exact, plus every key under one
// of prefixes (per-workspace families such as the sync bookmarks, whose
// full key set is only known by enumerating storage).
//
// The caller passes its own key constants rather than this package owning a
// second copy of the strings — key names already have exactly one home per
// key (internal/app, internal/uistate), and a duplicated literal here would
// silently drift from it.
//
// The result is sorted and de-duplicated so callers and tests get one
// deterministic answer, and it contains only keys that are actually
// present: "clear exactly what exists" is also how a caller can tell
// whether there was any old scope to reset at all.
func KeysToClearOnServerChange(present, exact, prefixes []string) []string {
	exactSet := make(map[string]bool, len(exact))
	for _, k := range exact {
		if k = strings.TrimSpace(k); k != "" {
			exactSet[k] = true
		}
	}
	seen := make(map[string]bool)
	var out []string
	for _, k := range present {
		if seen[k] {
			continue
		}
		if exactSet[k] {
			seen[k] = true
			out = append(out, k)
			continue
		}
		for _, p := range prefixes {
			if p != "" && strings.HasPrefix(k, p) {
				seen[k] = true
				out = append(out, k)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
