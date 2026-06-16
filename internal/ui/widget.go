//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

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

	// Grid placement comes from the shared layout when present (so drag-reorder
	// and resize take effect), falling back to the caller-provided defaults.
	layoutAtom := uistate.UseLayout()
	layout := layoutAtom.Get()
	gridCol, gridRow := props.GridColumn, props.GridRow
	if p, ok := layout.Get(props.ID); ok {
		gridCol, gridRow = p.GridColumn(), p.GridRow()
	}
	dragSrc := uistate.UseDragSource()

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
			OnDragStart(func() { dragSrc.Set(id) }),
			OnDragOver(Prevent(func() {})), // allow drop
			OnDrop(Prevent(func() {
				if src := dragSrc.Get(); src != "" && src != id {
					next := layout.Swap(src, id)
					layoutAtom.Set(next)
					uistate.PersistLayout(next)
				}
				dragSrc.Set("")
			})),
			OnDragEnd(func() { dragSrc.Set("") }), // clear if dropped outside a target
		)
	}
	args = append(args,
		Div(Class("wh"),
			Span(Class("grip"), "⠿"), // ⠿ drag grip
			H3(props.Title),
			uic.CreateElement(gearButton, gearButtonProps{OnClick: onGear}),
		),
		Div(Class(bodyClass), props.Body),
	)
	if props.Resizable {
		id := props.ID
		cur, _ := layout.Get(id)
		args = append(args,
			Div(Class("rz"), Attr("data-dir", "r"), Attr("title", "Widen"),
				OnClick(func() {
					span := cur.ColSpan + 1
					if span > maxColSpan {
						span = 1
					}
					next := layout.Resize(id, span, cur.RowSpan)
					layoutAtom.Set(next)
					uistate.PersistLayout(next)
				}),
			),
			Div(Class("rz"), Attr("data-dir", "b"), Attr("title", "Taller"),
				OnClick(func() {
					span := cur.RowSpan + 1
					if span > maxRowSpan {
						span = 1
					}
					next := layout.Resize(id, cur.ColSpan, span)
					layoutAtom.Set(next)
					uistate.PersistLayout(next)
				}),
			),
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
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		"⚙", // ⚙
	)
}
