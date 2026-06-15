//go:build js && wasm

// Command cashflux is the WebAssembly entrypoint for the CashFlux app.
//
// This is a Phase 0 smoke-test shell that verifies the GoWebComponents
// dependency resolves and renders. It is intentionally minimal and will be
// replaced by the Phase 1 application shell (router + screens) per SPEC.md.
package main

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
	"github.com/monstercameron/GoWebComponents/utils"
)

// App is the placeholder root component.
func App() ui.Node {
	return Main(Class("app"),
		H1(Class("title"), "CashFlux"),
		P(Class("subtitle"), "Toolchain and framework wiring verified — Phase 1 starts here."),
	)
}

func main() {
	utils.DisableAllDebug()
	ui.Render(ui.CreateElement(App), "#app")
	utils.WaitForever()
}
