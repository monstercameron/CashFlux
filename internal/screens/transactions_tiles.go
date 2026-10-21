// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nlfilter"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnreview"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// --- toolbar field helpers ------------------------------------------------------
// These compose the reusable uiw primitives (SelectInput / OptionsFrom) so the
// toolbar stays declarative — "label + options + a plain handler" — instead of
// repeating the hand-rolled Select/aria/Option-loop pattern per field.

// chipKeySep joins a filter chip's field and value into its stable key (an unlikely
// character so it never collides with an entity id, tag, or date value).
const chipKeySep = "\x1f"

type txnToolbarIndexes struct {
	accountNames, categoryNames, memberNames       map[string]string
	accountOptions, categoryOptions, memberOptions []uiw.SelectOption
	sourceOptions, tagOptions                      []uiw.SelectOption
	tags                                           []string
}

type txnToolbarSummary struct {
	review, duplicates, suspects         int
	presets                              txnfilter.PresetCounts
	shownNet, movedMinor                 int64
	spentMinor, incomeMinor              int64 // cashflow fold over Shown (R3P: feeds the count line)
	unconverted, movedRows, unpairedRows int
	conversionIssues                     []txnfilter.ConversionIssue
}

// txnSummaryScope strips the fields that reorder or paginate the SAME set.
// Nothing in txnToolbarSummary depends on them — sums and counts over a set
// do not change when it is re-sorted or re-paged — so keying the summary
// cache on the full filter re-ran its whole-ledger scans (review count,
// duplicate scan, preset counts) on every sort click and page change (R3P).
func txnSummaryScope(f uistate.TxFilter) uistate.TxFilter { return txntoolbar.SummaryScope(f) }

type txnToolbarSummaryCache struct {
	filter uistate.TxFilter
	memo   txntoolbar.Memo[txnToolbarSummary]
}

func (c *txnToolbarSummaryCache) get(revision int, filter uistate.TxFilter, compute func() txnToolbarSummary) txnToolbarSummary {
	return c.memo.Get(revision, filter, compute)
}

func buildTxnToolbarIndexes(accounts []domain.Account, categories []domain.Category, members []domain.Member, txns []domain.Transaction) txnToolbarIndexes {
	indexes := txnToolbarIndexes{}
	names := txntoolbar.BuildIndexes(accounts, categories, members, txns)
	indexes.accountNames, indexes.categoryNames, indexes.memberNames = names.Accounts, names.Categories, names.Members
	indexes.tags = names.Tags
	indexes.accountOptions = withAllOption(uistate.T("transactions.allAccounts"), uiw.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, ""))
	indexes.categoryOptions = withAllOption(uistate.T("transactions.allCategories"), uiw.OptionsFrom(categories, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name }, ""))
	indexes.memberOptions = withAllOption(uistate.T("transactions.allMembers"), uiw.OptionsFrom(members, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, ""))
	indexes.sourceOptions = withAllOption(uistate.T("transactions.allSources"), uiw.OptionsFrom(domain.AllTxnSources, func(s domain.TxnSource) string { return string(s) }, func(s domain.TxnSource) string { return s.Label() }, ""))
	indexes.tagOptions = withAllOption(uistate.T("transactions.allTags"), uiw.OptionsFrom(indexes.tags, func(s string) string { return s }, func(s string) string { return s }, ""))
	return indexes
}

// withAllOption prepends the empty-value "all / none" choice to an option list.
func withAllOption(allLabel string, opts []uiw.SelectOption) []uiw.SelectOption {
	return append([]uiw.SelectOption{{Value: "", Label: allLabel}}, opts...)
}

// withFieldLabel wraps a control in the toolbar's `field-label` shell.
func withFieldLabel(label string, control ui.Node) ui.Node {
	return Label(css.Class("field-label"), label, control)
}

// filterSelect is one labeled <select> filter: the reusable SelectInput (which owns
// its own change hook) inside a field-label. onPick gets the chosen value.
func filterSelect(label, selected string, opts []uiw.SelectOption, onPick func(string)) ui.Node {
	return withFieldLabel(label, uiw.SelectInput(uiw.SelectInputProps{
		Options: opts, Selected: selected, AriaLabel: label, OnChange: onPick,
	}))
}

// dateField / amountField are labeled date / amount inputs. Their change hook is
// created by the caller and passed in, since a text input fires on every keystroke
// (so the handler must live at a stable position in the owning component).
func dateField(label, value string, style prefs.DateStyle, onPick func(string)) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) {
		raw := strings.TrimSpace(e.GetValue())
		if raw == "" {
			onPick("")
			return
		}
		if parsed, ok := parseFilterDate(style, raw); ok {
			onPick(parsed)
		}
	})
	return withFieldLabel(label, Input(css.Class("field"), Type("text"),
		Attr("aria-label", label),
		Attr("data-testid", "txn-filter-date-"+strings.ToLower(strings.ReplaceAll(label, " ", "-"))),
		Placeholder(filterDatePlaceholder(style)), OnChange(onChange), uiw.FieldValue(formatFilterDate(style, value))))
}

func amountField(label, placeholder, value string, onInput ui.Handler) ui.Node {
	return withFieldLabel(label, Span(css.Class("filter-amount-control"),
		Span(css.Class("filter-amount-prefix"), Attr("aria-hidden", "true"), "$"),
		Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
			Attr("aria-label", label), Placeholder(placeholder), OnInput(onInput), uiw.FieldValue(value))))
}

func filterDatePlaceholder(style prefs.DateStyle) string {
	return formatFilterDate(style, "2006-01-02")
}

func formatFilterDate(style prefs.DateStyle, value string) string {
	return txntoolbar.FormatDate(style, value)
}

func parseFilterDate(style prefs.DateStyle, value string) (string, bool) {
	return txntoolbar.ParseDate(style, value)
}

func filterDateLayout(style prefs.DateStyle) string { return txntoolbar.DateLayout(style) }

// applyParsedDateBounds replaces both bounds when a parsed date clause exists.
// Empty bounds are meaningful: `since March` intentionally removes an older To.
func applyParsedDateBounds(current *uistate.TxFilter, parsed txnfilter.Criteria) {
	txntoolbar.ApplyParsedDateBounds(current, parsed)
}

// containsStr reports whether ss contains v.
func containsStr(ss []string, v string) bool { return txntoolbar.Contains(ss, v) }

func formatTxnChipDate(iso string) string {
	return formatTxnChipDateWith(iso, uistate.LoadPrefs().FormatDate)
}

func formatTxnChipDateWith(iso string, format func(time.Time) string) string {
	return txntoolbar.ChipDate(iso, format)
}

func txnSearchChipLabel(query string) string {
	return uistate.T("transactions.chipFiltering", query)
}

// txnAmountChipLabel shows which bound a chip removes and formats its value in
// the household's base currency. Invalid stored values remain visible for repair.
func txnAmountChipLabel(bound, value, base string) string {
	return txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}.AmountChipLabel(bound, value, base)
}

// txnFilterRangeError explains a user-entered range whose lower bound exceeds its
// upper bound. Malformed legacy values are left to the field and parser to handle.
func txnFilterRangeError(filter uistate.TxFilter, base string) string {
	return txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}.RangeError(filter, base)
}

// excludedTxnCount reports how many visible rows are omitted from reports.
func excludedTxnCount(txns []domain.Transaction) int { return txntoolbar.ExcludedCount(txns) }

// filterPillProps configures one toggle pill in a multi-select filter group. It is its
// own component so its OnClick hook sits at a stable position (the pill list is
// variable-length — see the framework loop-hook gotcha).
type filterPillProps struct {
	Field    txnfilter.FilterField
	Value    string
	Label    string
	Selected bool
	Option   bool
	OnToggle func(txnfilter.FilterField, string)
}

// FilterPill renders one selectable filter value as a toggle pill: neutral when off,
// accent-filled when on. Clicking toggles the value in its dimension's multi set.
func FilterPill(props filterPillProps) ui.Node {
	onClick := ui.UseEvent(Prevent(func() { props.OnToggle(props.Field, props.Value) }))
	cls := "filter-pill"
	pressed := "false"
	if props.Selected {
		cls = "filter-pill on"
		pressed = "true"
	}
	args := []any{css.Class(cls), Type("button"), Attr("aria-pressed", pressed), OnClick(onClick), props.Label}
	if props.Option {
		selected := "false"
		if props.Selected {
			selected = "true"
		}
		args = append(args, Attr("role", "option"), Attr("aria-selected", selected))
	}
	return Button(args...)
}

// filterMultiGroup renders one filter dimension as a labelled group of toggle pills —
// the multi-select control replacing the old single <select>. Multiple values can be
// on at once (OR-within the dimension). The empty-value "All" option is skipped (an
// empty set already means "all").
func filterMultiGroup(label string, field txnfilter.FilterField, selected []string, opts []uiw.SelectOption, onToggle func(txnfilter.FilterField, string)) ui.Node {
	if field == txnfilter.FieldAccount || field == txnfilter.FieldCategory || field == txnfilter.FieldTag {
		return ui.CreateElement(filterSearchGroup, filterGroupProps{
			Label: label, Field: field, Selected: selected, Opts: opts, OnToggle: onToggle,
		})
	}
	return ui.CreateElement(filterGroup, filterGroupProps{
		Label: label, Field: field, Selected: selected, Opts: opts, OnToggle: onToggle,
	})
}

func filterSearchGroup(props filterGroupProps) ui.Node {
	query := ui.UseState("")
	expanded := ui.UseState(false)
	onQuery := ui.UseEvent(func(e ui.Event) { query.Set(e.GetValue()) })
	onExpand := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))

	filtered, count := txnSearchOptions(props.Opts, props.Selected, query.Get())
	visible := visibleFilterGroupOptions(filtered, props.Selected, len(filtered) > filterGroupCollapseThreshold && !expanded.Get())
	items := MapKeyed(visible,
		func(opt uiw.SelectOption) any { return opt.Value },
		func(opt uiw.SelectOption) ui.Node {
			return ui.CreateElement(FilterPill, filterPillProps{
				Field: props.Field, Value: opt.Value, Label: opt.Label,
				Selected: containsStr(props.Selected, opt.Value), Option: true, OnToggle: props.OnToggle,
			})
		},
	)
	fieldName := string(props.Field)
	searchNoun := "items"
	switch props.Field {
	case txnfilter.FieldAccount:
		searchNoun = "accounts"
	case txnfilter.FieldCategory:
		searchNoun = "categories"
	case txnfilter.FieldTag:
		searchNoun = "tags"
	}
	return Div(css.Class("filter-group", "filter-search-group"),
		Div(css.Class("filter-group-head"),
			Span(css.Class("filter-group-label"), props.Label),
			If(count > 0, Span(css.Class("filter-group-selcount"), uistate.T("transactions.filterSelected", count))),
		),
		Input(css.Class("field", "filter-group-search"), Type("search"),
			Attr("role", "combobox"), Attr("aria-autocomplete", "list"), Attr("aria-expanded", "true"),
			Attr("aria-label", uistate.T("transactions.filterSearchAria", searchNoun)),
			Attr("aria-controls", "filter-options-"+fieldName),
			Attr("data-testid", "filter-search-"+fieldName),
			Placeholder(uistate.T("transactions.filterSearchPlaceholder", searchNoun)),
			uiw.FieldValue(query.Get()), OnInput(onQuery)),
		Div(css.Class("filter-pills", "filter-search-options"), Attr("id", "filter-options-"+fieldName),
			Attr("role", "listbox"), Attr("aria-multiselectable", "true"), items,
			If(len(filtered) > filterGroupCollapseThreshold, ui.CreateElement(filterGroupDisclosure, filterGroupDisclosureProps{
				Field: props.Field, Open: expanded.Get(),
				Label: filterGroupDisclosureLabel(len(filtered), expanded.Get()), Toggle: onExpand,
			})),
			If(len(visible) == 0, Span(css.Class("filter-search-empty"), uistate.T("transactions.filterSearchEmpty"))),
		),
	)
}

func filterGroupDisclosureLabel(total int, expanded bool) string {
	if expanded {
		return uistate.T("transactions.filterShowLess")
	}
	return uistate.T("transactions.filterShowAll", total)
}

// filterGroupCollapseThreshold is the option count above which a filter dimension
// collapses by default — beyond this, showing every pill turns the panel into a
// chip wall that buries the ledger (a Category dimension has ~25, Tags ~40).
const filterGroupCollapseThreshold = 8

type filterGroupProps struct {
	Label    string
	Field    txnfilter.FilterField
	Selected []string
	Opts     []uiw.SelectOption
	OnToggle func(txnfilter.FilterField, string)
}

// filterGroup renders one categorical filter dimension as a group of toggle pills.
// Large dimensions collapse by default to the first eight options plus any
// selected options outside that range, so the filter panel stays scannable
// without hiding the active filter. It owns its expand state (a stable
// per-dimension component, not a loop-level hook).
func filterGroup(props filterGroupProps) ui.Node {
	expanded := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))

	// Count real (non-"all") options and how many are selected.

	real, selCount := txnSearchOptions(props.Opts, props.Selected, "")
	total := len(real)
	collapsible := total > filterGroupCollapseThreshold
	open := expanded.Get()

	shown := visibleFilterGroupOptions(props.Opts, props.Selected, collapsible && !open)

	pills := MapKeyed(shown,
		func(o uiw.SelectOption) any { return o.Value },
		func(o uiw.SelectOption) ui.Node {
			if o.Value == "" {
				return Fragment()
			}
			return ui.CreateElement(FilterPill, filterPillProps{
				Field: props.Field, Value: o.Value, Label: o.Label,
				Selected: containsStr(props.Selected, o.Value), OnToggle: props.OnToggle,
			})
		},
	)

	var discBtn ui.Node = Fragment()
	if collapsible {
		discLabel := uistate.T("transactions.filterShowAll", total)
		if open {
			discLabel = uistate.T("transactions.filterShowLess")
		}
		discBtn = ui.CreateElement(filterGroupDisclosure, filterGroupDisclosureProps{
			Field: props.Field, Open: open, Label: discLabel, Toggle: toggle,
		})
	}

	// A "· N selected" badge on the label so a collapsed group still tells you it's active.
	var selBadge ui.Node = Fragment()
	if selCount > 0 {
		selBadge = Span(css.Class("filter-group-selcount"), uistate.T("transactions.filterSelected", selCount))
	}

	return Div(css.Class("filter-group"),
		Div(css.Class("filter-group-head"),
			Span(css.Class("filter-group-label"), props.Label),
			selBadge,
		),
		Div(css.Class("filter-pills"), pills, discBtn),
	)
}

// visibleFilterGroupOptions returns all real options unless a large group is
// collapsed. A collapsed group keeps the first filterGroupCollapseThreshold
// options and adds selected options beyond them in their original order.
func visibleFilterGroupOptions(options []uiw.SelectOption, selected []string, collapsed bool) []uiw.SelectOption {
	plain := make([]txntoolbar.Option, len(options))
	for i, o := range options {
		plain[i] = txntoolbar.Option{Value: o.Value, Label: o.Label}
	}
	visible := txntoolbar.VisibleOptions(plain, selected, collapsed)
	result := make([]uiw.SelectOption, len(visible))
	for i, o := range visible {
		result[i] = uiw.SelectOption{Value: o.Value, Label: o.Label}
	}
	return result
}

type filterGroupDisclosureProps struct {
	Field  txnfilter.FilterField
	Open   bool
	Label  string
	Toggle ui.Handler
}

func filterGroupDisclosure(props filterGroupDisclosureProps) ui.Node {
	return Button(css.Class("filter-group-disc"), Type("button"),
		Attr("data-testid", "filter-showall-"+string(props.Field)),
		Attr("aria-expanded", ariaBool(props.Open)), OnClick(props.Toggle), props.Label)
}

type txnInterpretRowProps struct {
	Preview string
	Apply   ui.Handler
}

func txnInterpretRow(props txnInterpretRowProps) ui.Node {
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Text13),
		Style(map[string]string{"padding": "0.4rem 0.1rem 0.1rem"}),
		Attr("role", "status"),
		Span(css.Class(tw.TextDim, tw.ShrinkO), uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Button(css.Class("btn", "btn-tool"), Type("button"), Attr("data-testid", "nl-interpret"),
			Attr("aria-label", uistate.T("transactions.nlInterpretAria", props.Preview)), OnClick(props.Apply),
			Text(uistate.T("transactions.nlInterpret", props.Preview))))
}

type txnCustomTextFilterProps struct {
	Label   string
	Value   string
	OnInput ui.Handler
}

func txnCustomTextFilter(props txnCustomTextFilterProps) ui.Node {
	return Input(css.Class("field"), Type("text"), Attr("aria-label", props.Label), Placeholder(props.Label),
		OnInput(props.OnInput), uiw.FieldValue(props.Value))
}

type txnBulkPickerPopupProps struct {
	ID          string
	SearchLabel string
	ActiveIndex int
	Query       string
	Options     []ui.Node
	OnInput     ui.Handler
	OnKeyDown   ui.Handler
}

func txnBulkPickerPopup(props txnBulkPickerPopupProps) ui.Node {
	return Div(css.Class("bulk-picker-popover"),
		Input(css.Class("field", "bulk-picker-search"), Type("search"),
			Attr("id", props.ID+"-search"), Attr("data-testid", props.ID+"-search"), Attr("role", "combobox"),
			Attr("aria-label", props.SearchLabel), Attr("aria-expanded", "true"),
			Attr("aria-controls", props.ID+"-listbox"), Attr("aria-activedescendant", fmt.Sprintf("%s-option-%d", props.ID, props.ActiveIndex)),
			Attr("autofocus", true), Placeholder(props.SearchLabel), uiw.FieldValue(props.Query), OnInput(props.OnInput), OnKeyDown(props.OnKeyDown)),
		Div(css.Class("bulk-picker-options"), Attr("id", props.ID+"-listbox"), Attr("role", "listbox"),
			If(len(props.Options) == 0, Span(css.Class("bulk-picker-empty"), uistate.T("r5b.noMatches"))), props.Options))
}

type txnBulkTagEditorProps struct {
	Value   string
	OnInput ui.Handler
	Add     ui.Handler
}

func txnBulkTagEditor(props txnBulkTagEditorProps) ui.Node {
	return Div(css.Class("bulk-tag-editor"),
		Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("r1c.tagInput")), Placeholder(uistate.T("r1c.tagInput")),
			OnInput(props.OnInput), uiw.FieldValue(props.Value)),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "bulk-apply-tag"), OnClick(props.Add), uistate.T("r1c.addTag")))
}

// toolbarIconBtn renders one transactions-toolbar action as a standard labeled
// button (.btn-tool): a slightly-grayed leading glyph followed by an always-visible
// text label, so the action reads at a glance instead of needing a hover to decode a
// bare icon. The label also serves as the aria-label. variant is "" (neutral),
// "primary" (accent — the Add action), or "danger" (delete).
func toolbarIconBtn(testID string, ic icon.Name, label string, onClick ui.Handler, variant string) ui.Node {
	return toolbarIconBtnOpen(testID, ic, label, onClick, variant, false)
}

func toolbarIconLink(testID string, ic icon.Name, label, href string) ui.Node {
	return A(css.Class("btn btn-tool"), Attr("data-testid", testID), Href(href), Attr("aria-label", label),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(label))
}

func txnBulkActionGroup(label string, children ...ui.Node) ui.Node {
	args := []any{css.Class("bulk-action-group", tw.Flex, tw.ItemsCenter, tw.Gap1),
		Span(css.Class("bulk-action-group-label", "muted"), label)}
	for _, child := range children {
		args = append(args, child)
	}
	return Div(args...)
}

// txnActiveFilterChips builds removable chips for filters that belong in the
// summary row. The period control owns date bounds, so those bounds never appear
// here or inflate the chip row.
func txnActiveFilterChips(active []txnfilter.ActiveFilter, label func(txnfilter.ActiveFilter) string) []uiw.Chip {
	values := txntoolbar.ActiveChips(active, label)
	result := make([]uiw.Chip, len(values))
	for i, value := range values {
		result[i] = uiw.Chip{Key: value.Key, Label: value.Label}
	}
	return result
}

// addTxnTag adds a tag once, comparing case-insensitively to avoid near-duplicate
// values created by capitalization differences.
func addTxnTag(tags []string, tag string) []string { return txntoolbar.AddTag(tags, tag) }

func txnHasTag(tags []string, tag string) bool { return txntoolbar.HasTag(tags, tag) }

// toolbarIconBtnState is toolbarIconBtn with an explicit disabled state and a
// tooltip. A toolbar verb that cannot run yet (C561's Categorize before a category
// is chosen) reads as unavailable AND says what would make it available, instead of
// looking live and mutating on click.
func toolbarIconBtnState(testID string, ic icon.Name, label string, onClick ui.Handler, variant string, disabled bool, title string) ui.Node {
	cls := "btn btn-tool"
	switch variant {
	case "primary":
		cls += " btn-primary"
	case "danger":
		cls += " bt-danger"
	}
	args := []any{
		css.Class(cls), Type("button"),
		Attr("aria-label", label), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(label),
	}
	if title != "" {
		args = append(args, Attr("title", title))
	}
	if disabled {
		args = append(args, Disabled(true), Attr("aria-disabled", "true"))
	}
	if testID != "" {
		args = append(args, Attr("data-testid", testID))
	}
	return Button(args...)
}

// bulkCatLabel is a category's qualified display path for a bulk-op result message
// ("Filed 12 under Housing > Mortgage"), or "" when the id no longer resolves.
func bulkCatLabel(app *appstate.App, id string) string {
	return txntoolbar.CategoryLabel(app.Categories(), id)
}

// toolbarIconBtnOpen is toolbarIconBtn with an explicit open flag: when true the button
// stays highlighted (the .is-open state) — used for the buttons that open a flip modal /
// panel so the trigger reads as "currently open" until it's dismissed.
func toolbarIconBtnOpen(testID string, ic icon.Name, label string, onClick ui.Handler, variant string, open bool) ui.Node {
	// Labeled toolbar buttons (the .btn-tool standard): a slightly-grayed left glyph + the
	// text label, so the action reads at a glance instead of needing a hover to decode a
	// bare glyph. variant tints it: primary = accent, danger = red.
	cls := "btn btn-tool"
	switch variant {
	case "primary":
		cls += " btn-primary"
	case "danger":
		cls += " bt-danger"
	}
	if open {
		cls += " is-open"
	}
	args := []any{
		css.Class(cls), Type("button"),
		Attr("aria-label", label), Attr("aria-expanded", boolStr(open)), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(label),
	}
	if testID != "" {
		args = append(args, Attr("data-testid", testID))
	}
	return Button(args...)
}

// txnToolbarProps carries the data the toolbar tile reads to build its filter
// option lists, chips, duplicate notice, and screen-reader summary.
type txnToolbarProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
	Shown []domain.Transaction
	// Derived optionally supplies the host's immutable, scope-checked derivation.
	Derived *txntoolbar.Snapshot
}

// txnToolbarWidget is the txn-toolbar tile: the search box, the collapsible filter
// fields, the active-filter chips, and the primary actions (add, clear, export CSV,
// and the import / review-duplicates sub-view toggles), plus a select-all control, a
// duplicate notice, and a screen-reader live summary of the filtered set. It writes
// the shared filter / view / selection atoms so the table and bulk tiles react in step.
// txnLargeThreshold is the absolute-amount floor (major units) the "Large" quick
// filter applies via AmountMin (TXC-3).
const txnLargeThreshold = "100"

// txnPresetProps configures one quick-filter preset chip.
type txnPresetProps struct {
	Label    string
	TestID   string
	Active   bool
	Count    int // matching-transaction count, shown like the toolbar's "Review N"
	OnToggle func()
}

// txnPresetChip is one quick-filter chip (its own component so its OnClick hook is
// never registered inside a loop). Clicking toggles the preset's filter; a trailing
// count communicates the payoff before the click.
func txnPresetChip(props txnPresetProps) ui.Node {
	on := ui.UseEvent(func() { props.OnToggle() })
	cls := "txn-preset"
	if props.Active {
		cls += " on"
	}
	// C568: the visible count is a bare number, which reads as an unqualified total.
	// The accessible name says what population it describes ("Large — 12 in the
	// current view"), so the figure is never mistaken for a household-wide count.
	return Button(css.Class(cls), Type("button"), Attr("data-testid", props.TestID),
		Attr("aria-pressed", ariaBool(props.Active)),
		Attr("aria-label", uistate.T("transactions.presetCountAria", props.Label, props.Count)),
		OnClick(on),
		Span(props.Label),
		Span(css.Class("txn-preset-count"), Attr("aria-hidden", "true"), strconv.Itoa(props.Count)))
}

func txnPresetRow(uncategorized, flagged, large int, uncatActive, flaggedActive, largeActive bool, onUncategorized, onFlagged, onLarge func()) ui.Node {
	return Div(css.Class("txn-presets"), Attr("data-testid", "txn-presets"),
		Span(css.Class("txn-presets-label"), uistate.T("transactions.presetsLabel")),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetUncategorized"), TestID: "txn-preset-uncat", Active: uncatActive, Count: uncategorized,
			OnToggle: onUncategorized,
		}),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetNeedsReview"), TestID: "txn-preset-review", Active: flaggedActive, Count: flagged,
			OnToggle: onFlagged,
		}),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetLarge"), TestID: "txn-preset-large", Active: largeActive, Count: large,
			OnToggle: onLarge,
		}),
		Span(css.Class("txn-presets-note"), Attr("data-testid", "txn-presets-note"), uistate.T("transactions.presetScopeNote")),
	)
}

type txnToolbarRenderProps struct {
	txnToolbarProps
	Own, Filter txnfilter.Criteria
	MemberLens  []string
	LensOn      bool
	Indexes     txnToolbarIndexes
	Summary     txnToolbarSummary
	Derived     txntoolbar.Derived
	SetFilter   func(func(*txnfilter.Criteria))
}

func txnToolbarWidget(props txnToolbarProps) ui.Node {
	defer txnPerfStart("toolbar")()
	app := props.App
	filterAtom := uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	own := filterAtom.Get()
	filter, lensOn := own.WithOwnerLens(memberLens)
	indexes := ui.UseMemo(func() txnToolbarIndexes {
		return buildTxnToolbarIndexes(app.Accounts(), app.Categories(), app.Members(), app.Transactions())
	}, app, app.Rev())
	ledger := ui.UseMemo(func() txnToolbarSummary {
		rows := app.Transactions()
		return txnToolbarSummary{review: reviewqueue.Count(rows), duplicates: dedupe.Count(dedupe.FindDuplicates(rows)), suspects: len(txnclassify.Suspects(rows, func(id string) string { return indexes.categoryNames[id] }))}
	}, app, app.Rev())
	scope := txntoolbar.SummaryScope(filter)
	month := dateutil.MonthStart(time.Now())
	derived := ui.UseMemo(func() txntoolbar.Derived {
		labels := txnfilter.Labels{Account: indexes.accountNames, Category: indexes.categoryNames, Payee: app.PayeeResolver().Resolve, BaseCurrency: props.Base, ToBase: func(m money.Money) (money.Money, error) { return props.Rates.Convert(m, props.Base) }}
		if props.Derived.Matches(app.Rev(), txnDerivedScope(app, filter)) {
			return props.Derived.Value
		}
		return deriveTxnScope(app, app.Transactions(), scope, labels, props.Rates, props.Base)
	}, app, app.Rev(), scope, props.Base, month, activeImportProfileWorkspaceID(), props.Derived)
	summary := ledger
	summary.presets = derived.Presets
	renderProps := txnToolbarRenderProps{txnToolbarProps: props, Own: own, Filter: filter, MemberLens: memberLens, LensOn: lensOn, Indexes: indexes, Summary: summary, Derived: derived, SetFilter: func(mut func(*txnfilter.Criteria)) { setTxFilterOn(filterAtom, mut) }}
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: txnResponsiveToolbar(ui.CreateElement(txnToolbarControls, renderProps), ui.CreateElement(txnUpcomingStrip, struct{}{}), ui.CreateElement(txnToolbarInterpret, renderProps), txnToolbarFoot(ui.CreateElement(txnToolbarCount, renderProps))),
	})
}

func txnToolbarChipLabel(props txnToolbarRenderProps) func(txnfilter.ActiveFilter) string {
	names := txntoolbar.Indexes{Accounts: props.Indexes.accountNames, Categories: props.Indexes.categoryNames, Members: props.Indexes.memberNames}
	customLabel := customFieldLabel(props.App.CustomFieldDefsFor("transaction"), props.Own.CustomKey)
	p := txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}
	return func(af txnfilter.ActiveFilter) string { return p.FilterChipLabel(af, names, props.Base, customLabel) }
}

func txnToolbarChips(props txnToolbarRenderProps) ([]uiw.Chip, []uiw.Chip) {
	fOwn, memberLens, lensOn := props.Own, props.MemberLens, props.LensOn
	memberName := props.Indexes.memberNames
	chipLabel := txnToolbarChipLabel(props)
	active := fOwn.PageFilters()
	var lensChips []uiw.Chip
	if lensOn {
		for _, id := range memberLens {
			lensChips = append(lensChips, uiw.Chip{
				Key:         lensChipKey + chipKeySep + id,
				Label:       uistate.T("transactions.lensChip", memberName[id]),
				Class:       "filter-chip-lens",
				RemoveLabel: uistate.T("transactions.lensChipRemove"),
			})
		}
	}
	if bd := uistate.BudgetDrillFor(fOwn); bd.BudgetID != "" {
		lensChips = append(lensChips, uiw.Chip{
			Key:         budgetDrillChipKey + chipKeySep + bd.BudgetID,
			Label:       uistate.T(budgetDrillChipKeyFor(bd), bd.Name),
			Class:       "filter-chip-lens",
			RemoveLabel: uistate.T("transactions.budgetDrillChipRemove"),
		})
	}
	chips := txnActiveFilterChips(active, chipLabel)

	return chips, lensChips
}

func txnToolbarControls(props txnToolbarRenderProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	searchPending := uistate.UseTxnSearchPending()
	setFilter := props.SetFilter
	debFilterD := func(key string, delay time.Duration, mut func(*uistate.TxFilter)) {
		debounce.Call("txn-filter:"+key, delay, func() { setFilter(mut) })
	}
	onFilterText := func(v string) {
		if !searchPending.Get() {
			searchPending.Set(true)
		}
		debFilterD("text", 150*time.Millisecond, func(x *uistate.TxFilter) { x.Text = v; searchPending.Set(false) })
	}
	clearSearchNow := func() {
		debounce.Flush("txn-filter:text")
		searchPending.Set(false)
		setFilter(func(x *uistate.TxFilter) { x.Text = "" })
	}
	chips, lensChips := txnToolbarChips(props)
	return uiw.FilterToolbar(uiw.FilterToolbarProps{
		Search: props.Filter.Text, SearchLabel: uistate.T("transactions.searchPlaceholder"), OnSearch: onFilterText, OnClearSearch: clearSearchNow, SearchPending: searchPending.Get(), ResultsSelector: ".data-table",
		FiltersLabel: uistate.T("transactions.filters"), FiltersTitle: uistate.T("transactions.filtersTitle"), FilterSheet: true,
		ActiveAriaLabel: func(n int) string {
			if n == 0 {
				return uistate.T("transactions.filters")
			}
			return uistate.T("transactions.filtersActiveAria", plural(n, "filter"))
		},
		FilterFields: ui.CreateElement(txnToolbarFilterPanel, props), Chips: chips, PreChips: lensChips,
		OnRemoveChip: func(key string) {
			field, value, _ := strings.Cut(key, chipKeySep)
			if field == lensChipKey {
				uistate.ClearMemberLens()
				return
			}
			if field == budgetDrillChipKey {
				uistate.ClearBudgetDrill()
				setFilter(func(x *uistate.TxFilter) { *x = txntoolbar.ClearBudgetScope(*x) })
				return
			}
			setFilter(func(x *uistate.TxFilter) { *x = x.RemoveValue(txnfilter.FilterField(field), value) })
		},
		OnClearAll: func() {
			cleared := filterAtom.Get().ClearPageFilters()
			filterAtom.Set(cleared)
			uistate.PersistTxFilter(cleared)
		},
		ClearAllLabel: clearAllLabel(len(props.Own.PageFilters())), RemoveLabel: uistate.T("transactions.removeFilter"),
		Actions: []ui.Node{
			Div(css.Class("txn-toolbar-context"), ui.CreateElement(txnDateScope, struct{}{}), ui.CreateElement(txnViewToggle, struct{}{})),
			Div(css.Class("txn-toolbar-commands"), ui.CreateElement(txnToolbarAdd, props), ui.CreateElement(txnToolbarReview, props), ui.CreateElement(txnToolbarMore, props)),
		},
	})
}

func txnToolbarAdd(_ txnToolbarRenderProps) ui.Node {
	transferOpenAtom := uistate.UseAcctTransferOpen()
	importPanelAtom := uistate.UseImportPanelOpen()
	onAdd := ui.UseEvent(Prevent(func() { uistate.SetQuickAdd(true) }))
	addExpense := func() { uistate.OpenQuickAddAs(uistate.QuickAddExpense) }
	addIncome := func() { uistate.OpenQuickAddAs(uistate.QuickAddIncome) }
	addTransfer := func() { transferOpenAtom.Set(true) }
	return Div(ClassStr("txn-toolbar-add"),
		Div(css.Class("btn-split"),
			toolbarIconBtn("txn-add-btn", icon.Plus, uistate.T("transactions.addTitle"), onAdd, "primary"),
			uiw.OverflowMenu(uiw.OverflowMenuProps{
				TriggerLabel:  uistate.T("transactions.addKindLabel"),
				TriggerIcon:   icon.ChevronDown,
				TriggerTestID: "txn-add-kind-btn",
				TriggerClass:  "btn btn-primary btn-split-caret",
				Items: []uiw.OverflowMenuItem{
					{Label: uistate.T("transactions.addExpense"), Icon: icon.ArrowDown, TestID: "txn-add-expense", OnSelect: addExpense},
					{Label: uistate.T("transactions.addIncome"), Icon: icon.ArrowUp, TestID: "txn-add-income", OnSelect: addIncome},
					{Label: uistate.T("transactions.addTransfer"), Icon: icon.Repeat, TestID: "txn-add-transfer", OnSelect: addTransfer},
					{Label: uistate.T("transactions.importBtn"), Icon: icon.Upload, TestID: "txn-add-import", Section: uistate.T("transactions.addFromFile"),
						OnSelect: func() { importPanelAtom.Set(true) }},
				},
			}),
		),
	)
}

func txnToolbarReview(props txnToolbarRenderProps) ui.Node {
	openReview := ui.UseEvent(Prevent(func() { uistate.OpenReviewInbox() }))
	summary := props.Summary
	return If(summary.review > 0, Button(css.Class("txn-review-callout txn-review-callout-attention"), Type("button"),
		Attr("data-testid", "txn-review-btn"), Attr("aria-label", uistate.T("r1c.reviewCallout", summary.review)),
		OnClick(openReview), Span(uistate.T("uxt4.review")),
		Span(css.Class("txn-review-count"), Attr("aria-hidden", "true"), strconv.Itoa(summary.review))))
}

func txnToolbarMore(props txnToolbarRenderProps) ui.Node {
	app, f, fOwn, summary := props.App, props.Filter, props.Own, props.Summary
	ruleCount := len(app.Rules())
	colsModalAtom := uistate.UseTxnColsModalOpen()
	smartCatAtom := uistate.UseTxnSmartCatOpen()
	importPanelAtom := uistate.UseImportPanelOpen()
	dupModalAtom := uistate.UseDuplicatesModalOpen()
	transferReviewAtom := uistate.UseTransferReviewOpen()
	viewAtom := uistate.UseTxnViewMode()
	registerAtom := uistate.UseTxnRegisterMode()
	selAtom := uistate.UseTxnSelection()
	nav := router.UseNavigate()
	calActive := viewAtom.Get() == uistate.TxnViewCalendar
	onRegister := func() { registerAtom.Set(!registerAtom.Get()) }
	selectAllFiltered := func() { selAtom.Set(txntoolbar.ToggleSelection(props.Shown, selAtom.Get())) }
	doExportCSV := func() { txnExportRows(app, props.Shown, "transactions.csv") }
	importBtnLabel := uistate.T("transactions.importBtn")
	dupBtnLabel := uistate.T("transactions.dupReviewBtn")
	if summary.duplicates > 0 {
		dupBtnLabel = uistate.T("transactions.dupReviewBadge", plural(summary.duplicates, "duplicate"))
	}
	suspectCount := summary.suspects
	_, singleAcct := f.SingleAccount()
	regLabel := uistate.T("transactions.registerView")
	if registerAtom.Get() {
		regLabel = "✓ " + regLabel
	}
	moreMenu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText:   uistate.T("transactions.moreActions"),
		TriggerTestID: "txn-more-btn",
		TriggerClass:  "btn btn-tool",
		ExtraContent: ui.CreateElement(TxnSavedViewsMenu, txnSavedViewsMenuProps{
			App: app, Filter: fOwn, Rates: props.Rates, Base: props.Base, Embedded: true,
		}),
		Items: []uiw.OverflowMenuItem{
			{Label: uistate.T("uxt4.review"), Icon: icon.CheckCircle, TestID: "txn-review-menu", OnSelect: func() { uistate.OpenReviewInbox() }, Hidden: summary.review == 0},
			{Label: uistate.T("uxt4.keyboardShortcuts"), Icon: icon.HelpCircle, TestID: "txn-keyboard-shortcuts-menu", OnSelect: func() { SetTxnKeyHelpOpen(true) }},
			{Label: uistate.T("transactions.rulesButton", ruleCount), Icon: icon.Workflow, TestID: "txn-rules-btn",
				OnSelect: func() { txnLeaveFor("/rules"); nav.Navigate(uistate.RoutePath("/rules")) }},
			{Label: regLabel, Icon: icon.List, TestID: "txn-register-btn", OnSelect: onRegister, Hidden: !(singleAcct && !calActive)},
			{Label: importBtnLabel, Icon: icon.Upload, TestID: "txn-import-btn", OnSelect: func() { importPanelAtom.Set(true) }},
			{Label: dupBtnLabel, Icon: icon.Copy, TestID: "txn-dupes-btn", OnSelect: func() { dupModalAtom.Set(true) }},
			{Label: transferReviewBtnLabel(suspectCount), Icon: icon.Repeat, TestID: "txn-transfer-review-btn",
				OnSelect: func() { transferReviewAtom.Set(true) }},
			{Label: uistate.T("smartcat.button"), Icon: icon.Sparkles, TestID: "txn-smartcat-btn", OnSelect: func() { smartCatAtom.Set(true) }},
			{Label: uistate.T("transactions.selectAllFiltered"), Icon: icon.CheckCircle, TestID: "txn-selectall-btn",
				OnSelect: selectAllFiltered, Hidden: len(props.Shown) == 0},
			{Label: uistate.T("transactions.exportCsv"), Icon: icon.ArrowDown, TestID: "txn-export-btn", OnSelect: doExportCSV},
			{Label: uistate.T("transactions.columns"), Icon: icon.List, TestID: "txn-columns-btn", OnSelect: func() { colsModalAtom.Set(true) }},
		},
	})
	return moreMenu
}

func txnExportRows(app *appstate.App, rows []domain.Transaction, filename string) {
	if len(rows) == 0 {
		uistate.PostNotice(uistate.T("transactions.noExport"), true)
		return
	}
	data, err := app.TransactionsCSV(rows)
	if err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	downloadBytes(filename, "text/csv", data)
}

func txnToolbarCustomFilter(props txnToolbarRenderProps) ui.Node {
	app, f, setFilter := props.App, props.Filter, props.SetFilter
	onFilterCustomValText := ui.UseEvent(func(v string) {
		debounce.Call("txn-filter:customval", 250*time.Millisecond, func() { setFilter(func(x *uistate.TxFilter) { x.CustomVal = v }) })
	})
	txnDefs := app.CustomFieldDefsFor("transaction")
	var customFilterNode ui.Node = Fragment()
	if len(txnDefs) > 0 {
		var selDef *customfields.Def
		for i := range txnDefs {
			if txnDefs[i].Key == f.CustomKey {
				selDef = &txnDefs[i]
			}
		}
		keyOpts := withAllOption(uistate.T("transactions.filterCustomNone"),
			uiw.OptionsFrom(txnDefs, func(d customfields.Def) string { return d.Key }, func(d customfields.Def) string { return d.Label }, ""))
		keySelect := uiw.SelectInput(uiw.SelectInputProps{
			Options: keyOpts, Selected: f.CustomKey, AriaLabel: uistate.T("transactions.filterCustomField"),
			OnChange: func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomKey, x.CustomVal = v, "" }) },
		})
		var valControl ui.Node = Fragment()
		if selDef != nil {
			setVal := func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = v }) }
			valAria := uistate.T("transactions.filterCustomValue")
			switch selDef.Type {
			case customfields.TypeSelect:
				opts := withAllOption(uistate.T("transactions.filterCustomAny"),
					uiw.OptionsFrom(selDef.Options, func(o string) string { return o }, func(o string) string { return o }, ""))
				valControl = uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: f.CustomVal, AriaLabel: valAria, OnChange: setVal})
			case customfields.TypeBool:
				opts := []uiw.SelectOption{
					{Value: "", Label: uistate.T("transactions.filterCustomAny")},
					{Value: "true", Label: uistate.T("common.yes")},
					{Value: "false", Label: uistate.T("common.no")},
				}
				valControl = uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: f.CustomVal, AriaLabel: valAria, OnChange: setVal})
			default:
				valControl = ui.CreateElement(txnCustomTextFilter, txnCustomTextFilterProps{Label: valAria, Value: f.CustomVal, OnInput: onFilterCustomValText})
			}
		}
		customFilterNode = withFieldLabel(uistate.T("transactions.filterCustomField"), Fragment(keySelect, valControl))
	}

	return customFilterNode
}

func txnToolbarFilterPanel(props txnToolbarRenderProps) ui.Node {
	f, fOwn, setFilter, summary := props.Filter, props.Own, props.SetFilter, props.Summary
	displayPrefs := uistate.UsePrefs().Get()
	indexes := props.Indexes
	accOpts, catOpts, memberOpts, sourceOpts, tagOpts, tagList := indexes.accountOptions, indexes.categoryOptions, indexes.memberOptions, indexes.sourceOptions, indexes.tagOptions, indexes.tags
	debFilter := func(key string, mut func(*uistate.TxFilter)) {
		debounce.Call("txn-filter:"+key, 250*time.Millisecond, func() { setFilter(mut) })
	}
	onFilterAmountMin := ui.UseEvent(func(v string) { debFilter("amtmin", func(x *uistate.TxFilter) { x.AmountMin = v }) })
	onFilterAmountMax := ui.UseEvent(func(v string) { debFilter("amtmax", func(x *uistate.TxFilter) { x.AmountMax = v }) })
	onFilterFrom := func(v string) {
		setFilter(func(x *uistate.TxFilter) { x.From, x.PeriodPreset = v, txnfilter.PeriodCustom })
	}
	onFilterTo := func(v string) {
		setFilter(func(x *uistate.TxFilter) { x.To, x.PeriodPreset = v, txnfilter.PeriodCustom })
	}
	onToggleFilter := func(field txnfilter.FilterField, value string) {
		setFilter(func(x *uistate.TxFilter) { *x = x.ToggleValue(field, value) })
	}
	flowOpts := []uiw.SelectOption{
		{Value: "", Label: uistate.T("transactions.flowAny")},
		{Value: "out", Label: uistate.T("transactions.flowOut")},
		{Value: "in", Label: uistate.T("transactions.flowIn")},
	}
	clearedOpts := []uiw.SelectOption{
		{Value: "", Label: uistate.T("transactions.clearedAny")},
		{Value: "no", Label: uistate.T("transactions.notCleared")},
		{Value: "yes", Label: uistate.T("transactions.cleared")},
	}
	uncatN, reviewN2, largeN := summary.presets.Uncategorized, summary.presets.NeedsReview, summary.presets.Large
	presetsRow := txnPresetRow(uncatN, reviewN2, largeN,
		f.Uncategorized, f.Tag == reviewqueue.ReviewTag, f.AmountMin == txnLargeThreshold,
		func() { setFilter(func(x *uistate.TxFilter) { x.Uncategorized = !x.Uncategorized }) },
		func() {
			setFilter(func(x *uistate.TxFilter) {
				if x.Tag == reviewqueue.ReviewTag {
					x.Tag = ""
				} else {
					x.Tag = reviewqueue.ReviewTag
				}
			})
		},
		func() {
			setFilter(func(x *uistate.TxFilter) {
				if x.AmountMin == txnLargeThreshold {
					x.AmountMin = ""
				} else {
					x.AmountMin = txnLargeThreshold
				}
			})
		},
	)

	filtersBody := Div(css.Class("filter-panel"),
		presetsRow,
		Div(css.Class("filter-groups"),
			filterMultiGroup(uistate.T("transactions.filterAccount"), txnfilter.FieldAccount, fOwn.SelectedValues(txnfilter.FieldAccount), accOpts, onToggleFilter),
			filterMultiGroup(uistate.T("transactions.filterCategory"), txnfilter.FieldCategory, fOwn.SelectedValues(txnfilter.FieldCategory), catOpts, onToggleFilter),
			filterMultiGroup(uistate.T("transactions.member"), txnfilter.FieldMember, fOwn.SelectedValues(txnfilter.FieldMember), memberOpts, onToggleFilter),
			filterMultiGroup(uistate.T("transactions.filterSource"), txnfilter.FieldSource, fOwn.SelectedValues(txnfilter.FieldSource), sourceOpts, onToggleFilter),
			If(len(tagList) > 0, filterMultiGroup(uistate.T("transactions.filterTag"), txnfilter.FieldTag, fOwn.SelectedValues(txnfilter.FieldTag), tagOpts, onToggleFilter)),
		),
		Div(css.Class("filter-ranges"),
			dateField(uistate.T("transactions.fromDate"), f.From, displayPrefs.DateStyle, onFilterFrom),
			dateField(uistate.T("transactions.toDate"), f.To, displayPrefs.DateStyle, onFilterTo),
			amountField(uistate.T("transactions.filterAmountMin"), uistate.T("transactions.filterAmountMinPh"), f.AmountMin, onFilterAmountMin),
			amountField(uistate.T("transactions.filterAmountMax"), uistate.T("transactions.filterAmountMaxPh"), f.AmountMax, onFilterAmountMax),
			If(txnFilterRangeError(fOwn, props.Base) != "", P(css.Class("field-error"), Attr("role", "alert"), txnFilterRangeError(fOwn, props.Base))),
			filterSelect(uistate.T("transactions.clearedStatus"), f.Cleared, clearedOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Cleared = v }) }),
			filterSelect(uistate.T("transactions.flowLabel"), f.Flow, flowOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Flow = v }) }),
		),
		ui.CreateElement(txnToolbarCustomFilter, props),
	)

	return filtersBody
}

func txnToolbarInterpret(props txnToolbarRenderProps) ui.Node {
	app, f, setFilter := props.App, props.Filter, props.SetFilter
	filterAtom := uistate.UseTxFilter()
	chipLabel := txnToolbarChipLabel(props)
	nlContext := func() nlfilter.Context {
		return txntoolbar.NaturalLanguageContext(app.Categories(), app.Transactions(), props.Indexes.tags, app.PayeeResolver().Resolve, time.Now(), uistate.CurrentPrefs().WeekStartWeekday())
	}
	applyNL := ui.UseEvent(Prevent(func() {
		parsed, ok := nlfilter.Parse(strings.TrimSpace(filterAtom.Get().Text), nlContext())
		if !ok {
			return
		}
		setFilter(func(x *uistate.TxFilter) { *x = txntoolbar.ApplyNaturalLanguage(*x, parsed) })
	}))
	var interpretRow ui.Node = Fragment()
	if q := strings.TrimSpace(f.Text); q != "" && uistate.LoadSmartSettings().IsEnabled("SMART-T3F") {
		if parsed, nlOK := nlfilter.Parse(q, nlContext()); nlOK {
			var previews []string
			for _, af := range parsed.AllCriteria() {
				previews = append(previews, chipLabel(af))
			}
			preview := strings.Join(previews, " · ")
			interpretRow = ui.CreateElement(txnInterpretRow, txnInterpretRowProps{Preview: preview, Apply: applyNL})
		}
	}

	return interpretRow
}

func txnToolbarCount(props txnToolbarRenderProps) ui.Node {
	viewMode := uistate.UseTxnViewMode()
	lensName := txntoolbar.LensName(props.MemberLens, props.LensOn, props.Indexes.memberNames, func(n int) string { return plural(n, "member") })
	view := props.Derived.View
	return ui.CreateElement(txnCountLine, txnCountLineProps{
		Shown: len(props.Shown), Total: len(props.App.Transactions()), Range: txnPageRange(props.Filter.Page, props.Filter.PageSize, len(props.Shown)), Revision: props.App.Rev(),
		CategoryTotals: txnCategoriesForView(view, props.Indexes.categoryNames), Net: money.New(view.Net, props.Base), Lens: lensName,
		Unconverted: view.Unconverted, ConversionIssues: props.Derived.Issues, Excluded: view.Excluded,
		Moved: money.New(view.Transferred, props.Base), MovedRows: view.MovedRows, UnpairedRows: view.UnpairedRows, Spent: money.New(view.Spent, props.Base), Income: money.New(view.Income, props.Base), CashflowProvided: true, FactsProvidedByTable: viewMode.Get() != uistate.TxnViewCalendar,
	})
}

func txnExpandedSelection(app *appstate.App, selected map[string]bool) map[string]bool {
	return txnV5RowExpandedSelection(selected, app.ExpandTransferPairs(txntoolbar.SelectionIDs(selected)))
}

func txnResponsiveToolbar(toolbar ui.Node, content ...any) ui.Node {
	children := []any{css.Class("txn-responsive-toolbar"), toolbar}
	children = append(children, content...)
	return Div(children...)
}

// txnUserColumnVisible limits the member column to an Everyone view with multiple
// household members.
func txnToolbarFoot(countLine ui.Node) ui.Node {
	return Div(css.Class("txn-toolbar-foot"), countLine)
}

func txnUserColumnVisible(enabled bool, memberScope string, memberCount int) bool {
	return txntoolbar.UserColumnVisible(enabled, memberScope, memberCount)
}

// txnColsForScope returns a copy of column visibility with User constrained to
// Everyone views that can actually distinguish more than one member.
func txnColsForScope(cols uistate.TxnCols, memberScope string, memberCount int) uistate.TxnCols {
	cols.User = txnUserColumnVisible(cols.User, memberScope, memberCount)
	return cols
}

// txnPageRange formats the non-interactive range summary shown beside the count
// line. Pagination controls themselves remain at the bottom of the table.
func txnPageRange(page, pageSize, total int) string {
	return txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney, Date: uistate.LoadPrefs().FormatDate}.PageRange(page, pageSize, total)
}

func txnCountNumber(n int) string { return txntoolbar.CountNumber(n) }

// txnMemberInitials keeps the optional member column compact while the cell's
// accessible name retains the full name.
func txnMemberInitials(name string) string { return txntoolbar.MemberInitials(name) }

// TxnColumnsBody is the body of the "show / hide columns" flip modal: a checkbox
// per optional ledger column, writing the persisted visibility atom so the table
// re-renders in step. Date and Description are the row's identity and always show,
// so they aren't offered. It is mounted at the SHELL ROOT (by the app package's
// TxnColumnsHost) rather than inside the toolbar tile, because a tile's CSS
// transform would otherwise mis-position and clip the fixed modal (C-modals).
func TxnColumnsBody(_ struct{}) ui.Node {
	colsAtom := uistate.UseTxnCols()
	cols := colsAtom.Get()
	apply := func(c uistate.TxnCols) { colsAtom.Set(c); uistate.PersistTxnCols(c); uistate.BumpDataRevision() }

	amount := ui.UseEvent(func() { c := colsAtom.Get(); c.Amount = !c.Amount; apply(c) })
	account := ui.UseEvent(func() { c := colsAtom.Get(); c.Account = !c.Account; apply(c) })
	category := ui.UseEvent(func() { c := colsAtom.Get(); c.Category = !c.Category; apply(c) })
	source := ui.UseEvent(func() { c := colsAtom.Get(); c.Source = !c.Source; apply(c) })
	user := ui.UseEvent(func() { c := colsAtom.Get(); c.User = !c.User; apply(c) })

	row := func(label, hint, testID string, on bool, onClick ui.Handler) ui.Node {
		return Label(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"padding": "0.35rem 0", "cursor": "pointer"}),
			Input(Type("checkbox"), Attr("data-testid", testID), CheckedIf(on), OnClick(onClick)),
			Span(label),
			If(hint != "", Span(css.Class("muted", tw.Text12), Attr("title", hint), hint)),
		)
	}
	return Div(css.Class(tw.FlexCol),
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("transactions.columnsHint")),
		row(uistate.T("transactions.colAmount"), "", "col-toggle-amount", cols.Amount, amount),
		row(uistate.T("transactions.colAccount"), "", "col-toggle-account", cols.Account, account),
		row(uistate.T("transactions.colCategory"), "", "col-toggle-category", cols.Category, category),
		row(uistate.T("transactions.colSource"), "", "col-toggle-source", cols.Source, source),
		row(uistate.T("transactions.colUser"), "", "col-toggle-user", cols.User, user),
	)
}

// txnBulkBarProps carries the app the bulk tile mutates.
type txnBulkBarProps struct {
	App *appstate.App
}

type txnBulkPickerOption = txntoolbar.PickerOption

type txnBulkPickerProps struct {
	ID          string
	TestID      string
	Label       string
	Placeholder string
	SearchLabel string
	Options     []txnBulkPickerOption
	OnChoose    func(string)
	OnCreate    func(string)
}

type txnBulkPickerOptionProps struct {
	PickerID string
	Index    int
	Option   txnBulkPickerOption
	Selected bool
	OnChoose func(txnBulkPickerOption)
}

func txnBulkPickerOptionRow(props txnBulkPickerOptionProps) ui.Node {
	onChoose := ui.UseEvent(Prevent(func() { props.OnChoose(props.Option) }))
	return Button(css.Class("bulk-picker-option"), Type("button"),
		Attr("role", "option"), Attr("id", fmt.Sprintf("%s-option-%d", props.PickerID, props.Index)),
		Attr("aria-selected", ariaBool(props.Selected)), Attr("data-testid", props.PickerID+"-option"),
		OnClick(onChoose), props.Option.Label)
}

func txnBulkCategoryPicker(initialQuery string, kind domain.CategoryKind, onPick func(string), onClose func()) ui.Node {
	return ui.CreateElement(CategoryPicker, CategoryPickerProps{
		Kind:         kind,
		InitialQuery: initialQuery,
		OnPick:       onPick,
		OnClose:      onClose,
	})
}

// txnBulkPicker commits a bulk assignment only after an explicit Enter or click.
func txnBulkPicker(props txnBulkPickerProps) ui.Node {
	open := ui.UseState(false)
	query := ui.UseState("")
	active := ui.UseState(0)

	filtered := txntoolbar.PickerOptions(props.Options, query.Get(), props.OnCreate != nil, func(name string) string { return uistate.T("r10t.createCategory", name) })
	activeIndex := txntoolbar.PickerIndex(active.Get(), len(filtered), 0)
	close := func() {
		open.Set(false)
		query.Set("")
		active.Set(0)
	}
	ui.UseEffect(func() func() {
		if open.Get() {
			focusByID(props.ID + "-search")
		}
		return nil
	}, open.Get())
	choose := func(option txnBulkPickerOption) {
		if option.Create {
			props.OnCreate(strings.TrimSpace(query.Get()))
		} else {
			props.OnChoose(option.Value)
		}
		close()
	}
	onToggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	onTriggerKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		if e.GetKey() == "ArrowDown" {
			e.PreventDefault()
			open.Set(true)
		}
	})
	onQuery := ui.UseEvent(func(e ui.Event) { query.Set(e.GetValue()); active.Set(0) })
	onKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		switch e.GetKey() {
		case "ArrowDown":
			e.PreventDefault()
			if len(filtered) > 0 {
				active.Set(txntoolbar.PickerIndex(activeIndex, len(filtered), 1))
			}
		case "ArrowUp":
			e.PreventDefault()
			if len(filtered) > 0 {
				active.Set(txntoolbar.PickerIndex(activeIndex, len(filtered), -1))
			}
		case "Enter":
			e.PreventDefault()
			if len(filtered) > 0 {
				choose(filtered[activeIndex])
			}
		case "Escape":
			e.PreventDefault()
			e.StopPropagation()
			close()
			focusByID(props.ID)
		}
	})
	var popup ui.Node = Fragment()
	if open.Get() {
		options := make([]ui.Node, 0, len(filtered))
		for i, option := range filtered {
			options = append(options, ui.CreateElement(txnBulkPickerOptionRow, txnBulkPickerOptionProps{
				PickerID: props.ID, Index: i, Option: option, Selected: i == activeIndex, OnChoose: choose,
			}))
		}
		popup = ui.CreateElement(txnBulkPickerPopup, txnBulkPickerPopupProps{ID: props.ID, SearchLabel: props.SearchLabel,
			ActiveIndex: activeIndex, Query: query.Get(), Options: options, OnInput: onQuery, OnKeyDown: onKey})
	}
	testID := props.TestID
	if testID == "" {
		testID = props.ID
	}
	return Div(css.Class("bulk-picker"),
		Button(css.Class("field", "bulk-picker-trigger"), Type("button"), Attr("id", props.ID), Attr("data-testid", testID),
			Attr("aria-label", props.Label), Attr("aria-haspopup", "listbox"), Attr("aria-expanded", ariaBool(open.Get())),
			OnClick(onToggle), OnKeyDown(onTriggerKey), props.Placeholder),
		popup,
	)
}

// txnBulkBarWidget is the txn-bulkbar tile, shown by the host when a selection
// exists. It recategorizes, marks cleared/uncleared, exports, or deletes the
// selected transactions, captures a before-snapshot into the shared undo atom, and
// clears the selection. Each op bumps the data revision so the surface re-renders.

type txnBulkController struct {
	App         *appstate.App
	Selection   func() map[string]bool
	Clear       func()
	Saving      func() bool
	SetSaving   func(bool)
	SetCategory func(string)
	FinishTag   func()
}

func txnBulkApply(c txnBulkController, label func(int) string, writes []domain.Transaction, done func()) {
	if len(writes) == 0 {
		return
	}
	_ = CommitUndoableCounted(c.App, label, txntoolbar.IDs(writes), func() error { return c.App.PutTransactionsAtomic(writes) }, func(err error) {
		if err != nil {
			uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
			return
		}
		c.Clear()
		if done != nil {
			done()
		}
	})
}

func txnBulkSimple(c txnBulkController, edit txntoolbar.Edit, key string) {
	selected := txnExpandedSelection(c.App, c.Selection())
	writes := txntoolbar.Draft(c.App.Transactions(), selected, edit)
	txnBulkApply(c, func(n int) string { return uistate.T(key, n) }, writes, nil)
}

func txnBulkCategory(c txnBulkController, cid string) {
	if c.Saving() {
		return
	}
	selected := txnExpandedSelection(c.App, c.Selection())
	c.SetSaving(true)
	uistate.SaveCheckpointThen(uistate.T("ckpt.beforeBulkRecat", plural(len(selected), "transaction")), func(_ string, err error) {
		c.SetSaving(false)
		if err != nil {
			uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
			return
		}
		writes := txntoolbar.Draft(c.App.Transactions(), selected, txntoolbar.Edit{Kind: txntoolbar.Category, Text: cid})
		label := func(n int) string {
			if cid != "" {
				return uistate.T("transactions.bulkCatApplied", n, bulkCatLabel(c.App, cid))
			}
			return uistate.T("transactions.bulkOpClearedCat", n)
		}
		txnBulkApply(c, label, writes, func() { c.SetCategory("") })
	})
}

func txnBulkDelete(c txnBulkController) {
	if c.Saving() {
		return
	}
	sel := c.Selection()
	count := len(sel)
	uistate.ConfirmModalLabeled(uistate.TN("uxt4.deleteOne", "uxt4.deleteMany", count), uistate.T("uxt4.delete"), true, func(ok bool) {
		if !ok {
			return
		}
		c.SetSaving(true)
		uistate.SaveCheckpointThen(uistate.T("ckpt.beforeBulkDelete", plural(count, "transaction")), func(_ string, err error) {
			c.SetSaving(false)
			if err != nil {
				uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
				return
			}
			doomed := c.App.ExpandTransferPairs(txntoolbar.SelectionIDs(sel))
			_ = CommitUndoableCounted(c.App, func(n int) string { return uistate.T("transactions.bulkOpDeleted", n) }, doomed, func() error { _, err := c.App.DeleteTransactionsBulkExpanded(doomed); return err }, func(err error) {
				if err != nil {
					uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
					return
				}
				c.Clear()
			})
		})
	})
}

func txnBulkGroup(c txnBulkController) {
	ids := txntoolbar.IDs(txntoolbar.SelectedRows(c.App.Transactions(), c.Selection(), true))
	if len(ids) < 2 {
		uistate.PostNotice(uistate.T("txnlinks.groupNeedTwo"), true)
		return
	}
	_ = CommitUndoable(c.App, uistate.T("txnlinks.grouped", len(ids)), ids, func() error { return c.App.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: ids}) }, func(err error) {
		if err != nil {
			uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
			return
		}
		c.Clear()
	})
}

func txnBulkExport(c txnBulkController) {
	selected := c.Selection()
	if len(selected) == 0 {
		return
	}
	txnExportRows(c.App, txntoolbar.SelectedRows(c.App.Transactions(), selected, false), "transactions-selected.csv")
}

func txnBulkTag(c txnBulkController, raw string) {
	tag := strings.TrimSpace(raw)
	if tag == "" {
		return
	}
	writes := txntoolbar.Draft(c.App.Transactions(), txnExpandedSelection(c.App, c.Selection()), txntoolbar.Edit{Kind: txntoolbar.Tag, Text: tag})
	txnBulkApply(c, func(n int) string { return uistate.T("r1c.tagged", n, tag) }, writes, c.FinishTag)
}

func useTxnBulkEscape(selectionActive bool, clearSel func()) {
	ui.UseEffect(func() func() {
		if !selectionActive {
			return nil
		}
		doc := js.Global().Get("document")
		if !doc.Truthy() || doc.Get("addEventListener").Type() != js.TypeFunction || doc.Get("removeEventListener").Type() != js.TypeFunction {
			return nil
		}
		keyCB := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Get("key").String() == "Escape" {
				clearSel()
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", keyCB)
		return func() {
			doc.Call("removeEventListener", "keydown", keyCB)
			keyCB.Release()
		}
	}, selectionActive)
}

func txnBulkBarWidget(props txnBulkBarProps) ui.Node {
	app := props.App
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	bulkCatAtom := uistate.UseTxnBulkCat()
	checkpointSaving := ui.UseState(false)
	tagOpen := ui.UseState(false)
	tagValue := ui.UseState("")
	categoryPickerOpen := ui.UseState(false)
	categoryPickerQuery := ui.UseState("")
	categoryPickerKind := ui.UseState(domain.KindExpense)
	clearSel := func() { selAtom.Set(map[string]bool{}); anchorAtom.Set("") }
	useTxnBulkEscape(len(selAtom.Get()) > 0, clearSel)
	c := txnBulkController{App: app, Selection: selAtom.Get, Clear: clearSel, Saving: checkpointSaving.Get, SetSaving: checkpointSaving.Set, SetCategory: bulkCatAtom.Set, FinishTag: func() { tagValue.Set(""); tagOpen.Set(false) }}
	bulkSetCleared := func(value bool) {
		key := "transactions.bulkOpCleared"
		if !value {
			key = "transactions.bulkOpUncleared"
		}
		txnBulkSimple(c, txntoolbar.Edit{Kind: txntoolbar.Clear, Value: value, Now: time.Now()}, key)
	}
	bulkMarkReviewedAction := func() { txnBulkSimple(c, txntoolbar.Edit{Kind: txntoolbar.Review}, "r1c.markedReviewed") }
	bulkSetExclude := func(value bool) {
		key := "transactions.bulkOpExcluded"
		if !value {
			key = "transactions.bulkOpIncluded"
		}
		txnBulkSimple(c, txntoolbar.Edit{Kind: txntoolbar.Exclude, Value: value}, key)
	}
	applyBulkCategory := func(cid string) { txnBulkCategory(c, cid) }
	assignBulkOwners := func(owners []string) {
		txnBulkSimple(c, txntoolbar.Edit{Kind: txntoolbar.Owners, Owners: owners}, "transactions.bulkOpAssigned")
	}
	bulkMarkReviewed := ui.UseEvent(Prevent(bulkMarkReviewedAction))
	addTag := ui.UseEvent(Prevent(func() { txnBulkTag(c, tagValue.Get()) }))
	onTagInput := ui.UseEvent(func(e ui.Event) { tagValue.Set(e.GetValue()) })
	toggleTagInputAction := func() { tagOpen.Set(!tagOpen.Get()) }
	bulkGroupOrder := func() { txnBulkGroup(c) }
	exportSelected := func() { txnBulkExport(c) }
	bulkDeleteAction := func() { txnBulkDelete(c) }
	bulkDelete := ui.UseEvent(Prevent(bulkDeleteAction))
	clearSelection := ui.UseEvent(Prevent(clearSel))
	n := len(selAtom.Get())
	onBulkCat := func(picked string) {
		bulkCatAtom.Set(picked)
		if picked != "" {
			applyBulkCategory(picked)
		}
	}
	onCreateBulkCategory := func(name string) {
		categoryPickerQuery.Set(name)
		categoryPickerKind.Set(txntoolbar.CategoryKind(app.Transactions(), selAtom.Get()))
		categoryPickerOpen.Set(true)
	}
	catOptions := []txnBulkPickerOption{{Value: "", Label: uistate.T("r1c.category")}}
	for _, o := range CategoryPickOptions(app.Categories()) {
		catOptions = append(catOptions, txnBulkPickerOption{Value: o.Value, Label: o.Label})
	}
	ownerOptions := txnBulkOwnerPickerOptions(app.Members())
	moreActions := uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerLabel: uistate.T("r1c.moreActions"), TriggerTestID: "bulk-more-actions",
		TriggerClass: "btn btn-tool bulk-more-trigger", TriggerText: "",
		Items: []uiw.OverflowMenuItem{
			{Label: uistate.T("r1c.addTagEllipsis"), Icon: icon.Tag, TestID: "bulk-add-tag-menu", OnSelect: toggleTagInputAction},
			{Label: uistate.T("txp3.bulkMarkCleared"), Icon: icon.CheckCircle, TestID: "bulk-mark-cleared", OnSelect: func() { bulkSetCleared(true) }},
			{Label: uistate.T("txp3.bulkMarkUncleared"), Icon: icon.Ban, TestID: "bulk-mark-uncleared", OnSelect: func() { bulkSetCleared(false) }},
			{Label: uistate.T("txp3.bulkClearCategory"), Icon: icon.Ban, TestID: "bulk-clear-category", OnSelect: func() {
				uistate.ConfirmModalLabeled(uistate.T("transactions.bulkClearCatConfirm", len(selAtom.Get())), uistate.T("transactions.bulkClearCat"), true, func(ok bool) {
					if ok {
						applyBulkCategory("")
					}
				})
			}},
			{Label: uistate.T("txp3.bulkExcludeReports"), Icon: icon.Ban, TestID: "bulk-exclude", OnSelect: func() { bulkSetExclude(true) }},
			{Label: uistate.T("txp3.bulkIncludeReports"), Icon: icon.Check, TestID: "bulk-include", OnSelect: func() { bulkSetExclude(false) }},
			{Label: uistate.T("txp3.bulkGroupPurchase"), Icon: icon.Box, TestID: "bulk-group-order", OnSelect: func() { bulkGroupOrder() }},
			{Label: uistate.T("txp3.bulkExportSelected"), Icon: icon.ArrowDown, TestID: "bulk-export-selected", OnSelect: func() { exportSelected() }},
			{Label: uistate.T("r1c.markReviewed"), Icon: icon.CheckCircle, TestID: "bulk-mark-reviewed-menu", OnSelect: bulkMarkReviewedAction},
			{Label: uistate.T("txp3.bulkDeleteSelected"), Icon: icon.Trash, TestID: "bulk-delete-menu", Danger: true, OnSelect: bulkDeleteAction},
		},
	})
	tagEditor := Fragment()
	if tagOpen.Get() {
		tagEditor = ui.CreateElement(txnBulkTagEditor, txnBulkTagEditorProps{Value: tagValue.Get(), OnInput: onTagInput, Add: addTag})
	}
	return ui.CreateElement(txnBulkBarContent, txnBulkBarContentProps{Count: n, Saving: checkpointSaving.Get(), Categories: catOptions, Owners: ownerOptions, OnCategory: onBulkCat, OnCreate: onCreateBulkCategory, OnOwners: assignBulkOwners, Reviewed: bulkMarkReviewed, Delete: bulkDelete, Clear: clearSelection, TagEditor: tagEditor, More: moreActions, PickerOpen: categoryPickerOpen.Get(), PickerQuery: categoryPickerQuery.Get(), PickerKind: categoryPickerKind.Get(), ClosePicker: func() { categoryPickerOpen.Set(false) }})
}

type txnBulkBarContentProps struct {
	Count                   int
	Saving                  bool
	Categories, Owners      []txnBulkPickerOption
	OnCategory, OnCreate    func(string)
	OnOwners                func([]string)
	Reviewed, Delete, Clear ui.Handler
	TagEditor, More         ui.Node
	PickerOpen              bool
	PickerQuery             string
	PickerKind              domain.CategoryKind
	ClosePicker             func()
}

func txnBulkBarContent(props txnBulkBarContentProps) ui.Node {
	n, catOptions, ownerOptions := props.Count, props.Categories, props.Owners
	onBulkCat, onCreateBulkCategory, assignBulkOwners := props.OnCategory, props.OnCreate, props.OnOwners
	bulkMarkReviewed, bulkDelete, clearSelection := props.Reviewed, props.Delete, props.Clear
	tagEditor, moreActions := props.TagEditor, props.More
	bodyArgs := append([]any{css.Class("bulk-bar bulk-bar-refined txn-bulkbar-overlay")}, txnBulkToolbarAttrs(n)...)
	bodyArgs = append(bodyArgs, Style(map[string]string{
		"box-sizing": "border-box", "display": "flex", "align-items": "center", "gap": "0.5rem",
		"min-height": "48px", "padding": "0 1rem", "flex-wrap": "nowrap",
		"white-space": "nowrap",
		"background":  "var(--bg-elev)", "border": "1px solid var(--border)",
		"border-radius": "var(--radius-lg)",
	}),
		Span(css.Class("bulk-count", tw.ShrinkO), Attr("aria-label", uistate.T("transactions.selected", plural(n, "transaction"))), uistate.T("transactions.bulkSelectedShort", n)),
		ui.CreateElement(txnBulkPicker, txnBulkPickerProps{ID: "bulk-category-picker", Label: uistate.T("transactions.categoryToApply"), Placeholder: uistate.T("r1c.category"), SearchLabel: uistate.T("r5b.searchCategories"), Options: catOptions, OnChoose: onBulkCat, OnCreate: onCreateBulkCategory}),
		ui.CreateElement(txnBulkOwnerPicker, txnBulkOwnerPickerProps{
			ID: "bulk-member-picker", TestID: "bulk-owner-picker", Label: uistate.T("bulk2.owners"),
			Options: ownerOptions, OnApply: assignBulkOwners,
		}),
		Button(css.Class("btn btn-tool", "txn-bulk-primary", "txn-bulk-inline-reviewed"), Type("button"), Attr("data-testid", "bulk-mark-reviewed"), Attr("aria-label", uistate.T("r1c.markReviewed")), OnClick(bulkMarkReviewed), uiw.Icon(icon.CheckCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("r1c.markReviewed"))),
		tagEditor,
		moreActions,
		buttonWithDisabled(props.Saving, []any{
			css.Class("btn btn-tool bt-danger", "txn-bulk-primary", "txn-bulk-inline-delete"), Type("button"), Attr("data-testid", "bulk-delete"),
			Attr("aria-label", uistate.T("txp3.bulkDeleteSelected")),
			Title(uistate.T("txp3.bulkDeleteSelected")),
			OnClick(bulkDelete),
		}, IfElse(props.Saving, Text(uistate.T("enc34.saving")), Fragment(uiw.Icon(icon.Trash, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("txp3.bulkDeleteSelected"))))),
		Button(css.Class("btn btn-tool", "bulk-clear-selection"), Type("button"), Attr("data-testid", "bulk-clear-selection"), Attr("aria-label", uistate.T("r1c.clearSelection")), Title(uistate.T("r1c.clearSelection")), OnClick(clearSelection), Text("×")),
		If(props.PickerOpen, txnBulkCategoryPicker(props.PickerQuery, props.PickerKind,
			func(categoryID string) { props.ClosePicker(); onBulkCat(categoryID) },
			func() { props.ClosePicker() },
		)),
	)
	body := Div(bodyArgs...)
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-bulkbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Style: txnBulkBarWidgetStyle(),
		Body:  body,
	})
}

func txnBulkToolbarAttrs(selectedCount int) []any {
	return []any{
		Attr("role", "toolbar"),
		Attr("aria-label", uistate.T("transactions.bulkToolbarLabel", selectedCount)),
	}
}

const txnBulkUnassignedOwner = "__bulk_unassigned__"

func txnBulkOwnerPickerOptions(members []domain.Member) []txnBulkPickerOption {
	return txntoolbar.OwnerOptions(members, uistate.T("bulk2.unassigned"))
}

type txnBulkOwnerPickerProps struct {
	ID      string
	TestID  string
	Label   string
	Options []txnBulkPickerOption
	OnApply func([]string)
}

func txnBulkOwnerPicker(props txnBulkOwnerPickerProps) ui.Node {
	open := ui.UseState(false)
	selected := ui.UseState([]string(nil))
	close := func() {
		if !open.Get() {
			return
		}
		commitTxnBulkOwnerPick(selected.Get(), props.OnApply)
		open.Set(false)
	}
	onToggle := ui.UseEvent(Prevent(func() {
		if !open.Get() {
			selected.Set(nil)
			open.Set(true)
		}
	}))
	rows := MapKeyed(props.Options,
		func(option txnBulkPickerOption) any { return option.Value },
		func(option txnBulkPickerOption) ui.Node {
			return ui.CreateElement(txnBulkOwnerOption, txnBulkOwnerOptionProps{
				ID: props.ID + "-option-" + option.Value, Option: option,
				Selected: containsStr(selected.Get(), option.Value),
				OnToggle: func(value string) { selected.Set(txnBulkOwnerChoice(selected.Get(), value)) },
			})
		},
	)
	panel := Div(css.Class("cf-multipicker-sheet", tw.WFull, tw.MaxW85, tw.RoundedXl, tw.BgBase, tw.P3, tw.ShadowLg),
		Div(Attr("id", props.ID+"-title"), props.Label),
		Div(css.Class("cf-multipicker-list", tw.MaxH55vh, tw.OverflowYAuto, tw.Flex, tw.FlexWrap, tw.Gap2),
			Attr("id", props.ID+"-listbox"), Attr("role", "listbox"),
			Attr("aria-label", props.Label), Attr("aria-multiselectable", "true"), rows),
	)
	trigger := Button(css.Class("field", "bulk-picker-trigger"), Type("button"),
		Attr("id", props.ID), Attr("data-testid", props.TestID), Attr("aria-label", props.Label),
		Attr("aria-haspopup", "listbox"), Attr("aria-expanded", ariaBool(open.Get())),
		Attr("aria-controls", props.ID+"-listbox"), OnClick(onToggle), props.Label)
	overlay := ui.AccessibleOverlay(ui.AccessibleOverlayProps{
		Open: open.Get(), Target: ui.PortalTarget{Selector: "body"}, SurfaceID: props.ID + "-surface",
		Kind: ui.OverlayKindSheet, Role: "dialog", LabelledBy: props.ID + "-title", Modal: true,
		TrapFocus: true, RestoreFocus: true, CloseOnEscape: true, CloseOnOutsideClick: true,
		InitialFocusSelector: "#" + props.ID + "-option-0", Child: panel, OnDismiss: close,
	})
	return Fragment(trigger, overlay)
}

type txnBulkOwnerOptionProps struct {
	ID       string
	Option   txnBulkPickerOption
	Selected bool
	OnToggle func(string)
}

func txnBulkOwnerOption(props txnBulkOwnerOptionProps) ui.Node {
	onClick := ui.UseEvent(Prevent(func() { props.OnToggle(props.Option.Value) }))
	return Button(css.Class("cf-multipicker-option", tw.InlineFlex, tw.ItemsCenter, tw.Gap2, tw.RoundedFull, tw.Px3, tw.Py2), Type("button"),
		Attr("id", props.ID), Attr("role", "option"), Attr("aria-selected", ariaBool(props.Selected)),
		OnClick(onClick), props.Option.Label)
}

func toggleTxnBulkOwner(selected []string, value string) []string {
	return txntoolbar.ToggleOwner(selected, value)
}

// txnBulkOwnerChoice stages one picker choice until the picker is dismissed.
func txnBulkOwnerChoice(selected []string, value string) []string {
	return toggleTxnBulkOwner(selected, value)
}

// commitTxnBulkOwnerPick applies the completed owner selection through the
// durable bulk path, which supplies the Undo notice after persistence.
func commitTxnBulkOwnerPick(selected []string, apply func([]string)) {
	if apply != nil {
		apply(txnBulkAppliedOwners(selected))
	}
}

func txnBulkAppliedOwners(selected []string) []string { return txntoolbar.AppliedOwners(selected) }

func txnBulkBarWidgetStyle() map[string]string {
	return map[string]string{
		"position": "fixed", "inset": "0", "z-index": "70", "pointer-events": "none",
		"overflow": "visible", "background": "transparent", "border": "0", "box-shadow": "none",
	}
}

func sameOwnerList(left, right []string) bool { return txntoolbar.SameOwners(left, right) }

func assignSelectedTxnOwners(app *appstate.App, selected map[string]bool, owners []string) ([]domain.Transaction, error) {
	writes := txntoolbar.Draft(app.Transactions(), selected, txntoolbar.Edit{Kind: txntoolbar.Owners, Owners: owners})
	prior := txntoolbar.SelectedRows(app.Transactions(), txntoolbar.SelectedIDs(writes), false)
	if err := app.PutTransactionsAtomic(writes); err != nil {
		return nil, err
	}
	return prior, nil
}

// txnCategoryAfterManualChoice records that a person explicitly chose this
// transaction's category, without rewriting how the transaction entered the ledger.
func txnCategoryAfterManualChoice(t domain.Transaction) domain.Transaction {
	return txnreview.Confirm(t, t.CategoryID)
}

func txnSearchOptions(options []uiw.SelectOption, selected []string, query string) ([]uiw.SelectOption, int) {
	plain := make([]txntoolbar.Option, len(options))
	for i, o := range options {
		plain[i] = txntoolbar.Option{Value: o.Value, Label: o.Label}
	}
	found, count := txntoolbar.SearchOptions(plain, selected, query)
	result := make([]uiw.SelectOption, len(found))
	for i, o := range found {
		result[i] = uiw.SelectOption{Value: o.Value, Label: o.Label}
	}
	return result, count
}
