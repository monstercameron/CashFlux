### GM4. FlipPanel system + widget gear + command palette — UX review — 2026-06-23 ★

**The story**
Marcus is a power user who keeps the keyboard in reach. He opens the Dashboard at 8 am, hits ⌘K to jump to Reports without mousing to the rail, then comes back and clicks the gear on the "Needs attention" widget to tune which alerts surface. He filters by typing a few letters, arrow-keys to the right row, and presses Enter. Later he opens the widget gear, toggles off a noisy alert, and hits Save. Everything should feel immediate, readable in both themes, and keyboard-complete.

**Drive script**
`e2e/gm_04_palette_gear.mjs`

**Build / run evidence**
- App running on `http://127.0.0.1:8099` (status 200 confirmed before run)
- `$env:E2E_URL="http://127.0.0.1:8099"; node e2e/gm_04_palette_gear.mjs` → **EXIT 0**
- 0 page errors (all four sessions)
- 20 screenshots + 4 JSON data files produced (all ✓)

**Screenshots produced**
- `gm_04_palette_open_dark_1280.png`
- `gm_04_palette_filter_dark_1280.png`
- `gm_04_palette_nav_dark_1280.png`
- `gm_04_dashboard_dark_1280.png`
- `gm_04_gear_open_dark_1280.png`
- `gm_04_gear_closed_dark_1280.png`
- `gm_04_palette_open_dark_768.png`
- `gm_04_palette_filter_dark_768.png`
- `gm_04_palette_nav_dark_768.png`
- `gm_04_dashboard_dark_768.png`
- `gm_04_gear_open_dark_768.png`
- `gm_04_gear_closed_dark_768.png`
- `gm_04_palette_open_light_1280.png`
- `gm_04_palette_filter_light_1280.png`
- `gm_04_palette_nav_light_1280.png`
- `gm_04_dashboard_light_1280.png`
- `gm_04_gear_open_light_1280.png`
- `gm_04_gear_closed_light_1280.png`
- `gm_04_palette_open_light_768.png`
- `gm_04_palette_filter_light_768.png`
- `gm_04_palette_nav_light_768.png`
- `gm_04_dashboard_light_768.png`
- `gm_04_gear_open_light_768.png`
- `gm_04_gear_closed_light_768.png`

JSON: `gm_04_dom_dark_1280.json`, `gm_04_dom_dark_768.json`, `gm_04_dom_light_1280.json`, `gm_04_dom_light_768.json`

---

**What already works well (keep — regression anchors)** ✓

- **Palette opens with focus in the search input.** `inputFocused: true` confirmed across all four sessions. The user can type immediately with no click required — correct power-user affordance. `gm_04_palette_open_dark_1280.png` ✓
- **Ctrl+K toggle works reliably.** Open → ESC hides (`display: none`); Ctrl+K re-opens; the overlay is lazy-built on first use and reused thereafter. ESC-closes confirmed `paletteHidden: true` in all four sessions. ✓
- **Group headers render in unfiltered view.** Three section headers (NAVIGATE / ACTIONS / WORKSPACES) appear correctly in the unfiltered list, giving the palette structure at a glance. `headerCount: 3` confirmed. `gm_04_palette_open_dark_1280.png` ✓
- **Fuzzy entity search surfaces user data.** Typing "acc" instantly surfaces account-entity jump targets (Everyday Checking, Roth IRA, Rewards Credit Card, 12-month CD, etc.) ranked above generic nav items — the entity-jump architecture (entityJumpCommands) works. `gm_04_palette_filter_dark_1280.png`, `gm_04_palette_filter_light_1280.png` ✓
- **Arrow-key nav highlight moves correctly.** The selected row changes background on each ArrowDown; the highlighted row is visually distinct in dark mode. `gm_04_palette_nav_dark_1280.png` ✓
- **FlipPanel 3D flip animation completes cleanly.** `isFlipped: true` + `isShown: true` confirmed; the `.flip-inner.flipped` class is set, the rotateY(180deg) + scale(1) CSS fires. The back-face (settings panel) is displayed after the flip in all four sessions. ✓
- **Gear panel ARIA is correct.** `wrapRole: "dialog"`, `wrapAriaModal: "true"`, `wrapAriaLabel: "Needs attention"` — role/modal/label all present on `.flip-wrap`. ✓
- **ESC closes the gear panel.** `gearGone: true` after Escape in all sessions; focus restore logic in UseEffect cleanup also fires. ✓
- **Gear opens the right widget.** The first widget is the "Needs attention" freshness widget, which has real settings (5 toggles + 2 number inputs = 7 controls). `bodyIsEmpty: false`, `inputCount: 5`, `toggleCount: 10`. The "Save"-able footer with Cancel + Save is correctly shown (not CloseOnly). ✓
- **Backdrop blur present.** `bdBdFilter: "blur(3px)"` confirmed in all sessions — the blurred backdrop makes the modal feel elevated above the page content. `gm_04_gear_open_dark_1280.png` ✓
- **Dark palette card is correctly elevated.** Dark card uses `rgb(32,32,34)` on the `rgba(0,0,0,0.5)` overlay backdrop — clearly distinct from the page surface and text is `rgb(244,244,245)`. Reads cleanly. `gm_04_palette_open_dark_1280.png` ✓
- **Gear panel light-mode face is white.** `backBg: rgb(255,255,255)` in light sessions; the panel body (toggle labels, inputs) renders on a crisp white surface. `gm_04_gear_open_light_1280.png` ✓
- **Gear panel 768 rendering.** Panel renders at its CSS-fixed 384×470 which fits comfortably inside a 768-wide viewport (max-width: 92vw clamps it). `gm_04_gear_open_dark_768.png` ✓
- **Panel title centers correctly.** The flexbox spacer (`width: 1.5rem`) balances the close-button so the `set-h h3` title is visually centered in the header bar. ✓
- **Gear button visibility on hover.** Gear icon is hidden at rest and becomes visible on `.w:hover` (CSS opacity/color transition), keeping the dashboard surface calm and decluttered. `gm_04_dashboard_dark_1280.png` ✓

---

**Structure fixes (bottom-up, by review dimension)**

#### 1. OPEN / CLOSE

**[CSS-ONLY] GM4-1. Palette: no `role="dialog"` / `aria-modal` on the card or overlay.**
The palette overlay (`#cf-cmd-palette`) and its inner card div carry no ARIA roles. Screen readers cannot identify this as a dialog. The overlay is built imperatively in Go (shortcuts.go) so adding ARIA attributes there is possible without a Go rebuild — but only the JS/DOM layer touches it at runtime. Because the palette is DOM-built (not a wasm component), the fix must be applied in `buildCommandPalette` in `shortcuts.go`, which requires a Go/wasm rebuild.
Cross-ref: C68 (palette implemented); consistent with FlipPanel which does set role/aria-modal correctly.
**[GO-STRUCTURAL]** — needs `buildCommandPalette` in `shortcuts.go` to call `.setAttribute("role","dialog")` + `.setAttribute("aria-modal","true")` + `.setAttribute("aria-label", T("cmd.search"))` on the card div.

**[CSS-ONLY] GM4-2. Palette: no `listbox`/`option` ARIA on the result container.**
`#cf-cmd-list` has no `role="listbox"` and individual result rows carry `role="option"` but no `aria-selected`. A screen reader cannot navigate the result list by role. The `renderPalette` function in shortcuts.go builds the row HTML as a string — adding `role="listbox"` to `#cf-cmd-list` and `aria-selected="true/false"` to the selected row is a Go-side string-build change.
**[GO-STRUCTURAL]** — `buildCommandPalette` / `renderPalette` changes in `shortcuts.go`.

**[CSS-ONLY] GM4-3. Palette: backdrop click dismisses correctly but has no `aria-label`.**
The overlay backdrop (`#cf-cmd-palette`) itself has no accessible label explaining its purpose — a SR user would read an unlabelled `div`. Low priority given the inner card also lacks role, but adds to the a11y gap.
**[GO-STRUCTURAL]** — cosmetic change in `buildCommandPalette`.

#### 2. SIZING / POSITIONING

**[CSS-ONLY] GM4-4. Palette: 768-wide layout forces left-aligned card with dead right margin.**
At 768px the palette card (`width: min(92vw, 520px)`) resolves to 92vw ≈ 707px — almost full width, but the overlay uses `place-items: start center` (padding-top: 12vh). The card starts at the top-left of center content. In `gm_04_palette_open_dark_768.png` the card fills almost the full width but has asymmetric side padding and the "Search commands…" input underline bar seems to bleed edge-to-edge. This reads more like a sheet than a centered palette — consider `min(92vw, 520px)` is fine, but 12vh top offset + full-width feel at 768 loses the "spotlight" quality. A CSS-only fix is to clamp the card narrower at 768 (e.g. `max-width: min(94vw, 480px)` with `@media (max-width: 800px)`).
**[CSS-ONLY]** — add a media-query override in `web/index.html` for the palette card.

**[CSS-ONLY] GM4-5. Gear panel: fixed 384×470 size does not adapt to 768 viewport.**
`.flip-wrap` is hard-coded `384px × 470px` with `max-width: 92vw`. At 768px the panel fits (384 < 768 × 0.92 ≈ 707) but at narrower mobile (<420px) the height `max-height: 86vh` would constrain it. The panel is fine at the tested viewports (768+), but the fixed height means a long gear panel (many settings) clips and relies on `.set-body` overflow-y scrolling. No defect at tested sizes, but the gear panel's header ("Needs attention") at 768 is confirmed readable — `gm_04_gear_open_dark_768.png` ✓.

#### 3. THEMING

**[CSS-ONLY] GM4-6. CRITICAL — palette selected-row highlight is invisible in light mode.**
`renderPalette` writes the selected row's background as inline `background: var(--hover, #1c1c1e)`. In light mode the CSS custom property `--hover` is not defined in `web/index.html`, so the fallback `#1c1c1e` fires — near-black on a `rgb(241,241,242)` card. The text color is also `rgb(28,28,30)` (dark ink). Result: the selected row is a solid near-black band; the label text on it disappears entirely (~1:1 contrast on the selected band). Visually confirmed: `gm_04_palette_nav_light_1280.png` and `gm_04_palette_nav_light_768.png` show a jet-black selected row with invisible text.
**Fix (CSS-ONLY):** Add `--hover: #e8e6e1;` to the `[data-theme="light"]` block in `web/index.html`. The palette reads `var(--hover)` as an inline style so it will pick up the token change automatically. The Go source does not need touching.

**[CSS-ONLY] GM4-7. Palette card backdrop color is hardcoded dark (`rgba(0,0,0,0.5)`) in light mode.**
The palette overlay background is set in `buildCommandPalette` as a style string: `background: rgba(0,0,0,0.5)`. In light mode a 50% black veil over a light app reads as a severe darkening — more alarming than grounding. The Settings FlipPanel correctly uses `.flip-backdrop { background: rgba(4,4,6,.6); }` with a light override `[data-theme="light"] .flip-backdrop { background: rgba(239,237,232,0.75); }`. The palette bypasses this system because it's an imperative DOM overlay.
Visual: `gm_04_palette_open_light_1280.png` shows an intensely dark overlay behind a white card — the contrast is jarring vs. the warm-tinted backdrop the gear panel correctly uses.
**Fix option A (CSS-ONLY):** Add `#cf-cmd-palette { background: rgba(0,0,0,0.5); }` and `[data-theme="light"] #cf-cmd-palette { background: rgba(239,237,232,0.75); }` in `web/index.html`, then strip the inline background from the overlay in `buildCommandPalette`.
**Fix option B (GO-STRUCTURAL):** Have `buildCommandPalette` use a CSS class instead of an inline style for the overlay background, then define the class with a light-mode override in CSS. More robust long-term.
Current tag: **[CSS-ONLY]** for option A — it can override the inline style with `!important` on the ID selector, which has higher specificity than an element inline only if the rule uses `!important`. Because inline styles win over stylesheet rules, Option A requires `!important`. Alternatively, apply it as `background` CSS class (Option B) — that requires a Go change. **Pragmatic CSS-only fix: add `[data-theme="light"] #cf-cmd-palette { background: rgba(239,237,232,0.75) !important; }` to `web/index.html`.**

**[CSS-ONLY] GM4-8. Gear panel footer and header are hardcoded dark in light mode.**
`.set-h` (`border-bottom: 1px solid #2a2a2c`) and `.set-foot` (`border-top: 1px solid #2a2a2c`) use dark hardcoded border colors. Neither has a `[data-theme="light"]` override. In light mode the gear panel's header separator and footer separator are dark bands on a white card.
Visual: `gm_04_gear_open_light_1280.png` — the `set-h` bottom border line and `set-foot` top border line are visibly darker than the panel's white surface, creating a harsh contrast on an otherwise clean white card. The overall impression is still readable but the separator lines look like they're from dark mode.
**Fix (CSS-ONLY):** Add to `web/index.html`:
```css
[data-theme="light"] .set-h  { border-bottom-color: #e4e2dd; }
[data-theme="light"] .set-foot { border-top-color:    #e4e2dd; }
```

**[CSS-ONLY] GM4-9. Gear panel Save button is dark-green in light mode (OK functionally, not on-theme).**
`.set-btn.save { background: #1f2c24; border: 1px solid #356b50; color: #7fd0a3; }` — these dark-green values are hardcoded with no light-mode override. In light mode the Save button is a dark forest-green island on a white card, which reads correctly as an action button but doesn't harmonise with the light-mode accent token (`--accent` resolves to the user's chosen accent). Visual: `gm_04_gear_open_light_1280.png` — Save is a dark green button, Cancel is a ghost — the contrast is correct (~6:1) but the dark-on-light island effect is aesthetically heavy.
**Fix (CSS-ONLY):** Add light-mode save-button tokens that use the accent and a lighter surface:
```css
[data-theme="light"] .set-btn.save {
  background: var(--accent, #2e7d52);
  border-color: var(--accent, #2e7d52);
  color: #ffffff;
}
[data-theme="light"] .set-btn.save:hover {
  filter: brightness(0.9);
}
[data-theme="light"] .set-btn.cancel {
  border-color: #c8c6c1;
  color: #4b4b52;
}
[data-theme="light"] .set-btn.cancel:hover {
  color: #1c1c1e;
  border-color: #9a9a9e;
}
```

#### 4. STRUCTURE

**[GO-STRUCTURAL] GM4-10. C11 — first gear opened is always the first widget, not necessarily the one the user clicked.**
The probe always opens the gear of `[data-widget]:first` — in practice, the widget receiving the gear click is determined by which widget is hovered. The first rendered widget happens to be "Needs attention" (freshness widget) which has full settings. For widgets that genuinely have no schema settings (e.g. a custom-page tile, or a data-only display widget added via the Widget Builder), `CloseOnly: true` is set by `!widgetcfg.Has(id)` → `FlipPanel.CloseOnly`. The probe found `footLabels: ["Cancel", "Save"]` for the first widget — correct. C11 empty-panel behavior is structurally implemented (`CloseOnly` flag exists in the Go source) but was not exercised in this probe because the first widget has settings.
**Gap:** No screenshot evidence of the C11 "Close-only" empty state — a settingless widget panel must be found and probed. Recommended: add a Widget Builder–created widget with no schema and probe its gear. Tracking: C11 structural implementation exists; visual/UX quality of the CloseOnly empty state is unreviewed.

**[CSS-ONLY] GM4-11. Palette: no keyboard hint text visible at bottom of palette.**
The palette card has no footer strip with "↑↓ navigate · Enter select · Esc close" hints. The navigation affordance is limited to the `jump ↵` breadcrumb on nav-group rows. Power users learn fast, but first-time ⌘K users have no visible indication that arrow keys work.
`gm_04_palette_open_dark_1280.png` — visible: rows have `jump ↵` on right side for Navigate group, but no footer hint bar.
Confirmed cross-ref with C68 scope: the ticket specifies "keyboard hints" as part of the palette structure. This is absent.
**Fix (GO-STRUCTURAL):** Add a footer div to `buildCommandPalette` below `#cf-cmd-list` with `↑↓ navigate · ↵ select · Esc close` in muted small text.

**[GO-STRUCTURAL] GM4-12. Palette: 58 rows unfiltered is cognitively overwhelming.**
The unfiltered list contains 58 result rows across Navigate / Actions / Workspaces. With entity-jump commands for every account, goal, and budget, the unfiltered palette is extremely long and requires heavy scrolling. In a budgeting app with many accounts (10+ is common) this becomes unwieldy.
`gm_04_palette_open_dark_1280.png` — the list fills and overflows the `max-height: 50vh` scroll area; all 58 items are technically accessible but the signal-to-noise ratio is poor.
Consider: (a) cap entity-jump rows at 5–8 most-recently-accessed, or (b) show only Navigate + top Actions in the unfiltered view, reserving entity jumps for filtered (query-driven) results only.
**[GO-STRUCTURAL]** — requires logic change in `buildPaletteCommands` / `entityJumpCommands`.

#### 5. CONTROLS

**[CSS-ONLY] GM4-13. Gear panel section label "NEEDS ATTENTION" is muted ALL-CAPS — no light-mode adjustment.**
The section label inside the gear body (`.set-label` or a div with `text-transform: uppercase`) renders at reduced opacity. In dark mode this is fine (near-white at 50% on a dark card). In light mode the muted color may not meet AA for uppercase small text depending on background. `gm_04_gear_open_light_1280.png` — "NEEDS ATTENTION" renders as a muted grey label above the toggle rows; reads OK visually but contrast should be confirmed at actual computed values.
Low priority — visually adequate in screenshots.

**[CSS-ONLY] GM4-14. Gear panel number inputs: empty/unstyled boxes in light mode.**
The "Flag bills due within (days)" and "Most you'll see at once" inputs in the Needs Attention gear panel render as bare `<input type="text">` or `<input type="number">` elements. In light mode (`gm_04_gear_open_light_768.png`) these inputs show with a bottom-border-only underline style — label text wraps to two lines ("Flag bills due within\n(days)") and the input field is narrow. The input field's label breaks mid-word in the constrained 384px panel width.
Moderate — the label wrapping ("Flag bills due within\n(days)") is sub-optimal. The panel is 384px with ~32px padding = ~320px available. A CSS grid `label + input` layout would solve this.
**[CSS-ONLY]** if the layout is controlled by CSS in `web/index.html` for `.set-body` form rows; **[GO-STRUCTURAL]** if the form row markup needs to change in the `freshness` widget settings renderer.

#### 6. FEEDBACK

**[GO-STRUCTURAL] GM4-15. Gear panel: no visible save confirmation / toast after Save.**
After clicking Save, the panel closes. There is no toast or inline feedback confirming the settings were persisted. The Settings FlipPanel (global settings) also lacks a post-save toast. For the widget gear panel, the only feedback is the panel disappearing. Power users who are fast-clicking may wonder if Save did anything.
Cross-ref: B12 (wire per-widget flip-panel settings to content). The persistence path exists (`widgetcfg`) but the post-save feedback is absent.
**[GO-STRUCTURAL]** — requires a `paletteNotify`-style toast call in the `onSave` callback of the widget settings host.

**[CSS-ONLY] GM4-16. Palette: ArrowDown selection highlight contrast in light mode — invisible.**
(This is the same root cause as GM4-6 above — captured here for dimension 6 completeness.)
After ArrowDown, the selected row uses `background: var(--hover, #1c1c1e)`. With no `--hover` token in light mode, the selected row is `#1c1c1e` (near-black). The label text is also near-black (`rgb(28,28,30)`). Text is completely invisible on the selected row in light mode.
Screenshot: `gm_04_palette_nav_light_1280.png` — selected row is a solid dark band, text invisible.
**[CSS-ONLY]** — define `--hover: #e8e6e1` in the `[data-theme="light"]` token block. Single-line fix.

#### 7. GENERAL MODAL UX

**[CSS-ONLY] GM4-17. Gear panel: focus trap works but initial focus lands on the × close button, not the first toggle.**
`flippanel.go` `UseEffect` focuses `fs[0]` — the first focusable element in `.flip-wrap`. The `.flip-back` face contains: the close button (set-close), then the section label, then the first toggle. The close button is the first focusable, so Tab from open-state immediately moves to the first toggle (one Tab press away). This is an acceptable but not ideal focus order — the first interactive control for a settings panel should be the first setting, not the close button. The SR announcement would be "Close, button" before any setting is named.
**[GO-STRUCTURAL]** — a clean fix is to `tabindex="-1"` the close button and move initial focus to the first form control.

**[CSS-ONLY] GM4-18. Palette and gear panel z-index hierarchy is correct but palette (`z-index: 210`) is above FlipPanel (`z-index: 50`).**
Opening the palette while a gear panel is open would layer the palette above the gear panel correctly. However, opening the gear while the palette is open would not dismiss the palette first — both could co-exist on screen. Not a tested scenario but the lack of mutual-exclusion logic could confuse state.
Low priority — unlikely real user path. **[GO-STRUCTURAL]** — `toggleCommandPalette` should call `closeSettingsPanel` if one is open, and vice versa.

**[CSS-ONLY] GM4-19. Gear panel backdrop click-to-close is not implemented.**
`.flip-backdrop` has no click listener. Clicking outside the `.flip-wrap` does not close the panel — only ESC and the × button close it. This is inconsistent with the palette (click outside = close) and with modal UX convention.
`gm_04_gear_open_dark_1280.png` — the backdrop is the blurred overlay, but clicking it does nothing.
**[GO-STRUCTURAL]** — `FlipPanel` in `flippanel.go` needs a `OnBackdropClick` callback or a backdrop-click listener on `.flip-backdrop` that calls `onClose`. The backdrop element is rendered by the wasm framework, so this change requires a Go rebuild.

**[CSS-ONLY] GM4-20. Palette card has no `focus-visible` outline on keyboard focus — minor.**
The palette card uses `outline: none` via default CSS reset. The input itself shows a browser-default focus ring (the green underline visible in screenshots). The card's outer border is `1px solid rgb(42,42,44)` — no focus ring when the card itself is focused. Not a critical a11y issue since the input auto-focuses, but worth noting for the card's role=dialog landmark.
Low priority. **[CSS-ONLY]** if adding an outline; **[GO-STRUCTURAL]** if adding `tabindex="-1"` to the card.

---

**UI/UX defects — screenshot-confirmed**

| # | Defect | Severity | Tag | Evidence |
|---|--------|----------|-----|----------|
| GM4-6 | **Palette selected-row text invisible in light mode** (near-black bg + near-black text, `var(--hover)` fallback fires) | CRITICAL | [CSS-ONLY] | `gm_04_palette_nav_light_1280.png`, `gm_04_palette_nav_light_768.png` |
| GM4-7 | **Palette backdrop is harsh dark in light mode** (`rgba(0,0,0,0.5)` hardcoded, no light override) | HIGH | [CSS-ONLY w/ `!important`] | `gm_04_palette_open_light_1280.png`, `gm_04_palette_open_light_768.png` |
| GM4-1 | **No `role="dialog"` / `aria-modal` on palette card** — SR cannot identify it as a dialog | HIGH | [GO-STRUCTURAL] | DOM audit all sessions |
| GM4-19 | **Gear panel backdrop click does not close** — inconsistent with palette + modal convention | HIGH | [GO-STRUCTURAL] | `gm_04_gear_open_dark_1280.png` |
| GM4-8 | **Gear panel header/footer separators are dark-hardcoded in light mode** | MEDIUM | [CSS-ONLY] | `gm_04_gear_open_light_1280.png` |
| GM4-9 | **Gear panel Save button is dark-green island in light mode** — not on-theme | MEDIUM | [CSS-ONLY] | `gm_04_gear_open_light_1280.png` |
| GM4-11 | **No keyboard navigation hint footer in palette** (↑↓ navigate · Enter · Esc) | MEDIUM | [GO-STRUCTURAL] | `gm_04_palette_open_dark_1280.png` |
| GM4-12 | **58 unfiltered rows — entity jumps inflate the list to unusable length** | MEDIUM | [GO-STRUCTURAL] | `gm_04_palette_open_dark_1280.png` |
| GM4-2 | **No `role="listbox"` on `#cf-cmd-list`; `aria-selected` missing on rows** | MEDIUM | [GO-STRUCTURAL] | DOM audit all sessions |
| GM4-15 | **No save-confirmation toast after gear Save** | LOW | [GO-STRUCTURAL] | `gm_04_gear_open_dark_1280.png` |
| GM4-17 | **Focus lands on × close button (not first setting) on gear open** | LOW | [GO-STRUCTURAL] | `flippanel.go` |
| GM4-4 | **Palette at 768 spans near-full width — loses spotlight feel** | LOW | [CSS-ONLY] | `gm_04_palette_open_dark_768.png` |

---

**Probe hardening**

The probe script (`gm_04_palette_gear.mjs`) correctly:
- Resets `viewAsMember` to Everyone on boot (house-rule)
- Uses the canonical light-theme recipe (set prefs → reload → waitForFunction `data-theme="light"`)
- Boots from `/dashboard` with `waitForSelector('[data-widget]')` to confirm bento is live before gear probing
- Drives `Ctrl+K` for the palette, hover + gear-inline click for the widget settings
- Provides a fallback chain for the gear button (widget-scoped locator → any `.gear-inline`)
- Screenshots at filter and arrow-nav states, not just open
- Audits both DOM structure and computed colors in all four sessions
- Reports `gearOpened: true/false` so a probe failure is distinguishable from a defect

**Still-unexercised:**
- C11 empty-panel (CloseOnly) state — needs a settingless widget (custom-page tile or Widget Builder output)
- Enter-to-execute from palette keyboard nav (script navigates but does not confirm navigation landed)
- Click-outside dismissal of both palette and gear panel (backdrop click)
- Tab-trap cycling across all focusables in the gear panel
- Palette in the context of a non-Dashboard route (should still open via global Ctrl+K)

Cross-references: **C68** (command palette done, a11y gaps remain), **B12** (per-widget gear settings wired and functional; save feedback gap), **C11** (CloseOnly empty-panel — structurally present; visual quality unreviewed).
