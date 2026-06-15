# GoWebComponents — Framework Notes (for CashFlux)

Working notes on the GoWebComponents framework as consumed by CashFlux. Distilled from the
framework README, its `examples/`, and source. Keep this current as we learn more.

- **What it is:** a Go + WebAssembly UI framework with a React-style component/hook model, a
  fiber-based runtime, typed HTML builders, a shorthand authoring layer, client-side routing,
  shared state, fetch helpers, SSR/hydration, i18n, a11y, PWA, feature flags, and devtools — all
  in one Go module. The whole UI is Go compiled to wasm; no JS build/bundler.
- **Module:** `github.com/monstercameron/GoWebComponents` (we pin a pseudo-version via `go get`).
- **Requires:** Go 1.26+. Target `GOOS=js GOARCH=wasm`.
- **Trade-offs:** not faster than hand-tuned React; wasm bundle is large raw (~6 MB for a counter)
  but compresses ~5x under brotli. Value = whole UI in Go, shared types client/server.

## Dependency wiring (important)

- Consume it as a **normal module** (`go get …@<commit>`); **do not** use a local `replace`.
- The framework's *own* `go.mod` has `replace agenthub => ./tools/agenthub` and a GoGRPCBridge
  replace. Replace directives are **ignored when the module is a dependency**, but the core public
  packages (`ui`, `html`, `html/shorthand`, `state`, `router`, `utils`) **do not import** agenthub,
  so Go's module-graph pruning drops it — `go mod tidy` resolves cleanly (only `cbor`, `float16`,
  `goldmark` come along as indirect deps). The agenthub bits are only used by the `gwc` tool, which
  we build from the checkout instead of `go install`.

## Public packages we use

| Package | Use |
|---|---|
| `ui` | components, hooks (`UseState`, `UseEvent`, `UseEffect`, …), `CreateElement`, `Render`, events |
| `html` | typed element builders (`html.Div(html.Props{…}, …)`) + the `Props` struct + `PropOption`s |
| `html/shorthand` | dot-imported mixed-arg sugar (`Div(Class("x"), …)`) + control-flow funcs |
| `state` | app-wide atoms (`UseAtom`, `UseComputed`, snapshots) |
| `router` | history/hash routing, navigation, route inspection |
| `fetch` | `UseFetch`, `UseResource[T]`, `UseWebSocket`/`UseEventSource`, imperative `Fetch` |
| `interop` | browser bridges incl. storage (our IndexedDB layer will build on this) |
| `utils` | `WaitForever()`, `DisableAllDebug()` |

`ui.Node` is `*runtime.Element`; `router.Element` is an alias of the same — so `ui.CreateElement(...)`
returns exactly what router handlers (`func(router.Attrs) *router.Element`) need.

## Authoring with `html/shorthand` (our default)

Dot-import it: `import . "github.com/monstercameron/GoWebComponents/html/shorthand"`.

- **Elements** are `func(...any) ui.Node`: `Div`, `Span`, `Button`, `Input`, `Select`, `Option`,
  `Form`, `Nav`, `Header`, `Main`, `Section`, `H1`–`H6`, `P`, `Ul`, `Li`, `Table`, … (full set in
  `html/html.go`). Mixed args: prop-options + children in any order.
- **Prop options:** `Class`, `ID`, `Type`, `Name`, `Value`, `Placeholder`, `Min`/`Max`/`Step`,
  `Title`, `For`, `Href`, `Disabled(…bool)`, `Selected(…bool)`, `SelectedIf(bool)`, `Attr(k,v)`,
  and events `OnClick`/`OnInput`/`OnChange`/`OnSubmit`/… (each takes `any`).
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
