// SPDX-License-Identifier: MIT

package domain

import (
	"strings"
	"unicode"
)

// IsValuation reports whether an account of this type holds a periodic estimate
// (an investment, retirement, crypto, property, vehicle or other asset) rather
// than a reconciled cash balance. Such accounts read as "Out of date / Update
// value" instead of the banking wording "Stale / Update balance", and have no
// cash curve.
func (t AccountType) IsValuation() bool {
	switch t {
	case TypeInvestment, TypeRetirement, TypeCrypto, TypeProperty, TypeVehicle, TypeOther:
		return true
	default:
		return false
	}
}

// ShowsCashCurve reports whether the account's balance moves as a cash curve:
// a day-by-day history and a forward projection from recurring flows. Liabilities
// and valuation-type assets do not, because their balances are not cash.
func (a Account) ShowsCashCurve() bool {
	return a.Class != ClassLiability && !a.Type.IsValuation()
}

// IsShared reports whether the account is shared with the household: shared
// scope, group-owned, or split by ownership shares.
func (a Account) IsShared() bool {
	return a.Scope == ScopeShared || a.OwnerID == GroupOwnerID || len(a.OwnershipShares) > 0
}

// OwnerName resolves the display name of the member identified by ownerID, or ""
// when the id is empty, group-owned or not a known member.
func OwnerName(members []Member, ownerID string) string {
	if ownerID == "" || ownerID == GroupOwnerID {
		return ""
	}
	for _, m := range members {
		if m.ID == ownerID {
			return m.Name
		}
	}
	return ""
}

// NormalizeInstitution returns the canonical spelling of a free-text institution
// name: surrounding space trimmed, inner runs of space collapsed to one, and the
// first letter of each word upper-cased. Letters after the first are kept as
// typed, so "bmo harris" becomes "Bmo Harris" while "BMO" stays "BMO". Every
// path that stores Account.Institution calls it so the same name never lands in
// two spellings.
func NormalizeInstitution(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// NameLess orders two display names case-insensitively, so "alpha" sorts beside
// "Alpha" instead of after every capitalised name.
func NameLess(a, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}
