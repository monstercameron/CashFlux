// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

// AccountGroups returns every persisted account group (the user-defined
// /accounts groupings — AC1), sorted by Order then Name so the page renders
// sections in a stable arrangement.
func (a *App) AccountGroups() []domain.AccountGroup {
	v, err := a.store.ListAccountGroups()
	a.logErr("account groups", err)
	sort.SliceStable(v, func(i, j int) bool {
		if v[i].Order != v[j].Order {
			return v[i].Order < v[j].Order
		}
		return v[i].Name < v[j].Name
	})
	return v
}

// GetAccountGroup returns the account group with the given id and whether it exists.
func (a *App) GetAccountGroup(groupID string) (domain.AccountGroup, bool) {
	g, ok, err := a.store.GetAccountGroup(groupID)
	a.logErr("account group", err)
	return g, ok
}

// PutAccountGroup validates and persists an account group, assigning an id when
// the group is new and defaulting its Order to the end of the list. A group needs
// a non-empty name. Membership is de-duplicated, and — because an account belongs
// to at most one group — assigning an account here removes it from every OTHER
// group so the sections stay a partition.
func (a *App) PutAccountGroup(g domain.AccountGroup) (domain.AccountGroup, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return domain.AccountGroup{}, fmt.Errorf("appstate: an account group needs a name")
	}
	isNew := strings.TrimSpace(g.ID) == ""
	if isNew {
		g.ID = id.New()
	}
	g.AccountIDs = dedupeStrings(g.AccountIDs)
	if isNew && g.Order == 0 {
		g.Order = len(a.AccountGroups()) + 1
	}
	if err := a.store.PutAccountGroup(g); err != nil {
		return domain.AccountGroup{}, fmt.Errorf("appstate: save account group: %w", err)
	}
	// Enforce single-membership: drop this group's accounts from any other group.
	if len(g.AccountIDs) > 0 {
		mine := make(map[string]bool, len(g.AccountIDs))
		for _, aid := range g.AccountIDs {
			mine[aid] = true
		}
		for _, other := range a.AccountGroups() {
			if other.ID == g.ID {
				continue
			}
			kept := other.AccountIDs[:0:0]
			changed := false
			for _, aid := range other.AccountIDs {
				if mine[aid] {
					changed = true
					continue
				}
				kept = append(kept, aid)
			}
			if changed {
				other.AccountIDs = kept
				if err := a.store.PutAccountGroup(other); err != nil {
					return domain.AccountGroup{}, fmt.Errorf("appstate: reassign account group membership: %w", err)
				}
			}
		}
	}
	a.log.Info("account group saved", "id", g.ID, "name", g.Name, "accounts", len(g.AccountIDs))
	return g, nil
}

// DeleteAccountGroup removes a group; its accounts simply become ungrouped
// (reassign-on-delete). The accounts themselves are never touched.
func (a *App) DeleteAccountGroup(groupID string) error {
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("appstate: account group id is required")
	}
	return a.del("account group", groupID, a.store.DeleteAccountGroup)
}

// dedupeStrings returns s with empty and duplicate entries removed, order preserved.
func dedupeStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
