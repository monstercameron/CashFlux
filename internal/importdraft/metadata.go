// SPDX-License-Identifier: MIT

package importdraft

import "maps"

// Metadata accompanies session-restored draft rows. It is scoped to stable row
// IDs so errors and source provenance cannot be applied to a different row.
type Metadata struct{ Sources, Errors map[string]string }

// Metadata returns an isolated snapshot for the session store.
func (m *Model) Metadata() Metadata {
	return Metadata{Sources: maps.Clone(m.Sources), Errors: maps.Clone(m.Errors)}
}

// RestoreMetadata restores only facts belonging to rows present in this draft.
func (m *Model) RestoreMetadata(meta Metadata) {
	for _, row := range m.Rows {
		if source, ok := meta.Sources[row.ID]; ok {
			m.Sources[row.ID] = source
		}
		if reason := meta.Errors[row.ID]; reason != "" {
			m.Errors[row.ID] = reason
		}
	}
}
