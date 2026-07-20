// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// rosterClass classifies a flow for the money-based lens.
//
// The lenses are FILTERS, NOT A PARTITION. "Bills" means anchored to a liability
// the payment settles — never domain.Recurring.AccountID, which names the funding
// account an occurrence posts FROM and which nearly every flow carries (reading
// it as the anchor once made "bills" match everything and left the Subscriptions
// lens permanently empty). "Subscriptions" is not the complement of that: it is
// its own claim, made positively by internal/subscriptions.
//
// A commitment that is neither — HOA dues, property tax, insurance — belongs to
// no lens and returns "", so it appears under All only. That is the intended
// outcome: a lens named Subscriptions that quietly holds the property tax bill
// would be worse than one that holds nothing.
func rosterClass(r domain.Recurring, anchored, subscription bool) string {
	switch {
	case !r.Amount.IsNegative():
		return "income"
	case anchored:
		return "bills"
	case subscription:
		return "subs"
	default:
		return ""
	}
}

// rhyRosterProps configures the lineup roster.
type rhyRosterProps struct {
	Focus rhythmFocus
	Acts  rhyActions
}

// rhyRosterSection is the weight-first lineup: lenses + sort picker over the
// claim rows, with the watching-after-cancellation tail. Its own component so
// the lens/sort state stays isolated.
func rhyRosterSection(props rhyRosterProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	now := time.Now()

	initLens := "all"
	if props.Focus == focusSubs {
		initLens = "subs"
	}
	lens := ui.UseState(initLens)
	sortKey := ui.UseState("size")
	onSort := ui.UseEvent(func(v string) { sortKey.Set(v) })

	v := recurViewOf(app, now)
	base := v.Base
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Price-creep percentages by flow name (positive changes only).
	creep := map[string]int{}
	if changes, err := subscriptions.DetectPriceChanges(app.Transactions(), rates, 3); err == nil {
		for _, c := range changes {
			if c.PercentChange > 0 {
				creep[strings.ToLower(strings.TrimSpace(c.Name))] = c.PercentChange
			}
		}
	}
	// Cancelled-but-present flows keep watching.
	watching := map[string]bool{}
	for _, c := range app.Cancellations() {
		watching[strings.ToLower(strings.TrimSpace(c.SubName))] = true
	}

	// The Bills lens asks the SAME question the agenda already answers when it
	// collapses a liability's statement onto the flow that pays it, so it asks it
	// through the same helper rather than inventing a second definition.
	anchors := bills.LiabilityAnchors(app.Accounts(), app.Recurring(), now, now.AddDate(0, 0, agendaWindowDays))
	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	confirmedSubs := loadConfirmedSubs()

	// Lens filter.
	cur := lens.Get()
	var filtered []domain.Recurring
	var subsMonthly int64
	for _, r := range v.Flows {
		_, anchored := anchors[r.ID]
		isSub := subscriptions.IsSubscriptionCommitment(r.Label, catName[r.CategoryID], confirmedSubs)
		cl := rosterClass(r, anchored, isSub)
		if cl == "subs" {
			subsMonthly += -r.MonthlyEquivalent()
		}
		if cur != "all" && cl != cur {
			continue
		}
		filtered = append(filtered, r)
	}

	// Sort.
	sortFlows(filtered, sortKey.Get(), creep)

	// Split off the watching tail.
	var main, tail []domain.Recurring
	for _, r := range filtered {
		if watching[strings.ToLower(strings.TrimSpace(r.Label))] {
			tail = append(tail, r)
		} else {
			main = append(main, r)
		}
	}

	rowOf := func(r domain.Recurring, isWatching bool) ui.Node {
		total := v.MonthlyOut
		if !r.Amount.IsNegative() {
			total = v.MonthlyIn
		}
		return ui.CreateElement(rhyClaimRow, rhyClaimRowProps{
			R: r, Base: base, ShareTotal: total, VarPrefix: v.VarPrefixByID[r.ID],
			HasBudget: r.CategoryID != "" && v.BudgetedCats[r.CategoryID],
			CreepPct:  creep[strings.ToLower(strings.TrimSpace(r.Label))],
			Watching:  isWatching, Acts: props.Acts,
		})
	}

	var body ui.Node
	if len(filtered) == 0 {
		body = P(css.Class("muted"), Attr("data-testid", "rhy-roster-none"), uistate.T("rhythm.rosterNone"))
	} else {
		rows := []any{css.Class("rhy-roster-list"), Attr("role", "list"), Attr("data-testid", "rhy-roster")}
		for _, r := range main {
			rows = append(rows, rowOf(r, false))
		}
		list := Div(rows...)
		var tailNode ui.Node = Fragment()
		if len(tail) > 0 {
			twargs := []any{css.Class("rhy-roster-list")}
			for _, r := range tail {
				twargs = append(twargs, rowOf(r, true))
			}
			tailNode = Details(css.Class("rhy-watch-group"),
				Summary(css.Class("rhy-watch-summary"), uistate.T("rhythm.watchTail", len(tail))),
				Div(twargs...),
			)
		}
		body = Fragment(list, tailNode)
	}

	setLens := func(val string) any { return Prevent(func() { lens.Set(val) }) }
	lenses := Div(css.Class("rhy-lenses"),
		rhyLensBtn(uistate.T("rhythm.lensAll"), "recurring-tab-scheduled", cur == "all", setLens("all")),
		rhyLensBtn(uistate.T("rhythm.lensBills"), "recurring-tab-bills", cur == "bills", setLens("bills")),
		rhyLensBtn(uistate.T("rhythm.lensSubs"), "recurring-tab-subscriptions", cur == "subs", setLens("subs")),
		rhyLensBtn(uistate.T("rhythm.lensIncome"), "rhy-lens-income", cur == "income", setLens("income")),
		If(subsMonthly > 0, Span(css.Class("rhy-lens-sub"), Attr("data-testid", "rhy-subs-subtotal"),
			uistate.T("rhythm.subsSubtotal", fmtMoney(money.New(subsMonthly, base)), fmtMoney(money.New(subsMonthly*12, base))))),
	)
	sortPick := Select(css.Class("field field-sm"), Attr("data-testid", "rhy-sort"),
		Attr("aria-label", uistate.T("rhythm.sortAria")), Value(sortKey.Get()), OnChange(onSort),
		Option(Value("size"), uistate.T("rhythm.sortSize")),
		Option(Value("next"), uistate.T("rhythm.sortNext")),
		Option(Value("name"), uistate.T("rhythm.sortName")),
		Option(Value("trend"), uistate.T("rhythm.sortTrend")),
	)
	return rhySection("sec-roster", uistate.T("rhythm.rosterTitle"), uistate.T("rhythm.rosterNote"), sortPick,
		Fragment(lenses, body))
}

// rhyLensBtn renders one money-based lens toggle.
func rhyLensBtn(label, testid string, on bool, onClick any) ui.Node {
	cls := "rhy-lens"
	if on {
		cls += " is-on"
	}
	return Button(ClassStr(cls), Type("button"), Attr("data-testid", testid), Attr("aria-pressed", ariaBool(on)),
		OnClick(onClick), label)
}

// sortFlows orders the roster by the chosen key (largest monthly first by
// default), all stable and deterministic.
func sortFlows(flows []domain.Recurring, key string, creep map[string]int) {
	switch key {
	case "next":
		sort.SliceStable(flows, func(i, j int) bool { return flows[i].NextDue.Before(flows[j].NextDue) })
	case "name":
		sort.SliceStable(flows, func(i, j int) bool { return strings.ToLower(flows[i].Label) < strings.ToLower(flows[j].Label) })
	case "trend":
		sort.SliceStable(flows, func(i, j int) bool {
			ci := creep[strings.ToLower(strings.TrimSpace(flows[i].Label))]
			cj := creep[strings.ToLower(strings.TrimSpace(flows[j].Label))]
			return ci > cj
		})
	default: // size
		sort.SliceStable(flows, func(i, j int) bool {
			return absMonthly(flows[i]) > absMonthly(flows[j])
		})
	}
}

func absMonthly(r domain.Recurring) int64 {
	me := r.MonthlyEquivalent()
	if me < 0 {
		return -me
	}
	return me
}

// rhyClaimRowProps drives one roster claim row.
type rhyClaimRowProps struct {
	R          domain.Recurring
	Base       string
	ShareTotal int64
	VarPrefix  string
	HasBudget  bool
	CreepPct   int
	Watching   bool
	Acts       rhyActions
}

// rhyClaimRow renders one weight-first roster row: a %-of-outflow spine, the
// name with mode/anchor/creep chips, the normalized /mo figure, and a kebab of
// row actions (destructive ones only under ⋯). Its own component so the menu
// hooks stay stable in the list.
func rhyClaimRow(props rhyClaimRowProps) ui.Node {
	r := props.R
	a := props.Acts

	editItem := ui.UseEvent(Prevent(func() { a.OnEdit(r.ID) }))
	viewTxnsItem := ui.UseEvent(Prevent(func() { a.OnViewTxns(r) }))
	viewBudgetItem := ui.UseEvent(Prevent(func() { a.OnViewBudget(r) }))
	viewAcctItem := ui.UseEvent(Prevent(func() { a.OnViewAccount(r.AccountID) }))
	delItem := ui.UseEvent(Prevent(func() { a.OnDelete(r.ID) }))
	pauseItem := ui.UseEvent(Prevent(func() { a.OnPauseToggle(r) }))
	cancelItem := ui.UseEvent(Prevent(func() { a.OnCancelWatch(r) }))
	copyVarItem := ui.UseEvent(Prevent(func() { a.OnCopyVar(props.VarPrefix + "monthly") }))
	anchorClick := ui.UseEvent(Prevent(func() { a.OnViewAccount(r.AccountID) }))

	pauseLabel := uistate.T("rhythm.pause")
	if r.Paused {
		pauseLabel = uistate.T("rhythm.resume")
	}
	items := []ui.Node{
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-edit-"+r.ID), Title(uistate.T("recurring.editTitle")), OnClick(editItem), uistate.T("recurring.editTitle")),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewtxns-"+r.ID), Title(uistate.T("recurring.viewTxnsTitle")), OnClick(viewTxnsItem), uistate.T("recurring.viewTxns")),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "rhy-pause-"+r.ID), OnClick(pauseItem), pauseLabel),
	}
	if props.HasBudget {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewbudget-"+r.ID), OnClick(viewBudgetItem), uistate.T("recurring.viewBudget")))
	}
	if r.AccountID != "" {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewacct-"+r.ID), OnClick(viewAcctItem), uistate.T("recurring.viewAccount")))
	}
	if !props.Watching && r.Amount.IsNegative() {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "rhy-cancel-"+r.ID), OnClick(cancelItem), uistate.T("rhythm.cancelWatch")))
	}
	if props.VarPrefix != "" {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "rhy-copyvar-"+r.ID), Title(uistate.T("recurring.varHint")), OnClick(copyVarItem), uistate.T("rhythm.copyVar")))
	}
	items = append(items, Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"),
		Attr("data-testid", "recurring-del-"+r.ID), Title(uistate.T("recurring.deleteTitle")), OnClick(delItem), uistate.T("recurring.deleteTitle")))

	// Spine: share of the relevant monthly total.
	me := r.MonthlyEquivalent()
	pct := 0.0
	if props.ShareTotal > 0 {
		pct = float64(absMonthly(r)) / float64(props.ShareTotal) * 100
	}
	// Income is a share of income, not of outflow — saying "of outflow" beside a
	// paycheck reads as a contradiction.
	shareKey := "recurring.shareLabel"
	if !r.Amount.IsNegative() {
		shareKey = "rhythm.shareOfIn"
	}
	spine := Div(css.Class("rhy-spine"),
		Span(css.Class("rhy-spine-pct"), uistate.T(shareKey, pct)),
		Div(css.Class("rhy-spine-track"), Div(css.Class("rhy-spine-fill"), Style(map[string]string{"width": pctWidth(pct)}))),
	)

	chips := []any{css.Class("rhy-claim-head")}
	chips = append(chips, Span(css.Class("rhy-claim-name"), r.Label))
	label, hint, cls := postingMode(r)
	chips = append(chips, Span(ClassStr("rhy-badge "+cls), Title(hint), label))
	if r.Paused {
		chips = append(chips, Span(css.Class("rhy-badge"), uistate.T("rhythm.pausedTag")))
	}
	if r.AccountID != "" {
		chips = append(chips, Button(css.Class("rhy-chip is-anchor"), Type("button"),
			Attr("data-testid", "rhy-anchor-"+r.ID), Title(uistate.T("rhythm.anchorTitle", rhyAccountName(r.AccountID))),
			OnClick(anchorClick), rhyAccountName(r.AccountID)))
	}
	if props.CreepPct > 0 {
		chips = append(chips, Span(css.Class("rhy-chip is-creep"), Title(uistate.T("rhythm.creepTitle")),
			"↗ "+uistate.T("recurring.shareLabel", float64(props.CreepPct))))
	}
	if props.Watching {
		chips = append(chips, Span(css.Class("rhy-badge is-watch"), uistate.T("rhythm.watchStatus")))
	}

	perMonth := uistate.T("rhythm.perMonth", fmtMoney(money.New(me, props.Base)))
	nextLine := recurCadence(r.Cadence) + " · " + uistate.T("rhythm.nextOn", uistate.LoadPrefs().FormatDate(r.NextDue))

	return Div(css.Class("rhy-claim"), Attr("role", "listitem"), Attr("data-testid", "rhy-claim-"+r.ID),
		spine,
		Div(css.Class("rhy-claim-main"),
			Div(chips...),
			Span(css.Class("rhy-claim-meta"), nextLine),
		),
		// The cadence already reads on the meta line ("Monthly · next Aug 1"), so the
		// amount column carries only the normalized figure — no second "Monthly".
		Div(css.Class("rhy-claim-amt"),
			Div(ClassStr("rhy-claim-per "+tw.Fold(tw.FontDisplay)+" "+recurAmountTone(money.New(me, props.Base))), perMonth),
		),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "recurring-menu-" + r.ID,
			AriaLabel:    uistate.T("recurring.moreActions"),
			ToggleTestID: "recurring-menu-" + r.ID,
			WrapClass:    "rhy-claim-menu",
			Items:        items,
		}),
	)
}

// pctWidth clamps a percent to a CSS width string.
func pctWidth(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return money.FormatMinor(int64(pct*100), 2) + "%"
}

// accountName resolves an account's display name for the anchor chip.
func rhyAccountName(accountID string) string {
	if app := appstate.Default; app != nil {
		for _, ac := range app.Accounts() {
			if ac.ID == accountID {
				return ac.Name
			}
		}
	}
	return accountID
}
