// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PayeeAliasSection is the merchant-name (payee alias) management surface (TX1),
// rendered as a tile on the Rules screen. It lists learned aliases with inline
// edit + delete and an add row, so a user can clean up processor-noise payee
// names in one place. The mapping is view-layer only; the raw payee stays on
// each transaction.
func PayeeAliasSection() ui.Node {
	return ui.CreateElement(payeeAliasSection, struct{}{})
}

func payeeAliasSection(_ struct{}) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	rawS := ui.UseState("")
	dispS := ui.UseState("")
	errS := ui.UseState("")

	onRaw := ui.UseEvent(func(v string) { rawS.Set(v) })
	onDisp := ui.UseEvent(func(v string) { dispS.Set(v) })

	add := ui.UseEvent(Prevent(func() {
		raw := strings.TrimSpace(rawS.Get())
		disp := strings.TrimSpace(dispS.Get())
		if raw == "" || disp == "" {
			errS.Set(uistate.T("payeealias.needBoth"))
			return
		}
		if err := app.PutPayeeAlias(domain.PayeeAlias{RawPayee: raw, Display: disp}); err != nil {
			errS.Set(err.Error())
			return
		}
		rawS.Set("")
		dispS.Set("")
		errS.Set("")
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))

	aliases := app.PayeeAliases()

	// Row callbacks are plain funcs passed to a per-row component (loop-hook-safe).
	saveRow := func(a domain.PayeeAlias, display string) {
		a.Display = strings.TrimSpace(display)
		if a.Display == "" {
			return
		}
		if err := app.PutPayeeAlias(a); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}
	deleteRow := func(a domain.PayeeAlias) {
		uistate.ConfirmModal(uistate.T("payeealias.deleteConfirm", a.Display), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeletePayeeAlias(a.ID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.RequestPersist()
			uistate.BumpDataRevision()
		})
	}

	addRow := Form(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb2), OnSubmit(add),
		Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("payeealias.rawLabel")),
			Placeholder(uistate.T("payeealias.rawPlaceholder")), Value(rawS.Get()), OnInput(onRaw)),
		Span(css.Class("muted"), "→"),
		Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("payeealias.displayLabel")),
			Placeholder(uistate.T("payeealias.displayPlaceholder")), Value(dispS.Get()), OnInput(onDisp)),
		Button(css.Class("btn btn-tool"), Type("submit"), Attr("data-testid", "payeealias-add"), uistate.T("payeealias.addBtn")),
	)

	var body ui.Node
	if len(aliases) == 0 {
		body = P(css.Class("empty"), uistate.T("payeealias.empty"))
	} else {
		body = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			MapKeyed(aliases,
				func(a domain.PayeeAlias) any { return a.ID },
				func(a domain.PayeeAlias) ui.Node {
					return ui.CreateElement(payeeAliasRow, payeeAliasRowProps{
						Alias:    a,
						OnSave:   saveRow,
						OnDelete: deleteRow,
					})
				},
			),
		)
	}

	return rptTile("rules-payeealias", "1 / span 4",
		rptSection("sec-payeealias", uistate.T("payeealias.sectionTitle"), nil, Fragment(
			P(css.Class("muted"), uistate.T("payeealias.sectionHint")),
			addRow,
			errText("payeealias-err", errS.Get()),
			body,
		)),
	)
}

// payeeAliasRowProps configures one editable alias row.
type payeeAliasRowProps struct {
	Alias    domain.PayeeAlias
	OnSave   func(domain.PayeeAlias, string)
	OnDelete func(domain.PayeeAlias)
}

// payeeAliasRow is one alias in the management list: the raw name (read-only,
// it is the transaction's data), an editable display name, and a delete button.
// Its own component so the edit-state hook is a stable render position (never a
// loop-registered On* handler).
func payeeAliasRow(props payeeAliasRowProps) ui.Node {
	dispS := ui.UseState(props.Alias.Display)
	onDisp := ui.UseEvent(func(v string) { dispS.Set(v) })
	save := ui.UseEvent(func() { props.OnSave(props.Alias, dispS.Get()) })
	del := ui.UseEvent(func() { props.OnDelete(props.Alias) })
	dirty := strings.TrimSpace(dispS.Get()) != strings.TrimSpace(props.Alias.Display)

	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("data-testid", "payeealias-row"),
		Span(css.Class(tw.Flex1, "t-caption"), props.Alias.RawPayee),
		Span(css.Class("muted"), "→"),
		Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("payeealias.displayLabel")),
			Value(dispS.Get()), OnInput(onDisp)),
		If(dirty, Button(css.Class("btn btn-tool"), Type("button"), OnClick(save), uistate.T("action.save"))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "payeealias-del"),
			Attr("aria-label", uistate.T("action.delete")), OnClick(del), uistate.T("action.delete")),
	)
}
