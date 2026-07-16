// SPDX-License-Identifier: MIT

// Package notifyhistory is the pure, platform-independent core of the
// Notifications archive: an ordered, de-duplicated, capped list of past
// notifications plus search/filter and JSON (de)serialization.
//
// It has NO syscall/js and no knowledge of the app's FeedItem type, so it
// compiles and unit-tests on native Go. The wasm state seam
// (internal/uistate/notifyhistory.go) is the single place this core meets the
// live notification feed and the KV store.
package notifyhistory

import (
	"encoding/json"
	"sort"
	"strings"
)

// maxRecords caps the archive so it can never grow without bound in the KV
// blob. The newest maxRecords records are kept; older ones are pruned.
const maxRecords = 500

// Record is one archived notification. It is a self-contained flattening of the
// live feed item: a stable ID (used for de-duplication), the severity tier
// ("info" | "warning" | "critical"; empty reads as info), the human-readable
// message, an optional in-app route the alert points at, the unix-second time
// it fired, and whether the user has read it.
type Record struct {
	ID       string `json:"id"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
	Route    string `json:"route,omitempty"`
	At       int64  `json:"at"` // unix seconds
	Read     bool   `json:"read,omitempty"`
}

// Archive is the ordered set of archived records, newest first.
type Archive struct {
	Items []Record `json:"items"`
}

// Add inserts r into the archive. It de-duplicates by ID (a repeated ID
// replaces the earlier record rather than appending a second copy), keeps the
// list ordered newest-first by the At timestamp, and prunes to the newest
// maxRecords entries. It is idempotent for a given ID+contents, so callers may
// safely re-record the whole live feed on every open.
func (a *Archive) Add(r Record) {
	// Drop any existing record with the same ID (replace-in-place semantics).
	if r.ID != "" {
		kept := a.Items[:0]
		for _, it := range a.Items {
			if it.ID != r.ID {
				kept = append(kept, it)
			}
		}
		a.Items = kept
	}
	a.Items = append(a.Items, r)

	// Newest first; stable so equal timestamps keep their relative order.
	sort.SliceStable(a.Items, func(i, j int) bool {
		return a.Items[i].At > a.Items[j].At
	})

	// Prune oldest beyond the cap.
	if len(a.Items) > maxRecords {
		a.Items = a.Items[:maxRecords]
	}
}

// Filter returns the records whose Message contains query (case-insensitive)
// and — when severity is non-empty — whose Severity matches it exactly. An
// empty query matches everything. Order is preserved (newest first). The result
// is a fresh slice; the archive is not mutated.
func (a Archive) Filter(query, severity string) []Record {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Record, 0, len(a.Items))
	for _, it := range a.Items {
		if severity != "" && it.Severity != severity {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(it.Message), q) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// MarkAllRead flags every record read.
func (a *Archive) MarkAllRead() {
	for i := range a.Items {
		a.Items[i].Read = true
	}
}

// UnreadCount reports how many records are unread.
func (a Archive) UnreadCount() int {
	n := 0
	for _, it := range a.Items {
		if !it.Read {
			n++
		}
	}
	return n
}

// Marshal serializes the archive to JSON.
func Marshal(a Archive) (string, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Unmarshal parses a JSON archive. It is deliberately tolerant: an empty string
// or malformed JSON yields an empty (non-nil-item-safe) archive and no error,
// so a corrupt or absent KV blob never breaks the surface.
func Unmarshal(s string) (Archive, error) {
	if strings.TrimSpace(s) == "" {
		return Archive{}, nil
	}
	var a Archive
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return Archive{}, nil
	}
	return a, nil
}
