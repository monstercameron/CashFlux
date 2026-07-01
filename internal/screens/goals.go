// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// goalView is the shared computed picture of the goals surface — the owner-scoped,
// partitioned, sorted goal lists plus the headline totals. Both the summary and the
// list tiles compute it, mirroring computeBudgetView on /budgets.
type goalView struct {
	Base        string
	Accounts    []domain.Account
	Members     []domain.Member
	Categories  []domain.Category
	All         []domain.Goal // owner-visible goals (all sections)
	Active      []domain.Goal // non-archived, non-fund; sorted most-actionable first
	Fund        []domain.Goal // sinking funds; alphabetical
	Achieved    []domain.Goal // archived; alphabetical
	SavedTotal  money.Money   // Σ saved across active goals (base currency)
	TargetTotal money.Money   // Σ target across active goals (base currency)
	OverallPct  int           // combined saved/target percent across active goals
}

// computeGoalView assembles the goalView for the active member view: owner-scoped,
// partitioned into active / sinking-fund / achieved, sorted, with the headline totals.
// Pure aggregation over the internal/goals package (no hooks, no mutation).
func computeGoalView(app *appstate.App, activeMemberID string) goalView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	v := goalView{Base: base, Accounts: app.Accounts(), Members: app.Members(), Categories: app.Categories()}
	for _, g := range app.Goals() {
		if ownerVisibleTo(g.OwnerID, activeMemberID) {
			v.All = append(v.All, g)
		}
	}
	for _, g := range v.All {
		switch {
		case g.Archived:
			v.Achieved = append(v.Achieved, g)
		case g.IsSinkingFund:
			v.Fund = append(v.Fund, g)
		default:
			v.Active = append(v.Active, g)
		}
	}
	// Active: most actionable first (nearest date → highest %); funds/achieved: alpha.
	sort.SliceStable(v.Active, func(i, j int) bool { return goalsvc.LessForList(v.Active[i], v.Active[j]) })
	sort.SliceStable(v.Fund, func(i, j int) bool { return v.Fund[i].Name < v.Fund[j].Name })
	sort.SliceStable(v.Achieved, func(i, j int) bool { return v.Achieved[i].Name < v.Achieved[j].Name })

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	v.SavedTotal, v.TargetTotal = goalsvc.Totals(v.Active, rates, base, false)
	v.OverallPct, _ = goalsvc.OverallProgress(v.Active, false)
	return v
}

type goalRowProps struct {
	Goal           domain.Goal
	Accounts       []domain.Account
	Members        []domain.Member
	OnDelete       func(string)
	OnContribute   func(domain.Goal, string, bool) // goal, amountStr, postLedger
	OnSave         func(id, name, target, date, accountID, owner string)
	OnDrillAccount func(accountID string)        // open Transactions filtered to the linked account
	OnArchive      func(id string, archive bool) // move goal to/from the Achieved section
	OnRedirect     func()                        // a completed goal frees its monthly — jump to Allocate (L20)
	// C189/C192: sinking-fund display data (zero values = not a fund / no link).
	// FundSetAside is the monthly set-aside in minor units (from FundSetAsideMinor).
	// LinkedCategoryName is the resolved name of CategoryID (empty when unlinked).
	FundSetAside       int64
	LinkedCategoryName string
}

// goalAccountOptions builds the linked-account SelectOptions for a goal, with a
// leading "no link" choice.
func goalAccountOptions(accounts []domain.Account, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noLink")}}
	for _, a := range accounts {
		opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
	}
	return opts
}

// goalCategoryOptions builds the linked-category SelectOptions for a goal/fund,
// with a leading "no link" choice. Used by goaladdform and the inline edit (C192).
func goalCategoryOptions(categories []domain.Category, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noCategoryLink")}}
	for _, c := range categories {
		opts = append(opts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	return opts
}

// accountName returns an account's name by id, or "" when not found.
func accountName(accounts []domain.Account, id string) string {
	if a, ok := domain.AccountByID(accounts, id); ok {
		return a.Name
	}
	return ""
}

// categoryNameByID returns a category's name by id, or "" when not found.
func categoryNameByID(categories []domain.Category, id string) string {
	for _, c := range categories {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// csvImportSummary builds the confirmation message after a CSV import, naming the
// destination account when one is known (C10): "Imported N transactions into
// <Account>." Falls back to the account-less phrasing for an unselected target.
func csvImportSummary(accounts []domain.Account, acctID string, n int) string {
	txns := plural(n, "transaction")
	if name := accountName(accounts, acctID); name != "" {
		return uistate.T("documents.importedCsvInto", txns, name)
	}
	return uistate.T("documents.importedCsv", txns)
}

// barFillStyle is the inline width for a goal's progress bar. The fill *tone* is
// driven by a CSS state class (see paceBarClass) so a near-complete, behind, or
// on-track goal reads differently at a glance instead of one flat accent (G5/C51).
func barFillStyle(pct int) string {
	return fmt.Sprintf("width:%d%%", pct)
}

// paceBarClass maps a goal's pace to a progress-bar fill modifier class. The
// classes (final/overdue/soon) are defined in the shared stylesheet; an empty
// modifier keeps the default accent for on-track / undated goals.
func paceBarClass(p goalsvc.Pace) string {
	switch p {
	case goalsvc.PaceComplete:
		return "done"
	case goalsvc.PaceFinalStretch:
		return "final"
	case goalsvc.PaceOverdue:
		return "overdue"
	case goalsvc.PaceDueSoon:
		return "soon"
	default:
		return ""
	}
}

// paceBadge renders a compact colored badge for a goal's pace, or an empty
// fragment when there's nothing to flag (undated, comfortably on track without a
// near-term signal). It answers Aaliyah's "am I on pace?" at a glance (G5).
func paceBadge(p goalsvc.Pace) ui.Node {
	var label, mod string
	switch p {
	case goalsvc.PaceFinalStretch:
		label, mod = uistate.T("goals.paceFinal"), "final"
	case goalsvc.PaceOverdue:
		label, mod = uistate.T("goals.paceOverdue"), "overdue"
	case goalsvc.PaceDueSoon:
		label, mod = uistate.T("goals.paceDueSoon"), "soon"
	case goalsvc.PaceOnTrack:
		label, mod = uistate.T("goals.paceOnTrack"), "ontrack"
	default:
		return Fragment()
	}
	return Span(ClassStr("pace-badge pace-"+mod), label)
}
