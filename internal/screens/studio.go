// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/pages"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
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

// studioFormulasPanel embeds the formula/compound-variable editor (Customize) as a
// Studio tab, isolated so its hooks are scoped to this component. Grouping formulas
// with the widget designer keeps every "what to measure" tool in one place.
func studioFormulasPanel(_ studioFormulasPanelProps) ui.Node { return Customize() }

type studioFieldsPanelProps struct{}

// studioFieldsPanel embeds the custom-fields editor as a Studio tab (isolated hooks),
// so custom values live alongside formulas and the designer that consumes them.
func studioFieldsPanel(_ studioFieldsPanelProps) ui.Node { return CustomFields() }

type studioManagerPanelProps struct{}

// studioManagerPanel wraps the WidgetManager screen in an isolated component scope.
func studioManagerPanel(_ studioManagerPanelProps) ui.Node { return WidgetManager() }

type studioPagesPanelProps struct{}

// studioPagesPanel lists all user-authored custom pages and provides inline create,
// navigate, and delete management. It is its own component so its hooks (UseState,
// UseEvent × 2, UseNavigate) are isolated; each page row is a further isolated
// component (studioPageRow) to satisfy the no-hooks-in-loops rule.
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

	// Inline "New page" form state.
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

	var rowNodes []ui.Node
	for _, pg := range ordered {
		pg := pg // capture loop variable
		rowNodes = append(rowNodes, ui.CreateElement(studioPageRow, studioPageRowProps{
			Page: pg,
			OnDelete: func() {
				if app != nil {
					_ = app.DeleteCustomPage(pg.ID)
					bump()
				}
			},
		}))
	}

	// Create-page form: title + form together in one section.
	formSection := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("studio.tabPages"),
		Body: Form(css.Class("form-grid"), OnSubmit(createPage),
			Input(css.Class("field"), Type("text"), Attr("id", "studio-new-page"),
				Attr("aria-label", uistate.T("studio.pageName")),
				Placeholder(uistate.T("studio.pageName")),
				Value(newName.Get()), OnInput(onNewName)),
			Button(css.Class("btn btn-primary"), Type("submit"),
				uistate.T("studio.createPage")),
		),
	})

	// Pages list: nil Rows → EntityListSection renders the EmptyState instead.
	listSection := uiw.EntityListSection(uiw.EntityListSectionProps{
		EmptyState: ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message: uistate.T("studio.pagesEmpty"),
		}),
		Rows: rowNodes, // nil when no pages → triggers EmptyState
	})

	return Fragment(formSection, listSection)
}

type studioPageRowProps struct {
	Page     domain.CustomPage
	OnDelete func()
}

// studioPageRow renders one custom-page row: the page name, its slug as meta, a
// navigation link, and a delete button. Its own component so the delete UseEvent
// hook sits at a stable position outside any loop.
func studioPageRow(props studioPageRowProps) ui.Node {
	pg := props.Page
	del := ui.UseEvent(Prevent(func() {
		if props.OnDelete != nil {
			props.OnDelete()
		}
	}))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), pg.Name),
			Span(css.Class("row-meta"), "/p/"+pg.Slug),
		),
		A(css.Class("btn", "btn-sm"),
			Href(uistate.RoutePath("/p/"+pg.Slug)),
			Attr("aria-label", uistate.T("studio.goToPage")),
			uistate.T("studio.goToPage"),
		),
		Button(css.Class("btn-del"), Type("button"),
			Attr("aria-label", uistate.T("studio.deletePageAria", pg.Name)),
			Title(uistate.T("studio.deletePage")),
			OnClick(del),
			uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4)),
		),
	)
}
