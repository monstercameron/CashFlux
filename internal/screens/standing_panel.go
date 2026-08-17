// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/standing"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// standingFloorID and standingProtectPrefix are the stable instruction ids. The
// floor has exactly one id because a household has exactly one cash floor;
// protections are one per account.
const (
	standingFloorID       = "keep-liquid"
	standingProtectPrefix = "never-draw-"
)

// standingPanelProps carries the accounts a protection can name and a callback
// to re-render after a change.
type standingPanelProps struct {
	App       *appstate.App
	Base      string
	OnChanged func()
}

// StandingPanel is where a household tells the app what to remember, and reads
// back what it has been told (WF-SM4).
//
// It is one panel rather than a setting buried per-feature because the ticket's
// complaint is repeated corrections: somebody who has to hunt for where they
// said a thing will say it again instead. Everything the app is holding is
// listed here with a way to lift it — a rule that cannot be found or lifted is
// a reason to distrust the whole feature.
func StandingPanel(props standingPanelProps) ui.Node {
	book := uistate.LoadStanding()
	floorMinor, hasFloor := book.KeepLiquidMinor()
	dec := currency.Decimals(props.Base)

	floorS := ui.UseState("")
	seeded := ui.UseState(false)
	if !seeded.Get() {
		seeded.Set(true)
		if hasFloor {
			floorS.Set(money.FormatMinor(floorMinor, dec))
		}
	}
	onFloor := ui.UseEvent(func(v string) { floorS.Set(v) })

	saveFloor := ui.UseEvent(Prevent(func() {
		raw := strings.TrimSpace(floorS.Get())
		if raw == "" {
			// An emptied box lifts the instruction rather than setting a floor of
			// zero: those are different statements, and the one nobody typed is
			// "never mind".
			uistate.ForgetStandingInstruction(standingFloorID)
			props.OnChanged()
			return
		}
		minor, err := money.ParseMinor(raw, dec)
		if err != nil || minor < 0 {
			return
		}
		uistate.SetStandingInstruction(standing.Instruction{
			ID: standingFloorID, Kind: standing.KeepLiquid, AmountMinor: minor,
		})
		props.OnChanged()
	}))

	protectSel := ui.UseState("")
	onProtectSel := ui.UseEvent(func(v string) { protectSel.Set(v) })
	addProtect := ui.UseEvent(Prevent(func() {
		id := strings.TrimSpace(protectSel.Get())
		if id == "" {
			return
		}
		uistate.SetStandingInstruction(standing.Instruction{
			ID: standingProtectPrefix + id, Kind: standing.NeverDrawFrom, Subject: id,
		})
		protectSel.Set("")
		props.OnChanged()
	}))
	// A plain func, not a hook: the ROW owns its click hook, so this is only the
	// callback passed down to it (the On*-hooks-in-loops rule).
	forget := func(id string) {
		uistate.ForgetStandingInstruction(id)
		props.OnChanged()
	}

	accounts := props.App.Accounts()
	opts := []any{
		css.Class("field"), Attr("data-testid", "standing-protect-select"),
		Attr("aria-label", uistate.T("standing.protectAria")),
		OnChange(onProtectSel),
		Option(Value(""), SelectedIf(protectSel.Get() == ""), uistate.T("standing.protectPick")),
	}
	for _, a := range accounts {
		if a.Archived || a.Class == domain.ClassLiability || !book.MayProposeDrawingFrom(a.ID) {
			continue // liabilities are not drawn FROM, and one already protected is not a choice
		}
		opts = append(opts, Option(Value(a.ID), SelectedIf(protectSel.Get() == a.ID), a.Name))
	}

	rows := make([]ui.Node, 0, book.Len())
	for _, i := range book.Instructions {
		rows = append(rows, ui.CreateElement(standingRow, standingRowProps{
			Text:     standingInstructionText(i, accounts, props.Base),
			ID:       i.ID,
			OnForget: forget,
		}))
	}

	return Div(css.Class("card"), Attr("data-testid", "standing-panel"),
		H3(css.Class("card-title"), uistate.T("standing.title")),
		P(css.Class("muted"), css.Class(tw.TextXs), uistate.T("standing.hint")),
		Div(css.Class("form-grid"),
			labeledField(uistate.T("standing.keepLiquid"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "standing-floor"),
					Placeholder(uistate.T("standing.keepLiquidPlaceholder", props.Base)),
					OnInput(onFloor), uiw.FieldValue(floorS.Get()))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "standing-floor-save"),
				OnClick(saveFloor), uistate.T("action.save")),
		),
		Div(css.Class("form-grid"),
			Select(opts...),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "standing-protect-add"),
				OnClick(addProtect), uistate.T("standing.protectAdd")),
		),
		If(len(rows) == 0,
			P(css.Class("empty"), Attr("data-testid", "standing-empty"), uistate.T("standing.empty"))),
		If(len(rows) > 0, Div(css.Class("rows"), Attr("data-testid", "standing-list"), rows)),
	)
}

// standingRowProps carries one instruction's line and its lift callback.
type standingRowProps struct {
	Text     string
	ID       string
	OnForget func(id string)
}

// standingRow is its own component so its click hook sits at a stable render
// position rather than inside the instruction loop.
func standingRow(props standingRowProps) ui.Node {
	forget := ui.UseEvent(Prevent(func() { props.OnForget(props.ID) }))
	return Div(css.Class("row"), Attr("data-testid", "standing-row"),
		Span(css.Class(tw.Flex1), props.Text),
		Button(css.Class("btn-link"), Type("button"),
			Attr("data-testid", "standing-forget-"+props.ID),
			OnClick(forget), uistate.T("standing.lift")),
	)
}

// standingInstructionText says what one instruction means, in the household's
// terms rather than the model's.
func standingInstructionText(i standing.Instruction, accounts []domain.Account, base string) string {
	switch i.Kind {
	case standing.KeepLiquid:
		return uistate.T("standing.lineKeepLiquid",
			fmtMoney(money.New(i.AmountMinor, base)))
	case standing.NeverDrawFrom:
		name := accountName(accounts, i.Subject)
		if name == "" {
			name = i.Subject
		}
		return uistate.T("standing.lineNeverDraw", name)
	}
	return ""
}
