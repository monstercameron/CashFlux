//go:build js && wasm

package ui

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
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

	args := []any{Class("w"), Attr("data-widget", props.ID)}
	if style := gridStyle(props.GridColumn, props.GridRow); style != nil {
		args = append(args, Style(style))
	}
	if props.Draggable {
		args = append(args, Attr("draggable", "true"))
	}
	args = append(args,
		Div(Class("wh"),
			Span(Class("grip"), "⠿"), // ⠿ drag grip
			H3(props.Title),
			uic.CreateElement(gearButton, gearButtonProps{OnClick: props.OnGear}),
		),
		Div(Class(bodyClass), props.Body),
	)
	if props.Resizable {
		args = append(args,
			Div(Class("rz"), Attr("data-dir", "r"), Attr("title", "Scale wide")),
			Div(Class("rz"), Attr("data-dir", "b"), Attr("title", "Scale tall")),
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
