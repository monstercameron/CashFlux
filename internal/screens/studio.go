// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/pages"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// StudioHubProps configures a StudioHub (currently empty; reserved for future
// extensibility such as a default-tab override from a route parameter).
type StudioHubProps struct{}

// StudioHub is the unified /studio surface: a tabbed hub combining the node-graph
// widget builder, the dashboard widget layout manager, and the custom-pages list.
// It owns the active-tab state; each tab body is a separate registered component
// so its hooks are scoped to that component and cannot interfere with the hub's
// own hook chain (GWC hook-ordering rule — tab bodies must be ui.CreateElement calls).
//
// NOTE: /studio is not yet routed. The route is registered in a later rail-regroup
// commit (FEATURE_MAP §5.3). The existing /widget-builder, /widget-manager, and
// /p/:slug routes continue to function unchanged.
func StudioHub(props StudioHubProps) ui.Node {
	tab := ui.UseState("design")

	return Div(
		Div(css.Class(tw.Mt2),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    uistate.T("studio.tabsLabel"),
				Selected: tab.Get(),
				OnSelect: func(v string) { tab.Set(v) },
				Options: []uiw.SegOption{
					{Value: "design", Label: uistate.T("studio.tabDesign")},
					{Value: "formulas", Label: uistate.T("studio.tabFormulas")},
					{Value: "fields", Label: uistate.T("studio.tabFields")},
					{Value: "workflows", Label: uistate.T("studio.tabWorkflows")},
					{Value: "build", Label: uistate.T("studio.tabBuild")},
					{Value: "manage", Label: uistate.T("studio.tabManage")},
					{Value: "pages", Label: uistate.T("studio.tabPages")},
				},
			}),
		),
		// Tab body: isolated via ui.CreateElement so hooks in each panel are
		// scoped to that component and never share positions with the hub's hooks.
		func() ui.Node {
			switch tab.Get() {
			case "formulas":
				return ui.CreateElement(studioFormulasPanel, studioFormulasPanelProps{})
			case "fields":
				return ui.CreateElement(studioFieldsPanel, studioFieldsPanelProps{})
			case "workflows":
				return ui.CreateElement(studioWorkflowsPanel, studioWorkflowsPanelProps{})
			case "build":
				return ui.CreateElement(studioBuilderPanel, studioBuilderPanelProps{})
			case "manage":
				return ui.CreateElement(studioManagerPanel, studioManagerPanelProps{})
			case "pages":
				return ui.CreateElement(studioPagesPanel, studioPagesPanelProps{})
			default: // "design" — the spec-based widget designer
				return ui.CreateElement(studioDesignerPanel, studioDesignerPanelProps{})
			}
		}(),
	)
}

// Studio returns a StudioHub node. This is the View func for the future /studio
// route (unrouted until the rail-regroup commit, FEATURE_MAP §5.3).
func Studio() ui.Node { return ui.CreateElement(StudioHub, StudioHubProps{}) }

// ─── Tab panel components ─────────────────────────────────────────────────────

type studioBuilderPanelProps struct{}

// studioBuilderPanel wraps the VisualBuilder screen as an isolated component so
// VisualBuilder's hooks (UseEffect × 3, UseState × 9, UseEvent × n) are scoped
// to this component and do not occupy positions inside StudioHub's hook chain.
func studioBuilderPanel(_ studioBuilderPanelProps) ui.Node { return VisualBuilder() }

type studioFormulasPanelProps struct{}

// studioFormulasPanel embeds the formula/compound-variable surface as a Studio
// tab, isolated so its hooks are scoped to this component. Grouping formulas
// with the widget designer keeps every "what to measure" tool in one place.
func studioFormulasPanel(_ studioFormulasPanelProps) ui.Node { return StudioFormulas() }

type studioFieldsPanelProps struct{}

// studioFieldsPanel embeds the custom-fields editor as a Studio tab (isolated
// hooks), under the studio masthead every sibling tab opens with, so custom
// values live alongside formulas and the designer that consumes them.
func studioFieldsPanel(_ studioFieldsPanelProps) ui.Node {
	// Fields() now owns its masthead (single source), so the tab just embeds it.
	return CustomFields()
}

type studioWorkflowsPanelProps struct{}

// studioWorkflowsPanel embeds the workflow-automation surface as a Studio tab
// (isolated hooks — Workflows owns a large hook chain of its own). Automations
// belong beside the formulas, fields, and widgets they act on; the standalone
// /workflows route stays off-rail for bookmarks.
func studioWorkflowsPanel(_ studioWorkflowsPanelProps) ui.Node { return Workflows() }

type studioManagerPanelProps struct{}

// studioManagerPanel wraps the WidgetManager screen in an isolated component scope.
func studioManagerPanel(_ studioManagerPanelProps) ui.Node { return WidgetManager() }

type studioPagesPanelProps struct{}

// studioPagesPanel is the Studio "My pages" surface, rebuilt as a bespoke page
// registry: each page a ledger row (serif name, mono address, widget count,
// Open link, ⋯ menu whose delete runs a two-step inline confirm — a page takes
// its widgets and layout with it), beside a composer rail whose live footprint
// previews the address the page will get. Its hooks are isolated here; each row
// is a further isolated component (studioPageRow) per the no-hooks-in-loops rule.
func studioPagesPanel(_ studioPagesPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	nav := router.UseNavigate()

	// version forces a re-render when a page is created or deleted without
	// changing the route (the atom read registers the re-render subscription).
	version := ui.UseState(0)
	_ = version.Get()
	bump := func() { version.Set(version.Get() + 1) }

	newName := ui.UseState("")
	onNewName := ui.UseEvent(func(v string) { newName.Set(v) })

	createPage := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(newName.Get())
		if name == "" || app == nil {
			return
		}
		all := app.CustomPages()
		p := domain.CustomPage{
			ID:        id.New(),
			Slug:      pages.UniqueSlug(name, all, ""),
			Name:      name,
			Icon:      "page",
			Order:     pages.NextOrder(all),
			CreatedAt: time.Now(),
		}
		if err := app.PutCustomPage(p); err != nil {
			return
		}
		newName.Set("")
		bump()
		nav.Navigate(uistate.RoutePath("/p/" + p.Slug))
	}))

	all := app.CustomPages()
	ordered := pages.Ordered(all)

	rows := MapKeyed(ordered,
		func(pg domain.CustomPage) any { return pg.ID },
		func(pg domain.CustomPage) ui.Node {
			return ui.CreateElement(studioPageRow, studioPageRowProps{
				Page: pg,
				OnDelete: func() {
					if app != nil {
						_ = app.DeleteCustomPage(pg.ID)
						bump()
					}
				},
			})
		},
	)

	countStr := uistate.T("spg.countNone")
	switch {
	case len(ordered) == 1:
		countStr = uistate.T("spg.countOne")
	case len(ordered) > 1:
		countStr = uistate.T("spg.countMany", len(ordered))
	}

	masthead := Div(css.Class("wman-head"),
		Span(css.Class("studio-eyebrow"), uistate.T("spg.eyebrow")),
		H2(css.Class("studio-design-title"), uistate.T("spg.title")),
		P(css.Class("studio-design-sub"), uistate.T("spg.lede")),
	)

	var registry ui.Node
	if len(ordered) == 0 {
		registry = P(css.Class("spg-empty"), uistate.T("spg.empty"))
	} else {
		registry = Div(css.Class("spg-rows"), rows)
	}

	// Composer: name → live address footprint → create.
	trimmed := strings.TrimSpace(newName.Get())
	slugPreview := ""
	if trimmed != "" {
		slugPreview = "/p/" + pages.UniqueSlug(trimmed, all, "")
	}
	composer := Div(css.Class("spg-composer"), Attr("data-testid", "pages-composer"),
		H3(css.Class("spg-comp-title"), uistate.T("spg.compTitle")),
		P(css.Class("spg-comp-lede"), uistate.T("spg.compLede")),
		Form(css.Class("spg-form"), OnSubmit(createPage),
			Label(css.Class("fld-field"),
				Span(css.Class("fld-lbl"), uistate.T("studio.pageName")),
				Input(css.Class("field"), Type("text"), Attr("id", "studio-new-page"),
					Attr("aria-label", uistate.T("studio.pageName")),
					Placeholder(uistate.T("spg.namePlaceholder")),
					Value(newName.Get()), OnInput(onNewName)),
			),
			Div(css.Class("fld-foot"),
				Span(css.Class("fld-foot-title"), uistate.T("spg.footTitle")),
				If(slugPreview != "",
					P(css.Class("fld-foot-line"), uistate.T("spg.livesAt"), " ",
						Span(css.Class("spg-slug"), slugPreview))),
				P(css.Class("fld-foot-line"), uistate.T("spg.footHint")),
			),
			Button(css.Class("btn btn-primary spg-create"), Type("submit"),
				uistate.T("studio.createPage")),
		),
	)

	return Div(css.Class("spg"),
		masthead,
		Div(css.Class("spg-grid"),
			Div(css.Class("spg-main"),
				Div(css.Class("spg-reg-head"),
					Span(css.Class("wman-aside-label"), uistate.T("spg.registryKicker")),
					Span(css.Class("wman-count"), countStr),
				),
				registry,
			),
			composer,
		),
	)
}

type studioPageRowProps struct {
	Page     domain.CustomPage
	OnDelete func()
}

// studioPageRow renders one page's registry row: serif name, monospace address,
// widget count, an Open link, and a ⋯ menu whose destructive item opens a
// two-step inline confirm (the page's widgets and layout go with it). Its own
// component so its hooks sit at stable positions outside any loop.
func studioPageRow(props studioPageRowProps) ui.Node {
	pg := props.Page
	confirming := ui.UseState(false)
	ask := ui.UseEvent(Prevent(func() {
		confirming.Set(true)
		fldFocusSoon("#spg-keep-" + pg.ID)
	}))
	keep := ui.UseEvent(Prevent(func() {
		confirming.Set(false)
		fldFocusSoon("#spg-menu-" + pg.ID + " button")
	}))
	del := ui.UseEvent(Prevent(func() {
		if props.OnDelete != nil {
			props.OnDelete()
		}
	}))

	widgets := uistate.T("spg.widgetsNone")
	switch {
	case len(pg.Widgets) == 1:
		widgets = uistate.T("spg.widgetsOne")
	case len(pg.Widgets) > 1:
		widgets = uistate.T("spg.widgetsMany", len(pg.Widgets))
	}

	return Div(css.Class("spg-row"),
		Div(css.Class("spg-row-main"),
			Div(css.Class("spg-row-top"),
				Span(css.Class("spg-name"), pg.Name),
				If(pg.Hidden, Span(css.Class("wman-hidden-tag"), uistate.T("wman.hiddenTag"))),
			),
			Div(css.Class("spg-row-sub"),
				Span(css.Class("spg-slug"), "/p/"+pg.Slug),
				Span(css.Class("spg-meta"), widgets),
			),
		),
		A(css.Class("spg-open"),
			Href(uistate.RoutePath("/p/"+pg.Slug)),
			Attr("aria-label", uistate.T("studio.goToPage")),
			uistate.T("spg.open"),
		),
		If(!confirming.Get(),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "spg-menu-" + pg.ID,
				ToggleTestID: "spg-menu-btn-" + pg.ID,
				Items: []ui.Node{
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"),
						Attr("data-testid", "spg-delete-btn-"+pg.ID),
						Attr("aria-label", uistate.T("studio.deletePageAria", pg.Name)),
						OnClick(ask), uistate.T("studio.deletePage")),
				},
			})),
		If(confirming.Get(), Div(css.Class("fld-confirm"), Attr("role", "alert"),
			Span(css.Class("fld-confirm-msg"), uistate.T("spg.deleteWarn")),
			Button(css.Class("fld-confirm-del"), Type("button"), OnClick(del), uistate.T("spg.deleteYes")),
			Button(css.Class("fld-confirm-keep"), Type("button"), Attr("id", "spg-keep-"+pg.ID), OnClick(keep), uistate.T("fld.deleteNo")),
		)),
	)
}
