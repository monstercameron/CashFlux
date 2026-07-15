// SPDX-License-Identifier: MIT

package domain

import (
	"sort"
	"strings"
	"time"
)

// AccountDocRef is one document filed against an account (AC8): a statement,
// contract, title, or payoff letter. It references a stored domain.Artifact by ID
// and carries the filing metadata the drawer renders — the display label, the date
// it was filed, and an optional renewal / expiry date (AC17) that drives a reminder
// task. The referenced artifact's bytes are retained by the blob GC as long as this
// ref exists (see internal/artifactref).
type AccountDocRef struct {
	// ArtifactID references the stored domain.Artifact holding the document's bytes
	// or parsed content. Required; a ref with a blank ArtifactID is ignored.
	ArtifactID string `json:"artifactId"`
	// Label is the human name shown in the drawer (e.g. "Auto insurance policy",
	// "March 2026 statement"). Falls back to the artifact name at render time when
	// blank. It is also the join key AC17 uses to decide a NEWER document of the same
	// kind has replaced this one (case-insensitive), auto-resolving the expiry task.
	Label string `json:"label,omitempty"`
	// AttachedAt is when the document was filed against the account. Used to sort the
	// drawer newest-first and to pick the most-recent doc per label.
	AttachedAt time.Time `json:"attachedAt,omitempty"`
	// ExpiresAt is an optional renewal / expiry date (insurance policy, registration,
	// warranty). When set, AC17 generates a reminder task ahead of this date. Zero
	// means the document never expires and produces no reminder.
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

// DisplayLabel returns the label to show for the ref, falling back to the supplied
// artifact name (and then a generic placeholder) when the ref has no explicit label.
func (r AccountDocRef) DisplayLabel(artifactName string) string {
	if l := strings.TrimSpace(r.Label); l != "" {
		return l
	}
	if n := strings.TrimSpace(artifactName); n != "" {
		return n
	}
	return "Document"
}

// LabelKey is the normalized join key used to group documents by "same kind" — the
// lower-cased, space-trimmed label. AC17 treats a later document with the same
// LabelKey as a renewal of an earlier one.
func (r AccountDocRef) LabelKey() string {
	return strings.ToLower(strings.TrimSpace(r.Label))
}

// SortDocRefsByDate returns a copy of the refs ordered newest-attached first (ties
// break by label then artifact ID for determinism), the order the drawer renders.
func SortDocRefsByDate(refs []AccountDocRef) []AccountDocRef {
	out := append([]AccountDocRef(nil), refs...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].AttachedAt.Equal(out[j].AttachedAt) {
			return out[i].AttachedAt.After(out[j].AttachedAt)
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ArtifactID < out[j].ArtifactID
	})
	return out
}

// AccountDocArtifactIDs collects the artifact IDs referenced by an account's
// documents, ignoring blanks. The blob GC unions this with its other reference
// sources so a filed document's bytes are never swept.
func AccountDocArtifactIDs(accounts []Account) map[string]bool {
	refs := map[string]bool{}
	for _, a := range accounts {
		for _, d := range a.DocRefs {
			if d.ArtifactID != "" {
				refs[d.ArtifactID] = true
			}
		}
	}
	return refs
}
