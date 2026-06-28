// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// ── HouseholdHub ─────────────────────────────────────────────────────────────

// householdHubProps is intentionally empty; HouseholdHub owns all its state via
// hooks and does not need call-site configuration.
type householdHubProps struct{}

// HouseholdHub is the tabbed /household screen component (FEATURE_MAP §5.3).
// Three tabs:
//
//   - Members  — member list, add/edit, roles, reassign-before-delete
//   - Split    — shared-expense calculator and running settle-up ledger
//   - By person — net-worth-by-owner + spending-by-member analytics
//
// The tab state is owned here; each panel is mounted via ui.CreateElement so its
// hooks run in an isolated component context — the parent hook count stays stable
// regardless of the active tab (GWC no-hooks-in-conditional-parent rule).
func HouseholdHub(props householdHubProps) ui.Node {
	tab := ui.UseState("members")

	// Pre-compute all three panel element descriptors unconditionally (mirrors the
	// Reports() tabbed-view pattern) so the GWC reconciler can mount/unmount each
	// panel component cleanly as the tab changes.
	membersSection := ui.CreateElement(householdMembersPanel, householdMembersPanelProps{})
	splitSection := ui.CreateElement(householdSplitPanel, householdSplitPanelProps{})
	byPersonSection := ui.CreateElement(householdByPersonPanel, householdByPersonPanelProps{})

	seg := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("household.tabAriaLabel"),
		Selected: tab.Get(),
		OnSelect: func(v string) { tab.Set(v) },
		Options: []uiw.SegOption{
			{Value: "members", Label: uistate.T("household.tabMembers")},
			{Value: "split", Label: uistate.T("household.tabSplit")},
			{Value: "byperson", Label: uistate.T("household.tabByPerson")},
		},
	})

	var body ui.Node
	switch tab.Get() {
	case "split":
		body = splitSection
	case "byperson":
		body = byPersonSection
	default: // "members"
		body = membersSection
	}

	return Div(
		Div(css.Class(tw.Mt2, tw.Mb1), seg),
		body,
	)
}

// Household is the screen entry-point for the future /household route. It wraps
// HouseholdHub in a ui.CreateElement call so the hub's hooks run isolated from
// the router shell.
//
// Note: the /household route is intentionally not registered here. A later
// rail-regroup commit will wire it into screens.All() alongside the navigation
// restructure (FEATURE_MAP §5.3 pending rail regroup).
func Household() ui.Node {
	return ui.CreateElement(HouseholdHub, householdHubProps{})
}

// ── Panel: Members ────────────────────────────────────────────────────────────

type householdMembersPanelProps struct{}

// householdMembersPanel delegates to the standalone Members screen so the hub's
// Members tab and the existing /members route share a single implementation.
func householdMembersPanel(_ householdMembersPanelProps) ui.Node {
	return Members()
}

// ── Panel: Split ──────────────────────────────────────────────────────────────

type householdSplitPanelProps struct{}

// householdSplitPanel delegates to the standalone Split screen so the hub's
// Split tab and the existing /split route share a single implementation.
func householdSplitPanel(_ householdSplitPanelProps) ui.Node {
	return Split()
}

// ── Panel: By person ──────────────────────────────────────────────────────────

type householdByPersonPanelProps struct{}

// householdByPersonPanel renders the per-member analytics view:
//   - Net worth by owner — each member's share of household net worth in the base currency
//   - Spending by member — who spent what this period
//   - Income split — equal apportionment of period income across members (when available)
//
// Figures use the same computation path as the /members and /reports screens so the
// numbers are consistent across all three surfaces.
func householdByPersonPanel(_ householdByPersonPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	members := app.Members()
	if len(members) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:   uistate.T("household.byPersonEmpty"),
			CTALabel:  uistate.T("members.addFirst"),
			AddTarget: "member",
		})
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Net worth per owner (member + group-shared), in base currency.
	// Mirrors the computation in Members() so figures are consistent.
	byOwner, _ := ledger.NetByOwner(app.Accounts(), app.Transactions(), rates)
	ownerDisp := func(ownerID string) money.Money {
		v := byOwner[ownerID]
		if v.Currency == "" {
			return money.New(0, base)
		}
		return v
	}
	ownerRows := make([]ui.Node, 0, len(members)+1)
	for _, m := range members {
		v := ownerDisp(m.ID)
		ownerRows = append(ownerRows, Div(css.Class("row"),
			Span(css.Class("row-desc"), m.Name),
			Span(ClassStr("amount "+accentFor(v)), fmtMoney(v)),
		))
	}
	grp := ownerDisp(domain.GroupOwnerID)
	ownerRows = append(ownerRows, Div(css.Class("row"),
		Span(css.Class("row-desc"), uistate.T("owner.group")),
		Span(ClassStr("amount "+accentFor(grp)), fmtMoney(grp)),
	))

	// Spending this period, by member. Uses the shared period window so the
	// figures match /reports when both screens show the same window.
	periodStart, periodEnd := uistate.UsePeriod().Get().Range()
	memberSpend, _ := reports.SpendingByMember(app.Transactions(), periodStart, periodEnd, rates)
	spendByMember := make(map[string]int64, len(memberSpend))
	for _, s := range memberSpend {
		spendByMember[s.MemberID] = s.Amount
	}
	spendRows := make([]ui.Node, 0, len(members)+1)
	for _, m := range members {
		amt := money.New(spendByMember[m.ID], base)
		spendRows = append(spendRows, Div(css.Class("row"),
			Span(css.Class("row-desc"), m.Name),
			Span(css.Class("amount"), fmtMoney(amt)),
		))
	}
	// Unattributed spend (MemberID == "") grouped under the household label.
	if unattr, ok := spendByMember[""]; ok && unattr > 0 {
		amt := money.New(unattr, base)
		spendRows = append(spendRows, Div(css.Class("row"),
			Span(css.Class("row-desc"), uistate.T("owner.group")),
			Span(css.Class("amount"), fmtMoney(amt)),
		))
	}

	// Income split this period: equal apportionment of total household income
	// across non-group members. Suppressed silently on error (e.g. missing FX
	// rates) — same guard as Members() (C279).
	var incomeRows []ui.Node
	if splits, err := allocate.SplitPeriodIncome(app.Transactions(), members, periodStart, periodEnd, base, rates); err == nil && len(splits) > 0 {
		incomeRows = make([]ui.Node, 0, len(splits))
		for _, s := range splits {
			amt := money.New(s.Amount, base)
			incomeRows = append(incomeRows, Div(css.Class("row"),
				Span(css.Class("row-desc"), s.Name),
				Span(css.Class("amount"), fmtMoney(amt)),
			))
		}
	}

	return Div(
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("members.netWorthTitle"),
			Rows:  ownerRows,
		}),
		If(len(spendRows) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("members.spendTitle"),
			Rows:  spendRows,
		})),
		If(len(incomeRows) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("members.incomeSplitTitle"),
			Rows:  incomeRows,
		})),
	)
}
