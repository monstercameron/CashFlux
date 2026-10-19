// SPDX-License-Identifier: MIT

// Package importmap — saved-profile registry.
//
// SavedProfile wraps a Profile with a human-readable name so users can recall
// a mapping for a particular bank without re-entering column positions. The
// appstate package persists profiles with the dataset and scopes them to a
// workspace.
package importmap

// SavedProfile pairs an import Profile with a user-chosen name so it can be
// stored, listed, and re-applied without re-entering column indices.
type SavedProfile struct {
	// ID is a short identifier unique within its workspace (e.g. id.New()).
	ID string
	// Profile is the column-mapping configuration.
	Profile Profile
}
