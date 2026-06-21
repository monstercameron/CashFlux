//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// CustomPagesNav renders the rail's "My pages" group: the user's visible custom
// pages in their chosen order (drag to reorder), each navigating to /p/<slug>
// with a per-row menu (rename / hide / delete); a "New page" action that creates
// a page and jumps to it; and a collapsible "Hidden pages" list to bring hidden
// pages back. It reuses the pure internal/pages logic for ordering, slugging, and
// reordering, and writes through appstate (autosave persists to localStorage).
func CustomPagesNav() uic.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	nav := router.UseNavigate()
	current := router.InspectCurrentRoute().Path
	// "My pages" is a collapsible rail section too (C67), keyed "mypages".
	collapsedAtom := uistate.UseCollapsedToolGroups()
	collapsed := collapsedAtom.Get()
	myPagesCollapsed := collapsed["mypages"]
	toggleMyPages := func() {
		next := map[string]bool{}
		for k, v := range collapsed {
			next[k] = v
		}
		next["mypages"] = !myPagesCollapsed
		collapsedAtom.Set(next)
		uistate.PersistCollapsedToolGroups(next)
	}
	// A version counter forces a re-render after a mutation that doesn't itself
	// change the route (rename in place, hide/show).
	version := uic.UseState(0)
	_ = version.Get()
	bump := func() { version.Set(version.Get() + 1) }

	all := app.CustomPages()
	visible := pages.Visible(all)
	var hidden []domain.CustomPage
	for _, p := range pages.Ordered(all) {
		if p.Hidden {
			hidden = append(hidden, p)
		}
	}

	dragSrc := uic.UseState("")
	reorder := func(targetSlug string) {
		src := dragSrc.Get()
		dragSrc.Set("")
		if src == "" || src == targetSlug {
			return
		}
		srcPage, ok := pages.BySlug(all, src)
		if !ok {
			return
		}
		ordered := pages.Ordered(all)
		ti := 0
		for i, p := range ordered {
			if p.Slug == targetSlug {
				ti = i
				break
			}
		}
		for _, p := range pages.Reorder(all, srcPage.ID, ti) {
			if err := app.PutCustomPage(p); err != nil {
				return
			}
		}
		bump()
	}

	create := func() {
		promptModal(uistate.T("pages.newPrompt"), uistate.T("pages.newDefault"), func(name string) {
			if name == "" {
				return
			}
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
			nav.Navigate(uistate.RoutePath("/p/" + p.Slug))
		})
	}

	rename := func(p domain.CustomPage) {
		promptModal(uistate.T("pages.renamePrompt"), p.Name, func(name string) {
			if name == "" || name == p.Name {
				return
			}
			p.Name = name
			p.Slug = pages.UniqueSlug(name, all, p.ID)
			if err := app.PutCustomPage(p); err != nil {
				return
			}
			// The slug may have changed; if we're viewing this page, follow it.
			if current == uistate.RoutePath("/p/"+p.Slug) || current != uistate.RoutePath("/") {
				nav.Navigate(uistate.RoutePath("/p/" + p.Slug))
			}
			bump()
		})
	}

	toggleHide := func(p domain.CustomPage) {
		p.Hidden = !p.Hidden
		if err := app.PutCustomPage(p); err != nil {
			return
		}
		bump()
	}

	del := func(p domain.CustomPage) {
		confirmModal(uistate.T("pages.deleteConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteCustomPage(p.ID); err != nil {
				return
			}
			if current == uistate.RoutePath("/p/"+p.Slug) {
				nav.Navigate(uistate.RoutePath("/"))
			} else {
				bump()
			}
		})
	}

	rows := make([]uic.Node, 0, len(visible))
	for _, p := range visible {
		p := p
		slug := p.Slug
		rows = append(rows, uic.CreateElement(customPageRow, customPageRowProps{
			Page:        p,
			Active:      current == uistate.RoutePath("/p/"+slug),
			OnRename:    func() { rename(p) },
			OnHide:      func() { toggleHide(p) },
			OnDelete:    func() { del(p) },
			OnDragStart: func() { dragSrc.Set(slug) },
			OnDrop:      func() { reorder(slug) },
		}))
	}

	// "New page" is a non-navigating action, so it's a plain styled row (navItem
	// only navigates) matching the muted nav styling.
	newPage := A(
		ClassStr("nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer text-faint"),
		Title(uistate.T("rail.newPage")),
		OnClick(create),
		ui.Icon(icon.Plus, ClassStr("w-4 h-4 shrink-0")),
		Span(uistate.T("rail.newPage")),
	)

	// Hidden pages: a small section so hidden pages can be brought back.
	var hiddenSection uic.Node = Fragment()
	if len(hidden) > 0 {
		hrows := make([]uic.Node, 0, len(hidden))
		for _, p := range hidden {
			p := p
			hrows = append(hrows, uic.CreateElement(customPageRow, customPageRowProps{
				Page:     p,
				OnRename: func() { rename(p) },
				OnHide:   func() { toggleHide(p) },
				OnDelete: func() { del(p) },
			}))
		}
		hiddenSection = Div(ClassStr("flex flex-col gap-0.5"),
			railHeader(uistate.T("pages.hiddenSection")),
			hrows,
		)
	}

	// A single root Div (not a bare Fragment) so the section renders inline at its
	// position among the other rail groups; a Fragment of mixed children doesn't
	// preserve sibling order next to the MapKeyed groups.
	body := []any{ClassStr("flex flex-col gap-0.5"),
		uic.CreateElement(toolGroupHeader, toolGroupHeaderProps{
			Label: uistate.T("rail.myPages"), Collapsed: myPagesCollapsed, OnToggle: toggleMyPages,
		}),
	}
	if !myPagesCollapsed {
		body = append(body,
			rows, // []ui.Node — flattened into children by the framework
			newPage,
			hiddenSection,
		)
	}
	return Div(body...)
}

type customPageRowProps struct {
	Page        domain.CustomPage
	Active      bool
	OnRename    func()
	OnHide      func()
	OnDelete    func()
	OnDragStart func()
	OnDrop      func()
}

// customPageRow is one "My pages" entry: a navigating link plus a "⋯" button that
// reveals rename/hide/delete. It's its own component so the menu-toggle and
// click hooks stay stable across the list (the On*-hooks-in-loops rule). The menu
// button is a sibling of the link (not nested), so clicking it doesn't navigate.
func customPageRow(props customPageRowProps) uic.Node {
	nav := router.UseNavigate()
	open := uic.UseState(false)
	p := props.Page
	path := uistate.RoutePath("/p/" + p.Slug)

	cls := "nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer min-w-0 flex-1"
	if props.Active {
		cls = "nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer bg-[#1c1c1e] text-fg font-medium min-w-0 flex-1"
	} else if p.Hidden {
		cls = "nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer text-faint min-w-0 flex-1"
	}

	link := A(ClassStr(cls), Title(p.Name), OnClick(func() { nav.Navigate(path) }))
	if props.OnDragStart != nil {
		onStart, onDrop := props.OnDragStart, props.OnDrop
		link = A(ClassStr(cls), Title(p.Name), OnClick(func() { nav.Navigate(path) }),
			Attr("draggable", "true"),
			OnDragStart(func() {
				if onStart != nil {
					onStart()
				}
			}),
			OnDragOver(Prevent(func() {})),
			OnDrop(Prevent(func() {
				if onDrop != nil {
					onDrop()
				}
			})),
			ui.Icon(icon.Page, ClassStr("w-4 h-4 shrink-0")),
			Span(ClassStr("truncate"), p.Name),
		)
	} else {
		link = A(ClassStr(cls), Title(p.Name), OnClick(func() { nav.Navigate(path) }),
			ui.Icon(icon.Page, ClassStr("w-4 h-4 shrink-0")),
			Span(ClassStr("truncate"), p.Name),
		)
	}

	hideLabel := uistate.T("pages.hide")
	if p.Hidden {
		hideLabel = uistate.T("pages.show")
	}

	var menu uic.Node = Fragment()
	if open.Get() {
		menu = Div(ClassStr("absolute right-1 top-full mt-1 z-30 min-w-[150px] rounded-[4px] border border-line bg-base p-1 text-[13px] shadow-lg flex flex-col gap-0.5"),
			Button(ClassStr("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"),
				OnClick(func() { open.Set(false); props.OnRename() }), uistate.T("pages.rename")),
			Button(ClassStr("w-full text-left px-2 py-1.5 rounded hover:bg-hover"), Type("button"),
				OnClick(func() { open.Set(false); props.OnHide() }), hideLabel),
			Button(ClassStr("w-full text-left px-2 py-1.5 rounded hover:bg-hover text-down"), Type("button"),
				OnClick(func() { open.Set(false); props.OnDelete() }), uistate.T("pages.delete")),
		)
	}

	return Div(ClassStr("relative flex items-center"),
		link,
		Button(ClassStr("rail-section shrink-0 px-1.5 py-1 text-faint hover:text-fg"), Type("button"),
			Title(uistate.T("pages.menu")),
			OnClick(func() { open.Set(!open.Get()) }), ui.Icon(icon.MoreH, ClassStr("w-4 h-4"))),
		menu,
	)
}
