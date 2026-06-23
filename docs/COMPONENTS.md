# CashFlux — Component Inventory (`internal/ui`)

All exported primitives live in `internal/ui` (build tag `js && wasm` unless noted).
Pure helpers (no build tag, native-testable): `SelectOption`, `OptionsFrom`, `IndentPx`, `JoinClass`.

---

## Primitives

### Card (`primitives.go`)
**Purpose:** Titled section card (`.card` + optional `.card-title` + optional header-action + body).
Absorbs the 170× hand-rolled `Section(css.Class("card"), H2(…), …)` scaffold.
```go
ui.Card(ui.CardProps{Title: "Accounts", HeaderAction: addBtn, Body: listNode})
```

### EntityListSection (`primitives.go`)
**Purpose:** Full card + title + header-action + empty-state-OR-body; the canonical CRUD section shell.
Wraps `Card`; screens supply title, optional add button, empty state, and list body.
```go
ui.EntityListSection(ui.EntityListSectionProps{Title: "Budgets", HeaderAction: addBtn, EmptyState: emptyNode, Body: rowsNode})
```

### FormField (`primitives.go`)
**Purpose:** Labeled field — visible caption above a control (`.labeled-field`). Fixes unlabelled
controls across C49–C65/B15.
```go
ui.FormField("Account name", Input(css.Class("field"), …))
```

### SelectInput (`select.go`)
**Purpose:** Native `<select>` from a typed `[]SelectOption`. Kills the 103× hand-rolled `Option`
loops; owns its `OnChange` hook (safe in loops via `uic.CreateElement`).
```go
ui.SelectInput(ui.SelectInputProps{Options: opts, Selected: cur, OnChange: fn, AriaLabel: "Category"})
```

### SelectOption / OptionsFrom (`select_pure.go` — no build tag)
**Purpose:** `SelectOption` is the typed value+label pair. `OptionsFrom[T]` converts any slice to
`[]SelectOption` with value/label extractor funcs.
```go
opts := ui.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, currentID)
```

### EntityRow (`primitives.go`)
**Purpose:** Generic list row (`.row`): optional leading node, `.row-desc` title, `.row-meta` lines,
trailing action nodes. Hookless — safe to call directly inside `MapKeyed` loops.
```go
ui.EntityRow(ui.EntityRowProps{Title: a.Name, Meta: []string{fmtBalance(a)}, Actions: []uic.Node{editBtn, deleteBtn}})
```

### IconButton (`primitives.go`)
**Purpose:** Accessible icon-only button (`aria-label` + `title`). Optional `.btn-del` danger style.
Owns its click hook — safe in variable-length loops.
```go
ui.IconButton(ui.IconButtonProps{Icon: icon.Edit, Label: "Edit account", OnClick: fn})
```

### DeleteButton (`primitives.go`)
**Purpose:** Destructive icon-only button (`.btn-del` + Close/Trash icon). Consolidates the 18×
hand-rolled delete-button pattern; owns its click hook (loop-safe).
```go
ui.DeleteButton(ui.DeleteButtonProps{AriaLabel: "Delete transaction", OnClick: fn})
```

### ExportButton (`primitives.go`)
**Purpose:** Labeled export/download button (inline-flex icon + span). Consolidates the 14× hand-rolled
export-button pattern; caller supplies `OnClick` (keeps primitive free of `downloadBytes`/`syscall/js`).
```go
ui.ExportButton(ui.ExportButtonProps{Label: "Export CSV", OnClick: func() { downloadBytes(…) }})
```

### StatGrid / Stat (`primitives.go`)
**Purpose:** `.stat-grid` of labeled figures with optional tone classes (`"pos"`, `"neg"`, `"dim"`).
Promotes the `stat()` helper repeated 9× across screens.
```go
ui.StatGrid([]ui.Stat{{Label: "Net worth", Value: "$12,400", Tone: "pos"}, …})
```

### OverflowMenu (`overflowmenu.go`)
**Purpose:** ⋯ trigger + popover menu (`add-wrap`/`add-menu` pattern). Consolidates 27× scattered
overflow/quick-add patterns. Each menu item may be `Hidden: true` for conditional visibility.
```go
ui.OverflowMenu(ui.OverflowMenuProps{Items: []ui.OverflowMenuItem{{Label: "Edit", Icon: icon.Edit, OnSelect: fn}}})
```

### InlineEditForm (`inlineeditform.go`)
**Purpose:** `.row-edit` + `.form-grid` + Save/Cancel chrome. The per-row edit wrapper repeated on
every CRUD screen; wires `OnSubmit`, `OnSave`, `OnCancel`, and optional extra content.
```go
ui.InlineEditForm(ui.InlineEditFormProps{Fields: []uic.Node{…}, OnSave: saveFn, OnCancel: cancelFn})
```

### TreeRows / IndentPx (`treerows.go` — no build tag)
**Purpose:** `IndentPx(depth int) string` returns the CSS `padding-left` value for a nesting depth
(16 px per level). Used by category-tree and nested-task rows to produce real indentation (not
em-dash prefixes per C63/C72).
```go
Style(map[string]string{"padding-left": ui.IndentPx(node.Depth)})
```

### DataTable (`datatable.go`)
**Purpose:** Sortable, paginated `<table>` shell. Caller renders `<tr>` rows; `DataTable` owns the
semantic table, sortable `<th>` headers (with `aria-sort` + caret), and pagination footer.
```go
ui.DataTable(ui.DataTableProps{Columns: cols, Body: rowNodes, Sort: sortKey, Dir: "asc", OnSort: fn, Page: p, PageSize: 25, Total: n, OnPage: pgFn})
```

### FilterToolbar (`filtertoolbar.go`)
**Purpose:** Search box + Filters popover trigger + active-filter chips + trailing action buttons.
Screen-agnostic — used by Transactions; adopt for Reports, Categories, Rules per Phase 2.
```go
ui.FilterToolbar(ui.FilterToolbarProps{Search: q, OnSearch: fn, FilterFields: fieldNodes, Chips: chips, OnRemoveChip: removeFn})
```

### FlipPanel (`flippanel.go`)
**Purpose:** Dimmed/blurred backdrop + 3D-flip settings overlay. Generic props-driven shell used by
per-widget and global settings; caller supplies title, body, size, `OnSave`, `OnClose`.
```go
ui.FlipPanel(ui.FlipPanelProps{Title: "Widget settings", Back: bodyNode, OnSave: saveFn, OnClose: closeFn})
```

### Widget (`widget.go`)
**Purpose:** Bento dashboard cell shell: grip handle, centered title (drill-to-screen), gear button,
padded body, optional resize handles, drag-reorder. Every dashboard tile uses this.
```go
ui.Widget(ui.WidgetProps{ID: "budgets", Title: "Budgets", Body: contentNode, Draggable: true, Resizable: true})
```

### AreaChart (`chart.go`)
**Purpose:** Filled area sparkline SVG from a `[]float64` value series. Stretches to container width.
```go
ui.AreaChart(ui.AreaChartProps{Values: vals, Stroke: "#2e8b57", Label: "Cash-flow trend"})
```

### ProgressBar (`progress.go`)
**Purpose:** Horizontal fill bar (`.prog-track` + `.prog-fill`) with a tone token (`"bg-up"`,
`"bg-warn"`, `"bg-down"`, `"bg-dim"`, `"bg-fg"`). Clamps percent to `[0, 100]`.
```go
ui.ProgressBar(ui.ProgressBarProps{Percent: 68, Tone: "bg-warn"})
```

### Icon (`icon.go`)
**Purpose:** Compile-checked stroked SVG icon from the curated `icon.Name` set; inherits color from
`currentColor`, sized by caller classes.
```go
ui.Icon(icon.Accounts, css.Class(tw.W4, tw.H4))
```

### Segmented (`controls.go`)
**Purpose:** Row of mutually exclusive options (`.seg`). Roving tabindex + arrow-key navigation
(ARIA `radiogroup`). Reused for time resolution, theme selector, etc.
```go
ui.Segmented(ui.SegmentedProps{Options: []ui.SegOption{{Value: "month", Label: "Month"}, …}, Selected: cur, OnSelect: fn})
```

### Toggle (`controls.go`)
**Purpose:** Pill on/off switch (`.switch`, ARIA `role="switch"`). Keyboard-accessible (Space/Enter).
```go
ui.Toggle(ui.ToggleProps{On: enabled, OnChange: fn, Label: "Dark mode"})
```

### ToggleRow (`controls.go`)
**Purpose:** Settings row (`.toggle-row`): label left + Toggle right. Standard building block for
preferences forms.
```go
ui.ToggleRow(ui.ToggleRowProps{Label: "Show archived accounts", On: prefs.ShowArchived, OnChange: fn})
```

### Swatch / SwatchPicker (`controls.go`)
**Purpose:** `Swatch` is one selectable color chip (ARIA `role="radio"`). `SwatchPicker` renders a
roving-tabindex radiogroup of swatches; used by accent-color and widget-style pickers.
```go
ui.SwatchPicker(ui.SwatchPickerProps{Colors: palette, Selected: cur, OnSelect: fn})
```

### StepperPill (`controls.go`)
**Purpose:** Centered label flanked by previous/next chevrons (`.rpill`). Generic stepped-value
control reused for period paging, budget-month navigation, etc.
```go
ui.StepperPill(ui.StepperPillProps{Label: "June 2026", OnPrev: prevFn, OnNext: nextFn})
```

### EmptyStateCTA (`screens/emptystate.go` — screen package)
**Purpose:** Friendly empty-state block: muted icon + message + CTA button. CTA can focus an
inline form field (`FocusID`), open a modal (`AddTarget`), or navigate (`Href`).
```go
ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: t("accounts.empty"), CTALabel: t("accounts.addFirst"), AddTarget: "account"})
```

---

## Porting Guide — Replacing Legacy Idioms

| Legacy idiom | Replace with |
|---|---|
| `Section(css.Class("card"), H2(css.Class("card-title"), title), body)` | `ui.Card(ui.CardProps{Title: title, Body: body})` |
| `Section(css.Class("card"), H2(…), addBtn, empty-or-rows)` | `ui.EntityListSection(ui.EntityListSectionProps{Title: …, HeaderAction: addBtn, EmptyState: …, Body: rowsNode})` |
| `Div(css.Class("rows"), MapKeyed(items, key, render))` | Pass as `Body` to `EntityListSection`; or keep as-is inside the Body arg |
| `for _, o := range items { Option(Value(o.ID), SelectedIf(cur == o.ID), o.Name) }` | `ui.SelectInput(ui.SelectInputProps{Options: ui.OptionsFrom(items, …), Selected: cur, OnChange: fn})` |
| `Button(css.Class("btn-del"), OnClick(…), Icon(icon.Close, …))` | `ui.DeleteButton(ui.DeleteButtonProps{AriaLabel: "Delete …", OnClick: fn})` |
| `Button(css.Class("btn", tw.InlineFlex, …), OnClick(…), Icon(…), Span(label))` (export) | `ui.ExportButton(ui.ExportButtonProps{Label: label, OnClick: fn})` |
| `Button(css.Class("btn"), Attr("aria-label", lbl), OnClick(…), Icon(…))` (icon-only) | `ui.IconButton(ui.IconButtonProps{Icon: ic, Label: lbl, OnClick: fn})` |
| `Div(css.Class("stat-grid"), stat(label, val), …)` / inline `stat()` helper | `ui.StatGrid([]ui.Stat{{Label: …, Value: …, Tone: …}, …})` |
| `Label(css.Class("labeled-field"), Span(css.Class("t-caption", …), lbl), control)` | `ui.FormField(lbl, control)` |
| `Style(map[string]string{"padding-left": fmt.Sprintf("%dpx", depth*16)})` (tree indent) | `Style(map[string]string{"padding-left": ui.IndentPx(depth)})` |
| Bare `P(css.Class("empty"), msg)` inside a card | `ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: msg, …})` |

### Migration order (Phase plan)
- **Phase 1 (forms):** Every add/edit form → `FormField` + `SelectInput`/`OptionsFrom`.
- **Phase 2 (lists):** `Div(.rows)` → `EntityListSection` (with existing body) or `DataTable`/`FilterToolbar` for sortable screens.
- **Phase 3 (rows):** Split `*Row` → `*DisplayRow` + `*EditForm`; fold display into `EntityRow`.
- **Phase 4 (done):** Super-screens decomposed (Planning, Documents, Allocate, Customize, settings).
- **Phase 5 (cleanup):** Delete residual bespoke markup; enforce ratchet via `scaffold_baseline_test.go`.
