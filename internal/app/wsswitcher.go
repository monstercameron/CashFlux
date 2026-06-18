//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// WorkspaceSwitcher is the rail's quick workspace picker: a button showing the
// active workspace that opens a menu to switch to another, create a fresh one, or
// duplicate the current one. Switching reloads the page (boot rehydrates the
// swapped-in context), which also dismisses the menu.
func WorkspaceSwitcher() uic.Node {
	open := uic.UseState(false)
	collapsed := uistate.UseRailCollapsed().Get()
	r := loadRegistry()
	active, ok := r.Active()
	if !ok {
		return Fragment()
	}

	rows := make([]uic.Node, 0, len(r.Workspaces))
	for _, w := range r.Workspaces {
		rows = append(rows, uic.CreateElement(wsMenuItem, wsMenuItemProps{ID: w.ID, Name: w.Name, Active: w.ID == active.ID}))
	}

	onNew := func() {
		if n := promptName(uistate.T("ws.newPrompt"), uistate.T("ws.newDefault")); n != "" {
			createWorkspace(n)
		}
	}
	onDup := func() {
		if n := promptName(uistate.T("ws.dupPrompt"), active.Name+" copy"); n != "" {
			duplicateWorkspace(n)
		}
	}

	// In the collapsed rail (58px) a full-width labelled button doesn't fit, so the
	// trigger becomes an icon-only square and its menu flies out to the right at a
	// readable fixed width instead of stretching edge-to-edge.
	menuCls := "absolute left-0 right-0 mt-1 z-30 rounded-[4px] border border-line bg-base p-1 text-[13px]"
	if collapsed {
		menuCls = "absolute left-full top-0 ml-1 z-40 w-48 rounded-[4px] border border-line bg-base p-1 text-[13px] shadow-lg"
	}
	menu := Fragment()
	if open.Get() {
		menu = Div(Class(menuCls),
			Div(Class("flex flex-col gap-0.5"), rows),
			Div(Class("border-t border-line my-1")),
			Button(Class("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"), OnClick(onNew), uistate.T("ws.new")),
			Button(Class("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"), OnClick(onDup), uistate.T("ws.duplicate")),
		)
	}

	if collapsed {
		return Div(Class("ws-switch relative mx-auto mt-3 w-9"),
			Button(Class("w-9 h-9 grid place-items-center rounded-[4px] border border-line text-[13px] font-medium hover:bg-hover"),
				Type("button"), Title(active.Name+" · "+uistate.T("ws.switch")),
				OnClick(func() { open.Set(!open.Get()) }),
				workspaceInitial(active.Name),
			),
			menu,
		)
	}

	return Div(Class("ws-switch relative mx-3 mt-3"),
		Button(Class("w-full flex items-center justify-between gap-2 px-3 py-2 rounded-[4px] border border-line text-[13px] hover:bg-hover"),
			Type("button"), Title(uistate.T("ws.switch")),
			OnClick(func() { open.Set(!open.Get()) }),
			Span(Class("truncate"), active.Name),
			Span(Class("text-faint"), "▾"),
		),
		menu,
	)
}

// workspaceInitial is the uppercased first letter of a workspace name, used as the
// compact glyph in the collapsed rail (falls back to a dot for an empty name).
func workspaceInitial(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return strings.ToUpper(string(r))
	}
	return "•"
}

type wsMenuItemProps struct {
	ID     string
	Name   string
	Active bool
}

// wsMenuItem is one workspace row in the picker; its own component so the click
// hook stays stable across the list (the On*-hooks-in-loops rule).
func wsMenuItem(props wsMenuItemProps) uic.Node {
	id := props.ID
	cls := "w-full text-left px-2 py-1.5 rounded hover:bg-hover flex items-center justify-between"
	if props.Active {
		cls += " bg-hover text-fg font-medium"
	}
	return Button(Class(cls), Type("button"),
		OnClick(func() { switchWorkspace(id) }),
		Span(Class("truncate"), props.Name),
		If(props.Active, Span(Class("text-up"), "✓")),
	)
}

type wsManageRowProps struct {
	ID, Name  string
	Active    bool
	CanDelete bool
	OnChange  func() // re-render the settings panel after an in-place change
}

// wsManageRow is one row in the Settings → Workspaces list: the name (marked when
// active) plus rename and delete actions. Its own component for stable hooks.
func wsManageRow(props wsManageRowProps) uic.Node {
	id, onChange := props.ID, props.OnChange
	rename := func() {
		if n := promptName(uistate.T("ws.renamePrompt"), props.Name); n != "" {
			renameWorkspace(id, n)
			if onChange != nil {
				onChange()
			}
		}
	}
	del := func() {
		if confirmAction(uistate.T("ws.deleteConfirm")) {
			deleteWorkspace(id) // reloads when deleting the active one
			if onChange != nil {
				onChange()
			}
		}
	}
	actions := []any{Class("flex gap-2"), dataBtn(uistate.T("ws.rename"), false, rename)}
	if props.CanDelete {
		actions = append(actions, dataBtn(uistate.T("ws.delete"), true, del))
	}
	return Div(Class("flex items-center justify-between gap-2 py-1"),
		Span(Class("flex items-center gap-2"),
			Span(props.Name),
			If(props.Active, Span(Class("text-xs text-up"), uistate.T("ws.active"))),
		),
		Span(actions...),
	)
}

// workspacesSection renders the Settings → Workspaces management list. onChange
// re-renders the panel after a rename/delete that doesn't reload.
func workspacesSection(onChange func()) uic.Node {
	r := loadRegistry()
	active, _ := r.Active()
	canDelete := len(r.Workspaces) > 1
	rows := make([]uic.Node, 0, len(r.Workspaces))
	for _, w := range r.Workspaces {
		rows = append(rows, uic.CreateElement(wsManageRow, wsManageRowProps{
			ID: w.ID, Name: w.Name, Active: w.ID == active.ID, CanDelete: canDelete, OnChange: onChange,
		}))
	}
	return Div(Class("flex flex-col"), rows)
}

// promptName shows a browser prompt and returns the trimmed entry ("" on cancel).
func promptName(message, def string) string {
	v := js.Global().Call("prompt", message, def)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return strings.TrimSpace(v.String())
}
