// SPDX-License-Identifier: MIT

// Package appstate — import-profile CRUD persisted with the dataset and workspace.
package appstate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importmap"
)

const importProfilesKey = "import.profiles.v1"

type scopedImportProfile struct {
	WorkspaceID string                 `json:"workspaceId,omitempty"`
	Profile     importmap.SavedProfile `json:"profile"`
}

func (a *App) readImportProfiles() ([]scopedImportProfile, error) {
	raw, ok, err := a.store.GetKV(importProfilesKey)
	if err != nil || !ok {
		return nil, err
	}
	var profiles []scopedImportProfile
	if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
		// Accept the old profile-only payload as the empty-workspace scope.
		var legacy []importmap.SavedProfile
		if legacyErr := json.Unmarshal([]byte(raw), &legacy); legacyErr != nil {
			return nil, fmt.Errorf("appstate: read import profiles: %w", err)
		}
		profiles = make([]scopedImportProfile, len(legacy))
		for i, profile := range legacy {
			profiles[i] = scopedImportProfile{Profile: profile}
		}
	}
	return profiles, nil
}

func (a *App) writeImportProfiles(profiles []scopedImportProfile) error {
	raw, err := json.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("appstate: encode import profiles: %w", err)
	}
	if err := a.store.SetKV(importProfilesKey, string(raw)); err != nil {
		return fmt.Errorf("appstate: save import profiles: %w", err)
	}
	return nil
}

// ImportProfiles returns the current list of saved import profiles.
// The returned slice is a copy; mutations do not affect the registry.
func (a *App) ImportProfiles(workspaceID ...string) []importmap.SavedProfile {
	scope := profileWorkspace(workspaceID)
	profiles, err := a.readImportProfiles()
	if err != nil {
		a.log.Error("list import profiles", "err", err)
		return nil
	}
	var out []importmap.SavedProfile
	for _, record := range profiles {
		if record.WorkspaceID == scope {
			out = append(out, record.Profile)
		}
	}
	return out
}

func profileWorkspace(workspaceID []string) string {
	if len(workspaceID) == 0 {
		return ""
	}
	return strings.TrimSpace(workspaceID[0])
}

// SaveImportProfile adds or updates a saved import profile. If sp.ID is empty
// a new ID is assigned. If a profile with that ID already exists it is
// replaced. The profile name must not be blank.
func (a *App) SaveImportProfile(sp importmap.SavedProfile, workspaceID ...string) (importmap.SavedProfile, error) {
	if err := a.roleGuard(); err != nil {
		return importmap.SavedProfile{}, err
	}
	if strings.TrimSpace(sp.Profile.Name) == "" {
		return importmap.SavedProfile{}, fmt.Errorf("appstate: import profile name is required")
	}
	if sp.ID == "" {
		sp.ID = id.New()
	}
	profiles, err := a.readImportProfiles()
	if err != nil {
		return importmap.SavedProfile{}, err
	}
	scope := profileWorkspace(workspaceID)
	for i, existing := range profiles {
		if existing.WorkspaceID == scope && existing.Profile.ID == sp.ID {
			profiles[i].Profile = sp
			if err := a.writeImportProfiles(profiles); err != nil {
				return importmap.SavedProfile{}, err
			}
			a.log.Info("import profile updated", "id", sp.ID, "name", sp.Profile.Name)
			return sp, nil
		}
	}
	profiles = append(profiles, scopedImportProfile{WorkspaceID: scope, Profile: sp})
	if err := a.writeImportProfiles(profiles); err != nil {
		return importmap.SavedProfile{}, err
	}
	a.log.Info("import profile saved", "id", sp.ID, "name", sp.Profile.Name)
	return sp, nil
}

// DeleteImportProfile removes the saved profile with the given ID. It is a
// no-op (and returns nil) when no profile with that ID exists.
func (a *App) DeleteImportProfile(profileID string, workspaceID ...string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	profiles, err := a.readImportProfiles()
	if err != nil {
		return err
	}
	scope := profileWorkspace(workspaceID)
	next := make([]scopedImportProfile, 0, len(profiles))
	for _, record := range profiles {
		if record.Profile.ID == profileID && record.WorkspaceID == scope {
			continue
		}
		next = append(next, record)
	}
	if err := a.writeImportProfiles(next); err != nil {
		return err
	}
	a.log.Info("import profile deleted", "id", profileID)
	return nil
}
