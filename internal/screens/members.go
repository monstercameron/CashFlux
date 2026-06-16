//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
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
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set("Enter a member name.")
			return
		}
		m := domain.Member{ID: id.New(), Name: n, Color: strings.TrimSpace(color.Get())}
		if err := app.PutMember(m); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		errMsg.Set("")
		bump()
	}))

	deleteMember := func(memberID string) {
		if err := app.DeleteMember(memberID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

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

	members := app.Members()
	renderRow := func(m domain.Member) ui.Node {
		return ui.CreateElement(MemberRow, memberRowProps{Member: m, OnDelete: deleteMember, OnSetDefault: setDefault})
	}
	keyOf := func(m domain.Member) any { return m.ID }

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Add member"),
			Form(Class("form-grid"), OnSubmit(add),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
				Input(Class("field"), Type("color"), Value(color.Get()), OnInput(onColor)),
				Button(Class("btn btn-primary"), Type("submit"), "Add member"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Household members"),
			IfElse(len(members) == 0, P(Class("empty"), "No members yet."), Div(Class("rows"), MapKeyed(members, keyOf, renderRow))),
		),
	)
}

type memberRowProps struct {
	Member       domain.Member
	OnDelete     func(string)
	OnSetDefault func(string)
}

// MemberRow is a per-member row with stable action-handler hooks.
func MemberRow(props memberRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Member.ID) }))
	mkDefault := ui.UseEvent(Prevent(func() { props.OnSetDefault(props.Member.ID) }))

	color := props.Member.Color
	if color == "" {
		color = "#7c83ff"
	}
	meta := "Member"
	if props.Member.IsDefault {
		meta = "Default member"
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"),
				Span(Class("swatch"), Style(map[string]string{"background": color, "display": "inline-block", "margin-right": "0.5rem", "vertical-align": "middle"})),
				props.Member.Name,
			),
			Span(Class("row-meta"), meta),
		),
		IfElse(props.Member.IsDefault,
			Span(Class("badge badge-soon"), "Default"),
			Button(Class("btn"), Type("button"), Title("Make default member"), OnClick(mkDefault), "Make default"),
		),
		Button(Class("btn-del"), Type("button"), Title("Delete member"), OnClick(del), "✕"),
	)
}
