# Testing guide

CashFlux has two distinct test layers: **native Go unit tests** (pure logic, runs anywhere) and
**browser/e2e gates** (require the running wasm app on `:8099`).

---

## 1. Native logic tests (`go test ./...`)

All `internal/` packages that contain only pure Go (no `syscall/js`) are unit-tested with the
standard Go toolchain and run without a browser or wasm build.

```powershell
go test ./...
```

This covers every logic package: `money`, `currency`, `ledger`, `budgeting`, `goals`, `freshness`,
`validate`, `dateutil`, `id`, `formula`, `rules`, `allocate`, `forecast`, `icon`, `store`, and
many others. All of these currently pass on native Go.

**Excluded packages** — packages that import `syscall/js` (or transitively depend on it) cannot
be built on native Go. They carry a `//go:build js && wasm` build constraint and are silently
skipped by `go test ./...` on a non-wasm host. This is expected and correct. The wasm/browser
lane (see §2) covers those.

Run `go vet ./internal/...` for additional static checks on the pure packages.

---

## 2. E2E browser gates (`e2e/*_check.mjs`)

These scripts drive the live running app via Playwright and check real DOM behaviour, visual
layout, and interactive flows. They require:

1. **The app running locally on port 8099.** Start it with:
   ```powershell
   .\.tools\gwc.exe dev -app .\main.go -root .
   ```
   The server listens at `http://localhost:8099` by default.

2. **Node.js + Playwright** installed and reachable on `PATH`.

3. Run a single gate:
   ```bash
   node e2e/<name>_check.mjs
   ```
   For example:
   ```bash
   node e2e/a11y_check.mjs
   node e2e/accounts_labels_check.mjs
   ```

Gate scripts live in `e2e/`. Files prefixed `_debug_` are scratch scripts, not gates.
Screenshot artifacts land in `e2e/` and `e2e/.artifacts/`.

---

## 3. Pre-existing vs. real failures

| Failure type | Meaning |
|---|---|
| A native `go test` package fails | A real regression — fix before committing. |
| A `_check.mjs` gate fails on a screen or feature listed in `TODOS.md` as not-yet-built | Pre-existing / known gap; log it in `DEVLOG.md` but do not block the commit. |
| A `_check.mjs` gate fails on a feature that WAS working | Real regression — investigate before merging. |

The CI workflow (`.github/workflows/`) runs the wasm build; the native `go test ./...` lane is a
fast local sanity check that every contributor can run without a browser.

---

## 4. Quick reference

| Task | Command |
|---|---|
| All native logic tests | `go test ./...` |
| Single package | `go test ./internal/icon/...` |
| Static checks (pure packages) | `go vet ./internal/...` |
| Format check | `gofmt -l ./internal/` |
| Start dev server (wasm, port 8099) | `.\.tools\gwc.exe dev -app .\main.go -root .` |
| Run one e2e gate | `node e2e/<name>_check.mjs` |
