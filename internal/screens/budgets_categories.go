// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetTrackMeta aggregates the VIEWED period's expense spend by category and by tag —
// the "N · $X" selection metadata shown in the tracked-categories/tags editor. Splits count
// per line's category; tags count the whole charge once per tag. Category values are pre-
// formatted (only for categories with spend in the period); tags return count + formatted total
// keyed by lowercased tag.
//
// C607: the window is the page's selected period, not today's month. It used to
// read time.Now() unconditionally, so opening Edit tracking from a closed Jul 2026
// view in August counted August's transactions under a caption promising July's.
func budgetTrackMeta(app *appstate.App, from, to time.Time) (map[string]string, map[string]int, map[string]string) {
	if app == nil {
		return map[string]string{}, map[string]int{}, map[string]string{}
	}
	base := budgetview.BaseCurrency(app.Settings().BaseCurrency)
	metadata := budgetview.TrackingMeta(budgetLedgerForApp(app), from, to, time.Now(), base, currency.Rates{Base: base, Rates: app.Settings().FXRates})
	return metadata.Categories, metadata.TagCount, metadata.TagTotal
}

// BudgetCategoriesBody is the "tracked categories" flip modal (mounted at the shell root
// by app.BudgetCategoriesHost). It lets a budget track 1..n expense categories: check
// the categories this budget should count, and its spend becomes their combined total
// (each still rolls up its own sub-categories). Overlap is allowed — a category already
// tracked by another budget shows a soft "also in …" note. Nothing is written until Save.
func BudgetCategoriesBody(_ struct{}) ui.Node {
	app := appstate.Default
	dataRev := uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetCategoriesEdit()
	budgetID := openAtom.Get()
	trackPeriod := uistate.UsePeriod().Get()
	from, to := trackPeriod.Range()
	var appRev uint64
	var budgets []domain.Budget
	if app != nil {
		appRev = app.Rev()
		budgets = app.Budgets()
	}
	meta := ui.UseMemo(func() budgetTrackMetaResult {
		categories, count, total := budgetTrackMeta(app, from, to)
		return budgetTrackMetaResult{Categories: categories, TagCount: count, TagTotal: total, DistinctTags: distinctTxnTags(app)}
	}, app, appRev, dataRev, from.Unix(), to.Unix())
	budget, found := budgetview.FindBudget(budgets, budgetID)
	seed, tagSeed := budgetview.SelectionSeed(budget)
	picked := ui.UseState(seed)
	tags := ui.UseState(tagSeed)
	cancel := func() { openAtom.Set("") }
	save := func() {
		if app == nil {
			return
		}
		b, ok := budgetview.TrackingDraft(budget, app.Categories(), picked.Get(), tags.Get())
		if !ok {
			return
		}
		if err := app.PutBudget(b); err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
			return
		}
		uistate.PostNotice(uistate.T("budgets.catsSaved"), false)
		uistate.BumpDataRevision()
		openAtom.Set("")
	}
	if !found {
		return ui.CreateElement(budgetTrackingUnavailable, budgetTrackingUnavailableProps{Close: cancel})
	}
	return ui.CreateElement(budgetTrackingContent, budgetTrackingContentProps{
		Budget: budget, Label: trackPeriod.Label(), Meta: meta, Picked: picked.Get(), Tags: tags.Get(), Cancel: cancel, Save: save,
		Toggle:    func(id string) { picked.Set(budgetview.ToggleSelection(picked.Get(), id, false)) },
		ToggleTag: func(tag string) { tags.Set(budgetview.ToggleSelection(tags.Get(), tag, true)) },
	})
}

type budgetTrackingUnavailableProps struct{ Close func() }

func budgetTrackingUnavailable(props budgetTrackingUnavailableProps) ui.Node {
	return Div(css.Class(tw.FlexCol, tw.Gap3),
		P(css.Class("muted"), uistate.T("common.notReady")),
		Div(css.Class("autobudget-footer"), Button(css.Class("btn"), Type("button"), OnClick(Prevent(props.Close)), uistate.T("action.close"))))
}

type budgetTrackingContentProps struct {
	Budget            domain.Budget
	Label             string
	Meta              budgetTrackMetaResult
	Picked, Tags      map[string]bool
	Toggle, ToggleTag func(string)
	Cancel, Save      func()
}

func budgetTrackingContent(props budgetTrackingContentProps) ui.Node {
	return Div(css.Class(tw.FlexCol),
		Div(css.Class("modal-scroll", tw.FlexCol, tw.Gap3),
			Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budgetcats-metahint"), uistate.T("budgets.trackMetaHintPeriod", props.Label)),
			// Categories section.
			Div(css.Class("budgettrack-section"),
				Span(css.Class("budgettrack-head"), uistate.T("budgets.catsSection")),
				ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{
					Picked: props.Picked, OnToggle: props.Toggle, ExcludeBudgetID: props.Budget.ID, Meta: props.Meta.Categories, Saving: props.Budget.IsSaving(),
				})),
			// Tags section — a searchable checklist of the tags already in use (cross-
			// category), plus "add a new tag" from the search box.
			Div(css.Class("budgettrack-section"),
				Span(css.Class("budgettrack-head"), uistate.T("budgets.tagsSection")),
				ui.CreateElement(budgetTagPicker, budgetTagPickerProps{
					Picked: props.Tags, OnToggle: props.ToggleTag, AllTags: props.Meta.DistinctTags,
					Count: props.Meta.TagCount, Total: props.Meta.TagTotal,
				}))),
		ui.CreateElement(budgetTrackingFooter, budgetTrackingFooterProps{Categories: budgetview.SelectionCount(props.Picked), Tags: budgetview.SelectionCount(props.Tags), Cancel: props.Cancel, Save: props.Save}))
}

type budgetTrackingFooterProps struct {
	Categories, Tags int
	Cancel, Save     func()
}

func budgetTrackingFooter(props budgetTrackingFooterProps) ui.Node {
	presentation := budgetview.PresentTrackingFooter(props.Categories, props.Tags, uistate.T)
	return Div(css.Class("modal-foot", "autobudget-footer"),
		Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "budgetcats-count"),
			presentation.Count),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgetcats-cancel"), OnClick(Prevent(props.Cancel)), uistate.T("action.cancel")),
		buttonWithDisabled(presentation.Disabled, []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "budgetcats-save"), OnClick(Prevent(props.Save))},
			uistate.T("budgets.tracksSave")))
}

type budgetTrackMetaResult struct {
	Categories   map[string]string
	TagCount     map[string]int
	TagTotal     map[string]string
	DistinctTags []string
}

// budgetCategoryPickerProps configures the reusable multi-category picker (used by the
// tracked-categories modal and the add/edit budget forms).
type budgetCategoryPickerProps struct {
	Picked          map[string]bool
	OnToggle        func(id string)
	ExcludeBudgetID string            // a budget id whose own tracking is ignored in the overlap note ("" = none)
	Meta            map[string]string // categoryID → this-month "N · $X" spend metadata (nil = show none)
	// Saving reflects the budget's direction. A SPENDING budget offers spending
	// categories only — an income category cannot accrue against a cap, so
	// offering one would ship a budget that is broken by construction (C534). A
	// SAVING budget is the case that needs them, so it offers everything and the
	// withheld-count note stands down.
	Saving bool
}

// budgetCategoryPicker is the reusable "which categories does this budget track"
// control: a search box that filters a clean one-line checklist of expense categories.
// It owns only its search-query state; the parent owns the picked set. Keeping it a
// component means its search hook stays at a stable position wherever it's embedded.
func budgetCategoryPicker(props budgetCategoryPickerProps) ui.Node {
	app := appstate.Default
	query := ui.UseState("")
	onQuery := ui.UseEvent(func(v string) { query.Set(v) })
	var appRev uint64
	if app != nil {
		appRev = app.Rev()
	}
	cats := ui.UseMemo(func() []domain.Category {
		if app == nil {
			return nil
		}
		return app.Categories()
	}, app, appRev)

	var budgets []domain.Budget
	if app != nil {
		budgets = app.Budgets()
	}
	shown, hiddenKinds := budgetview.CategoryOptions(cats, budgets, props.ExcludeBudgetID, query.Get(), props.Saving)

	keyOf := func(c budgetview.CategoryOption) any { return c.ID }
	rows := MapKeyed(shown, keyOf, func(c budgetview.CategoryOption) ui.Node {
		return ui.CreateElement(budgetCatRow, budgetCatRowProps{
			CategoryID: c.ID, CategoryName: c.Name, Checked: props.Picked[c.ID],
			AlsoIn: c.AlsoIn, Meta: props.Meta[c.ID], OnToggle: props.OnToggle,
		})
	})

	// Say what was withheld and why, whether or not anything matched.
	kindNote := If(hiddenKinds > 0, P(css.Class("muted", tw.Text13), Attr("data-testid", "budgetcats-kind-note"),
		uistate.T("budgets.catsIncomeHidden", hiddenKinds)))

	var list ui.Node
	if len(shown) == 0 {
		list = P(css.Class("muted", tw.Text13), Attr("data-testid", "budgetcats-none"), uistate.T("budgets.catsNoMatch"))
	} else {
		list = Div(css.Class("budgetcats-list"), Attr("data-testid", "budgetcats-rows"), rows)
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Input(css.Class("field"), Type("search"), Attr("data-testid", "budgetcats-search"),
			Attr("aria-label", uistate.T("budgets.catsSearch")), Placeholder(uistate.T("budgets.catsSearch")), OnInput(onQuery), uiw.FieldValue(query.Get())),
		list,
		kindNote,
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
	rowCls := budgetview.CheckedRowClass(props.Checked)
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
	return budgetview.DistinctTags(app.Transactions())
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
	shown, q, exact := budgetview.TagOptions(props.AllTags, props.Picked, query.Get())

	rows := MapKeyed(shown, func(t string) any { return t }, func(t string) ui.Node {
		meta := budgetview.CountTotalLabel(props.Count[t], props.Total[t])
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
			Attr("aria-label", uistate.T("budgets.tagsSearchPh")), Placeholder(uistate.T("budgets.tagsSearchPh")), OnInput(onQuery), uiw.FieldValue(query.Get())),
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
	rowCls := budgetview.CheckedRowClass(props.Checked)
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
