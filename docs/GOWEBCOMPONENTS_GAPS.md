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
