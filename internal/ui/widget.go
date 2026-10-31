// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetstyle"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// widgetRoute maps a dashboard tile's stable id to the screen that owns its data, so the tile
// title can drill into that screen on click. An empty result means the tile has no natural
// destination and its title stays plain text.
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
	case "health":
		return "/health"
	}
	return ""
}

// widgetIcon maps a dashboard tile's stable id to a leading header glyph, so KPI tiles are
// scannable by shape as well as by text. An empty name means the tile has no icon.
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
	case "health":
		return icon.Insights
	case "breakdown", "budgets":
		return icon.Budgets
	case "goals":
		return icon.Goals
	case "todo":
		return icon.Todo
	case "highlight":
		return icon.Insights
	}
	// User-built Widget Builder cards (namespaced "wb:") get a generic glyph so they match the
	// built-in tiles instead of having none.
	if strings.HasPrefix(id, "wb:") {
		return icon.Sparkles
	}
	return ""
}

// periodWindowedWidgets are the tiles whose figures are a window of activity. They follow the
// period picker, and they always show the selected period as a chip. Without that chip a reader
// cannot tell a "this month" figure from a "last three months" figure when both sit on one
// screen. A number that names its window is the cheapest form of data trust in the app.
var periodWindowedWidgets = map[string]bool{
	"kpi-income":   true,
	"kpi-spending": true,
	"cashflow":     true,
	"breakdown":    true,
	"savings":      true,
	"budgets":      true,
}

// todayBadgeWidgets are the tiles whose figures are current state: positions, queues and
// forward-looking lists. They do not re-window when the period changes. While the viewed period
// does not contain today, these tiles wear a "Today" chip, so the figure is labelled rather than
// read as the selected month's value. Period-windowed tiles such as income, spending and budgets
// are absent on purpose, because they follow the period.
var todayBadgeWidgets = map[string]bool{
	"kpi-networth":    true,
	"kpi-assets":      true,
	"kpi-liabilities": true,
	"kpi-safetospend": true,
	"attention":       true,
	"bills":           true,
	"recent":          true,
	"accounts":        true,
	"todo":            true,
	"goals":           true,
	"goal-states":     true,
	"health":          true,
	"freshness":       true,
	"highlight":       true,
	"smart-digest":    true,
	"anomaly-hub":     true,
}

// gridCols is the number of columns the bento grid packs tiles into.
const gridCols = 4

// Span limits for the resize handles: a tile spans at most all four columns and three rows.
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
	// Style overlays spec-level inline CSS (a declarative WidgetSpec.Style, §7.7) on
	// the tile, applied on top of the per-tile widgetstyle config. Used by custom
	// content-layout (compound) widgets that carry their own token-first style.
	Style     map[string]string
	Draggable bool   // mark the cell draggable (drag-reorder behavior wired separately)
	Resizable bool   // show the directional resize handles
	OnGear    func() // open this widget's settings (gear click)
	// ChromeHover renders the tile borderless and chromeless (no card surface, grip,
	// or gear) until it is hovered, when the surface + controls fade in. Used for the
	// dashboard welcome/hero so it reads as clean content but is still a configurable
	// widget. CSS: .w.chrome-hover in web/index.html.
	ChromeHover bool
	// Preview renders the tile as a non-interactive preview (e.g. the Studio live
	// preview): no settings gear, drag grip, or resize handles — the surrounding
	// editor owns configuration, so those per-tile affordances would be dead/
	// misleading here. Pair with Draggable=false / Resizable=false.
	Preview bool
}

// Widget is the bento cell shell shared by every dashboard widget: a square outlined cell with the
// unified header (grip, centered title, gear) and a padded body, plus optional edge resize
// handles. Callers supply only the title and the body, and the chrome is the same everywhere.
func Widget(props WidgetProps) uic.Node {
	return uic.CreateElement(widget, props)
}

func widget(props WidgetProps) uic.Node {
	bodyClass := "wbody"
	if props.BodyClass != "" {
		bodyClass += " " + props.BodyClass
	}

	// The viewed period decides whether a current-state tile shows its "Today" chip. The hook is
	// called unconditionally so the hook order stays stable.
	viewedPeriod := uistate.UsePeriod().Get()

	// By default the gear opens this widget's settings panel; callers may
	// override with an explicit OnGear.
	settings := uistate.UseSettings()
	onGear := props.OnGear
	if onGear == nil {
		id, title := props.ID, props.Title
		onGear = func() { settings.Set(uistate.Widget(id, title)) }
	}

	// Grid placement comes from packing the shared item sequence, so drag-reorder and resize
	// reflow without overlap. The layout mode decides the order before packing: Custom uses the
	// stored sequence as it is, and the auto modes reorder it. Sizes are not changed by the mode.
	// Pack returns 1-indexed rows, and the dashboard has no fixed header row, so tiles fill from
	// row 1.
	itemsAtom := uistate.UseLayoutItems()
	items := itemsAtom.Get()
	modeAtom := uistate.UseLayoutMode()
	mode := modeAtom.Get()
	// Roving tabindex and grab state are read unconditionally, and used only for draggable tiles.
	// The grid is a single Tab stop: arrows move focus between tiles, and Space or Enter grabs a
	// tile so that arrows can move or resize it.
	rovingAtom := uistate.UseCurrentTile()
	grabbedAtom := uistate.UseGrabbedTile()
	// Pointer drag-reorder and the visual rearranging chrome (grip and resize handles) engage only
	// in layout-edit mode. CSS hides that chrome through the bento's data-layout-edit attribute.
	// Every page other than the dashboard is always editable. The atom is read unconditionally so
	// the hook order stays the same, and only the attribute values change with edit mode.
	layoutEditable := uistate.UseLayoutEdit().Get() ||
		router.GetCurrentPath() != uistate.RoutePath("/")

	// Drop hidden widgets before packing so the visible tiles reflow into the gaps. The dashboard
	// skips rendering hidden tiles, and this keeps everyone else's placement correct. Visibility is
	// owned by the Widget Manager.
	if hidden := uistate.UseHiddenWidgets().Get(); len(hidden) > 0 {
		kept := items[:0:0]
		for _, it := range items {
			if !hidden.IsHidden(it.ID) {
				kept = append(kept, it)
			}
		}
		items = kept
	}

	// Every tile is configurable: the settings panel always offers a per-tile color and an
	// importance rank, so the gear always shows. A preview tile omits it, because a preview has no
	// settings host to open the panel in.
	var gear uic.Node = Fragment()
	if !props.Preview {
		gear = uic.CreateElement(gearButton, gearButtonProps{OnClick: onGear, Title: props.Title, ID: props.ID})
	}

	// The drag is driven by the DOM, not by state. The coordinator dims the dragged tile directly on
	// its element (internal/ui/bentoflip.go), and the reorder is written once, on drop. A render
	// during the drag would recompute the whole dashboard for every dragover.
	arranged := dashlayout.Arrange(items, mode)
	packed := dashlayout.Pack(arranged, gridCols)
	gridCol, gridRow := props.GridColumn, props.GridRow
	colSpan, rowSpan := 1, 1
	if p, ok := packed.Get(props.ID); ok {
		gridCol = p.GridColumn()
		gridRow = p.GridRow()
		colSpan = p.ColSpan
		rowSpan = p.RowSpan
	}

	cellClass := "w"
	if props.ChromeHover {
		cellClass += " chrome-hover" // borderless + chromeless until hovered (CSS)
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
	// Spec-level declarative style (compound content-layout widgets) overlays last so
	// the author's token-first WidgetSpec.Style is the source of truth for the tile.
	for k, v := range props.Style {
		style[k] = v
	}
	// The renderer doesn't reset an omitted style key in place, so a cleared accent
	// strip / shadow would linger; always write box-shadow (to "none" when unset) so
	// removing the style reverts the tile.
	if _, ok := style["box-shadow"]; !ok {
		style["box-shadow"] = "none"
	}
	args = append(args, Style(style))
	dragOptions := func() []any {
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
		dragAttr := "false"
		if layoutEditable {
			dragAttr = "true"
		}
		return []any{
			Attr("draggable", dragAttr),
			Attr("tabindex", tabidx),
			Attr("aria-grabbed", grabbedAttr),
			// APG grid pattern (WCAG 2.1.1 keyboard support without a Tab stop per tile): arrows move
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
					if e.GetShiftKey() {
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
						curCol, curRow := tileSpan(items, id)
						next := dashlayout.ResizeItem(items, id, dashlayout.ClampSpan(curCol+dc, maxColSpan), dashlayout.ClampSpan(curRow+dr, maxRowSpan))
						itemsAtom.Set(next)
						uistate.PersistItems(next)
						return
					}
					bk := dashlayout.Arrange(items, mode)
					ci := tileIndex(bk, id)
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
				ci := tileIndex(bk, id)
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
				if el := domValue(domDocument(), "querySelector", "[data-widget=\""+nextID+"\"]"); el.Truthy() {
					domValue(el, "focus")
				}
			}),
			OnDragStart(func() {
				// A panic in a drag handler is otherwise fatal (the framework re-panics
				// unhandled event errors → "Go program has already exited" → frozen app),
				// so contain + log it here. (internal/ui/bentoflip.go recoverBento.)
				defer recoverBento("widget.OnDragStart")
				bentoDragStart(id) // snapshots geometry + dims the tile directly (no atom)
			}),
			// Allow the drop: the coordinator's document dragover listener computes the
			// stable insertion target continuously, so the per-tile handler only needs to
			// preventDefault. No dragPreview atom is written, so there is no re-render.
			OnDragOver(Prevent(func(uic.Event) {})),
			OnDrop(Prevent(func(uic.Event) {})), // preventDefault only; reorder is in OnDragEnd
			OnDragEnd(func() {
				// The one place a drag writes state. dragend fires on the source element even when the
				// drop lands outside a tile. The coordinator has already stashed the source and the
				// insertion target, so they are read here. The dashboard re-renders once, for the
				// reorder, and never during the drag.
				defer recoverBento("widget.OnDragEnd")
				src, target := LastDropSource(), LastDropTarget()
				if src == "" {
					src = id // dragend fires on the source tile
				}
				if src == "" || target == "" || src == target {
					return
				}
				baked := dashlayout.Arrange(items, mode)
				ti := tileIndex(baked, target)
				if ti < 0 {
					return
				}
				next := dashlayout.Move(baked, src, ti)
				itemsAtom.Set(next)
				uistate.PersistItems(next)
				if mode != dashlayout.ModeCustom {
					modeAtom.Set(dashlayout.ModeCustom)
					uistate.PersistLayoutMode(dashlayout.ModeCustom)
				}
			}),
		}
	}()
	if props.Draggable {
		args = append(args, dragOptions...)
	}
	// The title links to the tile's data screen when there is one, and is a plain heading
	// otherwise. It is distinct from the grip (drag) and the gear (settings). The title is an H2,
	// not an H3, because the page's H1 sits in the topbar, so the heading levels do not skip.
	var titleNode uic.Node = H2(props.Title)
	if route := widgetRoute(props.ID); route != "" {
		titleNode = uic.CreateElement(viewTitle, viewTitleProps{Title: props.Title, Route: route})
	}
	// A leading glyph makes KPI tiles scannable by shape. It is decorative, so it sits beside the
	// title, which stays clickable, rather than inside the link.
	if ic := widgetIcon(props.ID); ic.Valid() {
		titleNode = Span(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tw.MinW0), Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)), titleNode)
	}
	// Current-state tiles wear a "Today" chip while the dashboard is paged to
	// another period (see todayBadgeWidgets).
	var todayBadge uic.Node = Fragment()
	switch {
	case props.Preview:
		// A settings/preview tile has no live window to describe.
	case todayBadgeWidgets[props.ID]:
		ws, we := viewedPeriod.Range()
		if now := time.Now(); now.Before(ws) || !now.Before(we) {
			label := uistate.T("widget.todayBadge")
			todayBadge = Span(css.Class("w-today"), Attr("data-testid", "w-today-badge"),
				Attr("title", uistate.T("widget.todayBadgeTitle")), label)
		}
	case periodWindowedWidgets[props.ID]:
		// Always shown: this figure is the selected window. Naming that window stops the figure
		// being compared against a figure with a different window elsewhere on the page.
		if label := viewedPeriod.Label(); label != "" {
			todayBadge = Span(css.Class("w-today", "w-window"), Attr("data-testid", "w-window-badge"),
				Attr("title", uistate.T("widget.windowBadgeTitle")), label)
		}
	}

	// The drag grip signals draggability; a non-interactive preview tile drops it.
	var grip uic.Node = Fragment()
	if !props.Preview {
		grip = Span(css.Class("grip"), Attr("aria-hidden", "true"), Icon(icon.Grip, css.Class(tw.W4, tw.H4))) // six-dot drag grip
	}
	// A preview tile with no title, no grip and no leading icon would render an empty header. That
	// left a gap above surface-page content. Skip the header and give the body its own top padding.
	if props.Preview && props.Title == "" && !widgetIcon(props.ID).Valid() {
		args = append(args, Div(ClassStr(bodyClass+" wbody-nohead"), props.Body))
	} else {
		args = append(args,
			Div(css.Class("wh"),
				grip,
				titleNode,
				todayBadge,
				gear,
			),
			Div(ClassStr(bodyClass), props.Body),
		)
	}
	if props.Resizable {
		id := props.ID
		curCol, curRow := tileSpan(items, id)
		resize := func(cs, rs int) {
			next := dashlayout.ResizeItem(items, id, cs, rs)
			itemsAtom.Set(next)
			uistate.PersistItems(next)
		}
		resizeHandle := func(dir, label string, disabled bool, onClick func()) uic.Node {
			// Every resizable tile renders all four handles, so each accessible name says
			// which tile it resizes as well as the direction.
			if props.Title != "" {
				label = label + " — " + props.Title
			}
			cls := "rz"
			if disabled {
				cls += " off"
			}
			return Button(
				ClassStr(cls),
				Type("button"),
				Attr("data-testid", "widget-rz-"+dir+"-"+id),
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

// tileIndex returns the position of the tile with the given id in items, or -1 when it is absent.
func tileIndex(items []dashlayout.Item, id string) int {
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

// tileSpan returns the column and row span of the tile with the given id. A tile that is absent
// from items reports a 1 by 1 span.
func tileSpan(items []dashlayout.Item, id string) (col, row int) {
	if i := tileIndex(items, id); i >= 0 {
		return items[i].ColSpan, items[i].RowSpan
	}
	return 1, 1
}

type viewTitleProps struct {
	Title string
	Route string
}

// viewTitle renders a dashboard tile's title as a link to the screen that owns the tile's data.
func viewTitle(props viewTitleProps) uic.Node {
	route := props.Route
	return A(
		css.Class("wh-title"),
		Attr("href", uistate.RoutePath(route)),
		Attr("title", uistate.T("widget.open")),
		Attr("aria-label", uistate.T("widget.openNamed", props.Title)),
		props.Title,
	)
}

type gearButtonProps struct {
	OnClick func()
	// Title is the owning widget's name. A dashboard renders many of these buttons, so each
	// accessible name says which widget it configures.
	Title string
	// ID is the owning widget's stable id, for the button's data-testid.
	ID string
}

// gearButton is its own component so its click hook stays at a stable position across the many
// widgets rendered in a list.
func gearButton(props gearButtonProps) uic.Node {
	onClick := props.OnClick
	label := uistate.T("widget.settings")
	if props.Title != "" {
		label = uistate.T("widget.settingsFor", props.Title)
	}
	return Button(
		css.Class("gear-inline"),
		Type("button"),
		Attr("data-testid", "widget-gear-"+props.ID),
		Attr("title", label),
		Attr("aria-label", label), // icon-only, so the button needs an explicit name
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		Icon(icon.Settings, css.Class(tw.W4, tw.H4)),
	)
}
