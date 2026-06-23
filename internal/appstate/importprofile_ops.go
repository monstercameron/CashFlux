// Package appstate — import-profile CRUD (session-only).
//
// Saved import profiles are stored in a package-level slice rather than the
// SQLite store because extending the store schema requires editing
// internal/store, which is off-limits for this change set. The slice lives for
// the life of the wasm process; profiles are lost on reload.
//
// When full persistence is added, the store.PutImportProfile /
// store.ListImportProfiles calls slot in here with no UI changes needed.
package appstate

import (
	"fmt"
	"strings"
	"sync"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importmap"
)

var (
	importProfilesMu sync.Mutex
	importProfiles   []importmap.SavedProfile
)

// ImportProfiles returns the current list of saved import profiles.
// The returned slice is a copy; mutations do not affect the registry.
func (a *App) ImportProfiles() []importmap.SavedProfile {
	importProfilesMu.Lock()
	defer importProfilesMu.Unlock()
	out := make([]importmap.SavedProfile, len(importProfiles))
	copy(out, importProfiles)
	return out
}

// SaveImportProfile adds or updates a saved import profile. If sp.ID is empty
// a new ID is assigned. If a profile with that ID already exists it is
// replaced. The profile name must not be blank.
func (a *App) SaveImportProfile(sp importmap.SavedProfile) (importmap.SavedProfile, error) {
	if strings.TrimSpace(sp.Profile.Name) == "" {
		return importmap.SavedProfile{}, fmt.Errorf("appstate: import profile name is required")
	}
	importProfilesMu.Lock()
	defer importProfilesMu.Unlock()
	if sp.ID == "" {
		sp.ID = id.New()
	}
	for i, existing := range importProfiles {
		if existing.ID == sp.ID {
			importProfiles[i] = sp
			a.log.Info("import profile updated", "id", sp.ID, "name", sp.Profile.Name)
			return sp, nil
		}
	}
	importProfiles = append(importProfiles, sp)
	a.log.Info("import profile saved", "id", sp.ID, "name", sp.Profile.Name)
	return sp, nil
}

// DeleteImportProfile removes the saved profile with the given ID. It is a
// no-op (and returns nil) when no profile with that ID exists.
func (a *App) DeleteImportProfile(profileID string) error {
	importProfilesMu.Lock()
	defer importProfilesMu.Unlock()
	next := importProfiles[:0]
	for _, sp := range importProfiles {
		if sp.ID != profileID {
			next = append(next, sp)
		}
	}
	importProfiles = next
	a.log.Info("import profile deleted", "id", profileID)
	return nil
}
