# GoWebComponents — Framework Gaps & Friction (from CashFlux usage)

**Purpose.** A structured catalog of every place the CashFlux UI had to fight, work around, or
escape out of [GoWebComponents][gwc] (GWC). Intended as input to fixing the framework in its own
repo. Each finding is self-contained and uniformly formatted so it can be triaged/refined by another
agent without re-reading this whole file.

[gwc]: https://github.com/monstercameron/GoWebComponents

- **Scope:** UI/UX-affecting framework limitations only (not app bugs). Evidence is cited as
  `path:line` into the CashFlux tree at the time of writing.
- **Source method:** swept `internal/ui/*`, `internal/screens/*`, `internal/app/*`, `internal/uistate/*`
  for `syscall/js` usage, raw-DOM calls, re-render hacks, and "the framework …" comments.
- **What "Severity" means:** `high` = forces a raw-DOM escape hatch or per-feature boilerplate across
  many files; `med` = a repeated workaround confined to a wrapper; `low` = a one-off quirk.

---

## Summary table

| ID  | Area              | Severity | One-line gap |
|-----|-------------------|----------|--------------|
| G1  | Hooks / lists     | high     | `On*` handlers can't appear in a variable-length loop → every interactive row must be its own component |
| G2  | DOM refs          | high     | No ref that resolves to a rendered DOM node; must `UseId()` + `getElementById` to reach an element |
| G3  | Raw HTML          | high     | No raw/unsafe-HTML node; can't inject markup (SVG, markdown, Mermaid) without parsing or `innerHTML` |
| G4  | Portals / overlays| high     | No portal; top-level overlays triggered from outside the tree are hand-built in raw DOM |
| G5  | Re-render control | med      | No imperative invalidate; force-refresh needs a manual "version/revision" state/atom |
| G6  | Router reactivity | high     | `InspectCurrentRoute()` isn't reactive; memoized components freeze unless the path is threaded as a prop |
| G7  | Routing / base    | med      | Deep-link refresh 404s; `<base href>` breaks in-page anchors |
| G8  | SVG coverage      | med      | Renderer only draws `path`/`circle`/`rect`; richer SVG (charts) must go through a JS shim |
| G9  | Global events     | med      | No document/window-level event hook; global keyboard shortcuts use raw `addEventListener` |
| G10 | Focus management  | med      | No focus-trap / focus-restore primitive; every modal re-implements it in `syscall/js` |
| G11 | Styling API       | low      | `Style` takes only `map[string]string`; SVG presentation attrs can't take `var()` |
| G12 | Effect ergonomics | low      | `UseEffect` dep is a single value (often a serialized string), not a dependency list |
| G13 | Lifecycle / cleanup| med     | No managed `js.Func` lifetime; persistent listeners are intentionally leaked |

---

## G1 — `On*` event handlers cannot be used inside a variable-length loop

- **Area:** hooks / list rendering · **Severity:** high
- **Symptom:** Any list whose rows have a button/input/handler must extract the row into its **own**
  `ui.CreateElement(Row, props)` component and pass plain `func` callbacks down as props, because
  `On*` prop options register hooks and hooks must sit at stable render positions.
- **Evidence:**
  - Documented as the project's #1 gotcha: `docs/GOWEBCOMPONENTS.md:82-88`, `CLAUDE.md` "Hooks & framework gotchas".
  - Forced row-component split, repeated across the codebase:
    - `internal/ui/filtertoolbar.go:100-121` (`filterChip` exists *only* so the remove button's hook is stable — comment at :100-102).
    - `internal/ui/controls.go:102-104` (`segButton`), `:228-235` (toggle rows).
    - `internal/ui/datatable.go:75-102` (`dtHeader` per column).
    - `internal/app/shell.go:289-291` (`navItem`), `:148` (note the framework also wraps nav items in a `<div>`).
    - `internal/app/settings.go:109,179-180,235`, `internal/app/wsswitcher.go:141`, `internal/app/custompagesnav.go:201`, `internal/app/addmenu.go:19`.
- **Impact:** Pervasive boilerplate; the single biggest shaper of the codebase's component count.
  Easy to violate accidentally (silent/odd breakage on wasm). Raises the barrier for contributors.
- **Proposed direction:** Give hooks a stable identity tied to the keyed-list key (so `MapKeyed`
  children may legally own hooks), or provide a sanctioned "row needs a handler" helper. At minimum,
  emit a build-time or dev-mode runtime diagnostic when an `On*` is registered inside a loop.

---

## G2 — No DOM ref; reaching a rendered element requires `UseId()` + `getElementById`

- **Area:** DOM refs / interop · **Severity:** high
- **Symptom:** `UseRef` holds a Go value only (e.g. a "first render" flag), not a handle to the
  rendered DOM node. To let an external library draw into an element, the code assigns a stable id,
  renders an empty container, then resolves it by id from a `UseEffect`.
- **Evidence:**
  - Value-only ref: `internal/app/shell.go:50` (`uic.UseRef(true)` used purely as a render flag).
  - Id-then-lookup pattern: `internal/ui/chartd3.go:31` (`UseId()`), `:45-66` (`document.getElementById(id)`
    inside `UseEffect`, then handing the element to a JS shim). The comment at `:22-27` explicitly calls
    this "the ref/portal pattern" — i.e. an ad-hoc stand-in for a feature the framework lacks.
- **Impact:** Any third-party/imperative DOM integration (charts, editors, maps, focus calls) needs
  a brittle id round-trip and `syscall/js`. Ids must be globally unique and survive re-render.
- **Proposed direction:** A real element ref (`r := ui.UseDOMRef(); ... Div(Ref(r))`) whose `.Value()`
  is the live `js.Value`/element after mount, with documented null-before-mount semantics.

---

## G3 — No raw/unsafe-HTML node; cannot inject markup

- **Area:** raw HTML · **Severity:** high
- **Symptom:** There is no equivalent of `dangerouslySetInnerHTML` / a `RawHTML(string)` node. Any
  pre-rendered markup must either be **parsed into shorthand nodes** or written via `innerHTML` with
  raw `syscall/js`.
- **Evidence:**
  - Icons: `internal/ui/icon.go:46-79` regex-parses the canonical SVG inner markup (`<path>/<circle>/<rect>`)
    into shorthand `Path/Circle/Rect` nodes — a parser written solely because raw markup can't be inserted.
  - Help overlay & command palette built with `innerHTML`: `internal/app/shortcuts.go:171,461`
    (`card.Set("innerHTML", ...)`, `list.Set("innerHTML", ...)`) plus a hand-rolled `htmlEscaper`
    at `:215` (`strings.NewReplacer` for `& < > "`).
- **Impact:** Blocks whole feature classes that are "render this HTML": Markdown bodies, Mermaid
  diagrams, sanitized rich text. (These are on the CashFlux backlog — Mermaid/`marked` — and are
  gated on this gap.) Forces an XSS-escaping burden onto app code (`htmlEscaper`).
- **Proposed direction:** A `RawHTML(s string)` node (clearly marked unsafe) and/or a sanitized
  `Markup` node. Pairs naturally with G2 (ref) for libraries that prefer to own a subtree.

---

## G4 — No portal; top-level overlays triggered from outside the tree are hand-built in raw DOM

- **Area:** portals / overlays · **Severity:** high
- **Symptom:** Overlays that must (a) render at `<body>` level and (b) be opened from a non-component
  context (a global key handler) are constructed entirely in `syscall/js` — `createElement`,
  `appendChild`, manual show/hide — instead of being framework components.
- **Evidence:**
  - `internal/app/shortcuts.go:138-193` (help overlay) and `:347-426` (command palette): both
    `createElement`/`appendChild` to `document.body`, manage `style.display` by hand, wire their own
    `addEventListener`s, and maintain selection state in package globals (`:209-213`).
  - The comment at `:136-137` and `:310-312` names the cause: "a self-contained DOM overlay (not a
    framework component)… owned by the shortcut layer."
  - Contrast: overlays that *are* triggered from within the tree use a host-component + global-atom
    workaround instead — `SettingsHost`, `QuickAddHost`, `Toast` mounted at `internal/app/shell.go:73-75`,
    driven by atoms like `uistate.UseQuickAdd()`. That pattern works but is the only available one.
- **Impact:** Two parallel, inconsistent overlay strategies; the raw-DOM one duplicates focus/escape/
  click-outside logic and can't use the design-system components.
- **Proposed direction:** A `Portal`/`Overlay` primitive that renders to a target node, plus a way to
  drive component visibility from outside a render (a first-class global signal/atom open API).

---

## G5 — No imperative re-render; refresh after external mutation needs a manual "version" counter

- **Area:** re-render control · **Severity:** med
- **Symptom:** After a mutation that doesn't change a subscribed value (in-place edit, a write that
  lands in the store/localStorage), there's no way to ask the component to re-render. The idiom is to
  bump a dummy state/atom whose only job is to invalidate.
- **Evidence:**
  - `internal/app/custompagesnav.go:34-38`: `version := uic.UseState(0); _ = version.Get(); bump := func(){ version.Set(version.Get()+1) }` with the comment "A version counter forces a re-render."
  - Generalized into a documented convention: `docs/GETTING_STARTED.md:138-146` ("bump a **revision
    atom**", `state.UseAtom("rev:things",0)`), plus `uistate.UseDataRevision()` as a shared variant.
- **Impact:** Boilerplate on nearly every mutating screen; the `_ =` read-to-subscribe is non-obvious
  and a common source of "why didn't it update" bugs.
- **Proposed direction:** An explicit `forceUpdate`/`invalidate` from a hook, and/or store/atom
  integration so a persisted write notifies subscribers without a manual revision atom.

---

## G6 — `router.InspectCurrentRoute()` is not reactive; memoized chrome freezes unless the path is threaded as a prop

- **Area:** router reactivity · **Severity:** high
- **Symptom:** Reading the current route at render time doesn't re-render on navigation. Memoized
  components (no prop change) keep a stale route, so the active-nav highlight and breadcrumb freeze.
  The fix is to thread the logical path down as an explicit prop from the route factory.
- **Evidence:**
  - `internal/app/shell.go:24-33` (`ActivePath` prop) with the comment at `:27-31`: "the chrome
    cannot read it from `router.InspectCurrentRoute()` at render time because Sidebar and TopBar are
    memoized… freezing the highlight (regression covered by e2e/navigation.test.mjs)."
  - The path is then plumbed through `Sidebar`/`TopBar`/`navItem` purely to defeat this: `shell.go:68-71,156,235,384`.
  - Non-reactive read still used where a re-render isn't required: `internal/app/custompagesnav.go:33`.
- **Impact:** Every piece of chrome that depends on the route must accept and forward a path prop;
  easy to get wrong (stale highlight) and needs an e2e regression test to guard.
- **Proposed direction:** A reactive `useRoute()`/`useLocation()` hook that subscribes the calling
  component to navigation, so chrome can read the live route without prop-drilling.

---

## G7 — Deep-link refresh 404s and `<base href>` breaks in-page anchors

- **Area:** routing / hosting · **Severity:** med
- **Symptom:** Refreshing on a deep link (e.g. `/transactions`) 404s on a static host; only the root
  boots. Separately, the `<base href>` needed to make deep-link asset paths resolve also makes a bare
  `#main` anchor resolve against the base (navigating to root), so in-page "skip to content" links
  must be rewritten to include the live path.
- **Evidence:**
  - `<base href>`/route interaction: `internal/app/app.go:55` and `internal/app/shell.go:62-67`
    (skip link must embed `RoutePath(props.ActivePath)+"#main"` — comment at :62-66).
  - Tracked in the app backlog as bug B1 (deep-link 404; probe must navigate from `/`).
- **Impact:** Hash-vs-history hosting friction; SPA-on-static-host (GitHub Pages) needs app-side
  compensation, and any in-page anchor is a footgun.
- **Proposed direction:** First-class static-host guidance/support (SPA fallback, or a hash-router
  option), and base-href-aware anchor/asset helpers so apps don't hand-correct.

---

## G8 — SVG renderer only draws `path`/`circle`/`rect`; richer SVG must go through a JS shim

- **Area:** SVG coverage · **Severity:** med
- **Symptom:** The shorthand SVG renderer effectively supports only `path`, `circle`, `rect`. Anything
  needing `g`, `line`, `polyline`, `polygon`, `text`, gradients, etc. (data-viz) can't be expressed as
  nodes and is delegated to an external D3 shim drawing into a container.
- **Evidence:**
  - Icon parser only emits those three element kinds: `internal/ui/icon.go:69-77`; a test enforces
    the limit: `internal/icon/icon_test.go:60,73` ("the renderer only draws path/circle/rect").
  - Charts therefore bypass the renderer entirely: `internal/ui/chartd3.go:22-27,54-56` (hands the
    element + JSON to `cashfluxRenderChart`, a JS function).
- **Impact:** All real charting lives in JS, not Go — undercutting the "pure Go on the frontend"
  story, and requiring G2's id round-trip to bridge.
- **Proposed direction:** Broaden SVG element/attribute coverage in `html/shorthand` (at least the
  common chart primitives + `<g>`/`<text>`), enabling native-Go rendering of charts.

---

## G9 — No document/window-level event hook; global shortcuts use raw `addEventListener`

- **Area:** global events · **Severity:** med
- **Symptom:** There's no hook to subscribe to document/window events from a component. Global
  keyboard shortcuts are installed once at boot via `syscall/js` `addEventListener` on `document`.
- **Evidence:** `internal/app/shortcuts.go:23-90` (`wireKeyboardShortcuts` → `doc.Call("addEventListener","keydown",…)`),
  with the `js.Func` "intentionally never released" (`:17-18`). Element-level keyboard *is* supported
  (`OnKeyDown` in `internal/ui/controls.go:64,206,276`) — the gap is specifically global/document scope.
- **Impact:** App-wide shortcuts, hotkeys, and outside-click detection drop to raw interop and must
  be coordinated by hand (e.g. suppressing shortcuts while typing — `:55,94-109`).
- **Proposed direction:** A `UseDocumentEvent`/`UseWindowEvent` (or `UseGlobalKey`) hook with managed
  listener lifetime (ties into G13).

---

## G10 — No focus-trap / focus-restore primitive; every modal reimplements it

- **Area:** focus management / a11y · **Severity:** med
- **Symptom:** Modal accessibility (move focus in on open, trap Tab/Shift-Tab, restore focus on
  close, Esc to close) has no framework support and is hand-written in `syscall/js` per modal.
- **Evidence:** `internal/ui/flippanel.go:48-147`: queries `.flip-wrap`, enumerates focusables,
  manages `prevFocus`, traps Tab, restores focus on cleanup — ~100 lines of raw DOM. The app-lock
  gate "mirrors the FlipPanel trap" independently (`internal/app/applockgate.go:236`).
- **Impact:** Accessibility correctness is duplicated and easy to get subtly wrong; each new
  dialog/overlay re-pays the cost (see also the raw-DOM overlays in G4 which trap focus separately).
- **Proposed direction:** A `UseFocusTrap(ref)` / `<Dialog>` primitive providing trap + restore +
  initial focus + Esc, building on G2 (ref) and G4 (portal).

---

## G11 — Styling API quirks: `Style` is `map[string]string` only; SVG attrs can't take `var()`

- **Area:** styling · **Severity:** low
- **Symptom:** Inline styles accept only `map[string]string`. Separately, themed SVG line weight has
  to be applied via inline `style` rather than the `stroke-width` presentation attribute, because SVG
  *attributes* don't accept `var()` while the CSS property does.
- **Evidence:** `internal/ui/icon.go:31-34` (comment: "SVG attributes don't accept var(), while the
  CSS property does — and inline style beats the attribute"); `Style(map[string]string{...})` usage
  throughout (`flippanel.go:192`, `chartd3.go:77`, `controls.go:273`).
- **Impact:** Minor, but the var()-in-attribute trap is non-obvious and the styling map is stringly typed.
- **Proposed direction:** Document the SVG/var() interaction; consider typed style helpers.

---

## G12 — `UseEffect` takes a single dependency value, not a dependency list

- **Area:** effect ergonomics · **Severity:** low
- **Symptom:** `UseEffect(fn, dep)` re-runs on change of one value. To depend on multiple inputs, code
  serializes them into one string (e.g. JSON) and keys the effect on that.
- **Evidence:** `internal/ui/chartd3.go:37-40,45,66` (marshals the whole spec to `specJSON` and uses
  it as the single effect dep). `internal/app/shell.go:52-60` (effect keyed on `props.ActivePath`).
- **Impact:** Serialization overhead and an awkward idiom when an effect genuinely depends on several
  values; allocation churn for large deps (the chart re-marshals on every render to compare).
- **Proposed direction:** Accept a variadic/slice dependency list with value equality, matching the
  mental model contributors arrive with from React.

---

## G13 — No managed `js.Func` lifetime; long-lived listeners are intentionally leaked

- **Area:** lifecycle / cleanup · **Severity:** med
- **Symptom:** `js.FuncOf` callbacks must be `Release()`d manually. App-lifetime listeners are
  knowingly never released; only modal-scoped ones get cleaned up (and only because the component
  unmounts).
- **Evidence:** Intentional leaks: `internal/app/shortcuts.go:17-18,89` (keydown), `:163` (overlay
  click/close funcs). Correct-but-manual cleanup: `internal/ui/flippanel.go:140-146`
  (`removeEventListener` + `cb.Release()` in the effect's teardown).
- **Impact:** Easy to leak `js.Func`s (memory) or release too early (dead callbacks). Cleanup
  correctness is entirely on the app author.
- **Proposed direction:** Effect-scoped event subscription helpers (G9) that own the `js.Func`
  lifetime, so apps never call `FuncOf`/`Release` directly for common cases.

---

## What works well (keep / don't regress)

Balancing the above — these were ergonomic and rarely fought:

- **`MapKeyed(items, keyFn, render)`** with auto-flattening children — clean keyed lists
  (`internal/app/shell.go:227-265`, `internal/ui/controls.go:338-354`).
- **Drag-and-drop handlers** `OnDragStart`/`OnDragOver`/`OnDrop` exist and work
  (`internal/app/shell.go:314-330`).
- **Element-level keyboard** `OnKeyDown` with a typed `KeyboardEvent` (`internal/ui/controls.go:64-73`).
- **Form input ergonomics:** `OnInput(func(string))`, `OnChange` + `e.GetValue()`, `SelectedIf`,
  `Value()` (`internal/ui/datatable.go:115-119`, `filtertoolbar.go:75-77`).
- **`If`/`IfElse`/`Fragment`** control-flow nodes (`internal/app/custompagesnav.go:31`,
  `internal/ui/filtertoolbar.go:61,81,90`).
- **Props-driven component composition** scales cleanly once the row-component pattern is internalized.

---

## Cross-cutting theme

Most `high`/`med` findings reduce to **three missing primitives** whose absence cascades into raw
`syscall/js`:

1. **A DOM ref (G2)** — needed by charts, focus, any imperative library.
2. **A portal + raw-HTML node (G3, G4)** — needed by overlays, markdown, Mermaid, charts.
3. **Reactive route + managed global events (G6, G9, G13)** — needed by chrome and app-wide shortcuts.

Delivering those three would let CashFlux delete its hand-rolled overlays (`shortcuts.go`), its chart
id round-trip (`chartd3.go`), its focus-trap duplication (`flippanel.go`), and most of its revision-
atom boilerplate — and would unblock the Markdown/Mermaid backlog. The `On*`-in-loops rule (G1) is
the remaining structural item and the highest-leverage single fix for contributor experience.

---

## Addendum — confirmations & new findings from `DEVLOG.md`

Scanning the developer journal corroborated the findings above with dated, real-world instances and
surfaced four additional items (G14–G17).

**Confirmations (same gaps, more evidence):**

- **G1 (`On*` in loops)** — recurring per-row component splits called out explicitly:
  `DEVLOG.md:475-476` ("the chip remove button is its own filterChip component (stable hook position
  for the loop)"), `:490-492` (FilterChip, "the framework loop-hook gotcha"), `:376` (`suggestChip`
  "so the click hook is stable"), `:569-572` (per-row hook safe only because the row is a component),
  `:791` (settle-up payment rows their own `settleTransferRow`, "no-hooks-in-loops rule").
- **G5 (manual re-render)** — `DEVLOG.md:791` ("a rev `UseState` bump forces the re-render") confirms
  the revision-counter idiom is reached for whenever a write doesn't change a subscribed value.
- **G10 (manual a11y/focus)** — `DEVLOG.md:585` (roving-tabindex Left/Up/Right/Down navigation
  hand-built; "without it, roving tabindex would have [been incomplete]"), `:453` (every control's
  accessible-name wiring done by hand). Accessibility correctness is consistently app-authored.

**New findings:**

### G14 — Native input/file pickers must be created off-DOM in raw `syscall/js`
- **Area:** DOM interop · **Severity:** med
- **Symptom:** A `<input type=file>` (incl. camera `capture`) can't be expressed/triggered as a
  framework node for a programmatic "pick a file" flow; it's created off-DOM and clicked via raw JS.
- **Evidence:** `DEVLOG.md:779` ("the input is created off-DOM … desktop ignores it"); the file-pick
  helper is used from the command palette (`internal/app/shortcuts.go:259-265`, `pickFile`).
- **Impact:** Import/upload/camera flows drop to raw interop; ties into the missing ref (G2) and
  portal (G4) primitives.
- **Proposed direction:** A file-input/`usePicker` helper, or general ref support so an app can hold
  and `.click()` a rendered input.

### G15 — UI-layer logic is only `js && wasm`, so it can't be unit-tested natively
- **Area:** testability · **Severity:** med
- **Symptom:** Logic that lives in the wasm UI layer can't run under `go test` on native Go, forcing
  extraction of otherwise-UI concerns into separate pure packages purely to make them testable.
- **Evidence:** `DEVLOG.md:261` (command-palette matching extracted into a NEW pure `internal/cmdmatch`
  package "because the live `shortcuts.go` match is js/wasm"), `:601` ("js/wasm-only, so this is the
  only way to unit-test it natively").
- **Impact:** This is partly by-design (CashFlux's bottom-up rule is a virtue), but the framework
  offers no native-testable seam for view logic, so any branch logic in a component is untestable
  unless manually carved out.
- **Proposed direction:** A headless/native render+assert harness for components (render to a string
  or virtual tree under native Go), so view logic has a unit-test path without a browser.

### G16 — Boot/first-render timing requires an app-managed splash workaround
- **Area:** mount lifecycle · **Severity:** low
- **Symptom:** There's a visible gap between page load and first wasm render; the app hand-manages a
  splash overlay and has to special-case "`#app` already has children" to avoid a missed first render.
- **Evidence:** `DEVLOG.md:621-625` ("if `#app` already has children when the script runs, hide
  immediately (no missed first-render); and a 4s [fallback]"), `:541,576` (splash-fix references).
- **Impact:** Every app re-invents loading-state handling; no framework signal for "first render
  committed."
- **Proposed direction:** A mount/ready callback or event the host page can hook to drop a splash
  deterministically.

### G17 — `OnInput` requires a framework `Handler`, not a plain Go func
- **Area:** event API ergonomics · **Severity:** low
- **Symptom:** Some event props expect a framework `Handler` wrapper rather than accepting a bare
  `func`, an inconsistency with the plain-`func` callbacks used elsewhere.
- **Evidence:** `DEVLOG.md:475` ("OnInput wants a framework Handler"). Compare the plain-func style
  threaded through row components for G1.
- **Impact:** Minor inconsistency; a small papercut when wiring inputs.
- **Proposed direction:** Accept plain `func(string)`/`func(Event)` uniformly across all `On*`, or
  document which props need the `Handler` wrapper and why.

---

## Addendum 2 — deeper raw-interop sweep (G18–G25)

A second, exhaustive pass over **all** `syscall/js` usage (129 `js.Global()` call sites across 36
files; 68 `localStorage` calls across 14 `uistate` files) found that the framework's missing
primitives push far more into raw DOM than the first pass showed — including whole screens. These are
the highest-volume sources of `syscall/js` in the app.

| ID  | Area                | Severity | One-line gap |
|-----|---------------------|----------|--------------|
| G18 | Dialogs             | high     | No dialog/confirm/prompt primitive → native `alert`/`confirm`/`prompt` for destructive guards & text input |
| G19 | Timers              | med      | No timer hook → `setTimeout`/`setInterval`/`clearTimeout` via raw JS, with manual `js.Func` release |
| G20 | Media queries       | med      | No media-query hook → `matchMedia` read imperatively (color-scheme, reduced-motion) and not reactive |
| G21 | State persistence   | high     | Atoms have no persistence layer → every preference hand-rolls a localStorage Load+Persist+atom triad |
| G22 | Autofocus / focus   | high     | No autofocus prop / element ref → inline-edit focus done via `focusByID` (getElementById+focus) in ~13 screens |
| G23 | File I/O            | med      | (expands G14) Both download and upload are 100% raw DOM (Blob+anchor; off-DOM input+FileReader) |
| G24 | Raw-DOM screens     | high     | (expands G4) Lacking portal/raw-HTML, an *entire screen* (passcode gate) is built in `innerHTML`+`cssText` |
| G25 | Global activity     | med      | (expands G9) Idle auto-lock attaches `mousemove/keydown/click/touchstart/scroll` listeners by hand |

### G18 — No dialog primitive; destructive guards & text input use native `alert`/`confirm`/`prompt`
- **Area:** dialogs · **Severity:** high
- **Symptom:** Confirmations ("delete this?", "wipe all data?") and one-off text input (new/rename
  workspace) fall back to the browser's blocking native dialogs, which ignore the theme/design system
  and can't be e2e-driven.
- **Evidence:** `confirm`: `internal/app/download.go:33-34` (`confirmAction`, wipe guard),
  `internal/app/custompagesnav.go:270` (delete page). `alert`: `internal/app/shortcuts.go:266,282`,
  `internal/app/wsswitcher.go:249`. `prompt`: `internal/app/wsswitcher.go:293` (`promptName`),
  used from the palette at `shortcuts.go:253-256`.
- **Impact:** Inconsistent, unthemeable, untestable UX for exactly the highest-stakes actions; blocks
  the main thread.
- **Proposed direction:** A framework `Dialog`/`Confirm`/`Prompt` (promise-returning) overlay, built
  on the portal (G4), so apps stop reaching for `window.confirm`.

### G19 — No timer/interval hook; raw `setTimeout`/`setInterval` with manual cleanup
- **Area:** timers · **Severity:** med
- **Symptom:** Auto-dismiss, debounce, and polling are implemented with raw `setTimeout`/`setInterval`
  and hand-managed `js.Func` release/clear.
- **Evidence:** Toast auto-dismiss: `internal/app/toast.go:48-62` (FuncOf + setTimeout + clearTimeout
  + Release in effect cleanup). AI debounce: `internal/ai/transport.go:116`. Idle-lock poll:
  `internal/app/applockgate.go:463` (`setInterval(check, 30000)`). Gate animation delay: `:55`.
- **Impact:** Every timed behavior re-implements the FuncOf/clear/Release dance (G13); easy to leak
  or fire stale callbacks.
- **Proposed direction:** `UseTimeout`/`UseInterval`/`UseDebounce` hooks with effect-scoped lifetime.

### G20 — No media-query hook; `matchMedia` read imperatively and non-reactively
- **Area:** media queries · **Severity:** med
- **Symptom:** Color-scheme and reduced-motion preferences are read via `matchMedia` at call time;
  there's no reactive subscription, and reduced-motion must be re-checked imperatively before each
  animation.
- **Evidence:** `prefers-color-scheme`: `internal/uistate/theme.go:82`, `internal/uistate/prefs.go:60`.
  `prefers-reduced-motion`: `internal/app/applockgate.go:34,72,167` (checked before each animation).
- **Impact:** No live response to OS theme changes; animation-gating logic is scattered and repeated.
- **Proposed direction:** A `UseMediaQuery(query)` hook returning a reactive bool (with a
  `UsePrefersReducedMotion` convenience).

### G21 — Atoms have no persistence layer; every preference hand-rolls localStorage
- **Area:** state persistence · **Severity:** high
- **Symptom:** `state.UseAtom` is in-memory only. To make any preference survive reload, the app
  writes a matching `loadX()` (read+JSON-unmarshal from localStorage as the atom seed) and `PersistX()`
  (JSON-marshal+write) pair, and must remember to call Persist on every mutation.
- **Evidence:** The pattern repeats across **14** `uistate` files (68 localStorage calls):
  `internal/uistate/navorder.go:34-54` (canonical triad), plus `layout.go`, `widgetcfg.go`, `i18n.go`,
  `txfilter.go`, `modules.go`, `fonts.go`, `freshness.go`, `rail.go`, `theme.go`, `banner.go`,
  `prefs.go`, `period.go`, `aikey.go`. The `Persist*` calls are then sprinkled through the UI
  (e.g. `shell.go:396`, `controls.go` resolution persistence).
- **Impact:** The largest single category of boilerplate; "forgot to Persist after Set" is a whole
  bug class. Every new preference is three pieces of ceremony.
- **Proposed direction:** A persisted-atom variant — `state.UsePersistentAtom(key, default)` — that
  reads its seed and writes through on `Set` (pluggable storage), eliminating the triad.

### G22 — No autofocus/element-focus; inline-edit focus uses `focusByID` across ~13 screens
- **Area:** focus / forms · **Severity:** high
- **Symptom:** There's no `autoFocus` prop and no element ref, so when an inline editor opens, the row
  component runs a `UseEffect` that builds the field's id and calls `getElementById(id).focus()`.
- **Evidence:** Helper `internal/screens/focus.go:12-25`; called from inline editors in
  `transactions.go:717`, `todo.go:236`, `budgets.go:503`, `goals.go:385-387`, `accounts.go:615-617`,
  `categories.go:270`, `members.go:296`, `rules.go:287`, `documents.go:495`, `custompage.go:317`,
  and the empty-state CTA `emptystate.go:34`. The "land the cursor in the first field (§6.7)" comment
  recurs in 9+ files.
- **Impact:** A ubiquitous, id-string-coupled workaround (this is the most common concrete symptom of
  the missing ref, G2); fragile if ids collide or the element isn't mounted yet.
- **Proposed direction:** An `AutoFocus()` prop option and/or the DOM ref from G2, so focus needs no
  id round-trip.

### G23 — File download and upload are entirely raw DOM (expands G14)
- **Area:** file I/O · **Severity:** med
- **Symptom:** Export builds a `Blob` + transient `<a>` and clicks it; import creates an off-DOM
  `<input type=file>`, wires a `FileReader`, and copies bytes — all in `syscall/js`.
- **Evidence:** `internal/app/download.go:11-29` (`downloadBytes`), `:40-82` (`pickFile`/`pickFileNamed`
  with manual `js.Func` release).
- **Impact:** All import/export/backup/camera flows live outside the framework; pure manual interop.
- **Proposed direction:** `useDownload(bytes, name, mime)` and `usePicker(accept) → bytes` helpers.

### G24 — Lacking portal/raw-HTML, an entire screen (passcode gate) is built in `innerHTML` + `cssText` (expands G4)
- **Area:** raw-DOM screens · **Severity:** high
- **Symptom:** The app-lock gate — a full-screen modal with inputs, buttons, hint, and animation — is
  constructed entirely with `createElement`, `innerHTML`, and inline `style.cssText` strings, with its
  own i18n escaper (`escT`), because it must exist above the component tree and be shown/hidden from
  outside a render.
- **Evidence:** `internal/app/applockgate.go` — `cssText` style strings at `:136,142,217,375-376`,
  `innerHTML` markup blocks around `:370-380`, its own activity listeners at `:442-443`. ~460 lines of
  raw DOM for one screen.
- **Impact:** A core security surface can't use the design system, themes (it inlines `var(--accent)`
  fallbacks by hand), or any UI primitive; it duplicates focus/animation/escape logic.
- **Proposed direction:** Same as G4 (portal + outside-render visibility control) would let this be a
  normal component; G3 (raw-HTML) and G18 (dialog) reduce what's left.

### G25 — Global activity listeners attached by hand (expands G9)
- **Area:** global events · **Severity:** med
- **Symptom:** Idle auto-lock listens for `mousemove/keydown/click/touchstart/scroll` on `document`
  via raw `addEventListener` to reset an activity timer.
- **Evidence:** `internal/app/applockgate.go:441-443`, paired with the `setInterval` poll at `:463`.
- **Impact:** Reinforces G9/G13 — app-wide event needs and listener lifetimes are entirely manual.
- **Proposed direction:** The managed global-event hook from G9 covers this.

### Updated cross-cutting count

The same three missing primitives still dominate, now with the additions folded in:

1. **DOM ref + autofocus (G2, G22)** — the single most common workaround (focusByID in ~13 screens).
2. **Portal + raw-HTML + dialog (G3, G4, G18, G24)** — overlays, the passcode gate, native confirms,
   and the Markdown/Mermaid backlog all trace here.
3. **Effect-scoped lifecycle for timers/events/media (G9, G13, G19, G20, G25)** — every timed or
   global behavior re-implements FuncOf/clear/Release by hand.

Plus the standalone **persisted-atom gap (G21)**, which alone accounts for ~68 of the raw
localStorage calls and is the easiest high-value framework win.

---

## Addendum 3 — missing APIs & utilities (U1–U7)

Distinct from the raw-DOM escapes above: places where the framework simply lacks a convenience
API/util, so the app either uses a generic escape hatch (`Attr(k,v)`) hundreds of times or reinvents
a helper that arguably belongs in the framework. These don't *force* `syscall/js`, but they're
high-volume papercuts and inconsistencies.

| ID  | Area                | Severity | One-line gap |
|-----|---------------------|----------|--------------|
| U1  | Typed attributes    | med      | Only a few typed attr options exist; ~200 common attrs use the stringly-typed `Attr(k,v)` escape hatch |
| U2  | Conditional attrs   | med      | `SelectedIf` exists but no `DisabledIf`/`CheckedIf`/`AttrIf`; conditional attrs built by slice-append |
| U3  | Class builder       | med      | No `clsx`/`Classes(...)` helper; conditional classes are manual string concatenation (26 sites) |
| U4  | Form/a11y helpers   | med      | No field/label/error-association helper; app hand-rolled `errAttrs`/`errText` for aria wiring |
| U5  | Roving-tabindex     | med      | No radiogroup/roving-tabindex primitive; Segmented/Swatch/Toggle each reimplement ARIA + arrow nav |
| U6  | Input binding       | low      | No two-way `Bind(state)` helper; every controlled input wires `Value(get)`+`OnInput(set)` by hand |
| U7  | Formatting utils    | low      | Generic text utils (e.g. snake_case→Title) reinvented per app |

### U1 — Sparse typed attribute helpers; ~200 attrs fall back to `Attr(k,v)`
- **Area:** DSL / typed attributes · **Severity:** med
- **Symptom:** The shorthand DSL provides typed options for a handful of attributes (`Class`, `Value`,
  `Placeholder`, `Type`, `Title`, `SelectedIf`) but **not** for most common ones — `id`, `disabled`,
  `checked`, `required`, `readonly`, `role`, `tabindex`, `aria-*`, `scope`, `for`, `min`/`max`/`step`,
  `draggable`, `target`/`rel`, and SVG attrs (`viewBox`, `stroke`, `fill`). Those use the
  stringly-typed `Attr("name", "value")` escape hatch.
- **Evidence:** **200** such `Attr("…")` calls for those attributes across 35 files (count via
  `Attr("disabled"|"checked"|"aria-…"|"role"|"tabindex"|…)`); 107 more for `id`/`href`/etc. Concrete:
  `internal/ui/icon.go:27-36` (`viewBox`/`fill`/`stroke`/`stroke-linecap` all via `Attr`),
  `internal/ui/controls.go:64-73,118-122` (`role`/`aria-checked`/`tabindex`),
  `internal/ui/datatable.go:77,99` (`scope`/`aria-sort`).
- **Impact:** Attribute-name typos are silent (no compile-time check — the whole value of typed Go on
  the frontend); inconsistent call style; no IDE discoverability.
- **Proposed direction:** Typed option helpers for the standard HTML/SVG/ARIA attribute set
  (`Id`, `Disabled`, `Checked`, `Required`, `Role`, `TabIndex`, `AriaLabel`, `Scope`, `For`, …),
  keeping `Attr` only for genuinely custom attributes.

### U2 — Only `SelectedIf` exists; no `DisabledIf`/`CheckedIf`/`AttrIf`
- **Area:** conditional attributes · **Severity:** med
- **Symptom:** There's a `SelectedIf(bool)` conditional option but no equivalents for the other common
  boolean attributes, so conditional `disabled`/`checked` is done by building an `[]any` args slice
  and conditionally appending `Attr("disabled","disabled")`. The a11y helper even returns a nil
  `[]any` to spread-or-no-op (`errAttrs`) — an idiom invented to fill the `AttrIf` gap.
- **Evidence:** `internal/ui/datatable.go:130-139` (prev/next buttons append `Attr("disabled",…)`
  conditionally); `internal/screens/aria.go:19-24` (`errAttrs` returns `nil` or attrs to spread).
  Contrast `SelectedIf` usage at `datatable.go:126,128`, `filtertoolbar.go` option lists.
- **Impact:** Asymmetric API; verbose slice-append plumbing for a one-bit decision.
- **Proposed direction:** `DisabledIf(bool)`, `CheckedIf(bool)`, and a general
  `AttrIf(cond, name, value)` / `When(cond, ...PropOption)`.

### U3 — No class-name builder (clsx/classnames)
- **Area:** class building · **Severity:** med
- **Symptom:** `Class` takes a single string, so conditional/variant classes are assembled with manual
  string concatenation (`cls := "base"; if active { cls += " on" }`, or full re-assignment per state).
- **Evidence:** **26** `cls :=`/`cls +=` sites across 10 files: `internal/ui/controls.go:105-116,
  185-191,249-255` (toggle/seg/swatch states), `internal/app/shell.go:293-299` (nav variants),
  `internal/ui/datatable.go:54-57`, `internal/ui/chartd3.go:68-71`.
- **Impact:** Repetitive, error-prone (stray spaces, forgotten variants); a classic solved problem.
- **Proposed direction:** A variadic `Classes(parts ...any)` accepting strings and `cond && "cls"` /
  `map[string]bool`, plus a `ClassIf(cond, cls)` option.

### U4 — No form-field / a11y wiring helper
- **Area:** forms / accessibility · **Severity:** med
- **Symptom:** Associating an input with its label and validation error (`aria-invalid`,
  `aria-describedby`, error `role="alert"` + matching `id`) has no framework helper; the app built its
  own `errAttrs`/`errText`, and every screen wires labels/ids by hand.
- **Evidence:** `internal/screens/aria.go:10-32` (app-authored `errAttrs`/`errText`); used across the
  inline-edit forms. No framework `Field`/`Label`/`FormError` primitive is referenced anywhere.
- **Impact:** Accessibility correctness is opt-in and duplicated; easy to ship an unlabeled or
  unassociated error.
- **Proposed direction:** A `Field`/`Label`/`ErrorText` set (or a `useField` hook) that generates and
  wires the ids and ARIA relationships.

### U5 — No roving-tabindex / radiogroup primitive
- **Area:** accessibility components · **Severity:** med
- **Symptom:** ARIA radiogroup semantics (one Tab stop, arrow-key navigation, `role=radio` +
  `aria-checked`, selection-follows-focus) are reimplemented in each segmented/swatch/toggle control.
- **Evidence:** `internal/ui/controls.go:37-92` (Segmented roving tabindex + arrow nav), `:298-355`
  (SwatchPicker, same logic again), `:184-217` (Toggle as a `role=switch` div needing manual
  `tabindex`/key handling). The app-lock gate and FlipPanel separately hand-roll focus order.
- **Impact:** The same intricate a11y logic is written 3+ times; high bug surface.
- **Proposed direction:** A `RadioGroup`/`useRovingTabIndex` primitive (and a `Switch` component) so
  controls get correct keyboard a11y for free.

### U6 — No two-way input binding helper
- **Area:** forms · **Severity:** low
- **Symptom:** Every controlled input manually pairs `Value(state.Get())` with
  `OnInput(func(v){ state.Set(v) })`; there's no `Bind(state)` shorthand.
- **Evidence:** Ubiquitous; e.g. `internal/ui/filtertoolbar.go:54,75-77`,
  `internal/app/shell.go` form fields, the inline editors in every screen.
- **Impact:** Boilerplate on every field; minor but pervasive.
- **Proposed direction:** A `Bind(state.Atom[string])` option expanding to value+handler (with a
  numeric/`Parse` variant).

### U7 — Generic text/format utilities reinvented per app
- **Area:** utilities · **Severity:** low
- **Symptom:** Small generic helpers that aren't domain-specific are written app-side because the
  framework ships none — e.g. enum/snake_case → Title-case humanization.
- **Evidence:** `internal/screens/format.go:52-59` (`humanizeType`: `credit_card` → "Credit card").
  (The money/tone formatters there are correctly app-domain and *should* stay app-side — only the
  generic string helper is the gap.)
- **Impact:** Trivial individually; noted for completeness since a small `strutil`/`textutil` would
  remove a few reinventions.
- **Proposed direction:** Optional — a tiny text-util subpackage, or simply documenting that these
  stay app-side by design.

### Note on what the DSL *does* provide (so fixes don't duplicate)

Confirmed present and good: `Class`, `Value`, `Placeholder`, `Type`, `Title`, `SelectedIf`,
`OnClick`/`OnInput`/`OnChange`/`OnKeyDown`/`OnDragStart`/`OnDragOver`/`OnDrop`, `Prevent(fn)`,
`Text`/`Textf`, control-flow `If`/`IfElse`/`Map`/`MapKeyed`/`Fragment`, and the SVG element nodes
`Svg`/`Path`/`Circle`/`Rect`. The U-series is about *filling out* this set, not replacing it.

---

## Addendum 4 — `css` typed-CSS gaps (CSS1–CSS6)

**Context.** CashFlux ships ~2,000 lines of CSS as two inline `<style>` blocks in `web/index.html`
(the bento shell, the WONDER animation suite, legacy component styles, the `:root` token palette,
and the `[data-theme="light"]` override layer). The C91 work already migrated the Tailwind *utility*
layer to the typed `css` package (`internal/ui/tw` over `css`/`css/u`), and per-component styling via
`css.New(...)` works well (`internal/ui/meter.go:63`, `internal/ui/progress.go:64`). The open question
was whether the *remaining inline stylesheet* can also move into the `css` package. The
component-scoped, animation, and responsive parts port cleanly. The items below are the parts that
**do not** have a first-class path today — they're the framework gaps to close before the global
stylesheet can leave `index.html`.

> Evidence cites the inline CSS in `web/index.html` and the `css` package API (module
> `github.com/monstercameron/GoWebComponents v1.1.1-0.20260621010857-935d73b0cd3a`, package `css`:
> `css.go`, `rule.go`, `selector.go`, `variant.go`, `theme.go`, `dynamic.go`).

| ID   | Area                  | Severity | One-line gap |
|------|-----------------------|----------|--------------|
| CSS1 | Global/element rules  | high     | `New` only emits one **hashed class** scoped under `&`; no way to author top-level global rules (`*`, `body`, bare `.semantic-class`, `:root`) |
| CSS2 | `:root` tokens / theme| high     | `css.Theme`/`UseTheme` resolves scales at class-gen time; no API emits a `:root{--…}` custom-property palette or drives utilities off **live** CSS variables |
| CSS3 | Ancestor-state variant| med      | Variants cover self pseudo-states (`Hover`/`Focus`) + `Media`, but there's no `[data-theme=…] &` / `[data-density=…] &` ancestor-attribute variant (the whole light-theme override model) |
| CSS4 | Cascade / ordering    | med      | Hashed classes emit in registration order; no layer/specificity control to make overrides reliably beat runtime-injected styles (`MarkImportant` is the only lever) |
| CSS5 | Base/reset layer      | low      | No managed preflight/reset; the app hand-maintains a "minimal Tailwind-preflight equivalent" inline |
| CSS6 | SSR critical-CSS seam | low      | The native buffer sink + `Seed` exist, but there's no documented build-time pipeline to extract a Go-authored sheet into `index.html` and hydrate it |

### CSS1 — `css.New` only emits a hashed class scoped under `&`; no global/element/semantic-class rules
- **Area:** global stylesheet authoring · **Severity:** high
- **Symptom:** `css.New(rules...)` folds its rules into a single **content-hashed** class, and every
  combinator/variant nests under that generated class (`&`). There is no way to emit an un-prefixed
  top-level rule — an element selector (`body`, `h3`), the universal selector (`*`, the preflight),
  `:root`, or a stable semantic class (`.nav-link`, `.btn`, `.bento`, `.w`) that other markup or the
  runtime theme engine targets by name. `selector.El`/`ClassSel`/`Sel` exist only as the *target* of a
  combinator (`Child`/`Descendant`/…), producing `& <target>`, so they still require the hashed-class
  prefix. The escape hatch — author rules with literal selectors and call `New` once at boot, throwing
  away the returned hash — leans entirely on `Sel(...)` and defeats the type-safety the package exists
  to provide.
- **Evidence:** Hashed-class model — `css.go:33-41` (`New` "folds a rule-set into a single
  content-hashed class"); combinators fold under `&` — `selector.go:48-81` (`Child`/`Descendant`/etc.
  scope "& <combinator> <target>"). The inline CSS that has no home: global element rules + preflight
  `web/index.html:58-101` (the `:root` block + "minimal Tailwind-preflight equivalent"), and the large
  body of semantic-class rules (`.nav-link`, `.bento`, `.w`, `.btn`, `.row-desc`, …) throughout both
  `<style>` blocks (e.g. `web/index.html:1293-2135`).
- **Impact:** ~91% of `index.html` is exactly this kind of global/semantic CSS, so the bulk of the
  stylesheet cannot move without either an unidiomatic escape-hatch convention or a full re-architecture
  to component-attached hashed classes (which also means deleting the semantic class names the theme
  engine and e2e selectors rely on).
- **Proposed direction:** A global-rule emission API — e.g. `css.Global(selector, rules...)` /
  `css.Root(rules...)` (un-prefixed, emitted into the same sink), so element/`:root`/semantic-class
  rules can be authored in typed Go and still produce ordinary global CSS.

### CSS2 — No API to emit a `:root` token palette or drive utilities off live CSS variables
- **Area:** theming / design tokens · **Severity:** high
- **Symptom:** `css.Theme` + `UseTheme` is the typed analog of `tailwind.config` and resolves named
  scales (spacing/color/type/radius) **at class-generation time** — it does not emit a `:root { --… }`
  custom-property block, and the utility layer resolves to literal values, not `var(--token)`. CashFlux
  instead themes at *runtime*: `internal/uistate/theme.go:56-65` writes every token via
  `style.setProperty("--…", …)` on `:root` and toggles a `data-theme` attribute, and the inline
  stylesheet's `:root` block is the default palette those vars override. There's no bridge: you can't
  ask `css` to (a) emit the canonical `:root` palette, or (b) make utilities/components reference live
  CSS variables so a runtime token change repaints them. `Dynamic` (`dynamic.go:20-46`) binds *one*
  property to a var per hashed class, but isn't a global token system.
- **Evidence:** `theme.go:5-24,91-99` (Theme/UseTheme swap scales; no var emission);
  `dynamic.go:20-46` (per-class single-var binding only); runtime token engine
  `internal/uistate/theme.go:50-88`; default palette inline at `web/index.html:59-101`.
- **Impact:** Two parallel theming mechanisms with no integration. Moving the `:root` tokens into `css`
  today means either keeping them as raw global rules (blocked by CSS1) or rewiring `theme.go` onto a
  mechanism `css` doesn't expose. This is the crux that keeps the token layer inline.
- **Proposed direction:** A token API that (a) emits a `:root`/`[data-theme]` custom-property palette
  from a typed `Theme`, and (b) lets the utility/`tw` layer resolve tokens to `var(--token)` so a
  runtime `setProperty` reskins without regenerating classes — unifying `css.Theme` with live-variable
  theming.

### CSS3 — No ancestor-attribute-state variant (`[data-theme="light"] &`, `[data-density="compact"] &`)
- **Area:** variants · **Severity:** med
- **Symptom:** The variant set scopes to the element's *own* state (`Hover`, `Focus`, `Active`,
  `WhenDisabled`, structural pseudo-classes) or to an at-rule (`Media`, `Dark`). There is no variant
  for "when an **ancestor** carries attribute X" — i.e. `[data-theme="light"] &` or
  `html[data-density="compact"] &` — which is the entire mechanism behind the light-theme override
  layer and the density system. `DefineVariant(selectorTemplate)` can express it manually, but it's an
  untyped escape hatch the author must hand-write per attribute/value.
- **Evidence:** Variant coverage — `variant.go:9-69` (self pseudo-states + `Media`/`Dark`, no ancestor
  form); manual escape hatch — `variant.go:73-83` (`DefineVariant`). The override layer that needs it:
  the `[data-theme="light"] …` rules at `web/index.html:~581-690+` (legacy-component re-skins) and the
  `data-density` compact rules (referenced from `theme.go:66`).
- **Impact:** The largest *accreted* part of the stylesheet (the GX*/W* light-theme override patches)
  is exactly ancestor-state styling; without a typed variant it can only be ported via repeated
  `DefineVariant` strings, which carries the same typo/specificity risk as raw CSS.
- **Proposed direction:** A typed `Within(selector, rules...)` / `AttrAncestor(name, value, rules...)`
  variant (and a `Theme("light", …)` / `Density(…)` convenience) that emits `<ancestor> &`.

### CSS4 — No cascade-layer / specificity control; override order is registration order only
- **Area:** cascade / ordering · **Severity:** med
- **Symptom:** Emitted classes carry single-class specificity and land in the sink in **registration
  order**; there's no `@layer` support or specificity/ordering primitive. Several inline overrides only
  work because they out-order or out-specify *runtime-injected* rules (the theme engine's
  `setProperty`-backed surfaces), and the only available lever is `MarkImportant` (`rule.go:54`).
  Reproducing the current cascade — where `[data-theme="light"]` shell rules must beat engine-injected
  backgrounds — is fragile when those classes are hash-named and order-dependent.
- **Evidence:** Emission/dedup model — `css.go:33-41`, `registry.go:9-24` ("Sink never sees the same
  class twice", ordered emission); only lever — `rule.go:51-55` (`MarkImportant`). Cascade fights are
  called out in the inline comments, e.g. `web/index.html:~672` ("the theme engine emits runtime
  backgrounds that outrank these shell …").
- **Impact:** Porting the override layer risks subtle visual regressions (light-on-dark bleed) that are
  hard to predict from Go, because the author can't declare "these win" except by sprinkling
  `!important`.
- **Proposed direction:** `@layer` support (named, ordered layers — e.g. `base`/`tokens`/
  `components`/`overrides`) or an explicit layer/priority argument to `New`, so override precedence is
  declared, not order-accidental.

### CSS5 — No managed base/reset (preflight) layer
- **Area:** base styles · **Severity:** low
- **Symptom:** With the Tailwind CDN removed (C91), the app hand-maintains a "minimal Tailwind-preflight
  equivalent" inline. `css` has no opt-in normalize/reset, so the base layer is app-owned and (per
  CSS1) can't even be expressed as global rules in the package.
- **Evidence:** `web/index.html:~98-101` (comment + inline preflight shim); no reset/normalize export
  in the `css` package surface.
- **Impact:** Minor, but the base layer stays inline and unversioned-with-the-framework; every app
  re-pastes a preflight.
- **Proposed direction:** An optional `css.Preflight()` / normalize layer (emitted into a `base`
  cascade layer per CSS4), toggleable so apps that don't want it opt out.

### CSS6 — No documented build-time critical-CSS extraction pipeline
- **Area:** SSR / build tooling · **Severity:** low
- **Symptom:** The pieces for "author in Go, ship inline" exist — the native buffer sink collects
  emissions for SSR and `Seed` suppresses re-injection on hydration — but there's no documented
  build-time path to (a) run a wasm/Go app's render to populate the sink, (b) serialize it into
  `index.html`, and (c) auto-`Seed` at boot. Without it, a migrated sheet either injects at runtime
  (FOUC risk on a render-blocking-critical app shell) or is hand-copied.
- **Evidence:** Sink/seed seam — `registry.go:39-52` (`Seed`/`Reset`), `doc.go:44-46` ("serialize it
  into the page"), wasm-seed tests (`css_wasm_test.go`). No extractor tool is referenced in the
  project or `gwc` runner usage.
- **Impact:** Blocks a clean "lean scaffold + Go-authored critical CSS" end state; the inline block
  stays hand-managed.
- **Proposed direction:** A `gwc`-runner command (or documented harness) that renders the app under the
  native sink, emits a critical `<style>` into the HTML template, and wires `Seed` — closing the loop
  on CSS1–CSS5.

### Cross-cutting (CSS series)

The component, animation, and responsive layers of the inline stylesheet port to `css` cleanly today.
What blocks the *global* stylesheet from leaving `index.html` reduces to two structural gaps —
**global/`:root` rule emission (CSS1, CSS2)** and **ancestor-state + cascade-layer control
(CSS3, CSS4)** — with the base-layer and SSR-extraction items (CSS5, CSS6) as the finishing pieces.
Until CSS1/CSS2 land, migrating the token palette and semantic-class shell means escape hatches or a
full re-architecture, so the recommended sequencing is: close these gaps in the framework first, then
port CashFlux's shell CSS.

---

## Addendum 5 — third raw-interop sweep across all 119 GWC-dependent UI files (G26–G35)

A third, exhaustive pass over every UI-layer Go file that imports the framework (119 files across
`internal/ui`, `internal/screens`, `internal/app`, `internal/uistate`) — searching specifically for
patterns the first two passes missed: layout/geometry reads, async browser-API promise chains,
router lifecycle, id-generation friction, dynamic style/script injection, and the modern Web API
surface (crypto, IndexedDB, Notifications, network status, View Transitions). All evidence below was
opened and confirmed at `path:line`.

| ID  | Area                    | Severity | One-line gap |
|-----|-------------------------|----------|--------------|
| G26 | Layout geometry reads   | med      | No layout-read hook → `offsetLeft`/`offsetWidth`/`getBoundingClientRect` read raw in effects for animated/geometry-aware components (and `Style()` silently drops `--` custom-property keys) |
| G27 | Animation/rAF lifecycle | med      | No `requestAnimationFrame` / View Transitions hook → page-enter restart uses raw double-rAF + `document.startViewTransition` |
| G28 | Router lifecycle        | med      | No per-navigation callbacks → scroll-reset (`scrollTop=0`) and `document.title` set raw on every route change |
| G29 | `UseId()` CSS-unsafe     | med      | `UseId()` emits colon ids (`gwc:3:1`) that throw `SyntaxError` in `querySelector("#id")` → must use `getElementById`; latent wasm panic |
| G30 | Dynamic CSS injection   | med      | No managed `<style>` injection → `@font-face` (custom fonts) and feature stylesheets hand-built with `createElement("style")` + `textContent` |
| G31 | Web Crypto / Promises   | med      | `crypto.subtle` (AES-GCM/PBKDF2) hand-rolled via `js.FuncOf` promise chains; no Promise→Go bridge |
| G32 | Network status          | low      | No connectivity hook → `navigator.onLine` + `window online/offline` wired by hand, listeners leaked |
| G33 | Notifications API       | low      | Browser `Notification` + `requestPermission` wired raw in `syscall/js` |
| G34 | IndexedDB               | med      | No async/blob storage abstraction (G21 stops at localStorage) → full hand-rolled IDB driver (~250 lines) |
| G35 | Imperative canvas/pointer| med     | No pointer/wheel hooks → widget-builder drag/pan/zoom is a ~90-line **JS string literal `eval`'d** from Go |

### Confirmations — documented gaps with notable new evidence
- **G1** — `internal/app/settingssectionnav.go:42`: another explicit per-item component split ("the framework forbids On* inside a variable-length loop") — the settings jump-nav.
- **G2 / G22** — `internal/ui/controls.go:109-128`: `UseId()` + `getElementById` + `querySelector(".seg-pill")` + `offsetLeft`/`offsetWidth` to position the Segmented pill — the strongest missing-DOM-ref instance, in a *reused framework-style UI component* (not just a screen).
- **G3 / G24** — `internal/uistate/fonts.go:83-106` (`createElement("style")` + `textContent` for `@font-face`) and `internal/screens/widget_builder.go:155-177` — two more raw-DOM style/markup bypasses.
- **G9 / G13** — `internal/ui/dismiss.go:107-125`: a reusable popover-dismiss helper over `document.addEventListener("keydown","pointerdown")` + manual `js.Func` release — the missing global-event hook, now wrapped into a shared library.
- **G12** — `internal/ui/controls.go:129`: the single-dep effect workaround used even in a shipped UI component (pill depends on `selected` + `len(options)`).
- **G18** — partially mitigated: `internal/app/dialoghost.go` now implements in-app `ConfirmModal`/`PromptModal`, but native `alert` remains (`internal/app/wsswitcher.go:249`). Gap stands; app-level mitigation exists. (`dialoghost.go:132` also uses raw `setTimeout` → confirms G19.)

### G26 — No layout-geometry read hook; `Style()` also drops `--` custom-property keys
- **Area:** DOM geometry / layout · **Severity:** med
- **Symptom:** Animated/geometry-aware UI reads element layout (`offsetLeft`, `offsetWidth`,
  `getBoundingClientRect`) in a `UseEffect` and writes styles via raw `setProperty`, because (a) there
  is no reactive layout-read primitive and (b) the shorthand `Style(map[string]string)` **silently
  drops keys beginning with `--`**, so even ordinary animated styles must go through `js` `setProperty`.
- **Evidence:** `internal/ui/controls.go:103-128` (Segmented pill: comment "the html `Style()` helper
  drops `--` keys; setProperty via js does not", then `offsetLeft`/`offsetWidth` reads +
  `setProperty`); `internal/app/addmenu.go:30-43` (`getBoundingClientRect` + `innerWidth` to choose
  popover direction).
- **Impact:** Every sliding indicator, auto-placed popover, or measure-then-position UI drops to raw
  DOM. The `--`-key drop is a more severe sub-case of G11 than first noted — it forces `setProperty`
  for *standard* properties too, not just SVG var() cases.
- **Proposed direction:** A `UseElementGeometry(ref)`/`UseLayout(ref)` hook returning measured box
  metrics post-render; and fix/ document `Style()` so custom properties are preserved (or add a typed
  `SetVar` path).

### G27 — No `requestAnimationFrame` / View Transitions hook; page-enter uses raw double-rAF
- **Area:** animation lifecycle · **Severity:** med
- **Symptom:** Restarting a keyframe animation on navigation needs the browser double-rAF idiom
  (remove class → rAF → rAF → re-add), and the app uses the View Transitions API
  (`document.startViewTransition`) — both via raw `syscall/js` with inline `js.FuncOf`s, plus an
  external `IntersectionObserver` living in `wonder.js` that Go cannot see.
- **Evidence:** `internal/app/pageenter.go:11-25` (pattern comment), `:57-82` (`startViewTransition`
  invoke + double-`requestAnimationFrame` fallback, each `FuncOf` created/released inline), `:85-98`
  (`window.cashfluxWonder.observe()` driving a JS-side IntersectionObserver).
- **Impact:** Every SPA route-change animation is raw interop + ephemeral `js.Func` allocation; the
  View Transitions API and observers are entirely outside the framework.
- **Proposed direction:** `UseAnimationRestart(ref, class)` (implements double-rAF) and
  `UseViewTransition(fn)` with graceful fallback and managed `js.Func` lifetime; consider a
  `UseIntersection(ref)` hook to bring scroll-reveal into Go.

### G28 — No router lifecycle callbacks; scroll-reset + `document.title` are raw on every navigation
- **Area:** router / navigation lifecycle · **Severity:** med
- **Symptom:** On route change the app must reset the scroll container (`scrollTop=0`) and set
  `document.title` (tab + SR announcement). Neither is a framework primitive; both run as raw `js` in a
  `UseEffect` keyed on the threaded route prop (and so also inherit G6's non-reactive-route plumbing).
- **Evidence:** `internal/app/focusmain.go:13-42` (`el.Set("scrollTop",0)`, `doc.Set("title",title)`,
  with rationale comment); wired from the route effect in `internal/app/shell.go`.
- **Impact:** Every app re-implements SPA scroll/title hygiene; pairs with G6 — a reactive route + a
  lifecycle hook would absorb both.
- **Proposed direction:** Router lifecycle callbacks (`OnNavigate(fn)` / `useRouteEffect(fn)`) firing
  post-navigation with the new path, plus first-class `title` and scroll-reset options on route config.

### G29 — `UseId()` generates colon ids that crash `querySelector("#id")`
- **Area:** id generation / DSL correctness · **Severity:** med
- **Symptom:** `UseId()` returns ids like `gwc:3:1`. `querySelector("#gwc:3:1")` throws a CSS
  `SyntaxError` (`:` is a pseudo-class separator) and **panics the wasm callback**. All `UseId()`
  lookups must therefore use `getElementById` (raw string, never throws) and must never feed the id to
  any CSS-selector API — a framework-generated footgun.
- **Evidence:** `internal/ui/dismiss.go:45-47` (explicit comment documenting the trap); the
  `getElementById`-not-`querySelector` workaround recurs at `chartd3.go:45`, `mermaidview.go:40`,
  `flippanel.go:55`, `focus.go:14`.
- **Impact:** Silent panic source for any contributor who naturally writes `querySelector("#"+id)`;
  mitigation is pure author discipline.
- **Proposed direction:** Emit CSS-safe ids (e.g. hyphenated `gwc-3-1`), or provide a selector-safe
  escaping helper; document prominently in framework gotchas.

### G30 — No managed `<style>` injection; `@font-face` and feature stylesheets are raw DOM
- **Area:** dynamic CSS injection · **Severity:** med
- **Symptom:** Features needing globally-scoped CSS at runtime (`@font-face` for user-uploaded fonts; a
  widget-builder canvas stylesheet) build a `<style>` element by hand (`createElement` + `textContent`
  + `appendChild`), each with its own id-guard. Distinct from CSS1 (the `css` package's scoping model):
  this is about injecting arbitrary CSS *strings* at runtime, which `css` also cannot do.
- **Evidence:** `internal/uistate/fonts.go:83-106` (`ApplyFonts`, `@font-face`); `internal/screens/widget_builder.go:167-177` (`vbStyleCSS`, guarded by `getElementById("vb-style")`).
- **Impact:** Each such feature re-invents the managed-style-element idiom in raw DOM.
- **Proposed direction:** A `css.Inject(id, cssText)` (idempotent by id) managing the `<style>` element
  lifecycle, so app code never touches the DOM for runtime CSS.

### G31 — Web Crypto (`crypto.subtle`) hand-rolled via `js.FuncOf` promise chains
- **Area:** crypto / async Web APIs · **Severity:** med
- **Symptom:** Dataset encryption (AES-GCM-256 / PBKDF2) calls `crypto.subtle` import/derive/
  encrypt/decrypt as raw Promise chains, each leg a `js.FuncOf` `then`/`catch` pair released by hand.
  No Promise→Go bridge exists, so this is the site where wasm's lack of `async/await` hurts most.
- **Evidence:** `internal/app/datasetcrypto.go:30+` (`subtle` accessor, `getRandomValues`,
  `importKey`→`deriveKey`→`encrypt`/`decrypt`, multiple `js.FuncOf` pairs with `Release()`); envelope
  format in `internal/cryptobox`.
- **Impact:** ~250 lines of pure interop for one feature; any future crypto/signing/hashing repeats it.
- **Proposed direction:** A `wasm/promise` helper wrapping a `js.Value` Promise into a Go channel/
  callback with managed `js.Func` lifetime (eliminates `then`/`catch` boilerplate), plus an optional
  thin `wasm/crypto` wrapper for `getRandomValues`/`subtle`.

### G32 — No network-status hook; `navigator.onLine` + online/offline wired by hand
- **Area:** network / connectivity · **Severity:** low
- **Symptom:** The offline indicator seeds from `navigator.onLine` and stays live via
  `window.addEventListener("online"/"offline")`, with two intentionally-leaked `js.Func`s.
- **Evidence:** `internal/app/onlinestatus.go:17-28`.
- **Impact:** Low (boot-time wiring) but the same G9/G13 pattern.
- **Proposed direction:** `UseNetworkStatus()` → reactive `bool` (builds on G9's `UseWindowEvent`).

### G33 — Browser Notifications API wired raw in `syscall/js`
- **Area:** browser notifications · **Severity:** low
- **Symptom:** Posting notifications checks `window.Notification`, calls `requestPermission().then(…)`
  via a `js.FuncOf`, and constructs `new Notification(...)` — no framework abstraction.
- **Evidence:** `internal/app/notifyrun.go:91-119`; permission flow also at `internal/app/settings.go:107-109`.
- **Impact:** Low individually; part of the "every browser capability = full raw interop" pattern
  (with G31/G32/G34).
- **Proposed direction:** `PostNotification(title, body)` + a general `UsePermission(name)` hook.

### G34 — IndexedDB entirely hand-rolled; no async/blob storage abstraction
- **Area:** async storage / IndexedDB · **Severity:** med
- **Symptom:** The artifact blob store implements a full Go/wasm IDB driver — `indexedDB.open`,
  `onupgradeneeded`/`onsuccess`/`onerror` as `js.FuncOf` callbacks feeding a channel,
  transaction-per-op, plus `navigator.storage.estimate()` for quota — because the framework's storage
  abstraction stops at `localStorage` atoms (G21).
- **Evidence:** `internal/artifactstore/idb.go:28-80` (open/upgrade/success/error), transaction
  pattern throughout; `:199` (`navigator.storage.estimate()`). (`idb_native.go` is the native stub.)
- **Impact:** Any feature needing persistent binary/larger-than-localStorage storage rolls its own IDB
  driver — the largest single browser-API raw-interop module by line count.
- **Proposed direction:** A `wasm/idb` (or `wasm/storage`) typed key-value blob store (open/get/put/
  delete/usage) hiding the callback machinery behind channels, or a first-class `BlobAtom`.

### G35 — Imperative canvas interaction injected as a JS string literal and `eval`'d
- **Area:** canvas / complex pointer interactions · **Severity:** med
- **Symptom:** The widget-builder canvas (node drag, pan, zoom, wiring) is a ~90-line **JavaScript
  string literal** (`vbDragShimJS`) executed via `js.Global().Call("eval", …)`, because Go/wasm has no
  idiomatic way to attach the `mousedown/mousemove/mouseup/wheel/click` document handlers (and capture)
  the interaction needs. The shim also reads `getBoundingClientRect`, writes `style.left/top/transform`,
  and touches `localStorage` directly — all invisible to Go.
- **Evidence:** `internal/screens/widget_builder.go:43-133` (`vbDragShimJS` verbatim JS, incl. the five
  `document.addEventListener` handlers); `:167` (`js.Global().Call("eval", vbDragShimJS)` in a
  `UseEffect`).
- **Impact:** An entire feature's interaction model lives in a JS blob — untestable in Go, un-debuggable
  with Go tooling, and undermining the pure-Go-frontend story. The most extreme raw-interop instance in
  the codebase.
- **Proposed direction:** `UsePointerEvents`/`UseDocumentPointerEvents` (pointermove/down/up with
  capture) and `UseWheel` hooks so drag/zoom canvases are authorable in Go; combines with G2 (ref) and
  G30 (managed injection) to retire the `eval` path.

### Cross-cutting (third pass)

Two themes dominate the new findings and both reduce to a single missing capability each:

1. **No Promise → Go bridge.** `requestAnimationFrame` (G27), `crypto.subtle` (G31),
   `Notification.requestPermission` (G33), and IndexedDB (G34) all repeat the same ~50–250-line idiom:
   manual `js.FuncOf` `then`/`catch` chains with hand `Release()`. A single `wasm/promise` primitive
   (Promise→channel, managed lifetime) would collapse the boilerplate across every modern async
   browser API and is the highest-leverage new fix.
2. **No path back to the rendered DOM's layout/behavior.** Layout reads (G26), router lifecycle
   side-effects (G28), and rich pointer/canvas interaction (G35) all exist because the framework renders
   *out* but offers no measured-geometry / lifecycle / pointer hooks back *in* — the same root as G2
   (DOM ref). Plus a discrete correctness bug: `UseId()` emits CSS-unsafe ids (G29).

Net: **10 new gaps (G26–G35)**, mostly `med`. With Addenda 1–5, the framework's missing-primitive set
now spans: DOM ref + autofocus + geometry (G2/G22/G26), portal + raw-HTML + dialog + injection
(G3/G4/G18/G24/G30), effect-scoped lifecycle for timers/events/media/animation (G9/G13/G19/G20/G25/G27),
router reactivity + lifecycle (G6/G28), a Promise/async-API bridge for crypto/IDB/notifications
(G31/G33/G34), persisted atoms (G21), and the `css` typed-CSS gaps (CSS1–CSS6).

---

## Addendum 6 — behavioral / correctness gaps (G36–G39)

Where Addenda 1–5 cataloged *missing capabilities* (raw-interop escapes, absent APIs), this pass
targeted a different class: **framework behaviors the app must defensively work around** — render/
paint timing, reconciler ownership of the DOM, and state-lifecycle correctness. Found by sweeping the
UI dirs for comment signals (`workaround`/`defensive`/`stale`/`flush`/`deferred`/`re-render`/"the
framework …") and reading the surrounding code. All evidence opened and confirmed at `path:line`.

| ID  | Area                       | Severity | One-line gap |
|-----|----------------------------|----------|--------------|
| G36 | Effect timing (pre-paint)  | high     | `UseEffect` fires before paint → post-render DOM work (focus, scroll, highlight) needs magic `setTimeout`/`rAF` delays; no `UseLayoutEffect` |
| G37 | Reconciler vs `innerHTML`  | high     | The vdom clears imperatively-set `innerHTML` on any self-re-render → components must fold every local state into the effect key to re-inject |
| G38 | Mount-only effect          | low      | No `UseMount`; "run once on mount" is the undocumented `UseEffect(fn, true)` constant-dep idiom |
| G39 | Atom access outside render | med      | `UseAtom` is hook-only → external writers need a render-phase "capture" var + guard; pre-render writes silently drop |

### G36 — `UseEffect` fires before the browser paints; post-render DOM work needs magic delays
- **Area:** effect / lifecycle timing · **Severity:** high
- **Symptom:** Effects run synchronously with render, before the new DOM is painted/laid out. Any work
  that must observe or touch the freshly rendered tree — focusing a just-opened dialog input, scrolling
  a thread to the bottom after content fills, jumping to a highlighted element, restoring focus after a
  list re-renders — cannot run directly in a `UseEffect`. Multiple sites defer via `setTimeout` with
  empirically-tuned delays (30/80/400 ms) or `requestAnimationFrame`.
- **Evidence:** `internal/app/dialoghost.go:121-132` (focus-into-dialog via `setTimeout(30)` in an
  effect — and this is the app's *in-framework* dialog, the G18 mitigation, so it tests the framework's
  own timing); `internal/screens/insights.go:1132-1145` (`scrollChatToEnd` `setTimeout(80)`, comment:
  "deferred … so it runs AFTER the bubbles' Markdown innerHTML has been filled … otherwise … scroll a
  still-empty container and land at the top"); `:1179-1196` (`scrollToID` `setTimeout(400)`);
  `internal/screens/focus.go:58-99` (`focusRowAfterDelete` wraps focus in `requestAnimationFrame` —
  "waits one animation frame so the re-render has repainted the list"), `:109-170` (`captureRowFocus`
  restore, same).
- **Impact:** The most common timing-fragility class in the codebase: dialogs, scroll, focus, and any
  imperative DOM interaction triggered by a state change guess a delay — too short and the element
  isn't there yet, too long and the UI lags.
- **Proposed direction:** A `UseLayoutEffect` hook that fires after DOM mutation but before paint
  (React `useLayoutEffect` / the browser "before paint" slot), so post-render imperative work runs at
  the right moment without magic numbers.

### G37 — The reconciler clobbers `innerHTML` written in an effect; state must be folded into the effect key
- **Area:** reconciler / vdom correctness · **Severity:** high
- **Symptom:** When a component sets a node's `innerHTML` imperatively in a `UseEffect` (e.g. injected
  sanitized Markdown), the next re-render of that component — even from an *unrelated* local state
  change — has the vdom reconciler clear/overwrite that node's children, discarding the injected
  content. The workaround is to fold every local state that can trigger a self-re-render into the
  effect's dependency signature, so the effect re-fires and re-injects after each such render.
- **Evidence:** `internal/screens/insights.go:1044-1058` (`AssistantBubble`: builds `sig := p.Text`
  then appends `|p`/`|c` for `pinned`/`copied` — comment: "folds in the local action toggles so the
  effect re-fills the innerHTML after a self re-render (pin/copy) that the vdom would otherwise
  clear"); `:1224-1230` (`PinnedInsightRow` folds `expanded` for the same reason); `:1431`
  (`UserBubble`, `p.HTML` as key).
- **Impact:** Any component mixing vdom children with imperative-DOM content (innerHTML, canvas,
  third-party libraries) is exposed: a contributor who adds a new local state to such a component
  without knowing this silently breaks the content render. Compounds G3 (no raw-HTML node) — all
  Markdown is forced through this fragile pattern.
- **Proposed direction:** The reconciler should treat a node whose `innerHTML` was set outside its own
  render as opaque (skip patching its children when the app returned none for it); or a first-class
  `RawHTML`/`dangerouslySetInnerHTML` (G3) so the framework owns the write and won't clobber it.

### G38 — No mount-only effect; `UseEffect(fn, true)` is the undocumented "run once" idiom
- **Area:** effect ergonomics / API clarity · **Severity:** low
- **Symptom:** There's no `UseMount(fn)` / `useEffect(fn, [])` equivalent. To run an effect once after
  mount, code passes the constant `true` as the dependency (a constant never changes, so it never
  re-fires). It works but is semantically misleading and undocumented — and silently changes behavior
  if the framework's dep-equality or the dep value is ever altered in a refactor.
- **Evidence:** `internal/ui/flippanel.go:45-50` (`UseEffect(func() func(){ shown.Set(true); return nil }, true)` to trigger the open transition once post-mount) — sole instance, but in a reusable
  framework-style component, and the flip-panel open animation breaks silently if the dep changes.
- **Impact:** Low (one site) but non-obvious and library-level.
- **Proposed direction:** A self-documenting `UseMount(fn func() func())` (run once post-mount, cleanup
  on unmount), or document a canonical mount-only dep form.

### G39 — `UseAtom` is hook-only; external writers need a render-phase "capture" var, and pre-render writes silently drop
- **Area:** state / atom lifecycle · **Severity:** med
- **Symptom:** `state.UseAtom` may only be called during render, so code that drives reactive state from
  *outside* a render (global key shortcuts, undo/redo, post-decrypt hydration, network events) can't
  call it. The pattern: during render, write the hook's atom into a package-level `captured*` variable
  guarded by a `*Captured` bool; external callers use that variable. Writes made **before the first
  render that captures** are silent no-ops.
- **Evidence:** The capture triad recurs across 6+ `uistate` atoms — `internal/uistate/notice.go:28-48`
  (`UseNotice` sets `capturedNotice`/`noticeCaptured`; `PostNotice` "is a no-op until the toast surface
  has rendered once"), `internal/uistate/settings.go:46-69` (`UseDataRevision`/`BumpDataRevision`,
  same), plus `dialog.go:51`, `addtarget.go`, `quickadd.go`, `rail.go`. External callers:
  `internal/app/shortcuts.go`, `internal/app/undo.go:67,80`.
- **Impact:** Three pieces of ceremony per externally-writable atom; "forgot to capture" and "wrote
  before first paint" are real bug classes, and the pre-render silent-drop (a global event firing at
  boot loses its write with no error) is the dangerous one.
- **Proposed direction:** A first-class `state.GlobalAtom(key, default)` / `state.Store` with stable
  identity, readable/writable from any goroutine or callback without a render-phase capture —
  retiring all 6+ capture patterns and the silent-drop risk.

### Confirmations (existing gaps, new evidence)
- **G5** — the `_ = atom.Get()` subscribe-to-force-rerender idiom is more pervasive than Addendum 1
  noted: `internal/app/settings.go:787`, `internal/screens/{artifacts.go:39, workflows.go:37,
  activity.go:188, todo.go:41, reports_screen.go:113, dashboard.go:47}` — 8+ screens.
- **G2 / G22** — `internal/screens/focus.go` (`focusRowAfterDelete`, `captureRowFocus`) are two shared
  utilities encoding the missing-DOM-ref/focus workaround.
- **G12** — `internal/screens/insights.go:1051-1058` folds two local states into one string dep
  because effects take a single dep.
- **G18 / G19 / G22** — even the *new* in-framework dialog host can't escape the older patterns:
  `internal/app/dialoghost.go:132` uses raw `setTimeout`, and native `alert` still remains at
  `internal/app/wsswitcher.go:249`.

### Cross-cutting (behavioral pass)

The dominant theme is **render/paint timing and DOM ownership**. G36 (effects fire pre-paint) and G37
(reconciler clears imperative `innerHTML`) are both `high` and share a root: the framework runs effects
before paint and treats all node children as exclusively vdom-owned — so any component bridging the
framework and the live DOM (dialogs, Markdown bubbles, focus, third-party libraries) must compensate
with delayed callbacks and redundant effect keys. A single `UseLayoutEffect` (G36) plus
opaque-innerHTML/raw-HTML handling (G37/G3) would remove most of it. G39 is a structural state gap
(hook-only atoms can't be driven from global code without a footgun), and G38 is a low-severity
ergonomics item. Net: **4 new gaps (G36–G39)**, two of them `high`.
