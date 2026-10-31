// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
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
// a non-empty name and every account it lists must exist (ErrAccountNotFound
// otherwise). Membership is de-duplicated, and — because an account belongs to at
// most one group — assigning an account here removes it from every OTHER group so
// the sections stay a partition. The group and every group it takes accounts from
// are written in one commit, so a failure leaves all of them as they were. It needs
// edit rights.
func (a *App) PutAccountGroup(g domain.AccountGroup) (domain.AccountGroup, error) {
	if err := a.roleGuard(); err != nil {
		return domain.AccountGroup{}, err
	}
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return domain.AccountGroup{}, fmt.Errorf("appstate: an account group needs a name")
	}
	isNew := strings.TrimSpace(g.ID) == ""
	if isNew {
		g.ID = id.New()
	}
	g.AccountIDs = dedupeStrings(g.AccountIDs)
	accounts, err := a.AccountsChecked()
	if err != nil {
		return domain.AccountGroup{}, fmt.Errorf("appstate: save account group: %w", err)
	}
	known := make(map[string]bool, len(accounts))
	for _, acc := range accounts {
		known[acc.ID] = true
	}
	for _, aid := range g.AccountIDs {
		if !known[aid] {
			return domain.AccountGroup{}, fmt.Errorf("appstate: save account group: %w: %q", ErrAccountNotFound, aid)
		}
	}
	groups, err := a.store.ListAccountGroups()
	if err != nil {
		return domain.AccountGroup{}, fmt.Errorf("appstate: read account groups: %w", err)
	}
	var before []byte
	for _, existing := range groups {
		if existing.ID != g.ID {
			continue
		}
		if before, err = json.Marshal(existing); err != nil {
			return domain.AccountGroup{}, fmt.Errorf("appstate: encode account group %q: %w", existing.ID, err)
		}
	}
	if isNew && g.Order == 0 {
		g.Order = len(groups) + 1
	}
	after, err := json.Marshal(g)
	if err != nil {
		return domain.AccountGroup{}, fmt.Errorf("appstate: encode account group %q: %w", g.ID, err)
	}
	changes := []store.RecordChange{{Table: "accountgroups", ID: g.ID, Before: before, After: after}}
	// Enforce single-membership: drop this group's accounts from any other group.
	mine := make(map[string]bool, len(g.AccountIDs))
	for _, aid := range g.AccountIDs {
		mine[aid] = true
	}
	for _, other := range groups {
		if other.ID == g.ID {
			continue
		}
		kept := make([]string, 0, len(other.AccountIDs))
		for _, aid := range other.AccountIDs {
			if !mine[aid] {
				kept = append(kept, aid)
			}
		}
		if len(kept) == len(other.AccountIDs) {
			continue
		}
		otherBefore, err := json.Marshal(other)
		if err != nil {
			return domain.AccountGroup{}, fmt.Errorf("appstate: encode account group %q: %w", other.ID, err)
		}
		other.AccountIDs = kept
		otherAfter, err := json.Marshal(other)
		if err != nil {
			return domain.AccountGroup{}, fmt.Errorf("appstate: encode account group %q: %w", other.ID, err)
		}
		changes = append(changes, store.RecordChange{Table: "accountgroups", ID: other.ID, Before: otherBefore, After: otherAfter})
	}
	if err := a.store.ReplayChanges(changes); err != nil {
		return domain.AccountGroup{}, fmt.Errorf("appstate: save account group: %w", err)
	}
	a.log.Info("account group saved", "id", g.ID, "name", g.Name, "accounts", len(g.AccountIDs))
	return g, nil
}

// DeleteAccountGroup removes a group; its accounts simply become ungrouped
// (reassign-on-delete). The accounts themselves are never touched. It needs edit
// rights.
func (a *App) DeleteAccountGroup(groupID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
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
