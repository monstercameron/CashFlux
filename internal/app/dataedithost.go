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

// The Data & People edit hosts, mounted at the shell root (beside
// BudgetEditHost). Each reads its uistate edit atom and renders the matching
// editor inside a FlipPanel modal; an empty atom renders nothing. Mounting at
// the shell root — rather than inside the row — keeps the modal's
// position:fixed backdrop resolving against the viewport instead of a
// transformed bento/tile ancestor. The forms own their Save/Cancel (NoFooter)
// and call OnDone, which clears the atom.

// MemberEditHost renders the member editor / PIN flip modal.
func MemberEditHost() uic.Node {
	e := uistate.UseMemberEdit().Get()
	if e.ID == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseMemberEdit() }
	title, width, height := uistate.T("members.editTitle"), "460px", "560px"
	if e.Mode == uistate.MemberEditModePIN {
		title, width, height = uistate.T("profileSwitch.setPIN"), "420px", "300px"
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    title,
		Width:    width,
		Height:   height,
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.MemberEditForm, screens.MemberEditFormProps{MemberID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}

// CategoryEditHost renders the category editor flip modal.
func CategoryEditHost() uic.Node {
	id := uistate.UseCategoryEdit().Get()
	if id == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseCategoryEdit() }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:   uistate.T("categories.editTitle"),
		Width:   uiw.FlipMediumW,
		Height:  uiw.FlipMediumH,
		FormID:  "category-edit-form",
		OnClose: closeModal,
		Back:    uic.CreateElement(screens.CategoryEditForm, screens.CategoryEditFormProps{CategoryID: id, OnDone: closeModal}),
	})
}

// RuleEditHost renders the rule editor flip modal at a standard size; the body
// scrolls under the pinned Save/Cancel footer when a condition-bearing rule opens
// taller than the panel.
func RuleEditHost() uic.Node {
	id := uistate.UseRuleEdit().Get()
	if id == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseRuleEdit() }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:   uistate.T("rules.editTitle"),
		Width:   uiw.FlipMediumW,
		Height:  uiw.FlipMediumH,
		FormID:  "rule-edit-form",
		OnClose: closeModal,
		Back:    uic.CreateElement(screens.RuleEditForm, screens.RuleEditFormProps{RuleID: id, OnDone: closeModal}),
	})
}

// ArtifactEditHost renders the artifact rename flip modal.
func ArtifactEditHost() uic.Node {
	id := uistate.UseArtifactEdit().Get()
	if id == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseArtifactEdit() }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:   uistate.T("artifacts.renameModalTitle"),
		Width:   uiw.FlipSmallW,
		Height:  uiw.FlipSmallH,
		FormID:  "artifact-rename-form",
		OnClose: closeModal,
		Back:    uic.CreateElement(screens.ArtifactRenameForm, screens.ArtifactRenameFormProps{ArtifactID: id, OnDone: closeModal}),
	})
}
