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
- **Stacking comes from GWC's overlay stack, not from CSS.** A managed overlay's backdrop gets its
  z-index inline from the stack, and a deeper overlay (a confirm opened from a modal) sits above
  the one below it. The `Topmost` flag added in `5334b72c` had no effect, because the inline
  z-index wins over any class. It was removed afterwards. FlipPanel does not set z-index.
- **Cancel is not an empty answer.** `PromptModalResult` returns an ok flag, so cancelling the
  set-institution prompt no longer clears the institution.

### Z-index

- **One scale on `:root`** (`a4f1de8f`): `--z-base`, `--z-raised`, `--z-sticky`, `--z-dropdown`,
  `--z-overlay`, `--z-popover`, `--z-modal`, `--z-dialog`, `--z-toast`, `--z-banner`, `--z-lock`.
- **Use a token for any layer above the page.** A value local to one component (ordering
  siblings inside one card, for example) may stay a literal.
- **Managed overlays are the exception.** FlipPanel and the other GWC overlays take their z-index
  from GWC's stack, so a token on them does nothing. The `--z-modal` and `--z-dialog` values only
  apply to layers the app positions itself, such as the app-level banners and the lock gate.
  Verify with `getComputedStyle` in the browser; an inline `z-index` on the element overrides the
  class.

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

### Shared UI elements

Build a page from these, not from a copy of one. Each lives in `internal/ui` and has a native or
wasm test where its logic can be tested on its own.

- **`FlipPanel`** (`flippanel.go`): every modal. See the Modals section.
- **`ToolButton`** (`toolbutton.go`): every page action and toolbar button. The Filters button is the
  reference, so every button shares one height, radius and text size (`b3e84e9a`, `ced94b7f`).
- **`RadioGroup`** (`radiogroup.go`, with `radiogroup_pure.go`): one choice from a short list. It has
  radiogroup semantics, a roving tab stop, and arrow-key movement. `RadioKeyTarget` is pure and
  tested natively. The selection slides between options, and the slide stops under reduced motion
  (`58529b4a`, `8b87e5e3`, `79411bed`).
- **Multi-select** (`picker.go`): several choices from a list, styled from tokens and kept open
  across picks (`e6878dc9`). The sheet's position comes from `pickerPanelPlacement`.
- **`UseMenu` and `MenuControl`** (`menushell.go`): the state of every popover menu. The More menu,
  the kebabs and the row menus share it (`b9a9245b`). The menu stays mounted through its fade-out.
- **`UsePanelFade` and `FadeClass`** (`panelfade.go`): open and close animations for panels, with
  reduced motion respected.
- **`PageHeadline`, `PageHeadlineTrend`, `PageHeadlineFigure`, `PageHeadlineFact`**
  (`pageheadline.go`): the summary that opens a list page. It reads in the same order on every page:
  one large figure with a plain sentence beside it, then an optional gauge and supporting facts.
  Budgets and Accounts share it (`31d144e8`).
- **GWC DOM helpers** (`gwcdom.go`): `domNode`, `domValue`, `domSet`, `domDocument`. Use them for any
  lookup in the browser, so the code does not touch raw JS.

When a new control is needed, look here first. If a shared element nearly fits, extend it rather than
making a second one.

### Typed CSS

Styles are Go, not strings.

- **Rules live in `internal/styles/rules_*.go`**, one file per area. A rule is `rule(selector, decls...)`.
  Responsive rules use `ruleMedia(query, selector, decls...)`, and `ruleContentMax` is for the
  content-width breakpoints.
- **Declarations use the typed constructors in `props_gen.go`**, such as `display`, `gap`,
  `zIndex` and `gridTemplateColumns`. Use `prop("name", "value")` only for a property that has no
  constructor yet, and add a constructor when a second file needs it.
- **Unused constructors are removed** when their last use goes (`2119d707`).
- **`rawBlock` is for CSS the typed helpers cannot say**, such as print rules. Do not use it for
  ordinary declarations.
- **Sizes come from tokens.** Text sizes are `--type-*` (`a8db1584`), headings are `--heading-*`
  (`ba423ee0`), and controls use the shared button tokens (`ced94b7f`). Colours and sizes are not
  hard-coded where a token exists.
- **Z-index comes from the scale** (see the Z-index section).
- **Only plain hex colours reach the stylesheet** from data. Institution colours are validated first
  (`711beb4a`), so user data cannot inject CSS.

**Done on the Accounts pages:** the static inline styles in `accountaddform.go` are typed rules
(`.acct-add-wide` and `.acct-liab-toggle` in `rules_accounts.go`). One inline style remains on
purpose: the row's left border in `accounts_row.go` takes the institution's colour from data.

**Still open outside the Accounts pages:** inline `style` attributes and `cssText` strings remain in
46 non-test files (94 occurrences), for example `budgets_row.go` and `budgets_income_alloc.go`.
Move them to typed rules as each area is touched.

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

**Done on the Accounts pages:** the assign-in-an-`if` sites in `accounts_edit_form.go`,
`accounts_forward.go`, `accounts_recurring.go`, `accounts_row.go` and `accounts_tiles.go` now use
`ui.If` or `ui.Match()`. The switch that chose the body in `accounts_tiles.go` is a `Match` too.
Each was checked by the screens tests; the browser check is still to run.

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
| Confirms could sit under a modal opened from another modal | Found in code. The fix attempted with a `Topmost` class did nothing: the browser showed GWC's inline z-index on the backdrop. The stack already orders nested overlays, as GWC's overlay test asserts |

## Open items

- **Nested-confirm stacking is not checked in the browser.** The stack's order is covered by GWC's
  overlay test, not by a nested confirm in the app. A confirm opened from inside a modal should be
  checked by hand.
- **Overflow menu section names** were not part of an entry's accessible name, so a screen reader
  could not tell which group an entry was in. Each entry now names its section as its accessible
  description (`aria-describedby`), and each run of entries gets its own heading id. The heading
  stays hidden from the accessibility tree, so it is not read twice.
- **Ticket-tagged test filenames, outside the four UI files, remain.** The four UI test files are
  renamed: `overflowmenu_focus_wasm_test.go`, `overflowmenu_entries_wasm_test.go`,
  `flippanel_closeonly_wasm_test.go` and `flippanel_title_wasm_test.go`, and their test ids are
  descriptive. About 300 other Go files still carry tags such as `_bc4b2`, `_tv8f`, `_br3dlg` or
  `_uxt5` in their names, in `internal/app`, `internal/screens`, `internal/styles`, `internal/i18n`,
  `internal/appstate` and others. Renaming them needs a name for each file, so it is a separate pass.
- **Raw JS remains** in `internal/ui` (`fieldcore.go`, `anchor.go`, `controls.go`,
  `inlineeditform.go`, `overlay_anchor.go`, `chartd3.go`, `bentoflip.go`, `datatable_virtual.go`,
  `mermaidview.go`), in `internal/app` (`applockgate.go`, `shell.go`), and in other overlays such as
  the command palette, help overlay and app lock gate.
- **Z-index literals:** the mid-range layers in the stylesheet now use named tokens with their
  existing values: `--z-lift` (30), `--z-cell` (40), `--z-row-menu` (51) and `--z-row-active` (61),
  alongside the existing `--z-sticky` and `--z-overlay`. Component-local values (0–6, and the
  popover at 20 in `rules_tv8c.go`) stay literal, since they only order siblings inside one component.
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
