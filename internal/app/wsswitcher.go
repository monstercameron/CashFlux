//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// workspacePalette is the set of accent colors auto-assigned to new workspaces
// (cycled by creation order) and offered in the color picker. Chosen to read on
// the dark shell and stay distinct from one another.
var workspacePalette = []string{
	"#2e8b57", // seagreen
	"#cfa14e", // amber
	"#7c83ff", // indigo
	"#d8716f", // coral
	"#38bdf8", // sky
	"#c084fc", // violet
}

// paletteColor returns a palette color for the given index, cycling.
func paletteColor(i int) string {
	if len(workspacePalette) == 0 {
		return ""
	}
	return workspacePalette[i%len(workspacePalette)]
}

// wsColorDot is a small filled circle in the workspace's color (a faint neutral
// when none is set), so workspaces are distinguishable at a glance.
func wsColorDot(color string) uic.Node {
	c := color
	if c == "" {
		c = "#6c6c72"
	}
	return Span(Class("inline-block w-2.5 h-2.5 rounded-full shrink-0"),
		Style(map[string]string{"background-color": c}))
}

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
		rows = append(rows, uic.CreateElement(wsMenuItem, wsMenuItemProps{ID: w.ID, Name: w.Name, Color: w.Color, Active: w.ID == active.ID}))
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
			Div(Class("border-t border-line my-2 pt-2")),
			Button(Class("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"), OnClick(onNew), uistate.T("ws.new")),
			Button(Class("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"), OnClick(onDup), uistate.T("ws.duplicate")),
		)
	}

	if collapsed {
		// Tint the glyph's border with the workspace color so the active context is
		// recognizable even in the icon-only rail.
		glyph := []any{Class("w-9 h-9 grid place-items-center rounded-[4px] border border-line text-[13px] font-medium hover:bg-hover"),
			Type("button"), Title(active.Name + " · " + uistate.T("ws.switch")),
			OnClick(func() { open.Set(!open.Get()) }),
			workspaceInitial(active.Name)}
		if active.Color != "" {
			glyph = append(glyph, Style(map[string]string{"border-color": active.Color}))
		}
		return Div(Class("ws-switch relative mx-auto mt-3 w-9"),
			Button(glyph...),
			menu,
		)
	}

	return Div(Class("ws-switch relative mx-3 mt-3"),
		Button(Class("w-full flex items-center justify-between gap-2 px-3 py-2 rounded-[4px] border border-line text-[13px] hover:bg-hover"),
			Type("button"), Title(uistate.T("ws.switch")),
			OnClick(func() { open.Set(!open.Get()) }),
			Span(Class("flex items-center gap-2 min-w-0"),
				wsColorDot(active.Color),
				Span(Class("truncate"), active.Name),
			),
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
	Color  string
	Active bool
}

// wsMenuItem is one workspace row in the picker; its own component so the click
// hook stays stable across the list (the On*-hooks-in-loops rule).
func wsMenuItem(props wsMenuItemProps) uic.Node {
	id := props.ID
	cls := "w-full text-left px-2 py-1.5 rounded hover:bg-hover flex items-center justify-between gap-2"
	if props.Active {
		cls += " bg-hover text-fg font-medium"
	}
	return Button(Class(cls), Type("button"),
		OnClick(func() { switchWorkspace(id) }),
		Span(Class("flex items-center gap-2 min-w-0"),
			wsColorDot(props.Color),
			Span(Class("truncate"), props.Name),
		),
		If(props.Active, Span(Class("text-up"), "✓")),
	)
}

type wsManageRowProps struct {
	ID, Name     string
	Color        string
	Index, Total int
	Active       bool
	CanDelete    bool
	OnChange     func() // re-render the settings panel after an in-place change
}

// wsManageRow is one row in the Settings → Workspaces list: a color swatch + the
// name (marked when active) plus rename and delete actions. Its own component for
// stable hooks.
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
	pickColor := func(c string) {
		setWorkspaceColor(id, c)
		if onChange != nil {
			onChange()
		}
	}
	// Reorder buttons. Move() clamps and no-ops out-of-range, so a click at a
	// boundary (the dimmed arrow) is harmless; the dimming is just a hint.
	moveTo := func(to int) func() {
		return func() {
			moveWorkspace(id, to)
			if onChange != nil {
				onChange()
			}
		}
	}
	moveCls := func(enabled bool) string {
		c := "shrink-0 px-1.5 py-1 text-faint hover:text-fg text-[13px] leading-none"
		if !enabled {
			c += " opacity-30 pointer-events-none"
		}
		return c
	}
	actions := []any{Class("flex items-center gap-2"),
		Button(Class(moveCls(props.Index > 0)), Type("button"), Title(uistate.T("ws.moveUp")), OnClick(moveTo(props.Index-1)), "↑"),
		Button(Class(moveCls(props.Index < props.Total-1)), Type("button"), Title(uistate.T("ws.moveDown")), OnClick(moveTo(props.Index+1)), "↓"),
		ui.SwatchPicker(ui.SwatchPickerProps{Colors: workspacePalette, Selected: props.Color, OnSelect: pickColor}),
		dataBtn(uistate.T("ws.rename"), false, rename),
		dataBtn(uistate.T("ws.export"), false, func() { exportWorkspace(id) }),
	}
	if props.CanDelete {
		actions = append(actions, dataBtn(uistate.T("ws.delete"), true, del))
	}
	return Div(Class("flex items-center justify-between gap-2 py-1"),
		Span(Class("flex items-center gap-2 min-w-0"),
			wsColorDot(props.Color),
			Span(Class("truncate"), props.Name),
			If(props.Active, Span(Class("text-xs text-up"), uistate.T("ws.active"))),
		),
		Span(actions...),
	)
}

// workspacesSection renders the Settings → Workspaces management list: a startup
// preference selector followed by one row per workspace. onChange re-renders the
// panel after a rename/delete/startup change that doesn't reload.
func workspacesSection(onChange func()) uic.Node {
	r := loadRegistry()
	active, _ := r.Active()
	canDelete := len(r.Workspaces) > 1
	rows := make([]uic.Node, 0, len(r.Workspaces))
	total := len(r.Workspaces)
	for i, w := range r.Workspaces {
		rows = append(rows, uic.CreateElement(wsManageRow, wsManageRowProps{
			ID: w.ID, Name: w.Name, Color: w.Color, Index: i, Total: total,
			Active: w.ID == active.ID, CanDelete: canDelete, OnChange: onChange,
		}))
	}
	importWS := func() {
		pickFile(".json", func(data []byte) {
			if !importWorkspace(data) {
				js.Global().Call("alert", uistate.T("ws.importErr"))
			}
		})
	}
	return Div(Class("flex flex-col"),
		uic.CreateElement(wsStartupSelect, wsStartupSelectProps{
			Workspaces: r.Workspaces, StartupID: r.StartupID, OnChange: onChange,
		}),
		rows,
		Div(Class("flex flex-wrap gap-2 py-1"),
			dataBtn(uistate.T("ws.import"), false, importWS),
		),
	)
}

type wsStartupSelectProps struct {
	Workspaces []workspace.Workspace
	StartupID  string
	OnChange   func()
}

// wsStartupSelect is the "On launch, open" preference: resume the last-used
// workspace (empty value) or always open a chosen one. Its own component so the
// OnChange hook stays stable. The setting takes effect on the next launch.
func wsStartupSelect(props wsStartupSelectProps) uic.Node {
	onSel := uic.UseEvent(func(e uic.Event) {
		setStartupWorkspace(e.GetValue())
		if props.OnChange != nil {
			props.OnChange()
		}
	})
	opts := make([]uic.Node, 0, len(props.Workspaces)+1)
	opts = append(opts, Option(Value(""), SelectedIf(props.StartupID == ""), uistate.T("ws.startupLast")))
	for _, w := range props.Workspaces {
		opts = append(opts, Option(Value(w.ID), SelectedIf(props.StartupID == w.ID), w.Name))
	}
	return Div(Class("flex flex-col gap-1 py-1"),
		Span(Class("text-xs text-faint"), uistate.T("ws.startupLabel")),
		Select(Class("set-input"), Title(uistate.T("ws.startupLabel")), OnChange(onSel), opts),
	)
}

// promptName shows a browser prompt and returns the trimmed entry ("" on cancel).
func promptName(message, def string) string {
	v := js.Global().Call("prompt", message, def)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return strings.TrimSpace(v.String())
}
