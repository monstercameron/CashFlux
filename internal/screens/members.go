// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Members manages the household: add a member (name + color), list members, set
// the default member, and per-row delete.
func Members() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:members", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // member awaiting reassignment before delete
	reassignTo := ui.UseState(domain.GroupOwnerID)

	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })

	ownedCount := func(memberID string) int {
		owned := 0
		for _, a := range app.Accounts() {
			if a.OwnerID == memberID {
				owned++
			}
		}
		for _, b := range app.Budgets() {
			if b.OwnerID == memberID {
				owned++
			}
		}
		for _, g := range app.Goals() {
			if g.OwnerID == memberID {
				owned++
			}
		}
		// Transactions can be assigned to a member directly (MemberID), independent
		// of account ownership. Count them too, so a member used only as a
		// transaction tag still routes through reassign-before-delete (which clears
		// those MemberIDs) instead of being deleted out from under them.
		for _, t := range app.Transactions() {
			if t.MemberID == memberID {
				owned++
			}
		}
		return owned
	}

	deleteMember := func(memberID string) {
		// If the member owns entities, open the reassign panel; otherwise delete now.
		if ownedCount(memberID) > 0 {
			reassignID.Set(memberID)
			reassignTo.Set(domain.GroupOwnerID)
			errMsg.Set("")
			return
		}
		if err := app.DeleteMember(memberID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	cancelReassign := ui.UseEvent(Prevent(func() { reassignID.Set("") }))
	confirmReassign := ui.UseEvent(Prevent(func() {
		from := reassignID.Get()
		to := reassignTo.Get()
		if _, err := app.DeleteMemberAfterReassign(from, to); err != nil {
			errMsg.Set(err.Error())
			return
		}
		reassignID.Set("")
		errMsg.Set("")
		bump()
	}))

	setDefault := func(memberID string) {
		if err := app.SetDefaultMember(memberID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	saveMember := func(id, newName, newColor, dateStyle, defAccountID string) {
		for _, m := range app.Members() {
			if m.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				m.Name = n
			}
			m.Color = strings.TrimSpace(newColor)
			// Per-member preferences (§1.19): empty = inherit the household default.
			m.Prefs.DateStyle = strings.TrimSpace(dateStyle)
			m.Prefs.DefaultAccountID = strings.TrimSpace(defAccountID)
			if err := app.PutMember(m); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}

	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTransactions := func(memberID string) {
		f := uistate.TxFilter{Member: memberID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	members := app.Members()
	renderRow := func(m domain.Member) ui.Node {
		return ui.CreateElement(MemberRow, memberRowProps{Member: m, OnDelete: deleteMember, OnSetDefault: setDefault, OnSave: saveMember, OnView: viewTransactions})
	}
	keyOf := func(m domain.Member) any { return m.ID }

	// Net worth per owner (member + group-shared), in base currency.
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
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
			// "amount" (not the bare accentFor class) so the figure carries tabular-nums
			// and the light-mode contrast pin — the net-worth amounts were inheriting
			// --text and vanishing on white (G16 CRITICAL).
			Span(ClassStr("amount "+accentFor(v)), fmtMoney(v)),
		))
	}
	grp := ownerDisp(domain.GroupOwnerID)
	ownerRows = append(ownerRows, Div(css.Class("row"),
		Span(css.Class("row-desc"), uistate.T("owner.group")),
		Span(ClassStr("amount "+accentFor(grp)), fmtMoney(grp)),
	))

	// When the reassign panel opens, move focus to its target select so a
	// keyboard user lands on the choice they must make (L-quickhit #47).
	ui.UseEffect(func() func() {
		if reassignID.Get() != "" {
			focusByID("member-reassign")
		}
		return nil
	}, reassignID.Get())

	// Reassign-before-delete panel, shown when a member who owns entities is deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		var targetName string
		opts := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(reassignTo.Get() == domain.GroupOwnerID), uistate.T("owner.group"))}
		for _, m := range members {
			if m.ID == rid {
				targetName = m.Name
				continue
			}
			opts = append(opts, Option(Value(m.ID), SelectedIf(reassignTo.Get() == m.ID), m.Name))
		}
		reassignPanel = uiw.Card(uiw.CardProps{
			Title: uistate.T("members.reassignTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("members.reassignDesc", targetName, ownedCount(rid))),
				Form(css.Class("form-grid"), OnSubmit(confirmReassign),
					Select(css.Class("field"), Attr("id", "member-reassign"), Attr("aria-label", uistate.T("members.reassignTitle")), OnChange(onReassignTo), opts),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("members.moveAndDelete")),
					Button(css.Class("btn"), Type("button"), OnClick(cancelReassign), uistate.T("action.cancel")),
				),
			),
		})
	}

	return Div(
		reassignPanel,
		uiw.Card(uiw.CardProps{
			Title: uistate.T("members.listTitle"),
			// G16: orientation description so the page self-explains on first visit.
			// i18n key to add: "members.desc" → "Manage who's in your household. Each
			// member can own accounts, budgets, and goals."
			Body: Fragment(
				P(css.Class("muted"), uistate.T("members.desc")),
				IfElse(len(members) == 0,
					ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("members.empty"), CTALabel: uistate.T("members.addFirst"), AddTarget: "member"}),
					Div(css.Class("rows"), MapKeyed(members, keyOf, renderRow)),
				),
			),
		}),
		If(len(members) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("members.netWorthTitle"),
			Rows:  ownerRows,
		})),
	)
}

// memberAvatar is a small colored initial avatar (the member's first letter on a
// disc tinted with their color), for scannability and a touch of personality (C62).
// Decorative — the member name follows as text — so it's aria-hidden. Inline-styled
// to avoid a stylesheet dependency; falls back to the border color when no color set.
func memberAvatar(name, color string) ui.Node {
	initial := "?"
	if t := strings.TrimSpace(name); t != "" {
		initial = strings.ToUpper(string([]rune(t)[0]))
	}
	bg := color
	if strings.TrimSpace(bg) == "" {
		bg = "var(--border)"
	}
	return Span(css.Class("member-avatar"), Attr("aria-hidden", "true"),
		Style(map[string]string{
			"display": "inline-flex", "align-items": "center", "justify-content": "center",
			"width": "1.5rem", "height": "1.5rem", "border-radius": "50%",
			"background": bg, "color": "#fff", "font-size": "0.7rem", "font-weight": "700",
			"margin-right": "0.5rem", "vertical-align": "middle", "flex-shrink": "0",
		}),
		initial,
	)
}

// memberDateStyleOptions are the per-member date-style choices: a leading
// "Inherit" (empty value = use the household default) then the concrete styles.
func memberDateStyleOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("members.prefInherit")},
		{Value: string(prefs.DateISO), Label: "2006-01-02"},
		{Value: string(prefs.DateUS), Label: "01/02/2006"},
		{Value: string(prefs.DateEU), Label: "02/01/2006"},
		{Value: string(prefs.DateLong), Label: "Jan 2, 2006"},
	}
}

// memberDefaultAccountOptions lists "Inherit" (no per-member default) then every
// account by name, for the per-member default-account preference.
func memberDefaultAccountOptions() []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("members.prefInherit")}}
	if app := appstate.Default; app != nil {
		for _, a := range app.Accounts() {
			opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
		}
	}
	return opts
}

type memberRowProps struct {
	Member       domain.Member
	OnDelete     func(string)
	OnSetDefault func(string)
	OnSave       func(id, name, color, dateStyle, defAccountID string)
	OnView       func(string)
}

// MemberRow is a per-member row. It can be edited inline (name + color). All
// hooks are declared unconditionally so the edit toggle never reorders them.
func MemberRow(props memberRowProps) ui.Node {
	m := props.Member
	color := m.Color
	if color == "" {
		color = "#7c83ff"
	}

	del := ui.UseEvent(Prevent(func() { props.OnDelete(m.ID) }))
	mkDefault := ui.UseEvent(Prevent(func() { props.OnSetDefault(m.ID) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(m.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(m.Name)
	colorS := ui.UseState(color)
	dateStyleS := ui.UseState(m.Prefs.DateStyle)
	defAcctS := ui.UseState(m.Prefs.DefaultAccountID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(m.Name)
		colorS.Set(color)
		dateStyleS.Set(m.Prefs.DateStyle)
		defAcctS.Set(m.Prefs.DefaultAccountID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(m.ID, nameS.Get(), colorS.Get(), dateStyleS.Get(), defAcctS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("member-edit-" + m.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("members.name"),
					Input(css.Class("field"), Attr("id", "member-edit-"+m.ID), Type("text"), Attr("aria-label", uistate.T("members.name")), Placeholder(uistate.T("members.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("members.color"),
					Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("members.color")), Attr("aria-label", uistate.T("members.color")), Value(colorS.Get()), OnInput(onColor))),
				// Per-member preferences (§1.19): an optional personal date style and a
				// default account that seeds this member's quick-add. "Inherit" = use the
				// household default.
				labeledField(uistate.T("members.prefDateStyle"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   memberDateStyleOptions(),
						Selected:  dateStyleS.Get(),
						OnChange:  func(v string) { dateStyleS.Set(v) },
						AriaLabel: uistate.T("members.prefDateStyle"),
					})),
				labeledField(uistate.T("members.prefDefaultAccount"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   memberDefaultAccountOptions(),
						Selected:  defAcctS.Get(),
						OnChange:  func(v string) { defAcctS.Set(v) },
						AriaLabel: uistate.T("members.prefDefaultAccount"),
					})),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	meta := uistate.T("members.roleMember")
	if m.IsDefault {
		meta = uistate.T("members.roleDefault")
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"),
				memberAvatar(m.Name, color),
				m.Name,
			),
			Span(css.Class("row-meta"), meta),
		),
		IfElse(m.IsDefault,
			Span(css.Class("badge badge-soon"), uistate.T("members.defaultBadge")),
			Button(css.Class("btn"), Type("button"), Title(uistate.T("members.makeDefaultTitle")), OnClick(mkDefault), uistate.T("members.makeDefault")),
		),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("members.viewTitle")), OnClick(view), uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions"))),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("members.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("members.deleteTitle")), Title(uistate.T("members.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}
