// SPDX-License-Identifier: MIT

// Saved-view persistence helpers for the scope package.
//
// A SavedView pairs a human-readable name with a ReportScope so users can
// recall frequently-used scope configurations without re-selecting the filters
// each time. Views are stored as individual JSON values inside a string→string
// KV map (one entry per view, keyed by SavedView.ID), making them portable
// across the app's KV layers (SQLite settings KV or localStorage).
package scope

import (
	"encoding/json"
	"sort"
	"strings"
)

// SavedView is a user-named snapshot of a ReportScope.
type SavedView struct {
	// ID is the stable, opaque storage key for this view. Callers should set it
	// to a UUIDv4 or similar unique string.
	ID string `json:"id"`
	// Name is the human-readable label shown in the UI.
	Name string `json:"name"`
	// Scope is the captured filter state.
	Scope ReportScope `json:"scope"`
}

// ListSavedViews decodes every valid JSON entry in kv into a SavedView and
// returns them sorted by Name (case-insensitively). Entries whose JSON cannot
// be decoded are silently skipped so a single corrupt entry never blocks the
// rest of the list.
func ListSavedViews(kv map[string]string) []SavedView {
	views := make([]SavedView, 0, len(kv))
	for _, raw := range kv {
		var v SavedView
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue // skip corrupt entries gracefully
		}
		views = append(views, v)
	}
	sort.Slice(views, func(i, j int) bool {
		return strings.ToLower(views[i].Name) < strings.ToLower(views[j].Name)
	})
	return views
}

// PutSavedView serialises v as JSON and stores it in kv under v.ID. If kv is
// nil a new map is allocated. The updated map is returned; callers must
// persist it back to their storage layer. If v cannot be serialised (which
// should never happen for well-formed input) the original map is returned
// unchanged.
func PutSavedView(kv map[string]string, v SavedView) map[string]string {
	if kv == nil {
		kv = make(map[string]string)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return kv
	}
	kv[v.ID] = string(b)
	return kv
}

// DeleteSavedView removes the entry for id from kv. It is a no-op when id
// is absent. The updated map is returned.
func DeleteSavedView(kv map[string]string, id string) map[string]string {
	if kv == nil {
		return kv
	}
	delete(kv, id)
	return kv
}
