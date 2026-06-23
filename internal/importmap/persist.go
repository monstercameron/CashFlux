// Package importmap — saved-profile registry.
//
// SavedProfile wraps a Profile with a human-readable name so users can recall
// a mapping for a particular bank without re-entering column positions each
// time. Profiles are kept in a session-only in-memory slice managed by
// appstate/importprofile_ops.go.
//
// NOTE: Full persistence (write-through to the SQLite store) is deferred
// because adding a new store table requires editing internal/store, which is
// off-limits for this change. The session slice survives the session; on
// reload the user re-saves. This is noted in DEVLOG.md. When store support is
// added, the CRUD surface here stays stable — only the backing store changes.
package importmap

// SavedProfile pairs an import Profile with a user-chosen name so it can be
// stored, listed, and re-applied without re-entering column indices.
type SavedProfile struct {
	// ID is a short identifier unique within the session (e.g. id.New()).
	ID string
	// Profile is the column-mapping configuration.
	Profile Profile
}
