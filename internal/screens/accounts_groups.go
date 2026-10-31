// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// --- grouping ---------------------------------------------------------------------

// acctGroupSection is one rendered section on the /accounts list: a group header
// (empty ID = the default "Ungrouped" section) plus the member accounts, already
// filtered to the visible set, with the group's net subtotal in the base currency.
type acctGroupSection = accountview.GroupSection

// groupSections splits the visible accounts into group sections. It reuses the shared
// account snapshot, so grouping adds no ledger scans.
func groupSections(shown []domain.Account, groups []domain.AccountGroup, balances map[string]money.Money, rates currency.Rates, base string) []acctGroupSection {
	return accountview.Groups(shown, groups, balances, rates, base)
}

// --- collapsible group sections ---------------------------------------------------

// ungroupedCollapseID is the stable sentinel key for the "Ungrouped" catch-all,
// whose group ID is the empty string; used so its collapsed state persists too.
const ungroupedCollapseID = "_ungrouped"

// acctGroupCollapseKey is the preserved-settings KV key holding whether the group
// with the given ID is collapsed on the accounts list. Persisted per group so a
// household's chosen shape survives reloads (durable via RequestPersist).
func acctGroupCollapseKey(groupID string) string {
	if groupID == "" {
		groupID = ungroupedCollapseID
	}
	return "cashflux:acct-grp-collapsed:" + groupID
}

// isAcctGroupCollapsed reports whether the group section is collapsed. Read fresh
// on every list render (the toggle bumps the shared data revision to re-render).
func isAcctGroupCollapsed(groupID string) bool {
	return uistate.SettingKVGet(acctGroupCollapseKey(groupID)) == "1"
}

// toggleAcctGroupCollapsed flips and persists a group section's collapsed state,
// then bumps the data revision so the list tile (and this header) re-render with
// the new shape. A plain func — the header component owns the click hook.
func toggleAcctGroupCollapsed(groupID string) {
	if isAcctGroupCollapsed(groupID) {
		uistate.SettingKVDelete(acctGroupCollapseKey(groupID))
	} else {
		uistate.SettingKVSet(acctGroupCollapseKey(groupID), "1")
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
}

// acctGroupHeader renders a group's header row: a collapse/expand toggle wrapping
// its name and member count, a net subtotal, and, for a real group (non-empty ID),
// an edit button that opens the group editor. It is its own component so the click
// hooks stay stable across a variable list of groups. The Collapsed prop is passed
// through so the memoized header re-renders when the collapsed state changes.
type acctGroupHeaderProps struct {
	Section   acctGroupSection
	Collapsed bool
}

func acctGroupHeader(props acctGroupHeaderProps) ui.Node {
	g := props.Section.Group
	name := g.Name
	isReal := g.ID != ""
	if !isReal {
		name = uistate.T("accounts.ungroupedSection")
	}
	edit := uistate.UseAccountGroupEdit()
	openEdit := ui.UseEvent(Prevent(func() { edit.Set(g.ID) }))
	toggle := ui.UseEvent(Prevent(func() { toggleAcctGroupCollapsed(g.ID) }))

	editBtn := If(isReal, Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
		Attr("data-testid", "acct-group-edit-"+g.ID),
		AriaLabel(uistate.T("accounts.editGroupTitle")),
		Title(uistate.T("accounts.editGroupTitle")), OnClick(openEdit),
		uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4))))

	chevron := icon.ChevronDown
	toggleTitle := uistate.T("accountsGroup.collapse")
	if props.Collapsed {
		chevron = icon.ChevronRight
		toggleTitle = uistate.T("accountsGroup.expand")
	}
	toggleTestID := "acct-group-toggle-" + g.ID
	if !isReal {
		toggleTestID = "acct-group-toggle-" + ungroupedCollapseID
	}
	subtotalNote := ""
	if props.Section.Incomplete {
		subtotalNote = uistate.T("accounts.subtotalExcluded")
		if len(props.Section.ExcludedCurrencies) > 0 {
			subtotalNote = uistate.T("accounts.subtotalMissingRates", strings.Join(props.Section.ExcludedCurrencies, ", "))
		}
	}

	return Div(css.Class("acct-group-header", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Role("heading"), Attr("aria-level", "3"),
		AriaLabel(uistate.T("accounts.groupRowAria", name)),
		// The whole label (chevron, name and count) is the collapse toggle: a wide,
		// keyboard-reachable target that shows or hides the section's rows.
		// aria-expanded reflects the open state. It is styled inline as bare text with a
		// chevron so it reads as a section header rather than a default button.
		Button(css.Class("acct-group-toggle", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			css.Class(css.New(css.Bg(css.Color("transparent")), css.BorderStyle(css.LineNone), css.Padding(css.Length("0")), css.Margin(css.Length("0")), tw.CursorPointer, css.TextColor(css.Color("var(--text)")), tw.TextLeft)),
			Attr("data-testid", toggleTestID), Title(toggleTitle),
			AriaExpanded(!props.Collapsed),
			AriaLabel(uistate.T("accountsGroup.toggleAria", name)),
			OnClick(toggle),
			uiw.Icon(chevron, css.Class("acct-group-chevron", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-group-name", tw.FontMedium), name),
			Span(css.Class("acct-group-count", tw.TextDim), plural(len(props.Section.Accounts), "account")),
		),
		Span(css.Class("acct-group-subtotal", tw.TextDim, tw.MlAuto),
			AriaLabel(uistate.T("accounts.groupSubtotalAria", fmtMoney(props.Section.Subtotal))),
			uistate.T("accounts.groupSubtotal", fmtMoney(props.Section.Subtotal)),
			If(props.Section.Incomplete, Span(css.Class("t-caption", tw.TextDim),
				Attr("data-testid", "acct-group-incomplete-"+g.ID), Title(subtotalNote), " · "+subtotalNote))),
		editBtn,
	)
}

// --- balance sparkline and flow figures -------------------------------------------

// sparklineW / sparklineH are the fixed inline-SVG geometry for a row sparkline.
const sparklineW, sparklineH = 120.0, 24.0

// seriesFlat reports whether every point in the series equals the first — a flat
// run is itself the "nothing has posted since your last update" signal.
func seriesFlat(series []int64) bool {
	for _, v := range series {
		if v != series[0] {
			return false
		}
	}
	return true
}

// sparklineFigure renders a balance series as a captioned inline-SVG polyline, with no
// chart library. ariaLabel names the series for screen readers, and caption is shown
// beneath it so the small line reads as a designed figure. Fewer than two points
// renders nothing.
func sparklineFigure(a domain.Account, series []int64, ariaLabel, caption string) ui.Node {
	if len(series) < 2 {
		return Fragment()
	}
	points := accountflow.Polyline(series, sparklineW, sparklineH, 2)
	if points == "" {
		return Fragment()
	}
	return Div(css.Class("acct-spark-fig"), Attr("data-testid", "acct-spark-fig-"+a.ID),
		Svg(css.Class("acct-spark"), Attr("data-testid", "acct-spark-"+a.ID),
			Attr("viewBox", "0 0 120 24"), Attr("width", "120"), Attr("height", "24"),
			Role("img"), AriaLabel(ariaLabel), Attr("preserveAspectRatio", "none"),
			Polyline(Attr("points", points), Attr("fill", "none"),
				Attr("stroke", "var(--accent)"), Attr("stroke-width", "1.5"),
				Attr("stroke-linejoin", "round"), Attr("stroke-linecap", "round")),
		),
		Span(css.Class("row-meta", tw.TextDim), AriaHidden(true), caption),
	)
}

// accountSparkline renders the account's 90-day balance series as a captioned sparkline.
// It is the plain figure used when the account has no range picker.
func accountSparkline(a domain.Account, series []int64) ui.Node {
	label := uistate.T("accounts.sparklineAria", a.Name)
	if seriesFlat(series) {
		label = uistate.T("accounts.sparklineFlat", a.Name)
	}
	return sparklineFigure(a, series, label, uistate.T("accounts.sparklineCaption"))
}

// accountBalanceChart is the account-detail balance chart with an optional range
// picker: 90 days, 12 months, or all. When hasRange is false (an account with no
// history beyond 90 days) it renders the plain 90-day sparkline. Otherwise a small
// segmented picker sits above the figure and switches the drawn window. sel is the
// selected range key ("90d", "12m" or "all"), and onSelect (a plain func, safe here)
// updates the row's range state.
func accountBalanceChart(a domain.Account, s90, s12m, sall []int64, hasRange bool, sel string, onSelect func(string)) ui.Node {
	if !hasRange {
		return accountSparkline(a, s90)
	}
	series, ariaKey, flatKey, captionKey := s90, "accounts.sparklineAria", "accounts.sparklineFlat", "accounts.sparklineCaption"
	switch sel {
	case "12m":
		series, ariaKey, flatKey, captionKey = s12m, "accountsRange.aria12m", "accountsRange.flat12m", "accountsRange.caption12m"
	case "all":
		series, ariaKey, flatKey, captionKey = sall, "accountsRange.ariaAll", "accountsRange.flatAll", "accountsRange.captionAll"
	}
	aria := uistate.T(ariaKey, a.Name)
	if seriesFlat(series) {
		aria = uistate.T(flatKey, a.Name)
	}
	picker := uiw.RadioGroup(uiw.RadioGroupProps{
		Label:    uistate.T("accountsRange.label"),
		Value:    sel,
		OnChange: onSelect,
		Options: []uiw.RadioOption{
			{Value: "90d", Label: uistate.T("accountsRange.d90"), TestID: "acct-range-90d-" + a.ID},
			{Value: "12m", Label: uistate.T("accountsRange.m12"), TestID: "acct-range-12m-" + a.ID},
			{Value: "all", Label: uistate.T("accountsRange.all"), TestID: "acct-range-all-" + a.ID},
		},
	})
	return Div(css.Class("acct-spark-ranged"), Attr("data-testid", "acct-spark-ranged-"+a.ID),
		Div(css.Class("acct-spark-range-picker"), picker),
		sparklineFigure(a, series, aria, uistate.T(captionKey)),
	)
}

// accountFlowFigures renders the account's money in, money out and net for this period
// as a compact row line. Transfers are left out, so a move between your own accounts
// never looks like income or spending. It renders only when show is set and there was
// activity this period.
func accountFlowFigures(a domain.Account, f accountflow.Flow, show bool) ui.Node {
	if !show || (f.In.Amount == 0 && f.Out.Amount == 0) {
		return Fragment()
	}
	tone := ""
	if f.Net.Amount > 0 {
		tone = tw.ColorClass("pos")
	} else if f.Net.Amount < 0 {
		tone = tw.ColorClass("neg")
	}
	return Div(css.Class("acct-flow"), Attr("data-testid", "acct-flow-"+a.ID),
		AriaLabel(uistate.T("accounts.flowAria", fmtMoney(f.In), fmtMoney(f.Out), fmtMoney(f.Net))),
		flowFigure("acct-flow-in", uistate.T("accounts.flowIn"), fmtMoney(f.In), ""),
		flowFigure("acct-flow-out", uistate.T("accounts.flowOut"), fmtMoney(f.Out), ""),
		flowFigure("acct-flow-net", uistate.T("accounts.flowNet"), fmtMoney(f.Net), tone),
	)
}

// flowFigure is one period figure: a quiet label above its amount, the amount in the body
// size so the three read as the panel's headline numbers. tone colours the amount and may be empty.
func flowFigure(class, label, amount, tone string) ui.Node {
	value := "acct-flow-value"
	if tone != "" {
		value += " " + tone
	}
	return Div(css.Class("acct-fig", class),
		Span(css.Class("acct-fig-label"), label),
		Span(css.Class(value), amount),
	)
}

// --- group editor -----------------------------------------------------------------

type groupAccountToggleProps struct {
	Account  domain.Account
	Checked  bool
	OnToggle func(string)
}

// groupAccountToggle is one checkable account row in the group editor. Its own
// component so the per-row click hook is stable inside the account list (never an
// On* handler in a loop).
func groupAccountToggle(props groupAccountToggleProps) ui.Node {
	a := props.Account
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(a.ID) }))
	cls := ClassNames("pool-acct-toggle", When(props.Checked, "is-checked"))
	checkMark := If(props.Checked, uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)))
	return Button(ClassStr(cls), Type("button"), Role("checkbox"), Attr("aria-checked", ariaBool(props.Checked)),
		Attr("data-testid", "group-acct-"+a.ID), OnClick(toggle),
		Span(css.Class("pool-acct-check"), AriaHidden(true), checkMark),
		Span(css.Class("pool-acct-name"), a.Name),
		Span(css.Class("inv-chip inv-class"), humanizeType(string(a.Type))),
	)
}

// AccountGroupsFormProps configures the create/edit account-group modal form.
type AccountGroupsFormProps struct {
	ID     string // "new" (or "") to create, else the group id to edit
	OnDone func()
}

// AccountGroupsForm is the create/edit account-group modal body: a name field and a
// checkable list of accounts to include, plus a Delete action when editing. Saving
// upserts the group. An account belongs to one group, so selecting it here moves it
// out of any other group. Deleting a group only ungroups its accounts.
func AccountGroupsForm(props AccountGroupsFormProps) ui.Node {
	app := appstate.Default
	var accounts []domain.Account
	if app != nil {
		for _, a := range app.Accounts() {
			if !a.Archived {
				accounts = append(accounts, a)
			}
		}
		sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	}
	isNew := props.ID == "" || props.ID == "new"
	var existing domain.AccountGroup
	if !isNew && app != nil {
		if g, ok := app.GetAccountGroup(props.ID); ok {
			existing = g
		}
	}

	nameS := ui.UseState(existing.Name)
	initSel := map[string]bool{}
	for _, aid := range existing.AccountIDs {
		initSel[aid] = true
	}
	selS := ui.UseState(initSel)
	errS := ui.UseState("")
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })

	toggle := func(aid string) {
		cur := selS.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		if next[aid] {
			delete(next, aid)
		} else {
			next[aid] = true
		}
		selS.Set(next)
	}

	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			errS.Set(uistate.T("accounts.groupNameRequired"))
			return
		}
		// Keep the user's chosen order stable: iterate the sorted account list.
		var ids []string
		for _, a := range accounts {
			if selS.Get()[a.ID] {
				ids = append(ids, a.ID)
			}
		}
		g := existing
		g.Name = name
		g.AccountIDs = ids
		if _, err := app.PutAccountGroup(g); err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("accounts.groupSaved"), false)
		done()
	}))

	del := ui.UseEvent(Prevent(func() {
		if app == nil || isNew {
			return
		}
		if err := app.DeleteAccountGroup(existing.ID); err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("accounts.groupDeleted", existing.Name), false)
		done()
	}))

	toggles := MapKeyed(accounts, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
		return ui.CreateElement(groupAccountToggle, groupAccountToggleProps{Account: a, Checked: selS.Get()[a.ID], OnToggle: toggle})
	})

	deleteRow := If(!isNew, Div(css.Class(tw.Mt3),
		Button(css.Class("btn btn-sm danger"), Type("button"), Attr("data-testid", "group-delete"),
			OnClick(del), uistate.T("accounts.deleteGroup"))))

	return Div(css.Class("inv-pool-modal"),
		Form(css.Class("inv-pool-modal-form"), Attr("id", "account-group-form"), OnSubmit(save),
			labeledField(uistate.T("accounts.groupNameLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "group-name"), Attr("autofocus", "true"),
					Placeholder(uistate.T("accounts.groupNamePh")), OnInput(onName), uiw.FieldValue(nameS.Get()))),
			Div(css.Class("pool-acct-list-label", tw.TextDim), uistate.T("accounts.groupPickAccounts")),
			If(len(accounts) == 0, P(css.Class("empty"), uistate.T("accounts.groupNoAccounts"))),
			Div(css.Class("pool-acct-list"), toggles),
			If(errS.Get() != "", P(css.Class("err"), Role("alert"), errS.Get())),
			deleteRow,
		),
	)
}
