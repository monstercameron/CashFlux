//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

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
	Resizable  bool     // show the right/bottom resize handles
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

	// Only show the gear on tiles that actually have something to configure — a
	// registered settings schema, an explicit custom OnGear, or (in the
	// auto-importance layout mode) every tile, since importance is then settable
	// per tile (C24). On the no-schema tiles in other modes the gear used to open
	// an empty "no settings yet" panel, reading as broken (C21); render an inert,
	// equal-width slot there instead so the header stays balanced.
	var gear uic.Node
	if props.OnGear != nil || widgetcfg.Has(props.ID) || mode == dashlayout.ModeAutoImportance {
		gear = uic.CreateElement(gearButton, gearButtonProps{OnClick: onGear})
	} else {
		gear = Span(Class("gear-inline"), Attr("aria-hidden", "true"), Style(map[string]string{"visibility": "hidden"}), "⚙")
	}

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
	if p, ok := packed.Get(props.ID); ok {
		gridCol = p.GridColumn()
		gridRow = dashlayout.Placement{Row: p.Row + 1, RowSpan: p.RowSpan}.GridRow()
	}

	cellClass := "w"
	if dragSrc.Get() == props.ID && props.ID != "" {
		cellClass += " drag" // dims the widget while it is being dragged
	}
	args := []any{Class(cellClass), Attr("data-widget", props.ID)}
	if style := gridStyle(gridCol, gridRow); style != nil {
		args = append(args, Style(style))
	}
	if props.Draggable {
		id := props.ID
		args = append(args,
			Attr("draggable", "true"),
			// Keyboard alternatives to pointer drag/resize (WCAG 2.1.1): focus a tile
			// and use the arrow keys to move it one slot earlier/later; hold Shift to
			// grow/shrink its span instead (B15).
			Attr("tabindex", "0"),
			Attr("aria-keyshortcuts", "ArrowUp ArrowDown ArrowLeft ArrowRight Shift+ArrowUp Shift+ArrowDown Shift+ArrowLeft Shift+ArrowRight"),
			OnKeyDown(func(e uic.KeyboardEvent) {
				key := e.GetKey()
				shift := e.JSValue().Get("shiftKey").Bool()
				if shift {
					// Resize: ←/→ adjust width, ↑/↓ adjust height (clamped to bounds).
					dc, dr := 0, 0
					switch key {
					case "ArrowLeft":
						dc = -1
					case "ArrowRight":
						dc = 1
					case "ArrowUp":
						dr = -1
					case "ArrowDown":
						dr = 1
					default:
						return
					}
					e.PreventDefault()
					curCol, curRow := 1, 1
					for _, it := range items {
						if it.ID == id {
							curCol, curRow = it.ColSpan, it.RowSpan
							break
						}
					}
					nc := clampSpan(curCol+dc, maxColSpan)
					nr := clampSpan(curRow+dr, maxRowSpan)
					next := dashlayout.ResizeItem(items, id, nc, nr)
					itemsAtom.Set(next)
					uistate.PersistItems(next)
					return
				}
				// Move one slot earlier (←/↑) or later (→/↓).
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
				baked := dashlayout.Arrange(items, mode)
				ci := -1
				for i, it := range baked {
					if it.ID == id {
						ci = i
						break
					}
				}
				if ci < 0 {
					return
				}
				next := dashlayout.Move(baked, id, ci+delta)
				itemsAtom.Set(next)
				uistate.PersistItems(next)
				if mode != dashlayout.ModeCustom {
					modeAtom.Set(dashlayout.ModeCustom)
					uistate.PersistLayoutMode(dashlayout.ModeCustom)
				}
			}),
			OnDragStart(func() { dragSrc.Set(id) }),
			OnDragOver(Prevent(func() { dragPreview.Set(id) })), // allow drop + live preview
			OnDrop(Prevent(func() {
				// Reorder the dragged tile to the drop target's position, then the
				// grid re-Packs around it (iOS-home-screen reflow) instead of a
				// pairwise swap. A manual drag is an explicit hand-arrangement, so it
				// bakes the current (possibly auto-arranged) order into the sequence
				// and switches to Custom mode (C24).
				if src := dragSrc.Get(); src != "" && src != id {
					baked := dashlayout.Arrange(items, mode)
					ti := -1
					for i, it := range baked {
						if it.ID == id {
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
			})),
			OnDragEnd(func() { dragSrc.Set(""); dragPreview.Set("") }), // clear (reverts preview if dropped outside)
		)
	}
	args = append(args,
		Div(Class("wh"),
			Span(Class("grip"), Attr("aria-hidden", "true"), "⠿"), // decorative drag grip
			H3(props.Title),
			gear,
		),
		Div(Class(bodyClass), props.Body),
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
		args = append(args,
			// Click cycles the span up and wraps at the max back to 1; with Pack
			// the grid reflows around the new size (so growing never overlaps and
			// the wrap is how you shrink). Tooltip says so.
			Div(Class("rz"), Attr("data-dir", "r"), Attr("title", "Resize width (cycles 1→4)"),
				OnClick(func() {
					span := curCol + 1
					if span > maxColSpan {
						span = 1
					}
					resize(span, curRow)
				}),
			),
			Div(Class("rz"), Attr("data-dir", "b"), Attr("title", "Resize height (cycles 1→3)"),
				OnClick(func() {
					span := curRow + 1
					if span > maxRowSpan {
						span = 1
					}
					resize(curCol, span)
				}),
			),
		)
	}
	return Div(args...)
}

// clampSpan keeps a grid span within [1, max].
func clampSpan(v, max int) int {
	if v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
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

type gearButtonProps struct {
	OnClick func()
}

// gearButton is its own component so its click hook stays stable across the many
// widgets rendered in a list (the On*-hooks-in-loops rule).
func gearButton(props gearButtonProps) uic.Node {
	onClick := props.OnClick
	return Button(
		Class("gear-inline"),
		Type("button"),
		Attr("title", "Widget settings"),
		Attr("aria-label", "Widget settings"), // icon-only button → explicit name (B15)
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		Span(Attr("aria-hidden", "true"), "⚙"),
	)
}
