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

// TaskEditHost is mounted at the shell root (beside GoalEditHost). It reads the
// task-editor atom and renders the task edit form inside a FlipPanel modal. When the
// atom is empty no overlay is shown. Mounting at the shell root keeps the modal centred
// (a task row lives under transformed bento/tile ancestors, which would push an in-row
// modal off-centre).
func TaskEditHost() uic.Node {
	edit := uistate.UseTaskEdit()
	e := edit.Get()
	if e.ID == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseTaskEdit() }

	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:   uistate.T("todo.editTitle"),
		Width:   uiw.FlipMediumW,
		Height:  uiw.FlipMediumH,
		FormID:  "task-edit-form",
		OnClose: closeModal,
		Back:    uic.CreateElement(screens.TaskEditForm, screens.TaskEditFormProps{TaskID: e.ID, OnDone: closeModal}),
	})
}
