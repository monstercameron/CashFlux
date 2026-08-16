// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"
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
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
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
func dateField(label, value string, onInput ui.Handler) ui.Node {
	return withFieldLabel(label, Input(css.Class("field"), Type("date"),
		Attr("aria-label", label), Value(value), OnInput(onInput)))
}

func amountField(label, placeholder, value string, onInput ui.Handler) ui.Node {
	return withFieldLabel(label, Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
		Attr("aria-label", label), Placeholder(placeholder), Value(value), OnInput(onInput)))
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

// filterPillProps configures one toggle pill in a multi-select filter group. It is its
// own component so its OnClick hook sits at a stable position (the pill list is
// variable-length — see the framework loop-hook gotcha).
type filterPillProps struct {
	Field    txnfilter.FilterField
	Value    string
	Label    string
	Selected bool
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
	return Button(css.Class(cls), Type("button"), Attr("aria-pressed", pressed), OnClick(onClick), props.Label)
}

// filterMultiGroup renders one filter dimension as a labelled group of toggle pills —
// the multi-select control replacing the old single <select>. Multiple values can be
// on at once (OR-within the dimension). The empty-value "All" option is skipped (an
// empty set already means "all").
func filterMultiGroup(label string, field txnfilter.FilterField, selected []string, opts []uiw.SelectOption, onToggle func(txnfilter.FilterField, string)) ui.Node {
	return ui.CreateElement(filterGroup, filterGroupProps{
		Label: label, Field: field, Selected: selected, Opts: opts, OnToggle: onToggle,
	})
}

// filterGroupCollapseThreshold is the option count above which a filter dimension
// collapses by default — beyond this, showing every pill turns the panel into a
// chip wall that buries the ledger (a Category dimension has ~25, Tags ~40).
const filterGroupCollapseThreshold = 12

type filterGroupProps struct {
	Label    string
	Field    txnfilter.FilterField
	Selected []string
	Opts     []uiw.SelectOption
	OnToggle func(txnfilter.FilterField, string)
}

// filterGroup renders one categorical filter dimension as a group of toggle pills.
// Large dimensions collapse by default to just their SELECTED pills plus a "Show
// all N" disclosure, so the filter panel stays scannable instead of rendering a
// flat wall of every account/category/tag at once. It owns its expand state (a
// stable per-dimension component, not a loop-level hook).
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

	// Collapsed + large: show only the selected pills (so the active filter stays
	// visible); expanded or small: show them all.
	shown := props.Opts
	if collapsible && !open {
		kept := make([]uiw.SelectOption, 0, selCount)
		for _, o := range props.Opts {
			if o.Value != "" && containsStr(props.Selected, o.Value) {
				kept = append(kept, o)
			}
		}
		shown = kept
	}

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
		discBtn = Button(css.Class("filter-group-disc"), Type("button"),
			Attr("data-testid", "filter-showall-"+string(props.Field)),
			Attr("aria-expanded", ariaBool(open)), OnClick(toggle), discLabel)
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

// toolbarIconBtn renders one transactions-toolbar action as a standard labeled
// button (.btn-tool): a slightly-grayed leading glyph followed by an always-visible
// text label, so the action reads at a glance instead of needing a hover to decode a
// bare icon. The label also serves as the aria-label. variant is "" (neutral),
// "primary" (accent — the Add action), or "danger" (delete).
func toolbarIconBtn(testID string, ic icon.Name, label string, onClick ui.Handler, variant string) ui.Node {
	return toolbarIconBtnOpen(testID, ic, label, onClick, variant, false)
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

func txnToolbarWidget(props txnToolbarProps) ui.Node {
	app := props.App
	filterAtom := uistate.UseTxFilter()
	selAtom := uistate.UseTxnSelection()
	colsModalAtom := uistate.UseTxnColsModalOpen()
	smartCatAtom := uistate.UseTxnSmartCatOpen()
	openReview := ui.UseEvent(Prevent(func() { uistate.OpenReviewInbox() }))
	importPanelAtom := uistate.UseImportPanelOpen()
	dupModalAtom := uistate.UseDuplicatesModalOpen()
	// C363: the Rules workbench (/rules) is a full first-class surface, but nothing
	// on Transactions pointed there. A labeled toolbar entry (with the active-rule
	// count) makes it reachable in one visible click.
	nav := router.UseNavigate()
	openRules := ui.UseEvent(Prevent(func() { txnLeaveFor("/rules"); nav.Navigate(uistate.RoutePath("/rules")) }))

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

	accounts := app.Accounts()
	categories := app.Categories()
	members := app.Members()
	txns := app.Transactions()
	reviewN := reviewqueue.Count(txns)
	ruleCount := len(app.Rules()) // C363: active-rule count for the toolbar Rules entry

	accName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		accName[a.ID] = a.Name
	}
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	memberName := make(map[string]string, len(members))
	for _, m := range members {
		memberName[m.ID] = m.Name
	}

	setFilter := func(mut func(*uistate.TxFilter)) { setTxFilterOn(filterAtom, mut) }
	clearAllFilters := func() {
		cleared := uistate.TxFilter{}.Normalize()
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
	// Search gets a longer debounce than the numeric fields: it's the most-typed filter
	// and re-filtering the whole ledger on each pause is the heaviest, so 400ms coalesces
	// more keystrokes (including deliberate typing that pauses past the shorter delay).
	onFilterText := func(v string) { debFilterD("text", 400*time.Millisecond, func(x *uistate.TxFilter) { x.Text = v }) }
	onFilterAmountMin := ui.UseEvent(func(v string) { debFilter("amtmin", func(x *uistate.TxFilter) { x.AmountMin = v }) })
	onFilterAmountMax := ui.UseEvent(func(v string) { debFilter("amtmax", func(x *uistate.TxFilter) { x.AmountMax = v }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })
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
	doExportCSV := func() {
		rows := txnfilter.Apply(app.Transactions(), filterAtom.Get())
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
		shown := txnfilter.Apply(app.Transactions(), filterAtom.Get())
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
	accOpts := withAllOption(uistate.T("transactions.allAccounts"),
		uiw.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, ""))
	catOpts := withAllOption(uistate.T("transactions.allCategories"),
		uiw.OptionsFrom(categories, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name }, ""))
	memberOpts := withAllOption(uistate.T("transactions.allMembers"),
		uiw.OptionsFrom(members, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, ""))
	sourceOpts := withAllOption(uistate.T("transactions.allSources"),
		uiw.OptionsFrom(domain.AllTxnSources, func(s domain.TxnSource) string { return string(s) }, func(s domain.TxnSource) string { return s.Label() }, ""))

	tagSet := map[string]struct{}{}
	for _, t := range txns {
		for _, tg := range t.Tags {
			if s := strings.TrimSpace(tg); s != "" {
				tagSet[s] = struct{}{}
			}
		}
	}
	tagList := make([]string, 0, len(tagSet))
	for tg := range tagSet {
		tagList = append(tagList, tg)
	}
	sort.Strings(tagList)
	tagOpts := withAllOption(uistate.T("transactions.allTags"),
		uiw.OptionsFrom(tagList, func(s string) string { return s }, func(s string) string { return s }, ""))

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
			// A text filter — whether just typed or carried in from earlier work —
			// reads as an intentional, one-click-clearable "Filtering: …" chip so a
			// retained search never looks like a stray leftover (detail-lane 2 #3).
			return uistate.T("transactions.chipFiltering", af.Value)
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
			return uistate.T("transactions.chipAmountMin", af.Value)
		case txnfilter.FieldAmountMax:
			return uistate.T("transactions.chipAmountMax", af.Value)
		case txnfilter.FieldFrom:
			return uistate.T("transactions.chipFrom", af.Value)
		case txnfilter.FieldTo:
			return uistate.T("transactions.chipTo", af.Value)
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
	active := fOwn.ActiveFilters()
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
	chips := make([]uiw.Chip, 0, len(active))
	for _, af := range active {
		// Key encodes field + value so a per-value chip ✕ removes just that value
		// (RemoveValue), not the whole dimension.
		chips = append(chips, uiw.Chip{Key: string(af.Field) + chipKeySep + af.Value, Label: chipLabel(af)})
	}

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
			if parsed.From != "" {
				x.From = parsed.From
			}
			if parsed.To != "" {
				x.To = parsed.To
			}
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
			for _, af := range parsed.ActiveFilters() {
				previews = append(previews, chipLabel(af))
			}
			preview := strings.Join(previews, " · ")
			interpretRow = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Text13),
				Style(map[string]string{"padding": "0.4rem 0.1rem 0.1rem"}),
				Attr("role", "status"),
				Span(css.Class(tw.TextDim, tw.ShrinkO), uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				Button(css.Class("btn", "btn-tool"), Type("button"),
					Attr("data-testid", "nl-interpret"),
					Attr("aria-label", uistate.T("transactions.nlInterpretAria", preview)),
					OnClick(applyNL),
					Text(uistate.T("transactions.nlInterpret", preview)),
				),
			)
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
				valControl = Input(css.Class("field"), Type("text"), Attr("aria-label", valAria), Placeholder(valAria), Value(f.CustomVal), OnInput(onFilterCustomValText))
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
	monthFrom := dateutil.FormatDate(monthStart)
	monthEndEx := dateutil.AddMonths(monthStart, 1)
	monthTo := dateutil.FormatDate(monthEndEx.AddDate(0, 0, -1))
	// Preset match counts — like the toolbar's "Review N", so each chip shows its
	// payoff before the click.
	//
	// C568: counted WITHIN the active scope (search, member, period, other filters),
	// not over the whole ledger. A chip beside a search that reads "Large 1665" while
	// the search shows twelve rows is describing a population no click can produce.
	// The arithmetic lives in txnfilter.CountPresets, tested on native Go.
	presetN := txnfilter.CountPresets(txns, f, txnfilter.PresetScope{
		ReviewTag:  reviewqueue.ReviewTag,
		LargeFor:   func(t domain.Transaction) int64 { return currency.MinorFromMajor(100, t.Amount.Currency) },
		MonthStart: monthStart,
		MonthEnd:   monthEndEx,
	})
	uncatN, reviewN2, largeN, monthN := presetN.Uncategorized, presetN.NeedsReview, presetN.Large, presetN.ThisMonth
	presetsRow := Div(css.Class("txn-presets"), Attr("data-testid", "txn-presets"),
		Span(css.Class("txn-presets-label"), uistate.T("transactions.presetsLabel")),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetUncategorized"), TestID: "txn-preset-uncat", Active: f.Uncategorized, Count: uncatN,
			OnToggle: func() { setFilter(func(x *uistate.TxFilter) { x.Uncategorized = !x.Uncategorized }) }}),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetNeedsReview"), TestID: "txn-preset-review", Active: f.Tag == reviewqueue.ReviewTag, Count: reviewN2,
			OnToggle: func() {
				setFilter(func(x *uistate.TxFilter) {
					if x.Tag == reviewqueue.ReviewTag {
						x.Tag = ""
					} else {
						x.Tag = reviewqueue.ReviewTag
					}
				})
			}}),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetLarge"), TestID: "txn-preset-large", Active: f.AmountMin == txnLargeThreshold, Count: largeN,
			OnToggle: func() {
				setFilter(func(x *uistate.TxFilter) {
					if x.AmountMin == txnLargeThreshold {
						x.AmountMin = ""
					} else {
						x.AmountMin = txnLargeThreshold
					}
				})
			}}),
		ui.CreateElement(txnPresetChip, txnPresetProps{
			Label: uistate.T("transactions.presetThisMonth"), TestID: "txn-preset-month", Active: f.From == monthFrom && f.To == monthTo, Count: monthN,
			OnToggle: func() {
				setFilter(func(x *uistate.TxFilter) {
					if x.From == monthFrom && x.To == monthTo {
						x.From, x.To = "", ""
					} else {
						x.From, x.To = monthFrom, monthTo
					}
				})
			}}),
		// One quiet line saying what the numbers count, so the chips are self-describing
		// rather than relying on the user inferring the scope from the rows below.
		Span(css.Class("txn-presets-note"), Attr("data-testid", "txn-presets-note"),
			uistate.T("transactions.presetScopeNote")),
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
			dateField(uistate.T("transactions.fromDate"), f.From, onFilterFrom),
			dateField(uistate.T("transactions.toDate"), f.To, onFilterTo),
			amountField(uistate.T("transactions.filterAmountMin"), uistate.T("transactions.filterAmountMinPh"), f.AmountMin, onFilterAmountMin),
			amountField(uistate.T("transactions.filterAmountMax"), uistate.T("transactions.filterAmountMaxPh"), f.AmountMax, onFilterAmountMax),
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
	dupCount := dedupe.Count(dedupe.FindDuplicates(props.Shown))
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
		Items: []uiw.OverflowMenuItem{
			{Label: regLabel, Icon: icon.List, TestID: "txn-register-btn", OnSelect: onRegister, Hidden: !(singleAcct && !calActive)},
			{Label: importBtnLabel, Icon: icon.Upload, TestID: "txn-import-btn", OnSelect: func() { importPanelAtom.Set(true) }},
			{Label: dupBtnLabel, Icon: icon.Copy, TestID: "txn-dupes-btn", OnSelect: func() { dupModalAtom.Set(true) }},
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
		Search:       f.Text,
		SearchLabel:  uistate.T("transactions.searchPlaceholder"),
		OnSearch:     onFilterText,
		FiltersLabel: uistate.T("transactions.filters"),
		FiltersTitle: uistate.T("transactions.filtersTitle"),
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
			// The budget-origin chip owns the whole drill scope: dropping it clears
			// the categories, tags and period the budget applied, and forgets the
			// origin — not one category out of the set (C585).
			if field == budgetDrillChipKey {
				uistate.ClearBudgetDrill()
				clearAllFilters()
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
		// Standard labeled toolbar buttons (.btn-tool): a slightly-grayed leading glyph
		// plus an always-visible text label, so each action reads at a glance instead of
		// needing a hover to decode a bare icon. All left-justified as one group, with the
		// primary "+ Add transaction" LAST so it anchors the right end of the group.
		Actions: []ui.Node{
			// Review inbox (CG-S2): the guided triage entry point, shown only when
			// something needs review, with a live count so the backlog is visible.
			// C575: the review backlog is counted across the WHOLE household and ignores
			// every filter on this page, so beside a filtered ledger its number belongs
			// to a different population than everything else on screen. The button keeps
			// the count — it is the backlog's whole point — and says which population it
			// is measuring, on hover and to assistive tech.
			If(reviewN > 0, toolbarIconBtnTitled("txn-review-btn", icon.ScanLine,
				uistate.T("review.button", reviewN), uistate.T("transactions.reviewScopeTitle", reviewN), openReview, "")),
			// C363: first-class Rules entry — labeled, with the active-rule count, so
			// the auto-categorization workbench is one visible click from the ledger.
			toolbarIconBtn("txn-rules-btn", icon.Workflow, uistate.T("transactions.rulesButton", ruleCount), openRules, ""),
			// There is no second "Clear filters" button here any more. It did exactly
			// what the chip row's reset does, so the page offered two differently-worded
			// controls ("Clear filters" / "Clear all filters") for one action, sitting a
			// few centimetres apart — the ambiguity C574 is about. The reset now lives
			// with the chips it removes, and counts them.
			// Select-all, Calendar, and Register moved into the "⋯ More" overflow (2026-07
			// command-bar consolidation): the resting row keeps only the everyday verbs plus
			// the primary Add, so the page reads as a focused workspace, not a tool pile.
			// Saved views / watchlists (TX3): list saved filter sets with their live
			// count + total, one-tap apply, save-current, pin-to-dashboard, and per-view
			// amount alerts. Own component so its popover + list hooks stay stable.
			// fOwn, not f: "Save current view" writes the filter it is handed, and the
			// member lens is a transient top-bar perspective, not part of the view the
			// user is naming. Saving the layered filter would bake whoever the top bar
			// happened to show into the saved view permanently, invisibly.
			ui.CreateElement(TxnSavedViewsMenu, txnSavedViewsMenuProps{App: app, Filter: fOwn, Rates: props.Rates, Base: props.Base}),
			// Secondary utilities (Import, Duplicates, Categorize, Export CSV, Columns)
			// folded into the "⋯ More" overflow built above.
			moreMenu,
			// Primary action last → right end of the left-justified group.
			//
			// C573: the page offered three ways to start a transaction — this button,
			// the top bar's "+", and "Add something else" — and all three opened the
			// same neutral form, so none of them said what was about to be created.
			// This is now the page's one primary, with a caret holding the three
			// creation kinds by name. Pressing the button still opens the form in one
			// click for the common case; pressing the caret says which kind first, and
			// the form arrives already set that way, so the label is a promise the
			// destination keeps.
			Div(css.Class("btn-split"),
				toolbarIconBtn("txn-add-btn", icon.Plus, uistate.T("transactions.addTitle"), onAdd, "primary"),
				uiw.OverflowMenu(uiw.OverflowMenuProps{
					TriggerLabel:  uistate.T("transactions.addKindLabel"),
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
		},
	})

	var shownNet int64
	for _, t := range props.Shown {
		if c, err := props.Rates.Convert(t.Amount, props.Base); err == nil {
			shownNet += c.Amount
		}
	}
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
	countLine := ui.CreateElement(txnCountLine, txnCountLineProps{
		Shown: len(props.Shown), Total: len(txns),
		Net: money.New(shownNet, props.Base), Lens: lensName,
	})

	// Status-glyph legend: the compact ✓✓ / ✓ / • markers a row wears (reconciled /
	// cleared / needs review) are otherwise undecoded shape+color, so a small key
	// spells each one out — labelled, not color-or-shape alone (a11y).
	//
	// It is shown only when the rows actually WEAR those markers. With the Status
	// column on (the default since C578) each row says its state in words and the
	// glyphs are gone, so a key to them would explain symbols that are not on screen
	// — and cost a line of chrome above the ledger to do it.
	//
	// The hook is read into a local FIRST: `len(props.Shown) > 0 && !UseTxnCols()…`
	// short-circuits, so on a render with no rows the hook would not run at all and
	// every atom slot after it would shift.
	statusColOn := uistate.UseTxnCols().Get().Status
	var legend ui.Node = Fragment()
	if len(props.Shown) > 0 && !statusColOn {
		legend = txnStatusLegend()
	}
	// Count and legend share one row (C582): they are both "how to read the rows
	// below", and above the ledger every row of chrome is a row the user scrolls
	// past to reach the thing they came for.
	footRow := Div(css.Class("txn-toolbar-foot"), countLine, legend)

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		// C560: the ledger's own period + view bar sits directly under the toolbar, so
		// the scope the rows are in is stated on the page that shows them rather than
		// by a top-bar pill reading different state.
		Body: Div(toolbar, ui.CreateElement(txnScopeBar, struct{}{}), interpretRow, footRow),
	})
}

// txnStatusLegend is the compact key for the ledger's row status glyphs. Each item
// pairs the exact glyph a row renders (✓✓ reconciled, ✓ cleared, • needs review)
// with a plain-English label and the same full-sentence tooltip the row badge uses,
// so the markers never rely on shape or color alone. It's a single quiet line, kept
// out of the resting toolbar rows so the ledger stays close to the top.
func txnStatusLegend() ui.Node {
	item := func(glyph, label, title, testID string) ui.Node {
		return Span(css.Class("txn-legend-item"), Attr("data-testid", testID), Attr("title", title),
			Span(css.Class("txn-legend-glyph"), Attr("aria-hidden", "true"), glyph),
			Span(css.Class("txn-legend-text"), label))
	}
	return Div(css.Class("txn-legend"), Attr("data-testid", "txn-status-legend"),
		Attr("role", "note"), Attr("aria-label", uistate.T("acctxn.legendAria")),
		Span(css.Class("txn-legend-label"), uistate.T("acctxn.legendLabel")),
		item("✓✓", uistate.T("acctxn.legendReconciled"), uistate.T("transactions.reconciledBadgeTitle"), "txn-legend-reconciled"),
		item("✓", uistate.T("acctxn.legendCleared"), uistate.T("transactions.clearedBadgeTitle"), "txn-legend-cleared"),
		item("•", uistate.T("acctxn.legendNeedsReview"), uistate.T("transactions.needsReviewBadgeTitle"), "txn-legend-review"),
	)
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
	status := ui.UseEvent(func() { c := colsAtom.Get(); c.Status = !c.Status; apply(c) })

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
		// C578: the one column whose purpose is not obvious from its name gets a line
		// saying what it buys you — it is the alternative to decoding the row glyphs.
		row(uistate.T("transactions.colStatus"), uistate.T("transactions.colStatusHint"), "col-toggle-status", cols.Status, status),
	)
}

// txnBulkBarProps carries the app the bulk tile mutates.
type txnBulkBarProps struct {
	App *appstate.App
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
	bulkMemAtom := uistate.UseTxnBulkMember()
	undoAtom := uistate.UseTxnUndo()

	clearSel := func() {
		selAtom.Set(map[string]bool{})
		anchorAtom.Set("")
	}

	bulkSetCleared := func(val bool) {
		sel := selAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.Cleared != val {
				prior = append(prior, t)
			}
		}
		app.BulkMutate(func() {
			for _, t := range app.Transactions() {
				if !sel[t.ID] || t.Cleared == val {
					continue
				}
				t.Cleared = val
				if err := app.PutTransaction(t); err != nil {
					uistate.PostNotice(uistate.T("transactions.bulkClearErr", err.Error()), true)
				}
			}
		})
		opKey := "transactions.bulkOpCleared"
		if !val {
			opKey = "transactions.bulkOpUncleared"
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T(opKey, len(prior)), Prior: prior})
		clearSel()
		uistate.BumpDataRevision()
		// C364: tell the undo story at the moment of risk.
		if len(prior) > 0 {
			postUndoStory(uistate.T(opKey, len(prior)))
		}
	}
	bulkMarkCleared := ui.UseEvent(Prevent(func() { bulkSetCleared(true) }))
	bulkMarkUncleared := ui.UseEvent(Prevent(func() { bulkSetCleared(false) }))

	// TXC-1: bulk exclude / include the selected transactions in budgets & reports —
	// the main use case (a batch of reimbursements or internal transfers), so it
	// doesn't have to be done one row at a time.
	bulkSetExclude := func(val bool) {
		sel := selAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.ExcludeFromReports != val {
				prior = append(prior, t)
			}
		}
		app.BulkMutate(func() {
			for _, t := range app.Transactions() {
				if !sel[t.ID] || t.ExcludeFromReports == val {
					continue
				}
				t.ExcludeFromReports = val
				if err := app.PutTransaction(t); err != nil {
					uistate.PostNotice(err.Error(), true)
				}
			}
		})
		opKey := "transactions.bulkOpExcluded"
		if !val {
			opKey = "transactions.bulkOpIncluded"
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T(opKey, len(prior)), Prior: prior})
		clearSel()
		uistate.BumpDataRevision()
		// C364: undo story for bulk exclude/include.
		if len(prior) > 0 {
			postUndoStory(uistate.T(opKey, len(prior)))
		}
	}
	bulkExclude := ui.UseEvent(Prevent(func() { bulkSetExclude(true) }))
	bulkInclude := ui.UseEvent(Prevent(func() { bulkSetExclude(false) }))

	// C561: "assign a category" and "clear the category" used to be the same button
	// with the same blank default, so Categorize on a fresh selection wrote "" to every
	// selected row — an immediate, unasked-for mutation whose only visible trace was the
	// rows going uncategorized. They are two actions now: applyCategory needs a real
	// choice (the button is disabled until there is one), and clearing is its own,
	// confirmed action that says what it costs.
	applyBulkCategory := func(cid string) {
		sel := selAtom.Get()
		// #55: checkpoint before the bulk recategorize.
		uistate.SaveCheckpoint(uistate.T("ckpt.beforeBulkRecat", plural(len(sel), "transaction")))
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && !t.IsTransfer() && t.CategoryID != cid {
				prior = append(prior, t)
			}
		}
		app.BulkMutate(func() {
			for _, t := range app.Transactions() {
				if !sel[t.ID] || t.IsTransfer() || t.CategoryID == cid {
					continue
				}
				t.CategoryID = cid
				if err := app.PutTransaction(t); err != nil {
					uistate.PostNotice(uistate.T("transactions.bulkRecatErr", err.Error()), true)
				}
			}
		})
		// The result names the count AND the category it landed in — "Filed 12 under
		// Housing > Mortgage" — so the commit is verifiable without re-reading the rows.
		opLabel := uistate.T("transactions.bulkOpClearedCat", len(prior))
		if cid != "" {
			opLabel = uistate.T("transactions.bulkCatApplied", len(prior), bulkCatLabel(app, cid))
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: opLabel, Prior: prior})
		clearSel()
		bulkCatAtom.Set("")
		uistate.BumpDataRevision()
		// C364: undo story for bulk recategorize.
		if len(prior) > 0 {
			postUndoStory(opLabel)
		}
	}
	bulkRecategorize := ui.UseEvent(Prevent(func() {
		cid := bulkCatAtom.Get()
		if cid == "" {
			// Belt and braces: the button is disabled, so this is only reachable via a
			// keyboard/e2e path. Say what is missing instead of writing a blank category.
			uistate.PostNotice(uistate.T("transactions.bulkCatDisabled"), true)
			return
		}
		applyBulkCategory(cid)
	}))
	bulkClearCategory := ui.UseEvent(Prevent(func() {
		n := len(selAtom.Get())
		uistate.ConfirmModalLabeled(
			uistate.T("transactions.bulkClearCatConfirm", n),
			uistate.T("transactions.bulkClearCat"),
			true,
			func(ok bool) {
				if ok {
					applyBulkCategory("")
				}
			},
		)
	}))

	bulkAssignMember := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		mid := bulkMemAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.MemberID != mid {
				prior = append(prior, t)
			}
		}
		app.BulkMutate(func() {
			for _, t := range app.Transactions() {
				if !sel[t.ID] || t.MemberID == mid {
					continue
				}
				t.MemberID = mid
				if err := app.PutTransaction(t); err != nil {
					uistate.PostNotice(uistate.T("transactions.bulkAssignErr", err.Error()), true)
				}
			}
		})
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("transactions.bulkOpAssigned", len(prior)), Prior: prior})
		clearSel()
		bulkMemAtom.Set("")
		uistate.BumpDataRevision()
		// C364: undo story for bulk member assignment.
		if len(prior) > 0 {
			postUndoStory(uistate.T("transactions.bulkOpAssigned", len(prior)))
		}
	}))

	// XC1: group the selected transactions into one logical purchase (an order
	// that posted as several card charges). Order-group members keep their atoms;
	// the link is a read-model overlay. First selected (in ledger order) is the
	// group's primary/original.
	bulkGroupOrder := ui.UseEvent(Prevent(func() {
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
		if err := app.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: ids}); err != nil {
			uistate.PostNotice(uistate.T("txnlinks.groupErr", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("txnlinks.grouped", len(ids)), false)
		clearSel()
		uistate.BumpDataRevision()
	}))

	exportSelected := ui.UseEvent(Prevent(func() {
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
	}))

	bulkDelete := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		count := len(sel)
		uistate.ConfirmModal(uistate.T("transactions.bulkDeleteConfirm", count), true, func(ok bool) {
			if !ok {
				return
			}
			// #55: whole-dataset checkpoint before the bulk delete (restorable
			// from Settings → Data even after the session undo stack is gone).
			uistate.SaveCheckpoint(uistate.T("ckpt.beforeBulkDelete", plural(count, "transaction")))
			var prior []domain.Transaction
			for _, t := range app.Transactions() {
				if sel[t.ID] {
					prior = append(prior, t)
				}
			}
			ids := make([]string, 0, len(sel))
			for id := range sel {
				ids = append(ids, id)
			}
			if err := app.DeleteTransactionsBulk(ids); err != nil {
				uistate.PostNotice(err.Error(), true)
			}
			undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("transactions.bulkOpDeleted", len(prior)), Prior: prior})
			clearSel()
			uistate.BumpDataRevision()
			// C364: undo story naming the count + the reversal path (Ctrl+Z / Activity).
			postUndoStory(uistate.T("transactions.bulkOpDeleted", len(prior)))
		})
	}))

	clearSelection := ui.UseEvent(Prevent(clearSel))

	n := len(selAtom.Get())
	onBulkCat := ui.UseEvent(func(e ui.Event) { bulkCatAtom.Set(e.GetValue()) })
	onBulkMem := ui.UseEvent(func(e ui.Event) { bulkMemAtom.Set(e.GetValue()) })

	// Standard `.field` selects for the category / member to apply.
	//
	// C561: the resting option reads "Choose a category" — a prompt, not a value. It
	// used to read "No category", which is a real, destructive choice presented as the
	// default. C570: the choices carry their full path, so two leaves with the same
	// name are distinguishable here as they are in Review and Edit.
	catSel := bulkCatAtom.Get()
	catOpts := []any{css.Class("field"), Attr("data-testid", "bulk-category-select"), Attr("aria-label", uistate.T("transactions.categoryToApply")), OnChange(onBulkCat),
		Option(Value(""), SelectedIf(catSel == ""), uistate.T("transactions.bulkPickCategory"))}
	for _, o := range CategoryPickOptions(app.Categories()) {
		catOpts = append(catOpts, Option(Value(o.Value), SelectedIf(catSel == o.Value), o.Label))
	}
	memSel := bulkMemAtom.Get()
	memOpts := []any{css.Class("field"), Attr("data-testid", "bulk-member-select"), Attr("aria-label", uistate.T("transactions.memberToAssign")), OnChange(onBulkMem),
		Option(Value(""), SelectedIf(memSel == ""), uistate.T("transactions.bulkNoMember"))}
	for _, m := range app.Members() {
		memOpts = append(memOpts, Option(Value(m.ID), SelectedIf(memSel == m.ID), m.Name))
	}

	// Bulk-action bar: the app's standard toolbar buttons (glyph + short label — the same
	// `.btn btn-tool` the toolbar above uses) and standard `.field` selects. It wraps to a
	// second row on narrow widths rather than scrolling or stacking into paragraphs.
	body := Div(css.Class("bulk-bar", tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2),
		Span(css.Class("bulk-count", tw.ShrinkO), Attr("aria-label", uistate.T("transactions.selected", plural(n, "transaction"))), uistate.T("transactions.bulkSelectedShort", n)),
		Select(catOpts...),
		// Categorize stays inert until a category is chosen, and says why on hover /
		// to assistive tech. "Clear category" is the separate, confirmed way to make a
		// selection uncategorized — the thing the blank default used to do by accident.
		toolbarIconBtnState("bulk-apply-category", icon.Check, uistate.T("transactions.bulkCatShort"), bulkRecategorize, "",
			catSel == "", uistate.T("transactions.bulkCatDisabled")),
		toolbarIconBtnState("bulk-clear-category", icon.Ban, uistate.T("transactions.bulkClearCat"), bulkClearCategory, "",
			false, uistate.T("transactions.bulkClearCatTitle")),
		If(len(app.Members()) > 0, Select(memOpts...)),
		If(len(app.Members()) > 0, toolbarIconBtn("bulk-assign-member", icon.Users, uistate.T("transactions.bulkAssignShort"), bulkAssignMember, "")),
		toolbarIconBtn("bulk-mark-cleared", icon.CheckCircle, uistate.T("transactions.bulkClearedShort"), bulkMarkCleared, ""),
		toolbarIconBtn("bulk-mark-uncleared", icon.Ban, uistate.T("transactions.bulkUnclearedShort"), bulkMarkUncleared, ""),
		toolbarIconBtn("bulk-exclude", icon.Ban, uistate.T("transactions.bulkExcludeShort"), bulkExclude, ""),
		toolbarIconBtn("bulk-include", icon.Check, uistate.T("transactions.bulkIncludeShort"), bulkInclude, ""),
		toolbarIconBtn("bulk-group-order", icon.Box, uistate.T("transactions.bulkGroupShort"), bulkGroupOrder, ""),
		toolbarIconBtn("bulk-export-selected", icon.ArrowDown, uistate.T("transactions.bulkExportShort"), exportSelected, ""),
		toolbarIconBtn("bulk-delete", icon.Close, uistate.T("transactions.bulkDeleteShort"), bulkDelete, "danger"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "bulk-clear-selection"), OnClick(clearSelection), uistate.T("transactions.clearSelection")),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-bulkbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// txnUndoBarProps carries the app the undo tile restores into.
type txnUndoBarProps struct {
	App *appstate.App
}

// txnUndoBarWidget is the txn-undobar tile, shown by the host while a bulk op can be
// undone. It restores the snapshot the last op captured and clears the pending undo.
func txnUndoBarWidget(props txnUndoBarProps) ui.Node {
	undoAtom := uistate.UseTxnUndo()
	snap := undoAtom.Get()
	nav := router.UseNavigate()

	undoLastBulk := ui.UseEvent(Prevent(func() {
		s := undoAtom.Get()
		if len(s.Prior) == 0 {
			return
		}
		if err := props.App.RestoreTransactions(s.Prior); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		undoAtom.Set(uistate.BulkSnapshot{})
		uistate.BumpDataRevision()
	}))
	// C364: a working "View in Activity" link right on the undo bar, so the full
	// change history (with per-change undo) is one click from the bulk op.
	viewActivity := ui.UseEvent(Prevent(func() { txnLeaveFor("/activity"); nav.Navigate(uistate.RoutePath("/activity")) }))

	body := Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Span(css.Class("muted"), uistate.T("transactions.bulkUndoBanner", snap.Label)),
		Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.undoTitle")), Title(uistate.T("transactions.undoTitle")), OnClick(undoLastBulk), uistate.T("transactions.undoButton")),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-undobar-activity"), Attr("aria-label", uistate.T("activity.viewTitle")), Title(uistate.T("activity.viewTitle")), OnClick(viewActivity), uistate.T("activity.viewLink")),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-undobar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
