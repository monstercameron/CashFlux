// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// The top bar can wrap at narrower desktop widths. Sticky content below it
// needs its rendered height rather than the single-row minimum from CSS.
func useTopbarHeight() {
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		bar := doc.Call("querySelector", "main.cf-scroll > .topbar")
		shell := doc.Call("querySelector", ".cf-shell")
		if !bar.Truthy() || !shell.Truthy() {
			return nil
		}
		setHeight := func(height float64) {
			shell.Get("style").Call("setProperty", "--topbar-h", strconv.FormatFloat(height, 'f', -1, 64)+"px")
		}
		update := func() {
			// offsetHeight is layout size and stays stable through the shell's
			// entrance transform; getBoundingClientRect briefly scales with it.
			setHeight(float64(bar.Get("offsetHeight").Int()))
		}
		update()
		resizeObserver := js.Global().Get("ResizeObserver")
		if !resizeObserver.Truthy() {
			return nil
		}
		callback := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Length() > 0 {
				box := args[0].Index(0).Get("borderBoxSize")
				if box.Truthy() && box.Length() > 0 {
					// ResizeObserver reports the fractional layout height without
					// entrance-transform scaling or offsetHeight rounding.
					setHeight(box.Index(0).Get("blockSize").Float())
					return nil
				}
			}
			update()
			return nil
		})
		observer := resizeObserver.New(callback)
		observer.Call("observe", bar, js.ValueOf(map[string]any{"box": "border-box"}))
		return func() {
			observer.Call("disconnect")
			callback.Release()
		}
	}, "once")
}
