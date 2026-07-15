// SPDX-License-Identifier: MIT

// Package artifactref computes the set of artifact IDs the dataset still
// references, so a blob garbage-collector can safely delete only the artifacts
// nothing points at. It is the single source of truth for "what is still in
// use", which keeps the sweep honest as new artifact holders are added.
//
// Two holders reference artifacts today:
//   - transaction receipts (domain.Transaction.Attachments → AttachmentRef.ArtifactID) — TX5;
//   - custom-page Image/Table widgets (domain.PageWidget.Binding.ArtifactID).
//
// The package is pure (no syscall/js), so it unit-tests on native Go and the GC
// job can reuse it on either the wasm or a future backend side.
package artifactref

import "github.com/monstercameron/CashFlux/internal/domain"

// Referenced collects every artifact ID the dataset still points at: transaction
// receipt attachments and custom-page widget bindings. The returned set is what a
// blob GC must retain; anything not in it is safe to delete. Blank IDs are
// ignored.
func Referenced(txns []domain.Transaction, pages []domain.CustomPage) map[string]bool {
	refs := map[string]bool{}
	for _, t := range txns {
		for _, a := range t.Attachments {
			if a.ArtifactID != "" {
				refs[a.ArtifactID] = true
			}
		}
	}
	for _, p := range pages {
		for _, w := range p.Widgets {
			if w.Binding.ArtifactID != "" {
				refs[w.Binding.ArtifactID] = true
			}
		}
	}
	return refs
}

// Orphans returns the ids of artifacts that nothing in the dataset references —
// the set a blob GC may delete. It is Referenced inverted over the artifact list,
// exposed as a helper so callers don't re-implement the set difference.
func Orphans(artifacts []domain.Artifact, txns []domain.Transaction, pages []domain.CustomPage) []string {
	refs := Referenced(txns, pages)
	var out []string
	for _, a := range artifacts {
		if a.ID != "" && !refs[a.ID] {
			out = append(out, a.ID)
		}
	}
	return out
}
