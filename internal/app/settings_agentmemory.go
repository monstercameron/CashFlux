// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// agentMemoryForm is the transparent agent-memory editor (AG19) in Settings → AI:
// it lists the durable facts the assistant remembers and lets the user edit or
// forget any of them, plus add one manually. The store lives in the dataset's
// settings KV (uistate.LoadAgentMemory/PersistAgentMemory) so it travels with the
// dataset and survives a wipe. Capture stays explicit — the assistant only writes
// here via its approved remember_fact tool, and everything it writes is visible and
// editable in this list.
func agentMemoryForm() uic.Node {
	rev := uic.UseState(0)
	addDraft := uic.UseState("")
	refresh := func() { rev.Update(func(n int) int { return n + 1 }) }
	_ = rev.Get() // re-render on mutation

	mem := uistate.LoadAgentMemory()

	save := func(i int, val string) {
		next := uistate.LoadAgentMemory().Edit(i, val)
		uistate.PersistAgentMemory(next)
		uistate.RequestPersist()
		refresh()
	}
	del := func(i int) {
		next := uistate.LoadAgentMemory().Delete(i)
		uistate.PersistAgentMemory(next)
		uistate.RequestPersist()
		refresh()
	}
	onAdd := uic.UseEvent(func(v string) { addDraft.Set(v) })
	commitAdd := uic.UseEvent(func() {
		if uistate.RememberFact(addDraft.Get()) {
			addDraft.Set("")
		}
		refresh()
	})

	var body uic.Node
	if mem.Len() == 0 {
		body = P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.memoryEmpty"))
	} else {
		rows := make([]uic.Node, 0, mem.Len())
		for i, f := range mem.Facts {
			rows = append(rows, uic.CreateElement(agentMemoryRow, agentMemoryRowProps{
				Index: i, Fact: f, OnSave: save, OnDelete: del,
			}))
		}
		body = Div(Attr("data-testid", "settings-agent-memory-list"), rows)
	}

	return Div(Attr("data-testid", "settings-agent-memory"),
		H4(css.Class("set-label"), uistate.T("settings.memoryTitle")),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.memoryHint")),
		body,
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
			Input(css.Class("set-input"), Type("text"), Attr("spellcheck", "false"),
				Attr("aria-label", uistate.T("settings.memoryAddPlaceholder")), Attr("data-testid", "settings-agent-memory-add"),
				Placeholder(uistate.T("settings.memoryAddPlaceholder")), Value(addDraft.Get()), OnInput(onAdd)),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "settings-agent-memory-add-btn"),
				OnClick(commitAdd), uistate.T("settings.memoryAdd")),
		),
	)
}

type agentMemoryRowProps struct {
	Index    int
	Fact     string
	OnSave   func(i int, val string)
	OnDelete func(i int)
}

// agentMemoryRow is one editable remembered-fact row. Its own component so the
// input's change hook and the delete-click hook sit at stable positions across the
// variable-length list (the On*-hooks-in-loops rule).
func agentMemoryRow(props agentMemoryRowProps) uic.Node {
	draft := uic.UseState(props.Fact)
	onEdit := uic.UseEvent(func(v string) {
		draft.Set(v)
		if props.OnSave != nil && strings.TrimSpace(v) != "" {
			props.OnSave(props.Index, v)
		}
	})
	onDelete := uic.UseEvent(func() {
		if props.OnDelete != nil {
			props.OnDelete(props.Index)
		}
	})
	return Div(css.Class("toggle-row", tw.Gap2), Attr("data-testid", "settings-agent-memory-row-"+strconv.Itoa(props.Index)),
		Input(css.Class("set-input", tw.WFull), Type("text"), Attr("spellcheck", "false"),
			Attr("aria-label", uistate.T("settings.memoryEditAria")),
			Value(draft.Get()), OnChange(onEdit)),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("aria-label", uistate.T("settings.memoryDeleteAria")), Title(uistate.T("settings.memoryDeleteAria")),
			OnClick(onDelete), uistate.T("settings.memoryDelete")),
	)
}
