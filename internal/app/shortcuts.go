//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/router"
)

// wireKeyboardShortcuts installs global keyboard shortcuts. Alt+1..9 jumps to the
// Nth primary navigation screen (Dashboard, Accounts, …) so the keyboard alone can
// move between sections. Registered once at boot; the listener lives for the app's
// lifetime, so its js.Func is intentionally never released.
//
// It keys off KeyboardEvent.code ("Digit1".."Digit9") so it's keyboard-layout
// independent and never matches the numpad (where Alt+number is an OS alt-code),
// and it stays out of the way while the user is typing in a field.
func wireKeyboardShortcuts() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	nav := primaryNav() // static — the screen set doesn't change at runtime

	onKeyDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		key := e.Get("key").String()
		// Esc dismisses the help/command overlays (no-op when closed); FlipPanel
		// keeps handling Esc for open settings panels independently.
		if key == "Escape" {
			closeHelpOverlay()
			closeCommandPalette()
			return nil
		}
		// Cmd/Ctrl+K toggles the command palette (works even from a field).
		if (e.Get("metaKey").Bool() || e.Get("ctrlKey").Bool()) && !e.Get("altKey").Bool() && e.Get("code").String() == "KeyK" {
			e.Call("preventDefault")
			toggleCommandPalette()
			return nil
		}
		if isEditableTarget(doc) {
			return nil
		}
		// "?" toggles the keyboard cheat sheet.
		if key == "?" {
			e.Call("preventDefault")
			toggleHelpOverlay()
			return nil
		}
		if !e.Get("altKey").Bool() || e.Get("ctrlKey").Bool() || e.Get("metaKey").Bool() {
			return nil
		}
		code := e.Get("code").String()
		// Alt+N opens the quick-add transaction panel.
		if code == "KeyN" {
			e.Call("preventDefault")
			uistate.UseQuickAdd().Set(true)
			return nil
		}
		if len(code) != 6 || code[:5] != "Digit" {
			return nil
		}
		d := code[5]
		if d < '1' || d > '9' {
			return nil
		}
		idx := int(d - '1')
		if idx >= len(nav) {
			return nil
		}
		e.Call("preventDefault")
		router.Navigate(nav[idx].Path)
		return nil
	})
	doc.Call("addEventListener", "keydown", onKeyDown)
}

// isEditableTarget reports whether focus is in a text input, so a shortcut chord
// doesn't fire (and steal the keystroke) while the user is typing.
func isEditableTarget(doc js.Value) bool {
	el := doc.Get("activeElement")
	if el.IsNull() || el.IsUndefined() {
		return false
	}
	if tag := el.Get("tagName"); !tag.IsNull() && !tag.IsUndefined() {
		switch tag.String() {
		case "INPUT", "TEXTAREA", "SELECT":
			return true
		}
	}
	if ce := el.Get("isContentEditable"); !ce.IsNull() && !ce.IsUndefined() && ce.Bool() {
		return true
	}
	return false
}

const helpOverlayID = "cf-help-overlay"

// helpHTML is the shortcuts cheat sheet body. (English for now — a follow-up can
// route these through the i18n catalog like the rest of the UI.)
const helpHTML = `<div style="display:flex;justify-content:space-between;align-items:center;gap:1rem;margin-bottom:0.8rem;">
  <strong style="font-size:1rem;">Keyboard shortcuts</strong>
  <button id="cf-help-close" type="button" aria-label="Close" style="background:transparent;border:0;color:inherit;cursor:pointer;font-size:1.15rem;line-height:1;min-width:24px;min-height:24px;">&times;</button>
</div>
<table style="width:100%;border-collapse:collapse;">
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Jump to a section</td><td style="text-align:right;white-space:nowrap;">Alt + 1&ndash;9</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Add a transaction</td><td style="text-align:right;white-space:nowrap;">Alt + N</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Command palette</td><td style="text-align:right;white-space:nowrap;">Ctrl/&#8984; + K</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Save the open settings panel</td><td style="text-align:right;white-space:nowrap;">Enter</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Close a panel / this help</td><td style="text-align:right;white-space:nowrap;">Esc</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Show dashboard resize handles</td><td style="text-align:right;white-space:nowrap;">Hold Shift</td></tr>
  <tr><td style="padding:0.28rem 0;opacity:0.85;">Toggle this help</td><td style="text-align:right;white-space:nowrap;">?</td></tr>
</table>`

// toggleHelpOverlay shows or hides the keyboard cheat sheet, building it on first
// use. It's a self-contained DOM overlay (not a framework component), so the
// shortcut layer owns it end to end and nothing else has to mount it.
func toggleHelpOverlay() {
	doc := js.Global().Get("document")
	ov := doc.Call("getElementById", helpOverlayID)
	if ov.IsNull() || ov.IsUndefined() {
		buildHelpOverlay(doc)
		return
	}
	style := ov.Get("style")
	if style.Get("display").String() == "none" {
		style.Set("display", "grid")
	} else {
		style.Set("display", "none")
	}
}

// closeHelpOverlay hides the cheat sheet if it's open (a no-op otherwise).
func closeHelpOverlay() {
	doc := js.Global().Get("document")
	if ov := doc.Call("getElementById", helpOverlayID); !ov.IsNull() && !ov.IsUndefined() {
		ov.Get("style").Set("display", "none")
	}
}

// buildHelpOverlay creates the overlay once and appends it to <body>, visible.
// Subsequent opens just toggle its display. The click/close js.Funcs live for the
// app's lifetime (intentionally not released), matching the persistent overlay.
func buildHelpOverlay(doc js.Value) {
	ov := doc.Call("createElement", "div")
	ov.Set("id", helpOverlayID)
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:200;display:grid;place-items:center;background:rgba(0,0,0,0.55);")

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "background:var(--bg-elev,#1a1a1d);color:var(--text,#f4f4f5);border:1px solid var(--border,#2a2a2c);border-radius:10px;padding:1.1rem 1.35rem;max-width:min(92vw,440px);box-shadow:0 12px 40px rgba(0,0,0,0.5);font-size:0.9rem;line-height:1.5;")
	card.Set("innerHTML", helpHTML)
	ov.Call("appendChild", card)

	// Click the dimmed backdrop (not the card) to dismiss.
	backdropCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("target").Equal(ov) {
			ov.Get("style").Set("display", "none")
		}
		return nil
	})
	ov.Call("addEventListener", "click", backdropCb)

	doc.Get("body").Call("appendChild", ov)

	// Wire the ✕ button inside the card.
	if x := doc.Call("getElementById", "cf-help-close"); !x.IsNull() && !x.IsUndefined() {
		closeCb := js.FuncOf(func(js.Value, []js.Value) any {
			ov.Get("style").Set("display", "none")
			return nil
		})
		x.Call("addEventListener", "click", closeCb)
	}
}

// ---- Command palette (Cmd/Ctrl+K) ----------------------------------------

const (
	cmdPaletteID = "cf-cmd-palette"
	cmdInputID   = "cf-cmd-input"
	cmdListID    = "cf-cmd-list"
)

// paletteCmd is one searchable command: a label and the action to run.
type paletteCmd struct {
	label string
	run   func()
}

var (
	cmdPaletteCmds  []paletteCmd // built once, on first open
	cmdPaletteShown []int        // command indices currently displayed (filtered)
	cmdPaletteSel   int          // selection within cmdPaletteShown
)

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// buildPaletteCommands enumerates the searchable commands: jump to any screen
// (primary, tools, system groups) plus a couple of direct actions.
func buildPaletteCommands() []paletteCmd {
	var cmds []paletteCmd
	add := func(items []railItem) {
		for _, it := range items {
			path := it.Path
			cmds = append(cmds, paletteCmd{label: uistate.T(it.Key), run: func() { router.Navigate(path) }})
		}
	}
	add(primaryNav())
	add(toolsNav())
	add(systemNav())
	cmds = append(cmds,
		paletteCmd{label: uistate.T("addmenu.transaction"), run: func() { uistate.UseQuickAdd().Set(true) }},
		paletteCmd{label: "Keyboard shortcuts", run: toggleHelpOverlay},
	)
	return cmds
}

// toggleCommandPalette shows or hides the command palette, building it on first
// use. Like the help overlay it's a self-contained DOM surface owned by the
// shortcut layer.
func toggleCommandPalette() {
	doc := js.Global().Get("document")
	ov := doc.Call("getElementById", cmdPaletteID)
	if ov.IsNull() || ov.IsUndefined() {
		buildCommandPalette(doc)
		return
	}
	if ov.Get("style").Get("display").String() == "none" {
		openCommandPalette(doc, ov)
	} else {
		ov.Get("style").Set("display", "none")
	}
}

// closeCommandPalette hides the palette if open (a no-op otherwise).
func closeCommandPalette() {
	doc := js.Global().Get("document")
	if ov := doc.Call("getElementById", cmdPaletteID); !ov.IsNull() && !ov.IsUndefined() {
		ov.Get("style").Set("display", "none")
	}
}

// openCommandPalette reveals the palette, clears the query, focuses the input,
// and renders the full command list.
func openCommandPalette(doc, ov js.Value) {
	ov.Get("style").Set("display", "grid")
	if inp := doc.Call("getElementById", cmdInputID); !inp.IsNull() && !inp.IsUndefined() {
		inp.Set("value", "")
		inp.Call("focus")
	}
	renderPalette(doc, "")
}

func buildCommandPalette(doc js.Value) {
	cmdPaletteCmds = buildPaletteCommands()

	ov := doc.Call("createElement", "div")
	ov.Set("id", cmdPaletteID)
	ov.Get("style").Set("cssText", "position:fixed;inset:0;z-index:210;display:grid;place-items:start center;padding-top:12vh;background:rgba(0,0,0,0.5);")

	card := doc.Call("createElement", "div")
	card.Get("style").Set("cssText", "width:min(92vw,520px);background:var(--bg-elev,#1a1a1d);color:var(--text,#f4f4f5);border:1px solid var(--border,#2a2a2c);border-radius:10px;box-shadow:0 12px 40px rgba(0,0,0,0.5);overflow:hidden;")

	inp := doc.Call("createElement", "input")
	inp.Set("id", cmdInputID)
	inp.Set("type", "text")
	inp.Call("setAttribute", "placeholder", "Search commands…")
	inp.Call("setAttribute", "aria-label", "Search commands")
	inp.Get("style").Set("cssText", "width:100%;box-sizing:border-box;padding:0.8rem 1rem;background:transparent;border:0;border-bottom:1px solid var(--border,#2a2a2c);color:inherit;font:inherit;font-size:1rem;outline:none;")
	card.Call("appendChild", inp)

	list := doc.Call("createElement", "div")
	list.Set("id", cmdListID)
	list.Get("style").Set("cssText", "max-height:50vh;overflow-y:auto;padding:0.35rem;")
	card.Call("appendChild", list)

	ov.Call("appendChild", card)

	// Filter as you type.
	inputCb := js.FuncOf(func(js.Value, []js.Value) any {
		renderPalette(doc, strings.ToLower(strings.TrimSpace(inp.Get("value").String())))
		return nil
	})
	inp.Call("addEventListener", "input", inputCb)

	// Arrow/Enter/Esc navigation within the input.
	navCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) == 0 {
			return nil
		}
		e := a[0]
		switch e.Get("key").String() {
		case "ArrowDown":
			e.Call("preventDefault")
			movePaletteSel(doc, 1)
		case "ArrowUp":
			e.Call("preventDefault")
			movePaletteSel(doc, -1)
		case "Enter":
			e.Call("preventDefault")
			runPaletteSel()
		case "Escape":
			e.Call("preventDefault")
			closeCommandPalette()
		}
		return nil
	})
	inp.Call("addEventListener", "keydown", navCb)

	// Backdrop click dismisses; a click on a row runs that command (delegated, so
	// the dynamic rows need no per-row listeners).
	clickCb := js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) == 0 {
			return nil
		}
		t := a[0].Get("target")
		if t.Equal(ov) {
			ov.Get("style").Set("display", "none")
			return nil
		}
		row := t.Call("closest", "[data-cmd-row]")
		if !row.IsNull() && !row.IsUndefined() {
			if attr := row.Call("getAttribute", "data-cmd-row"); !attr.IsNull() && !attr.IsUndefined() {
				if ci, err := strconv.Atoi(attr.String()); err == nil {
					runPaletteCmd(ci)
				}
			}
		}
		return nil
	})
	ov.Call("addEventListener", "click", clickCb)

	doc.Get("body").Call("appendChild", ov)
	openCommandPalette(doc, ov)
}

// renderPalette filters the commands by query and rebuilds the result rows.
func renderPalette(doc js.Value, query string) {
	list := doc.Call("getElementById", cmdListID)
	if list.IsNull() || list.IsUndefined() {
		return
	}
	cmdPaletteShown = cmdPaletteShown[:0]
	for i, c := range cmdPaletteCmds {
		if query == "" || strings.Contains(strings.ToLower(c.label), query) {
			cmdPaletteShown = append(cmdPaletteShown, i)
		}
	}
	cmdPaletteSel = 0

	var b strings.Builder
	for pos, ci := range cmdPaletteShown {
		bg := "transparent"
		if pos == cmdPaletteSel {
			bg = "var(--hover,#1c1c1e)"
		}
		b.WriteString(`<div data-cmd-row="`)
		b.WriteString(strconv.Itoa(ci))
		b.WriteString(`" role="option" style="padding:0.5rem 0.7rem;border-radius:6px;cursor:pointer;background:`)
		b.WriteString(bg)
		b.WriteString(`;">`)
		b.WriteString(htmlEscaper.Replace(cmdPaletteCmds[ci].label))
		b.WriteString(`</div>`)
	}
	if len(cmdPaletteShown) == 0 {
		b.WriteString(`<div style="padding:0.6rem 0.7rem;opacity:0.6;">No matching commands</div>`)
	}
	list.Set("innerHTML", b.String())
}

// movePaletteSel moves the highlighted row, wrapping at the ends.
func movePaletteSel(doc js.Value, delta int) {
	n := len(cmdPaletteShown)
	if n == 0 {
		return
	}
	cmdPaletteSel = (cmdPaletteSel + delta + n) % n
	list := doc.Call("getElementById", cmdListID)
	if list.IsNull() || list.IsUndefined() {
		return
	}
	children := list.Get("children")
	for i := 0; i < children.Get("length").Int(); i++ {
		row := children.Index(i)
		if row.Call("getAttribute", "data-cmd-row").IsNull() {
			continue
		}
		if i == cmdPaletteSel {
			row.Get("style").Set("background", "var(--hover,#1c1c1e)")
			row.Call("scrollIntoView", map[string]any{"block": "nearest"})
		} else {
			row.Get("style").Set("background", "transparent")
		}
	}
}

// runPaletteSel runs the highlighted command.
func runPaletteSel() {
	if cmdPaletteSel < 0 || cmdPaletteSel >= len(cmdPaletteShown) {
		return
	}
	runPaletteCmd(cmdPaletteShown[cmdPaletteSel])
}

// runPaletteCmd closes the palette and runs command ci.
func runPaletteCmd(ci int) {
	if ci < 0 || ci >= len(cmdPaletteCmds) {
		return
	}
	closeCommandPalette()
	if r := cmdPaletteCmds[ci].run; r != nil {
		r()
	}
}
