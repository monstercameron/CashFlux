### GM2. Add/Edit entity modals — UX review — 2026-06-23 ★

**The story**
A user opens CashFlux and wants to add several things in a session: an account, a budget, a goal,
and then log a transaction. She clicks the "+" in the top bar, picks an entity, fills the modal
form, and saves. She also double-clicks a transaction row to fix a description in the inline edit
form. She uses light mode on a 1280px desktop; her partner uses dark mode on a 768px tablet.

---

**Drive script**
`e2e/gm_02_addedit.mjs` — opens the +Add menu and exercises: Add transaction (QuickAdd panel),
Add account, Add budget, Add goal (FlipPanel modal via AddHost), and the inline edit form on a
transaction row. Screenshots at 1280 + 768 × dark + light.

```
node e2e/gm_02_addedit.mjs   # against :8099 (gwc dev server)
EXIT: 0, no warnings
```

**Screenshots produced (31 files)**
- `gm_02_addedit_dark_1280_baseline.png`
- `gm_02_addedit_dark_1280_addmenu_open.png`
- `gm_02_addedit_dark_1280_add_account.png`
- `gm_02_addedit_dark_1280_add_budget.png`
- `gm_02_addedit_dark_1280_add_goal.png`
- `gm_02_addedit_dark_1280_add_transaction.png`
- `gm_02_addedit_dark_1280_inline_edit.png`
- `gm_02_addedit_dark_768_baseline.png`
- `gm_02_addedit_dark_768_addmenu_open.png`
- `gm_02_addedit_dark_768_add_account.png`
- `gm_02_addedit_dark_768_add_budget.png`
- `gm_02_addedit_dark_768_add_goal.png`
- `gm_02_addedit_dark_768_add_transaction.png`
- `gm_02_addedit_dark_768_inline_edit.png`
- `gm_02_addedit_dark_768_add_account_responsive.png`
- `gm_02_addedit_light_1280_baseline.png`
- `gm_02_addedit_light_1280_addmenu_open.png`
- `gm_02_addedit_light_1280_add_account.png`
- `gm_02_addedit_light_1280_add_budget.png`
- `gm_02_addedit_light_1280_add_goal.png`
- `gm_02_addedit_light_1280_add_transaction.png`
- `gm_02_addedit_light_1280_inline_edit.png`
- `gm_02_addedit_light_768_baseline.png`
- `gm_02_addedit_light_768_addmenu_open.png`
- `gm_02_addedit_light_768_add_account.png`
- `gm_02_addedit_light_768_add_budget.png`
- `gm_02_addedit_light_768_add_goal.png`
- `gm_02_addedit_light_768_add_transaction.png`
- `gm_02_addedit_light_768_inline_edit.png`
- `gm_02_addedit_light_768_add_account_responsive.png`

DOM audit: `gm_02_addedit_dom.json`

---

**Architecture: entity modals share a system — highest-leverage finding**

All entity add modals (account, budget, goal, task, category, member, rule) go through a single
shared path:
- `AddMenu()` (top-bar "+" popover) → sets `AddTarget` atom
- `AddHost()` (shell-root component) reads the atom → renders `FlipPanel(CloseOnly:true, Back:<EntityAddForm>)`
- Every FlipPanel is 384×470px, has `role=dialog aria-modal=true aria-label=<title>`, uses `.set-h /
  .set-body / .set-foot` chrome, and the entity form renders inside `.set-body` as `.form-grid`.

This means any CSS fix to the FlipPanel chrome (title color, foot button theming, body scrollbar)
lands for ALL 7+ entity modals at once with a single CSS change. That is the highest-leverage
lever in this review.

The inline-edit (transaction row `.row-edit`) is a different system — `InlineEditForm` component
inside the table row — that does NOT share the FlipPanel chrome.

The QuickAdd (transaction) is a third path — uses the FlipPanel container but its inner form is
the `quickadd.go` template, not an EntityAddForm.

---

**What already works well (keep — regression anchors)** ✓

1. **Single global entry point** (C79 resolved): one "+" button → `AddMenu` popover → entity form.
   No more hunting per-page for the add affordance. The 9-item menu is correctly aria-wired
   (`role=menu`, `role=menuitem`, `aria-haspopup`, `aria-expanded`). ✓
2. **FlipPanel is a real dialog**: `role=dialog`, `aria-modal=true`, `aria-label` all confirmed
   present on all modals. Focus-trap (Tab cycling) and Esc-to-close wired in JS. ✓
3. **Dark-mode theming is correct across the board**: face bg `rgb(18,18,20)`, input bg
   `rgb(32,32,34)`, input color `rgb(244,244,245)`, title color `rgb(244,244,245)`, border
   `rgb(42,42,44)`. No dark-mode contrast failures detected. ✓
4. **`labeled-field` is used in entity add forms**: Account=5, Budget=5, Goal=3. Most forms use
   the `.labeled-field` wrapper which provides visible `<span>` caption above each control — a
   real label-equivalent, not just a placeholder. ✓
5. **Backdrop semantics**: dark mode uses correct `rgba(4,4,6,0.6)` + blur; light mode uses the
   warm-white `rgba(239,237,232,0.75)` (G21 fix already landed). ✓
6. **768px: modal does not overflow viewport** — `flip-wrap` respects `max-width:92vw` so at 768px
   the 384px panel = 50% of vw, no horizontal overflow. ✓
7. **Consistent footer pattern across modals**: all entity add modals use `CloseOnly:true` →
   single "Close" footer button + entity-owned "Add …" primary submit inside the form body. ✓
8. **Inline edit Save/Cancel placement**: "Save" is `.btn-primary` (left/first), "Cancel" is
   `.btn` (secondary) — correct visual hierarchy. ✓
9. **Light mode input theming correct**: `inputBg: rgb(241,241,242)`, `inputColor: rgb(28,28,30)`,
   `inputBorder: rgb(228,226,221)` — all pass contrast. ✓

---

**Structure fixes (bottom-up, highest-impact first)**

### 1. CRITICAL: FlipPanel modal title is invisible in light mode [CSS-ONLY]

**Dimension: Theming**

`titleColor: rgb(244, 244, 245)` on `faceBg: rgb(255, 255, 255)` in light mode — measured across
all 7 entity modals (account, budget, goal; consistent). Contrast ratio ≈ 1.02:1. **Catastrophic
WCAG AA failure**: the modal title ("Add account", "Add goal", "Add budget", etc.) is invisible
against the white panel face.

Root cause: `.set-h h3` has no `color` rule; it inherits the document color chain. In dark,
the inherited `--text` is `rgb(244,244,245)` and the face is dark — passes. In light, the face
switches to white (via `[data-theme="light"] .flip-face { background: #ffffff; }`) but no rule
resets the inherited text to dark — it stays near-white.

Fix: add a rule that forces readable color inside the light flip-face.

```css
/* GM2 FIX 1: FlipPanel modal title contrast in light mode */
[data-theme="light"] .flip-face .set-h h3,
[data-theme="light"] .flip-face .set-h { color: #1c1c1e; }
```

Applies to ALL 7+ entity modals. **Single 2-line CSS-only fix with maximum blast radius.**

Screenshots: `gm_02_addedit_light_1280_add_account.png`, `gm_02_addedit_light_1280_add_budget.png`,
`gm_02_addedit_light_1280_add_goal.png` (and all light-768 equivalents).

---

### 2. HIGH: FlipPanel footer "Close" / "Save" buttons use hardcoded dark-theme colors in light [CSS-ONLY]

**Dimension: Theming**

The `.set-btn.save` CSS is entirely hardcoded (no CSS variable):
```css
.set-btn.save { background:#1f2c24; border:1px solid #356b50; color:#7fd0a3; font-weight:600; }
```
In light mode, this dark-forest-green button (very dark bg, light-green text) sits on a white
panel face. It does not adapt — the result is a dark-green rectangle on white that looks like it
belongs to a different design system. Similarly `.set-btn.cancel` has hardcoded `color:#a6a6ac`
which in light is low-contrast on white.

Additionally: for the add-entity modals (CloseOnly=true), the sole footer button is "Close" using
`.set-btn.save`. A "Close" action labelled with a Save-style visual hierarchy is semantically
misleading — it appears to be the primary/submit action when it is actually just dismiss.

Fix CSS:
```css
[data-theme="light"] .set-btn.save {
  background: #e8f4ed; border-color: #4caf7a; color: #1a6b3c; }
[data-theme="light"] .set-btn.cancel {
  background: transparent; border-color: #c5c3be; color: #3c3c43; }
[data-theme="light"] .set-btn.cancel:hover { color: #1c1c1e; border-color: #6a6a72; }
```

Screenshots: `gm_02_addedit_light_1280_add_account.png` (footer area).

---

### 3. HIGH: Transaction QuickAdd form has 5 of 6 inputs with no visible label [GO-STRUCTURAL]

**Dimension: Form structure — visible labels vs placeholder-only (a11y)**

The QuickAdd panel (Add transaction → opens FlipPanel with the quickadd form) has 6 inputs:
- Type select (expense/income/transfer) — no label, no placeholder (icon-only? or aria-label?)
- Amount input — placeholder "Amount", no label
- Description input — placeholder "What was it for?", no label
- Account select — no label, no placeholder
- Date input — no label, no placeholder
- "I've reviewed this" checkbox — has a label ✓

5 of 6 controls are placeholder-only or aria-label–only. Placeholders disappear on input, leaving
a screen reader with no field identification. This differs from the entity add modals (which use
`.labeled-field` wrappers). The QuickAdd form is the most-used form in the app and has the worst
a11y.

Fix: wire `.labeled-field` wrapping into `quickadd.go` for each of the 5 unlabeled controls.
This is GO-STRUCTURAL because it changes Go source.

---

### 4. HIGH: Inline-edit form has 2 of 5 inputs with no visible label [GO-STRUCTURAL]

**Dimension: Form structure — visible labels vs placeholder-only (a11y)**

The transaction inline-edit form (`.row-edit`) has:
- Description input — placeholder "Description", no `labeled-field`
- Amount input — placeholder "Amount", no `labeled-field`
- (3 selects — status unknown from DOM; likely have `aria-label` from prior fixes)
- `labeledFieldCount: 0` — **zero labeled-field wrappers across the entire inline-edit form**

`InlineEditForm` takes a `Fields []uic.Node` array; the callers (transactions.go) do not wrap in
`labeledField()`. Unlike the add modals which use `labeledField()` per-field, the inline-edit
form regresses to placeholder-only labels.

Fix: wrap the Description and Amount field nodes with `labeledField()` (or a minimal
`FormField(label, control)` helper) in the call sites inside `transactions.go`. GO-STRUCTURAL.

---

### 5. MEDIUM: "Account number (last 4)" field in Add Account form is placeholder-only [GO-STRUCTURAL]

**Dimension: Form structure — visible labels vs placeholder-only (a11y)**

Account add form: `labeledFieldCount=5` but `placeholderOnly=1` — the "Account number (last 4)"
input. It is rendered outside of a `.labeled-field` wrapper; it has no aria-label. All 4 run-
throughs (dark/light × 1280/768) confirm the same finding.

This is the only unlabeled field in any entity add modal. It appears to have been added to the
form after the initial `labeledField()` wrapper pass, skipping the pattern.

Fix: wrap `accountNumberField` in `labeledField(uistate.T("accounts.accountNumber"), input)` in
`accountaddform.go`. GO-STRUCTURAL.

---

### 6. MEDIUM: Add-menu button blends into topbar in light mode — no affordance [CSS-ONLY]

**Dimension: Open/close — Add-menu affordance**

Dark mode: `addBtnBg: rgb(32,32,34)` — matches topbar `--bg-elev`, the + icon is the differentiator.
Light mode: `addBtnBg: rgb(241,241,242)` — same as the topbar/card bg. The + button has no border,
no shadow, and matches the background exactly. On `gm_02_addedit_light_1280_baseline.png` the
button is hard to find.

Fix: in light mode, give `.add-btn` a subtle border or slightly different background to establish
affordance:
```css
[data-theme="light"] .add-btn {
  border: 1px solid var(--border);
  border-radius: 6px;
}
```

---

### 7. MEDIUM: 384px fixed modal is oversized for simple 3-field forms; undersized for account [CSS-ONLY + GO-STRUCTURAL]

**Dimension: Sizing/positioning**

All entity add modals share the same fixed 384×470px dimensions regardless of form complexity:
- Goal: 3 fields (Name, Target, Target date) — 470px height leaves ~200px of dead space below the
  3rd field and the "Add" button. The form looks sparse in the modal. (C13: "quick-add was
  transaction-only with big empty space" — same issue now in entity modals.)
- Account: up to 10+ fields visible (type-dependent: credit card shows 5 extra liability fields);
  at 470px body=350px height, the form requires scrolling on liability accounts.

The FlipPanel `Height` prop defaults to `"470px"` but could be set per entity:
- CSS-ONLY short-term: `.set-body` could use `min-height: auto` instead of fixed, letting content
  drive height up to `max-height:86vh`.
- GO-STRUCTURAL proper fix: pass entity-appropriate `Height` values in `AddHost()` (e.g. goal →
  `"320px"`, account → `"580px"` for liability / `"420px"` for asset).

For now the CSS-only mitigation is the quick win:
```css
/* GM2 FIX 7: let the flip-wrap shrink to content up to 86vh */
.flip-wrap { height: auto !important; min-height: 300px; max-height: 86vh; }
.flip-inner { height: auto; }
.flip-back  { position: relative; height: auto; min-height: 300px; }
```
Note: this conflicts with the 3D flip animation (which needs `position:absolute` on faces). A safe
version requires setting `height` back after animation completes — track as GO-STRUCTURAL full fix.

---

### 8. MEDIUM: Primary submit button label inconsistency across entity modals [GO-STRUCTURAL]

**Dimension: Footer/actions — primary-action prominence**

- Account modal: "Add account" (entity-specific) ✓
- Budget modal:  "Add" (generic) — inconsistent
- Goal modal:    "Add" (generic) — inconsistent

"Add" alone is too generic for a primary CTA in a modal context; "Add budget" / "Add goal" follows
the pattern established by Account and provides clearer confirmation of what will be created.

Fix: update submit button labels in `budgetaddform.go` and `goaladdform.go` from
`uistate.T("common.add")` or a bare `"Add"` to entity-specific keys like
`uistate.T("budgets.addTitle")` / `uistate.T("goals.addTitle")`. GO-STRUCTURAL.

---

### 9. LOW: No toast or success confirmation after entity add (cross-cutting) [GO-STRUCTURAL]

**Dimension: Footer/actions — success confirmation after add**

Recurring finding from L39–L42: after adding a transaction, budget, goal, or account via a modal,
the modal simply closes with no feedback that the action succeeded. On a dense data set there is no
visual confirmation that the new entity appeared in the list.

All entity add modals call `props.OnDone()` on success, which sets `AddTarget=""` — closing the
modal. The toast system exists (`toast.go`) but is not invoked here.

Fix: after a successful add in each EntityAddForm, call the toast API (e.g.
`appstate.ShowToast("Account added")`) before calling `props.OnDone()`. GO-STRUCTURAL. One fix per
entity add form file (accountaddform.go, budgetaddform.go, goaladdform.go, etc.).

---

### 10. LOW: FlipPanel "Close" footer button label for entity adds is semantically wrong [GO-STRUCTURAL]

**Dimension: Footer/actions**

`AddHost()` passes `CloseOnly:true` to FlipPanel. Inside `flippanel.go`, CloseOnly renders a
single `.set-btn.save`-styled button labelled "Close". The `.save` CSS class on a dismiss action
is incorrect: green-toned "save" styling on a close/cancel action misleads users into thinking
clicking it will submit something.

Fix A (CSS-ONLY): add `.set-btn.close` as an alias for `.set-btn.cancel` and use it for the
CloseOnly button:
```css
.set-btn.close { /* same as .set-btn.cancel */ background: transparent; border: 1px solid #34343a; color: #a6a6ac; }
[data-theme="light"] .set-btn.close { border-color: #c5c3be; color: #3c3c43; }
```
Fix B (GO-STRUCTURAL proper): change `cls: "set-btn save"` to `cls: "set-btn close"` in
`flippanel.go` CloseOnly branch.

---

### 11. LOW: "Scan a document" in Add menu routes to Documents page — no modal [GO-STRUCTURAL]

**Dimension: Open/close — Add-menu affordance**

8 of 9 menu items open an entity add modal or the QuickAdd panel. Item 9 ("Scan a document")
navigates to `/documents`, breaking the mental model: the user expects a modal form, not a full
navigation. This is a C79-era holdover — documents import is complex enough to require a full page.

The fix is at minimum a visual separator or label differentiating "navigate to →" from "add modal"
items, so the user is not surprised by the page transition. A divider line + "Go to" prefix would
set the right expectation without a structural change. CSS-ONLY for the divider; GO-STRUCTURAL
to add the "Go to" prefix text.

---

### 12. LOW: `set-body` scrollbar CSS is hardcoded dark-only [CSS-ONLY]

**Dimension: Theming**

`.set-body` (line 823-824 in `web/index.html`):
```css
scrollbar-color: #34343a transparent;
.set-body::-webkit-scrollbar-thumb { background: #2d2d33; border: 2px solid #121214; }
```
All values are hardcoded dark. In light mode, the scrollbar thumb is dark-grey against white — low
contrast, visually jarring.

Fix:
```css
[data-theme="light"] .set-body {
  scrollbar-color: #c5c3be transparent;
}
[data-theme="light"] .set-body::-webkit-scrollbar-thumb {
  background: #c5c3be; border-color: #ffffff;
}
```

---

**UI/UX defects (screenshot-confirmed + named file)**

| # | Defect | Screenshots | Severity |
|---|--------|-------------|----------|
| 1 | **Modal title invisible in light** — `rgb(244,244,245)` on white bg (~1.02:1) | `light_1280_add_account.png`, `light_1280_add_budget.png`, `light_1280_add_goal.png`, `light_768_*` | CRITICAL |
| 2 | **Footer buttons hardcoded dark-green** in light mode | `light_1280_add_account.png` (footer band) | HIGH |
| 3 | **QuickAdd: 5/6 inputs placeholder-only** (no visible label) | `dark_1280_add_transaction.png` | HIGH |
| 4 | **Inline-edit: 0 labeled-field wrappers** (description + amount placeholder-only) | `dark_1280_inline_edit.png`, `light_1280_inline_edit.png` | HIGH |
| 5 | **"Account number (last 4)" unlabeled** in Account add modal | `dark_1280_add_account.png` (DOM audit) | MEDIUM |
| 6 | **+ button invisible against topbar in light** — same bg, no border | `light_1280_baseline.png` | MEDIUM |
| 7 | **Goal/Budget add modal: ~200px dead space** below 3-field form | `dark_1280_add_goal.png`, `dark_1280_add_budget.png` | MEDIUM |
| 8 | **"Add" generic label** on Budget/Goal primary button vs "Add account" on Account | `dark_1280_add_budget.png`, `dark_1280_add_goal.png` | MEDIUM |
| 9 | **No toast on successful add** — modal closes silently (cross L39–L42) | (behavior, not screenshot) | MEDIUM |
| 10 | **".set-btn.save" CSS on Close-only button** — wrong semantic | `light_1280_add_account.png` | LOW |
| 11 | **"Scan a document" breaks modal mental model** (navigates, no add form) | `dark_1280_addmenu_open.png` | LOW |
| 12 | **Scrollbar hardcoded dark** in `.set-body` | `light_1280_add_account.png` (scroll thumb) | LOW |

---

**Cross-references**

- C13: "quick-add was transaction-only with big empty space" — same dead-space issue now in all
  3-field entity modals (Goal, Task, Category, Member).
- C79: global +Add menu (resolved) — menu structure is correct; this ticket reviews the modal UX
  downstream of it.
- L39–L42: "silent add, no toast" — finding #9 above confirms this is still unresolved for all
  entity add modals.

---

**Shared system vs. ad-hoc verdict**

**The entity add modals share a system.** All 7+ entities go through `AddHost()` → `FlipPanel` →
`.form-grid` / `labeled-field` pattern. The flippanel.go chrome (`set-h`, `set-body`, `set-foot`)
is single-sourced. CSS fixes to FlipPanel theming = maximum blast radius, affecting all modals.
The inline-edit form (`InlineEditForm` in `inlineeditform.go`) and the QuickAdd form are separate
systems that do NOT share the labeled-field discipline of the entity add modals — both have a11y
regressions.

---

**Probe hardening**

- [ ] After each modal open, assert `document.querySelector(".flip-backdrop.show")` is present
      before screenshotting — guards against missed open.
- [ ] After entity submit, assert the backdrop is gone AND a toast appears (regression gate for
      finding #9 once fixed).
- [ ] Assert `[data-theme="light"] .set-h h3` computed color is not near-white — regression gate
      for finding #1 once fixed.
- [ ] Assert `.labeled-field` count in inline-edit form ≥ 2 once finding #4 is fixed.
- [ ] For QuickAdd: assert each non-checkbox input has a corresponding `aria-label` or `<label>`.
- [ ] `closeAddMenu()` in the script uses `.add-backdrop.force-click` — correct, since Escape
      is not wired to the add-menu (only to FlipPanel). Document this in probe comments.
