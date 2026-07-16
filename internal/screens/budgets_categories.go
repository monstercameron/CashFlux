// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetTrackMeta aggregates THIS calendar month's expense spend by category and by tag —
// the "N · $X" selection metadata shown in the tracked-categories/tags editor. Splits count
// per line's category; tags count the whole charge once per tag. Category values are pre-
// formatted (only for categories with spend this month); tags return count + formatted total
// keyed by lowercased tag.
func budgetTrackMeta(app *appstate.App) (catMeta map[string]string, tagCount map[string]int, tagTotal map[string]string) {
	catMeta, tagCount, tagTotal = map[string]string{}, map[string]int{}, map[string]string{}
	if app == nil {
		return
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	mStart := dateutil.MonthStart(time.Now())
	mEnd := dateutil.AddMonths(mStart, 1)
	catN, catT, tagT := map[string]int{}, map[string]int64{}, map[string]int64{}
	for _, t := range app.Transactions() {
		if !t.CountsInReports() || t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if t.Date.Before(mStart) || !t.Date.Before(mEnd) {
			continue
		}
		whole, err := rates.Convert(t.Amount.Abs(), base)
		if err != nil {
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				sm, err := rates.Convert(s.Amount.Abs(), base)
				if err != nil {
					continue
				}
				catN[s.CategoryID]++
				catT[s.CategoryID] += sm.Amount
			}
		} else {
			catN[t.CategoryID]++
			catT[t.CategoryID] += whole.Amount
		}
		for _, tg := range t.Tags {
			if k := strings.ToLower(strings.TrimSpace(tg)); k != "" {
				tagCount[k]++
				tagT[k] += whole.Amount
			}
		}
	}
	for id, n := range catN {
		catMeta[id] = fmt.Sprintf("%d · %s", n, fmtMoney(money.New(catT[id], base)))
	}
	for k, amt := range tagT {
		tagTotal[k] = fmtMoney(money.New(amt, base))
	}
	return
}

// BudgetCategoriesBody is the "tracked categories" flip modal (mounted at the shell root
// by app.BudgetCategoriesHost). It lets a budget track 1..n expense categories: check
// the categories this budget should count, and its spend becomes their combined total
// (each still rolls up its own sub-categories). Overlap is allowed — a category already
// tracked by another budget shows a soft "also in …" note. Nothing is written until Save.
func BudgetCategoriesBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetCategoriesEdit()
	budgetID := openAtom.Get()

	var budget domain.Budget
	found := false
	if app != nil {
		for _, b := range app.Budgets() {
			if b.ID == budgetID {
				budget, found = b, true
				break
			}
		}
	}

	// Seed the checklist from the budget's current tracked set.
	seed := make(map[string]bool)
	for _, id := range budget.TrackedCategoryIDs() {
		seed[id] = true
	}
	picked := ui.UseState(seed)
	toggle := func(id string) {
		m := picked.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[id] = !nm[id]
		picked.Set(nm)
	}
	// Cross-category tags this budget also tracks — a picked SET (lowercased tag keys),
	// mirroring the category checklist. A budget may track categories, tags, or both.
	tagSeed := make(map[string]bool)
	for _, tg := range budget.TrackedTags {
		if k := strings.ToLower(strings.TrimSpace(tg)); k != "" {
			tagSeed[k] = true
		}
	}
	pickedTags := ui.UseState(tagSeed)
	toggleTag := func(tag string) {
		k := strings.ToLower(strings.TrimSpace(tag))
		if k == "" {
			return
		}
		m := pickedTags.Get()
		nm := make(map[string]bool, len(m)+1)
		for kk, v := range m {
			nm[kk] = v
		}
		nm[k] = !nm[k]
		pickedTags.Set(nm)
	}

	onCancel := ui.UseEvent(Prevent(func() { openAtom.Set("") }))
	onSave := ui.UseEvent(Prevent(func() {
		var sel []string
		for _, c := range app.Categories() {
			if c.Kind == domain.KindExpense && picked.Get()[c.ID] {
				sel = append(sel, c.ID)
			}
		}
		var tags []string
		for k, on := range pickedTags.Get() {
			if on && k != "" {
				tags = append(tags, k)
			}
		}
		sort.Strings(tags)
		if len(sel) == 0 && len(tags) == 0 {
			return // Save is disabled in this state; guard anyway.
		}
		b := budget
		// Store a single category in the historical shape; only reach for CategoryIDs when
		// tracking more than one. A tag-only budget keeps no category at all.
		switch {
		case len(sel) == 0:
			b.CategoryID, b.CategoryIDs = "", nil
		case len(sel) == 1:
			b.CategoryID, b.CategoryIDs = sel[0], nil
		default:
			b.CategoryID, b.CategoryIDs = sel[0], sel
		}
		b.TrackedTags = tags // nil when the field is empty
		if err := app.PutBudget(b); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.PostNotice(uistate.T("budgets.catsSaved"), false)
		uistate.BumpDataRevision()
		openAtom.Set("")
	}))

	if !found {
		return Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted"), uistate.T("common.notReady")),
			Div(css.Class("autobudget-footer"),
				Button(css.Class("btn"), Type("button"), OnClick(onCancel), uistate.T("action.close"))))
	}

	nSel := 0
	for id, on := range picked.Get() {
		if on && id != "" {
			nSel++
		}
	}
	nTags := 0
	for k, on := range pickedTags.Get() {
		if on && k != "" {
			nTags++
		}
	}
	catMeta, tagCount, tagTotal := budgetTrackMeta(app)
	allTags := distinctTxnTags(app)

	return Div(css.Class(tw.FlexCol),
		Div(css.Class("modal-scroll", tw.FlexCol, tw.Gap3),
			Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budgetcats-metahint"), uistate.T("budgets.trackMetaHint")),
			// Categories section.
			Div(css.Class("budgettrack-section"),
				Span(css.Class("budgettrack-head"), uistate.T("budgets.catsSection")),
				ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{
					Picked: picked.Get(), OnToggle: toggle, ExcludeBudgetID: budgetID, Meta: catMeta,
				})),
			// Tags section — a searchable checklist of the tags already in use (cross-
			// category), plus "add a new tag" from the search box.
			Div(css.Class("budgettrack-section"),
				Span(css.Class("budgettrack-head"), uistate.T("budgets.tagsSection")),
				ui.CreateElement(budgetTagPicker, budgetTagPickerProps{
					Picked: pickedTags.Get(), OnToggle: toggleTag, AllTags: allTags,
					Count: tagCount, Total: tagTotal,
				}))),
		Div(css.Class("modal-foot", "autobudget-footer"),
			Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "budgetcats-count"),
				uistate.T("budgets.tracksCount",
					fmt.Sprintf("%d %s", nSel, pluralWord(nSel, "category")),
					fmt.Sprintf("%d %s", nTags, pluralWord(nTags, "tag")))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgetcats-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			buttonWithDisabled(nSel == 0 && nTags == 0, []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "budgetcats-save"), OnClick(onSave)},
				uistate.T("budgets.tracksSave"))))
}

// budgetCategoryPickerProps configures the reusable multi-category picker (used by the
// tracked-categories modal and the add/edit budget forms).
type budgetCategoryPickerProps struct {
	Picked          map[string]bool
	OnToggle        func(id string)
	ExcludeBudgetID string            // a budget id whose own tracking is ignored in the overlap note ("" = none)
	Meta            map[string]string // categoryID → this-month "N · $X" spend metadata (nil = show none)
}

// budgetCategoryPicker is the reusable "which categories does this budget track"
// control: a search box that filters a clean one-line checklist of expense categories.
// It owns only its search-query state; the parent owns the picked set. Keeping it a
// component means its search hook stays at a stable position wherever it's embedded.
func budgetCategoryPicker(props budgetCategoryPickerProps) ui.Node {
	app := appstate.Default
	query := ui.UseState("")
	onQuery := ui.UseEvent(func(v string) { query.Set(v) })

	// Which OTHER budgets track each category — for the soft overlap tag.
	otherBudget := make(map[string]string)
	if app != nil {
		for _, b := range app.Budgets() {
			if b.ID == props.ExcludeBudgetID {
				continue
			}
			for _, cid := range b.TrackedCategoryIDs() {
				if otherBudget[cid] == "" {
					otherBudget[cid] = b.Name
				}
			}
		}
	}

	q := strings.ToLower(strings.TrimSpace(query.Get()))
	var shown []domain.Category
	if app != nil {
		for _, c := range app.Categories() {
			if c.Kind != domain.KindExpense {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
				continue
			}
			shown = append(shown, c)
		}
	}

	keyOf := func(c domain.Category) any { return c.ID }
	rows := MapKeyed(shown, keyOf, func(c domain.Category) ui.Node {
		return ui.CreateElement(budgetCatRow, budgetCatRowProps{
			CategoryID: c.ID, CategoryName: c.Name, Checked: props.Picked[c.ID],
			AlsoIn: otherBudget[c.ID], Meta: props.Meta[c.ID], OnToggle: props.OnToggle,
		})
	})

	var list ui.Node
	if len(shown) == 0 {
		list = P(css.Class("muted", tw.Text13), Attr("data-testid", "budgetcats-none"), uistate.T("budgets.catsNoMatch"))
	} else {
		list = Div(css.Class("budgetcats-list"), Attr("data-testid", "budgetcats-rows"), rows)
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Input(css.Class("field"), Type("search"), Attr("data-testid", "budgetcats-search"),
			Attr("aria-label", uistate.T("budgets.catsSearch")), Placeholder(uistate.T("budgets.catsSearch")),
			Value(query.Get()), OnInput(onQuery)),
		list,
	)
}

// budgetCatRowProps drives one selectable category in the picker.
type budgetCatRowProps struct {
	CategoryID   string
	CategoryName string
	Checked      bool
	AlsoIn       string // name of another budget already tracking this category ("" = none)
	Meta         string // this-month "N · $X" spend metadata ("" = no spend this month)
	OnToggle     func(id string)
}

// budgetCatRow is one clean, one-line category checklist row: checkbox + name, with a
// subtle right-aligned "in <budget>" tag only when the category is already budgeted
// elsewhere. Its own component so its checkbox hook is never registered in a loop.
func budgetCatRow(props budgetCatRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.CategoryID) })
	rowCls := "budgetcat-row"
	if props.Checked {
		rowCls += " is-on"
	}
	return Label(ClassStr(rowCls),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgetcat-pick-"+props.CategoryID), OnChange(onToggle)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("budgetcat-name"), props.CategoryName),
		If(props.Meta != "", Span(css.Class("budgetcat-meta"), Attr("data-testid", "budgetcat-meta-"+props.CategoryID), props.Meta)),
		If(props.AlsoIn != "", Span(css.Class("budgetcat-also"), uistate.T("budgets.catsAlsoIn", props.AlsoIn))),
	)
}

// distinctTxnTags returns every tag in use across the ledger, lowercased, deduped, and
// sorted — the candidate list for the tag picker's "pre-existing tag search".
func distinctTxnTags(app *appstate.App) []string {
	if app == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range app.Transactions() {
		for _, tg := range t.Tags {
			if k := strings.ToLower(strings.TrimSpace(tg)); k != "" && !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// budgetTagPickerProps configures the searchable tracked-tags checklist.
type budgetTagPickerProps struct {
	Picked   map[string]bool   // lowercased tag → tracked
	OnToggle func(tag string)  // toggle (also how a searched new tag is added)
	AllTags  []string          // distinct existing tags (lowercased, sorted)
	Count    map[string]int    // tag → this-month transaction count
	Total    map[string]string // tag → this-month formatted total
}

// budgetTagPicker is the tags counterpart of budgetCategoryPicker: a search box over the
// tags already in use, each a checkable row with its this-month reach. Typing a tag that
// doesn't exist yet offers an "Add" row, so a budget can also track a brand-new tag. It
// owns only its search state; the parent owns the picked set.
func budgetTagPicker(props budgetTagPickerProps) ui.Node {
	query := ui.UseState("")
	onQuery := ui.UseEvent(func(v string) { query.Set(v) })
	q := strings.ToLower(strings.TrimSpace(query.Get()))

	// Candidate list: every tag in use, plus any currently-picked tag not in use (a custom
	// tag the user just added). Deduped + sorted.
	seen := map[string]bool{}
	var all []string
	for _, t := range props.AllTags {
		if !seen[t] {
			seen[t] = true
			all = append(all, t)
		}
	}
	for t, on := range props.Picked {
		if on && !seen[t] {
			seen[t] = true
			all = append(all, t)
		}
	}
	sort.Strings(all)

	var shown []string
	exact := false
	for _, t := range all {
		if q == "" || strings.Contains(t, q) {
			shown = append(shown, t)
		}
		if t == q {
			exact = true
		}
	}

	rows := MapKeyed(shown, func(t string) any { return t }, func(t string) ui.Node {
		meta := ""
		if n := props.Count[t]; n > 0 {
			meta = fmt.Sprintf("%d · %s", n, props.Total[t])
		}
		return ui.CreateElement(budgetTagRow, budgetTagRowProps{
			Tag: t, Checked: props.Picked[t], Meta: meta, OnToggle: props.OnToggle,
		})
	})

	var addRow ui.Node = Fragment()
	if q != "" && !exact {
		addRow = ui.CreateElement(budgetTagAddRow, budgetTagAddRowProps{Tag: q, OnAdd: props.OnToggle})
	}
	var body ui.Node = Div(css.Class("budgetcats-list"), Attr("data-testid", "budgettags-rows"), rows)
	if len(shown) == 0 && q == "" {
		body = P(css.Class("muted", tw.Text13), Attr("data-testid", "budgettags-none"), uistate.T("budgets.tagsNoneYet"))
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Input(css.Class("field"), Type("search"), Attr("data-testid", "budgettags-search"),
			Attr("aria-label", uistate.T("budgets.tagsSearchPh")), Placeholder(uistate.T("budgets.tagsSearchPh")),
			Value(query.Get()), OnInput(onQuery)),
		body,
		addRow,
	)
}

// budgetTagRowProps drives one selectable tag in the picker.
type budgetTagRowProps struct {
	Tag      string
	Checked  bool
	Meta     string // this-month "N · $X" ("" = no spend this month)
	OnToggle func(tag string)
}

// budgetTagRow is one clean checklist row: checkbox + #tag + this-month meta. Its own
// component so its checkbox hook is never registered in a loop.
func budgetTagRow(props budgetTagRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.Tag) })
	rowCls := "budgetcat-row"
	if props.Checked {
		rowCls += " is-on"
	}
	return Label(ClassStr(rowCls),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgettag-pick-"+props.Tag), OnChange(onToggle)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("budgetcat-name"), "#"+props.Tag),
		If(props.Meta != "", Span(css.Class("budgetcat-meta"), Attr("data-testid", "budgettag-meta-"+props.Tag), props.Meta)),
	)
}

// budgetTagAddRowProps drives the "add a new tag" row shown when the search matches none.
type budgetTagAddRowProps struct {
	Tag   string
	OnAdd func(tag string)
}

// budgetTagAddRow lets the user track a tag that isn't on any transaction yet (e.g. a tag
// they're about to start using). Its own component for a stable click hook.
func budgetTagAddRow(props budgetTagAddRowProps) ui.Node {
	onAdd := ui.UseEvent(Prevent(func() { props.OnAdd(props.Tag) }))
	return Button(css.Class("budgettag-add"), Type("button"), Attr("data-testid", "budgettag-add"),
		OnClick(onAdd), uistate.T("budgets.tagsAddNew", props.Tag))
}
