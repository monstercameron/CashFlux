// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — accounts-page shared atoms.
//
// Like /transactions, the /accounts page is composed of widget-engine tiles (a
// summary tile, a filter toolbar, a transfer sub-view, the asset list, the
// archived list) rendered through the spec/render pipeline rather than one screen
// embedding everything. The interaction state those tiles share — the active
// search/type filter and whether the page-level transfer form is open — lives here
// as shared atoms so any tile can read or mutate it and every other tile re-renders
// in step.
package uistate

import (
	"slices"
	"strings"

	"github.com/monstercameron/GoWebComponents/v7/state"
)

const (
	acctFilterAtomID   = "accounts:filter"
	acctTransferAtomID = "accounts:transferOpen"
	acctFormulasAtomID = "accounts:showFormulas"
	acctEditAtomID     = "accounts:edit"
	acctCredsAtomID    = "accounts:credentials"
	acctClassStore     = "cashflux:accounts-class"
)

// AccountEdit selects the account + editor a modal should show. A zero value (empty
// ID) means no modal is open. Mode is one of the AcctEditMode* constants.
type AccountEdit struct {
	ID   string
	Mode string
}

// The account editor modes the shell-mounted host can render.
const (
	AcctEditModeEdit      = "edit"      // the account's details form
	AcctEditModeSetBal    = "setbal"    // update the balance or value
	AcctEditModeReconcile = "reconcile" // reconcile to a statement
	AcctEditModeTransfer  = "transfer"  // transfer between accounts
)

// AccountsFilter is the cross-tile filter the toolbar writes and the asset and archived
// list tiles read. It holds a free-text name search, the account types to show, the
// asset or liability view, and whether archived accounts are shown. Owner scoping is
// already handled by the top-bar scope, so it is not repeated here.
type AccountsFilter struct {
	Search       string   // case-insensitive substring match on the account name
	Types        []string // domain.AccountType strings; empty = all types
	Class        string   // domain.AccountClass string ("asset"/"liability"); "" = both
	ShowArchived bool     // reveal the archived-accounts tile
}

// Normalize trims the search text so a stray space never reads as an active filter.
func (f AccountsFilter) Normalize() AccountsFilter {
	f.Search = strings.TrimSpace(f.Search)
	return f
}

// HasNarrowing reports whether a name/type narrow is active (archived visibility is
// a view toggle, not a narrowing of the matched set, so it is excluded).
func (f AccountsFilter) HasNarrowing() bool {
	return f.Search != "" || len(f.Types) > 0
}

// Matches reports whether an account name + type passes the active name/type narrow.
// Archived visibility is handled by the host (which tile an account lands in), not
// here.
func (f AccountsFilter) Matches(name, accType string) bool {
	if len(f.Types) > 0 && !slices.Contains(f.Types, accType) {
		return false
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		if !strings.Contains(strings.ToLower(name), strings.ToLower(s)) {
			return false
		}
	}
	return true
}

// Without returns a copy with the named field cleared, for chip removal. Keys are
// the stable chip ids the toolbar emits ("search", "type" for all types, or "type:<value>" for one).
func (f AccountsFilter) Without(field string) AccountsFilter {
	switch {
	case field == "search":
		f.Search = ""
	case field == "type":
		f.Types = nil
	case strings.HasPrefix(field, "type:"):
		f.Types = slices.DeleteFunc(slices.Clone(f.Types), func(t string) bool { return t == strings.TrimPrefix(field, "type:") })
	}
	return f
}

// ToggleType returns a copy with the account type added to the narrow, or removed
// when it is already chosen. Several types may be chosen at once; none means all.
func (f AccountsFilter) ToggleType(accType string) AccountsFilter {
	if slices.Contains(f.Types, accType) {
		return f.Without("type:" + accType)
	}
	f.Types = append(slices.Clone(f.Types), accType)
	return f
}

// UseAccountsFilter returns the shared atom holding the accounts list filter. The
// toolbar tile writes it; the asset-list and archived-list tiles read it to narrow the
// rows they show. The search, type and archived settings reset on reload. The class
// view (All, Assets or Liabilities) starts from the saved preference, so it survives a
// reload (see PersistAcctClass).
func UseAccountsFilter() state.Atom[AccountsFilter] {
	return state.UseAtom(acctFilterAtomID, AccountsFilter{Class: loadAcctClass()})
}

// PersistAcctClass saves the accounts list class view ("" = All, "asset",
// "liability") so the All/Assets/Liabilities toggle survives reloads. Call it
// alongside setting the filter atom's Class (the atom itself is otherwise
// ephemeral).
func PersistAcctClass(class string) { chromePreferenceSet(acctClassStore, class) }

// loadAcctClass reads the saved accounts class view, defaulting to "" (All) when
// absent.
func loadAcctClass() string { return chromePreferenceGet(acctClassStore) }

// UseAcctTransferOpen returns the shared atom selecting whether the page-level
// transfer form sub-view is open. The toolbar's "Transfer money" action sets it; the
// host swaps the transfer tile into the surface when it is true, mirroring how the
// transactions import/duplicates sub-views fill the main slot.
func UseAcctTransferOpen() state.Atom[bool] { return state.UseAtom(acctTransferAtomID, false) }

// UseAcctShowFormulas returns the shared atom selecting whether the "Account metrics"
// formula tile is revealed. The toolbar's Formulas toggle sets it; the host appends
// the formula tile when it is on. Opt-in so the default page stays focused on the
// accounts themselves, while power users can compute metrics over their account
// aggregates (and number-typed custom fields, which surface as cf_acct_* variables).
func UseAcctShowFormulas() state.Atom[bool] { return state.UseAtom(acctFormulasAtomID, false) }

// UseAccountEdit returns the shared atom selecting which account editor modal is open.
// The row's action buttons set it, and the shell-mounted AccountEditHost reads it and
// renders the matching form in a flip modal. The modal sits at the shell root, not in
// the row, so it centres on the viewport whatever transformed tile ancestors the row
// has. A zero value closes the modal.
func UseAccountEdit() state.Atom[AccountEdit] {
	a := state.UseAtom(acctEditAtomID, AccountEdit{})
	capturedAcctEdit = a
	acctEditCaptured = true
	return a
}

var (
	capturedAcctEdit state.Atom[AccountEdit]
	acctEditCaptured bool
)

// SetAccountEdit opens the account editor modal (id + mode) from outside a component
// render; pass a zero AccountEdit to close. No-op until the host has rendered once.
func SetAccountEdit(e AccountEdit) {
	if acctEditCaptured {
		capturedAcctEdit.Set(e)
	}
}

// CloseAccountEdit clears the account editor atom (closes any open modal).
func CloseAccountEdit() { SetAccountEdit(AccountEdit{}) }

// UseAccountCredentials returns the shared atom holding the id of the account whose
// encrypted-credential modal is open ("" = closed). The row's "Login & credentials"
// action sets it, and the shell-root CredentialVaultHost reads it. It is separate from
// the editor atom because the vault lives in the app package with the encryption code.
func UseAccountCredentials() state.Atom[string] {
	a := state.UseAtom(acctCredsAtomID, "")
	capturedAcctCreds = a
	acctCredsCaptured = true
	return a
}

var (
	capturedAcctCreds state.Atom[string]
	acctCredsCaptured bool
)

// SetAccountCredentials opens (id) or closes ("") the credential modal from outside a
// component render. No-op until the host has rendered once.
func SetAccountCredentials(id string) {
	if acctCredsCaptured {
		capturedAcctCreds.Set(id)
	}
}
