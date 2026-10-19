// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
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
	"github.com/monstercameron/CashFlux/internal/txnview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
func txnSummaryScope(f uistate.TxFilter) uistate.TxFilter {
	f.Sort, f.Dir = "", ""
	f.Page, f.PageSize = 0, 0
	return f
}

type txnToolbarSummaryCache struct {
	revision int
	filter   uistate.TxFilter
	value    txnToolbarSummary
	valid    bool
}

func (c *txnToolbarSummaryCache) get(revision int, filter uistate.TxFilter, compute func() txnToolbarSummary) txnToolbarSummary {
	if !c.valid || c.revision != revision || c.filter != filter {
		c.value = compute()
		c.revision, c.filter, c.valid = revision, filter, true
	}
	return c.value
}

func buildTxnToolbarIndexes(accounts []domain.Account, categories []domain.Category, members []domain.Member, txns []domain.Transaction) txnToolbarIndexes {
	indexes := txnToolbarIndexes{
		accountNames: make(map[string]string, len(accounts)), categoryNames: make(map[string]string, len(categories)),
		memberNames: make(map[string]string, len(members)),
	}
	for _, a := range accounts {
		indexes.accountNames[a.ID] = a.Name
	}
	for _, c := range categories {
		indexes.categoryNames[c.ID] = c.Name
	}
	for _, m := range members {
		indexes.memberNames[m.ID] = m.Name
	}
	indexes.accountOptions = withAllOption(uistate.T("transactions.allAccounts"), uiw.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, ""))
	indexes.categoryOptions = withAllOption(uistate.T("transactions.allCategories"), uiw.OptionsFrom(categories, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name }, ""))
	indexes.memberOptions = withAllOption(uistate.T("transactions.allMembers"), uiw.OptionsFrom(members, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, ""))
	indexes.sourceOptions = withAllOption(uistate.T("transactions.allSources"), uiw.OptionsFrom(domain.AllTxnSources, func(s domain.TxnSource) string { return string(s) }, func(s domain.TxnSource) string { return s.Label() }, ""))
	tagSet := make(map[string]struct{})
	for _, t := range txns {
		for _, tag := range t.Tags {
			if tag = strings.TrimSpace(tag); tag != "" {
				tagSet[tag] = struct{}{}
			}
		}
	}
	for tag := range tagSet {
		indexes.tags = append(indexes.tags, tag)
	}
	sort.Strings(indexes.tags)
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
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	layout := filterDateLayout(style)
	return parsed.Format(layout)
}

func parseFilterDate(style prefs.DateStyle, value string) (string, bool) {
	for _, layout := range []string{filterDateLayout(style), "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Format("2006-01-02"), true
		}
	}
	return "", false
}

func filterDateLayout(style prefs.DateStyle) string {
	switch style {
	case prefs.DateUS:
		return "01/02/2006"
	case prefs.DateEU:
		return "02/01/2006"
	case prefs.DateLong:
		return "Jan 2, 2006"
	default:
		return "2006-01-02"
	}
}

// applyParsedDateBounds replaces both bounds when a parsed date clause exists.
// Empty bounds are meaningful: `since March` intentionally removes an older To.
func applyParsedDateBounds(current *uistate.TxFilter, parsed txnfilter.Criteria) {
	if parsed.From != "" || parsed.To != "" {
		current.From, current.To = parsed.From, parsed.To
		current.PeriodPreset = txnfilter.PeriodCustom
	}
}

// containsStr reports whether ss contains v.
func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func formatTxnChipDate(iso string) string {
	return formatTxnChipDateWith(iso, uistate.LoadPrefs().FormatDate)
}

func formatTxnChipDateWith(iso string, format func(time.Time) string) string {
	date, err := dateutil.ParseDate(iso)
	if err != nil {
		return iso
	}
	return format(date)
}

func txnSearchChipLabel(query string) string {
	return uistate.T("transactions.chipFiltering", query)
}

// txnAmountChipLabel shows which bound a chip removes and formats its value in
// the household's base currency. Invalid stored values remain visible for repair.
func txnAmountChipLabel(bound, value, base string) string {
	operator := ""
	switch bound {
	case "min":
		operator = "≥"
	case "max":
		operator = "≤"
	default:
		return value
	}
	minor, err := money.ParseMinor(value, currency.Decimals(base))
	if err != nil {
		return uistate.T("transactions.chipAmountBound", operator, value)
	}
	return uistate.T("transactions.chipAmountBound", operator, fmtMoney(money.New(minor, base)))
}

// txnFilterRangeError explains a user-entered range whose lower bound exceeds its
// upper bound. Malformed legacy values are left to the field and parser to handle.
func txnFilterRangeError(filter uistate.TxFilter, base string) string {
	if filter.AmountMin != "" && filter.AmountMax != "" {
		min, minErr := money.ParseMinor(filter.AmountMin, currency.Decimals(base))
		max, maxErr := money.ParseMinor(filter.AmountMax, currency.Decimals(base))
		if minErr == nil && maxErr == nil && min > max {
			return uistate.T("transactions.filterRangeAmountInvalid")
		}
	}
	if filter.From != "" && filter.To != "" {
		from, fromErr := dateutil.ParseDate(filter.From)
		to, toErr := dateutil.ParseDate(filter.To)
		if fromErr == nil && toErr == nil && from.After(to) {
			return uistate.T("transactions.filterRangeDateInvalid")
		}
	}
	return ""
}

// excludedTxnCount reports how many visible rows are omitted from reports.
func excludedTxnCount(txns []domain.Transaction) int {
	n := 0
	for _, txn := range txns {
		if txn.ExcludeFromReports {
			n++
		}
	}
	return n
}

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
	count := 0
	for _, opt := range props.Opts {
		if opt.Value != "" && containsStr(props.Selected, opt.Value) {
			count++
		}
	}
	needle := strings.ToLower(strings.TrimSpace(query.Get()))
	filtered := make([]uiw.SelectOption, 0, len(props.Opts))
	for _, opt := range props.Opts {
		if opt.Value == "" || (needle != "" && !strings.Contains(strings.ToLower(opt.Label), needle)) {
			continue
		}
		filtered = append(filtered, opt)
	}
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
	total, selCount := 0, 0
	for _, o := range props.Opts {
		if o.Value == "" {
			continue
		}
		total++
		if containsStr(props.Selected, o.Value) {
			selCount++
		}
	}
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
	if !collapsed {
		visible := make([]uiw.SelectOption, 0, len(options))
		for _, option := range options {
			if option.Value != "" {
				visible = append(visible, option)
			}
		}
		return visible
	}

	visible := make([]uiw.SelectOption, 0, filterGroupCollapseThreshold+len(selected))
	for _, option := range options {
		if option.Value == "" {
			continue
		}
		if len(visible) < filterGroupCollapseThreshold || containsStr(selected, option.Value) {
			visible = append(visible, option)
		}
	}
	return visible
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
	chips := make([]uiw.Chip, 0, len(active))
	for _, af := range active {
		// PageFilters already excludes these. Keep the guard for callers that
		// supply a pre-built chip list (and for legacy tests/data) directly.
		if af.Field == txnfilter.FieldFrom || af.Field == txnfilter.FieldTo {
			continue
		}
		chips = append(chips, uiw.Chip{Key: string(af.Field) + chipKeySep + af.Value, Label: label(af)})
	}
	return chips
}

// addTxnTag adds a tag once, comparing case-insensitively to avoid near-duplicate
// values created by capitalization differences.
func addTxnTag(tags []string, tag string) []string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return tags
	}
	for _, existing := range tags {
		if strings.EqualFold(strings.TrimSpace(existing), tag) {
			return tags
		}
	}
	return append(append([]string(nil), tags...), tag)
}

func txnHasTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if strings.EqualFold(strings.TrimSpace(existing), strings.TrimSpace(tag)) {
			return true
		}
	}
	return false
}

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
	cats := app.Categories()
	for _, c := range cats {
		if c.ID == id {
			return catname.Path(cats, c)
		}
	}
	return ""
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

func txnToolbarWidget(props txnToolbarProps) ui.Node {
	defer txnPerfStart("toolbar")()
	app := props.App
	filterAtom := uistate.UseTxFilter()
	displayPrefs := uistate.UsePrefs().Get()
	selAtom := uistate.UseTxnSelection()
	colsModalAtom := uistate.UseTxnColsModalOpen()
	smartCatAtom := uistate.UseTxnSmartCatOpen()
	openReview := ui.UseEvent(Prevent(func() { uistate.OpenReviewInbox() }))
	importPanelAtom := uistate.UseImportPanelOpen()
	dupModalAtom := uistate.UseDuplicatesModalOpen()
	transferReviewAtom := uistate.UseTransferReviewOpen()
	nav := router.UseNavigate()
	// Keep the former Rules action's event slot stable while its callback now
	// lives in the More menu.
	ui.UseEvent(Prevent(func() { txnLeaveFor("/rules"); nav.Navigate(uistate.RoutePath("/rules")) }))
	// C363: the Rules workbench (/rules) is a full first-class surface, but nothing
	// on Transactions pointed there. A labeled toolbar entry (with the active-rule
	// count) makes it reachable in one visible click.

	// View-mode toggles: Calendar swaps the main slot (TX8); Register adds a running-
	// balance column (TX12) and is only offered when the filter scopes to one account.
	viewAtom := uistate.UseTxnViewMode()
	registerAtom := uistate.UseTxnRegisterMode()
	calActive := viewAtom.Get() == uistate.TxnViewCalendar
	// C560: Calendar has left the "⋯ More" menu for the scope bar's explicit
	// Ledger/Calendar pair. Buried in an overflow it had no pressed state and no
	// obvious way back — a user who switched views had to find the same hidden entry
	// again to return. Register stays here: it decorates the table rather than
	// replacing it, so it is a setting, not a view. Its handler is a plain closure
	// invoked from OverflowMenuItem.OnSelect, never an On* hook inside a loop.
	onRegister := func() { registerAtom.Set(!registerAtom.Get()) }

	// fOwn is the filter the USER set on this page. f layers the top bar's member
	// perspective on top of it for the purpose of filtering, but the two are kept
	// apart on purpose: the lens belongs to the top bar and is cleared there, so it
	// gets its own chip rather than masquerading as one of the page's filters (C574).
	fOwn := filterAtom.Get()
	// One hook call, held in a local: UseMemberLens wraps a state atom, so calling it
	// again later (inside the chip loop, say) would be a conditional hook.
	memberLens := uistate.UseMemberLens()
	f, lensOn := fOwn.WithOwnerLens(memberLens)

	dataRevision := int(app.Rev())
	accounts := app.Accounts()
	categories := app.Categories()
	members := app.Members()
	txns := app.Transactions()
	ruleCount := len(app.Rules()) // C363: active-rule count for the toolbar Rules entry

	indexes := ui.UseMemo(func() txnToolbarIndexes {
		return buildTxnToolbarIndexes(accounts, categories, members, txns)
	}, dataRevision, app.Rev())
	accName, catName, memberName := indexes.accountNames, indexes.categoryNames, indexes.memberNames

	setFilter := func(mut func(*uistate.TxFilter)) { setTxFilterOn(filterAtom, mut) }
	clearAllFilters := func() {
		cleared := filterAtom.Get().ClearPageFilters()
		filterAtom.Set(cleared)
		uistate.PersistTxFilter(cleared)
	}

	// Text/amount inputs fire on every keystroke, and each setFilter re-filters the whole
	// ledger + persists the filter — so those are DEBOUNCED (~250ms) to filter/persist
	// once you pause typing rather than on every character; the native input holds what
	// you type between renders, so search-as-you-type stays smooth on a large ledger. The
	// date fields commit a single value from the picker, so they set immediately.
	debFilterD := func(key string, delay time.Duration, mut func(*uistate.TxFilter)) {
		debounce.Call("txn-filter:"+key, delay, func() { setFilter(mut) })
	}
	debFilter := func(key string, mut func(*uistate.TxFilter)) { debFilterD(key, 250*time.Millisecond, mut) }
	// Search applies after a short pause so input stays immediate while the expensive
	// filtered result is only rebuilt once per typing burst.
	// C619: the ledger shows the PREVIOUS query's rows during the debounce, so the
	// search control marks that interval explicitly. Pending goes up
	// the moment a keystroke lands and comes down inside the debounced callback, so
	// it spans exactly the window where what is on screen is out of date.
	//
	// C628: the flag moved from this tile's own state to a shared atom. The rows it
	// warns about live in the TABLE tile, and a warning that cannot reach the thing
	// it is warning about is only half a warning — the table now dims them and
	// makes them inert for the same window this note covers.
	searchPending := uistate.UseTxnSearchPending()
	onFilterText := func(v string) {
		// Only raise the flag when it is not already up. Setting a shared atom
		// re-renders the surface, and this surface's render re-runs the whole
		// filter — ApplyWithLabels over every transaction in the ledger, which is
		// ~1.5s on a 3,000-row household. Re-asserting `true` on every keystroke
		// therefore bought one full re-filter per character to say something the
		// screen was already saying: typing six characters cost six of them, about
		// 9.5 seconds, and the debounce below could not help because it only
		// coalesces the FILTER write, not these renders. One raise per typing burst,
		// one lower when it commits.
		if !searchPending.Get() {
			searchPending.Set(true)
		}
		debFilterD("text", 150*time.Millisecond, func(x *uistate.TxFilter) {
			x.Text = v
			searchPending.Set(false)
		})
	}
	// C628: the ✕ is the one control whose whole job is "undo what I just typed",
	// and routing it through the same coalescing left the query on screen for
	// exactly the window it was pressed to end. Cancel the pending call first —
	// otherwise it fires afterwards and writes the query straight back.
	clearSearchNow := func() {
		debounce.Flush("txn-filter:text")
		searchPending.Set(false)
		setFilter(func(x *uistate.TxFilter) { x.Text = "" })
	}
	onFilterAmountMin := ui.UseEvent(func(v string) { debFilter("amtmin", func(x *uistate.TxFilter) { x.AmountMin = v }) })
	onFilterAmountMax := ui.UseEvent(func(v string) { debFilter("amtmax", func(x *uistate.TxFilter) { x.AmountMax = v }) })
	onFilterFrom := func(v string) {
		setFilter(func(x *uistate.TxFilter) { x.From, x.PeriodPreset = v, txnfilter.PeriodCustom })
	}
	onFilterTo := func(v string) {
		setFilter(func(x *uistate.TxFilter) { x.To, x.PeriodPreset = v, txnfilter.PeriodCustom })
	}
	onFilterCustomValText := ui.UseEvent(func(v string) { debFilter("customval", func(x *uistate.TxFilter) { x.CustomVal = v }) })
	onAdd := ui.UseEvent(Prevent(func() { uistate.SetQuickAdd(true) }))
	// C573: the type-scoped entries behind the primary Add. Plain closures — the
	// OverflowMenu item component owns each click hook — and each one lands the user
	// somewhere that matches what its label promised, which is the whole point: three
	// buttons that all opened the same neutral form told nobody anything.
	transferOpenAtom := uistate.UseAcctTransferOpen()
	addExpense := func() { uistate.OpenQuickAddAs(uistate.QuickAddExpense) }
	addIncome := func() { uistate.OpenQuickAddAs(uistate.QuickAddIncome) }
	addTransfer := func() { transferOpenAtom.Set(true) }

	// doExportCSV downloads the filtered ledger as CSV. It's a plain closure (not a
	// UseEvent hook) because it's invoked from the "More" overflow menu's OnSelect
	// (a func()), not wired directly to a button's OnClick.
	// props.Shown, NOT a re-derived filter. Both of the closures below used to
	// recompute the population with txnfilter.Apply(app.Transactions(),
	// filterAtom.Get()), which differed from what the ledger was actually showing in
	// two ways: it dropped the top bar's member lens (filterAtom holds the user's own
	// filter; the lens is layered on afterwards), and it passed no Labels, so the
	// payee-alias resolver was nil and rows that matched the search only through a
	// learned merchant alias fell out. Exporting or selecting a population the user
	// cannot see is bad on its own; with Bulk Delete on the other end of a selection
	// it is data loss. props.Shown is the very slice the table and the count line are
	// built from, so these now agree with the screen by construction.
	doExportCSV := func() {
		rows := props.Shown
		if len(rows) == 0 {
			uistate.PostNotice(uistate.T("transactions.noExport"), true)
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		downloadBytes("transactions.csv", "text/csv", data)
	}
	// Export CSV and Columns are now items in the "⋯ More" overflow (built below), so
	// they use the doExportCSV closure / colsModalAtom directly — no separate handlers.

	// Plain func (not UseEvent): Select-all runs from the "⋯ More" overflow menu
	// (2026-07-17 audit — thin the resting toolbar row), whose item component owns
	// the click hook.
	selectAllFiltered := func() {
		shown := props.Shown
		cur := selAtom.Get()
		// Toggle: if every shown row is already selected, clear the selection;
		// otherwise select all shown rows.
		allSelected := len(shown) > 0
		for _, t := range shown {
			if !cur[t.ID] {
				allSelected = false
				break
			}
		}
		if allSelected {
			selAtom.Set(map[string]bool{})
			return
		}
		nm := map[string]bool{}
		for _, t := range shown {
			nm[t.ID] = true
		}
		selAtom.Set(nm)
	}

	// Filter option lists, built once from the entity slices via the reusable
	// OptionsFrom helper (value extractor + label extractor) rather than per-field loops.
	accOpts, catOpts, memberOpts, sourceOpts, tagOpts, tagList := indexes.accountOptions, indexes.categoryOptions, indexes.memberOptions, indexes.sourceOptions, indexes.tagOptions, indexes.tags

	// The resting (empty) value reads as "no cleared-status filter" — the old
	// "Cleared & not" label made the select look like an active filter at rest
	// (review #19). Empty still maps to no-op; picking yes/no filters, clearing
	// back to Any round-trips.
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

	chipLabel := func(af txnfilter.ActiveFilter) string {
		switch af.Field {
		case txnfilter.FieldText:
			// Search text is named directly so the chip reads clearly beside the
			// toolbar's "Filtering by" summary.
			return txnSearchChipLabel(af.Value)
		case txnfilter.FieldAccount:
			return uistate.T("transactions.chipAccount", accName[af.Value])
		case txnfilter.FieldCategory:
			return uistate.T("transactions.chipCategory", catName[af.Value])
		case txnfilter.FieldMember:
			return uistate.T("transactions.chipMember", memberName[af.Value])
		case txnfilter.FieldSource:
			return uistate.T("transactions.chipSource", domain.TxnSource(af.Value).Label())
		case txnfilter.FieldTag:
			return uistate.T("transactions.chipTag", af.Value)
		case txnfilter.FieldAmountMin:
			return txnAmountChipLabel("min", af.Value, props.Base)
		case txnfilter.FieldAmountMax:
			return txnAmountChipLabel("max", af.Value, props.Base)
		case txnfilter.FieldFrom:
			return uistate.T("transactions.chipFrom", formatTxnChipDate(af.Value))
		case txnfilter.FieldTo:
			return uistate.T("transactions.chipTo", formatTxnChipDate(af.Value))
		case txnfilter.FieldCleared:
			if af.Value == "yes" {
				return uistate.T("transactions.cleared")
			}
			return uistate.T("transactions.notCleared")
		case txnfilter.FieldFlow:
			if af.Value == "in" {
				return uistate.T("transactions.chipFlowIn")
			}
			return uistate.T("transactions.chipFlowOut")
		case txnfilter.FieldUncategorized:
			// The quick filter stores "1", and the chip used to render that literally —
			// a chip that reads "1" beside chips that read "Groceries" (C574).
			return uistate.T("transactions.chipUncategorized")
		case txnfilter.FieldCustom:
			// A custom-field chip showed only the value ("blue"), never which field it
			// belonged to, so two custom filters were indistinguishable.
			return uistate.T("transactions.chipCustom", customFieldLabel(app.CustomFieldDefsFor("transaction"), fOwn.CustomKey), af.Value)
		}
		return af.Value
	}
	// Chips come from the user's OWN filter; the ambient member lens gets a chip of
	// its own below. Deriving them from the layered filter instead would offer a
	// member chip whose ✕ calls RemoveValue on a criteria that never held the member —
	// a control that names something and then removes nothing (C574).
	active := fOwn.PageFilters()
	// The lens goes in the summary row's PRE-chip slot: ahead of the "Filtering by"
	// lead-in, outside the Filters badge count, and untouched by the reset — because
	// it is the top bar's perspective, not one of this page's filters.
	var lensChips []uiw.Chip
	if lensOn {
		// One chip per lensed member, keyed so OnRemoveChip can tell "clear the top
		// bar's perspective" from "drop one of this page's filters".
		for _, id := range memberLens {
			lensChips = append(lensChips, uiw.Chip{
				Key:         lensChipKey + chipKeySep + id,
				Label:       uistate.T("transactions.lensChip", memberName[id]),
				Class:       "filter-chip-lens",
				RemoveLabel: uistate.T("transactions.lensChipRemove"),
			})
		}
	}
	// C585: when this view was opened from a budget, name the budget ahead of the
	// filter sentence. The drill applies one chip per tracked category AND per
	// descendant, which is the correct scope but arrives unattributed; saying
	// where it came from lets the reader check it against the card they clicked,
	// and its ✕ drops the whole scope in one action.
	if bd := uistate.BudgetDrillFor(fOwn); bd.BudgetID != "" {
		lensChips = append(lensChips, uiw.Chip{
			Key:         budgetDrillChipKey + chipKeySep + bd.BudgetID,
			Label:       uistate.T(budgetDrillChipKeyFor(bd), bd.Name),
			Class:       "filter-chip-lens",
			RemoveLabel: uistate.T("transactions.budgetDrillChipRemove"),
		})
	}
	// Date bounds are represented by the period control, never as removable chips.
	chips := txnActiveFilterChips(active, chipLabel)

	// Natural-language search (TX2 / SMART-T3F, Free tier): when the typed query
	// compiles into at least one structured clause, offer a quiet row that turns
	// the words into the normal removable filter chips. This teaches the filter
	// system — users watch their sentence become chips they can then tweak. Gated
	// on the Free-tier feature toggle so it can be turned off.
	//
	// nlContext builds the parser's vocabulary + clock from the live entity lists.
	// It's a closure (not precomputed) so the apply handler below re-reads the
	// current query at click time rather than capturing a stale render's value.
	nlContext := func() nlfilter.Context {
		nlCats := make([]nlfilter.NameID, 0, len(categories))
		for _, c := range categories {
			nlCats = append(nlCats, nlfilter.NameID{Name: c.Name, ID: c.ID})
		}
		resolver := app.PayeeResolver()
		payeeSet := map[string]struct{}{}
		payeeNames := make([]string, 0)
		for _, t := range txns {
			if name := resolver.Resolve(t.Payee); name != "" {
				if _, seen := payeeSet[name]; !seen {
					payeeSet[name] = struct{}{}
					payeeNames = append(payeeNames, name)
				}
			}
		}
		return nlfilter.Context{
			Now:          time.Now().UTC(),
			WeekStart:    uistate.CurrentPrefs().WeekStartWeekday(),
			Categories:   nlCats,
			Tags:         tagList,
			Payees:       payeeNames,
			ResolvePayee: func(raw string) string { return resolver.Resolve(raw) },
		}
	}
	// applyNL compiles the current query and overwrites only the dimensions the
	// parser recognized, replacing the raw typed text with structured chips. The
	// hook is created every render (stable position); the button that fires it is
	// conditional (see below).
	applyNL := ui.UseEvent(Prevent(func() {
		parsed, ok := nlfilter.Parse(strings.TrimSpace(filterAtom.Get().Text), nlContext())
		if !ok {
			return
		}
		setFilter(func(x *uistate.TxFilter) {
			x.Text = parsed.Text
			if parsed.AmountMin != "" {
				x.AmountMin = parsed.AmountMin
			}
			if parsed.AmountMax != "" {
				x.AmountMax = parsed.AmountMax
			}
			applyParsedDateBounds(x, parsed)
			if parsed.Cleared != "" {
				x.Cleared = parsed.Cleared
			}
			if parsed.Flow != "" {
				x.Flow = parsed.Flow
			}
			if parsed.Categories != "" {
				x.Categories = parsed.Categories
			}
			if parsed.Tags != "" {
				x.Tags = parsed.Tags
			}
		})
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

	// Custom-field filter (L18): a field picker + a value control shaped by the field's
	// type — both built from the same reusable SelectInput.
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

	// Toggling a value in a categorical dimension (account/category/member/source/tag)
	// adds or removes it from that dimension's multi set — multiple values allowed.
	onToggleFilter := func(field txnfilter.FilterField, value string) {
		setFilter(func(x *uistate.TxFilter) { *x = x.ToggleValue(field, value) })
	}

	// TXC-3: quick-filter preset chips — one-tap common filters that write the
	// persisted criteria (each shows "on" when its filter is engaged, toggles off).
	// They live INSIDE the Filters panel (2026-07-19 two-row-toolbar pass) rather than
	// as a separate always-on strip, so the resting page is just the toolbar row + the
	// active-filter chips before the first ledger row.
	monthStart := dateutil.MonthStart(time.Now())
	monthEndEx := dateutil.AddMonths(monthStart, 1)
	ledgerCounts := ui.UseMemo(func() txnToolbarSummary {
		defer txnPerfStart("ledger-counts")()
		return txnToolbarSummary{review: reviewqueue.Count(txns),
			duplicates: dedupe.Count(dedupe.FindDuplicates(txns)),
			suspects:   len(txnclassify.Suspects(txns, func(id string) string { return catName[id] }))}
	}, app, app.Rev())
	summaryCache := ui.UseMemo(func() *txnToolbarSummaryCache { return &txnToolbarSummaryCache{} }, app, app.Rev(), monthStart)
	summary := summaryCache.get(dataRevision, txnSummaryScope(f), func() txnToolbarSummary {
		defer txnPerfStart("summary")()
		finishPresets := txnPerfStart("summary-presets")
		result := txnToolbarSummary{
			review: ledgerCounts.review,
			presets: txnfilter.CountPresetsWithLabels(txns, f, txnfilter.PresetScope{
				ReviewTag:  reviewqueue.ReviewTag,
				LargeMinor: currency.MinorFromMajor(100, props.Base),
				MonthStart: monthStart, MonthEnd: monthEndEx,
			}, txnfilter.Labels{Account: accName, Category: catName, Payee: app.PayeeResolver().Resolve,
				BaseCurrency: props.Base, ToBase: func(m money.Money) (money.Money, error) { return props.Rates.Convert(m, props.Base) }}),
			duplicates: ledgerCounts.duplicates,
			suspects:   ledgerCounts.suspects,
		}
		finishPresets()
		finishView := txnPerfStart("summary-view")
		view, ok := txnDerivedViewCache.Lookup(app.Rev(), txnDerivedScope(app, f))
		if !ok {
			view, _ = txnview.Summarize(props.Shown, props.Rates, props.Base)
		}
		result.shownNet, result.spentMinor, result.incomeMinor = view.Net, view.Spent, view.Income
		result.movedMinor, result.movedRows, result.unpairedRows = view.Transferred, view.MovedRows, view.UnpairedRows
		result.unconverted = view.Unconverted
		finishView()
		finishConversion := txnPerfStart("summary-conversion")
		cached := txnConversionIssuesCache
		if cached.valid && cached.app == app && cached.revision == app.Rev() && cached.scope == txnDerivedScope(app, f) {
			result.conversionIssues = cached.issues
		} else {
			conversionScope := f
			conversionScope.Page, conversionScope.PageSize = 0, 0
			conversionScope.Sort, conversionScope.Dir = "", ""
			_, result.conversionIssues = txnfilter.ApplyWithLabelsChecked(txns, conversionScope, txnfilter.Labels{
				Account: accName, Category: catName, Payee: app.PayeeResolver().Resolve, BaseCurrency: props.Base,
				ToBase: func(m money.Money) (money.Money, error) { return props.Rates.Convert(m, props.Base) },
			})
		}
		finishConversion()
		return result
	})
	// Preset match counts — like the toolbar's "Review N", so each chip shows its
	// payoff before the click.
	//
	// C568: counted WITHIN the active scope (search, member, period, other filters),
	// not over the whole ledger. A chip beside a search that reads "Large 1665" while
	// the search shows twelve rows is describing a population no click can produce.
	// The arithmetic lives in txnfilter.CountPresets, tested on native Go.
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

	// Redesigned filter panel: quick-filter presets on top, then each categorical
	// dimension as a group of toggle pills (multi-select), followed by the date/amount
	// ranges, the cleared status, and any custom-field filter.
	// EVERY control in this panel reads and writes the user's OWN filter (fOwn), not
	// the lens-layered one. Showing the layered value here would light the lensed
	// member's pill as if the user had chosen it, and the click that "turns it off"
	// goes through ToggleValue against the persisted filter — which never held that
	// member, so the toggle ADDS it. The lens would then be permanently baked into
	// the saved filter and the top bar's "Everyone" would stop working. A panel that
	// edits a value must display that same value.
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
			// C517: filtering by direction. The Criteria field, its matching logic
			// and its chip label all already existed — the only way to reach any of
			// it was to type a natural-language search, so this is a control over
			// machinery that was already built and tested, not new filtering.
			filterSelect(uistate.T("transactions.flowLabel"), f.Flow, flowOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Flow = v }) }),
		),
		customFilterNode,
	)

	// The Import and duplicates-review buttons both open flip modals now, so they're
	// plain actions — no open/close view-toggle labels. The dupes button badges its
	// count when there are possible duplicates to review.
	// Counted over the WHOLE ledger, because that is what the panel this button
	// opens scans (DuplicatesPanel builds its groups from app.Transactions()). The
	// count used to be taken from props.Shown, so with any filter active the button
	// advertised one number and the panel behind it showed another. Scoping the
	// panel to the filter instead would be worse than the mismatch: a duplicate pair
	// is only visible when BOTH rows are in the population, so a filter that hides
	// one half would hide the duplicate entirely.
	dupCount := summary.duplicates
	// C676: rows that look like transfers and are not linked. Counted over the
	// WHOLE ledger like the duplicate count above, not the filtered view: the badge
	// is a claim about the household's data, and one that shrank when you typed in
	// the search box would be describing the search.
	suspectCount := summary.suspects
	importBtnLabel := uistate.T("transactions.importBtn")
	dupBtnLabel := uistate.T("transactions.dupReviewBtn")
	if dupCount > 0 {
		dupBtnLabel = uistate.T("transactions.dupReviewBadge", plural(dupCount, "duplicate"))
	}

	// The transactions toolbar has the most actions of any page — too many to fit one
	// row without reading as button-soup. The secondary utilities (Import, Review
	// duplicates, Categorize, Export CSV, Columns) fold into a labeled "⋯ More" overflow
	// so the resting row stays a short line of high-frequency actions with the primary
	// Add at its right end. Each item keeps its original testid so e2e selectors still
	// resolve it (the menu items live in the DOM, revealed on open).
	// Register stays in "More": it decorates the table rather than replacing it, and
	// it is offered only when the ledger is scoped to one account and the calendar
	// isn't showing. A ✓ prefix marks it active. Calendar has moved out to the scope
	// bar's explicit view toggle (C560).
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
			// Select-all joined the overflow (2026-07-17 audit): a bulk-selection
			// setup step, not an everyday resting-row verb.
			{Label: uistate.T("transactions.selectAllFiltered"), Icon: icon.CheckCircle, TestID: "txn-selectall-btn",
				OnSelect: selectAllFiltered, Hidden: len(props.Shown) == 0},
			{Label: uistate.T("transactions.exportCsv"), Icon: icon.ArrowDown, TestID: "txn-export-btn", OnSelect: doExportCSV},
			{Label: uistate.T("transactions.columns"), Icon: icon.List, TestID: "txn-columns-btn", OnSelect: func() { colsModalAtom.Set(true) }},
		},
	})
	toolbar := uiw.FilterToolbar(uiw.FilterToolbarProps{
		Search:        f.Text,
		SearchLabel:   uistate.T("transactions.searchPlaceholder"),
		OnSearch:      onFilterText,
		OnClearSearch: clearSearchNow,
		SearchPending: searchPending.Get(),
		// The ledger table is what a typed query re-filters, so it is what must stop
		// being clickable while the rows on screen answer the previous one.
		ResultsSelector: ".data-table",
		FiltersLabel:    uistate.T("transactions.filters"),
		FiltersTitle:    uistate.T("transactions.filtersTitle"),
		FilterSheet:     true,
		ActiveAriaLabel: func(n int) string {
			if n == 0 {
				return uistate.T("transactions.filters")
			}
			return uistate.T("transactions.filtersActiveAria", plural(n, "filter"))
		},
		FilterFields: filtersBody,
		Chips:        chips,
		PreChips:     lensChips,
		OnRemoveChip: func(key string) {
			field, value, _ := strings.Cut(key, chipKeySep)
			// The member lens lives in the top bar, not in this page's filter, so its
			// ✕ clears the perspective. Routing it through RemoveValue (as every chip
			// used to be) removed nothing at all, because the persisted filter never
			// held the member the chip named.
			if field == lensChipKey {
				uistate.ClearMemberLens()
				return
			}
			// The budget-origin chip owns the SCOPE the drill applied — its
			// categories, tags and period — and drops all of it at once, because
			// removing one category out of a rolled-up set leaves a filter that
			// still claims to be the budget while no longer being it.
			//
			// It drops only what the budget contributed. Anything the user added
			// after arriving (a search, an account, a member) is their work, not the
			// budget's, and a control labelled for the origin note must not throw it
			// away (C585).
			if field == budgetDrillChipKey {
				uistate.ClearBudgetDrill()
				setFilter(func(x *uistate.TxFilter) {
					x.Category, x.Categories = "", ""
					x.Tag, x.Tags = "", ""
					x.ScopeAny = false
					x.From, x.To = "", ""
					x.PeriodPreset = txnfilter.PeriodAllDates
				})
				return
			}
			setFilter(func(x *uistate.TxFilter) { *x = x.RemoveValue(txnfilter.FilterField(field), value) })
		},
		OnClearAll: clearAllFilters,
		// The reset states its size, so "clear" is never a leap of faith about how
		// much of the current view is about to change — and it is absent when the
		// only chip showing is the member lens, which it does not clear (C574).
		ClearAllLabel: clearAllLabel(len(active)),
		RemoveLabel:   uistate.T("transactions.removeFilter"),
		// Transactions puts its period control with the view and add controls so the
		// responsive toolbar can keep Search + Filters together on its first row.
		// Standard labeled toolbar buttons (.btn-tool).
		Actions: []ui.Node{
			ui.CreateElement(txnDateScope, struct{}{}),
			ui.CreateElement(txnViewToggle, struct{}{}),
			Div(ClassStr("txn-toolbar-add"),
				// C573: the page offered three ways to start a transaction — this button,
				// the top bar's "+", and "Add something else" — and all three opened the
				// same neutral form. The page primary now stays with Search and Filters.
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
			),
			// Review inbox (CG-S2): the guided triage entry point, shown only when
			// something needs review, with a live count so the backlog is visible.
			// C575: the review backlog is counted across the WHOLE household and ignores
			// every filter on this page, so beside a filtered ledger its number belongs
			// to a different population than everything else on screen. The button keeps
			// the count — it is the backlog's whole point — and says which population it
			// is measuring, on hover and to assistive tech.
			If(summary.review > 0, Button(css.Class("txn-review-callout txn-review-callout-attention"), Type("button"),
				Attr("data-testid", "txn-review-btn"), Attr("aria-label", uistate.T("r1c.reviewCallout", summary.review)),
				OnClick(openReview), Span(css.Class("txn-review-dot"), Attr("aria-hidden", "true")),
				Span(uistate.T("r1c.reviewCallout", summary.review)))),
			// There is no second "Clear filters" button here any more. It did exactly
			// what the chip row's reset does, so the page offered two differently-worded
			// controls ("Clear filters" / "Clear all filters") for one action, sitting a
			// few centimetres apart — the ambiguity C574 is about. The reset now lives
			// with the chips it removes, and counts them.
			// Select-all, Calendar, and Register moved into the "⋯ More" overflow (2026-07
			// command-bar consolidation): the resting row keeps only the everyday verbs plus
			// the primary Add, so the page reads as a focused workspace, not a tool pile.
			// Saved Views is an extra group within More, not a separate toolbar stop.
			// Secondary utilities (Import, Duplicates, Categorize, Export CSV, Columns)
			// folded into the "⋯ More" overflow built above.
			moreMenu,
		},
	})

	// The net is a sum over the SAME rows the count beside it reports, so a row that
	// cannot be converted into the base currency must not be dropped in silence —
	// that prints a count and a money figure side by side measured against different
	// populations, and understates the total by exactly the rows nobody was told
	// about. The unconvertible rows are counted so the line can say so.
	// C681: a transfer leg is NOT cash flow, and summing it into a figure labelled
	// "net" describes it as one. A complete checking→savings pair cancels itself
	// and hides the problem; filter to either leg and the same line calls $500 of
	// money you still own spending — the exact figure reports and budgets already
	// exclude. So the sum splits: cash flow is what entered or left the household,
	// and moved money is stated separately rather than folded in or dropped.
	//
	// Counted, not hidden, because a filtered view of one leg has to say something:
	// silence about a row that is on screen is how someone concludes the ledger
	// lost it.
	shownNet, movedMinor := summary.shownNet, summary.movedMinor
	unconverted, movedRows, unpairedRows := summary.unconverted, summary.movedRows, summary.unpairedRows
	// C575: the count line replaces the screen-reader-only summary that used to live
	// here. It states the size of the view WITH its denominator and names the
	// perspective, so a sighted user can explain the number the same way a screen
	// reader could. It is the live region too — keeping the hidden one as well would
	// announce every filter change twice.
	lensName := ""
	if lensOn && len(memberLens) > 0 {
		lensName = memberName[memberLens[0]]
		if len(memberLens) > 1 {
			lensName = plural(len(memberLens), "member")
		}
	}
	countTotal := len(props.Shown)
	countLine := ui.CreateElement(txnCountLine, txnCountLineProps{
		Shown: len(props.Shown), Total: countTotal, Range: txnPageRange(f.Page, f.PageSize, len(props.Shown)),
		Revision: app.Rev(), CategoryTotals: txnCountCategories(f, memberLens),
		Net: money.New(shownNet, props.Base), Lens: lensName,
		Unconverted: unconverted, ConversionIssues: summary.conversionIssues,
		Excluded: func() int {
			if view, ok := txnDerivedViewCache.Lookup(app.Rev(), txnDerivedScope(app, f)); ok {
				return view.Excluded
			}
			return excludedTxnCount(props.Shown)
		}(),
		Moved: money.New(movedMinor, props.Base), MovedRows: movedRows, UnpairedRows: unpairedRows,
		Spent: money.New(summary.spentMinor, props.Base), Income: money.New(summary.incomeMinor, props.Base),
		CashflowProvided: true,
	})

	footRow := txnToolbarFoot(countLine)

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		// C560: the ledger's own period + view bar sits directly under the toolbar, so
		// the scope the rows are in is stated on the page that shows them rather than
		// by a top-bar pill reading different state.
		Body: txnResponsiveToolbar(toolbar,
			// Scheduled activity is context for the ledger, so it sits below the
			// controls and immediately before the count line and table.
			ui.CreateElement(txnUpcomingStrip, struct{}{}), interpretRow, footRow),
	})
}

func txnExpandedSelection(app *appstate.App, selected map[string]bool) map[string]bool {
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	return txnV5RowExpandedSelection(selected, app.ExpandTransferPairs(ids))
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
	return enabled && memberScope == "" && memberCount > 1
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
	if total <= 0 {
		return ""
	}
	if pageSize == 0 {
		pageSize = txnfilter.DefaultPageSize
	}
	if pageSize <= 0 || pageSize >= total {
		return uistate.T("txu5.range", txnCountNumber(1), txnCountNumber(total), txnCountNumber(total))
	}
	if page < 1 {
		page = 1
	}
	lastPage := (total + pageSize - 1) / pageSize
	if page > lastPage {
		page = lastPage
	}
	start := (page-1)*pageSize + 1
	end := start + pageSize - 1
	if end > total {
		end = total
	}
	return uistate.T("txu5.range", txnCountNumber(start), txnCountNumber(end), txnCountNumber(total))
}

func txnCountNumber(n int) string {
	digits := strconv.Itoa(n)
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var out strings.Builder
	out.Grow(len(digits) + len(digits)/3)
	out.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		out.WriteByte(',')
		out.WriteString(digits[i : i+3])
	}
	return out.String()
}

// txnMemberInitials keeps the optional member column compact while the cell's
// accessible name retains the full name.
func txnMemberInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "—"
	}
	first := []rune(parts[0])
	if len(first) == 0 {
		return "—"
	}
	initials := strings.ToUpper(string(first[0]))
	if len(parts) > 1 {
		last := []rune(parts[len(parts)-1])
		if len(last) > 0 {
			initials += strings.ToUpper(string(last[0]))
		}
	}
	return initials
}

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

type txnBulkPickerOption struct {
	Value  string
	Label  string
	Create bool
}

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
	filtered := make([]txnBulkPickerOption, 0, len(props.Options))
	needle := strings.ToLower(strings.TrimSpace(query.Get()))
	for _, option := range props.Options {
		// Empty values are the button's placeholder / no-selection value, not a
		// category or person that can be assigned to selected transactions.
		if option.Value == "" {
			continue
		}
		if needle == "" || strings.Contains(strings.ToLower(option.Label), needle) {
			filtered = append(filtered, option)
		}
	}
	if props.OnCreate != nil && needle != "" && len(filtered) == 0 {
		filtered = append(filtered, txnBulkPickerOption{
			Value: "__create_category__", Label: uistate.T("r10t.createCategory", strings.TrimSpace(query.Get())), Create: true,
		})
	}
	activeIndex := active.Get()
	if activeIndex >= len(filtered) {
		activeIndex = 0
	}
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
				active.Set((activeIndex + 1) % len(filtered))
			}
		case "ArrowUp":
			e.PreventDefault()
			if len(filtered) > 0 {
				index := activeIndex - 1
				if index < 0 {
					index = len(filtered) - 1
				}
				active.Set(index)
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

	clearSel := func() {
		selAtom.Set(map[string]bool{})
		anchorAtom.Set("")
	}
	selectionActive := len(selAtom.Get()) > 0
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

	// Every bulk edit validates all rows and commits one batch. Selection remains
	// available on failure; there is one durable Undo affordance for the action.
	applyBatch := func(label func(int) string, writes []domain.Transaction, done func()) {
		if len(writes) == 0 {
			return
		}
		ids := make([]string, len(writes))
		for i, t := range writes {
			ids[i] = t.ID
		}
		_ = CommitUndoableCounted(app, label, ids, func() error { return app.PutTransactionsAtomic(writes) }, func(err error) {
			if err != nil {
				uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
				return
			}
			clearSel()
			if done != nil {
				done()
			}
		})
	}
	bulkSetCleared := func(value bool) {
		sel := txnExpandedSelection(app, selAtom.Get())
		var writes []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.Cleared != value {
				writes = append(writes, t.MarkCleared(value, time.Now()))
			}
		}
		key := "transactions.bulkOpCleared"
		if !value {
			key = "transactions.bulkOpUncleared"
		}
		applyBatch(func(n int) string { return uistate.T(key, n) }, writes, nil)
	}
	bulkMarkReviewedAction := func() {
		sel := txnExpandedSelection(app, selAtom.Get())
		var writes []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] {
				confirmed := txnreview.Confirm(t, t.CategoryID)
				// A reviewed row can still carry a review tag; confirmation must remove it.
				if !reflect.DeepEqual(t, confirmed) {
					writes = append(writes, confirmed)
				}
			}
		}
		applyBatch(func(n int) string { return uistate.T("r1c.markedReviewed", n) }, writes, nil)
	}
	bulkMarkReviewed := ui.UseEvent(Prevent(bulkMarkReviewedAction))
	bulkSetExclude := func(value bool) {
		sel := txnExpandedSelection(app, selAtom.Get())
		var writes []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.ExcludeFromReports != value {
				t.ExcludeFromReports = value
				writes = append(writes, t)
			}
		}
		key := "transactions.bulkOpExcluded"
		if !value {
			key = "transactions.bulkOpIncluded"
		}
		applyBatch(func(n int) string { return uistate.T(key, n) }, writes, nil)
	}
	applyBulkCategory := func(cid string) {
		if checkpointSaving.Get() {
			return
		}
		sel := txnExpandedSelection(app, selAtom.Get())
		checkpointSaving.Set(true)
		uistate.SaveCheckpointThen(uistate.T("ckpt.beforeBulkRecat", plural(len(sel), "transaction")), func(_ string, err error) {
			checkpointSaving.Set(false)
			if err != nil {
				uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
				return
			}
			var writes []domain.Transaction
			for _, t := range app.Transactions() {
				if sel[t.ID] && !t.IsTransfer() {
					confirmed := txnreview.Confirm(t, cid)
					if !reflect.DeepEqual(t, confirmed) {
						writes = append(writes, confirmed)
					}
				}
			}
			label := func(n int) string {
				if cid != "" {
					return uistate.T("transactions.bulkCatApplied", n, bulkCatLabel(app, cid))
				}
				return uistate.T("transactions.bulkOpClearedCat", n)
			}
			applyBatch(label, writes, func() {
				if bulkCatAtom.Get() != "" {
					bulkCatAtom.Set("")
				}
			})
		})
	}
	assignBulkOwners := func(owners []string) {
		sel := txnExpandedSelection(app, selAtom.Get())
		var writes []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && !sameOwnerList(t.Owners(), owners) {
				t.SetOwners(owners)
				writes = append(writes, t)
			}
		}
		applyBatch(func(n int) string { return uistate.T("transactions.bulkOpAssigned", n) }, writes, nil)
	}
	applyBulkTag := func(raw string) {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			return
		}
		sel := txnExpandedSelection(app, selAtom.Get())
		var writes []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && !txnHasTag(t.Tags, tag) {
				t.Tags = addTxnTag(t.Tags, tag)
				writes = append(writes, t)
			}
		}
		applyBatch(func(n int) string { return uistate.T("r1c.tagged", n, tag) }, writes, func() { tagValue.Set(""); tagOpen.Set(false) })
	}
	addTag := ui.UseEvent(Prevent(func() { applyBulkTag(tagValue.Get()) }))
	onTagInput := ui.UseEvent(func(e ui.Event) { tagValue.Set(e.GetValue()) })
	toggleTagInputAction := func() { tagOpen.Set(!tagOpen.Get()) }
	bulkGroupOrder := func() {
		sel := selAtom.Get()
		var ids []string
		for _, t := range app.Transactions() {
			if sel[t.ID] && !t.IsTransfer() {
				ids = append(ids, t.ID)
			}
		}
		if len(ids) < 2 {
			uistate.PostNotice(uistate.T("txnlinks.groupNeedTwo"), true)
			return
		}
		_ = CommitUndoable(app, uistate.T("txnlinks.grouped", len(ids)), ids, func() error { return app.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: ids}) }, func(err error) {
			if err != nil {
				uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
				return
			}
			clearSel()
		})
	}

	exportSelected := func() {
		sel := selAtom.Get()
		if len(sel) == 0 {
			return
		}
		rows := make([]domain.Transaction, 0, len(sel))
		for _, t := range app.Transactions() {
			if sel[t.ID] {
				rows = append(rows, t)
			}
		}
		if len(rows) == 0 {
			uistate.PostNotice(uistate.T("transactions.noExport"), true)
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		downloadBytes("transactions-selected.csv", "text/csv", data)
	}

	bulkDeleteAction := func() {
		if checkpointSaving.Get() {
			return
		}
		sel := selAtom.Get()
		count := len(sel)
		uistate.ConfirmModalLabeled(
			uistate.TN("uxt4.deleteOne", "uxt4.deleteMany", count),
			uistate.T("uxt4.delete"), true, func(ok bool) {
				if !ok {
					return
				}
				// #55: whole-dataset checkpoint before the bulk delete (restorable
				// from Settings → Data even after the session undo stack is gone).
				checkpointSaving.Set(true)
				uistate.SaveCheckpointThen(uistate.T("ckpt.beforeBulkDelete", plural(count, "transaction")), func(_ string, checkpointErr error) {
					checkpointSaving.Set(false)
					if checkpointErr != nil {
						uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
						return
					}
					ids := make([]string, 0, len(sel))
					for id := range sel {
						ids = append(ids, id)
					}
					doomed := app.ExpandTransferPairs(ids)
					_ = CommitUndoableCounted(app, func(n int) string { return uistate.T("transactions.bulkOpDeleted", n) }, doomed, func() error {
						_, err := app.DeleteTransactionsBulkExpanded(doomed)
						return err
					}, func(err error) {
						if err != nil {
							uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
							return
						}
						clearSel()
					})
				})
			})
	}
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
		kind := domain.KindExpense
		for _, txn := range app.Transactions() {
			if selAtom.Get()[txn.ID] && !txn.IsTransfer() {
				if txn.Amount.Amount > 0 {
					kind = domain.KindIncome
				}
				break
			}
		}
		categoryPickerKind.Set(kind)
		categoryPickerOpen.Set(true)
	}

	// Options for the category and owner pickers. The owner list includes the
	// explicit unassigned value so it shares the same commit path as members.
	//
	// C561: the resting option reads "Choose a category" — a prompt, not a value. It
	// used to read "No category", which is a real, destructive choice presented as the
	// default. C570: the choices carry their full path, so two leaves with the same
	// name are distinguishable here as they are in Review and Edit.
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
	// Keep the primary organization actions visible in one compact row; secondary
	// bank/report/export operations live in the overflow menu.
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
		buttonWithDisabled(checkpointSaving.Get(), []any{
			css.Class("btn btn-tool bt-danger", "txn-bulk-primary", "txn-bulk-inline-delete"), Type("button"), Attr("data-testid", "bulk-delete"),
			Attr("aria-label", uistate.T("txp3.bulkDeleteSelected")),
			Title(uistate.T("txp3.bulkDeleteSelected")),
			OnClick(bulkDelete),
		}, IfElse(checkpointSaving.Get(), Text(uistate.T("enc34.saving")), Fragment(uiw.Icon(icon.Trash, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("txp3.bulkDeleteSelected"))))),
		Button(css.Class("btn btn-tool", "bulk-clear-selection"), Type("button"), Attr("data-testid", "bulk-clear-selection"), Attr("aria-label", uistate.T("r1c.clearSelection")), Title(uistate.T("r1c.clearSelection")), OnClick(clearSelection), Text("×")),
		If(categoryPickerOpen.Get(), txnBulkCategoryPicker(categoryPickerQuery.Get(), categoryPickerKind.Get(),
			func(categoryID string) { categoryPickerOpen.Set(false); onBulkCat(categoryID) },
			func() { categoryPickerOpen.Set(false) },
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
	options := make([]txnBulkPickerOption, 0, len(members)+1)
	options = append(options, txnBulkPickerOption{Value: txnBulkUnassignedOwner, Label: uistate.T("bulk2.unassigned")})
	for _, member := range members {
		options = append(options, txnBulkPickerOption{Value: member.ID, Label: member.Name})
	}
	return options
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
	if value == txnBulkUnassignedOwner {
		if len(selected) == 1 && selected[0] == value {
			return nil
		}
		return []string{value}
	}
	result := make([]string, 0, len(selected)+1)
	found := false
	for _, owner := range selected {
		if owner == txnBulkUnassignedOwner {
			continue
		}
		if owner == value {
			found = true
			continue
		}
		result = append(result, owner)
	}
	if !found {
		result = append(result, value)
	}
	return result
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

func txnBulkAppliedOwners(selected []string) []string {
	for _, owner := range selected {
		if owner == txnBulkUnassignedOwner {
			return nil
		}
	}
	return append([]string(nil), selected...)
}

func txnBulkBarWidgetStyle() map[string]string {
	return map[string]string{
		"position": "fixed", "inset": "0", "z-index": "70", "pointer-events": "none",
		"overflow": "visible", "background": "transparent", "border": "0", "box-shadow": "none",
	}
}

func sameOwnerList(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func assignSelectedTxnOwners(app *appstate.App, selected map[string]bool, owners []string) ([]domain.Transaction, error) {
	var prior []domain.Transaction
	var writes []domain.Transaction
	for _, txn := range app.Transactions() {
		if selected[txn.ID] && !sameOwnerList(txn.Owners(), owners) {
			prior = append(prior, txn)
			txn.SetOwners(owners)
			writes = append(writes, txn)
		}
	}
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
