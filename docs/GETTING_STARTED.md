# Getting Started — Adding Components & Pages

A practical guide to building UI in CashFlux on the [GoWebComponents] framework. If you've used React,
this will feel familiar — it's the same component/hook/props model, written in Go and compiled to WebAssembly.

See also: [`docs/GOWEBCOMPONENTS.md`](./GOWEBCOMPONENTS.md) (framework reference) and
[`CLAUDE.md`](../CLAUDE.md) (the engineering bar).

[GoWebComponents]: https://github.com/monstercameron/GoWebComponents

---

## 0. Mental model

```
pure Go logic (internal/<domain>)   ← no syscall/js, table-tested on native Go
        ▲
internal/appstate                   ← the ONE validated read/write seam
        ▲
internal/store (SQLite, in-tab)     ← persistence
        ▲
UI shell (internal/screens · ui · app · uistate)   ← thin; just renders the above
```

**Golden rule:** build **bottom-up** — model + tested logic first, UI *last*. The UI should never contain
business logic; it reads from `appstate` and renders.

Run the dev loop while you work:

```sh
./.tools/gwc.exe dev -app ./main.go -root .      # live-reload at http://127.0.0.1:8080
go test ./...                                     # pure logic, native Go
```

All UI files start with the build tag `//go:build js && wasm` and dot-import the shorthand DSL:

```go
//go:build js && wasm

package screens

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)
```

---

## 1. Anatomy of a component

A component is a **function** that returns a `ui.Node`. Stateless ones can take a plain props struct:

```go
// Badge is a tiny pill label. Pure presentation, no hooks.
func Badge(text, tone string) ui.Node {
	return Span(Class("badge "+tone), text)
}
```

Use it like any element: `Badge("Over", "prio-high")`.

A component **with state or events** uses hooks and is mounted via `ui.CreateElement`:

```go
type counterProps struct{ Start int }

func Counter(props counterProps) ui.Node {
	n := ui.UseState(props.Start)                       // .Get() / .Set() / .Update()
	inc := ui.UseEvent(Prevent(func() { n.Set(n.Get() + 1) }))
	return Div(Class("row"),
		Span(Textf("Count: %d", n.Get())),
		Button(Class("btn"), Type("button"), OnClick(inc), "Add"),
	)
}

// mount: ui.CreateElement(Counter, counterProps{Start: 0})
```

Hooks you'll use most: `ui.UseState`, `ui.UseEvent` (handler — `fn` can be `func()`, `func(string)` for
input values, or `func(ui.Event)`), `ui.UseEffect`, and `state.UseAtom(key, default)` for cross-component
state.

### ⚠️ The one rule that bites everyone: no `On*` inside a loop

Event handlers register hooks, so they must sit at **stable render positions**. You **cannot** put
`OnClick`/`OnInput`/etc. inside a variable-length `Map`/loop. Instead, make each row its **own component**
and pass plain `func` callbacks down as props — the row owns its handler hook:

```go
type rowProps struct {
	Item     domain.Thing
	OnDelete func(id string)        // plain func, NOT a hook
}

func ThingRow(props rowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Item.ID) }))  // hook lives here, stable
	return Div(Class("row"),
		Span(Class("row-desc"), props.Item.Name),
		Button(Class("btn-del"), Type("button"), OnClick(del), "✕"),
	)
}

// in the list:
MapKeyed(items,
	func(t domain.Thing) any { return t.ID },
	func(t domain.Thing) ui.Node {
		return ui.CreateElement(ThingRow, rowProps{Item: t, OnDelete: onDelete})
	},
)
```

`MapKeyed` results flatten into children automatically, so `Div(Class("rows"), MapKeyed(...))` just works.

---

## 2. Reading & writing data (the `appstate` seam)

Never touch the store from the UI. Read and write through `appstate.Default`:

```go
app := appstate.Default
if app == nil {
	return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
}

things := app.Things()                 // read

save := ui.UseEvent(Prevent(func() {
	if err := app.PutThing(t); err != nil {  // validated write
		errMsg.Set(err.Error())
		return
	}
	bump()                                    // re-render (see below)
}))
```

To re-render after a mutation, bump a **revision atom** (the established pattern across screens):

```go
rev := state.UseAtom("rev:things", 0)
bump := func() { rev.Set(rev.Get() + 1) }
```

(Reading `rev.Get()` once in the component subscribes it; bumping re-renders. For dashboard widgets, the
shared `uistate.UseDataRevision()` plays the same role after imports/loads.)

---

## 3. Adding a brand-new page (screen)

Pages are **registry-driven**: add a `Route` and it's automatically routed *and* shown in the rail (you
can't accidentally ship an unreachable screen — this is the B7 rule). Five steps:

**① Logic + persistence first (if the page has its own data).** Add a pure package under `internal/` with
table tests, then wire reads/writes into `internal/appstate` (+ `internal/store`) with lossless
export/import. No UI yet.

**② Write the view** in `internal/screens/<name>.go`:

```go
//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Widgets is the new screen.
func Widgets() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	return Div(
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.widgets")),
			P(Class("muted"), uistate.T("widgets.hint")),
			// …content…
		),
	)
}
```

**③ Register it** in `internal/screens/screens.go`, in `All()`:

```go
{Path: "/widgets", Label: "nav.widgets", Title: "nav.widgets",
 Subtitle: "screen.widgetsSub", Phase: 2, Group: GroupTools, View: Widgets},
```

`Group` is `GroupPrimary` / `GroupTools` / `GroupSystem` (decides the rail section). That's all routing
needs — `internal/app/app.go` loops `screens.All()` and registers each path wrapped in the `Shell`.

**④ Add the i18n strings** in `internal/i18n/en.go` (labels/titles/subtitles are keys resolved by
`uistate.T`, so the registry stays display-text-free): `nav.widgets`, `screen.widgetsSub`, `widgets.hint`, …

**⑤ Give it a rail icon** in `internal/app/shell.go` (`railMeta` map) and, if needed, add the glyph to
`internal/icon`:

```go
"/widgets": {"nav.widgets", icon.Box},
```

(If you skip this, the screen still appears in the rail with a default icon — nothing is dropped.)

Build, and the page is live, routed, and in the rail. Navigate in code with
`router.UseNavigate().Navigate(uistate.RoutePath("/widgets"))`.

---

## 4. Common building blocks

**A form** (state + submit + validation + inline error):

```go
name := ui.UseState("")
errMsg := ui.UseState("")
onName := ui.UseEvent(func(v string) { name.Set(v) })
add := ui.UseEvent(Prevent(func() {
	if name.Get() == "" { errMsg.Set(uistate.T("widgets.nameRequired")); return }
	// …app.PutThing(...)…; name.Set(""); errMsg.Set(""); bump()
}))

Form(Class("form-grid"), OnSubmit(add),
	Input(Class("field"), Type("text"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)),
	Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
)
```

**A select** built from a slice:

```go
opts := []ui.Node{Option(Value(""), SelectedIf(sel == ""), uistate.T("widgets.choose"))}
for _, c := range cats {
	opts = append(opts, Option(Value(c.ID), SelectedIf(sel == c.ID), c.Name))
}
Select(Class("field"), Attr("aria-label", uistate.T("widgets.category")), OnChange(onCat), opts)
```

**Reuse the component library** (`internal/ui`, imported as `uiw`) instead of hand-rolling:

- `uiw.DataTable(...)` — sortable, paginated table (give it columns + body rows).
- `uiw.FilterToolbar(...)`, `uiw.FlipPanel(...)` (modal), `uiw.ToggleRow(...)`, `uiw.ProgressBar(...)`,
  `uiw.Segmented(...)`, `uiw.Chart(...)` / `uiw.AreaChart(...)`.
- `screens.EmptyStateCTA(...)` for empty states with a call-to-action.
- `uiw.Icon(icon.Name, Class("w-4 h-4"))` for glyphs.

Prefer these over new markup — it keeps screens consistent and small.

---

## 5. Testing & finishing

```sh
go test ./...                                   # pure logic (native) — the important one
gofmt -l -w . && go vet ./...                   # clean before committing
GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .   # wasm builds
```

- **New logic ships with table-driven tests** in the same change.
- For UI flows, there are Playwright "story" scripts in `e2e/` (`node e2e/<story>.mjs`) that drive the
  real wasm app — copy one as a template if your feature needs an end-to-end check.
- **One feature per commit**, Conventional Commit subject, and update `CHANGELOG.md` + `DEVLOG.md`.

That's it — model it, test it, persist it, then render it. Welcome aboard. 💸
