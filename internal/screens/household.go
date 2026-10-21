// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/householdview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ── HouseholdHub ─────────────────────────────────────────────────────────────

// householdHubProps is intentionally empty; HouseholdHub owns all its state via
// hooks and does not need call-site configuration.
type householdHubProps struct{}

// hhTakeaway builds the hero's one-sentence read: who holds the largest share
// of the household's worth, and who did the spending this period. All decisions
// are native prepared facts; unavailable clauses make no financial claim.
func hhTakeaway(f *householdview.Model) string {
	name := func(id string) string {
		for _, member := range f.Members {
			if member.ID == id {
				return member.Name
			}
		}
		return ""
	}
	lead, spending := "", ""
	if f.Holder.Err == nil {
		switch f.Holder.Kind {
		case householdview.LeaderShared:
			lead = uistate.T("hh.holderAll")
		case householdview.LeaderMember:
			lead = uistate.T("hh.holderLead", name(f.Holder.MemberID))
		}
	}
	if f.Spender.Err == nil {
		switch f.Spender.Kind {
		case householdview.LeaderNone:
			spending = uistate.T("hh.spendClauseNone")
		case householdview.LeaderShared:
			spending = uistate.T("hh.spendClauseShared")
		case householdview.LeaderMember:
			spending = uistate.T("hh.spendClauseTop", name(f.Spender.MemberID))
		}
	}
	if lead == "" {
		return spending
	}
	if spending == "" {
		return lead
	}
	return lead + " " + spending
}

// HouseholdHub is the tabbed /household screen (FEATURE_MAP §5.3), redesigned
// in the Understand-surface language: a hero tile carrying the household's net
// worth with figure chips and a plain-English takeaway, a toolbar tile with the
// three views, and per-view section tiles.
//
//   - Members  — the person roster (add/edit, roles, PINs, reassign-before-delete)
//   - Split    — shared-expense calculator and running settle-up ledger
//   - By person — worth / spending / income-split analytics with share bars
//
// The tab state is owned here; each panel is mounted via ui.CreateElement so its
// hooks run in an isolated component context — the parent hook count stays stable
// regardless of the active tab (GWC no-hooks-in-conditional-parent rule).
func HouseholdHub(props householdHubProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	// Re-render whenever any mutation bumps the shared data revision (a member
	// edit, a recorded settlement, a new transaction — all move the figures).
	// The revision also threads into the panel props: empty-struct props NEVER
	// re-render a memoized child, so a modal save would leave the roster stale.
	rev := uistate.UseDataRevision().Get()
	tab := ui.UseState("members")
	pr := uistate.UsePrefs().Get()
	periodStart, periodEnd := uistate.UsePeriod().Get().Range()

	prepared := householdPreparedAt(app, periodStart, periodEnd)
	f := &prepared.Model
	members := prepared.Members

	// ── Hero: the household's one number, its figure chips, and the takeaway. ──
	people := uistate.T("household.peopleN", len(members))
	if len(members) == 1 {
		people = uistate.T("household.peopleOne")
	}
	peopleCount := fmt.Sprintf("%d", len(members))
	if f.RosterErr != nil {
		people = uistate.T("calculation.unavailable")
		peopleCount = people
	}
	baseLabel := uistate.T("household.baseSuffix", f.Base)
	if f.Base == "" {
		baseLabel = uistate.T("calculation.unavailable")
	}
	eyebrow := people + " · " + baseLabel + " · " +
		pr.FormatDate(periodStart) + " – " + pr.FormatDate(periodEnd)

	chips := []ui.Node{
		rptChip(uistate.T("household.chipPeople"), peopleCount, ""),
		rptChip(uistate.T("household.chipSpend"), householdAmountText(f.Spending.Total), rptToneCls(func() string {
			if f.Spending.Total.Err == nil && f.Spending.Total.Value.Amount > 0 {
				return "neg"
			}
			return ""
		}())),
	}
	if f.Income.Applicable && f.Income.Total.Err == nil && f.Income.Total.Value.Amount > 0 {
		chips = append(chips, rptChip(uistate.T("household.chipIncome"), householdAmountText(f.Income.Total), rptToneCls("pos")))
	} else if f.Income.Err != nil || f.Income.Applicable && f.Income.Total.Err != nil {
		chips = append(chips, rptChip(uistate.T("household.chipIncome"), uistate.T("calculation.unavailable"), ""))
	}
	if grp := f.Worth.Owner(domain.GroupOwnerID); grp.Err == nil && grp.Value.Amount != 0 {
		chips = append(chips, rptChip(uistate.T("household.chipShared"), householdAmountText(grp), rptToneCls(householdAmountTone(grp))))
	}
	incomeErr := f.Income.Err
	if incomeErr == nil && f.Income.Applicable {
		incomeErr = f.Income.Total.Err
	}
	takeaway := hhTakeaway(f)

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-hh-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), eyebrow),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("household.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)+rptToneCls(householdAmountTone(f.Worth.Total))), Attr("data-countup", ""), Attr("data-testid", "hh-worth-total"), householdAmountText(f.Worth.Total)),
			),
		),
		Div(css.Class("debt-chips"), chips),
		householdUnavailable("hh-worth-unavailable", f.Worth.Total.Err),
		householdUnavailable("hh-spend-unavailable", f.Spending.Total.Err),
		householdUnavailable("hh-income-unavailable", incomeErr),
		householdUnavailable("hh-roster-unavailable", f.RosterErr),
		If(len(members) > 0 && takeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "hh-takeaway"), takeaway)),
	)
	heroTile := rptTile("hh-hero", "1 / span 4", rptSection("", uistate.T("household.heroTitle"), nil, heroBody))

	// ── Toolbar: the three household views. ────────────────────────────────────
	seg := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("household.tabAriaLabel"),
		Selected: tab.Get(),
		OnSelect: func(v string) { tab.Set(v) },
		Options: []uiw.SegOption{
			{Value: "members", Label: uistate.T("household.tabMembers"), TestID: "hh-tab-members"},
			{Value: "split", Label: uistate.T("household.tabSplit"), TestID: "hh-tab-split"},
			{Value: "byperson", Label: uistate.T("household.tabByPerson"), TestID: "hh-tab-byperson"},
		},
	})
	toolbar := rptTile("hh-toolbar", "1 / span 4", Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"), seg)))

	// Pre-compute all three panel element descriptors unconditionally (mirrors the
	// Reports() tabbed-view pattern) so the GWC reconciler can mount/unmount each
	// panel component cleanly as the tab changes.
	membersSection := ui.CreateElement(householdMembersPanel, householdMembersPanelProps{Rev: rev, Prepared: prepared, Language: string(uistate.ActiveLanguage())})
	splitSection := ui.CreateElement(householdSplitPanel, householdSplitPanelProps{Rev: rev})
	byPersonSection := ui.CreateElement(householdByPersonPanel, householdByPersonPanelProps{Rev: rev, Model: f, Language: string(uistate.ActiveLanguage())})

	var body ui.Node
	switch tab.Get() {
	case "split":
		body = splitSection
	case "byperson":
		body = byPersonSection
	default: // "members"
		body = membersSection
	}
	bodyTile := rptTile("hh-body", "1 / span 4", body)

	return Div(css.Class("bento bento-house"), heroTile, toolbar, bodyTile)
}

// Household is the screen entry-point for the /household route. It wraps
// HouseholdHub in a ui.CreateElement call so the hub's hooks run isolated from
// the router shell.
func Household() ui.Node {
	return ui.CreateElement(HouseholdHub, householdHubProps{})
}

// ── Panel: Members ────────────────────────────────────────────────────────────

type householdMembersPanelProps struct {
	Rev      int // shared data revision — forces re-render after modal saves
	Prepared *householdPrepared
	Language string
}

// householdMembersPanel renders the person roster inside the hub. The per-person
// analytics live on the "By person" tab, so the roster is rendered without its
// standalone analytics sections.
func householdMembersPanel(props householdMembersPanelProps) ui.Node {
	return rptSection("sec-people", uistate.T("household.rosterTitle"), nil, membersBody(props.Prepared))
}

// ── Panel: Split ──────────────────────────────────────────────────────────────

type householdSplitPanelProps struct {
	Rev int // shared data revision — forces re-render after modal saves
}

// householdSplitPanel delegates to the standalone Split screen so the hub's
// Split tab and the existing /split route share a single implementation.
func householdSplitPanel(_ householdSplitPanelProps) ui.Node {
	return Split()
}

// ── Panel: By person ──────────────────────────────────────────────────────────

type householdByPersonPanelProps struct {
	Rev      int // shared data revision — forces re-render after modal saves
	Model    *householdview.Model
	Language string
}

// hhRowsList wraps pre-built row nodes in the canonical .rows list container —
// the single literal shared by the household surface (rows-container ratchet).
func hhRowsList(rows any) ui.Node {
	return Div(css.Class("rows"), rows)
}

// hhRankedRow is one label + share-bar + amount row for the analytics sections.
// Negative amounts tone the bar in the money-down hue (nw-bar-down).
type householdRankedRowProps struct {
	Row        householdview.RankedRow
	ToneBySign bool
}

func hhRankedRow(props householdRankedRowProps) ui.Node {
	row := props.Row
	label := row.Name
	if row.Kind == householdview.RowShared {
		label = uistate.T("owner.group")
	}
	fillCls := "share-bar-fill"
	amtCls := "amount"
	if props.ToneBySign && row.Amount.Err == nil && row.Amount.Value.IsNegative() {
		fillCls += " nw-bar-down"
	}
	if props.ToneBySign {
		amtCls += " " + householdAmountTone(row.Amount)
	}
	bar := Div(css.Class("share-bar", "share-bar-thin"),
		If(row.Percent.Err == nil, Div(ClassStr(fillCls), Style(map[string]string{"width": fmt.Sprintf("%d%%", row.Percent.Value)}))))
	return Div(css.Class("row"), Attr("data-testid", "hh-row-"+row.Key),
		Div(css.Class("row-main"), Span(css.Class("row-desc"), label), bar),
		Span(ClassStr(amtCls), Attr("data-testid", "hh-amount-"+row.Key), householdAmountText(row.Amount)),
	)
}

// householdByPersonPanel renders the per-member analytics view in the
// Understand language — each section leads with a quiet takeaway and ranks its
// rows with share bars:
//   - Net worth by person — each member's share of household net worth
//   - Spending by person — who spent what this period
//   - Income split — equal apportionment of period income across members
//
// Figures use the same computation path as the roster and /reports so the
// numbers are consistent across all three surfaces.
func householdByPersonPanel(props householdByPersonPanelProps) ui.Node {
	f := props.Model
	if f == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	if f.RosterErr != nil {
		return Div(householdUnavailable("hh-by-person-roster-unavailable", f.RosterErr), Fragment(byPersonSections(f)...))
	}
	if len(f.Members) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:   uistate.T("household.byPersonEmpty"),
			CTALabel:  uistate.T("members.addFirst"),
			AddTarget: "member",
		})
	}
	return Div(byPersonSections(f)...)
}

// byPersonSections builds the three analytics sections from a shared figure
// set. Used by the hub's By-person tab (and available to the standalone
// /members route).
func byPersonSections(f *householdview.Model) []any {
	rows := func(items []householdview.RankedRow, tone bool) ui.Node {
		return hhRowsList(MapKeyed(items, func(row householdview.RankedRow) any { return row.Key }, func(row householdview.RankedRow) ui.Node {
			return ui.CreateElement(hhRankedRow, householdRankedRowProps{Row: row, ToneBySign: tone})
		}))
	}
	take := func(key string) ui.Node {
		return P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), uistate.T(key))
	}
	sections := []any{
		rptSection("sec-worth-by-person", uistate.T("members.netWorthTitle"), nil,
			Fragment(take("members.netWorthTake"), householdUnavailable("hh-worth-rows-unavailable", f.Worth.Total.Err), rows(f.WorthRows, true))),
	}
	if len(f.SpendRows) > 0 || f.Spending.Err != nil {
		sections = append(sections, rptSection("sec-spend-by-person", uistate.T("members.spendTitle"), nil,
			Fragment(take("members.spendTake"), householdUnavailable("hh-spend-rows-unavailable", f.Spending.Total.Err), rows(f.SpendRows, false))))
	}
	if f.Income.Applicable || f.Income.Err != nil {
		sections = append(sections, rptSection("sec-income-by-person", uistate.T("members.incomeSplitTitle"), nil,
			Fragment(take("members.incomeTake"), householdUnavailable("hh-income-rows-unavailable", f.Income.Total.Err), rows(f.IncomeRows, false))))
	}
	return sections
}

func householdUnavailable(testID string, err error) ui.Node {
	if err == nil {
		return nil
	}
	return P(css.Class("muted"), Attr("role", "status"), Attr("data-testid", testID), uistate.T("calculation.unavailable")+": "+err.Error())
}
