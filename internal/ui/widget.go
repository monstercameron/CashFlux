// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetstyle"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// widgetRoute maps a dashboard tile's stable id to the screen that owns its data,
// so the tile title can drill into that screen on click (C30). An empty result
// means the tile has no natural destination and its title stays plain text.
func widgetRoute(id string) string {
	switch id {
	case "kpi-networth", "kpi-liabilities", "accounts", "trend", "bills", "freshness":
		return "/accounts"
	case "kpi-income", "kpi-spending", "recent", "cashflow", "savings", "breakdown":
		return "/transactions"
	case "budgets":
		return "/budgets"
	case "goals":
		return "/goals"
	case "todo":
		return "/todo"
	case "highlight":
		return "/insights"
	}
	return ""
}

// widgetIcon maps a dashboard tile's stable id to a leading header glyph (C46),
// so KPI tiles are scannable by shape, not just text. A zero (invalid) name means
// the tile gets no icon (e.g. user custom-page widgets).
func widgetIcon(id string) icon.Name {
	switch id {
	case "kpi-networth":
		return icon.Accounts
	case "kpi-liabilities":
		return icon.CreditCard
	case "accounts":
		return icon.Landmark
	case "trend", "cashflow":
		return icon.TrendingUp
	case "bills":
		return icon.Bills
	case "freshness":
		return icon.Clock
	case "kpi-income":
		return icon.ArrowDownCircle
	case "kpi-spending":
		return icon.ArrowUpCircle
	case "recent":
		return icon.Receipt
	case "savings":
		return icon.Reports
	case "breakdown", "budgets":
		return icon.Budgets
	case "goals":
		return icon.Goals
	case "todo":
		return icon.Todo
	case "highlight":
		return icon.Insights
	}
	// User-built Widget Builder cards (namespaced "wb:") get a generic glyph so they're
	// visually consistent with the built-in tiles instead of icon-less.
	if strings.HasPrefix(id, "wb:") {
		return icon.Sparkles
	}
	return ""
}

// gridCols is the bento width Pack flows tiles into.
const gridCols = 4

// Span bounds for the resize handles (the bento grid is 4 columns × 7 rows).
const (
	maxColSpan = 4
	maxRowSpan = 3
)

// WidgetProps configures a bento Widget shell.
type WidgetProps struct {
	ID         string   // stable id (drag/reorder/layout key)
	Title      string   // centered header title
	Body       uic.Node // widget content (rendered inside the padded body)
	BodyClass  string   // extra classes for the body, e.g. "flex flex-col justify-center" or "kpi"
	GridColumn string   // CSS grid-column span, e.g. "1" or "1 / span 2"
	GridRow    string   // CSS grid-row span, e.g. "2" or "3 / span 2"
	Draggable  bool     // mark the cell draggable (drag-reorder behavior wired separately)
	Resizable  bool     // show the directional resize handles
	OnGear     func()   // open this widget's settings (gear click)
}

// Widget is the candidate-C bento cell shell shared by every dashboard widget: a
// square outlined cell with the unified header (grip · centered title · gear) and
// a padded body, plus optional edge resize handles. Generic and props-driven —
// callers supply only the title and body; the chrome is identical everywhere.
func Widget(props WidgetProps) uic.Node {
	return uic.CreateElement(widget, props)
}

func widget(props WidgetProps) uic.Node {
	bodyClass := "wbody"
	if props.BodyClass != "" {
		bodyClass += " " + props.BodyClass
	}

	// By default the gear opens this widget's settings panel; callers may
	// override with an explicit OnGear.
	settings := uistate.UseSettings()
	onGear := props.OnGear
	if onGear == nil {
		id, title := props.ID, props.Title
		onGear = func() { settings.Set(uistate.Widget(id, title)) }
	}

	// Grid placement comes from packing the shared item sequence (so drag-reorder
	// and resize reflow without overlap), falling back to the caller-provided
	// defaults. The layout mode decides the order before packing: Custom uses the
	// stored sequence as-is; the auto modes reorder it (C24). Sizes are untouched.
	// Packed rows are offset by 1 because the fixed header owns grid row 1.
	itemsAtom := uistate.UseLayoutItems()
	items := itemsAtom.Get()
	modeAtom := uistate.UseLayoutMode()
	mode := modeAtom.Get()
	// Roving tabindex + grab-mode state (unconditional hooks, used below only for
	// draggable tiles): the grid is a single Tab stop and arrows move focus between
	// tiles, with Space/Enter to grab a tile for keyboard move/resize (widget a11y).
	rovingAtom := uistate.UseCurrentTile()
	grabbedAtom := uistate.UseGrabbedTile()

	// Drop hidden widgets before packing so the visible tiles reflow into the gaps
	// (the dashboard skips rendering hidden tiles; this keeps everyone else's
	// placement correct). Visibility is owned by the Widget Manager.
	if hidden := uistate.UseHiddenWidgets().Get(); len(hidden) > 0 {
		kept := items[:0:0]
		for _, it := range items {
			if !hidden.IsHidden(it.ID) {
				kept = append(kept, it)
			}
		}
		items = kept
	}

	// Every tile is configurable now — the settings panel always offers a
	// per-tile color and an importance rank (plus any schema fields) — so the
	// gear always shows. (It used to be hidden on no-schema tiles outside the
	// auto-importance mode, when the panel could read as empty (C21); the
	// per-widget color (B20) gives every tile a meaningful setting.)
	gear := uic.CreateElement(gearButton, gearButtonProps{OnClick: onGear})

	dragSrc := uistate.UseDragSource()
	dragPreview := uistate.UseDragPreview()
	// Live drag-over preview (B2): while dragging, show the dragged tile moved in
	// front of the tile under the cursor — a render-time reorder only, so the
	// persisted layout is untouched and the preview reverts cleanly on cancel.
	arranged := dashlayout.Arrange(items, mode)
	if src, tgt := dragSrc.Get(), dragPreview.Get(); src != "" && tgt != "" && src != tgt {
		ti := -1
		for i, it := range arranged {
			if it.ID == tgt {
				ti = i
				break
			}
		}
		if ti >= 0 {
			arranged = dashlayout.Move(arranged, src, ti)
		}
	}
	packed := dashlayout.Pack(arranged, gridCols)
	gridCol, gridRow := props.GridColumn, props.GridRow
	colSpan, rowSpan := 1, 1
	if p, ok := packed.Get(props.ID); ok {
		gridCol = p.GridColumn()
		// Pack already returns 1-indexed rows; the dashboard no longer has a fixed
		// header cell occupying row 1, so widgets fill from row 1 directly.
		gridRow = p.GridRow()
		colSpan = p.ColSpan
		rowSpan = p.RowSpan
	}

	cellClass := "w"
	if dragSrc.Get() == props.ID && props.ID != "" {
		cellClass += " drag" // dims the widget while it is being dragged
	}
	args := []any{
		ClassStr(cellClass),
		Attr("data-widget", props.ID),
		Attr("data-col-span", fmt.Sprintf("%d", colSpan)),
		Attr("data-row-span", fmt.Sprintf("%d", rowSpan)),
	}
	// Per-widget styling: overlay this tile's effective style (the global "_all"
	// default merged with any per-widget overrides — colors, font, weight, shape,
	// border, shadow, accent) onto the grid placement. Only set fields are emitted,
	// so anything left blank keeps the global theme value. Edited in the Widget
	// Manager's tile-style editor.
	style := gridStyle(gridCol, gridRow)
	if style == nil {
		style = map[string]string{}
	}
	cfgs := uistate.UseWidgetConfigs().Get()
	for k, v := range widgetstyle.InlineStyle(widgetstyle.Effective(cfgs.For(widgetstyle.GlobalID), cfgs.For(props.ID))) {
		style[k] = v
	}
	// The renderer doesn't reset an omitted style key in place, so a cleared accent
	// strip / shadow would linger; always write box-shadow (to "none" when unset) so
	// removing the style reverts the tile.
	if _, ok := style["box-shadow"]; !ok {
		style["box-shadow"] = "none"
	}
	args = append(args, Style(style))
	if props.Draggable {
		id := props.ID
		// Roving tabindex: the grid is a SINGLE Tab stop — only the "current" tile is
		// tabbable (tabindex 0), the rest are -1 — so Tab no longer steps through all
		// 12+ tiles. The current tile defaults to the first in document order.
		baked := dashlayout.Arrange(items, mode)
		firstID := ""
		if len(baked) > 0 {
			firstID = baked[0].ID
		}
		cur := rovingAtom.Get()
		isTabStop := id != "" && (id == cur || (cur == "" && id == firstID))
		grabbed := id != "" && grabbedAtom.Get() == id
		tabidx := "-1"
		if isTabStop {
			tabidx = "0"
		}
		grabbedAttr := "false"
		if grabbed {
			grabbedAttr = "true"
		}
		args = append(args,
			Attr("draggable", "true"),
			Attr("tabindex", tabidx),
			Attr("aria-grabbed", grabbedAttr),
			// APG grid pattern (WCAG 2.1.1 keyboard, without 12+ tab stops): arrows move
			// FOCUS between tiles; Space/Enter GRABS a tile, then arrows move it (Shift
			// to resize) until Space/Enter/Escape drops it.
			Attr("aria-keyshortcuts", "Space Enter ArrowUp ArrowDown ArrowLeft ArrowRight"),
			OnKeyDown(func(e uic.KeyboardEvent) {
				key := e.GetKey()
				// Grab / release toggle.
				if key == " " || key == "Spacebar" || key == "Enter" {
					e.PreventDefault()
					if grabbed {
						grabbedAtom.Set("")
					} else {
						grabbedAtom.Set(id)
						rovingAtom.Set(id)
					}
					return
				}
				if key == "Escape" {
					if grabbed {
						e.PreventDefault()
						grabbedAtom.Set("")
					}
					return
				}
				var delta int
				switch key {
				case "ArrowLeft", "ArrowUp":
					delta = -1
				case "ArrowRight", "ArrowDown":
					delta = 1
				default:
					return
				}
				e.PreventDefault()
				if grabbed {
					// MOVE / RESIZE the grabbed tile.
					if e.JSValue().Get("shiftKey").Bool() {
						dc, dr := 0, 0
						if key == "ArrowLeft" {
							dc = -1
						} else if key == "ArrowRight" {
							dc = 1
						} else if key == "ArrowUp" {
							dr = -1
						} else {
							dr = 1
						}
						curCol, curRow := 1, 1
						for _, it := range items {
							if it.ID == id {
								curCol, curRow = it.ColSpan, it.RowSpan
								break
							}
						}
						next := dashlayout.ResizeItem(items, id, dashlayout.ClampSpan(curCol+dc, maxColSpan), dashlayout.ClampSpan(curRow+dr, maxRowSpan))
						itemsAtom.Set(next)
						uistate.PersistItems(next)
						return
					}
					bk := dashlayout.Arrange(items, mode)
					ci := -1
					for i, it := range bk {
						if it.ID == id {
							ci = i
							break
						}
					}
					if ci < 0 {
						return
					}
					next := dashlayout.Move(bk, id, ci+delta)
					itemsAtom.Set(next)
					uistate.PersistItems(next)
					if mode != dashlayout.ModeCustom {
						modeAtom.Set(dashlayout.ModeCustom)
						uistate.PersistLayoutMode(dashlayout.ModeCustom)
					}
					return
				}
				// NOT grabbed: arrows move FOCUS between tiles (roving tabindex).
				bk := dashlayout.Arrange(items, mode)
				ci := -1
				for i, it := range bk {
					if it.ID == id {
						ci = i
						break
					}
				}
				if ci < 0 {
					return
				}
				ni := ci + delta
				if ni < 0 {
					ni = 0
				}
				if ni >= len(bk) {
					ni = len(bk) - 1
				}
				nextID := bk[ni].ID
				rovingAtom.Set(nextID)
				if doc := js.Global().Get("document"); doc.Truthy() {
					if el := doc.Call("querySelector", "[data-widget=\""+nextID+"\"]"); el.Truthy() && el.Get("focus").Type() == js.TypeFunction {
						el.Call("focus")
					}
				}
			}),
			OnDragStart(func() {
				dragSrc.Set(id)
				if fn := js.Global().Get("cashfluxBentoDragStart"); fn.Type() == js.TypeFunction {
					fn.Invoke(id)
				}
			}),
			OnDragOver(Prevent(func(e uic.Event) {
				// Native dragover fires constantly, and the animated grid moves
				// underneath the pointer. Use the JS drag snapshot to choose a
				// stable target from the pre-drag geometry instead of trusting the
				// current DOM hit target, which can oscillate during FLIP.
				target := id
				if fn := js.Global().Get("cashfluxBentoDragTarget"); fn.Type() == js.TypeFunction {
					v := fn.Invoke(e.JSValue().Get("clientX"), e.JSValue().Get("clientY"))
					if v.Type() == js.TypeString && v.String() != "" {
						target = v.String()
					}
				}
				if target != "" && target != dragPreview.Get() {
					dragPreview.Set(target)
				}
			})), // allow drop + live preview
			OnDrop(Prevent(func(e uic.Event) {
				// Reorder the dragged tile to the drop target's position, then the
				// grid re-Packs around it (iOS-home-screen reflow) instead of a
				// pairwise swap. A manual drag is an explicit hand-arrangement, so it
				// bakes the current (possibly auto-arranged) order into the sequence
				// and switches to Custom mode (C24).
				target := dragPreview.Get()
				if target == "" {
					target = id
				}
				if fn := js.Global().Get("cashfluxBentoDragTarget"); fn.Type() == js.TypeFunction {
					v := fn.Invoke(e.JSValue().Get("clientX"), e.JSValue().Get("clientY"))
					if v.Type() == js.TypeString && v.String() != "" {
						target = v.String()
					}
				}
				if src := dragSrc.Get(); src != "" && target != "" && src != target {
					baked := dashlayout.Arrange(items, mode)
					ti := -1
					for i, it := range baked {
						if it.ID == target {
							ti = i
							break
						}
					}
					if ti >= 0 {
						next := dashlayout.Move(baked, src, ti)
						itemsAtom.Set(next)
						uistate.PersistItems(next)
						if mode != dashlayout.ModeCustom {
							modeAtom.Set(dashlayout.ModeCustom)
							uistate.PersistLayoutMode(dashlayout.ModeCustom)
						}
					}
				}
				dragSrc.Set("")
				dragPreview.Set("")
				if fn := js.Global().Get("cashfluxBentoDragEnd"); fn.Type() == js.TypeFunction {
					fn.Invoke()
				}
			})),
			OnDragEnd(func() {
				if src, target := dragSrc.Get(), dragPreview.Get(); src != "" && target != "" && src != target {
					baked := dashlayout.Arrange(items, mode)
					ti := -1
					for i, it := range baked {
						if it.ID == target {
							ti = i
							break
						}
					}
					if ti >= 0 {
						next := dashlayout.Move(baked, src, ti)
						itemsAtom.Set(next)
						uistate.PersistItems(next)
						if mode != dashlayout.ModeCustom {
							modeAtom.Set(dashlayout.ModeCustom)
							uistate.PersistLayoutMode(dashlayout.ModeCustom)
						}
					}
				}
				dragSrc.Set("")
				dragPreview.Set("")
				if fn := js.Global().Get("cashfluxBentoDragEnd"); fn.Type() == js.TypeFunction {
					fn.Invoke()
				}
			}), // clear (reverts preview if dropped outside)
		)
	}
	// The title drills into the tile's data screen when one exists (C30); it stays
	// a plain heading otherwise. Distinct from the grip (drag) and gear (settings).
	// GX4-F4: H2 (not H3) — page H1 is in the topbar shell; widget titles are the
	// next heading level, so H1→H2 is the correct hierarchy (no level skip).
	var titleNode uic.Node = H2(props.Title)
	if route := widgetRoute(props.ID); route != "" {
		titleNode = uic.CreateElement(viewTitle, viewTitleProps{Title: props.Title, Route: route})
	}
	// A leading glyph makes KPI tiles scannable by shape (C46); decorative, so it
	// sits beside the (still-clickable) title rather than inside the link.
	if ic := widgetIcon(props.ID); ic.Valid() {
		titleNode = Span(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tw.MinW0), Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)), titleNode)
	}
	args = append(args,
		Div(css.Class("wh"),
			Span(css.Class("grip"), Attr("aria-hidden", "true"), Icon(icon.MoreH, css.Class(tw.W4, tw.H4))), // decorative drag grip
			titleNode,
			gear,
		),
		Div(ClassStr(bodyClass), props.Body),
	)
	if props.Resizable {
		id := props.ID
		// Current intrinsic spans from the item sequence.
		curCol, curRow := 1, 1
		for _, it := range items {
			if it.ID == id {
				curCol, curRow = it.ColSpan, it.RowSpan
				break
			}
		}
		resize := func(cs, rs int) {
			next := dashlayout.ResizeItem(items, id, cs, rs)
			itemsAtom.Set(next)
			uistate.PersistItems(next)
		}
		resizeHandle := func(dir, label string, disabled bool, onClick func()) uic.Node {
			cls := "rz"
			if disabled {
				cls += " off"
			}
			return Button(
				ClassStr(cls),
				Type("button"),
				Attr("data-dir", dir),
				Attr("title", label),
				Attr("aria-label", label),
				DisabledIf(disabled),
				OnClick(func(e uic.MouseEvent) {
					e.PreventDefault()
					e.StopPropagation()
					if !disabled {
						onClick()
					}
				}),
			)
		}
		args = append(args,
			// Hover/focus reveals four direct resize actions. Disabled handles are
			// hidden at the span bounds so the tile keeps a calm default surface.
			resizeHandle("l", uistate.T("widget.narrower"), curCol <= 1, func() {
				resize(curCol-1, curRow)
			}),
			resizeHandle("r", uistate.T("widget.wider"), curCol >= maxColSpan, func() {
				resize(curCol+1, curRow)
			}),
			resizeHandle("t", uistate.T("widget.shorter"), curRow <= 1, func() {
				resize(curCol, curRow-1)
			}),
			resizeHandle("b", uistate.T("widget.taller"), curRow >= maxRowSpan, func() {
				resize(curCol, curRow+1)
			}),
		)
	}
	return Div(args...)
}

// gridStyle builds the inline grid-placement style, omitting empty axes.
func gridStyle(col, row string) map[string]string {
	if col == "" && row == "" {
		return nil
	}
	style := map[string]string{}
	if col != "" {
		style["grid-column"] = col
	}
	if row != "" {
		style["grid-row"] = row
	}
	return style
}

type viewTitleProps struct {
	Title string
	Route string
}

// viewTitle renders a dashboard tile's title as a button that navigates to the
// tile's data screen (C30). It is its own component so its click hook stays
// stable across the many widgets rendered in a list (the On*-hooks-in-loops
// rule), mirroring gearButton.
func viewTitle(props viewTitleProps) uic.Node {
	route := props.Route
	return Button(
		css.Class("wh-title"),
		Type("button"),
		Attr("title", uistate.T("widget.open")),
		Attr("aria-label", uistate.T("widget.openNamed", props.Title)),
		OnClick(func() { router.Navigate(uistate.RoutePath(route)) }),
		props.Title,
	)
}

type gearButtonProps struct {
	OnClick func()
}

// gearButton is its own component so its click hook stays stable across the many
// widgets rendered in a list (the On*-hooks-in-loops rule).
func gearButton(props gearButtonProps) uic.Node {
	onClick := props.OnClick
	return Button(
		css.Class("gear-inline"),
		Type("button"),
		Attr("title", uistate.T("widget.settings")),
		Attr("aria-label", uistate.T("widget.settings")), // icon-only button → explicit name (B15)
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		Icon(icon.Settings, css.Class(tw.W4, tw.H4)),
	)
}
