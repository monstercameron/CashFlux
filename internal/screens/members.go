//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:members", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	name := ui.UseState("")
	color := ui.UseState("#7c83ff")
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // member awaiting reassignment before delete
	reassignTo := ui.UseState(domain.GroupOwnerID)

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })
	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })

	memberDefs := app.CustomFieldDefsFor("member")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set("Enter a member name.")
			return
		}
		m := domain.Member{
			ID: id.New(), Name: n, Color: strings.TrimSpace(color.Get()),
			Custom: customValuesToMap(memberDefs, customVals.Get()),
		}
		if err := app.PutMember(m); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		bump()
	}))

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
		if to == from {
			errMsg.Set("Pick a different owner to move these to.")
			return
		}
		if _, err := app.ReassignOwner(from, to); err != nil {
			errMsg.Set(err.Error())
			return
		}
		if err := app.DeleteMember(from); err != nil {
			errMsg.Set(err.Error())
			return
		}
		reassignID.Set("")
		errMsg.Set("")
		bump()
	}))

	setDefault := func(memberID string) {
		for _, m := range app.Members() {
			want := m.ID == memberID
			if m.IsDefault == want {
				continue
			}
			m.IsDefault = want
			if err := app.PutMember(m); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		bump()
	}

	saveMember := func(id, newName, newColor string) {
		for _, m := range app.Members() {
			if m.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				m.Name = n
			}
			m.Color = strings.TrimSpace(newColor)
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
		nav.Navigate("/transactions")
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
		ownerRows = append(ownerRows, Div(Class("row"),
			Span(Class("row-desc"), m.Name),
			Span(Class(accentFor(v)), fmtMoney(v)),
		))
	}
	grp := ownerDisp(domain.GroupOwnerID)
	ownerRows = append(ownerRows, Div(Class("row"),
		Span(Class("row-desc"), "Group (shared)"),
		Span(Class(accentFor(grp)), fmtMoney(grp)),
	))

	// Reassign-before-delete panel, shown when a member who owns entities is deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		var targetName string
		opts := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(reassignTo.Get() == domain.GroupOwnerID), "Group (shared)")}
		for _, m := range members {
			if m.ID == rid {
				targetName = m.Name
				continue
			}
			opts = append(opts, Option(Value(m.ID), SelectedIf(reassignTo.Get() == m.ID), m.Name))
		}
		reassignPanel = Section(Class("card"),
			H2(Class("card-title"), "Reassign before deleting"),
			P(Class("muted"), fmt.Sprintf("%q owns %d account(s), budget(s), or goal(s). Move them to another owner, then this member will be deleted.", targetName, ownedCount(rid))),
			Form(Class("form-grid"), OnSubmit(confirmReassign),
				Select(Class("field"), OnChange(onReassignTo), opts),
				Button(Class("btn btn-primary"), Type("submit"), "Move and delete"),
				Button(Class("btn"), Type("button"), OnClick(cancelReassign), "Cancel"),
			),
		)
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Add member"),
			Form(Class("form-grid"), OnSubmit(add),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
				Input(Class("field"), Type("color"), Value(color.Get()), OnInput(onColor)),
				MapKeyed(memberDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(Class("btn btn-primary"), Type("submit"), "Add member"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		),
		reassignPanel,
		Section(Class("card"),
			H2(Class("card-title"), "Household members"),
			IfElse(len(members) == 0, P(Class("empty"), "No members yet."), Div(Class("rows"), MapKeyed(members, keyOf, renderRow))),
		),
		If(len(members) > 0, Section(Class("card"),
			H2(Class("card-title"), "Net worth by member"),
			Div(Class("rows"), ownerRows),
		)),
	)
}

type memberRowProps struct {
	Member       domain.Member
	OnDelete     func(string)
	OnSetDefault func(string)
	OnSave       func(id, name, color string)
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
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(m.Name)
		colorS.Set(color)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(m.ID, nameS.Get(), colorS.Get())
		editing.Set(false)
	}))

	if editing.Get() {
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(nameS.Get()), OnInput(onName)),
				Input(Class("field"), Type("color"), Value(colorS.Get()), OnInput(onColor)),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

	meta := "Member"
	if m.IsDefault {
		meta = "Default member"
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"),
				Span(Class("swatch"), Style(map[string]string{"background": color, "display": "inline-block", "margin-right": "0.5rem", "vertical-align": "middle"})),
				m.Name,
			),
			Span(Class("row-meta"), meta),
		),
		IfElse(m.IsDefault,
			Span(Class("badge badge-soon"), "Default"),
			Button(Class("btn"), Type("button"), Title("Make default member"), OnClick(mkDefault), "Make default"),
		),
		Button(Class("btn"), Type("button"), Title("View this member's transactions"), OnClick(view), "Transactions"),
		Button(Class("btn"), Type("button"), Title("Edit member"), OnClick(startEdit), "Edit"),
		Button(Class("btn-del"), Type("button"), Title("Delete member"), OnClick(del), "✕"),
	)
}
