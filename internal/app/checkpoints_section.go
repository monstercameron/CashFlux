// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/checkpoint"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// checkpointsSection renders the Data tab's safety-checkpoint ring (#55): the
// automatic pre-operation snapshots taken before imports and bulk actions,
// each restorable in one click. Mounted as its own component so the list's
// state lives here, not in the pane renderer.
func checkpointsSection() uic.Node {
	return uic.CreateElement(checkpointsCard)
}

// checkpointsCard lists the saved checkpoints newest-first with per-row
// Restore/Delete. Restores confirm first — a restore replaces the LIVE dataset.
func checkpointsCard() uic.Node {
	_ = uistate.UseDataRevision().Get()
	bump := uic.UseState(0)
	rerender := func() { bump.Set(bump.Get() + 1) }

	onRestore := func(cpID, label string) {
		uistate.ConfirmModal(uistate.T("ckpt.restoreConfirm", label), true, func(ok bool) {
			if !ok {
				return
			}
			if uistate.RestoreCheckpoint(cpID) {
				uistate.PostNotice(uistate.T("ckpt.restored", label), false)
			} else {
				uistate.PostNotice(uistate.T("ckpt.restoreErr"), true)
			}
		})
	}
	onDelete := func(cpID string) {
		uistate.DeleteCheckpoint(cpID)
		rerender()
	}

	cps := uistate.Checkpoints()
	// Newest first for display; the store keeps oldest-first.
	rows := make([]checkpoint.Checkpoint, 0, len(cps))
	for i := len(cps) - 1; i >= 0; i-- {
		rows = append(rows, cps[i])
	}
	keyOf := func(c checkpoint.Checkpoint) any { return c.ID }
	render := func(c checkpoint.Checkpoint) uic.Node {
		return uic.CreateElement(checkpointRow, checkpointRowProps{CP: c, OnRestore: onRestore, OnDelete: onDelete})
	}

	return Div(Attr("data-testid", "checkpoints-section"),
		H4(css.Class("set-label"), uistate.T("ckpt.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("ckpt.sectionHint")),
		If(len(rows) == 0, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "checkpoints-empty"),
			uistate.T("ckpt.empty"))),
		If(len(rows) > 0, Div(css.Class("rows", tw.Mt045), MapKeyed(rows, keyOf, render))),
	)
}

// checkpointRowProps feeds one checkpoint row.
type checkpointRowProps struct {
	CP        checkpoint.Checkpoint
	OnRestore func(id, label string)
	OnDelete  func(id string)
}

// checkpointRow is its own component so its click hooks live at stable
// positions per row (framework rule: no On* inside variable-length loops).
func checkpointRow(p checkpointRowProps) uic.Node {
	restore := uic.UseEvent(Prevent(func() { p.OnRestore(p.CP.ID, p.CP.Label) }))
	del := uic.UseEvent(Prevent(func() { p.OnDelete(p.CP.ID) }))
	kb := p.CP.Size / 1024
	if kb < 1 {
		kb = 1
	}
	return Div(css.Class("row"), Attr("data-testid", "checkpoint-row"),
		Style(map[string]string{"display": "flex", "justify-content": "space-between", "align-items": "center", "gap": "1rem"}),
		Div(
			Div(p.CP.Label),
			Div(css.Class(tw.TextFaint, tw.Text12), p.CP.At.Format("Jan 2, 2006 3:04 PM")+" · "+uistate.T("ckpt.sizeKB", kb)),
		),
		Div(css.Class(tw.Flex, tw.Gap2, tw.ShrinkO),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "checkpoint-restore"),
				Title(uistate.T("ckpt.restoreTitle")), OnClick(restore), uistate.T("ckpt.restoreBtn")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "checkpoint-delete"),
				Attr("aria-label", uistate.T("ckpt.deleteAria", p.CP.Label)), OnClick(del), "✕"),
		),
	)
}
