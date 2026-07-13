// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
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
	taskParent := uistate.UseTaskAddParent() // hook — must run every render, before the early return

	if target.Get() == "" {
		return Fragment()
	}

	// Closing the add modal also clears any pending sub-task parent so the next plain "Add
	// task" starts at the top level.
	close := func() { uistate.SetAddTarget(""); uistate.SetTaskAddParent("") }

	switch target.Get() {
	case "goal":
		// NoFooter + FlushBody: the form owns a pinned Cancel + Add goal bar.
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("goals.add"),
			Width:     uiw.FlipMediumW,
			Height:    uiw.FlipMediumH,
			NoFooter:  true,
			FlushBody: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.GoalAddForm, screens.GoalAddFormProps{OnDone: close}),
		})
	case "account":
		// Standard pinned Cancel + Add account footer (submits the body form by id).
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("accounts.addTitle"),
			Width:     uiw.FlipMediumW,
			Height:    uiw.FlipMediumH,
			FormID:    "account-add-form",
			SaveLabel: uistate.T("accounts.addTitle"),
			OnClose:   close,
			Back:      uic.CreateElement(screens.AccountAddForm, screens.AccountAddFormProps{OnDone: close}),
		})
	case "budget":
		// NoFooter + FlushBody: the form owns its own Cancel + Add budget bar, pinned to
		// the bottom so it never scrolls off; no separate Close footer, no dead space.
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("budgets.add"),
			Width:     uiw.FlipMediumW,
			Height:    uiw.FlipMediumH,
			NoFooter:  true,
			FlushBody: true,
			OnClose:   close,
			Back:      uic.CreateElement(screens.BudgetAddForm, screens.BudgetAddFormProps{OnDone: close}),
		})
	case "task":
		// NoFooter: the form is a two-zone "compose slip" that bleeds to the panel edges
		// and owns its own footer (summary + Cancel + Add). When a parent is set the same
		// full form composes a sub-task (title reflects that).
		taskTitle := uistate.T("todo.addTitle")
		if taskParent.Get() != "" {
			taskTitle = uistate.T("todo.subtaskTitle")
		}
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:    taskTitle,
			Width:    "720px",
			Height:   "560px",
			NoFooter: true,
			OnClose:  close,
			Back:     uic.CreateElement(screens.TaskAddForm, screens.TaskAddFormProps{OnDone: close, ParentID: taskParent.Get()}),
		})
	case "category":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("categories.add"),
			Width:     uiw.FlipSmallW,
			Height:    uiw.FlipSmallH,
			FormID:    "category-add-form",
			SaveLabel: uistate.T("categories.add"),
			OnClose:   close,
			Back:      uic.CreateElement(screens.CategoryAddForm, screens.CategoryAddFormProps{OnDone: close}),
		})
	case "member":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("members.add"),
			Width:     uiw.FlipSmallW,
			Height:    uiw.FlipSmallH,
			FormID:    "member-add-form",
			SaveLabel: uistate.T("members.add"),
			OnClose:   close,
			Back:      uic.CreateElement(screens.MemberAddForm, screens.MemberAddFormProps{OnDone: close}),
		})
	case "rule":
		return uiw.FlipPanel(uiw.FlipPanelProps{
			Title:     uistate.T("rules.add"),
			Width:     uiw.FlipMediumW,
			Height:    uiw.FlipMediumH,
			FormID:    "rule-add-form",
			SaveLabel: uistate.T("rules.add"),
			OnClose:   close,
			Back:      uic.CreateElement(screens.RuleAddForm, screens.RuleAddFormProps{OnDone: close}),
		})
	default:
		return Fragment()
	}
}
