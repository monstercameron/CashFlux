// SPDX-License-Identifier: MIT

// Package memberprefs resolves a member's effective preferences by layering their
// personal MemberPrefs over the household defaults (§1.19). It is the typed,
// member-scoped front end to internal/configlayer: the household Settings supply
// the broad layer, the member's Prefs supply the more-specific one, and the most
// specific non-empty value wins. Pure Go (no syscall/js); unit-tested natively.
package memberprefs

import (
	"github.com/monstercameron/CashFlux/internal/configlayer"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Effective is a member's resolved preferences after layering over the household.
type Effective struct {
	DateStyle        string // never empty — falls back to the household value
	DefaultAccountID string // "" when neither member nor household sets one
	DefaultMemberID  string // defaults to the member's own ID when unset
}

// Resolve layers a member's MemberPrefs over the household defaults. householdDateStyle
// is the household's configured date style (e.g. prefs.DateStyle as a string); it is
// the fallback when the member sets no personal date style. The member is their own
// default transaction owner unless they pin a different DefaultMemberID.
func Resolve(member domain.Member, householdDateStyle string) Effective {
	layers := configlayer.Layers{
		Household: map[string]string{"dateStyle": householdDateStyle},
		Member: map[string]string{
			"dateStyle":        member.Prefs.DateStyle,
			"defaultAccountId": member.Prefs.DefaultAccountID,
			"defaultMemberId":  member.Prefs.DefaultMemberID,
		},
	}
	defMember := layers.Resolve("defaultMemberId")
	if defMember == "" {
		defMember = member.ID
	}
	return Effective{
		DateStyle:        layers.Resolve("dateStyle"),
		DefaultAccountID: layers.Resolve("defaultAccountId"),
		DefaultMemberID:  defMember,
	}
}
