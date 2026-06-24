// SPDX-License-Identifier: MIT

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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
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
			// bump() re-renders the nav so the new page appears in MY PAGES
			// immediately after navigation (C32 gap #67).
			bump()
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
		css.Class("nav nv", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.Px3, tw.Py2, tw.Rounded4, tw.CursorPointer, tw.TextFaint),
		Title(uistate.T("rail.newPage")),
		OnClick(create),
		ui.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
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
		hiddenSection = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap05),
			railHeader(uistate.T("pages.hiddenSection")),
			hrows,
		)
	}

	// A single root Div (not a bare Fragment) so the section renders inline at its
	// position among the other rail groups; a Fragment of mixed children doesn't
	// preserve sibling order next to the MapKeyed groups.
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap05),
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
	menuID := uic.UseId()
	// WAI-ARIA dismissal for the per-page ⋯ menu: Escape (refocus trigger) + outside
	// pointerdown. This menu had NO dismissal at all (not even a backdrop), so it
	// stayed open until an item was picked. See ui.DismissPopover.
	ui.DismissPopover(open.Get(), menuID, func() { open.Set(false) })
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	p := props.Page
	path := uistate.RoutePath("/p/" + p.Slug)

	navBase := tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap25, tw.Px3, tw.Py2, tw.Rounded4, tw.CursorPointer, tw.MinW0, tw.Flex1)
	cls := "nav nv " + navBase
	if props.Active {
		cls = "nv " + navBase + " " + tw.Fold(tw.BgHex1c, tw.TextFg, tw.FontMedium)
	} else if p.Hidden {
		cls = "nav nv " + navBase + " " + tw.Fold(tw.TextFaint)
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
			ui.Icon(icon.Page, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class(tw.Truncate), p.Name),
		)
	} else {
		link = A(ClassStr(cls), Title(p.Name), OnClick(func() { nav.Navigate(path) }),
			ui.Icon(icon.Page, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class(tw.Truncate), p.Name),
		)
	}

	hideLabel := uistate.T("pages.hide")
	if p.Hidden {
		hideLabel = uistate.T("pages.show")
	}

	var menu uic.Node = Fragment()
	if open.Get() {
		menu = Div(css.Class(tw.Absolute, tw.Right1, tw.TopFull, tw.Mt1, tw.Z30, tw.MinW150, tw.Rounded4, tw.Border, tw.BorderLine, tw.BgBase, tw.P1, tw.Text13, tw.ShadowLg, tw.Flex, tw.FlexCol, tw.Gap05),
			Attr("role", "menu"),
			Button(css.Class(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover), Type("button"), Attr("role", "menuitem"),
				OnClick(func() { open.Set(false); props.OnRename() }), uistate.T("pages.rename")),
			Button(css.Class(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover), Type("button"), Attr("role", "menuitem"),
				OnClick(func() { open.Set(false); props.OnHide() }), hideLabel),
			Button(css.Class(tw.WFull, tw.TextLeft, tw.Px2, tw.Py15, tw.Rounded, tw.HoverBgHover, tw.TextDown), Type("button"), Attr("role", "menuitem"),
				OnClick(func() { open.Set(false); props.OnDelete() }), uistate.T("pages.delete")),
		)
	}

	return Div(css.Class(tw.Relative, tw.Flex, tw.ItemsCenter), Attr("id", menuID),
		link,
		Button(css.Class("rail-section", tw.ShrinkO, tw.Px15, tw.Py1, tw.TextFaint, tw.HoverTextFg), Type("button"),
			Title(uistate.T("pages.menu")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
			OnClick(func() { open.Set(!open.Get()) }), ui.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
		menu,
	)
}
