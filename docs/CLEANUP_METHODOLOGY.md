# Cleanup methodology: the last 73 commits

This records how the Accounts page and the shared UI were cleaned up across the last 73
commits (`8698520b` to `a4f1de8f`, 30–31 October 2026). It covers the method, the standards that
came out of it, and what is still open. Read it before starting similar work.

## Scope

- **73 commits**, touching **113 files**: about 4,900 lines added and 3,700 removed.
- **Commit types:** 29 `fix`, 24 `refactor`, 15 `feat`, 4 `style`, 1 `perf`.
- **Most of the range** rebuilt the Accounts page and the shared pieces it depends on: menus,
  filters, the radio group, the multi-select, the toolbar and the page headline.
- **The final stretch** (`1b133396` to `a4f1de8f`) was the cleanup pass proper: one modal
  layout, one modal component, one backdrop, one z-index scale, and a comment and code review.

## Method

The same loop ran for almost every change:

1. **Read the code and the spec first.** Check `SPEC.md`, `TODOS.md` and the framework notes in
   `docs/GOWEBCOMPONENTS.md` before changing behaviour. Model the data, then the logic, then the UI.
2. **Make one change in one area.** Commit each finished change on its own, with a `CHANGELOG.md`
   line under `Unreleased` and a version bump in `internal/version/version.go`.
3. **Check it in the browser.** Open the real page, exercise the control, and read the console.
   Every kebab option was tested, not just the first. Layout changes were measured on the page
   (computed styles, bounding boxes), not judged from a screenshot alone.
4. **Add a test where the bug lived.** Logic goes in a pure package with native tests. DOM
   behaviour goes in a wasm test. Run one package per `tools/wasm-test.ps1` call.
5. **Review in passes.** The comment and code review ran as four parallel reviewers on disjoint
   file groups. Each reviewer's change was read as a diff before it was accepted. Changes that
   altered behaviour were tested again, not taken on trust.
6. **Stop at what is verified.** Anything not checked in the browser is written down as unverified,
   not as done.

## Standards introduced

These are the rules a new change should follow. Most are already in the code.

### Modals

- **One modal shell.** Every flip-style modal is a `ui.FlipPanel`. It builds on GWC's managed
  overlay, which traps Tab, handles Escape and outside clicks, and restores focus. No raw JS
  focus or key handling.
- **The Update value layout** (`1b133396`). A modal is a titled panel with `NoFooter: true` and
  `FlushBody: true`, a scrolling `.modal-scroll` body, and a pinned `.modal-foot` bar with Cancel
  and the primary action. Confirms, prompts, the credential gate and the vault form follow it.
- **One backdrop.** The backdrop comes from `flipBackdropClass`, and its look lives in one
  `.flip-backdrop` rule set (`2119d707`). The card's shadow sits on the wrapper, because the
  flip's 3D layer clips a shadow on the face into a hard-edged box.
- **Confirms sit above modals.** `FlipPanelProps.Topmost` lifts a backdrop to `--z-dialog`. The
  confirm host sets it (`5334b72c`).
- **Cancel is not an empty answer.** `PromptModalResult` returns an ok flag, so cancelling the
  set-institution prompt no longer clears the institution.

### Z-index

- **One scale on `:root`** (`a4f1de8f`): `--z-base`, `--z-raised`, `--z-sticky`, `--z-dropdown`,
  `--z-overlay`, `--z-popover`, `--z-modal`, `--z-dialog`, `--z-toast`, `--z-banner`, `--z-lock`.
- **Use a token for any layer above the page.** A value local to one component (ordering
  siblings inside one card, for example) may stay a literal.

### Browser and GWC

- **Route DOM work through GWC.** Menus, the picker and the filter toolbar use GWC hooks for
  listeners, timers and frames, and its interop wrapper for lookups (`77560186`). FlipPanel has
  no raw JS (`9ac867a2`).
- **Measure before placing.** A popover measures its settled height before choosing where to
  open (`94fe2157`). A menu opens on the same side every time (`3515055f`).
- **Shared menu state.** `UseMenu` drives the More menu, the kebabs and the row menus
  (`b9a9245b`).

### Shared components over copies

- Tool button for every page action (`b3e84e9a`).
- Radio group, with a sliding selection (`58529b4a`, `8b87e5e3`).
- Multi-select with token styling (`e6878dc9`).
- Page headline shared across Budgets and Accounts (`31d144e8`).

### Styling

- **Typed style rules.** Rules live in `internal/styles/rules_*.go`, built from the typed
  property constructors in `props_gen.go`. Unused constructors are removed (`2119d707`).
- **Tokens for sizes.** Text sizes come from the `--type-*` scale (`a8db1584`), heading tokens
  from one place (`ba423ee0`), and controls from shared tokens (`ced94b7f`).
- **Unused CSS is removed when found.** Dead dialog-scrim and dialog-backdrop rules were removed
  once nothing used them (`2119d707`).

### Render helpers

Markup is built with the framework's helpers, not with Go `if` blocks around a variable.

- **`If`, `Unless`, `IfElse`, `Show` (shorthand, eager)** for a single expression that is cheap
  and needs no guard. Both branches of `IfElse` are built before the call, so keep them simple.
- **`ui.If` (lazy, `ui/branching.go`)** when a branch needs statements, a guard, or work that
  should not run when hidden. It takes `NodeFactory` closures, so only the chosen branch is built.
  `IfLazy` was a local copy of this and was removed in `472d21d8`. Do not add another.
- **`ClassNames` and `When`** for conditional classes. `ClassMap` is the same from a map.
- **`Map` and `MapKeyed`** for lists. Use `MapKeyed` when items can reorder or change, so each
  row keeps its identity. Use `MapKeyedComponent` when a row needs its own hooks or handlers,
  because it gives each row its own hook state.
- **`Match()` chains** when more than two branches depend on one value. Do not write an
  `if` / `else if` ladder that assigns a node.
- **Keep `On*` options out of variable-length loops.** Use `MapKeyedComponent` for per-row handlers.

Anti-pattern: `var x ui.Node = Fragment()`, then `if cond { x = ... }`, then `x` used in markup.
Replace it with `ui.If` or `If`. This was done for the Accounts row and its notes, valuation change
and delta (`22b9fedb`, `78c67d88`).

**Still open:** about 11 sites on the Accounts pages keep the assign-in-an-`if` pattern:
`accounts_edit_form.go` (balances, line), `accounts_forward.go` (read),
`accounts_recurring.go` (body), `accounts_row.go` (projection, driver, details, chart) and
`accounts_tiles.go` (body content, class segment). Each needs a browser check after it changes.

### Comments

- A comment describes the current design, not the history. No ticket tags, no "used to" lines.
- Any comment that claimed behaviour the code does not have was corrected or removed
  (`a35342bf`).

### Commits

- One feature or fix per commit, with a `CHANGELOG.md` line under `Unreleased` in the same commit.
- A version bump in `internal/version/version.go` per commit.
- Commit locally. Do not push unless the user asks.

## Bugs found on the way, and how they were caught

| Bug | How it was caught |
| --- | --- |
| Expanded account row overflowed its width by 28px | Measured the details block against its row |
| Enter on the document crashed the app | Found while checking Enter-key shortcuts in the browser; guarded the target in `shortcuts.go` |
| Cancelling set-institution cleared the institution | Cancelled in the browser and read the stored value |
| Dialog Enter cancelled after a footer rewrite | Enter test; restored the `OnSave` path |
| Overlay tests failed with "call to released function" | The renderer does not unmount between tests; `unmountFlipPanel` added to the FlipPanel tests |
| FlipPanel initial focus landed on the header close button | Focus checked after open in the browser; bounded retry added |
| Shadow around the flip modal clipped to a hard edge | Zoomed in the browser; a probe showed the 3D layer clipping the shadow |
| Confirms could sit under a modal opened from another modal | Read the z-index scale in code; not yet confirmed in the browser |

## Open items

- **Browser checks not completed.** The nested-confirm stacking and the final Set institution prompt
  after the shadow fix were not checked in the browser. The dev page stopped responding to clicks
  after hot reloads. A clean restart of the dev server would be the first step.
- **Overflow menu section names** are not part of the item's accessible name. This is an
  accessibility gap for a decision by the user.
- **Ticket-tagged test files and ids** remain: `overflowmenu_tc4n11_wasm_test.go`,
  `overflowmenu_uxt4_wasm_test.go`, `flippanel_imv2_wasm_test.go`, `flippanel_tv8f_wasm_test.go`
  and the test id `tv8f-flip-body`. Renaming needs `git mv`.
- **Raw JS remains** in `internal/ui` (`fieldcore.go`, `anchor.go`, `controls.go`,
  `inlineeditform.go`, `overlay_anchor.go`, `chartd3.go`, `bentoflip.go`, `datatable_virtual.go`,
  `mermaidview.go`), in `internal/app` (`applockgate.go`, `shell.go`), and in other overlays such as
  the command palette, help overlay and app lock gate.
- **Component-local z-index literals** (1–6, 30–100) remain in the stylesheet, by design.
- **Pre-existing failures outside this work:**
  - `internal/loadgen` does not build (`backendrpc.JSONCallOptions` is undefined).
  - `go vet` on `internal/screens` reports a struct tag in `ag_reports_edit.go:78`.
  - `TestTV7CTransactionPageCalendarToggleShowsLiabilityDueDate` depends on the date.
- **Unused code kept deliberately:** `FlipPanel.LeftFooter` has no callers; `DialogRequest.Title`
  is exported but unused by callers.
- **Full app package** has not been run on the latest code, only filtered subsets.

## Where the record lives

- `CHANGELOG.md` under `Unreleased`: one line per commit.
- `git log 8698520b^..HEAD`: the commit history for this range.
- `docs/GOWEBCOMPONENTS.md`: framework notes that the standards above rely on.
