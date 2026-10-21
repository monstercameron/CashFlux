# GoWebComponents — Framework Notes (for CashFlux)

Working notes on the GoWebComponents framework as consumed by CashFlux. Distilled from the
framework README, its `examples/`, and source. Keep this current as we learn more.

- **What it is:** a Go + WebAssembly UI framework with a React-style component/hook model, a
  fiber-based runtime, typed HTML builders, a shorthand authoring layer, client-side routing,
  shared state, fetch helpers, SSR/hydration, i18n, a11y, PWA, feature flags, and devtools — all
  in one Go module. The whole UI is Go compiled to wasm; no JS build/bundler.
- **Module:** `github.com/monstercameron/GoWebComponents/v7`, pinned to `v7.0.1`.
- **Requires:** Go 1.27.0+, with the CashFlux toolchain pinned to Go 1.27.3. Target `GOOS=js GOARCH=wasm`.
- **Trade-offs:** not faster than hand-tuned React; wasm bundle is large raw (~6 MB for a counter)
  but compresses ~5x under brotli. Value = whole UI in Go, shared types client/server.

## Dependency wiring (important)

- Consume it as a **normal versioned module**; **do not** use a local `replace` in CashFlux.
- In v7 the CLI is a separate nested module under the framework's `tools/` directory.
  Build the local runner from that module without `-trimpath`; the runner locates its source
  tree with `runtime.Caller`. Keep that source tree after building the runner.
- The Windows v7 release runner has a trimmed source path and cannot locate its framework
  root here. See [the upgrade record](DEPENDENCY_UPGRADE_2026-10-21.md) for the verified source setup.
- The v7 `gwc build` command forces `GOEXPERIMENT=nojsonv2`, which conflicts with CashFlux's
  SQLite dependency on Go 1.27. Use the existing direct Go build scripts for application builds.
  `gwc dev` does not force this setting and remains the live development entry point.
- Markdown is opt-in in v7. `internal/screens/custompage.go` imports `html/markdown` so custom
  text widgets retain their headings, emphasis, lists, and links.

## Public packages we use

| Package | Use |
|---|---|
| `ui` | components, hooks (`UseState`, `UseEvent`, `UseEffect`, …), `CreateElement`, `Render`, events |
| `html` | typed element builders (`html.Div(html.Props{…}, …)`) + the `Props` struct + `PropOption`s |
| `html/shorthand` | dot-imported mixed-arg sugar (`Div(Class("x"), …)`) + control-flow funcs |
| `state` | app-wide atoms (`UseAtom`, `UseComputed`, snapshots) |
| `router` | history/hash routing, navigation, route inspection |
| `fetch` | `UseFetch`, `UseResource[T]`, `UseWebSocket`/`UseEventSource`, imperative `Fetch` |
| `interop` | browser bridges incl. storage, clipboard, observers (used for non-DB browser glue) |
| `utils` | `WaitForever()`, `DisableAllDebug()` |

`ui.Node` is `*runtime.Element`; `router.Element` is an alias of the same — so `ui.CreateElement(...)`
returns exactly what router handlers (`func(router.Attrs) *router.Element`) need.

## Authoring with `html/shorthand` (our default)

Dot-import it: `import . "github.com/monstercameron/GoWebComponents/v7/html/shorthand"`.

- **Elements** are `func(...any) ui.Node`: `Div`, `Span`, `Button`, `Input`, `Select`, `Option`,
  `Form`, `Nav`, `Header`, `Main`, `Section`, `H1`–`H6`, `P`, `Ul`, `Li`, `Table`, … (full set in
  `html/html.go`). Mixed args: prop-options + children in any order.
- **Prop options:** `Class`, `ID`, `Type`, `Name`, `Value`, `Placeholder`, `Min`/`Max`/`Step`,
  `Title`, `For`, `Href`, `Disabled(…bool)`, `Selected(…bool)`, `SelectedIf(bool)`, `Attr(k,v)`,
  and events `OnClick`/`OnInput`/`OnChange`/`OnSubmit`/…. `OnInput` and `OnChange` accept typed
  callbacks or `ui.Handler`; input-handler props must retain those types instead of `any`.
- **Children** auto-normalize: a `string` becomes text; a `[]ui.Node` (e.g. from `Map`/`MapKeyed`)
  is **flattened** — so `Div(Class("rows"), MapKeyed(items, key, render))` just works.
- **Control flow:** `If(cond, node)`, `IfElse(cond, a, b)`, `Unless`, `Show`, `Switch`/`Case`/
  `Default`, `Map`, `MapKeyed(items, keyFn, render)`, `MapIndexed`, `FilterMap`, `Range`, `Repeat`,
  `Fragment`, `Text`, `Textf`, `When`/`ClassNames`/`ClassMap` (class helpers).

The typed `html` package (`html.Div(html.Props{…}, …)`) is the stable equivalent; shorthand wraps it.

## Hooks & state

- `ui.UseState(initial)` → `.Get()`, `.Set(v)`, `.Update(func(prev) next)`.
- `ui.UseEvent(fn)` → an event `Handler`. `fn` may be `func()`, `func(string)` (input value), or
  `func(ui.Event)` (use `.GetValue()`). Also `ui.UsePrevious`, `UseEffect`, `UseReducer`, `UseRef`,
  `UseId`, `UseDeferredValue`, `UseTransition`, plus async: `UseTask`, `UseChannel`, `UseDebounced`,
  `UseThrottled`, `AsyncBoundary`, `UseForm`.
- `state.UseAtom(key, default)` → shared, cross-component state (`.Get`/`.Set`/`.Update`).
- `state.UseComputed(fn, deps…)` → derived value recomputed when deps change.
- Mount: `ui.Render(ui.CreateElement(App), "#app")` then `utils.WaitForever()`.

## Routing

```go
r := router.NewHistoryRouter(router.RouterOptions{DefaultRoute: "/"})
r.Register("/", func(router.Attrs) *router.Element { return ui.CreateElement(Dashboard) })
r.Register("*", func(router.Attrs) *router.Element { return ui.CreateElement(NotFound) })
r.Mount("#app")
```
Inside components: `nav := router.UseNavigate(); nav.Navigate("/path")` and
`router.InspectCurrentRoute().Path`. `router.Attrs` is `map[string]interface{}`. There is also a
hash router and hydration-aware mounts (`HydrateMount`).

## CRITICAL gotchas

1. **`On*` prop options register hooks on the wasm build.** They must only run at **stable render
   positions** — never inside a variable-length loop. For per-row interactive elements (delete
   buttons, etc.), make the row its **own component** (`ui.CreateElement(Row, props)`) and pass
   plain `func` callbacks down as props; the row owns its handler hook. (Confirmed by the comment
   in `html/shorthand/shorthand.go` and the `todo-advanced` example.)
2. **Build tags:** app files are `//go:build js && wasm`. Pure logic packages should have **no**
   build tags and **no `syscall/js`** so they unit-test on native Go.
3. **`go mod tidy` must run with `GOOS=js GOARCH=wasm`** so wasm-tagged files are seen and their
   imports resolved (otherwise tidy may drop the framework requirement).
4. Keep generated wasm out of git.

## Build / run / test workflow

Use the `gwc` runner (we keep a built copy at `.tools/gwc.exe`; also wired as the `gwc` MCP server).

```powershell
.\.tools\gwc.exe doctor                              # verify toolchain + assets
.\.tools\gwc.exe dev -app .\main.go -root .          # build wasm, serve, live-reload
.\.tools\gwc.exe build -app .\main.go -profile development
.\.tools\gwc.exe release -app .\main.go -out-dir .\bin\release   # compressed (br/gzip)
.\.tools\gwc.exe test -lane unit -lane wasm          # validation lanes
.\.tools\gwc.exe wasm measure -package .             # raw + compressed size
```
Direct build also works: `$env:GOOS="js"; $env:GOARCH="wasm"; go build -o static\bin\main.wasm .`

### CashFlux live development on port 8080

Use `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1` for
CashFlux development. It builds both `main.wasm` and `services.wasm`, watches
source changes, and serves the app at `http://127.0.0.1:8080`. CashFlux requires a
full-page reload after rebuilding: swapping the running WASM runtime can leave
multiple autosave loops writing the same workspace. The script requests
`-hot=false` for this reason.

The local runner repaired on 2026-10-20 was built from the ignored
`.tools/gwc-framework-v5` worktree at framework tag `v5.0.1`, matching `go.mod`.
The previous executable resolved its development sources from a separate checkout
that had moved to an incompatible framework version. Keep the runner's source
checkout available; rebuilding from an unrelated moving checkout can reintroduce
that failure. These local tool repairs do not change the app's versioned module.

That ignored tool installation also carries local compatibility fixes: it forwards
the boolean flag as `-hot=false` rather than separate arguments, classifies that
mode as a full reload for every rebuild, and serves the SPA entry for missing
extensionless GET/HEAD routes such as `/transactions`. Missing asset paths still
return 404. Replacing the ignored runner/source checkout requires preserving or
upstreaming those fixes; they are not part of the downloaded framework module.

The same local tool source serializes WebSocket data-frame writes across status
delivery and reload broadcasts. Concurrent broadcasts previously terminated the
server with `concurrent write to websocket connection`. A concurrent large-frame
broadcast regression passed repeated native runs; the race detector was unavailable
because this toolchain requires cgo for it. This repair is in the ignored source
checkout, so replacing that checkout also requires preserving the write lock.

The tracked `scripts/devwatch.ps1` helper prunes generated/tool directories before
walking service sources, skips reparse-point directories and observes module-file
timestamps. Run `scripts/devwatch.Tests.ps1` for the deterministic fixture;
`-Probe` adds an optional local wall-time comparison with the previous scanner.

Verify `/__gwc/status` reports `lastBuild.success: true` and
`lastBuild.reloadType: "full"`, and verify `/transactions` returns 200. A source
timestamp change should produce a new successful build with that file listed in
`lastClassification.changedFiles`. This checks the watcher, not just a previously
built artifact. Port 8142/8143 test servers are not required for this workflow.

Host page needs `wasm_exec.js` (from `GOROOT/lib/wasm/wasm_exec.js`; gwc copies it) and a boot
script that instantiates `./bin/main.wasm` into a `#app` div (see framework README "Host HTML").

`gwc start` scaffolds new apps but **requires an interactive TTY** — not usable from this agent, so
we wire modules/host files by hand instead.

## gwc MCP server

`gwc mcp` is a JSON-RPC-over-stdio MCP server (protocol `2024-11-05`, server name `gwc`). It exposes
**~81** commands as `gwc_*` tools, each taking `{ args: []string, json?: bool }`. Categories:

- **Build/dev:** `gwc_doctor`, `gwc_dev`, `gwc_build`, `gwc_release`, `gwc_test`, `gwc_verify`,
  `gwc_bench`, `gwc_clean`, `gwc_fmt`, `gwc_deadcode`, `gwc_deps`, `gwc_env`, `gwc_docs`,
  `gwc_examples`, `gwc_wasm`.
- **Live browser driving / inspection:** `gwc_browser`, `gwc_dom`, `gwc_eval`, `gwc_click`,
  `gwc_drag`, `gwc_hover`, `gwc_console`, `gwc_expect`, `gwc_describe`, `gwc_screenshot`,
  `gwc_a11y`, `gwc_audit`, `gwc_crash_report`, `gwc_emit`, `gwc_delete_atom`.

Run `.\.tools\gwc.exe help -json` for the full command list and flags. Prefer these tools for
build/test/run and for verifying UI behavior in a real browser during development.
