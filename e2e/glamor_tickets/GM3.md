### GM3. Confirm/destructive dialogs — UX review (cf-dialog, C42) — 2026-06-23 ★

**The story**
Dana is cleaning up her household's transactions: she selects a batch of 50 stale entries,
clicks "Delete selected", and expects to see a clear, safe dialog that names what's about to
be destroyed, keeps the dangerous button visually distinct, and defaults focus to Cancel so a
slip of the Enter key can't nuke her data. She also occasionally renames workspaces and custom
pages, where a prompt dialog should auto-focus the text field and close cleanly on Escape. Both
flows should be themeable and fully keyboard-operable.

**Drive script**
`e2e/gm_03_dialogs.mjs`

**Build/run evidence**
- App running on `http://127.0.0.1:8099` (stale wasm + live `web/index.html` — GI0 mode)
- `$env:E2E_URL="http://127.0.0.1:8099"; node e2e/gm_03_dialogs.mjs` → **EXIT 1** (8 unique warnings across 4 theme×width combos; all structural, none are crashes)
- 4 DOM audit JSON files produced: `gm_03_dialogs_{dark,light}_{1280,768}_audit.json`
- 0 native browser dialogs fired throughout — C42 regression check **PASS**

**Screenshots produced (e2e/screenshots/)**
- `gm_03_dialogs_dark_1280_1280_txn_delete_open.png` — confirm dialog, dark, 1280
- `gm_03_dialogs_dark_1280_confirm_dialog_full.png` — confirm dialog full-page, dark 1280
- `gm_03_dialogs_dark_1280_bulk_selection_active.png` — bulk bar with selection, dark 1280
- `gm_03_dialogs_dark_1280_bulk_no_confirm.png` — bulk delete fired with no dialog, dark 1280
- `gm_03_dialogs_dark_1280_prompt_dialog_open.png` — prompt dialog, dark 1280
- `gm_03_dialogs_dark_768_confirm_dialog_full.png` — confirm dialog, dark 768
- `gm_03_dialogs_dark_768_bulk_no_confirm.png` — bulk delete no dialog, dark 768
- `gm_03_dialogs_dark_768_prompt_dialog_open.png` — prompt dialog, dark 768
- `gm_03_dialogs_light_1280_confirm_dialog_full.png` — confirm dialog, light 1280
- `gm_03_dialogs_light_1280_bulk_no_confirm.png` — bulk delete no dialog, light 1280
- `gm_03_dialogs_light_1280_prompt_dialog_open.png` — prompt dialog, light 1280
- `gm_03_dialogs_light_768_confirm_dialog_full.png` — confirm dialog, light 768
- `gm_03_dialogs_light_768_prompt_dialog_open.png` — prompt dialog, light 768
- `gm_03_dialogs_txn_delete_dark_1280.png`, `_dark_768.png`, `_light_768.png`

---

## What already works well (keep) ✓

- **No native dialogs** — `window.confirm/prompt/alert` never fire; C42 is complete and stable.
- **Danger styling is correct** — the destructive confirm button carries `btn btn-danger` and
  renders red/crimson in both themes (`rgb(216,113,111)` dark, `rgb(179,50,47)` light). ✓
- **Backdrop + scrim present** — `.cf-dialog-scrim` exists; click-outside cancels. ✓
- **Centering** — dialog is horizontally and vertically centered at both 1280 and 768. ✓
  (rect 448×110px, centered in both viewports; `width:min(28rem,100%)` keeps it bounded)
- **ARIA basics** — `role="dialog"` + `aria-modal="true"` on `.cf-dialog-backdrop`. ✓
- **Keyboard entry/exit** — Enter confirms, Escape cancels (verified on prompt), Tab trapped
  within `.cf-dialog` controls. ✓
- **Prompt dialog focus** — `focusOnInput=true` confirmed; input also gets `.select()` so
  any pre-filled default is ready to overwrite. ✓
- **Cancel closes** — `.locator(".cf-dialog").count()` = 0 after Cancel every time. ✓
- **Message copy** — "Delete 'Household & shopping'? This can't be undone." — names the
  entity, includes irreversible warning in-message. Solid for single-row deletes. ✓
- **Light-mode theming** — dialog face `rgb(255,255,255)`, message text `rgb(28,28,30)`,
  cancel button `rgb(241,241,242)`. Readable, no residual dark tokens. ✓

---

## Structure fixes (bottom-up, highest impact first)

### 1. [GO-STRUCTURAL] Bulk-delete has no confirmation dialog — L50 safety gap ★ CRITICAL

**Finding:** clicking "Delete selected" after "Select all" (50 rows in probe) fires the
delete immediately with NO `cf-dialog` and no intermediate confirmation. The only safety net
is the undo button that appears in the bulk bar AFTER the fact. Probe confirmed: `bulkDialogOpen`
was `false` on every run; the undo button was present and clicked to recover.

**Evidence:** `gm_03_dialogs_dark_1280_bulk_no_confirm.png`, `_dark_768`, `_light_1280`.

**Impact:** a user who fat-fingers "Delete selected" or selects all via keyboard shortcuts
loses all selected data in one click. With 50+ transactions selected, that is total data loss
for the visible filter view.

**Fix:** wire `uistate.ConfirmModal` into `bulkDelete` in `internal/screens/transactions.go`
(line 277). The message should be count-aware, e.g. "Delete 50 transactions? This can't be
undone." using the already-translated `transactions.bulkOpDeleted` key as a template.

```go
// internal/screens/transactions.go ~line 277 — bulkDelete handler
bulkDelete := ui.UseEvent(Prevent(func() {
    sel := selected.Get()
    count := len(sel)
    uistate.ConfirmModal(
        uistate.T("transactions.bulkDeleteConfirm", count),
        true,
        func(ok bool) {
            if !ok { return }
            // existing delete + snapshot logic here
        },
    )
}))
```

Also add `"transactions.bulkDeleteConfirm"` to `internal/i18n/en.go`:
```
"transactions.bulkDeleteConfirm": "Delete %d transactions? This can't be undone.",
```

_Cross-links: L50 (this is the exact gap L50 named)._

---

### 2. [GO-STRUCTURAL] Default focus on destructive confirm is WRONG — should be Cancel

**Finding:** across all 4 theme×width combos, `focusedId = "cf-dialog-confirm"` — the red
danger button is default-focused when a confirm dialog opens. Per WCAG 3.2.4 and
destruction-safety UX convention, the SAFE option (Cancel) must be default-focused for
destructive dialogs so pressing Enter can't accidentally delete.

**Evidence:** every audit JSON shows `"focusOnSafe": false, "focusedId": "cf-dialog-confirm"`.

**Current code** (`internal/app/dialoghost.go` line 111):
```go
focusID := "cf-dialog-confirm"
if req.Kind == uistate.DialogPrompt {
    focusID = dialogInputID
}
```

**Fix:** reverse the default for destructive confirms:
```go
focusID := "cf-dialog-confirm"
if req.Kind == uistate.DialogPrompt {
    focusID = dialogInputID
} else if req.Destructive {
    focusID = "cf-dialog-cancel"
}
```

Also add `id="cf-dialog-cancel"` to the Cancel button in the render:
```go
Button(css.Class("btn"), Type("button"), Attr("id", "cf-dialog-cancel"),
    OnClick(func() { finish(false) }), uistate.T("action.cancel")),
```

_Cross-links: C42 (a11y + keyboard must-keep list), WCAG SC 3.2.4._

---

### 3. [GO-STRUCTURAL] Confirm dialogs have no title — missing `cf-dialog-title`

**Finding:** single-transaction delete dialogs render only a `<p class="cf-dialog-msg">` with
no `<h3 class="cf-dialog-title">`. The `DialogRequest.Title` field is empty in all callers
(e.g. `transactions.go:713` passes only a `message` to `ConfirmModal`, which has no `Title`
parameter at all).

**Evidence:** `"hasTitle": false` across all 4 audit JSONs.
Observed message: "Delete 'Household & shopping'? This can't be undone."

**Impact:** (a) No `aria-labelledby` is possible without a title element — ARIA pattern for
`role=dialog` requires a labelling heading (ARIA APG). (b) At glance a user can't
distinguish the "confirm a delete" chrome from a generic notice. (c) The `.cf-dialog-title`
Fraunces serif style is defined in CSS but never triggered for confirms.

**Fix:** surface the `Title` field. For destructive confirms, provide a clear action-class
title. Two routes:

Option A — add `Title` to the `ConfirmModal` helper signature:
```go
// uistate/dialog.go
func ConfirmModal(title, message string, destructive bool, onResult func(bool))
```
Then each call site passes e.g. `"Delete transaction"`.

Option B (simpler, no API change) — auto-derive a title in `DialogHost` when `Destructive`
is true and `Title` is empty:
```go
if req.Title == "" && req.Destructive {
    req.Title = uistate.T("dialog.deleteTitle") // "Are you sure?"
}
```

Either way add `aria-labelledby="cf-dialog-title"` on the backdrop element and
`id="cf-dialog-title"` on the `<h3>`.

_Cross-links: C42 (a11y — labelledby listed as missing in this review), ARIA APG Dialog pattern._

---

### 4. [GO-STRUCTURAL] Missing `aria-labelledby` on the dialog backdrop

**Finding:** `labelledBy: null` in all audit JSONs. The `role=dialog` element (`.cf-dialog-backdrop`)
has `aria-modal="true"` but no `aria-labelledby`, so screen readers cannot announce the dialog
title when it opens.

**Fix:** once item #3 above adds a title `<h3 id="cf-dialog-title">`, add
`Attr("aria-labelledby", "cf-dialog-title")` to the backdrop element in `dialoghost.go`.
Also consider `role="alertdialog"` (vs `role="dialog"`) for destructive-only confirms —
`alertdialog` is announced with urgency by most screen readers.

_Cross-links: C42 (a11y must-keep list), ARIA APG._

---

### 5. [CSS-ONLY] Backdrop blur/dim is plain black scrim — consider frosted glass

**Finding:** `.cf-dialog-scrim { background: rgba(0,0,0,.45) }` — a flat translucent black
with no blur. The FlipPanel (Settings, Quick-add) uses `backdrop-filter:blur(...)` for a
frosted-glass modal feel (per GM1 review). The cf-dialog system is visually lighter/cheaper
by comparison.

**Fix (CSS only, landable now in `web/index.html`):**
```css
.cf-dialog-scrim {
  position: absolute; inset: 0;
  background: rgba(0,0,0,.4);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}
```

Impact: low — purely cosmetic consistency between the two modal systems.

---

### 6. [CSS-ONLY] Dialog box-shadow is inconsistent with FlipPanel elevation

**Finding:** `.cf-dialog { box-shadow: 0 12px 40px rgba(0,0,0,.3) }` — uses `rgba` black with
no variable-aware color. In light mode where the background is white, the shadow reads well;
in dark mode the card-on-dark shadow has limited visible contrast. The FlipPanel uses
`--shadow-xl` (if defined) or similar.

**Fix (CSS only):**
```css
.cf-dialog {
  box-shadow: 0 12px 40px rgba(0,0,0,.35), 0 0 0 1px var(--line,#e5e7eb);
}
```

---

### 7. [CSS-ONLY] Confirm dialog is very compact — minimal breathing room

**Finding:** rect height = 110px at all viewports. With message text + two buttons and
1.25rem padding, the dialog reads as cramped relative to the FlipPanel modals (which have
full 560px height). This is functional but the spatial economy is tight for a moment that
asks the user to make a deliberate choice.

**Fix (CSS only):** increase padding slightly and enforce a min-height:
```css
.cf-dialog {
  padding: 1.5rem 1.5rem 1.25rem;
  min-height: 6rem;
}
```

---

### 8. [GO-STRUCTURAL] Bulk-delete undo relies on post-hoc recovery rather than pre-action confirmation

**Note** (ties to #1): the bulk undo ("Undo" button in the bulk bar) is well-implemented as
a secondary safety net. It should be kept even after adding a pre-confirmation dialog —
it protects against accidental confirms. The undo button's `title="Undo the last bulk action"`
was detectable and clickable by the probe; this is the correct fallback.

---

## UI/UX defects (screenshot-confirmed)

| # | Defect | Screenshots | Tag |
|---|--------|------------|-----|
| D1 | Bulk delete fires immediately — no confirm, no count | `dark_1280_bulk_no_confirm.png`, `dark_768_bulk_no_confirm.png`, `light_1280_bulk_no_confirm.png` | [GO-STRUCTURAL] |
| D2 | Destructive confirm autofocuses the danger button, not Cancel | All `_audit.json`: `focusedId=cf-dialog-confirm` | [GO-STRUCTURAL] |
| D3 | No dialog title on any confirm dialog | All `_audit.json`: `hasTitle: false` | [GO-STRUCTURAL] |
| D4 | No `aria-labelledby` on `role=dialog` backdrop | All `_audit.json`: `labelledBy: null` | [GO-STRUCTURAL] |
| D5 | Scrim has no backdrop blur (inconsistent with FlipPanel) | `dark_1280_confirm_dialog_full.png` | [CSS-ONLY] |
| D6 | Dialog shadow is not theme-token-aware | `light_1280_confirm_dialog_full.png` | [CSS-ONLY] |
| D7 | Prompt confirm button is `btn-primary` (correct), no accidental danger-styling | `dark_1280_prompt_dialog_open.png` | No defect — ✓ |

---

## Probe hardening notes

- gwc-error-overlay dismissed via `page.evaluate(() => document.getElementById('gwc-error-overlay')?.remove())`
  before every interaction — essential; without it, overlay intercepts all clicks (confirmed on prior G-series probes).
- Bulk selection triggered via `button[title*="Select all transactions in the current filtered view"]` — robust;
  avoids per-row checkbox assumption (the transactions list uses atom-based selection, not `<input type=checkbox>`).
- Undo triggered via `button[title*="Undo the last bulk"]` after accidental bulk delete — probe recovered
  all deleted data on every run.
- P3 (custom-page delete) required a pre-existing custom page; probe skipped gracefully when none was
  present. To cover this path: seed a custom page before the probe pass, or create one via `createCustomPage`
  helper (pattern from `glamor_22_custompages.mjs`), then verify its delete confirm.
- Native dialog listener `page.on("dialog")` present throughout — fired 0 times.

---

## Destructive-action safety verdict

**UNSAFE as shipped** for bulk operations. D1 (no bulk-delete confirmation) is the primary
risk: a user with 50 transactions selected can destroy all of them in one mis-click with no
warning and no chance to review. The undo button mitigates but does not substitute for
pre-confirmation. D2 (wrong focus on confirm button) compounds this: if a confirm dialog
WERE shown, a stray Enter keypress while thinking would still confirm.

Single-row delete is safer — the message names the entity ("Delete 'Household & shopping'?
This can't be undone."), the danger button is visually red, and Cancel is present — but the
wrong default focus (D2) still means Enter-key users are one keystroke from data loss.

Fix priority: D1 then D2 then D3/D4 (a11y), then CSS-only D5/D6.

_Cross-links: C42 (replace native popups — DONE), L50 (bulk delete had no confirmation — this review confirms still open)._
