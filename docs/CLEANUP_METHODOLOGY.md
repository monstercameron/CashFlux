# Page cleanup checklist

Use this when you clean up a page or a shared element. It is built from the last 300 commits
(`41364603` to `c02afc95`, on `main`, 30–31 October 2026). Those commits were 126 fixes, 76
refactors, 28 features, 26 performance changes, 18 docs, 15 tests and 5 chores. Most of the
work landed in `internal/screens` and the shared `internal/ui` library, with `internal/app`,
`internal/appstate`, `internal/styles`, `internal/uistate` and `internal/i18n` close behind.
Across the range, git counts about 310 commits (merges included) with 107,708 lines added and
18,825 removed in 1,584 files.

Work through the sections in order for each page. Tick each item before you call the page done.
Anything you skip goes in the page's notes as unverified, not as done.

Scope rule: one commit covers one page or one shared element. Do not mix an unrelated area in.
Twice on the Accounts work, a change reached outside the page and had to be pulled back out.

---

## 0. Read the existing docs first

Do not re-derive what these already record.

| Doc | Read it for |
| --- | --- |
| `SPEC.md`, `TODOS.md` | What the page must do, and what is already planned |
| `docs/GOWEBCOMPONENTS.md` | The framework's hooks, routing, and its CRITICAL gotchas |
| `docs/GOWEBCOMPONENTS_GAPS.md` | Framework gaps G1–G13, and the workaround each one needs |
| `docs/COMPONENTS.md` | The inventory of shared components in `internal/ui`, with props |
| `docs/APP_COMPONENTIZATION.md` | Boundaries between views and logic, and the completion checks |
| `docs/PRIMARY_PAGE_PERFORMANCE.md` | How to capture and compare render performance, and past results |
| `docs/TESTING.md` | The two test layers, and which failures are known |
| `DEVLOG.md` | Dated notes on decisions and problems |
| `docs/CLEANUP_METHODOLOGY.md` (this file) | The order of the work, and the rules that came out of it |

Checklist:
- [ ] Read the rows that touch the page's area. Note any gap in the framework doc that affects you.
- [ ] Check whether the page is already in `APP_COMPONENTIZATION.md`, and what it recorded.

## 1. Check the framework and the library before you write code

Most of the hand-written markup and state on earlier pages had a built-in answer. Look first.

- [ ] **The framework source is in the module cache**, at the pinned version (v7.0.1):
      `C:\Users\mreca\go\pkg\mod\github.com\monstercameron\!go!web!components\v7@v7.0.1`.
      Go escapes capitals in module paths (`!g`, `!w`), so search the escaped path. Grep the package
      that defines a helper before you write a new one, for example `ui/form.go` for `UseForm` and
      `html/bind.go` for `BindFunc`.
- [ ] **Search `internal/ui` and `internal/styles` for an existing element or rule.** Use
      `docs/COMPONENTS.md` as the index. If a shared element nearly fits, extend it.
- [ ] **Search the page's own package for a prepared fact** before you compute one in a view.
      Dates, sums, sign rules and eligibility usually already have a pure function.
- [ ] **Read the framework gap the page touches** (`GOWEBCOMPONENTS_GAPS.md`) and use the
      workaround it names. Do not invent a new one.
- [ ] Record any helper you had to write because nothing existed, with the reason, in the commit
      message.

## 2. Inventory and baseline

- [ ] Count the page's raw element calls (`Div`, `Span`, `Button`, `Input`, `Label`, `P`) and its
      shared component calls (`uiw.*`). Record both numbers. The Accounts page went from about 400
      raw calls and 199 shared calls to about 316 and 370. Expect a similar step, not a rewrite.
- [ ] List each repeated pattern: a row, a list, a disclosure, a notice, a form field, a stat tile,
      a toolbar, a dialog. Each one maps to a shared component in section 3.
- [ ] List each local helper (`func` in the page's files). A helper that only builds markup a
      shared component covers should be replaced.
- [ ] Capture a performance baseline for the page with `e2e/perf/primary-pages-benchmark.mjs`
      (see `PRIMARY_PAGE_PERFORMANCE.md`). Keep captures bounded; a capture that never ends hides
      the regression it was meant to show.

## 3. Use the shared components

Build from `internal/ui`. Each element below has a component in the library.

| The page needs | Use | Notes |
| --- | --- | --- |
| A modal or side panel | `FlipPanel`, `Sheet` | Every modal is a FlipPanel. A sheet is the side version. Do not nest a sheet inside a FlipPanel host. |
| A page action or toolbar button | `ToolButton` | One height, radius and text size. `Active` and `Open` carry state. |
| An icon-only button | `IconButton` | Always give a `Label`. |
| A popover or row menu | `OverflowMenu`, `UseMenu` | Items take `Title` for hover text. Sections name themselves for screen readers. |
| A list row | `EntityRow` | `Leading`, `Title`, `Meta`, `Value` (trailing figure), `Actions`, `TestID`, `Attrs`. Title and Meta are plain strings, so badges need their own line. |
| A list with a heading | `EntityListSection` | Use it when the section is not already inside a card. Do not nest a card in a card. |
| A collapsible region | `Disclosure` | A real button with `aria-expanded` and `aria-controls`. `OnToggle` reports changes, so a saved state can persist. |
| A notice, warning or empty state | `Callout` | Tones: info, warn, danger, ok. `Live: alert` for errors, `status` for updates. |
| A set of facts | `KeyValue` | A `<dl>` of label and value. Period figures and reconcile sums. |
| Summary figures | `StatGrid`, `PageHeadline` | One large figure with a plain sentence, then supporting facts. |
| A form field | `Field`, `FormField` | `Field` carries label, hint and error. `FormField` wraps one control. |
| A text, number or money input | `TextInput`, `NumberInput`, `MoneyInput` | `MoneyInput` takes `TestID`, `Placeholder`, `ID` and `Extra`. |
| A dropdown | `SelectInput` | `ID` ties the field's label to the select. |
| One choice from a few | `RadioGroup`, `Segmented` | Roving tab stop and arrow keys come with them. |
| Several choices | `Picker` | Stays open across picks. |
| A toggle | `Toggle`, `ToggleRow` | |
| A money amount | `MoneyFig` | Formatting and tone in one place. |
| A status pill | `Pill`, `Badge` | |
| A filter bar | `ControlStrip`, `FilterToolbar` | |
| A dashboard tile | `Widget` | |

Checklist:
- [ ] Every repeated row, list, disclosure, notice, field, stat and dialog uses a component from
      this table.
- [ ] Page-specific layout that no shared component covers stays as classes in the page's own
      styles file, `internal/styles/rules_<page>.go`.
- [ ] A shared element that lacks a prop the page needs (a test id, a title, an id) gets the prop,
      with a test, rather than a workaround in the page.
- [ ] A new shared element goes in `internal/ui` with a native or wasm test, and a line in
      `docs/COMPONENTS.md`.

## 4. Use the framework's features, not hand-written code

These are the GWC features the page code relies on. Check each one before you write its equivalent.

| Need | Use | Watch for |
| --- | --- | --- |
| A form with typed state | `ui.UseForm` (`ui/form.go`) | One typed draft instead of many state hooks. Copy and clear are single updates. |
| A text field bound to state | `uiw.FieldValue` for text inputs | `BindFunc` (`html/bind.go`) writes the `value` property; `internal/ui/fieldcore.go` explains a stale render can overwrite a newer keystroke. Prefer `FieldValue`. |
| A checkbox bound to state | `CheckedIf` (`html/attrs.go`) | |
| An invalid-field state | `ClassIf` plus `AttrIf` | `ClassIf` overwrites the element's class (section 6). Put the class in the element's one `css.Class` call. |
| Focus on the first error | `ui.UseFocusManager().FocusFirstError` | Focus the first invalid field after a refused save. |
| Announce a result | `ui.UseAnnouncer().Polite` | The region must stay mounted after the modal closes. |
| A unique id for a label or error | `ui.UseId()` | Each form instance gets its own id. |
| A computed value that must not recompute every render | `ui.UseMemo(fn, deps...)`, or `ui.UseMemoOf` for a comparable dependency | Depend on the data revision, not on a wall-clock time (section 5). |
| A handler that keeps its identity | `ui.UseCallback(fn, deps...)` | Pass stable callbacks to children, so they do not re-render. |
| A click or input handler | `ui.UseEvent` | Keep it above any early return. |
| A side effect | `ui.UseEffect(fn, deps...)`, or `ui.UseEffectOf` | |
| A list whose rows change | `MapKeyed`; `MapKeyedComponent` for rows with their own hooks | Keys keep row identity. Never put `On*` options in a plain loop. |
| A lazy branch | `ui.If` | Only the chosen branch is built. `IfLazy` was removed; do not add a copy. |
| More than two branches on one value | `ui.Match()` | Not an `if`/`else if` ladder that assigns a node. |
| Conditional classes | `ClassNames`, `When`, `ClassMap` | |
| A DOM lookup | `gwcdom.go` helpers (`domNode`, `domValue`, `domSet`, `domDocument`) | No raw JS for lookups. |

Checklist:
- [ ] Each form uses `UseForm`, `CheckedIf`, `FocusFirstError` and `Polite`, as the Accounts add form
      does, unless a note says why not.
- [ ] Each id a label or error points to comes from `UseId`, not a fixed string.
- [ ] No raw JS for a lookup, a timer or a listener where a GWC hook covers it.
- [ ] For any framework behaviour you rely on, the source line is cited in a comment or in the
      commit message.

## 5. Performance

The repo's most common performance change was moving computation out of render, into a prepared
value that a native test can check. Measure before and after; keep the change that the numbers justify.

- [ ] **Prepare facts outside the view.** A view reads a prepared value. Sums, sign rules, dates,
      eligibility and sorting belong in a pure package with a native test. The Accounts page, and
      about 40 of the last 300 commits, did this.
- [ ] **Do not read the dataset inside render.** Read once per render and pass the result down. The
      view switch on Accounts used to read the dataset on every frame and was moved off it.
- [ ] **Key every cache on the data revision.** A cache keyed on `time.Now()` never hits. The
      Accounts details cache was keyed on the current time and missed every render. Key on the
      revision, and add the minute only where time changes the answer.
- [ ] **Retry a failed checked read at the same revision.** A failed read should not leave a stale
      cached answer for the rest of that revision (`8647ccf1`).
- [ ] **Keep hidden work hidden.** Build an optional section only when it is shown (`ui.If`). The
      Accounts add form built its owner, debt, asset, advanced and split sections on every render
      before this.
- [ ] **Memoise lists and option sets on the revision.** Option lists for pickers and selects are
      `UseMemo` values keyed on the revision.
- [ ] **Keep children's callbacks stable.** `UseCallback` for handlers passed down, so a child does
      not re-render when nothing it shows has changed.
- [ ] **Read accounts and members once per render**, then pass them down. Handlers still read the
      live list at the moment they run.
- [ ] **Keep remounts cheap.** A page that remounts with an unchanged revision should reuse its
      caches (`dba92044`, Dashboard).
- [ ] **Measure.** Run the primary-page benchmark before and after. Record the numbers in the commit
      or in `DEVLOG.md`. A change with no measured gain needs a reason to stay.

## 6. GWC pitfalls

These cost real time on the Accounts page. Check every page against them.

- [ ] **Class options overwrite each other.** `css.Class`, `html.Class` and `html.ClassIf` each set the
      element's whole class attribute, so two class options on one element leave only the last. Merge
      them into one `css.Class(base, extra...)` call. For a condition, use a helper that returns the
      name or `""` (`condName` in the Accounts screens; promote it to `internal/ui` when a second page
      needs it).
- [ ] **Search the page for double class options.** The Accounts audit found several, including one
      that dropped the base classes of every row with an institution.
- [ ] **A later rule can win over a component's class.** The Budgets button rules (`.budget-limit-btn`)
      set a card background, and a later rule set the card colour. A compound selector
      (`.acct-row-figure.budget-limit-btn`) wins reliably. Check the computed style in the browser, not
      only the class name.
- [ ] **Amount tone rules lose the same way.** `.amount-income` and `.amount-expense` were overridden on
      the Accounts balance. Use a compound selector for the page's figures.
- [ ] **A pre-paint layout rule can shrink content.** The boot splash's grid centred the loading
      skeleton and shrank it to its content width when the Go stylesheet arrived. The skeleton shell
      now sets `width: 100%` in the inline boot CSS.
- [ ] **Hooks run before any early return.** A hook after an early return changes the hook count
      between renders. The screenlint hook baseline (`internal/screenlint/testdata/hook_baseline.txt`)
      checks this. A baseline entry for a fixed function should be removed.
- [ ] **Keep the dev server from reloading in a loop.** Start it with `-hot=false` (section 13).

## 7. Styling

- [ ] Rules live in `internal/styles/rules_<page>.go`, built from the typed constructors in
      `props_gen.go`. `prop(...)` only for a property with no constructor yet.
- [ ] Text sizes use `--type-*`, headings `--heading-*`, controls the shared button tokens. No
      hard-coded sizes or colours where a token exists.
- [ ] Z-index comes from the scale on `:root`. Managed overlays take their z-index from GWC's stack;
      verify any layer with `getComputedStyle`.
- [ ] Only plain hex colours from data reach the stylesheet. Validate institution and user colours
      first.
- [ ] A rule file is registered from `install.go`. A file that is never registered does nothing, and
      the page looks wrong without any error.
- [ ] Remove the page's dead rules when their markup goes.
- [ ] Inline styles are a last resort. The one exception on the Accounts page is a data-driven colour
      (an institution swatch), which goes through a custom property.

## 8. Markup

- [ ] `If`, `Unless`, `IfElse`, `Show` for a single cheap expression. Both branches of `IfElse` are
      built before the call, so keep them small.
- [ ] `ui.If` when a branch needs statements, a guard, or work that should not run when hidden.
- [ ] No `var x ui.Node = Fragment()` followed by an `if` that assigns `x`. Use `ui.If` or `If`.
- [ ] `Map` for lists; `MapKeyed` when items reorder or change; `MapKeyedComponent` when a row has
      its own hooks or handlers.
- [ ] Build rows as components so `On*` options never sit in a variable-length loop.
- [ ] Comments describe the current design. No ticket tags, no "used to" lines.

## 9. Data and logic (for any page that writes)

These came from the Accounts review rounds. Each one was a real defect in a write path.

- [ ] **Read through the checked accessors.** The unchecked accessors (`a.Accounts()`,
      `a.Transactions()`, `a.Recurring()`, `a.Goals()`) return empty on a storage error. A money,
      proposal or deletion decision must use the `*Checked` version and return the error.
- [ ] **Every writer checks the role and readiness.** `roleGuard` and `ready()` run before any write.
      Account groups had neither until the review.
- [ ] **A change over several records is one commit.** One `ReplayChanges` (or the package's
      equivalent), so a failure leaves nothing half-written.
- [ ] **Failures are returned, not reported as success.** A save that closes its form on failure, or a
      sweep that posts a success notice when nothing moved, is a bug.
- [ ] **Refusals leave the stored value alone.** A value the store refuses must not replace the stored
      one in the form, and must not reach a device credential store.
- [ ] **Guard against double work.** Check for a live reference before deleting a category, and for a
      duplicate before a write (the recent guard fixes for categories and pay-yourself-first).
- [ ] **Dates are calendar days in the caller's zone.** Compare calendar days, not instants or UTC
      midnights, when a rule is about a day. Test in a fixed zone east of UTC and one west of it with
      `time.FixedZone`.
- [ ] **Money stays in integer minor units**, with checked arithmetic, and is never a float.
- [ ] **An import or restore never refuses on one optional field.** A refused import at launch can lock
      a user out of their data. Drop the bad field, keep the rest, and log it.
- [ ] **Validate once, in one place.** A rule the form and the store both enforce comes from one
      function, so they cannot disagree.

## 10. Forms

- [ ] The add form and the edit form use the same save rules, from one pure function that returns
      typed issues.
- [ ] A save error is visible without scrolling: focus or scroll to the first invalid field, and clear
      the message when the field changes.
- [ ] Step and validation follow the currency's decimals. A 3-decimal currency cannot be blocked by a
      `0.01` step, and rates need a finer step than money.
- [ ] `novalidate` on the form, so the app's own messages decide, not the browser's tooltips.
- [ ] Hidden fields are not validated. An Advanced-only field must not block a save while its section
      is closed.
- [ ] No field silently drops a value. A negative credit limit or a bad date is flagged.

## 11. Accessibility

- [ ] Every icon-only control has a name (`IconButton` `Label`, or `ToolButton` `Title`/`AriaLabel`).
- [ ] Repeated buttons name their subject ("Edit Chase", not "Edit").
- [ ] Every input has a label, and `SelectInput` takes an `ID` so the label is tied to it.
- [ ] Errors use `Callout` with `Live: alert`; updates use `Live: status`. Keep buttons out of the live
      region; the message goes in it, the buttons beside it.
- [ ] Focus stays on the control that was used, or moves to the next logical one.

## 12. Tests

The layers are in `docs/TESTING.md`. Use them this way:

- [ ] Pure logic: a native `go test` in its package, table-driven, one table per rule.
- [ ] DOM behaviour: a wasm test (`*_wasm_test.go`). Run one package per `tools/wasm-test.ps1` call,
      with a `-Run` filter.
- [ ] Assertions name the exact error or value. A test that only checks `err != nil` passes for the
      wrong error.
- [ ] Check that a test would fail if its behaviour regressed. Read the assertion, not only the name.
- [ ] Failure injection for every read and every write on the error path. A closed store fails every
      read at once and proves less than an injected failure of one read.
- [ ] A changed test id or tag gets its assertion updated; the behaviour check stays.
- [ ] Known pre-existing failures are excluded by name, not by skipping a package.

## 13. Browser check

Measure and read. Do not judge from one screenshot.

- [ ] Start the dev server with hot reload off:
      `./.tools/gwc.exe dev -app ./main.go -root ./web -html ./web/index.html -wasm ./web/bin/main.wasm -hot=false -no-doctor`.
      With hot reload on, the page reloads in a loop and never settles.
- [ ] Load the root and navigate in-app. Deep links may not resolve in the dev server.
- [ ] If the page stalls on its splash, bring the tab to the front (`tabs_select`) and wait. A hidden
      tab's boot can stop partway.
- [ ] Exercise every control: each button, menu item, tab, filter, toggle, form submit and cancel. Open
      each modal and disclosure. Read the console after each.
- [ ] Read computed styles for anything you changed (colours, backgrounds, widths, padding), not only
      class names.
- [ ] Check a phone width (`resize_window` preset `mobile`), then reset to `desktop`. Look for
      horizontal scroll and for content under the floating action button.
- [ ] Record anything not checked as unverified.

## 14. Ship

- [ ] A `CHANGELOG.md` line under `Unreleased`, in the same commit.
- [ ] A `DEVLOG.md` entry for meaningful work: what changed, the decision, the problem, what is next.
- [ ] A version bump in `internal/version/version.go` per commit.
- [ ] One area per commit. Stage only the page's files, and check `git status` before committing.
- [ ] Commit on the branch. Merge to `main` only when the user asks. Do not push unless asked.
- [ ] The commit message ends with the required `Co-Authored-By` line.

---

## Standards already in place

- **Modals:** every modal is a `ui.FlipPanel`, built on GWC's managed overlay, which traps Tab, handles
  Escape and outside clicks, and restores focus. The modal shell has `NoFooter`, `FlushBody`, a
  scrolling `.modal-scroll` body and a pinned `.modal-foot` bar. One backdrop, styled once in
  `.flip-backdrop`. Stacking comes from GWC's overlay stack, not from CSS.
- **Cancel is not an empty answer.** `PromptModalResult` returns an ok flag, so cancelling a prompt
  does not clear the stored value.
- **Z-index:** one scale on `:root`: `--z-base`, `--z-raised`, `--z-sticky`, `--z-dropdown`,
  `--z-overlay`, `--z-popover`, `--z-modal`, `--z-dialog`, `--z-toast`, `--z-banner`, `--z-lock`, plus
  `--z-lift` (30), `--z-cell` (40), `--z-row-menu` (51) and `--z-row-active` (61).
- **Route DOM work through GWC.** Menus, pickers and the filter toolbar use GWC hooks for listeners,
  timers and frames.
- **Measure before placing.** A popover measures its settled height before it opens.
- **Shared menu state:** `UseMenu` drives the More menu, the kebabs and the row menus.
- **Comments** describe the current design. A comment that claimed behaviour the code does not have
  is corrected or removed.

## Lessons from the bugs

| Bug | How it was caught |
| --- | --- |
| Expanded account row overflowed its width by 28px | Measured the details block against its row |
| Enter on the document crashed the app | Checking Enter-key shortcuts in the browser; guarded the target in `shortcuts.go` |
| Cancelling set-institution cleared the institution | Cancelled in the browser and read the stored value |
| Overlay tests failed with "call to released function" | The renderer does not unmount between tests; `unmountFlipPanel` added to the FlipPanel tests |
| FlipPanel initial focus landed on the header close button | Focus checked after open in the browser; bounded retry added |
| Shadow around the flip modal clipped to a hard edge | Zoomed in; a probe showed the 3D layer clipping the shadow |
| Confirms could sit under a modal opened from another modal | Found in code; the stack already orders nested overlays |
| Account balances lost their green and red tone, and got a dark box | Computed styles showed a later Budgets rule winning on the same class |
| Every account row with an institution lost its base classes | Two class options on one element; the last overwrote the first |
| The loading skeleton shrank when the stylesheet arrived | Sampled the skeleton's width during boot; a grid rule centred it |
| The account details cache never hit | Its key used the current time, which changes on every render |
| A sweep could report success for a replay that moved nothing | Reviewed the replay paths against the stamps and windows |
| A sweep could miss a due date west of UTC | Tested in fixed zones on either side of UTC |
| Group membership changes were not atomic and had no role check | The round-eight review of the write paths |
| A sweep could overdraw a source if the ledger read failed | Found in review; the read is now checked and returns the error |

## Open items

- **Nested-confirm stacking** is not checked in the browser. Check a confirm opened from inside a modal
  by hand.
- **Ticket-tagged test filenames** remain in about 300 Go files (`_bc4b2`, `_tv8f`, `_br3dlg`, `_uxt5`
  and others). Renaming needs a name for each file, so it is a separate pass.
- **Raw JS remains** in `internal/ui` (`fieldcore.go`, `anchor.go`, `controls.go`, `inlineeditform.go`,
  `overlay_anchor.go`, `chartd3.go`, `bentoflip.go`, `datatable_virtual.go`, `mermaidview.go`), in
  `internal/app` (`applockgate.go`, `shell.go`), and in other overlays.
- **Inline styles and `cssText`** remain in about 46 non-test files, for example `budgets_row.go` and
  `budgets_income_alloc.go`. Move them to typed rules as each area is touched.
- **`condName` and `fieldInvalidName`** live in the Accounts screens. Promote them to `internal/ui` when a
  second page uses them.
- **Accounts:**
  - The AI key and base URL guard is not on `main`. It is saved as a patch in
    `scratchpad/ai_guard_saved/`, and its review found an import lockout. It needs rework before it lands.
  - A clean reviewer round on the Accounts sweep and group changes has not yet happened.
  - The chevron on each row sits under the badges, and "Mark all updated" appears twice.
- **Pre-existing failures outside this work:**
  - `internal/loadgen` does not build (`backendrpc.JSONCallOptions` is undefined).
  - `go vet` on `internal/screens` reports a struct tag in `ag_reports_edit.go:78`.
  - `TestTV7CTransactionPageCalendarToggleShowsLiabilityDueDate` depends on the date.
  - `TestAutoMatchBillsOnPost` and `TestAutoMatchBillsAmbiguousLeftAlone` depend on the date.
  - `TestPrimaryNetWorthDashboardKpiAndHouseholdDisclosure` fails only in a full package run; an earlier
    test leaves state behind. It passes on its own.
- **Unused code kept deliberately:** `FlipPanel.LeftFooter` has no callers; `DialogRequest.Title` is
  exported but unused by callers.
- **Full app package** has not been run on the latest code, only filtered subsets.

## Where the record lives

- `CHANGELOG.md` under `Unreleased`: one line per commit.
- `DEVLOG.md`: dated decisions and problems.
- `git log 41364603^..HEAD`: the commit history for this range.
- `docs/` (see section 0): the framework, componentization, performance and testing docs.
- `internal/screenlint/testdata/hook_baseline.txt`: the hook-order baseline.
