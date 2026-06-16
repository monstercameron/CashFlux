//go:build js && wasm

package app

import "syscall/js"

// wireResizeReveal makes the dashboard widgets' resize handles appear only while
// the Shift key is held, keeping the bento grid visually calm the rest of the
// time. It toggles a data-resize attribute on the document root that the CSS
// keys off (.rz is hidden unless [data-resize] is present). Window blur clears
// it so the handles never get stuck visible if focus is lost mid-hold.
//
// Registered once at boot; the listeners live for the app's lifetime, so their
// js.Func callbacks are intentionally never released.
func wireResizeReveal() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	root := doc.Get("documentElement")

	set := func(on bool) {
		if on {
			root.Call("setAttribute", "data-resize", "on")
		} else {
			root.Call("removeAttribute", "data-resize")
		}
	}

	onKeyDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Shift" {
			set(true)
		}
		return nil
	})
	onKeyUp := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Shift" {
			set(false)
		}
		return nil
	})
	onBlur := js.FuncOf(func(js.Value, []js.Value) any {
		set(false)
		return nil
	})

	doc.Call("addEventListener", "keydown", onKeyDown)
	doc.Call("addEventListener", "keyup", onKeyUp)
	js.Global().Call("addEventListener", "blur", onBlur)
}
