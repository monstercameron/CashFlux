### GM1. Settings modal — UX deep-dive (from G21) — 2026-06-23 ★

**The story**
Renée is a household manager opening Settings for the first time on a new device. She clicks the
household card (button.hh) at the rail bottom, the FlipPanel animates in, and she works through it
top to bottom: household members, base currency, appearance, module visibility toggles, the AI key
field, import/export data buttons, and finally closes satisfied. She is on a 1280×900 desktop,
and wants to verify both dark and light themes look right.

**Drive script**
`e2e/gm_01_settings.mjs`

Run command and exit code:
```
$env:E2E_URL="http://127.0.0.1:8099"; node e2e/gm_01_settings.mjs
EXIT 0
```

**Screenshots produced** (all in `e2e/screenshots/`):

| File | Width | Theme | Description |
|------|-------|-------|-------------|
| `gm01_dark_1280_top.png` | 1280 | dark | Panel open — top (Household + Appearance) |
| `gm01_dark_1280_mid.png` | 1280 | dark | Panel scrolled mid (Notifications + Cloud + Data) |
| `gm01_dark_1280_bottom.png` | 1280 | dark | Panel scrolled bottom (Workspaces + AppLock + Languages + Debug) |
| `gm01_dark_768.png` | 768 | dark | Panel at mobile-breakpoint width |
| `gm01_light_1280_top.png` | 1280 | light | Panel open — top, light theme |
| `gm01_light_1280_mid.png` | 1280 | light | Panel scrolled mid, light theme |
| `gm01_light_1280_bottom.png` | 1280 | light | Panel scrolled bottom, light theme |
| `gm01_light_768.png` | 768 | light | Panel at 768px, light theme |

**JSON audits**: `gm01_dark_1280_dom.json`, `gm01_dark_768_dom.json`,
`gm01_light_1280_dom.json`, `gm01_light_768_dom.json`

---

**G21 modal issues resolved by the global light-mode token fix (already landed in index.html)**

G21 identified three CRITICAL light-mode failures in the Settings panel. The global CSS fix
(landed before this GM1 run) addressed all three:

| G21 defect | G21 measured value | GM1 measured value | Status |
|---|---|---|---|
| #1 Toggle row labels white-on-white | `rgb(244,244,245)` in light | `rgb(28,28,30)` in light | **FIXED** ✓ |
| #2 Panel backdrop dark in light | `rgba(4,4,6,0.6)` in light | `rgba(239,237,232,0.75)` in light | **FIXED** ✓ |
| Panel face dark in light | `(dark face)` in light | `rgb(255,255,255)` in light | **FIXED** ✓ |
| set-label muted in light | `rgb(86,86,92)` | `rgb(60,60,67)` (slightly darker) | **Improved** ✓ |
| set-input/data-btn/rate-in | no light override | `[data-theme=light]` rule added | **FIXED** ✓ |

The password inputs now report proper `aria-label` via the section name (confirmed:
`["AI (OpenAI · bring your own key)", "Web search (chat)", "Backend bearer token"]`) —
G21 defect #6 (password inputs with no aria) is **partially resolved** (section-name labels
are present; dedicated `aria-label` on the input elements themselves still missing — see
structural fixes below).

The `.data-btn-danger` CSS class for the Wipe data button danger styling is now in
`index.html` (replacing the inline `Style()` call noted in G21 defect _4_) — **FIXED** ✓.

The `@media (max-width: 768px) { .flip-wrap .grid-cols-2 { grid-template-columns: 1fr !important; } }`
rule is present in `index.html` — however at viewport 768px the panel still renders 2 columns
(screenshots `gm01_dark_768.png`, `gm01_light_768.png`). The class-name the Go code emits
at runtime may not be literally `grid-cols-2` in the rendered DOM — the media-query CSS is
present but not firing. See structural fix #S4 below.

---

**What already works well (keep — regression anchors)** ✓

- **Panel trigger is natural and discoverable.** `button.hh` at rail bottom — gear icon +
  household name + member/currency summary. `waitFor('.set-label')` reliably signals panel
  ready. Confirmed `gm01_dark_1280_top.png`. ✓
- **Two-column layout at 1280 is information-dense without cramped.** Left column: household/
  data config (members, currency, exchange rates, screen toggles, freshness, notifications).
  Right column: appearance + AI + cloud. Mental model is sensible. Confirmed
  `gm01_dark_1280_top.png`. ✓
- **Light theme is now fully correct.** Backdrop warm-white (`rgba(239,237,232,0.75)`), face
  pure white (`rgb(255,255,255)`), toggle labels dark (`rgb(28,28,30)`). In `gm01_light_1280_top.png`
  "Daniel Carter" and "Jordan Lee (roommate)" member chips, household member section, base currency
  selector, and Appearance controls all render cleanly. Light mode is no longer near-identical
  to dark mode. ✓
- **Footer is pinned.** `footPinned: true` — Cancel + Save are always accessible without
  scrolling. Confirmed at all 4 widths/themes. ✓
- **Save button is prominent and correctly styled.** Dark green background + green label in both
  themes, clearly differentiated from the neutral Cancel. `gm01_dark_1280_top.png`,
  `gm01_light_1280_top.png`. ✓
- **Data action buttons have unique labels.** "Export JSON", "Export CSV", "Import dataset…",
  "Load sample", "Wipe data" — each distinct. "Import dataset…" (renamed from bare "Import…")
  is the correct label. Confirmed `gm01_dark_1280_mid.png`. ✓
- **Password inputs have section-scoped aria labels.** All 3 password inputs report a non-empty
  aria-label (the section heading value). G21 defect partially resolved. ✓
- **Exchange rate rows are clean and scannable.** Currency code + `1 X =` + editable input +
  `USD`. Pattern is immediately parseable. Dark `gm01_dark_1280_top.png`,
  light `gm01_light_768.png`. ✓
- **Debug log is unobtrusive at bottom.** "imported dataset / INFO", "IndexedDB artifact store ready /
  INFO" below all user-facing settings. CashFlux v0.1.0 + "What's new" anchor.
  `gm01_dark_1280_bottom.png`, `gm01_light_1280_bottom.png`. ✓
- **Appearance section is at the TOP of the right column.** In both themes, `gm01_*_top.png`
  shows Appearance (Dark/Light/System toggle, Accent swatches, Theme presets Forest/Midnight/Paper,
  Colors, Shape & type) as the first right-column content. The G21 complaint that Appearance was
  buried has been resolved — section order in the current wasm puts Appearance first. ✓
- **Wipe data danger styling now uses CSS class** (`.data-btn-danger` with `color: var(--danger)`).
  The inline `Style()` override noted in G21 is gone from index.html. ✓

---

**Structure fixes (bottom-up)** grouped by the 7 modal dimensions

_1. OPEN/CLOSE_ **[GO-STRUCTURAL]**

- **S1. No `role="dialog"` or `aria-modal` on the FlipPanel backdrop.** DOM audit: `ariaModal: null`,
  `roleDialog: null`. The `.flip-backdrop` is the modal root element but carries neither
  `role="dialog"` nor `aria-modal="true"`. Screen readers cannot identify the panel as a dialog or
  restrict navigation to its content. A proper focus trap and `aria-labelledby` pointing to the
  "Settings" H3 heading are also absent. Fix: in `internal/app/settings.go` (or whichever Go file
  renders `.flip-backdrop`), add `role="dialog"`, `aria-modal="true"`, and
  `aria-labelledby="settings-title"` on the backdrop element; give the H3 header `id="settings-title"`.
  **[GO-STRUCTURAL]**

- **S2. ESC-to-close and click-outside-to-close are not verified.** The `×` close button works
  (confirmed in probe — `closeSettings()` via `.set-close` closes reliably). ESC and backdrop-click
  were not wired in the probe. Future GM1b run should assert `Escape` key dismisses the panel and that
  clicking the `.flip-backdrop` outside `.flip-wrap` closes it. **[GO-STRUCTURAL]** (wiring) /
  **[CSS-ONLY]** (no fix needed if already wired — just verify).

_2. SIZING/POSITIONING_ **[CSS-ONLY + GO-STRUCTURAL]**

- **S3. Panel height is viewport-filling at 774px, not the designed 560px.** DOM audit:
  `flipWrapDims: {width:760, height:774}` at 1280 and `{width:707, height:774}` at 768. The CSS reads
  `height:470px; max-height:86vh`. At 900px viewport, `86vh = 774px`, so `max-height` wins and the
  panel grows to 774px. This means the "380px scroll window" critique from G21 no longer applies —
  the body scroll area is now ~700px tall. Screenshots confirm: most of the panel content is visible
  at the top without scrolling; mid-scroll reveals cloud/data; bottom-scroll reaches languages/debug.
  The panel height behavior is now **correct** for tall viewports. No fix needed for the max-height
  rule. The underlying structural complaint (23 sections without nav) remains valid regardless.
  **NOTE:** The `height:470px` base height in the CSS (line 814 of index.html) is lower than the
  actual rendered height — the `max-height:86vh` is doing the heavy lifting. On very short viewports
  (600px or less) the panel would be only 516px, reinstating the cramped scroll. **[CSS-ONLY]**:
  raise the base `height` to `min(86vh, 680px)` to keep the panel spacious on short viewports too.

- **S4. Two-column grid does NOT collapse to single column at 768px.** `gm01_dark_768.png` and
  `gm01_light_768.png` both show the two-column layout intact at 768px viewport. The CSS media query
  at `max-width:768px` targets `.flip-wrap .grid-cols-2` but the Go code likely emits a Tailwind
  utility class (`tw.GridCols2`, resolved at runtime to a different class name) that doesn't literally
  match `grid-cols-2` in the DOM. Result: at 768px the left column is ~330px and right column
  ~330px — visually crowded, with the currency-code labels on FX rows partially clipped
  (`gm01_dark_768.png`: "AUD", "CAD" etc. flush against the left panel edge).
  Fix: **[CSS-ONLY]** — audit what class the Go runtime emits for the two-column grid wrapper in
  the Settings panel. If the element carries an inline `style="grid-template-columns: repeat(2, ...)"`,
  target that with `@media (max-width:768px) { .set-body > div[style*="grid-template"] { grid-template-columns: 1fr !important; } }`.
  Alternatively add a Go-side class (e.g. `set-grid`) and target that. **[CSS-ONLY]** once the
  selector is confirmed; **[GO-STRUCTURAL]** if a new class attribute is needed.

- **S5. Panel width at 768 is 707px (92vw), leaving only 30px margin each side.** `gm01_light_768.png`:
  the panel edges are ~15px from the viewport edge. The close (×) button at top-right nearly clips
  the viewport edge. At 480px or below this becomes unusable. The `.flip-wrap { max-width:92vw }`
  rule is correct for desktop but at phone widths 92vw is too greedy. Recommend
  `max-width: min(760px, calc(100vw - 24px))` so there is always at least 12px breathing room.
  **[CSS-ONLY]**

_3. THEMING_ **[CSS-ONLY — all now fixed in index.html]**

- **[RESOLVED] Toggle row labels**: `rgb(28,28,30)` in light (was `rgb(244,244,245)`) — fix landed. ✓
- **[RESOLVED] Backdrop**: `rgba(239,237,232,0.75)` in light (was `rgba(4,4,6,0.6)`) — fix landed. ✓
- **[RESOLVED] Panel face**: `rgb(255,255,255)` in light (was dark) — fix landed. ✓
- **[RESOLVED] set-input / data-btn / rate-in**: light overrides present in index.html. ✓

- **S6. `.switch` toggle OFF state in light is `background:#d4d2cc` with `::after` white.** Visually
  confirmed `gm01_light_1280_top.png`: the OFF toggle (Dark/Light mode — currently "Dark" is selected,
  so the "Light" toggle-row switch is OFF) renders as a soft grey pill. Contrast of the white thumb
  on `#d4d2cc` background is acceptable. However the ON state for `.switch.on` uses `background:#3e7f5e`
  (dark green) in both themes — in light mode this dark-green pill on a white card reads well.
  No fix needed. ✓

- **S7. `set-label` section headers in light: `rgb(60,60,67)` on white.** Contrast ratio ~8:1 for
  the ALL-CAPS 0.7rem labels. This is a meaningful improvement over G21's `rgb(86,86,92)` (~5.8:1).
  Passes WCAG AA for large text. No further fix needed. ✓

_4. STRUCTURE/NAV_ **[GO-STRUCTURAL]**

- **S8. 23 sections with no in-panel navigation.** DOM: `sectionCount: 23`. The panel body scrolls
  through 23 topic sections with zero jump links, tab strips, or section sidebar. In `gm01_dark_1280_top.png`
  the visible content shows Household + Appearance; mid-scroll shows Notifications + Cloud + Data;
  bottom-scroll shows Workspaces + AppLock + Languages + Debug. Renée must scroll 3+ panel-heights
  to reach the bottom. There is no progress indicator or section overview.
  Fix (two options): (a) **Quick win** — add a `<nav>` jump-link row at the top of `.set-body` with
  anchors to the 6 major clusters: Household · Appearance · AI · Data · Advanced. This is
  **[GO-STRUCTURAL]** (needs a Go render change) but the HTML shape is simple. (b) **Full fix** —
  introduce a left-rail section tab within the panel (like macOS System Settings), each tab showing
  one compact group. This is a larger **[GO-STRUCTURAL]** redesign. The jump-link row is the
  MVP. Cross-ref G21 defect #4.

- **S9. Section ordering — right column: AI section appears BELOW the Appearance section.** DOM
  `sectionNames` index order (right column, as confirmed by `gm01_*_top.png`): Appearance → Theme →
  Colors → Shape & type → Dashboard banner → Preferences → AI → Web search → Cloud & server → Data →
  Backup → Workspaces → App lock → Languages → Debug log. This is the corrected order vs G21 (where
  AI was first). The current wasm already has Appearance at the top. G21 defect #5 is resolved. ✓

- **S10. `set-label` section headings are `<div>` not `<h4>`.** DOM: `setLabelTags: ["div"]`. All 23
  section labels are plain `<div>` elements. Screen readers navigating by heading find only the H3
  "Settings" title — there is no heading hierarchy inside the panel. Fix: change the Go render from
  `Div(css.Class("set-label"), ...)` to `H4(css.Class("set-label"), ...)`. **[GO-STRUCTURAL]**
  Cross-ref G21 defect #6, G16–G19 same pattern.

_5. CONTROLS_ **[GO-STRUCTURAL + CSS-ONLY]**

- **S11. `labelCount: 8` for 30 inputs — most inputs lack a proper `<label>` element.** DOM audit:
  30 inputs, 11 selects, only 8 `<label>` elements. Password inputs carry aria-label via section name
  (the section heading is associated at the section level, not the input level). A focused password
  input reads back the section heading as context, which is acceptable but not ideal. Dedicated
  `aria-label` on each password input would be cleaner. The select elements carry `aria-label` as
  confirmed in G21. Fix: add `aria-label` directly on each `<input type="password">`:
  `"OpenAI API key"`, `"Web search API key"`, `"Backend bearer token"`. **[GO-STRUCTURAL]**

- **S12. 4 "Import" buttons with identical `.data-btn` styling — L47 confirmed.** DOM:
  `importBtnCount: 4`, texts: `["Import theme", "Import dataset…", "Import workspace", "Import languages"]`.
  All 4 use the same `.data-btn` class. The rename from bare "Import…" to "Import dataset…" is already
  in place (resolves L47 for the data import button). The other three are uniquely named. The remaining
  issue is purely **visual** — all four look identical (same border, same font-size, same bg). They
  appear in different sections (theme editor, data, workspaces, languages) so context disambiguates them,
  but a Renée scanning the panel sees 4 identical buttons. No further rename is needed; the L47 rename
  is done. Lower-priority: add a subtle icon prefix (`↑ Import theme`, `↑ Import dataset…`) to add
  visual scent at a glance. **[CSS-ONLY]** (icon via CSS `content:` or Go-side icon glyph).

- **S13. Toggle count mismatch: 26 `.toggle-row span` elements but only 18 `.switch` elements.**
  DOM: `toggleRowCount: 26`, `toggleCount: 18`. 8 toggle rows have no switch — these are likely
  "inactive" rows or read-only display rows that use the `.toggle-row` layout class without a
  `.switch` control. This is worth auditing: any `.toggle-row` without a `.switch` may be a render
  gap where a control was expected. **[GO-STRUCTURAL]** investigation.

_6. FOOTER/ACTIONS_ **[CSS-ONLY + GO-STRUCTURAL]**

- **S14. Save button closes panel with no success feedback.** `footPinned: true`, Save is always
  accessible. But no toast, notice, or banner confirms what was saved after closing. The `noticeAtom`
  already fires for import/wipe events. A "Settings saved" success notice on `Save()` completion
  would close the feedback loop. **[GO-STRUCTURAL]** Cross-ref G21 defect #7.

- **S15. "Wipe data" is styled as `.data-btn-danger` (CSS class now).** Visual check:
  `gm01_dark_1280_mid.png` shows "Wipe data" with red text and border (distinct from the neutral
  "Export JSON", "Export CSV", "Import dataset…", "Load sample" siblings). The CSS-class fix from
  G21 _4_ is confirmed landed. No remaining action. ✓

- **S16. No destructive-action confirmation guard visible in the panel UI.** The Wipe data button
  appears in-panel with no secondary confirmation (no "Are you sure?" inline guard). A click of Wipe
  data during a normal settings session would be catastrophic. Confirm that a confirmation dialog
  fires before wiping — this is not visible in a static screenshot but should be verified in a future
  flow probe. **[GO-STRUCTURAL]** verify.

_7. GENERAL MODAL UX_ **[GO-STRUCTURAL + CSS-ONLY]**

- **S17. No `aria-modal`, no `role="dialog"`, no focus trap.** (Duplicate of S1 — highest a11y
  impact, listed again for prominence.) Tab order inside the panel is not confined to the panel
  content. A keyboard user tabbing through the Settings panel can tab out into the page behind it.
  This is a WCAG 2.1 failure (criterion 1.3.6 / 4.1.2). **[GO-STRUCTURAL]**

- **S18. "Enable AI features" toggle off but AI key inputs remain active.** Not directly audited
  in this run (would require toggling the switch and checking `disabled` state). G21 defect #10:
  if the AI toggle is off, the key input and remember-key toggle should be dimmed/disabled.
  **[GO-STRUCTURAL]** — needs conditional rendering or `disabled` prop wiring.

- **S19. AI section hard-codes OpenAI.** Section header "AI (OpenAI · bring your own key)",
  placeholder "sk-…", model list limited to GPT variants. C81 (multi-provider) is the gating
  ticket. Track C81; no action here until C81 lands. Cross-ref G21 defect #9.

---

**UI/UX defects (screenshot-confirmed)**

| # | Priority | Screenshot | Issue | Fix | Tag |
|---|---|---|---|---|---|
| 1 | CRITICAL | `gm01_*_768.png` | 2-column grid NOT collapsing at 768px — FX code labels clip left edge, columns ~330px each | Fix media-query selector to match the actual runtime class or add `set-grid` class | **[CSS-ONLY]** once selector found |
| 2 | HIGH | all `gm01_*_dom.json` | `role="dialog"` / `aria-modal` absent — focus not trapped in panel | Add `role="dialog" aria-modal="true" aria-labelledby="settings-title"` in settings.go | **[GO-STRUCTURAL]** |
| 3 | HIGH | all `gm01_*_dom.json` | `setLabelTags: ["div"]` — 23 section headers are `<div>`, no heading hierarchy | Change to `<h4>` in Go render | **[GO-STRUCTURAL]** |
| 4 | HIGH | all `gm01_*_dom.json` | 23 sections, no jump links or section nav — 3+ scroll-lengths to reach bottom sections | Add anchor jump-links at top of `.set-body` (Household · Appearance · AI · Data · Advanced) | **[GO-STRUCTURAL]** |
| 5 | MEDIUM | `gm01_*_dom.json` | 30 inputs, 8 `<label>` elements — most inputs unlabelled at the element level | Add `aria-label` directly on password inputs; add `<label for>` to unlabelled number inputs | **[GO-STRUCTURAL]** |
| 6 | MEDIUM | `gm01_*_768.png` | Panel edge 15px from viewport at 768px — X button and content nearly clip | Change `max-width:92vw` to `min(760px, calc(100vw - 24px))` | **[CSS-ONLY]** |
| 7 | MEDIUM | `gm01_*_dom.json` | No save-success feedback — Save closes panel silently | Post "Settings saved" via `noticeAtom` on Save() | **[GO-STRUCTURAL]** |
| 8 | MEDIUM | `gm01_*_dom.json` | 4 identical-styled Import buttons — no visual differentiation | Add icon glyph prefix `↑` or subtle label prefix; unique icons per import type | **[CSS-ONLY]** / **[GO-STRUCTURAL]** |
| 9 | LOW | `gm01_*_dom.json` | `height:470px` base in `.flip-wrap` — on short viewports (<600px) panel would be only 516px | Raise base to `min(86vh, 680px)` | **[CSS-ONLY]** |
| 10 | LOW | `gm01_*_dom.json` | Toggle count: 26 rows but only 18 switches — 8 toggle-row elements have no switch | Audit which rows are missing controls | **[GO-STRUCTURAL]** |

---

**G21 cross-reference — which issues are now fixed vs still open**

| G21 defect | Status in GM1 |
|---|---|
| #1 Toggle labels white-on-white in light | **FIXED** — `rgb(28,28,30)` confirmed ✓ |
| #2 Panel backdrop dark in light | **FIXED** — `rgba(239,237,232,0.75)` confirmed ✓ |
| #3 Five identical "Import" buttons | **PARTIALLY FIXED** — "Import…" renamed to "Import dataset…"; 4 remain (not 5); all uniquely named but visually identical |
| #4 No section navigation | **OPEN** — S8 above |
| #5 Appearance buried under AI + Cloud | **RESOLVED** — current wasm puts Appearance first in right column ✓ |
| #6 `set-label` as `<div>`, few `<label>` elements | **OPEN** — S10 + S11 above |
| #7 No save confirmation | **OPEN** — S14 above |
| #8 768px overflow (25 elements) | **PARTIALLY RESOLVED** — overflow audit shows 216 elements at 768 but these are below-fold elements in the scroll container, not horizontal overflow; the 2-column-at-768 layout issue is still present (S4) |
| #9 AI hard-coded OpenAI | **OPEN** — gated on C81 |
| #10 AI toggle off, inputs still active | **OPEN** — S18 above |
| Wipe data inline Style() danger | **FIXED** — `.data-btn-danger` CSS class confirmed ✓ |
| Password inputs no aria-label | **PARTIALLY FIXED** — section-name labels present; input-level `aria-label` still missing |

---

**Probe hardening**

- Light theme boot: `localStorage.setItem('cashflux:prefs', JSON.stringify({theme:'light'}))` then
  `page.reload()` + `waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light')`
  — canonical recipe, confirmed working. `data-theme: "light"` verified before opening panel.
- Panel opened via `page.locator('button.hh').first()` — reliable; `.set-label` as the panel-ready
  signal confirmed (appears only when panel is mounted).
- `overflowCount: 213/216` in the audit is a **false positive** — the overflow check computed
  `r.bottom > window.innerHeight + 2` which flags every element inside `.set-body` that is below the
  viewport fold (the panel body scrolls internally). The count does not represent actual horizontal
  overflow. Future probes should restrict overflow checks to `r.right > window.innerWidth + 2`
  for horizontal-only, or check `el.scrollWidth > el.clientWidth` on the container elements.
- `flipWrapDims: {width:760, height:774}` — the panel renders at `max-height:86vh` (86% of 900px
  viewport = 774px), overriding the `height:470px` base. The G21 "380px scroll window" analysis
  was based on a different rendering context; at 900px viewport height the panel is comfortable.
- The `@media (max-width:768px) .flip-wrap .grid-cols-2` rule is present in index.html line 951
  but does NOT fire at viewport 768px — the runtime class is not literally `grid-cols-2`.
  Investigate what class the Go `tw.GridCols2` constant resolves to in the rendered DOM.

**Cross-references**
- G21: primary source — most findings trace back to G21 defects.
- C69: panel backdrop + toggle label light-mode fixes — now confirmed resolved.
- L47: four identical-styled Import buttons (down from five) — "Import dataset…" rename done.
- C81: AI multi-provider — gating S19; no action until C81.
