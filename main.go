//go:build js && wasm

// Command cashflux is the WebAssembly entrypoint for the CashFlux app.
// It delegates to internal/app, which owns routing and the app shell.
package main

import "github.com/monstercameron/CashFlux/internal/app"

func main() {
	app.Run()
}
