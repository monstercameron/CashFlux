// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// AddHost is mounted at the shell root (beside QuickAddHost). It reads the
// AddTarget atom and renders the matching entity's add form inside a FlipPanel
// modal. When the atom is empty no overlay is shown.
//
// The FlipPanel is configured with CloseOnly: true so its footer is a single
// "Close" button — the add form owns its own submit button and errText.
// On a successful add the form calls OnDone (which sets AddTarget to ""),
// closing the modal. On a validation error the form stays open.
func AddHost() uic.Node {
	target := uistate.UseAddTarget()

	if target.Get() == "" {
		return Fragment()
	}

	close := func() { uistate.SetAddTarget("") }

	switch target.Get() {
	case "goal":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("goals.add"),
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.GoalAddForm, screens.GoalAddFormProps{OnDone: close}),
		})
	case "account":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("accounts.addTitle"),
			Width:     "560px",
			Height:    "820px",
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.AccountAddForm, screens.AccountAddFormProps{OnDone: close}),
		})
	case "budget":
		// NoFooter: the form owns its own Cancel + Add budget bar (pinned to the bottom),
		// so there's no separate Close footer and no dead space above it.
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:    uistate.T("budgets.add"),
			Width:    "540px",
			Height:   "700px",
			NoFooter: true,
			OnClose:  close,
			Back:     uic.CreateElement(screens.BudgetAddForm, screens.BudgetAddFormProps{OnDone: close}),
		})
	case "task":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("todo.addTitle"),
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.TaskAddForm, screens.TaskAddFormProps{OnDone: close}),
		})
	case "category":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("categories.add"),
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.CategoryAddForm, screens.CategoryAddFormProps{OnDone: close}),
		})
	case "member":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("members.add"),
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.MemberAddForm, screens.MemberAddFormProps{OnDone: close}),
		})
	case "rule":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("rules.add"),
			CloseOnly: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.RuleAddForm, screens.RuleAddFormProps{OnDone: close}),
		})
	default:
		return Fragment()
	}
}
