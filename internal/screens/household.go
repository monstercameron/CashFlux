// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

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
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// ── HouseholdHub ─────────────────────────────────────────────────────────────

// householdHubProps is intentionally empty; HouseholdHub owns all its state via
// hooks and does not need call-site configuration.
type householdHubProps struct{}

// hhFigures bundles the household-level figures the hero and the per-person
// panels share: net worth per owner, spending per member, and income splits,
// all in the base currency over the active period window.
type hhFigures struct {
	Base       string
	Members    []domain.Member
	ByOwner    map[string]money.Money
	SpendByID  map[string]int64
	SpendTotal int64
	Income     []allocate.MemberIncomeSplit
	IncomeOK   bool
}

// ownerWorth returns the owner's net worth, zero-valued in base when unset.
func (f hhFigures) ownerWorth(ownerID string) money.Money {
	v := f.ByOwner[ownerID]
	if v.Currency == "" {
		return money.New(0, f.Base)
	}
	return v
}

// totalWorth sums every owner's net (members + shared) into one household figure.
func (f hhFigures) totalWorth() money.Money {
	var total int64
	for _, v := range f.ByOwner {
		total += v.Amount
	}
	return money.New(total, f.Base)
}

// hhFiguresNow computes the shared household figures for the active period
// window. It reads the shared period atom, so callers must invoke it at a
// stable hook position.
func hhFiguresNow(app *appstate.App, members []domain.Member) hhFigures {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	byOwner, _ := ledger.NetByOwner(app.Accounts(), app.Transactions(), rates)

	periodStart, periodEnd := uistate.UsePeriod().Get().Range()
	memberSpend, _ := reports.SpendingByMember(app.Transactions(), periodStart, periodEnd, rates)
	spendByID := make(map[string]int64, len(memberSpend))
	var spendTotal int64
	for _, s := range memberSpend {
		spendByID[s.MemberID] = s.Amount
		spendTotal += s.Amount
	}

	splits, err := allocate.SplitPeriodIncome(app.Transactions(), members, periodStart, periodEnd, base, rates)
	return hhFigures{
		Base: base, Members: members, ByOwner: byOwner,
		SpendByID: spendByID, SpendTotal: spendTotal,
		Income: splits, IncomeOK: err == nil && len(splits) > 0,
	}
}

// hhTakeaway builds the hero's one-sentence read: who holds the largest share
// of the household's worth, and who did the spending this period.
func hhTakeaway(f hhFigures) string {
	// Largest holder by absolute net worth (shared pot counts as "everything shared").
	var topName string
	var topAbs, memberAbs int64
	for _, m := range f.Members {
		v := f.ownerWorth(m.ID).Amount
		a := v
		if a < 0 {
			a = -a
		}
		memberAbs += a
		if a > topAbs {
			topAbs, topName = a, m.Name
		}
	}
	grpAbs := f.ownerWorth(domain.GroupOwnerID).Amount
	if grpAbs < 0 {
		grpAbs = -grpAbs
	}
	lead := ""
	switch {
	case grpAbs > 0 && grpAbs >= memberAbs:
		lead = uistate.T("hh.holderAll")
	case topName != "":
		lead = uistate.T("hh.holderLead", topName)
	}

	// Spending clause: shared vs the top spender.
	spendClause := uistate.T("hh.spendClauseNone")
	if f.SpendTotal > 0 {
		var topSpendName string
		var topSpend int64
		for _, m := range f.Members {
			if s := f.SpendByID[m.ID]; s > topSpend {
				topSpend, topSpendName = s, m.Name
			}
		}
		if unattr := f.SpendByID[""]; unattr >= f.SpendTotal-unattr {
			spendClause = uistate.T("hh.spendClauseShared")
		} else if topSpendName != "" {
			spendClause = uistate.T("hh.spendClauseTop", topSpendName)
		}
	}
	if lead == "" {
		return spendClause
	}
	return lead + " " + spendClause
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

	members := app.Members()
	f := hhFiguresNow(app, members)
	total := f.totalWorth()

	// ── Hero: the household's one number, its figure chips, and the takeaway. ──
	people := uistate.T("household.peopleN", len(members))
	if len(members) == 1 {
		people = uistate.T("household.peopleOne")
	}
	eyebrow := people + " · " + uistate.T("household.baseSuffix", f.Base) + " · " +
		pr.FormatDate(periodStart) + " – " + pr.FormatDate(periodEnd)

	chips := []ui.Node{
		rptChip(uistate.T("household.chipPeople"), fmt.Sprintf("%d", len(members)), ""),
		rptChip(uistate.T("household.chipSpend"), fmtMoney(money.New(f.SpendTotal, f.Base)), rptToneCls(func() string {
			if f.SpendTotal > 0 {
				return "neg"
			}
			return ""
		}())),
	}
	if f.IncomeOK {
		var incomeTotal int64
		for _, s := range f.Income {
			incomeTotal += s.Amount
		}
		if incomeTotal > 0 {
			chips = append(chips, rptChip(uistate.T("household.chipIncome"), fmtMoney(money.New(incomeTotal, f.Base)), rptToneCls("pos")))
		}
	}
	if grp := f.ownerWorth(domain.GroupOwnerID); grp.Amount != 0 {
		chips = append(chips, rptChip(uistate.T("household.chipShared"), fmtMoney(grp), rptToneCls(accentFor(grp))))
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-hh-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), eyebrow),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("household.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)+rptToneCls(accentFor(total))), Attr("data-countup", ""), fmtMoney(total)),
			),
		),
		Div(css.Class("debt-chips"), chips),
		If(len(members) > 0, P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "hh-takeaway"), hhTakeaway(f))),
	)
	heroTile := rptTile("hh-hero", "1 / span 4", rptSection("", uistate.T("household.heroTitle"), nil, heroBody))

	// ── Toolbar: the three household views. ────────────────────────────────────
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
	toolbar := rptTile("hh-toolbar", "1 / span 4", Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"), seg)))

	// Pre-compute all three panel element descriptors unconditionally (mirrors the
	// Reports() tabbed-view pattern) so the GWC reconciler can mount/unmount each
	// panel component cleanly as the tab changes.
	membersSection := ui.CreateElement(householdMembersPanel, householdMembersPanelProps{Rev: rev})
	splitSection := ui.CreateElement(householdSplitPanel, householdSplitPanelProps{Rev: rev})
	byPersonSection := ui.CreateElement(householdByPersonPanel, householdByPersonPanelProps{Rev: rev})

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
	Rev int // shared data revision — forces re-render after modal saves
}

// householdMembersPanel renders the person roster inside the hub. The per-person
// analytics live on the "By person" tab, so the roster is rendered without its
// standalone analytics sections.
func householdMembersPanel(_ householdMembersPanelProps) ui.Node {
	return rptSection("sec-people", uistate.T("household.rosterTitle"), nil, membersBody())
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
	Rev int // shared data revision — forces re-render after modal saves
}

// hhRowsList wraps pre-built row nodes in the canonical .rows list container —
// the single literal shared by the household surface (rows-container ratchet).
func hhRowsList(rows any) ui.Node {
	return Div(css.Class("rows"), rows)
}

// hhRankedRow is one label + share-bar + amount row for the analytics sections.
// Negative amounts tone the bar in the money-down hue (nw-bar-down).
func hhRankedRow(label string, amt money.Money, maxAbs int64, toneBySign bool) ui.Node {
	a := amt.Amount
	if a < 0 {
		a = -a
	}
	pct := 0
	if maxAbs > 0 {
		pct = int(a * 100 / maxAbs)
	}
	if pct > 100 {
		pct = 100
	}
	fillCls := "share-bar-fill"
	amtCls := "amount"
	if toneBySign && amt.IsNegative() {
		fillCls += " nw-bar-down"
	}
	if toneBySign {
		amtCls += " " + accentFor(amt)
	}
	bar := Div(css.Class("share-bar", "share-bar-thin"),
		Div(ClassStr(fillCls), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
	return Div(css.Class("row"),
		Div(css.Class("row-main"), Span(css.Class("row-desc"), label), bar),
		Span(ClassStr(amtCls), fmtMoney(amt)),
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
	f := hhFiguresNow(app, members)
	return Div(byPersonSections(f)...)
}

// byPersonSections builds the three analytics sections from a shared figure
// set. Used by the hub's By-person tab (and available to the standalone
// /members route).
func byPersonSections(f hhFigures) []any {
	// Net worth by person, ranked by absolute value; the shared pot rides along.
	type entry struct {
		label string
		amt   money.Money
	}
	worth := make([]entry, 0, len(f.Members)+1)
	var maxWorth int64
	for _, m := range f.Members {
		v := f.ownerWorth(m.ID)
		worth = append(worth, entry{m.Name, v})
		if a := v.Amount; a > maxWorth {
			maxWorth = a
		} else if -a > maxWorth {
			maxWorth = -a
		}
	}
	if grp := f.ownerWorth(domain.GroupOwnerID); grp.Amount != 0 || len(f.Members) > 0 {
		worth = append(worth, entry{uistate.T("owner.group"), grp})
		if a := grp.Amount; a > maxWorth {
			maxWorth = a
		} else if -a > maxWorth {
			maxWorth = -a
		}
	}
	worthRows := make([]ui.Node, 0, len(worth))
	for _, e := range worth {
		worthRows = append(worthRows, hhRankedRow(e.label, e.amt, maxWorth, true))
	}

	// Spending by person this period.
	var maxSpend int64
	for _, m := range f.Members {
		if s := f.SpendByID[m.ID]; s > maxSpend {
			maxSpend = s
		}
	}
	if u := f.SpendByID[""]; u > maxSpend {
		maxSpend = u
	}
	spendRows := make([]ui.Node, 0, len(f.Members)+1)
	for _, m := range f.Members {
		spendRows = append(spendRows, hhRankedRow(m.Name, money.New(f.SpendByID[m.ID], f.Base), maxSpend, false))
	}
	if unattr := f.SpendByID[""]; unattr > 0 {
		spendRows = append(spendRows, hhRankedRow(uistate.T("owner.group"), money.New(unattr, f.Base), maxSpend, false))
	}

	take := func(key string) ui.Node {
		return P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), uistate.T(key))
	}
	sections := []any{
		rptSection("sec-worth-by-person", uistate.T("members.netWorthTitle"), nil,
			Fragment(take("members.netWorthTake"), hhRowsList(worthRows))),
	}
	if len(spendRows) > 0 {
		sections = append(sections, rptSection("sec-spend-by-person", uistate.T("members.spendTitle"), nil,
			Fragment(take("members.spendTake"), hhRowsList(spendRows))))
	}
	if f.IncomeOK {
		var maxIncome int64
		for _, s := range f.Income {
			if s.Amount > maxIncome {
				maxIncome = s.Amount
			}
		}
		incomeRows := make([]ui.Node, 0, len(f.Income))
		for _, s := range f.Income {
			incomeRows = append(incomeRows, hhRankedRow(s.Name, money.New(s.Amount, f.Base), maxIncome, false))
		}
		sections = append(sections, rptSection("sec-income-by-person", uistate.T("members.incomeSplitTitle"), nil,
			Fragment(take("members.incomeTake"), hhRowsList(incomeRows))))
	}
	return sections
}
