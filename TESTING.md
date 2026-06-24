# Testing CashFlux

CashFlux is a Go→WASM app. The codebase splits cleanly into two test surfaces:

- **Logic packages** — pure Go with no build tag (`internal/budgeting`, `ledger`,
  `appstate`, `store`, `currency`, `reports`, `theme`, `customfields`, …). These
  run on the host architecture and hold the bulk of the test suite.
- **UI packages** — gated with `//go:build js && wasm` (`internal/screens`,
  `internal/app`, `internal/ui`). Their files only compile for `GOOS=js GOARCH=wasm`,
  so the host toolchain skips them automatically.

## Native test command

```bash
go test ./...
```

Run on the **host** (no `GOOS=js`), this compiles and tests only the non-wasm
packages — the `js && wasm` build constraint excludes the UI packages with no
flags or package list needed. CI does exactly this (`go test ./...` in `ci.yml`).
The cross-cutting English-catalog guard (`TestDefaultCatalogQuality`) and the
screen-lint ratchet (`internal/screenlint`, which reads screen files as text, not
as a wasm build) both run here too.

## Build the WASM bundle

```bash
GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .
```

This is the compile-check for the UI packages (they have no native tests). Keep it
green; a failing wasm build is the UI equivalent of a failing test.

## End-to-end (browser)

E2E lives under `e2e/` (Playwright via the repo's `.tools`), served by
`go run e2e/serve.go` (static `web/` with SPA history fallback + correct MIME).
These require a browser and are run deliberately, not in the default `go test` lane.

## One-liner

| Goal | Command |
|------|---------|
| Logic tests (native, fast) | `go test ./...` |
| UI compile-check | `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` |
| Race detector on logic | `go test -race ./...` |
