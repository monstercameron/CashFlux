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

// CustomPagesNav renders the rail's "My pages" group: the user's custom pages in
// their chosen order (drag to reorder), each navigating to /p/<slug>, plus a
// "New page" action that creates a page and jumps to it. It reuses navItem (so
// click/drag and the collapsed-rail flyout work exactly like the built-in nav)
// and the pure internal/pages logic for ordering and reorder. Rename/delete/hide
// management lands in a follow-up; this is the create + navigate + reorder slice.
func CustomPagesNav() uic.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	nav := router.UseNavigate()
	current := router.InspectCurrentRoute().Path
	all := app.CustomPages()
	visible := pages.Visible(all)

	// Drag source is the slug being dragged; held at this component so it survives
	// across the two row components (drag start on one, drop on another).
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
		// Find the target's index in display order and move the dragged page there.
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
	}

	create := func() {
		name := promptName(uistate.T("pages.newPrompt"), uistate.T("pages.newDefault"))
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
		nav.Navigate("/p/" + p.Slug)
	}

	rows := make([]uic.Node, 0, len(visible)+1)
	for _, p := range visible {
		slug := p.Slug
		path := "/p/" + slug
		rows = append(rows, uic.CreateElement(navItem, navItemProps{
			Label:       p.Name,
			Path:        path,
			Icon:        icon.Page,
			Active:      current == path,
			Draggable:   true,
			OnDragStart: func() { dragSrc.Set(slug) },
			OnDrop:      func() { reorder(slug) },
		}))
	}
	// "New page" is always available, even when there are no pages yet. It's a
	// non-navigating action, so it's a plain styled row (navItem only navigates)
	// matching the muted nav styling — the .nv class also gets the collapsed flyout.
	newPage := A(
		Class("nav nv flex items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer text-faint"),
		Title(uistate.T("rail.newPage")),
		OnClick(create),
		ui.Icon(icon.Plus, Class("w-4 h-4 shrink-0")),
		Span(uistate.T("rail.newPage")),
	)

	// A single root Div (not a bare Fragment) so this component renders inline at
	// its position among the other rail sections; a Fragment of mixed children
	// doesn't preserve sibling order next to the MapKeyed groups.
	return Div(Class("flex flex-col gap-0.5"),
		railHeader(uistate.T("rail.myPages")),
		rows, // []ui.Node — flattened into children by the framework
		newPage,
	)
}
