// SPDX-License-Identifier: MIT

package domain

// AccountGroup is a user-defined grouping of accounts on the /accounts surface
// (AC1) — "His / Hers / Shared", "Liquid / Invested / Property". A group is a
// VIEW label, not schema: accounts keep their own class/type, and a group is
// simply a named, ordered set of account IDs (the PoolDef shape from the
// investments page, generalized). Deleting a group just ungroups its accounts
// (reassign-on-delete = they fall back to the default "Ungrouped" section); the
// accounts themselves are never touched.
//
// Each group's net subtotal surfaces as a group_<slug>_total engine variable, so
// a grouping like "Retirement" is addressable by name in any formula or widget,
// exactly as investment pools are.
type AccountGroup struct {
	// ID is the stable identifier for the group.
	ID string `json:"id"`
	// Name is the user-facing label ("Shared", "Liquid", "Property").
	Name string `json:"name"`
	// AccountIDs are the member accounts, in the display order the user arranged.
	// An account may appear in at most one group; membership is by ID only.
	AccountIDs []string `json:"accountIds,omitempty"`
	// Order is the group's position among the sections on the page (ascending).
	Order int `json:"order,omitempty"`
	// VarName is an optional explicit variable name for this group in the
	// formula/widget engine. When set, the group's subtotal is exposed as
	// group_<slug(VarName)>_total instead of the name-derived slug. Empty =
	// derive from Name.
	VarName string `json:"varName,omitempty"`
}

// HasAccount reports whether the given account id is a member of the group.
func (g AccountGroup) HasAccount(accountID string) bool {
	for _, id := range g.AccountIDs {
		if id == accountID {
			return true
		}
	}
	return false
}
