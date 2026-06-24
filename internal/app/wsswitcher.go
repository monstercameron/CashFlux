// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	"github.com/monstercameron/GoWebComponents/css"
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
	return Span(css.Class(tw.ShrinkO, tw.InlineBlock, tw.W25, tw.H25, tw.RoundedFull),
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
		promptModal(uistate.T("ws.newPrompt"), uistate.T("ws.newDefault"), func(n string) {
			if n != "" {
				createWorkspace(n)
			}
		})
	}
	onDup := func() {
		promptModal(uistate.T("ws.dupPrompt"), active.Name+" copy", func(n string) {
			if n != "" {
				duplicateWorkspace(n)
			}
		})
	}

	// In the collapsed rail (58px) a full-width labelled button doesn't fit, so the
	// trigger becomes an icon-only square and its menu flies out to the right at a
	// readable fixed width instead of stretching edge-to-edge.
	menuCls := tw.Fold(tw.Absolute, tw.Left0, tw.Right0, tw.Mt1, tw.Z30, tw.Rounded4, tw.Border, tw.BorderLine, tw.BgBase, tw.P1, tw.Text13)
	if collapsed {
		menuCls = tw.Fold(tw.Absolute, tw.LeftFull, tw.Top0, tw.Ml1, tw.Z40, tw.W48, tw.Rounded4, tw.Border, tw.BorderLine, tw.BgBase, tw.P1, tw.Text13, tw.ShadowLg)
	}
	menu := Fragment()
	if open.Get() {
		menu = Div(ClassStr(menuCls),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap05), rows),
			Div(css.Class(tw.BorderT, tw.BorderLine, tw.My2, tw.Pt2)),
			Button(css.Class(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover), Type("button"), OnClick(onNew), uistate.T("ws.new")),
			Button(css.Class(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover), Type("button"), OnClick(onDup), uistate.T("ws.duplicate")),
		)
	}

	if collapsed {
		// Tint the glyph's border with the workspace color so the active context is
		// recognizable even in the icon-only rail.
		glyph := []any{css.Class(tw.W9, tw.H9, tw.Grid, tw.PlaceItemsCenter, tw.Rounded4, tw.Border, tw.BorderLine, tw.Text13, tw.FontMedium, tw.HoverBgHover),
			Type("button"), Title(active.Name + " · " + uistate.T("ws.switch")),
			OnClick(func() { open.Set(!open.Get()) }),
			workspaceInitial(active.Name)}
		if active.Color != "" {
			glyph = append(glyph, Style(map[string]string{"border-color": active.Color}))
		}
		return Div(css.Class("ws-switch", tw.Relative, tw.MxAuto, tw.Mt3, tw.W9),
			Button(glyph...),
			menu,
		)
	}

	return Div(css.Class("ws-switch", tw.Relative, tw.Mx3, tw.Mt3),
		Button(css.Class(tw.WFull, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Px3, tw.Py2, tw.Rounded4, tw.Border, tw.BorderLine, tw.Text13, tw.HoverBgHover),
			Type("button"), Title(uistate.T("ws.switch")),
			OnClick(func() { open.Set(!open.Get()) }),
			Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.MinW0),
				wsColorDot(active.Color),
				Span(css.Class(tw.Truncate), active.Name),
			),
			ui.Icon(icon.ChevronDown, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextFaint)),
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
	cls := tw.Fold(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2)
	if props.Active {
		cls += " " + tw.Fold(tw.BgHover, tw.TextFg, tw.FontMedium)
	}
	return Button(ClassStr(cls), Type("button"),
		OnClick(func() { switchWorkspace(id) }),
		Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.MinW0),
			wsColorDot(props.Color),
			Span(css.Class(tw.Truncate), props.Name),
		),
		If(props.Active, Span(css.Class(tw.TextUp), "✓")),
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
		promptModal(uistate.T("ws.renamePrompt"), props.Name, func(n string) {
			if n != "" {
				renameWorkspace(id, n)
				if onChange != nil {
					onChange()
				}
			}
		})
	}
	del := func() {
		confirmModal(uistate.T("ws.deleteConfirm"), true, func(ok bool) {
			if ok {
				deleteWorkspace(id) // reloads when deleting the active one
				if onChange != nil {
					onChange()
				}
			}
		})
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
		c := tw.Fold(tw.ShrinkO, tw.Px15, tw.Py1, tw.TextFaint, tw.HoverTextFg, tw.Text13, tw.LeadingNone)
		if !enabled {
			c += " " + tw.Fold(tw.Opacity30, tw.PointerEventsNone)
		}
		return c
	}
	actions := []any{css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		Button(ClassStr(moveCls(props.Index > 0)), Type("button"), Attr("aria-label", uistate.T("ws.moveUp")), Title(uistate.T("ws.moveUp")), OnClick(moveTo(props.Index-1)), ui.Icon(icon.ArrowUp, css.Class(tw.W4, tw.H4))),
		Button(ClassStr(moveCls(props.Index < props.Total-1)), Type("button"), Attr("aria-label", uistate.T("ws.moveDown")), Title(uistate.T("ws.moveDown")), OnClick(moveTo(props.Index+1)), ui.Icon(icon.ArrowDown, css.Class(tw.W4, tw.H4))),
		ui.SwatchPicker(ui.SwatchPickerProps{Colors: workspacePalette, Selected: props.Color, OnSelect: pickColor}),
		dataBtn(uistate.T("ws.rename"), false, rename),
		dataBtn(uistate.T("ws.export"), false, func() { exportWorkspace(id) }),
	}
	if props.CanDelete {
		actions = append(actions, dataBtn(uistate.T("ws.delete"), true, del))
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Py1),
		Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.MinW0),
			wsColorDot(props.Color),
			Span(css.Class(tw.Truncate), props.Name),
			If(props.Active, Span(css.Class(tw.TextXs, tw.TextUp), uistate.T("ws.active"))),
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
				paletteNotify(uistate.T("ws.importErr"), true)
			}
		})
	}
	return Div(css.Class(tw.Flex, tw.FlexCol),
		uic.CreateElement(wsStartupSelect, wsStartupSelectProps{
			Workspaces: r.Workspaces, StartupID: r.StartupID, OnChange: onChange,
		}),
		rows,
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
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
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Py1),
		Span(css.Class(tw.TextXs, tw.TextFaint), uistate.T("ws.startupLabel")),
		Select(css.Class("set-input"), Title(uistate.T("ws.startupLabel")), OnChange(onSel), opts),
	)
}
