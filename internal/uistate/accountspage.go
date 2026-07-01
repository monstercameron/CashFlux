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
	"strings"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	acctFilterAtomID   = "accounts:filter"
	acctTransferAtomID = "accounts:transferOpen"
	acctFormulasAtomID = "accounts:showFormulas"
	acctEditAtomID     = "accounts:edit"
)

// AccountEdit selects the account + editor a modal should show. A zero value (empty
// ID) means no modal is open. Mode is one of the AcctEditMode* constants.
type AccountEdit struct {
	ID   string
	Mode string
}

// The account editor modes the shell-mounted host can render.
const (
	AcctEditModeEdit      = "edit"      // full inline-edit form
	AcctEditModeSetBal    = "setbal"    // update balance / value (reconcile-to-target)
	AcctEditModeReconcile = "reconcile" // reconcile to statement
	AcctEditModeTransfer  = "transfer"  // transfer between accounts
)

// AccountsFilter is the cross-tile filter the toolbar writes and the asset/archived
// list tiles read. It is intentionally small: a free-text name search, an optional
// account-type narrow, and whether archived accounts are shown. Owner scoping is
// already handled by the top-bar active scope, so it is deliberately not duplicated
// here.
type AccountsFilter struct {
	Search       string // case-insensitive substring match on the account name
	Type         string // domain.AccountType string; "" = all types
	ShowArchived bool   // reveal the archived-accounts tile
}

// Normalize trims the search text so a stray space never reads as an active filter.
func (f AccountsFilter) Normalize() AccountsFilter {
	f.Search = strings.TrimSpace(f.Search)
	return f
}

// HasNarrowing reports whether a name/type narrow is active (archived visibility is
// a view toggle, not a narrowing of the matched set, so it is excluded).
func (f AccountsFilter) HasNarrowing() bool {
	return f.Search != "" || f.Type != ""
}

// Matches reports whether an account name + type passes the active name/type narrow.
// Archived visibility is handled by the host (which tile an account lands in), not
// here.
func (f AccountsFilter) Matches(name, accType string) bool {
	if f.Type != "" && accType != f.Type {
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
// the stable chip ids the toolbar emits ("search", "type").
func (f AccountsFilter) Without(field string) AccountsFilter {
	switch field {
	case "search":
		f.Search = ""
	case "type":
		f.Type = ""
	}
	return f
}

// UseAccountsFilter returns the shared atom holding the accounts list filter. The
// toolbar tile writes it; the asset-list and archived-list tiles read it to narrow
// the rows they show. It is ephemeral (not persisted): account filtering is a
// transient action, unlike the persisted transaction ledger filter.
func UseAccountsFilter() state.Atom[AccountsFilter] {
	return state.UseAtom(acctFilterAtomID, AccountsFilter{})
}

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

// UseAccountEdit returns the shared atom selecting which account editor modal is open
// (the row's action buttons set it; the shell-mounted AccountEditHost reads it and
// renders the matching form inside a flip modal). It is a shell-root modal — rather
// than an in-row overlay — so it centers on the viewport regardless of the transformed
// bento/tile ancestors a row lives under. A zero value closes the modal.
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
