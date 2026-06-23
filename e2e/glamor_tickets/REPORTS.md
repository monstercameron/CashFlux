### G9.1. Reports — DEEP beautification + redesign spec (extends G9, from C55) — 2026-06-23 ★★

---

## The story

**Priya's monthly review.** It is the 3rd of July. Priya opens Reports to answer one question: *where did the money go this month?* She has 90 seconds. She needs to land on net in/out/net, see the biggest spending categories at a glance, spot anything unusual, and close the tab. Right now she gets a single-column scroll of uniform cards — 13 of them, stacked identically, no visual weight difference between the headline figure and the eighth card in the list. Every card whispers at the same volume. The page is correct; it is not glanceable.

---

## Drive script

```
node e2e/reports_beautify_analysis.mjs
```

Exit code: **0**. Screenshots written (6 total):

| File | Width |
|---|---|
| `e2e/reports_dark_768.png` | 768px dark |
| `e2e/reports_dark_1280.png` | 1280px dark |
| `e2e/reports_dark_1440.png` | 1440px dark |
| `e2e/reports_light_768.png` | 768px light |
| `e2e/reports_light_1280.png` | 1280px light |
| `e2e/reports_light_1440.png` | 1440px light |

Computed-style measurements: `e2e/reports_metrics.json`. Source measured: `reports_screen.go` (778 lines) and `web/index.html`.

---

## Deep analysis

### 1. Information architecture — section order and grouping

**Current DOM order** (confirmed from `internal/screens/reports_screen.go` L384–531):

1. Period caption (`.t-caption` 12px dim-gray) — smallest text, first element
2. Stat grid — 6 uniform tiles: income / spend / net / savings rate / runway / no-spend
3. Spend trend sentence + spend stats — two floating `.muted` paragraphs, no container
4. Heads Up card (conditional) — same `.card` wrapper as everything else
5. `SPENDING THIS PERIOD` section divider
6. Spending by category card — rollup toggle + narrative + share bars + 2 export buttons
7. `INCOME & MONEY FLOW` section divider
8. Money flow Sankey card (conditional)
9. Biggest deposits card
10. Income by source card + CSV export
11. Top payees card + CSV export
12. Biggest expenses card + CSV export
13. By member card (conditional) + CSV export
14. `TRENDS` section divider
15. Cash flow trend AreaChart card
16. Net worth composition stat-grid card
17. Net worth trend AreaChart card
18. Savings-rate trend AreaChart card
19. Custom field spend card (conditional)
20. Deductible totals card (conditional)

**Measured (metrics.json, all 6 captures):** 13 cards, 37 rows, 3 section dividers, 30 share bars, 6 CSV export buttons. Scroll height = 900px at 900px viewport — app has seed data that fits the screen. With a full year of real data and all conditional sections visible, estimated 6–8 viewports of scroll.

**Problems:**

- The Sankey is card 8, after a full wall of category text rows. It is the richest visual on the page — income fanning to categories — and Priya has to scroll past the entire category list to reach it.
- "Heads up" anomaly card (the most urgent content) floats above the section divider with identical visual weight to a trends chart.
- "Biggest deposits" and "Income by source" are near-redundant adjacent cards. "Top payees" and "Biggest expenses" are also near-redundant. Neither pair is visually differentiated or explained.
- The net worth composition stat-grid sits between two area-chart cards (NW trend before, savings-rate after) — it interrupts the trend visual rhythm.
- Section dividers render unconditionally — when their content group is empty (no income data), the divider floats alone.

**Proposed target IA:**

```
[HEADLINE ZONE — no card, hero numbers on page bg]
  Period caption (promoted weight)
  Net figure: display size (2.5rem/800)
  Income + Spend flanking: 1.75rem/700

[ANOMALIES — not a card, accent callout strip]
  Heads up alerts (if any)

SPENDING THIS PERIOD
  Card: Spending by category (share bars, narrative)
  Card: Money flow Sankey    ← moved up from position 8
  Card: Top payees
  Card: Biggest expenses

INCOME
  Card: Income by source (with share bars added)
  Card: Biggest deposits

HOUSEHOLD (conditional)
  Card: By member

TRENDS
  Card: Cash flow trend (AreaChart)
  Card: Net worth + composition (merged: stat-grid + area chart in one card)
  Card: Savings-rate trend (AreaChart)

ADVANCED (collapsed chevron by default)
  Card: Custom field spend
  Card: Deductible totals
```

Target scroll depth: ≤ 4 viewports with all sections showing. Net answer is above the fold at every width.

---

### 2. Visual hierarchy — focal point

**Measured (all 6 captures consistent):**

- `.card-title` font-size: **16.8px**, font-weight: **400** (body-text weight — no CSS `font-weight` set on `.card-title`, defaults to 400)
- `.stat-value` font-size: **24px**, font-weight: **700**
- `.row-desc` font-size: **14.5px**, font-weight: **400**
- All 3 card-title samples: identical `16.8px / 400` — every card header is the same weight

**Visual evidence (reports_dark_1280.png, reports_light_1280.png):** The six stat tiles are the only element with visual prominence (24px/700). Everything else — card titles, row labels, section dividers, narrative — clusters between 11–17px at weight 400. There is no hierarchy between "Spending by category" (primary) and "By member" (secondary). Net is the same tile size as "No-spend days."

**Problems:**

- Card titles at `font-weight: 400` are indistinguishable from body text. `index.html` L353: `.card-title { margin: 0 0 0.75rem; font-size: 1.05rem; color: var(--text); }` — no `font-weight`.
- The rollup toggle button is a full `.btn` (14.5px, 6.4px×12.8px padding) sharing the card header row with the 16.8px card title — the control dominates the row.
- "Heads up" card has no visual urgency signal — same white card / 0px border-radius as a trends chart.
- Period caption at 12px / `rgb(60,60,67)` (light) — the smallest text on the page answers the most important context question (which period?).

**Proposed:**

- Hero strip for Net / Income / Spend: `2.5rem / 800` / `1.75rem / 700` respectively, above the card flow.
- Card title weight by tier: primary (category, Sankey) → `font-weight: 600`; secondary (payees, deposits, trends) → `font-weight: 500`.
- Rollup toggle: demote to `font-size: 0.78rem; padding: 0.2rem 0.5rem` ghost button.
- Heads up card: `border-left: 4px solid var(--danger)` + faint danger background tint.
- Period caption: promote to `0.825rem / 500`.

---

### 3. Typography — the measured scale

**All 6 captures consistent:**

| Element | CSS class | Measured size | Weight | Color (dark) | Color (light) |
|---|---|---|---|---|---|
| Card title | `.card-title` | 16.8px | **400** | rgb(244,244,245) | rgb(28,28,30) |
| Row label | `.row-desc` | 14.5px | **400** | rgb(46,139,87)* | rgb(28,28,30) |
| Stat value | `.stat-value` | 24px | 700 | rgb(46,139,87) | rgb(46,139,87) |
| Stat label | `.stat-label` | 12.8px | — | rgb(171,171,179) | rgb(60,60,67) |
| Muted/narrative | `.muted` | 14.5px | — | rgb(171,171,179) | rgb(60,60,67) |
| Period caption | `.t-caption` | 12px | — | rgb(171,171,179) | rgb(60,60,67) |
| Section divider | `.section-divider` | 11.84px | 700 | rgb(108,108,113) | rgb(104,104,112) |
| Export button | `.btn` | 14.5px | — | rgb(244,244,245) | rgb(28,28,30) |
| Row amounts | `.budget-amount` | 14.5px | — | rgb(171,171,179) | rgb(28,28,30) |

*`row-desc` dark color is accent-green because the first `.row-desc` is also a `.btn-link` drill-through button (reports_screen.go L671). Non-link rows resolve correctly.

**Problems:**

- Only **two effective size steps** above baseline: 24px (stats) and 11–17px (everything else). No display tier, no mid-title tier.
- Card titles at 400 are body weight. Section hierarchy does not exist in the type scale.
- `.budget-amount` in dark: `rgb(171,171,179)` = same dim color as stat labels and metadata. Dollar amounts — the most scannable data — have the least contrast in dark mode.
- No `font-variant-numeric: tabular-nums` on `.budget-amount`. Dollar columns do not align.

**Proposed type ramp:**

| Tier | Use | Size | Weight |
|---|---|---|---|
| Display | Net hero | 2.5rem | 800 |
| Hero | Income / Spend flanking | 1.75rem | 700 |
| Stat | Stat grid values | 1.5rem | 700 (keep) |
| Title-L | Primary cards (category, Sankey) | 1.1rem | 600 |
| Title-S | Secondary cards | 1.0rem | 500 |
| Body | Row labels, narrative | 0.9rem | 400 (keep) |
| Amount | Dollar figures in rows | 0.9rem | 600 + tabular-nums |
| Caption | Period caption, stat labels | 0.8rem | 500 (promote weight) |
| Meta | Row meta, section dividers | 0.74–0.8rem | 600–700 (keep) |

---

### 4. Color and semantics

**Measured (metrics.json):**

| Surface | Dark | Light | Issue |
|---|---|---|---|
| Card bg | rgb(18,18,20) | rgb(255,255,255) | OK |
| Card border | rgb(42,42,44) | rgb(228,226,221) | OK |
| Card border-radius | **0px** | **0px** | Sharp corners everywhere |
| Spending stat value | rgb(46,139,87) | rgb(46,139,87) | **SEMANTIC BUG — should be danger red** |
| Net stat (negative) | rgb(220,38,38) | rgb(220,38,38) | Correct |
| `.budget-amount` dark | rgb(171,171,179) | rgb(28,28,30) | Dark: dim-gray for dollar amounts |
| Share bar fill | var(--accent) green | var(--accent) green | No semantic meaning, no category color |

**Critical defect D-3 (spending stat colored green, screenshots reports_dark_1280.png + reports_light_1280.png):** "SPENDING $4,068.00" renders in accent-green (`rgb(46,139,87)`) — identical to the income color. The Go code at L391 correctly passes `"neg"` to the `stat()` helper, which should apply `.stat-value.neg { color: var(--danger) }`. But `index.html` L699 has a later rule `.stat-value { color: var(--text); }` with no `.neg` qualifier — it wins by source order and overwrites the danger color. Fix: add `!important` to the `.neg` rule. **CSS-ONLY.**

**Card border-radius = 0px (D-1):** This is the highest-impact cosmetic defect on the page. Every card, stat tile, and share bar has perfectly square corners. Modern financial UI uses 8–12px radius consistently. This single change transforms the page's character from "spreadsheet" to "product." Confirmed from `card.radius: "0px"` in metrics.json across all 6 captures, and visually in all 6 screenshots.

---

### 5. Data visualization — the C55 core problem

**Measured:** 30 share bars, 0 area charts in `areaCharts` count (class selector mismatch — charts render but are below fold at 900px viewport height or the class is not matched by `[class*="chart"]`). Sankey: 978×403px at 1280px width.

**Share bar defects (confirmed from metrics.json + screenshots):**

- Height: 4px (inline `style="height:4px"`) — reads as a decorative underline, not a data bar
- Max-width: 260px (inline `style="max-width:260px"`) — at 1280px the bar fills ~27% of the available card width, leaving 73% empty white/dark space to the right of every row
- Track background: `var(--border)` — `rgb(42,42,44)` in dark, nearly invisible against `rgb(18,18,20)` card bg
- Fill: always `var(--accent)` green — no category-level color, no semantic differentiation
- The bar is below the label text within `.row-main` — requires three vertical zones per row to scan (label → bar → amount)

**Area chart defects (from source code + G9/L61 prior notes):**

- `AreaChartProps` at reports_screen.go L509, L522, L527 pass no `Labels` field → x-axis shows integer indices 0–5, not calendar period labels
- Axis label fix requires computing `[]string` from the `bounds` slice and passing to `AreaChartProps.Labels`

**Proposals:**

1. **Share bar height 4px → 8px** — CSS override on `.share-bar` and `.share-bar > div` (CSS-ONLY partial, max-width fix needs Go)
2. **Share bar max-width: 260px → 100%** — remove the inline cap in reports_screen.go L216 (GO-STRUCTURAL)
3. **Move Sankey above category list** — DOM reorder in reports_screen.go return block (GO-STRUCTURAL)
4. **Area chart calendar labels** — compute labels from bounds, pass to AreaChartProps (GO-STRUCTURAL)
5. **Category color coding** — inject `--cat-idx` CSS var per row, use `hsl(calc(var(--cat-idx)*37deg), 55%, 55%)` as bar fill (GO-STRUCTURAL)

---

### 6. Spacing and rhythm

**Measured (metrics.json, 1280px):**

| Property | Value |
|---|---|
| Card padding | 20px (1.25rem) |
| Card margin-bottom | 16px (1rem) |
| Card border-radius | 0px |
| Row padding (top + bottom) | 9.6px each (0.6rem) |
| Stat grid gap | 12px |
| Section divider margin-top | 25.6px (1.6rem) |
| Section divider margin-bottom | ~9.6px (0.6rem) |

**Problems:**

- Card margin-bottom (16px) is tighter than card padding (20px). Content inside a card has more breathing room than the gap between cards — inverted rhythm.
- No spacing distinction between within-section card gaps and between-section card gaps. The section divider provides textual grouping but no spatial separation.
- Section divider margin-top (25.6px / 1.6rem) is only marginally larger than the card margin-bottom (16px) — the sections do not feel spatially distinct.

**Proposed (CSS-ONLY):**

```css
.card { margin-bottom: 1.25rem; }  /* 20px — match internal padding */
.section-divider { margin-top: 2.25rem; margin-bottom: 0.85rem; }
```

---

### 7. Charts detail

**Sankey:**

- Measured: 978×403px at 1280px, 468×314px at 768px — good proportions, fills the card
- Default Mermaid font is browser default (not the app type stack)
- Node/link colors are Mermaid defaults (not the app semantic palette)
- No legend; no "Income" node label explanation
- CSS-injectable fix: `.mermaid svg text { font-family: inherit; font-size: 13px; }`

**Share bars:** see §5 above. Key numbers: 4px height, 260px max-width inline, `var(--border)` track (near-invisible in dark), `var(--accent)` fill.

**Area charts:**

- 3 instances (cash flow L509, net worth L522, savings rate L527)
- SVG elements measured as `{w:0, h:0}` at the 900px capture viewport — charts are below fold or not initializing in the stale-wasm build
- No `Labels` field passed → x-axis shows indices, not calendar dates

---

### 8. Responsive — 768px behavior

**Measured (reports_dark_768.png, reports_light_768.png):**

- Stat grid at 768px: 3 columns (161px each) → two rows of 3 — correct reflow
- Top bar splits into three stacked rows: resolution pills / date nav / icon row — cramped but functional
- "Roll up sub-categories" button: at 768px it occupies ~40% of the card header width, competing visually with the card title
- Category rows: share bar 260px in a ~500px card — proportionally less empty space than at 1440px but still sub-optimal
- Sankey at 768px: 468×314px — readable but label collisions possible
- No horizontal overflow observed — all content reflows

**Problems specific to 768px:**

- The three-row top bar layout wastes ~120px of screen height before the content begins
- "Roll up sub-categories" label wraps at narrow width
- Share bars at 260px still do not use the full row width

---

### 9. Empty / loading states

**Observed:** With minimal data, `EmptyStateCTA` renders for the category section ("No spending data for this period. Add your first transaction."). The stat grid renders with all-zero values. Section dividers render even when their content group is empty.

**Problems:**

- An all-zero stat grid reads as a load failure, not an empty state. When `flow.Income == 0 && flow.Expense == 0`, a top-level empty-state message should replace or precede the stat grid.
- Section dividers ("INCOME & MONEY FLOW", "TRENDS") render with no content cards below them when data is absent — orphaned headers.
- No skeleton/shimmer — the page flickers from blank to populated.

---

### 10. Export UX — 6 CSV buttons

**Measured (all 6 captures):** `exportBtns: 6` — all full `.btn` (14.5px, 6.4px×12.8px padding).

Locations: Spending by category card (2 buttons: CSV + Tax Summary), Income by source, Top payees, Biggest expenses, By member.

**Problems:**

- 6 "Download CSV" buttons with no label differentiation — the user must read the card heading to know what they are downloading
- Tax Summary button sits beside the category CSV button with identical styling but materially different behavior (always calendar-year, regardless of viewed period)
- 6 full-size buttons across 5 cards are the dominant source of visual noise in the lower half of the page
- At 768px the button rows add significant card height

**Proposed:** Consolidate into a single "Export" dropdown in the page-level toolbar. Each option labeled by content: "Spending by category", "Income by source", etc. Removes all 6 inline button rows. **[GO-STRUCTURAL]**

**CSS-ONLY interim:** demote export buttons to small ghost links so they recede:

```css
.card .btn[title*="Download"], .card .btn[title*="Tax"] {
  font-size: 0.78rem;
  padding: 0.2rem 0.55rem;
  opacity: 0.65;
}
.card .btn[title*="Download"]:hover, .card .btn[title*="Tax"]:hover {
  opacity: 1;
}
```

---

## Quick wins — land now (CSS-ONLY)

Listed highest-impact first. All target `web/index.html` only. No Go changes, no wasm rebuild.

- [ ] **QW-1. Card and stat border-radius.** The single highest-impact cosmetic fix. Confirmed: `card.radius: "0px"` across all 6 captures; visible as hard square edges in every screenshot. Transforms the page character from spreadsheet to product.

  ```css
  .card { border-radius: 12px; }
  .stat { border-radius: 10px; }
  [data-density="compact"] .card { border-radius: 8px; }
  ```

- [ ] **QW-2. Card title font-weight 400 → 600.** Confirmed: `cardTitle.weight: "400"` in all 6 captures. `index.html` L353 sets `font-size: 1.05rem` with no `font-weight` — browser defaults to 400. Bumping to 600 creates section hierarchy.

  ```css
  .card-title { font-weight: 600; }
  ```

- [ ] **QW-3. Fix spending stat value color — !important on .stat-value.neg.** Confirmed defect: spend value is `rgb(46,139,87)` green in all 6 captures. Root cause: `index.html` L699 `.stat-value { color: var(--text); }` wins over `.stat-value.neg { color: var(--danger); }` by source order. Fix:

  ```css
  .stat-value.neg { color: var(--danger) !important; }
  .stat-value.pos { color: var(--accent) !important; }
  ```

- [ ] **QW-4. Share bars height 4px → 8px.** Confirmed: `shareBar.height: "4px"` in all 6 captures. The CSS can override the inner fill div height (the outer `.share-bar` max-width is inline and cannot be overridden without Go changes — that is QW-4b under redesign):

  ```css
  .share-bar { height: 8px !important; }
  .share-bar > div { height: 8px !important; }
  ```

- [ ] **QW-5. `.budget-amount` — tabular-nums + full contrast in dark.** Confirmed: `amounts.budget: "rgb(171,171,179)"` in dark — same dim color as metadata. Dollar amounts in every row list are visually de-emphasized.

  ```css
  .budget-amount { font-variant-numeric: tabular-nums; font-weight: 600; }
  .card .budget-amount { color: var(--text); }
  [data-theme="light"] .card .budget-amount { color: #1c1c1e; }
  ```

- [ ] **QW-6. Section divider spacing — increase margin-top from 1.6rem to 2.25rem.** Confirmed: `sectionDivider.marginTop: "25.6px"` (1.6rem). Barely larger than the 16px card gap — sections don't feel spatially distinct.

  ```css
  .section-divider { margin-top: 2.25rem; margin-bottom: 0.85rem; }
  ```

- [ ] **QW-7. Card margin-bottom — match internal padding.** Confirmed: `card.marginBottom: "16px"` vs `card.padding: "20px"`. Inverted rhythm — content inside breathes more than the gap between cards.

  ```css
  .card { margin-bottom: 1.25rem; }
  ```

- [ ] **QW-8. Mermaid font alignment.** Sankey uses browser-default font stack, not the app's. One CSS rule:

  ```css
  .mermaid { font-family: inherit; }
  .mermaid svg text { font-family: inherit !important; font-size: 13px !important; }
  ```

- [ ] **QW-9. Period caption — promote weight and size.** Confirmed: `caption.size: "12px"`, `caption.color: "rgb(60,60,67)"` — smallest text answers the most important context question.

  ```css
  main .t-caption:first-child {
    font-size: 0.825rem;
    font-weight: 500;
    color: var(--text-dim);
    margin-bottom: 0.85rem;
  }
  ```

- [ ] **QW-10. Demote export buttons to ghost-small.** Confirmed: 6 full `.btn` (6.4px×12.8px padding) scattered across card footers. Interim demotion:

  ```css
  .card .btn[title*="Download"], .card .btn[title*="Tax"] {
    font-size: 0.78rem;
    padding: 0.2rem 0.55rem;
    opacity: 0.65;
  }
  .card .btn[title*="Download"]:hover, .card .btn[title*="Tax"]:hover { opacity: 1; }
  ```

---

## Redesign — build-gated (GO-STRUCTURAL)

Requires changes to `internal/screens/reports_screen.go`. All blocked on GI0 import cycle fix. Listed highest-impact first.

- [ ] **R-1. Move Sankey to position 2 in DOM order — immediately after stat grid.** `reports_screen.go` L439: Sankey currently renders after the full category card (L406). Reorder the `return Div(...)` block: stat-grid → Heads Up → section-divider → Sankey → category list. Pure line-order reorder, zero logic change.

- [ ] **R-2. Hero strip for Net / Income / Spend.** Split the flat 6-tile `.stat-grid` into two zones: (a) a hero `Div` with Net at `font-size: 2.5rem / 800` flanked by Income and Spend at `1.75rem / 700` — no card border, directly on the page bg; (b) a secondary `.stat-grid` below for savings rate / runway / no-spend.

- [ ] **R-3. Share bar max-width: 260px → 100%.** `reports_screen.go` L216: inline `"max-width": "260px"`. Change to `"max-width": "100%"`. Also applies to the duplicate inline bars in `customFieldSpendSection` (L588) and `deductibleSection` (L744).

- [ ] **R-4. Area chart calendar labels.** At reports_screen.go L509, L522, L527: compute period labels from the `bounds` slice and pass to `AreaChartProps.Labels`:

  ```go
  labels := make([]string, len(bounds)-1)
  for i, b := range bounds[:len(bounds)-1] {
      labels[i] = b.Format("Jan 06")
  }
  // then: uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, Labels: labels, ...})
  ```

- [ ] **R-5. "Heads up" card — add `card-headsup` CSS class.** `reports_screen.go` L399: change `Section(css.Class("card"), ...)` to `Section(css.Class("card", "card-headsup"), ...)`. CSS:

  ```css
  .card-headsup {
    border-left: 4px solid var(--danger);
    background: color-mix(in srgb, var(--danger) 5%, var(--bg-card));
  }
  ```

- [ ] **R-6. Rollup toggle — demote to small ghost button.** `reports_screen.go` L409–412: add inline style or a `btn-xs` class to the rollup toggle so it does not compete with the 16.8px card title.

- [ ] **R-7. Consolidate 6 CSV export buttons into page-level Export dropdown.** Remove per-card export rows (L418–435, L451–455, L461–469, L474–483, L492–503). Add a single page-header Export control with labeled options: "Spending by category", "Income by source", "Top payees", "Biggest expenses", "By member", "Tax summary".

- [ ] **R-8. Suppress stat grid when all values are zero (empty-state guard).** Add: `if flow.Income == 0 && flow.Expense == 0` → render a single `EmptyStateCTA` before the stat grid, or suppress the grid. Prevents the misleading all-zeros display.

- [ ] **R-9. Suppress orphaned section dividers.** Wrap each `H3(css.Class("section-divider"), ...)` in an `If(sectionHasContent, ...)` guard so dividers do not float alone when their conditional cards are all absent.

- [ ] **R-10. Category color coding on share bars.** Inject `--cat-idx: N` as an inline CSS var on each category row's share bar outer div (N = 0, 1, 2…). CSS:

  ```css
  .share-bar > div { background: hsl(calc(var(--cat-idx, 0) * 37deg), 55%, 55%) !important; }
  ```

  This gives each category a stable hue identity across the list — lightweight legend without a full chart.

- [ ] **R-11. Merge net worth composition stat-grid and NW trend AreaChart into a single card.** Currently two separate cards (L511–518 stat-grid, L520–522 area chart) that together answer "what is my net worth trajectory." Merge: stat-grid on top (assets / liabilities / net / NW change), area chart below. Reduces card count from 13 to 12 and eliminates the rhythm interruption.

---

## What already works well (keep) ✓

- **The Sankey** — income fanning to categories is the correct visualization for this page. The Mermaid integration works. Size at 1280px (978×403) is good. Promote it, do not replace it.
- **Section dividers** — the three `.section-divider` elements shipped in G9 are the right scaffolding. They need more spatial weight (QW-6) and the proposed IA reorder (R-1), not removal.
- **Delta arrows on category rows** — `↑ 13% / ↓ 24%` with color-coded `.text-up` / `.text-down` tone is correct, well-implemented, B15-friendly (shape + tone). Keep exactly.
- **Category drill-through** — clicking a category name navigates to `/transactions` pre-filtered to that category (L58 wiring). Correct affordance. Keep.
- **Spending narrative** — plain-English "You spent $4,068.00 across 14 categories. Your biggest expense was Housing at $2,175.00." — exactly the right tone. Keep.
- **No-spend days stat** — motivating positive signal in the headline grid. Keep.
- **Cash runway accent-for-runway coloring** — `< 3 months` → danger, `≥ 6 months` → positive. Logic correct; currently masked by the `.stat-value.neg` specificity bug (QW-3 fixes that).
- **Light-mode color pins** — G9/G23 fixes to `.card-title`, `.row-desc`, `.stat-value`, `.budget-amount` in light mode are working correctly in all 3 light-mode screenshots. No regression.
- **Period caption** — "Covering 2026-06-01 – 2026-07-01 · compared with 2026-05-01 – 2026-06-01" is the right context anchor. Promote its visual weight (QW-9), do not remove it.

---

## UI/UX defects (screenshot-confirmed + named files)

| ID | Defect | Evidence | Fix |
|---|---|---|---|
| D-1 | All cards 0px border-radius — hard square edges, spreadsheet feel | `card.radius: "0px"` metrics.json all 6 captures; visible in all 6 screenshots | QW-1 CSS-ONLY |
| D-2 | Card title font-weight 400 — no hierarchy vs body text | `cardTitle.weight: "400"` metrics.json all 6 captures | QW-2 CSS-ONLY |
| D-3 | Spending stat value colored accent-green — semantic inversion | `statValue.color: "rgb(46,139,87)"` dark+light; "SPENDING" green in reports_dark_1280.png, reports_light_1280.png; caused by L699 rule winning by source order | QW-3 CSS-ONLY |
| D-4 | `.budget-amount` dark = rgb(171,171,179) — row dollar amounts dim-gray, same as metadata | `amounts.budget: "rgb(171,171,179)"` dark metrics.json all 3 dark captures | QW-5 CSS-ONLY |
| D-5 | Share bar 4px height — unreadable as data | `shareBar.height: "4px"` all 6 captures | QW-4 CSS-ONLY |
| D-6 | Share bar 260px max-width — bars fill ~27% of card width at 1280px | `shareBar.maxWidth: "260px"` all 6 captures; visually confirmed in all screenshots | R-3 GO-STRUCTURAL |
| D-7 | Sankey at position 8 — most visual insight buried behind category list | reports_screen.go DOM order: category card L406, Sankey L439 | R-1 GO-STRUCTURAL |
| D-8 | Area chart no calendar labels — x-axis shows indices 0–5 | AreaChartProps no Labels field at L509/L522/L527; L61 defect | R-4 GO-STRUCTURAL |
| D-9 | "Heads up" card no urgency signal — identical visually to a trends chart | reports_dark_1280.png: Heads up card same white box as all other cards | R-5 GO-STRUCTURAL + CSS |
| D-10 | 6 full-size CSV export buttons across 5 card footers | `exportBtns: 6` all 6 captures; visually confirmed all screenshots | QW-10 interim CSS; R-7 full GO-STRUCTURAL |
| D-11 | Section dividers render when content group is empty — orphaned headers | reports_screen.go H3 dividers unconditional; confirmed in empty-data state | R-9 GO-STRUCTURAL |
| D-12 | Period caption 12px dim-gray — smallest text is most important context label | `caption.size: "12px"`, `caption.color: "rgb(60,60,67)"` light; all 6 captures | QW-9 CSS-ONLY |
| D-13 | Card margin-bottom (16px) tighter than card padding (20px) — inverted rhythm | `card.marginBottom: "16px"`, `card.padding: "20px"` all 6 captures | QW-7 CSS-ONLY |

---

## Probe hardening

Add to the probe suite once GI0 resolves:

```javascript
// D-1 regression guard: card border-radius must be set
const cardRadius = await page.evaluate(() =>
  getComputedStyle(document.querySelector('.card')).borderRadius);
expect(cardRadius).not.toBe('0px');

// D-2 regression guard: card-title must be semibold
const titleWeight = await page.evaluate(() =>
  getComputedStyle(document.querySelector('.card-title')).fontWeight);
expect(parseInt(titleWeight)).toBeGreaterThanOrEqual(500);

// D-3 regression guard: spending stat must not be accent-green
const spendColor = await page.evaluate(() => {
  const vals = [...document.querySelectorAll('.stat-grid .stat-value')];
  return vals[1] ? getComputedStyle(vals[1]).color : null;
});
expect(spendColor).not.toBe('rgb(46, 139, 87)');

// D-5 regression guard: share bars taller than 4px
const barH = await page.evaluate(() => {
  const b = document.querySelector('.share-bar');
  return b ? getComputedStyle(b).height : null;
});
if (barH) expect(parseFloat(barH)).toBeGreaterThan(4);

// Section dividers present (G9 regression guard)
const divs = await page.evaluate(() =>
  document.querySelectorAll('.section-divider').length);
expect(divs).toBeGreaterThanOrEqual(3);

// QW-5 regression guard: budget-amount has tabular-nums
const tabNum = await page.evaluate(() => {
  const el = document.querySelector('.budget-amount');
  return el ? getComputedStyle(el).fontVariantNumeric : null;
});
if (tabNum) expect(tabNum).toContain('tabular');
```

---

*Analysis date: 2026-06-23. Drive script: `node e2e/reports_beautify_analysis.mjs` (exit 0). Screenshots: `e2e/reports_{dark,light}_{768,1280,1440}.png` (6 files). Metrics: `e2e/reports_metrics.json`. Source read in full: `internal/screens/reports_screen.go` (778 lines), `web/index.html` (reports-relevant CSS). Extends G9 (2026-06-23) and C55 (2026-06-20).*