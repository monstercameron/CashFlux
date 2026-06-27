# CashFlux Enterprise UI Style Spec

Desktop-first responsive enterprise UI style spec for CashFlux.

This spec uses the current app only as evidence. It keeps what is already
working: the local-first finance positioning, typed theme engine, reusable Go
UI primitives, accessible control foundations, and the idea of configurable
work surfaces. It does not freeze the current page map, bento layout, sidebar
taxonomy, or card-stack paradigm. The goal is a durable visual and interaction
system that can support a better future information architecture.

## 0. Scope

This document defines product styling and interaction rules for a high-data-
density financial tool across desktop, tablet-class, and narrow responsive
viewports. It is intentionally not page-specific.

Use it for:

- Design tokens and semantic theming.
- Layout, spacing, density, typography, shape, and surface rules.
- Responsive behavior across viewport, container, pointer, and density changes.
- Reusable component anatomy.
- Interaction states, command behavior, and motion rules.
- High-density table and workstation behavior.
- Workflow templates for finance tasks.
- Visual and accessibility quality gates.

Do not use it to justify preserving:

- The current route list.
- The current navigation grouping.
- The current bento/dashboard composition.
- The current number of cards, widgets, or visible controls.
- Any hardcoded page-level exception that exists only because the current UI
  needed a patch.

## 1. Target Experience

CashFlux should feel like a serious financial workstation: calm, information
dense, fast to scan, trustworthy, keyboard-friendly, and useful without making
the user decode a wall of cards.

The north star is not "more polished cards." It is:

- A user can open any desktop page and understand the primary question in under
  3 seconds.
- The first viewport answers one job before exposing secondary tooling.
- Dense pages show more relevant financial data while exposing fewer resting
  controls.
- Colors have one meaning at a time: interaction, money direction, severity,
  selection, or passive chrome.
- Widgets and cards are recognizable by structure and rhythm, not just title
  text.
- Every chart, Smart card, KPI, and action exists because it supports a clear
  financial decision.

## 2. Reusable Foundations

The current system has ingredients worth keeping and formalizing:

- `internal/theme.Theme` already models theme tokens: `BgBase`, `BgCard`,
  `Border`, `Text`, `TextDim`, `Accent`, `Up`, `Down`, `Radius`, `FontUI`,
  `FontDisplay`, `Scale`, `Density`, and `IconStroke`.
- `internal/theme.derivedVars()` emits useful derived tokens:
  `--bg-elev`, `--text-faint`, `--accent-dim`, `--warn`, `--danger`,
  plus light-mode `--muted` and `--hover`.
- `docs/COMPONENTS.md` lists a reusable primitive layer: `Card`,
  `EntityListSection`, `DataTable`, `FilterToolbar`, `Widget`, `StatGrid`,
  `OverflowMenu`, `Segmented`, `Toggle`, `StepperPill`, `EmptyStateCTA`,
  `FlipPanel`, and other core blocks.
- The app has accessibility work already in place: visible focus rules,
  labelled icon buttons, table semantics, filter keyboard shortcut, and target
  sizing guardrails.

The gaps this spec fixes:

- A large stylesheet still contains hardcoded light-mode patches and component
  exceptions. Theme adherence needs semantic tokens, not more local overrides.
- Cards, Smart items, widgets, setup blocks, and lists often share the same
  silhouette and visual weight.
- The first viewport on many dense desktop surfaces is overloaded. The
  2026-06-26 probe found several surfaces above 70 visible controls and some
  above 100, which is a density-system failure rather than a reason to preserve
  the current layouts.
- Button hierarchy is functional but too noisy at rest.
- Headline figures and chart takeaways are not governed by a cross-page
  financial storytelling model.

## 3. Product Principles

### 3.1 Decision First

Each page starts with the decision it supports, not with everything the product
can do. Required priority order:

1. Primary question.
2. Headline answer or state.
3. Most important driver, exception, or next action.
4. Supporting evidence.
5. Tools and advanced controls.

Example workflow questions:

- Ledger cleanup: "What changed, and what needs cleanup?"
- Budget planning: "Am I on plan this period?"
- Obligations: "What must be paid next?"
- Analysis: "What moved my money, and why?"
- Allocation: "Where should the next dollar go?"

### 3.2 Dense, Not Crowded

High-density finance screens should earn density through structure:

- Stable columns.
- Clear numeric alignment.
- Fewer visible controls.
- Compact rows.
- Progressive details.
- Keyboard and bulk modes.

Do not create density by shrinking everything equally or stacking more cards.

### 3.3 Semantic Color Discipline

Color must not carry competing meanings. A green or red tone used for money
direction must not also mean severity or generic button priority in the same
surface. Accent is for interaction. Up/down are for financial direction.
Warning/critical are for attention and risk. Selection is its own state.

### 3.4 Tool, Not Brochure

CashFlux is a repeated-use pro tool. Avoid marketing-style composition inside
the app shell. Prefer calm, compact, task-oriented surfaces with obvious scan
paths, strong tables, and useful defaults.

### 3.5 Explainable Trust

Finance users need to know why the app recommends something and what action
will change. Smart insights, imports, restores, destructive actions, and
automations must show source, impact, reversibility, and audit evidence.

## 4. Design Token Model

### 4.1 Token Migration Rule

All new styling must use semantic CSS variables. Hardcoded colors are allowed
only in theme-generation code or documented one-off assets. Component CSS must
not hardcode light/dark values.

Required migration direction:

```css
:root {
  /* existing core tokens emitted by internal/theme */
  --bg-base: #0e1116;
  --bg-card: #161b22;
  --bg-elev: #20262e;
  --border: #2a3038;
  --text: #e6edf3;
  --text-dim: #9aa4af;
  --text-faint: #737d88;
  --accent: #7c83ff;
  --accent-dim: #343756;
  --up: #54b884;
  --down: #d8716f;
  --warn: #e0a93b;
  --danger: var(--down);
  --radius: 8px;
  --font-ui: Inter;
  --font-display: Fraunces;
  --ui-scale: 1;
  --icon-stroke: 1.6;
}
```

Add aliases for component semantics. These can be derived from the existing
theme fields first, then formalized in `Theme` once the migration path is
settled.

```css
:root {
  /* surfaces */
  --surface-page: var(--bg-base);
  --surface-card: var(--bg-card);
  --surface-raised: var(--bg-elev);
  --surface-sunken: color-mix(in srgb, var(--bg-base) 88%, #000 12%);
  --surface-selected: color-mix(in srgb, var(--accent) 16%, var(--bg-card));

  /* borders */
  --border-subtle: color-mix(in srgb, var(--border) 70%, transparent);
  --border-strong: color-mix(in srgb, var(--border) 72%, var(--text) 28%);
  --border-selected: var(--accent);

  /* text */
  --text-primary: var(--text);
  --text-secondary: var(--text-dim);
  --text-tertiary: var(--text-faint);
  --text-inverse: #ffffff;

  /* interaction */
  --interactive: var(--accent);
  --interactive-hover: color-mix(in srgb, var(--accent) 84%, var(--text) 16%);
  --interactive-muted: var(--accent-dim);
  --focus-ring: var(--accent);

  /* money semantics */
  --money-positive: var(--up);
  --money-negative: var(--down);
  --money-neutral: var(--text-primary);

  /* severity */
  --severity-info: var(--accent);
  --severity-nudge: var(--accent);
  --severity-warn: var(--warn);
  --severity-alert: var(--danger);

  /* charts */
  --chart-1: var(--accent);
  --chart-2: var(--up);
  --chart-3: var(--warn);
  --chart-4: #52a3ff;
  --chart-5: #b88cff;
  --chart-negative: var(--down);
}
```

### 4.2 Legacy Token Compatibility

CashFlux already uses several older CSS variables and selector-specific
patches. Do not delete them in one pass. First alias them, then migrate
component selectors, then remove hardcoded fallback rules after screenshot and
contrast verification.

| Current token/value pattern | Semantic destination | Migration rule |
|---|---|---|
| `--bg` | `--surface-page` | Keep `--bg` as an alias during migration; new CSS uses `--surface-page`. |
| `--bg-base` | `--surface-page` | Theme engine remains source of truth. Alias only in CSS. |
| `--bg-card` | `--surface-card` | Cards, widgets, stats, and panels use the semantic alias. |
| `--bg-elev` | `--surface-raised` | Popovers, hover fills, inputs, and raised controls use this alias. |
| `--border`, `--line` | `--border-subtle` / `--border-strong` | `--line` should become an alias of `--border-subtle`; strong boundaries opt in. |
| `--text` | `--text-primary` | No component should set raw text colors except through role aliases. |
| `--text-dim`, `--muted` | `--text-secondary` | Use for descriptions, labels, table headers, metadata. |
| `--text-faint` | `--text-tertiary` | Use only for nonessential metadata and disabled-adjacent text. |
| `--accent` | `--interactive` | Buttons, active nav, links, focus, and affordances. Not money-positive. |
| `--accent-dim` | `--interactive-muted` / `--surface-selected` | Use for selected/active backgrounds only through semantic aliases. |
| `--up` | `--money-positive` | Money direction only. |
| `--down` | `--money-negative` | Money direction only. Destructive fill needs its own contrast-safe token. |
| `--danger` | `--severity-alert` / `--action-danger` | Split destructive action from negative money during semantic-token migration. |
| `--warn` | `--severity-warn` | Warning, caution, attention states. |
| `--radius` | `--radius-lg` default | Keep as compatibility alias, but new CSS uses the radius scale. |

Compatibility CSS target:

```css
:root {
  --bg: var(--surface-page);
  --line: var(--border-subtle);
  --muted: var(--text-secondary);
  --action-danger: #c0392b;
}
```

Migration order:

1. Add semantic aliases without changing rendered UI.
2. Replace selector groups in this order: fields/buttons, cards/stats,
   ledgers/tables, widgets, Smart/alerts, settings/modals, light-mode patches.
3. Add automated contrast checks for the pairs in section 12.1.
4. Remove hardcoded light-mode overrides only after dark and light contact
   sheets preserve hierarchy.

### 4.3 Color Usage Rules

- `--interactive`: primary actions, active controls, links, focus accents.
- `--money-positive`: inflow, gain, under-budget success, progress on goal.
- `--money-negative`: outflow, loss, debt increase, overspend amount.
- `--severity-warn`: needs attention but not urgent.
- `--severity-alert`: urgent, blocking, destructive, high-risk.
- `--surface-selected`: selected rows/cards. Selection must not be expressed by
  accent text alone.
- `--text-secondary`: descriptions, labels, metadata.
- `--text-tertiary`: timestamps, hints, inactive supporting labels only.

Never use:

- `--accent` for positive money.
- `--down` for destructive button fill if white text fails contrast. Use a
  dedicated accessible destructive fill token.
- Hardcoded light-mode surfaces in component rules.
- Color alone to distinguish selected, warning, or error states.

### 4.4 Chart Palette

Charts need a stable semantic palette:

- Single primary trend: `--chart-1`.
- Income or positive contribution: `--money-positive`.
- Expense or negative contribution: `--money-negative`.
- Warning threshold: `--severity-warn`.
- Comparison series: `--chart-4` and `--chart-5`.

Every chart must include:

- A title phrased as the question or takeaway.
- Unit and period.
- Baseline or comparison when relevant.
- Accessible text summary.
- Nearby action or drilldown when the chart implies work.

## 5. Spacing, Layout, and Density

### 5.1 Spacing Scale

Use a 4px sub-unit and 8px primary rhythm.

```css
:root {
  --space-0: 0;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 32px;
  --space-7: 48px;
  --space-8: 64px;
}
```

Default component spacing:

| Element | Comfortable | Compact |
|---|---:|---:|
| Page top padding | 24px | 16px |
| Page side padding | 24px | 16px |
| Section gap | 24px | 16px |
| Card padding | 16px | 12px |
| Compact KPI padding | 12px | 8px |
| Toolbar gap | 8px | 6px |
| Row height, data table | 40px | 32px |
| Row height, entity list | 48px | 40px |
| Button height | 36px | 32px |
| Form field height | 40px | 36px |
| Icon button | 32px | 28px |

The existing 44px button/field sizing is appropriate for touch-heavy contexts,
but desktop pro-tool density should allow 32-40px controls while preserving
keyboard focus visibility and minimum pointer target guidance.

### 5.2 Desktop App Shell

Desktop shell layout:

- Left rail width expanded: 240-280px.
- Left rail collapsed: 56-64px.
- Topbar height: 48-56px.
- Main page max width:
  - Workstation pages: none; use available width.
  - Reading/help/settings pages: 960-1120px.
  - Overview surfaces: 1280-1440px.
  - Ledgers/tables: full width with inner constraints per columns.
- First viewport must not be consumed by global chrome unless it is blocking
  work, such as locked data, import conflict, or failed sync.

### 5.3 Page Grid

Use page templates rather than one universal card stack.

```css
.page {
  width: 100%;
  margin: 0 auto;
  padding: var(--space-5) var(--space-5) var(--space-7);
}

.page-grid {
  display: grid;
  gap: var(--space-4);
}

.page-grid--workstation {
  grid-template-columns: minmax(0, 1fr);
}

.page-grid--split {
  grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
}

.page-grid--inspector {
  grid-template-columns: minmax(560px, 1fr) 360px;
}
```

Rules:

- Ledgers and builder tools can use full width.
- Summary pages can use constrained width.
- Do not put cards inside cards.
- Do not make entire page sections look like floating cards.
- Side panels should be inspectors, not decorative columns.
- Horizontal overflow is a release blocker for desktop work pages.

### 5.4 Density Modes

CashFlux should support three desktop density modes:

| Mode | Purpose | Default surfaces |
|---|---|---|
| Comfortable | New users, guided setup, forms, lower-density planning | Overview, setup, settings |
| Compact | Repeated finance work | Ledgers, queues, rules, activity, management tables |
| Focus | One task at a time, low chrome | Import wizard, restore, destructive confirm, builder edit mode |

Density mode changes spacing and control exposure. It must not change semantic
meaning, hide required labels, or reduce focus visibility.

### 5.5 Responsive Design System

Responsive design must preserve the user's job, not the desktop layout. Each
surface adapts by re-prioritizing content, changing interaction patterns, and
collapsing secondary controls. It must not simply shrink columns, hide labels,
or squeeze desktop tables into narrow viewports.

#### 5.5.1 Breakpoint and Container Model

Use breakpoints for app shell changes and container queries for components.
Components should respond to their own available width whenever possible,
because the same component may appear in a wide content area, side inspector,
popover, or narrow stacked view.

| Range | Width | Primary intent |
|---|---:|---|
| Narrow | 320-599px | One task, one primary action, stacked summaries |
| Compact | 600-899px | Two-region layouts only when both remain readable |
| Medium | 900-1199px | Side panels become optional drawers or lower regions |
| Wide | 1200-1599px | Standard desktop work surfaces |
| Extended | 1600px+ | More data columns or inspector space, not more chrome |

CSS target:

```css
:root {
  --content-max-reading: 1040px;
  --content-max-overview: 1440px;
  --rail-expanded-w: 260px;
  --rail-collapsed-w: 60px;
  --inspector-w: 360px;
}

.surface {
  container-type: inline-size;
}

@container (max-width: 720px) {
  .surface-grid {
    grid-template-columns: 1fr;
  }
}
```

Rules:

- Use viewport breakpoints for shell/navigation only.
- Use container queries for cards, KPI grids, tables, charts, toolbars, and
  inspectors.
- Extended width should improve data comparison, not add unrelated modules.
- Narrow width should reduce simultaneous choices, not remove essential actions.

#### 5.5.2 Responsive Priority Contract

Every surface contract needs a responsive order:

```text
Surface/workflow:
Wide first viewport:
Medium first viewport:
Narrow first viewport:
Primary action at each size:
Controls hidden/collapsed at each size:
Table/list adaptation:
Inspector/detail adaptation:
Chart adaptation:
Failure or empty-state adaptation:
```

Acceptance:

- The same primary job is visible at every width.
- The first answer appears before controls at every width.
- A narrow view shows fewer decisions, not less context for the same decision.
- No critical action is available only through hover.

#### 5.5.3 Shell and Navigation Adaptation

Shell behavior:

| Range | Navigation model | Content rule |
|---|---|---|
| Wide/extended | Expanded or collapsible rail plus topbar | Content may use inspectors and full-width tables |
| Medium | Collapsed rail or compact nav plus topbar | Inspectors become drawers or secondary column only when readable |
| Compact | Compact nav, command/search entry, minimal topbar | Primary content dominates; global status collapses |
| Narrow | Single primary nav model | No duplicate rail + bottom nav; content gets full width |

Rules:

- There must never be two competing primary navigation systems at once.
- Global status, promos, sync, sample state, and household controls collapse
  into compact chips or menus unless blocking.
- The primary action remains reachable near the surface title or sticky action
  region.
- Command/search can replace visible navigation breadth, but not the only path
  to core workflows.

#### 5.5.4 Responsive Layout Patterns

Overview:

- Wide: hero state + exception rail + configurable modules.
- Medium: hero state + exceptions, modules in two columns.
- Narrow: hero state, top exception, primary action, then summaries.
- Never let a widget grid outrank the core financial state.

Ledger:

- Wide: table + optional inspector.
- Medium: table with fewer default columns; inspector as drawer.
- Compact: table/list hybrid with pinned identity, date, amount, status.
- Narrow: summary rows with expandable details; full table only in horizontal
  scroll if explicitly entered as "table mode".

Planning:

- Wide: status, variance, scenario controls, evidence.
- Medium: status and recommendation first; scenario controls collapse.
- Narrow: one recommendation or scenario at a time; comparisons become stacked
  cards with clear labels.

Builder/customization:

- Wide: canvas/list + inspector + preview.
- Medium: preview and inspector switch by tabs.
- Narrow: step-by-step editor; expert controls behind disclosure.

Import/export/restore:

- Wide: stepper + preview table + conflict panel.
- Medium: stepper + preview summary + expandable conflict details.
- Narrow: staged wizard; preview count and top issues before detailed rows.

#### 5.5.5 Responsive Tables and Lists

Dense financial tables need explicit responsive modes.

| Width/context | Pattern |
|---|---|
| Wide | Full table with sticky header, stable columns, optional inspector |
| Medium | Priority columns plus column chooser; hidden columns in details |
| Compact | Row summary + 2-3 key facts + expandable detail |
| Narrow | Card-like record rows or table mode toggle; no page-level overflow |

Column priority:

1. Identity: payee/source/object.
2. Financial value: amount/balance/impact.
3. Time: date/due/period.
4. Status: cleared/paid/warning/error.
5. Classification: category/account/member.
6. Secondary metadata: tags, notes, IDs, audit info.
7. Actions: hidden until selection, focus, details, or overflow.

Rules:

- Amounts and dates remain visible in all ledger modes unless the workflow is
  not financial.
- Hidden columns must be available in row details.
- Row actions never force horizontal overflow.
- Selection works in every layout mode.
- Bulk action preview summarizes affected records when rows are not all visible.
- Sort/filter state remains visible after responsive collapse.

#### 5.5.6 Responsive Toolbars and Controls

Toolbar priority order:

1. Primary action.
2. Search or command entry.
3. Active filter count/chips.
4. Most-used facet control.
5. Secondary actions.
6. Export/settings/advanced overflow.

Adaptation:

- Wide: search, facets, chips, and secondary actions can sit inline.
- Medium: facets move into popover; chips wrap or become count + clear.
- Compact: search and filter trigger remain; chips become count and summary.
- Narrow: one-line toolbar with search/filter/action; rest in overflow or
  bottom sheet/drawer.

Rules:

- Do not wrap toolbars into three-line control stacks above the user's data.
- Active constraints must stay visible even when chips collapse.
- Button labels can become icon+tooltip only when the icon is familiar and an
  accessible name remains.
- Destructive actions never collapse into anonymous icon-only controls.

#### 5.5.7 Responsive Forms and Wizards

Forms:

- Wide: two-column only when labels and validation remain readable.
- Medium: grouped one/two-column based on field relationship.
- Narrow: one column, visible labels, inline validation, persistent save/cancel
  area for long forms.

Wizards:

- Step indicators collapse from labelled stepper to current step + progress
  count.
- Preview/confirm screens keep affected counts and risk state above details.
- Advanced options collapse behind disclosure but remain searchable through
  command/help.

Rules:

- Fields do not shrink below readable input width.
- Labels stay visible; placeholders are not labels.
- Error text wraps under the field and never pushes the primary action out of
  reach without preserving a summary.
- Numeric inputs need enough width for realistic currency values.

#### 5.5.8 Responsive Charts and KPIs

Charts:

- Wide: full chart with legend, axes, comparison, and action.
- Medium: preserve trend/comparison, simplify legend and secondary series.
- Narrow: show takeaway, key value, and compact trend; detailed chart opens in
  drilldown.

Rules:

- Chart labels must not overlap at any width.
- Legends collapse into direct labels or a detail drawer.
- Tooltips must work for pointer and keyboard/touch.
- A chart that cannot remain legible at a width becomes a summary plus drilldown,
  not a tiny unreadable chart.

KPI grids:

- Wide: 3-5 KPIs when they form one story.
- Medium: 2-3 KPIs.
- Narrow: primary KPI first, secondary KPIs stacked or behind summary details.
- Use stable min/max sizes so values do not wrap unpredictably.

#### 5.5.9 Touch, Pointer, and Hybrid Devices

Responsive is not only viewport width. Adapt to pointer precision.

```css
@media (pointer: coarse) {
  :root {
    --control-h: 44px;
    --field-h: 44px;
    --icon-button-size: 44px;
  }
}
```

Rules:

- Coarse pointer controls target 44px minimum where practical.
- Hover-only reveals must become persistent, focusable, or available through
  details/overflow.
- Drag-and-drop has non-drag alternatives: move up/down, reorder menu, or
  keyboard shortcuts.
- Popovers near edges reposition without clipping.
- Sticky action regions must respect safe areas.

#### 5.5.10 Responsive Motion

Motion adapts to both width and pointer:

- Narrow and compact layouts use less spatial motion because movement consumes
  more of the viewport.
- Drawers slide from the side on medium/wide and from bottom only when the
  content is short enough to remain understandable.
- Tables switching to row-summary mode should not animate every row; use an
  immediate layout swap or brief opacity transition.
- Sticky bars and bottom sheets avoid bounce effects in finance workflows.
- Reduced-motion rules override responsive motion at every width.

#### 5.5.11 Responsive Quality Gates

Each responsive implementation must be verified at:

- 1440 x 1000: wide desktop.
- 1200 x 900: standard desktop.
- 900 x 800: medium/tablet-class.
- 600 x 800: compact.
- 390 x 844: narrow.
- 320 x 720: minimum narrow.

Acceptance:

- No page-level horizontal overflow.
- No duplicate primary navigation systems.
- Primary question, primary answer, and primary action are visible without
  hunting.
- Text does not overlap, clip, or require viewport-scaled font sizes.
- Toolbars do not wrap into dominant control walls.
- Tables/lists preserve identity, amount/value, time, and status.
- Hidden controls remain reachable by keyboard and non-hover input.
- Modals, drawers, menus, and inspectors fit within the viewport and restore
  focus.
- Charts remain legible or collapse into summary + drilldown.
- Touch/coarse pointer target sizes meet the responsive control rules.
- Dark and light themes preserve hierarchy at every verified width.

## 6. Typography and Numbers

### 6.1 Font Roles

- UI font: `Inter`.
- Display font: `Fraunces`, reserved for brand moments and the single
  headline figure on summary pages.
- Data and money figures use tabular numbers.

```css
.money,
.stat-value,
.table-number,
.kpi-value {
  font-variant-numeric: tabular-nums lining-nums;
}
```

### 6.2 Type Scale

Do not scale font size with viewport width. Use fixed steps adjusted by
`--ui-scale`.

```css
:root {
  --type-11: calc(11px * var(--ui-scale));
  --type-12: calc(12px * var(--ui-scale));
  --type-13: calc(13px * var(--ui-scale));
  --type-14: calc(14px * var(--ui-scale));
  --type-16: calc(16px * var(--ui-scale));
  --type-18: calc(18px * var(--ui-scale));
  --type-20: calc(20px * var(--ui-scale));
  --type-24: calc(24px * var(--ui-scale));
  --type-32: calc(32px * var(--ui-scale));
}
```

| Role | Size / line | Weight | Notes |
|---|---:|---:|---|
| Page title | 24 / 32 | 650 | One per page |
| Page subtitle | 14 / 20 | 400 | Plain explanation, not marketing |
| Section title | 16 / 24 | 650 | Card or section head |
| Body | 14 / 20 | 400 | Default |
| Table cell | 13 / 18 | 400 | Dense data |
| Table header | 12 / 16 | 650 | Uppercase optional |
| Caption/metadata | 12 / 16 | 400 | Secondary only |
| Tiny label | 11 / 14 | 600 | Use sparingly |
| Hero figure | 32 / 40 | 650 | One per summary page |
| KPI figure | 20 / 28 | 650 | Repeated summary figures |

Letter spacing must be `0` for normal text. Uppercase labels may use
`0.04em`, never more, and only for short labels.

### 6.3 Headline Financial Figures

Every summary page gets one primary headline figure or state.

Anatomy:

1. Label: "Net worth", "Left to allocate", "Due next", "Budget status".
2. Value: tabular, largest figure in first viewport.
3. Delta: period comparison with semantic color.
4. Driver: one sentence or chip naming the cause.
5. Action: one primary action only if action is obvious.

Rules:

- Use display font for the single hero figure only.
- Repeated KPI cards use UI font or restrained display font at smaller size.
- Never show 4-6 same-weight money figures at the top of a page.
- Currency precision:
  - Whole dollars for summaries.
  - Cents for transactions, bills, imports, and reconciliation.
  - Percentages show one decimal only when decisions require it.

## 7. Shape, Elevation, and Surface System

### 7.1 Radius Scale

The app currently mixes `--radius`, 12px cards, 10px stats, 8px Smart cards,
and hardcoded 6px controls. Enterprise desktop should use a tighter, consistent
radius scale.

```css
:root {
  --radius-0: 0;
  --radius-xs: 2px;
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 8px;
  --radius-pill: 999px;
}
```

Rules:

- Cards and widgets: `--radius-lg` max.
- Buttons, fields, chips: `--radius-md`.
- Table row highlights and focus surfaces: `--radius-sm`.
- Pills, avatars, progress bars: `--radius-pill`.
- Modal/dialog panels: `--radius-lg`.
- Avoid playful oversized rounding in dense financial tools.

### 7.2 Elevation

Use elevation to show hierarchy, not decoration.

| Surface | Fill | Border | Shadow |
|---|---|---|---|
| Page | `--surface-page` | none | none |
| Card/widget | `--surface-card` | `--border-subtle` | none or level 1 |
| Raised popover | `--surface-raised` | `--border-strong` | level 2 |
| Modal | `--surface-raised` | `--border-strong` | level 3 |
| Selected row | `--surface-selected` | left or inset accent | none |

```css
:root {
  --shadow-1: 0 1px 2px rgba(0,0,0,0.22);
  --shadow-2: 0 8px 24px rgba(0,0,0,0.28);
  --shadow-3: 0 20px 60px rgba(0,0,0,0.38);
}
```

Hover lift should be subtle and optional. In dense ledgers, hover should change
row background, not move layout.

## 8. Component Specifications

### 8.1 Buttons

Button variants:

| Variant | Use | Styling |
|---|---|---|
| Primary | One dominant action in a region | Filled `--interactive` |
| Secondary | Common non-primary action | Surface fill, border |
| Ghost | Navigation, low-priority toolbar, tabs | Transparent until hover |
| Icon | Common row/tool action | 32px square, tooltip/title |
| Destructive | Delete/wipe/overwrite | Accessible danger fill or ghost-danger |
| Link | Drill-in text action | Text color accent, no filled shape |

Required CSS target:

```css
.btn {
  min-height: var(--control-h, 36px);
  padding: 0 var(--space-3);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: var(--surface-raised);
  color: var(--text-primary);
  font-size: var(--type-13);
  font-weight: 550;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
}

.btn-primary {
  background: var(--interactive);
  border-color: var(--interactive);
  color: var(--text-inverse);
}

.btn-ghost {
  background: transparent;
  border-color: transparent;
  color: var(--text-secondary);
}

.btn-icon {
  width: var(--icon-button-size, 32px);
  min-width: var(--icon-button-size, 32px);
  height: var(--icon-button-size, 32px);
  padding: 0;
}
```

Button density budgets:

- Page header: 1 primary, up to 2 secondary, remainder in overflow.
- Card header: 0-1 visible action.
- Table row: 0 visible actions at rest when row is selectable; show primary
  row affordance on hover/focus or in details pane.
- Toolbar: group controls by job; hide advanced filters behind filter panel.
- First viewport: target <= 35 visible controls on overview surfaces and <= 55 on
  dense ledgers.

### 8.2 Cards and Widgets

Stop using one universal card shape for every information type. Use these
patterns:

| Pattern | Purpose | Shape |
|---|---|---|
| KPI card | One number and one supporting signal | compact, no heavy header |
| Alert card | Exception requiring attention | left severity rail, action row |
| Chart card | Trend or comparison | header takeaway, chart, footer evidence |
| Table card | List or ledger region | minimal shell, table owns density |
| Form card | Add/edit settings | labelled fields, grouped actions |
| Feed card | Activity/notifications | timeline rhythm, severity chips |
| Setup card | First-run or missing data | concise explanation, one CTA |
| Config widget | Builder/settings control | dense labels, inspector layout |

Card anatomy:

```text
section.card
  header.card-header
    h2.card-title
    optional .card-meta
    optional one action or overflow
  div.card-body
  optional footer.card-footer
```

Rules:

- Card title is not always required. KPI cards often need labels, not headings.
- A card cannot contain another card. Use `section`, `fieldset`, or list rows.
- Smart insight cards inline above content are capped at 3.
- Widget headers should not center every title by default; centered titles can
  work for configurable overview tiles, not dense pro-tool inspectors.

### 8.2.1 Widget Primitive Catalog

Review of the current dashboard widgets is evidence, not a fixed page contract.
The builder should not start from arbitrary freeform shapes. It should offer a
compact primitive catalog with stable anatomy, tokens, data-density limits,
states, and responsive collapse rules. These primitives can be combined, but
every composite widget should still be explainable as a small stack of these
base structures.

| Primitive | Structure | Styling Contract | Builder Use |
|---|---|---|---|
| Widget shell | Surface, optional header, body, optional footer/chrome | `--surface-card`, 1px subtle border, 6-8px radius, role-based padding, restrained shadow only on raised/dragging state | The outer container for every widget. Styles apply through theme, global widget defaults, primitive defaults, widget override, then state override. |
| Widget header/chrome | Grip, title, optional context label, action menu, resize handles | Header height 32-40px, title 13-14px semibold, controls icon-only where possible, chrome visually quiet outside edit mode | Used for draggable/resizable widgets and configurable tiles. Avoid making header chrome compete with the data. |
| Hero KPI block | Label/context, large value, delta or caption | Tabular numerals, display weight, high contrast value, muted caption, no heavy header | For one decisive figure in a medium or large tile. Use when the figure is the answer, not a supporting stat. |
| Standard KPI block | Small label, value, subline | Compact padding, value 20-28px, caption 12-13px, tone only on delta/status | For small dashboard cells, summary rows, and metric grids. |
| Metric pair/grid | Repeating label/value cells in 2-4 columns | Shared row height, aligned baselines, tabular numerals, faint dividers only when needed | For account totals, hero stat strips, side summaries, and comparison widgets. |
| Ledger mini table | Optional header, date/label/value columns, aligned numeric cell | Dense row height 32-40px, right-aligned money, clipped long labels, hover row state if clickable | For recent transactions, bills, imports, and compact financial ledgers. |
| Key-value row list | Label, optional meta, value/status/action | One row per entity, value column stable width, subdued secondary text | For accounts, upcoming bills, category summaries, and compact entity lists. |
| Checklist/action row | Checkbox or toggle, priority/status marker, label, optional overflow/action | Hit target >= 32px on desktop, clear focus ring, completed/disabled state distinct from secondary text | For tasks, reminders, approvals, and setup steps. |
| Attention item | Severity marker, short issue text, optional evidence, action | Severity rail/dot/icon, warning/error colors from semantic tokens, row can become a compact chip at small sizes | For exceptions, stale data, overspending, missing inputs, and risk callouts. |
| Chip group | Wrapping chips with label, value, status, or dismiss affordance | 24-30px height, 999px radius only for chips, modest border/fill contrast, no dense paragraphs inside chips | For filters, stale-account notices, insight tags, and compact multi-status summaries. |
| Progress row | Label/value header, progress track, optional caption/action | 8-10px track, rounded ends, semantic fill, show exact value in text, never rely on fill color alone | For budgets, goals, savings progress, completion, and limits. |
| Linear progress module | Primary figure, target/current context, progress bar | Combines a KPI block with a progress row; target caption stays adjacent to the bar | For single-goal or single-budget widgets where the ratio is the main story. |
| Segmented composition bar | Horizontal proportional segments plus legend | Segments must total 100%, minimum visible segment treatment, legend uses labels and values, palette from chart tokens | For spending mix, portfolio/category composition, and budget allocation. |
| Mini bar chart | Small repeated bars, label row, optional right-side net figure | Fixed chart height, baseline visible, positive/negative tokens, direct labels when axis is omitted | For cash-flow periods and compact comparisons. |
| Trend chart frame | Headline, period controls or context, line/area/bar chart, interpretation | Chart height tied to tile size, axes/units when needed, muted grid, one accent series by default | For net worth, income, spending, balance, and forecast trends. |
| Donut/composition chart frame | Donut or ring, direct total, legend/table fallback | Limit to 4 categories unless paired with sorted table, no unlabeled color-only slices | For simple composition summaries when segment order is less important than share. |
| Badge/status pill | Short label with semantic state | Pill or rectangular badge, 11-12px text, semantic tokens, optional icon | For state labels, risk levels, sync freshness, account type, or automation state. |
| Text/note block | Title optional, paragraph/list/caption content | Body text 13-14px, max line length, enough contrast, no decorative card nesting | For explanations, smart summaries, notes, and generated insight text. |
| Image/artifact block | Media frame, optional caption/source/action | Fixed aspect ratio, object-fit rules, caption below, explicit empty/error state | For receipts, attachments, imports, document previews, and widget artwork. |
| Data table block | Header, rows, optional sort/filter/footer | Table owns density; sticky or repeated headers for scrollable bodies; numeric columns aligned | For larger table widgets that need comparison and scanning, not just a short list. |
| Recommendation item | Insight title, evidence, confidence/severity, action | Similar to attention item but lower urgency; evidence is always visible or one click away | For smart digest, recommendations, anomaly summaries, and next-best actions. |
| Empty/setup CTA | Specific missing state, one primary action, optional secondary path | Quiet icon or marker, concise copy, action alignment consistent with widget body | For widgets with no data, missing configuration, gated features, and first-run states. |
| Action button row | One primary command, optional secondary/ghost commands | Buttons follow global hierarchy and density tokens; destructive actions need confirm/preview | For widget-local actions like import, remind, dismiss, configure, or drill down. |
| Toggle/control primitive | Label, control, optional helper/status | Control state must be visible without color alone; keyboard and pointer behavior match forms | For configurable widgets, automations, and local display switches. |
| Composite stack | Ordered stack of other primitives in row or column layout | Uses internal gap tokens, not nested card surfaces; each child declares min size and collapse behavior | For builder-created widgets that combine KPI, chart, list, progress, and actions. |

Primitive size guidance:

| Tile Size | Prefer | Avoid |
|---|---|---|
| 1x1 | Standard KPI, single alert, badge, short progress module, one action | Tables, multi-series charts, long text, more than 3 rows |
| 2x1 | KPI plus sparkline/progress, short row list, segmented bar, mini bars | Dense forms, long ledgers, multi-step controls |
| 1x2 | Vertical checklist, goal stack, trend preview, key-value list | Wide tables, side-by-side comparisons |
| 2x2+ | Full trend chart, ledger table, composition plus legend, smart summary stack | Single lonely number unless it is intentionally hero-level |

Primitive styling tokens:

```css
:root {
  --widget-pad-x: var(--space-3);
  --widget-pad-y: var(--space-3);
  --widget-gap: var(--space-2);
  --widget-header-h: 36px;
  --widget-row-h-compact: 32px;
  --widget-row-h-default: 40px;
  --widget-chart-h-sm: 112px;
  --widget-chart-h-md: 180px;
  --widget-accent: var(--accent);
  --widget-danger: var(--danger);
  --widget-warning: var(--warning);
  --widget-success: var(--success);
}
```

Builder requirements:

- Each primitive declares anatomy, minimum tile size, preferred tile sizes,
  formatter rules, empty/loading/error states, focus behavior, and responsive
  collapse behavior.
- Style controls map to semantic roles, not arbitrary descendants: surface,
  border, text, muted text, accent, semantic state, chart palette, radius,
  shadow, font family, and font weight.
- Per-widget visual overrides cannot break contrast, focus visibility, row
  alignment, numeric formatting, or semantic state colors.
- The builder should expose primitives as choices such as `KPI`, `Progress`,
  `Mini table`, `Row list`, `Trend chart`, `Composition`, `Checklist`,
  `Attention`, `Action row`, `Text`, `Image`, and `Composite`.
- Composite widgets should use a visible layer tree so a user understands which
  primitive is being styled.
- Widget export/import should preserve primitive type and role tokens, not only
  inline CSS.

### 8.3 KPI and Stat Cards

KPI anatomy:

```text
Label
Value
Delta + driver
Optional sparkline
```

CSS target:

```css
.kpi {
  padding: var(--space-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--surface-card);
  min-width: 0;
}

.kpi-label {
  font-size: var(--type-12);
  line-height: 16px;
  color: var(--text-secondary);
}

.kpi-value {
  margin-top: var(--space-1);
  font-size: var(--type-20);
  line-height: 28px;
  font-weight: 650;
}
```

Rules:

- KPI cards should not all use uppercase labels.
- Only semantic deltas get money-positive/money-negative colors.
- Avoid wrapping KPI values. If a value is too long, use compact currency
  notation or smaller fixed size for that card.

### 8.4 Data Tables and Ledgers

Ledgers are core enterprise surfaces. They need more discipline than card lists.

Required anatomy:

```text
Page priority header
Filter toolbar
Active filter chips
Bulk toolbar, hidden until selection
Table with sticky header
Optional right inspector/details pane
Pagination or virtualization footer
```

Table rules:

- Row height: 32px compact, 40px comfortable.
- Text cells left aligned.
- Numeric and money cells right aligned with tabular numbers.
- Date column fixed width.
- Identity column can truncate but must expose full value on hover/focus/title.
- Amount and balance columns must not wrap.
- Header sticks inside the scroll container.
- Secondary row actions live in overflow or details panel.
- Bulk toolbar appears only after selection.
- Filter panel must not occlude the table without preserving context.
- Horizontal overflow is acceptable only inside a deliberate table scroll area,
  never at page level.

Table CSS target:

```css
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--type-13);
}

.data-table th {
  height: 32px;
  padding: 0 var(--space-2);
  color: var(--text-secondary);
  font-size: var(--type-12);
  font-weight: 650;
  text-align: left;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-page);
}

.data-table td {
  height: 36px;
  padding: 0 var(--space-2);
  border-bottom: 1px solid var(--border-subtle);
}

.data-table .num,
.data-table .money {
  text-align: right;
}
```

### 8.4.1 Enterprise Workstation Behavior

Dense finance work needs pro-tool behavior beyond visible buttons.

Command palette:

- Global shortcut opens a command palette from any desktop surface.
- Commands include: create record, import data, add account/source, define plan,
  open filters, clear filters, export current view, open settings, switch
  workspace, and run workflow.
- Commands show keyboard shortcut, workspace scope, and whether they affect current
  selection.
- Destructive commands never execute directly from the palette; they open the
  preview/confirm flow.

Saved views:

- Ledgers and logs support saved views: filter set, sort, visible columns,
  density, and grouping.
- Saved views appear as compact tabs/chips above the table, not as full cards.
- Default views should be job-based: "Needs review", "Uncategorized", "This
  month", "Large transactions", "Upcoming bills", "Dismissed alerts".

Column and density personalization:

- Tables expose column visibility and order in a view/settings menu.
- Default columns are conservative and task-specific.
- Amount/date/account/category columns keep stable widths.
- Hidden columns are available in row details.
- Compact density may reduce row height and padding but not remove the focus
  ring, accessible name, or selected state.

Inspector pane:

- Row selection opens a right inspector on workstation pages when space allows.
- Inspector shows details, edit fields, related records, audit history, and
  secondary actions.
- Inline edit remains available for quick corrections, but complex edits move
  to the inspector.
- Closing the inspector restores focus to the selected row.

Bulk state model:

- Resting: no bulk toolbar, rows optimized for scan.
- Selecting: bulk toolbar appears, table gains selection count, row actions
  remain secondary.
- Previewing: affected records are summarized before mutation.
- Committing: controls disable, progress/status is visible.
- Completed: result toast plus undo/rollback where feasible.
- Failed: row-level errors are recoverable and exportable.

Keyboard table model:

- Arrow keys move row focus.
- Enter opens row details.
- Space toggles selection.
- Shift+arrow extends selection.
- `f` opens filters.
- `/` focuses search when search exists.
- Escape closes inspector, popover, or filter panel in that order.
- `?` or the help surface exposes the shortcut list.

### 8.5 Filter Toolbars

Filter toolbar order:

1. Search.
2. Primary facet trigger.
3. Date/period control if relevant.
4. Active filter chips.
5. Export or advanced actions.

Rules:

- Show clear filters only when filters exist.
- Chips use consistent removable affordance.
- Saved filter views are a follow-on enterprise feature for high-use tables.
- Keyboard shortcut hints belong in tooltip/title and `/help`, not as visible
  instructional text in dense toolbars.

### 8.6 Smart Insights and Alerts

Smart and Smart+ must become a mechanical-advantage layer, not repeated content
furniture. A Smart surface earns its place only when it shortens a workflow,
prevents a financial mistake, explains a decision, or safely prepares an action
the user can review and commit.

Definitions:

- Smart: deterministic, on-device rules, scoring, grouping, anomaly detection,
  pacing, projections, and suggestions.
- Smart+: provider-backed AI controls that parse natural language, summarize,
  classify, extract, or explain. Smart+ must be click-before-run unless the user
  explicitly schedules it, because it can send data to a provider and cost money.

Required integration layers:

| Layer | Purpose | Design Contract |
|---|---|---|
| Page strip | Top contextual findings for the current job | Max 3 inline items; shows only high-value items with reason, evidence, action, and dismiss/snooze. |
| Row/entity marker | Flag a specific transaction, bill, account, goal, subscription, or task | Quiet badge at rest; opens exact evidence and one local action; never sends the user to a generic hub as the only path. |
| Section action | Run analysis for the current table, chart, form, or selection | Label says the job, not just "Smart"; opens preview/result in context. |
| Field assist | Suggest a value while a form is being completed | One-click apply, visible source, undoable field change, no hidden submit. |
| Empty-state assist | Convert missing data into setup guidance | Explains the requirement and offers the next smallest setup step. |
| Workflow preview | Prepare a proposed mutation before commit | Shows affected records, confidence, cost/privacy, and reversible/irreversible effects. |
| Digest/notification | Proactive cross-page summary | Deduped, scheduled, severity-ranked, links to exact entity/workflow, never a vague alert. |
| Smart+ run control | AI-backed parse/summarize/extract/explain | Shows provider/cost, input scope, model result, confidence/caveat, accept/edit/discard. |

Insight anatomy:

```text
Source / feature
Severity
Short finding
Why shown / evidence
Primary next action
Preview impact if action mutates data
Dismiss / snooze / mute feature / details
```

Rules:

- Inline Smart cards: max 3 per page.
- Sort by severity and actionability.
- Group repeated rule hits.
- Explain "why shown" in details or hover/disclosure.
- Severity rail uses `--severity-*`; money inside uses money tokens.
- A dedicated recommendations inbox can show the full catalog; other surfaces
  show only contextual top items.
- Smart+ prompt boxes are fallback controls, not the target experience. Prefer
  page-native controls such as "Categorize selected", "Draft scenario",
  "Explain this forecast", "Map this import", or "Find duplicate bills".
- Every actionable Smart item needs a preview state before data mutation and a
  confirmation state after mutation.
- Smart cards that only navigate to another page should be treated as weak
  integration unless the target page opens to the exact affected entity/action.
- The user must be able to mute a feature, snooze one finding, dismiss one
  finding, and inspect why it appeared.
- Smart+ controls must disclose data scope and cost before running, cache
  results, and avoid automatic repeated paid calls.
- The same insight cannot appear as a strip, widget, digest, row badge, and
  notification unless those surfaces have different jobs and share dismissal
  state.

Smart integration scoring rubric:

| Score | Meaning |
|---:|---|
| 1 | No useful page-native Smart surface or the integration points at the wrong page/job. |
| 2 | Mentions Smart, AI, or recommendations, but provides no contextual workflow value. |
| 3 | Has adjacent help or a hub link; the user still does the work manually. |
| 4 | Shows contextual findings but mostly as cards, strips, or navigation. |
| 5 | Provides in-context findings plus one helper/control, but weak preview/commit mechanics. |
| 6 | Multiple page-native affordances with entity context and some direct action. |
| 7 | Strong workflow acceleration with preview, exact entity targeting, and undo/confirmation. |
| 8 | Cross-entity reasoning, bulk/selection support, explainability, and scheduled follow-up. |
| 9 | Enterprise-grade assistive workflows with audit trail, policy controls, and measurable saved work. |
| 10 | Trusted autopilot: user-governed, auditable, adaptive, and consistently superior to manual work. |

Smart quality gates:

- Every page declares whether Smart is core, supportive, or intentionally absent.
- Every Smart-capable page has a page-specific job statement, not a generic
  "Smart" header.
- Every Smart+ feature has a local fallback or a clear "requires provider"
  explanation with no dead controls.
- Every recommendation stores enough metadata to explain source, confidence,
  affected entities, generated time, and dismissal/mute state.
- Smart integration is measured per route: visible Smart surfaces, enabled
  engines, implemented Smart+ controls, direct actions, preview states, undo
  support, and exact entity targeting.

### 8.7 Charts

Charts must answer a question. Decorative mini-charts are allowed only inside
KPI cards when they support the primary number.

Chart card anatomy:

```text
Question/takeaway title
Period and comparison
Visualization
One-line interpretation
Drilldown or action
```

Rules:

- Use line/area charts for trend over time.
- Use bars for comparison across categories or periods.
- Use stacked bars only when composition matters and labels remain legible.
- Avoid pie/donut for more than 4 categories.
- Every axis needs units unless obvious from labels.
- Show baseline for debt, budget, and target comparisons.
- Do not rely on red/green alone.

### 8.8 Forms and Editors

Form rules:

- Labels above fields for dense but clear scanning.
- Help text below fields only where it prevents error.
- Errors appear next to the field and in a form-level summary for multi-step
  workflows.
- Save/cancel location is consistent.
- `Save and add another` is a secondary action, not equal visual weight to
  primary Save.
- Advanced fields live behind disclosure when they are not required.

Field CSS target:

```css
.field {
  min-height: var(--field-h, 36px);
  padding: 0 var(--space-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-raised);
  color: var(--text-primary);
  font-size: var(--type-13);
}
```

### 8.9 Empty, Gated, and Loading States

Empty states should be useful, not decorative.

Anatomy:

```text
State title
Specific reason
One primary CTA
Optional secondary import/sample path
```

Rules:

- Empty accounts should lead to add/import account, not broad sample data.
- Empty transactions should lead to import, quick add, or connect guidance.
- AI-key-gated features must expose non-AI alternatives first when possible.
- Loading states use skeleton structure where layout matters.

### 8.10 Destructive and High-Trust Actions

High-trust flows include import, restore, wipe, delete, cloud sync, automation,
and bulk edits.

Required flow:

1. Initiate.
2. Preview affected data.
3. Validate conflicts/errors.
4. Confirm with clear irreversible/reversible language.
5. Commit.
6. Show result and audit evidence.
7. Offer undo/rollback when feasible.

Destructive buttons must not share the same shape/color as primary positive
actions. Prefer ghost-danger until final confirmation.

## 9. Screen Archetypes

### 9.1 Overview Surface

Job: "Where do I stand, and what needs attention?"

First viewport order:

1. Net worth or current cash position.
2. This-period income/spend/left-to-allocate.
3. Top 1-3 exceptions.
4. Quick add/import affordance.
5. Widget grid.

Rules:

- Overview controls target: <= 35 in first viewport.
- Configurable widgets must not outrank urgent exceptions.
- Nonblocking global status should collapse after first acknowledgement.
- Modules use varied shapes: KPI strip, exception feed, trend panel, task list.

### 9.2 Ledger and Record Surface

Job: "Find, clean up, and act on records."

Template:

```text
Header with count and primary state
Search/filter toolbar
Table/list
Details inspector or inline expansion
Bulk toolbar on selection
```

Rules:

- Resting rows expose data first, controls second.
- Selection mode reveals bulk tools.
- Row details show secondary actions.
- Keyboard navigation is mandatory for enterprise quality.

### 9.3 Planning and Decision Surface

Job: "Know the plan, see variance, decide next move."

Template:

```text
Hero financial state
Variance/driver strip
Decision cards
Evidence table/chart
Scenario or action pane
```

Rules:

- One hero number/state per page.
- Separate plan, actual, variance, and recommendation.
- Charts explain the driver, not just movement.
- Smart insight count is capped and contextual.

### 9.4 Import, Export, Restore, and Evidence Surface

Job: "Bring data in or out safely."

Template:

```text
Step indicator
Source/target selection
Preview
Validation/conflict resolution
Commit
Result/audit trail
```

Rules:

- Never bury the safe/manual path below AI-key-gated paths.
- Preview affected account, row count, skipped rows, duplicates, and conflicts.
- Export states say exactly what will be included.

### 9.5 Builder, Automation, and Customization Surface

Job: "Configure without losing control."

Template:

```text
Object list or canvas
Inspector panel
Preview
Validation/status
Save/publish controls
```

Rules:

- Use inspector layout, not equal-weight card stacks.
- Separate edit mode from browse mode.
- Avoid page-level horizontal overflow.
- Keep undo/reset visible but not primary unless changes are unsaved.

### 9.6 Settings, Help, and Admin Surface

Job: "Understand and configure the system."

Template:

```text
Section navigation
Focused setting groups
Inline status
Danger zone separated
```

Rules:

- Settings sections use fieldsets and rows, not generic cards for every item.
- Destructive/admin actions are visually separated.
- Help content links to the exact workflow where help applies.

## 10. Information Priority Contracts

Each new or redesigned surface must define this contract before implementation:

```text
Surface/workflow:
Primary user question:
Primary first-viewport answer:
Primary action:
Secondary evidence:
Advanced controls:
Hidden/collapsed by default:
Success state:
Empty state:
Risk/destructive states:
```

Acceptance:

- The rendered first viewport matches the contract.
- The primary answer appears before secondary controls.
- There is at most one filled primary button per region.
- Global chrome does not outrank the surface job.

## 11. Control Density Budgets

Budgets are measured in first viewport visible controls. Desktop budgets are
below; compact and narrow layouts should usually expose fewer visible controls,
with secondary capability moved into drawers, overflow, details, saved views,
or command/search.

| Page type | Target | Hard ceiling |
|---|---:|---:|
| Overview | 35 | 45 |
| Ledger/table | 55 | 65 |
| Planning/decision | 45 | 55 |
| Builder/config tool | 55 | 70 only in explicit edit mode |
| Settings/help | 40 | 50 |
| Admin/danger flows | 30 | 40 |

Visibility rules:

- Row action buttons are not counted if hidden until hover/focus or overflow.
- Bulk actions are not counted until selection mode.
- Collapsed filter fields are not counted.
- Keyboard-only shortcuts do not add visible control load.

Any surface exceeding the hard ceiling should be redesigned before visual
polish. Do not solve density failures by shrinking text, reducing contrast, or
adding horizontal overflow.

### 11.1 Measurement Protocol

Density budgets are only meaningful when measured the same way every time.

Audit setup:

- Viewports: 1440 x 1000, 1200 x 900, 900 x 800, 600 x 800, 390 x 844, and
  320 x 720.
- Browser zoom: 100%.
- Theme: dark and light must both pass; dark is measured first because it is the
  product baseline.
- Data state: seeded sample data, Smart free rules enabled, no modal open, no
  row hovered, no row selected, no settings panel open, no advanced/edit mode
  unless the surface is specifically an editor.
- Surface readiness: wait until the title and primary heading match the
  intended workflow and no
  visible loading text remains.

Control counting:

- Count visible controls inside the content viewport separately from global
  shell controls.
- Content controls include visible `button`, `a[href]`, `input`, `select`,
  `textarea`, `[role="button"]`, `[role="switch"]`, `[role="checkbox"]`,
  `[role="radio"]`, `[role="tab"]`, `[contenteditable="true"]`, and visible
  elements with positive `tabindex`.
- Exclude disabled controls only when they are visually disabled and not
  actionable.
- Exclude controls hidden by `display:none`, `visibility:hidden`, zero opacity,
  offscreen positioning, collapsed disclosure, overflow menu, or hover/focus
  state not active during the audit.
- Count shell controls separately: rail navigation, topbar household controls,
  sample mode, notification, help, add menu, and period controls.

Expected report shape:

```json
{
  "surface": "ledger",
  "path": "<surface-path>",
  "viewport": "1440x1000",
  "pointer": "fine",
  "theme": "dark",
  "h1": "Ledger",
  "surfaceType": "ledger/table",
  "contentControls": 54,
  "shellControls": 28,
  "visibleCards": 3,
  "visibleTables": 1,
  "visibleCharts": 0,
  "horizontalOverflow": false,
  "loadingText": false,
  "primaryButtonCount": 1,
  "destructiveButtonCount": 0,
  "duplicatePrimaryNavigation": false,
  "touchTargetsBelowMinimum": 0,
  "chartLabelOverlap": false,
  "firstViewportChars": 1850
}
```

Required future audit script shape:

```powershell
Set-Location C:\Users\mreca\Desktop\CashFlux
node e2e/ux-audit-2026-06-26/desktop-audit.mjs --url http://127.0.0.1:8080/ --viewport 1440x1000 --themes dark,light
```

The future audit script should also support a responsive matrix:

```powershell
node e2e/ux-audit-2026-06-26/responsive-audit.mjs --url http://127.0.0.1:8080/ --viewports 1440x1000,1200x900,900x800,600x800,390x844,320x720 --themes dark,light
```

If using GWC screenshots as the visual evidence path:

```powershell
Set-Location C:\Users\mreca\Desktop\GoWebComponents
go run -tags playwrightgo ./tools/gwc screenshot -url "http://127.0.0.1:8080/<surface-path>" -out "C:\Users\mreca\Desktop\CashFlux\e2e\ux-audit-2026-06-26\<surface-name>-dark.png"
```

## 12. Accessibility Requirements

Target: WCAG 2.2 AA minimum, with AAA where feasible for static finance text.

Requirements:

- Text contrast: normal text >= 4.5:1, large text >= 3:1.
- Non-text UI contrast: focus indicators, selected states, chart lines, and
  control boundaries >= 3:1 against adjacent colors.
- Focus visible: at least 2px outline or equivalent, not obscured.
- Keyboard:
  - Navigate tables.
  - Open/close filter panels.
  - Move row focus.
  - Select rows.
  - Open row actions.
  - Escape modals/panels.
  - Restore focus after close.
- Screen-reader order must match visual priority.
- Tables use semantic headers and sort state.
- Form errors are announced and linked to fields.
- Reduced motion disables decorative motion; functional state changes remain
  perceivable.
- Color is never the only signal for money direction, severity, selected state,
  or validation.

### 12.1 Required Contrast Matrix

Every built-in and custom theme must pass these pairs. Tests should run against
computed colors, not assumed token values, because `color-mix()` and light-mode
aliases can change the final result.

| Pair | Minimum |
|---|---:|
| `--text-primary` on `--surface-page` | 4.5:1 |
| `--text-primary` on `--surface-card` | 4.5:1 |
| `--text-primary` on `--surface-raised` | 4.5:1 |
| `--text-secondary` on `--surface-card` | 4.5:1 |
| `--text-tertiary` on `--surface-card` | 3:1 for metadata, 4.5:1 if actionable |
| Primary button label on primary button fill | 4.5:1 |
| Destructive button label on destructive fill | 4.5:1 |
| Ghost-danger text on page/card surface | 4.5:1 |
| Selected row text on selected row surface | 4.5:1 |
| Selected row boundary against row surface | 3:1 |
| Focus ring against adjacent surface | 3:1 |
| Input border against input/page surface | 3:1 when field is active or invalid |
| Warning text/icon on warning surface | 4.5:1 |
| Alert text/icon on alert surface | 4.5:1 |
| Chart line against chart background | 3:1 |
| Adjacent chart series against each other | 3:1 where comparison is required |

Theme parity means the hierarchy is equivalent in dark and light themes. Passing
contrast is necessary but not sufficient: active, selected, warning, and
destructive states must remain visually distinct in both themes.

## 13. Interaction and Motion Layers

Interaction and motion are part of the product system, not decorative polish.
They must make a dense finance tool feel fast, understandable, recoverable, and
trustworthy. They should answer four questions:

- What can I do?
- What has focus or selection?
- What just changed?
- Is the app working, blocked, or done?

### 13.1 Interaction Principles

Use these rules across all desktop surfaces:

- Every interactive element has a visible resting affordance or appears in a
  predictable reveal pattern.
- Hover may preview affordance; focus must reveal the same affordance for
  keyboard users.
- Selection, focus, hover, active/pressed, dirty, loading, success, warning,
  and error are separate states. Do not reuse one style for all of them.
- Primary actions execute the main workflow. Secondary actions support it.
  Tertiary actions move into overflow, command palette, or inspector.
- Dense record surfaces optimize resting state for reading. Action controls
  appear through selection, hover/focus, row details, or command palette.
- Any action that changes data must produce visible feedback within 100ms:
  optimistic state, spinner/progress, toast, inline status, or disabled pending
  state.
- Any action that can fail must have a stable failure state and recovery path.
- Any action that is destructive, bulk, irreversible, or high-trust opens a
  preview/confirm flow instead of executing from an incidental click.

### 13.2 State Model

All interactive components must support this state vocabulary where relevant.

| State | Meaning | Required visual treatment |
|---|---|---|
| Rest | Available but not engaged | Clear affordance, no exaggerated weight |
| Hover | Pointer is exploring | Subtle surface/border/text change; no layout shift |
| Focus visible | Keyboard focus | 2px+ ring or equivalent, 3:1 contrast against adjacent surface |
| Pressed | Pointer/key activation in progress | Immediate pressed feedback; no persistent ambiguity |
| Selected | User has chosen an item | Dedicated selected surface plus non-color signal where needed |
| Expanded | More detail is visible | Chevron/state marker, preserved context |
| Editing | User is changing data | Stronger field boundaries, dirty state, save/cancel available |
| Dirty | Unsaved changes exist | Persistent unsaved marker and enabled save/revert controls |
| Validating | Input or action is being checked | Inline spinner/status, controls constrained but not frozen silently |
| Loading | Data is being fetched or computed | Skeleton/progress matched to final layout |
| Success | Action completed | Brief confirmation near origin or toast; no permanent celebration |
| Warning | User can continue but should notice risk | Warning token, explanation, optional detail |
| Error | User is blocked or action failed | Error token, specific message, recovery action |
| Disabled | Not currently available | Reduced contrast but still readable if explanatory text is present |

CSS state hooks should use attributes before ad hoc classes:

```css
[data-state="selected"] { background: var(--surface-selected); }
[data-state="dirty"] { border-color: var(--severity-warn); }
[data-state="error"] { border-color: var(--severity-alert); }
[aria-expanded="true"] .chevron { transform: rotate(90deg); }
[aria-busy="true"] { cursor: progress; }
```

### 13.3 Interaction Tokens

Define interaction and motion tokens beside the visual tokens.

```css
:root {
  /* control sizing */
  --control-h: 36px;
  --control-h-compact: 32px;
  --field-h: 36px;
  --icon-button-size: 32px;

  /* interaction feedback */
  --focus-ring-width: 2px;
  --focus-ring-offset: 2px;
  --hover-tint: color-mix(in srgb, var(--interactive) 8%, transparent);
  --pressed-scale: 0.985;
  --disabled-opacity: 0.48;

  /* motion durations */
  --motion-instant: 0ms;
  --motion-fast: 100ms;
  --motion-base: 160ms;
  --motion-medium: 220ms;
  --motion-slow: 320ms;

  /* motion easing */
  --ease-standard: cubic-bezier(0.2, 0, 0, 1);
  --ease-enter: cubic-bezier(0, 0, 0, 1);
  --ease-exit: cubic-bezier(0.4, 0, 1, 1);
  --ease-emphasized: cubic-bezier(0.2, 0, 0, 1);
}
```

Compact density can reduce spacing and row height. It must not reduce
`--focus-ring-width`, remove labels, or hide required status feedback.

### 13.4 Input Modality Rules

Pointer:

- Hover reveals secondary row actions only when the same actions are reachable
  by focus, row details, command palette, or context menu.
- Pressed feedback is immediate and small: scale, inset shadow, or tint.
- Drag handles must be visible before dragging and have keyboard alternatives.

Keyboard:

- Tab moves through major controls in visual order.
- Arrow keys move inside composite controls: tables, menus, segmented controls,
  swatches, stepper controls, trees, and grids.
- Enter activates primary row/detail action.
- Space toggles selection or switches.
- Escape closes the topmost transient layer: menu, tooltip, popover, inspector,
  modal.
- Focus returns to the initiating control after close.

Command palette:

- Commands expose hidden capability without adding visible density.
- Commands must be searchable by user intent, not only exact feature names.
- Commands show scope: current surface, current selection, global, or unsafe.
- High-trust commands open preview/confirm flows, never execute directly.

Context menus:

- Context menus are secondary productivity affordances, not the only path.
- Menu items use verbs, not icons alone.
- Destructive menu items are separated and use danger styling only in the menu
  row, not the whole menu.

### 13.5 Layering Model

Use consistent UI layers so interaction never feels random.

| Layer | Examples | Behavior |
|---|---|---|
| Base | Page content, tables, cards | Stable; no entry animation after ready |
| Inline reveal | Row details, disclosures, validation | Pushes content only when user requested it |
| Floating | Tooltip, menu, popover, date picker | Dismiss on Escape/outside click; restore focus |
| Inspector | Right/side detail panel | Persistent until closed; selection-aware |
| Modal | Confirm, import preview, blocking setup | Focus trapped; background inert |
| Toast/status | Save result, undo, background sync | Nonblocking; never the only error location |

Z-index and elevation should follow this order. Do not solve overlap with
random z-index values; promote the component to the correct layer.

### 13.6 Animation Layers

Motion is organized into layers. Each layer has a job and a maximum intensity.

| Layer | Job | Duration | Allowed properties |
|---|---|---:|---|
| Feedback | Confirm hover, focus, press, toggle | 80-120ms | color, background, border, opacity, small scale |
| Reveal | Open menus, disclosures, filters | 120-180ms | opacity, transform, height with measured content |
| Spatial | Move inspector, panel, modal | 180-260ms | transform, opacity |
| Data change | Insert/remove/update rows, chips, toasts | 120-220ms | opacity, background flash, transform if stable |
| Progress | Long-running import/export/analysis | continuous but restrained | progress bar, spinner, skeleton shimmer only if subtle |
| Brand delight | Rare, nonessential polish | 160-320ms | only on overview or success moments, disabled by reduced motion |

Never animate:

- Table column widths during reading.
- Money values in a way that obscures the final value.
- Layout shifts caused by late-loading controls.
- Row height in dense ledgers unless the user explicitly expands/collapses.
- Error messages disappearing before the user can read them.
- Decorative loops on work surfaces.

### 13.7 Data and Financial Change Motion

Financial data changes need clarity more than flair.

- New row: brief background tint, then settle.
- Updated amount: tint the changed cell only; do not animate every cell.
- Deleted row: remove after confirmation or undo timeout; keep undo visible.
- Bulk edit: show selected count, preview affected records, then progress and
  result summary.
- Recalculated KPI: show a small "updated" status or timestamp; avoid count-up
  animations for money unless the value is purely illustrative.
- Import progress: stage-based progress with row counts and validation state,
  not a generic spinner.
- Sync/backup/export: show source, destination, progress, completion, and
  failure recovery.

### 13.8 Loading, Skeletons, and Readiness

Loading states must preserve layout and explain readiness.

- Use skeletons only when final structure is known.
- Use progress bars for staged work with measurable progress.
- Use spinners only for short indeterminate waits under 2 seconds.
- Long-running operations show a stage label and safe background behavior.
- Route/surface changes should not show mixed old/new content.
- Once content is ready, controls should not jump due to late badges, icons, or
  banners.

Skeletons should match the density of the final UI. A ledger skeleton uses rows
and columns; a KPI skeleton uses compact blocks; a wizard skeleton uses step
regions.

### 13.9 Reduced Motion and Accessibility

Reduced motion must be a first-class mode, not an afterthought.

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```

Reduced motion rules:

- Preserve instant state changes: selected, focused, expanded, loading, error.
- Replace spatial movement with opacity or immediate show/hide.
- Keep progress indicators, but avoid shimmer and looping decorative motion.
- Do not depend on animation completion for focus restoration or state updates.

### 13.10 Interaction Acceptance Checklist

Before accepting an interaction-heavy surface:

- Every visible control has rest, hover, focus-visible, pressed, disabled, and
  loading behavior where applicable.
- Every hidden or hover-revealed control has a keyboard path.
- Focus order matches visual priority.
- Escape closes layers in the correct order and restores focus.
- Dirty forms show unsaved state and provide save/revert.
- Bulk actions have selection, preview, commit, result, and undo/rollback where
  feasible.
- Async actions provide feedback within 100ms.
- Errors are specific, persistent enough to read, and recoverable.
- Reduced-motion mode still communicates every state change.
- No motion causes layout shift in dense reading or ledger contexts.

## 14. Implementation Mapping

### 14.1 CSS Architecture

Recommended migration:

1. Add semantic alias tokens after existing theme vars.
2. Add density vars:
   `--control-h`, `--field-h`, `--row-h`, `--card-pad`, `--section-gap`.
3. Convert primitives in `internal/ui` to use semantic classes.
4. Replace hardcoded component light-mode overrides with token-driven surfaces.
5. Add an archetype class per surface family.
6. Add visual regression checks for token usage, overflow, density, and theme
   parity.

### 14.2 Primitive Mapping

| Current primitive | Spec responsibility |
|---|---|
| `Card` | Section/card anatomy, no nested cards |
| `EntityListSection` | Entity list template and empty states |
| `DataTable` | Dense ledger/table rules |
| `FilterToolbar` | Search/facet/chip/action order |
| `Widget` | Configurable work-surface widget mechanics |
| `StatGrid` / `Stat` | KPI and headline figure system |
| `OverflowMenu` | Action demotion and row control density |
| `FlipPanel` | Focus mode, inspectors, layer behavior, and focus restoration |
| `EmptyStateCTA` | Useful empty/gated states |
| `Segmented`, `Toggle`, `StepperPill` | Compact controls with keyboard semantics |

### 14.3 Migration Bridge for Proven Existing Pieces

Use this checklist as a bridge from current CSS to the target system. These
selectors are not the future design contract. They identify where reusable
behavior already exists and where hardcoded styling should be replaced.

| Selector group | Required migration |
|---|---|
| `.page`, `.page-title`, `.page-sub` | Add archetype classes and fixed type tokens; remove viewport-scaled title sizing on desktop. |
| `.card`, `.card-head`, `.card-title` | Move to `--surface-card`, `--border-subtle`, `--radius-lg`, `--card-pad`; remove nested-card usage before visual polish. |
| `.stat-grid`, `.stat`, `.kpi`, `.stat-value` | Convert to KPI anatomy; use tabular numbers, semantic money tokens, and one headline figure rule. |
| `.btn`, `.btn-primary`, `.btn-ghost`, `.btn-sm` | Move to control height tokens; enforce one primary per region and demote extra actions to `OverflowMenu`. |
| `.btn-danger`, `.btn-ghost-danger`, `.btn-del` | Split destructive action tokens from negative-money tokens; verify label contrast in both themes. |
| `.field`, `select:not(.set-input):not(.seg-btn)` | Use field height/density tokens and focus matrix; remove hardcoded light/dark surface fixes. |
| `.txn-table`, `.th-sort`, `.td-actions`, `.td-select` | Move to `DataTable` rules: stable row height, right-aligned money, hidden secondary actions, bulk toolbar states. |
| `.row`, `.row-main`, `.row-meta`, `.row-edit` | Separate scan rows from edit rows; prevent generic hover motion on ledger-like lists. |
| `.w`, `.wh`, `.bento` | Keep reusable widget mechanics only where useful; do not preserve the bento paradigm by default. Tokenize surfaces and let future archetypes choose layout. |
| `.smart-card`, `.card-alert`, severity badges | Use the Smart/alert anatomy with capped inline count, severity rail, evidence, and action hierarchy. |
| `.seg`, `.seg-btn`, `.rpill`, `.rstep`, `.toggle-row` | Normalize compact control dimensions and roving-tabindex/focus states. |
| `.set-btn`, `.set-input`, `.set-close`, settings panels | Align with form/editor rules; separate save, dismiss, and destructive roles. |
| `.add-wrap`, `.add-menu`, `.menu-btn`, `.icon-btn` | Treat as popovers/icon controls with 32px desktop target and strong focus restoration. |
| `[data-theme="light"] ...` overrides | Replace with semantic aliases and computed contrast tests; delete only after light contact sheets pass hierarchy review. |
| hardcoded hexes in component rules | Replace with semantic token roles or document as generated asset constants. |

### 14.4 Archetype Contract Examples

These examples define reusable contracts. A future IA can combine, rename, or
split surfaces without invalidating the visual system.

| Archetype | Primary question | First answer | Primary action | Collapse or defer |
|---|---|---|---|---|
| Overview | Where do I stand, and what needs attention? | Current position plus top exceptions | Review top exception or add/update financial record | Large widget grids, promotions, secondary insights |
| Ledger | What changed, and what needs cleanup? | Searchable record set, count, total, and unresolved state | Add record, open filters, or inspect selected row | Row secondary actions, advanced filters, bulk toolbar until selection |
| Planning | Am I on plan, and what should change? | Status, variance, main driver, and recommendation | Adjust plan or create scenario | Assumptions, secondary cards, long explanations |
| Payment/obligation | What must happen next, and by when? | Next due item, overdue state, cash impact | Mark paid, schedule, or resolve issue | Calendar detail, history, low-priority actions |
| Recommendation | What is worth acting on now? | Ranked finding with severity, evidence, and confidence | Act, snooze, dismiss, or inspect why | Catalog settings and repeated rule hits |
| Import/export/restore | What will change if I proceed? | Source, affected records, conflicts, and reversibility | Validate, commit, or cancel | Advanced mapping and raw logs until needed |
| Evidence/document | What evidence exists, and is it trustworthy? | Storage health, recent evidence, linked records | Upload/import or link evidence | Low-value metadata and destructive controls |
| Builder/automation | What am I configuring, and what will it affect? | Preview, selected object, validation, and unsaved state | Save, test, publish, or revert | Formula syntax, matrices, expert controls |
| Settings/admin | What can I configure safely? | Current status, grouped controls, risk level | Save setting or authenticate | Danger zone, advanced tokens, audit details |
| Help/explainability | How do I complete this workflow? | Search result or contextual explanation | Open guide, inspect why, or jump to workflow | Generic articles below task-specific help |

## 15. Quality Gates

Before accepting any desktop UI implementation:

- Surface has an information priority contract.
- Surface has a responsive priority contract for wide, medium, compact, and
  narrow viewports.
- First viewport control count is within budget or documented as an explicit
  edit mode exception.
- No page-level horizontal overflow at any verified width.
- No duplicate primary navigation systems at any verified width.
- No nested cards.
- No hardcoded component colors outside token/theme code.
- Light and dark themes preserve hierarchy, not just contrast.
- Keyboard-only workflow works for the primary job.
- Interaction states cover rest, hover, focus-visible, pressed, selected,
  dirty, loading, success, warning, error, and disabled where applicable.
- Hidden/hover-revealed controls have keyboard, command, or inspector access.
- Motion uses the duration/easing tokens and never causes layout shift in dense
  reading contexts.
- Reduced-motion mode preserves all state changes without decorative movement.
- Tables/lists preserve identity, amount/value, time, and status when collapsed
  responsively.
- Charts remain legible or collapse into summary + drilldown.
- Coarse-pointer controls meet responsive target sizing and do not depend on
  hover-only actions.
- Screen-reader order matches the visual priority.
- Every chart has a stated takeaway and accessible description.
- Smart cards are capped, ranked, and explain why they appear.
- Destructive/import/restore flows have preview, confirmation, result, and
  audit trail.
- Page screenshots pass visual review in dark and light themes.

## 16. References Used

These sources informed the spec. The CashFlux implementation should adapt them
to a local-first household finance product rather than copy any single system.

- IBM Carbon Data Table usage:
  https://carbondesignsystem.com/components/data-table/usage/
- IBM Carbon spacing:
  https://carbondesignsystem.com/elements/spacing/overview/
- IBM Carbon 2x Grid:
  https://carbondesignsystem.com/elements/2x-grid/overview/
- Material Design 3 density:
  https://m3.material.io/foundations/layout/grids-spacing/density
- Material Design 3 spacing:
  https://m3.material.io/foundations/layout/grids-spacing/spacing
- Microsoft Fluent 2 layout:
  https://fluent2.microsoft.design/layout
- Microsoft Fluent UI web component styling:
  https://learn.microsoft.com/en-us/fluent-ui/web-components/getting-started/styling
- Atlassian spacing:
  https://atlassian.design/foundations/spacing
- Nielsen Norman Group, dashboard data visualization:
  https://www.nngroup.com/videos/data-visualizations-dashboards/
- Nielsen Norman Group, preattentive dashboard attributes:
  https://www.nngroup.com/articles/dashboards-preattentive/
- Nielsen Norman Group, choosing chart types:
  https://www.nngroup.com/articles/choosing-chart-types/
- Nielsen Norman Group, data tables and user tasks:
  https://www.nngroup.com/articles/data-tables/
- W3C WCAG 2.2:
  https://www.w3.org/TR/WCAG22/
- W3C Focus Appearance:
  https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html
- W3C WCAG 2.2 new criteria:
  https://www.w3.org/WAI/standards-guidelines/wcag/new-in-22/
- Apple Human Interface Guidelines, typography:
  https://developer.apple.com/design/human-interface-guidelines/typography
- Apple Human Interface Guidelines, layout:
  https://developer.apple.com/design/human-interface-guidelines/layout
