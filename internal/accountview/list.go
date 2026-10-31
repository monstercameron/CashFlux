// SPDX-License-Identifier: MIT

package accountview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
)

// StaleSummary counts the stale balances in a list and says whether the list is
// stale enough that a badge on every row ranks nothing.
type StaleSummary struct {
	// Count is how many of the accounts have a stale balance.
	Count int
	// Collapsed is true once more than half of the accounts are stale. The rows then
	// drop their individual badges and the list leads with one summary instead.
	Collapsed bool
}

// Stale summarises the stale balances among the active accounts at the given
// instant. Archived accounts are neither stale nor counted toward the majority,
// so a long archive cannot hold a stale list below the collapse threshold.
// Callers pass the accounts of the scope they display; Stale does not filter by
// member.
func Stale(accounts []domain.Account, windows freshness.Windows, now time.Time) StaleSummary {
	active := make([]domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if !a.Archived {
			active = append(active, a)
		}
	}
	count := len(freshness.StaleAccounts(active, windows, now))
	return StaleSummary{Count: count, Collapsed: len(active) > 0 && count*2 > len(active)}
}

// ListInput selects what the accounts list shows.
type ListInput struct {
	Accounts []domain.Account
	// MemberID limits the list to one member's accounts and the household's. Empty
	// means everyone.
	MemberID string
	// Class is the requested class view: "" for every account, a domain.AccountClass
	// value for one class, or ServicesClass for the recurring-services view.
	Class string
	// ServicesClass is the Class value that selects the services view. Empty
	// means the caller has no services view, so no Class selects it.
	ServicesClass string
	// Match reports whether an account passes the search and type filters. A nil
	// Match accepts every account.
	Match func(name, accountType string) bool
}

// ListView is the resolved shape of the accounts list.
type ListView struct {
	// Active holds the owner-visible, non-archived accounts, in stored order.
	Active []domain.Account
	// Liabilities holds the liability accounts among Active.
	Liabilities []domain.Account
	// Shown holds the Active accounts that match the class view and the filters, in
	// stored order. It is empty in the services view.
	Shown []domain.Account
	// Class is the class view in effect. It differs from the requested class when
	// there are no liabilities to switch between.
	Class string
	// InServices reports that the services view is in effect.
	InServices bool
	// AssetsOnly reports that the assets-only view is in effect.
	AssetsOnly bool
}

// List resolves the accounts list for a member scope, a class view and a filter.
// Without liabilities the class toggle has nothing to switch between, so the view
// falls back to assets only. The services view stays on once chosen.
func List(in ListInput) ListView {
	var v ListView
	for _, a := range in.Accounts {
		if a.Archived || !Visible(a.OwnerID, in.MemberID) {
			continue
		}
		v.Active = append(v.Active, a)
		if a.Class == domain.ClassLiability {
			v.Liabilities = append(v.Liabilities, a)
		}
	}
	v.Class = in.Class
	services := in.ServicesClass != "" && v.Class == in.ServicesClass
	if !services && len(v.Liabilities) == 0 {
		v.Class = string(domain.ClassAsset)
	}
	v.InServices = services
	v.AssetsOnly = v.Class == string(domain.ClassAsset)
	if v.InServices {
		return v
	}
	for _, a := range v.Active {
		if v.AssetsOnly && a.Class == domain.ClassLiability {
			continue
		}
		if v.Class == string(domain.ClassLiability) && a.Class != domain.ClassLiability {
			continue
		}
		if in.Match == nil || in.Match(a.Name, string(a.Type)) {
			v.Shown = append(v.Shown, a)
		}
	}
	return v
}

// Archived returns the owner-visible archived accounts of every class, in stored
// order. Assets only partitions non-liability accounts, so an archived card or
// loan appears here and nowhere else.
func Archived(accounts []domain.Account, memberID string) []domain.Account {
	var out []domain.Account
	for _, a := range accounts {
		if a.Archived && Visible(a.OwnerID, memberID) {
			out = append(out, a)
		}
	}
	return out
}

// FilterChoices are the options the accounts toolbar offers for a member scope.
type FilterChoices struct {
	// Types lists the account types present among the owner-visible, non-archived
	// accounts, sorted.
	Types []string
	// HasForeign reports whether any owner-visible, non-archived account is held in
	// a currency other than the base.
	HasForeign bool
	// StaleCount is the number of owner-visible, non-archived accounts with a
	// stale balance.
	StaleCount int
}

// FilterOptions derives the toolbar's type choices, foreign-currency flag and stale
// count from the accounts the list can show: those visible to the member scope
// and not archived.
func FilterOptions(accounts []domain.Account, memberID, base string, windows freshness.Windows, now time.Time) FilterChoices {
	seen := map[string]bool{}
	var out FilterChoices
	var shown []domain.Account
	for _, a := range accounts {
		if a.Archived || !Visible(a.OwnerID, memberID) {
			continue
		}
		shown = append(shown, a)
		seen[string(a.Type)] = true
		if a.Currency != "" && a.Currency != base {
			out.HasForeign = true
		}
	}
	for t := range seen {
		out.Types = append(out.Types, t)
	}
	sort.Strings(out.Types)
	out.StaleCount = Stale(shown, windows, now).Count
	return out
}
