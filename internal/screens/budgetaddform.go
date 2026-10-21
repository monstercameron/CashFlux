// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgetform"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func budgetCountKey(prefix string, count int) string { return budgetform.CountKey(prefix, count) }
func budgetAddNameForCategory(current, previous, next string) string {
	return budgetform.NameForCategory(current, previous, next)
}

func budgetAddCategoryOptions(categories []domain.Category, budgets []domain.Budget, period, owner string) []uiw.PickerOption {
	options := budgetform.CategoryOptions(categories, budgets, period, owner)
	out := make([]uiw.PickerOption, 0, len(options))
	for _, o := range options {
		option := uiw.PickerOption{Value: o.ID, Label: o.Name, Disabled: o.Disabled}
		if o.Disabled {
			option.Secondary = uistate.T("budgets.categoryAlreadyBudgeted")
		}
		out = append(out, option)
	}
	return out
}

// BudgetAddFormProps configures the BudgetAddForm component.
type BudgetAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
	// Seed pre-fills the form when the modal was opened from another surface
	// ("Budget this" on an unbudgeted category). Zero value = a blank form.
	Seed uistate.BudgetAddSeed
}

// BudgetAddForm is the standalone add-a-budget form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Budgets() for use in the AddHost modal.
func BudgetAddForm(props BudgetAddFormProps) ui.Node {
	return ui.CreateElement(budgetAddForm, props)
}

type budgetAddFormComponentProps struct {
	Props BudgetAddFormProps
	App   *appstate.App
}

func budgetAddForm(props BudgetAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return ui.CreateElement(budgetAddFormReady, budgetAddFormComponentProps{Props: props, App: app})
}

type budgetAddActions struct {
	Name, Limit, NewCategory, Direction, Owner, Period, Method, Tags, VarName, Category, Copy func(string)
	Create, Rollover, Advanced, OpenTemplate, CloseTemplate, Cancel, Submit, Apply, Suggest   func()
	Also, Exclude                                                                             func(string)
	Custom, Bucket                                                                            func(string, string)
}

type budgetAddViewProps struct {
	Draft           budgetform.Draft
	Preview         budgetform.Preview
	Actions         budgetAddActions
	Base            string
	GlobalMethod    string
	Error           string
	LimitError      string
	Blocker         string
	Fate            string
	OwnerHint       string
	SuggestScope    string
	Categories      []domain.Category
	Budgets         []domain.Budget
	Members         []domain.Member
	Defs            []customfields.Def
	CategoryOptions []uiw.PickerOption
	CopyOptions     []uiw.SelectOption
	Template        budgetAddTemplateMemo
	Suggestion      budgetform.Suggestion
}

func budgetAddFormReady(cp budgetAddFormComponentProps) ui.Node {
	app := cp.App
	revision := uistate.UseDataRevision().Get()
	base := budgetBaseCurrency(app.Settings().BaseCurrency)
	initial := budgetform.Initial(cp.Props.Seed.Name, cp.Props.Seed.LimitMajor, cp.Props.Seed.CategoryID, cp.Props.Seed.Period, base)
	draft := ui.UseState(initial)
	errorState := ui.UseState("")
	d := draft.Get()
	now := time.Now()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	categories, budgets := app.Categories(), app.Budgets()
	income := uistate.CurrentPrefs().MonthlyIncomeMinor
	template := ui.UseMemo(func() budgetAddTemplateMemo {
		proposals, err := budgetform.Template(income, app.Transactions(), budgetLedgerForApp(app), categories, budgets, now, rates, d.Buckets)
		return budgetAddTemplateMemo{Proposals: proposals, Err: err}
	}, revision, app.Rev(), base, budgetform.MonthIdentity(now), d.Buckets, income, len(categories), len(budgets))
	preview := budgetform.Inspect(d, categories, budgets, app.Members(), cp.Props.Seed.Name, cp.Props.Seed.LimitMajor)
	suggestion := ui.UseMemo(func() budgetform.Suggestion {
		return budgetform.Suggest(preview.SuggestCategory, categories, budgetLedgerForApp(app), now, d.Period, d.Currency, rates)
	}, revision, app.Rev(), preview.SuggestCategory, d.Period, d.Currency, base, budgetform.MonthIdentity(now), len(categories))
	if template.Err != nil {
		slog.Error("calculate 50/30/20 budget template", "error", template.Err)
	}
	if suggestion.Err != nil {
		slog.Error("calculate budget limit suggestion", "error", suggestion.Err)
	}
	p := budgetAddViewProps{Draft: d, Preview: preview, Base: base, GlobalMethod: app.Settings().BudgetMethodology, Error: errorState.Get(), Categories: categories, Budgets: budgets, Members: app.Members(), Defs: app.CustomFieldDefsFor("budget"), Template: template, Suggestion: suggestion}
	p.Actions = budgetAddHandlers(draft, errorState, cp, base, template.Proposals, suggestion, now)
	p = budgetAddMessages(p)
	if d.Template {
		return ui.CreateElement(budgetAddTemplateReview, p)
	}
	return ui.CreateElement(budgetAddContent, p)
}

func budgetAddHandlers(state ui.State[budgetform.Draft], errors ui.State[string], cp budgetAddFormComponentProps, base string, proposals []budgeting.BudgetProposal, suggestion budgetform.Suggestion, now time.Time) budgetAddActions {
	change := func(action budgetform.Action, value string) { state.Set(budgetform.Change(state.Get(), action, value)) }
	return budgetAddActions{
		Name:        func(v string) { state.Set(budgetform.ChangeName(state.Get(), v)) },
		Limit:       func(v string) { change(budgetform.EditLimit, v) },
		NewCategory: func(v string) { change(budgetform.EditNewCategory, v) },
		Direction:   func(v string) { change(budgetform.EditDirection, v) },
		Owner:       func(v string) { change(budgetform.EditOwner, v) },
		Period:      func(v string) { change(budgetform.EditPeriod, v) },
		Method:      func(v string) { change(budgetform.EditMethod, v) },
		Tags:        func(v string) { change(budgetform.EditTags, v) },
		VarName:     func(v string) { change(budgetform.EditVariable, v) },
		Category: func(v string) {
			state.Set(budgetform.ChooseCategory(state.Get(), budgetform.Inspect(state.Get(), cp.App.Categories(), nil, nil, "", "").Categories, v))
		},
		Copy: func(v string) {
			d, name, ok := budgetform.Copy(state.Get(), cp.App.Budgets(), v)
			if ok {
				d.Name = uistate.T("budgets.copySuffix", name)
				state.Set(d)
			}
		},
		Create:        func() { change(budgetform.ToggleCreate, "") },
		Rollover:      func() { change(budgetform.ToggleRollover, "") },
		Advanced:      func() { change(budgetform.ToggleAdvanced, "") },
		OpenTemplate:  func() { change(budgetform.OpenTemplate, "") },
		CloseTemplate: func() { change(budgetform.CloseTemplate, "") },
		Cancel: func() {
			if cp.Props.OnDone != nil {
				cp.Props.OnDone()
			}
		},
		Submit:  func() { budgetAddSubmit(state, errors, cp, base) },
		Apply:   func() { budgetAddApplyTemplate(cp, proposals, state.Get().Excluded, base, now) },
		Suggest: func() { change(budgetform.EditLimit, budgetform.FormatLimit(suggestion.Amount)) },
		Also:    func(id string) { d := state.Get(); d.Also = budgetform.Toggle(d.Also, id); state.Set(d) },
		Exclude: func(id string) { d := state.Get(); d.Excluded = budgetform.Toggle(d.Excluded, id); state.Set(d) },
		Custom: func(key, v string) {
			d := state.Get()
			d.Custom = budgetform.SetCustom(d.Custom, key, v)
			state.Set(d)
		},
		Bucket: func(id, v string) {
			d := state.Get()
			d.Buckets = budgetform.SetBucket(d.Buckets, id, v)
			state.Set(d)
		},
	}
}

func budgetAddSubmit(state ui.State[budgetform.Draft], errors ui.State[string], cp budgetAddFormComponentProps, base string) {
	app := cp.App
	p := budgetform.Prepare(state.Get(), app.Categories(), app.Budgets(), app.CustomFieldDefsFor("budget"), id.New(), id.New())
	if len(p.CustomIssues) > 0 {
		errors.Set(strings.Join(p.CustomIssues, " "))
		return
	}
	if p.ProblemKey != "" {
		if p.ProblemKey == "budgets.varNameTaken" {
			errors.Set(uistate.T(p.ProblemKey, p.CollisionName))
		} else {
			errors.Set(uistate.T(p.ProblemKey))
		}
		return
	}
	if !p.LimitProblem.OK() {
		errors.Set(budgetLimitError(p.LimitProblem, state.Get().Currency))
		return
	}
	if err := app.CreateBudgetWithCategory(p.Category, p.Budget); err != nil {
		slog.Error("create budget with category", "error", err)
		errors.Set(budgetErrorText(err))
		return
	}
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	state.Set(budgetform.Reset(state.Get(), base, cp.Props.Seed.CategoryID))
	errors.Set("")
	if p.CreatedName != "" {
		uistate.PostNotice(uistate.T("budgets.addedWithCatToast", p.CreatedName), false)
	} else {
		uistate.PostNotice(uistate.T("budgets.addedToast"), false)
	}
	if cp.Props.OnDone != nil {
		cp.Props.OnDone()
	}
}

func budgetAddApplyTemplate(cp budgetAddFormComponentProps, proposals []budgeting.BudgetProposal, excluded map[string]bool, base string, now time.Time) {
	n, err := applyBudgetTemplate(cp.App, proposals, excluded, base, now)
	if err != nil {
		uistate.PostNotice(budgetErrorText(err), true)
		return
	}
	if n == 0 {
		uistate.PostNotice(uistate.T("budgets.tmplNothingToAdd"), false)
		return
	}
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	uistate.PostUndoable(uistate.T("budgets.tmplApplied", uistate.T(budgetCountKey("budgets.addedCount", n), n)))
	if cp.Props.OnDone != nil {
		cp.Props.OnDone()
	}
}

func budgetAddMessages(p budgetAddViewProps) budgetAddViewProps {
	v := p.Preview
	if v.LimitFieldError {
		p.LimitError = budgetLimitError(v.LimitProblem, p.Draft.Currency)
	}
	if v.BlockerKey != "" {
		p.Blocker = uistate.T(v.BlockerKey)
	} else if !v.LimitProblem.OK() {
		p.Blocker = p.LimitError
	}
	switch v.FateKey {
	case "budgets.catWillReuse":
		p.Fate = uistate.T(v.FateKey, v.FatePath)
	case "budgets.catAmbiguous":
		p.Fate = uistate.T(v.FateKey, v.Ambiguous, v.TypedCategory)
	case "budgets.catWillCreate":
		p.Fate = uistate.T(v.FateKey, v.TypedCategory)
	}
	p.OwnerHint = uistate.T(v.OwnerKey)
	if v.OwnerKey == "budgets.ownerIndividualHint" {
		p.OwnerHint = uistate.T(v.OwnerKey, v.OwnerName)
	}
	if p.Suggestion.ScopePath != "" {
		p.SuggestScope = uistate.T("budgets.suggestScoped", uistate.T("budgets.scopeNoteCats", p.Suggestion.ScopePath))
	}
	p.CategoryOptions = budgetAddCategoryOptions(v.Categories, p.Budgets, p.Draft.Period, p.Draft.Owner)
	p.CopyOptions = []uiw.SelectOption{{Value: "", Label: uistate.T("budgets.copyExisting")}}
	for _, b := range p.Budgets {
		p.CopyOptions = append(p.CopyOptions, uiw.SelectOption{Value: b.ID, Label: b.Name})
	}
	return p
}

func budgetAddContent(p budgetAddViewProps) ui.Node {
	submit := OnSubmit(ui.UseEvent(Prevent(p.Actions.Submit)))
	return Form(css.Class("budget-add-shell"), Attr("data-testid", "budget-add-form"), submit,
		Div(css.Class("modal-scroll"),
			ui.CreateElement(budgetAddShortcuts, p),
			Div(css.Class("budget-add-or"), Span(uistate.T("budgets.tmplOr"))),
			Div(css.Class("form-grid", "budget-add-grid"),
				ui.CreateElement(budgetAddIdentity, p), ui.CreateElement(budgetAddCategory, p), ui.CreateElement(budgetAddLimit, p), ui.CreateElement(budgetAddSuggestion, p), ui.CreateElement(budgetAddAdvanced, p)),
			errText("budget-err", p.Error)), ui.CreateElement(budgetAddFooter, p))
}

func budgetAddAdvanced(p budgetAddViewProps) ui.Node {
	toggle := OnClick(ui.UseEvent(Prevent(p.Actions.Advanced)))
	return Fragment(
		Div(css.Class("ba-full"), Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "budget-add-advanced"), Attr("aria-expanded", ariaBool(p.Draft.Advanced)), toggle, Text(uistate.T(p.Preview.AdvancedKey)))),
		If(p.Draft.Advanced, Fragment(ui.CreateElement(budgetAddTracking, p), ui.CreateElement(budgetAddBehaviour, p), ui.CreateElement(budgetAddFormula, p))))
}

func budgetAddSuggestionNote(p budgetAddViewProps) ui.Node {
	if p.Preview.WantNew && p.Suggestion.Amount.Amount == 0 && p.Preview.TypedCategory != "" {
		return Div(css.Class("ba-full"), Span(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-suggest-none"), uistate.T("budgets.suggestNoHistory")))
	}
	if p.Suggestion.Err != nil {
		return Div(css.Class("ba-full"), P(css.Class("form-err"), Attr("role", "status"), uistate.T("budgets.incomplete")))
	}
	return Fragment()
}

func budgetAddShortcuts(p budgetAddViewProps) ui.Node {
	openTmplOnClick := OnClick(ui.UseEvent(Prevent(p.Actions.OpenTemplate)))
	return Fragment(
		Div(css.Class("budget-add-tmpl"), Attr("data-testid", "budget-add-tmpl"),
			Div(css.Class("row-main"),
				Span(css.Class("budget-add-tmpl-title"), uistate.T("budgets.tmplBannerTitle")),
				Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.tmplBannerHint")),
			),
			Div(css.Class("budget-add-tmpl-actions"),
				Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "budgets-template-503020"),
					Title(uistate.T("budgets.tmplTitle")), openTmplOnClick,
					uiw.Icon(icon.Split, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.tmpl503020"))),
				If(len(p.CopyOptions) > 1, uiw.SelectInput(uiw.SelectInputProps{
					Options:   p.CopyOptions,
					Selected:  "",
					OnChange:  func(v string) { p.Actions.Copy(v) },
					AriaLabel: uistate.T("budgets.copyExisting"),
					TestID:    "budget-copy-existing",
				})),
			),
			If(p.Template.Err != nil, P(css.Class("form-err"), Attr("role", "status"), Attr("data-testid", "budget-template-income-incomplete"), uistate.T("budgets.incomplete"))),
		),
	)
}

func budgetAddIdentity(p budgetAddViewProps) ui.Node {
	nameOnInput := OnInput(ui.UseEvent(p.Actions.Name))
	return Fragment(
		Div(css.Class("ba-full"),
			uiw.Field(uiw.FieldProps{ID: "budget-add", Label: uistate.T("common.name"), Required: true, TestID: "budget-add-name-field",
				Control: func(a uiw.FieldA11y) ui.Node {
					attrs := []any{css.Class("field"), Attr("id", a.ID), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), nameOnInput}
					if p.Preview.NameAutofocus {
						attrs = append(attrs, Attr("autofocus", "autofocus"))
					}
					attrs = append(attrs, errAttrs("budget-err", p.Error)...)
					return Input(append(attrs, uiw.FieldValue(p.Draft.Name))...)
				},
			})),
		// What this budget measures, before what it watches: the direction
		// changes which categories are even offered below.
		Div(css.Class("ba-full"),
			labeledField(uistate.T("budgets.directionLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: []uiw.SelectOption{
						{Value: string(domain.DirectionSpend), Label: uistate.T("budgets.directionSpend")},
						{Value: string(domain.DirectionSave), Label: uistate.T("budgets.directionSave")},
					},
					Selected:  p.Draft.Direction,
					OnChange:  p.Actions.Direction,
					AriaLabel: uistate.T("budgets.directionLabel"),
					TestID:    "budget-direction",
				}))),
		If(p.Preview.Saving,
			P(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-direction-hint"),
				uistate.T("budgets.directionSaveHint"))),
	)
}

func budgetAddLimit(p budgetAddViewProps) ui.Node {
	limitOnInput := OnInput(ui.UseEvent(p.Actions.Limit))
	return Fragment(
		uiw.Field(uiw.FieldProps{ID: "budget-add-limit", Label: uistate.T("budgets.limitLabel"), Required: true,
			Error: p.LimitError, TestID: "budget-add-limit-field",
			Control: func(a uiw.FieldA11y) ui.Node {
				attrs := []any{css.Class("field"), Attr("id", a.ID), Type("number"), Attr("data-testid", "budget-add-limit"),
					Attr("aria-required", "true"), Attr("aria-describedby", a.DescribedBy),
					Placeholder(uistate.T("budgets.limitPlaceholder", p.Draft.Currency)), Step("0.01"), Attr("min", "0.01"), limitOnInput}
				if p.Preview.LimitAutofocus {
					attrs = append(attrs, Attr("autofocus", "autofocus"))
				}
				if p.LimitError != "" {
					attrs = append(attrs, Attr("aria-invalid", "true"))
				}
				return Input(append(attrs, uiw.FieldValue(p.Draft.Limit))...)
			},
		}),
		labeledField(uistate.T("budgets.period"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   periodOptions(p.Draft.Period),
				Selected:  p.Draft.Period,
				OnChange:  p.Actions.Period,
				AriaLabel: uistate.T("budgets.period"),
			})),
	)
}

func budgetAddSuggestion(p budgetAddViewProps) ui.Node {
	suggestOnClick := OnClick(ui.UseEvent(Prevent(p.Actions.Suggest)))
	return Fragment(
		If(p.Suggestion.Amount.Amount > 0, Div(css.Class("ba-full", "suggest-row"),
			Span(css.Class("muted"), uistate.T("budgets.suggest", fmtMoney(money.New(p.Suggestion.Amount.Amount, p.Draft.Currency)))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-use-suggest"), suggestOnClick, uistate.T("budgets.useSuggest")),
		)),
		// Where a rolled-up average came from, so it is never read as one
		// category's spending (C586).
		If(p.SuggestScope != "", Div(css.Class("ba-full"),
			Span(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-suggest-scope"), p.SuggestScope))),
		ui.CreateElement(budgetAddSuggestionNote, p),
	)
}

func budgetAddTracking(p budgetAddViewProps) ui.Node {
	tagsOnInput := OnInput(ui.UseEvent(p.Actions.Tags))
	return Fragment(
		// --- What else this budget counts ---
		Div(css.Class("ba-full", "ba-group"), Attr("data-testid", "budget-add-group-tracking"),
			Span(css.Class("ba-group-head"), uistate.T("budgets.groupTracking")),
			Span(css.Class("ba-group-hint", tw.TextFaint), uistate.T("budgets.groupTrackingHint"))),
		// Optional multi-category: track more existing categories in this one budget.
		If(p.Preview.ExpenseCount > 0, Div(css.Class("ba-full"),
			labeledField(uistate.T("budgets.catsAlsoTrack"),
				ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{
					Picked: p.Draft.Also, OnToggle: p.Actions.Also,
					Saving: p.Preview.Saving,
				})))),
		// Optional cross-category tag tracking: count any charge with these tags,
		// whatever its category. Comma-separated; parsed + deduped on save.
		Div(css.Class("ba-full"),
			labeledField(uistate.T("budgets.tagsFieldLabel"),
				Fragment(
					Input(css.Class("field"), Type("text"), Attr("data-testid", "budget-add-tags"),
						Placeholder(uistate.T("budgets.tagsPlaceholder")), tagsOnInput, uiw.FieldValue(p.Draft.Tags)),
					Span(css.Class("budget-owner-hint", tw.TextFaint), uistate.T("budgets.tagsFieldHint"))))),
	)
}

func budgetAddBehaviour(p budgetAddViewProps) ui.Node {
	rolloverOnChange := OnChange(ui.UseEvent(p.Actions.Rollover))
	return Fragment(
		// --- How it behaves ---
		Div(css.Class("ba-full", "ba-group"), Attr("data-testid", "budget-add-group-behaviour"),
			Span(css.Class("ba-group-head"), uistate.T("budgets.groupBehaviour")),
			Span(css.Class("ba-group-hint", tw.TextFaint), uistate.T("budgets.groupBehaviourHint"))),
		// Owner (hidden until members exist) with its scope consequence spelled out.
		If(len(p.Members) > 0, Fragment(
			labeledField(uistate.T("common.owner"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   ownerSelectOptions(p.Members, p.Draft.Owner),
					Selected:  p.Draft.Owner,
					OnChange:  p.Actions.Owner,
					AriaLabel: uistate.T("common.owner"),
				})),
			Div(css.Class("ba-full"),
				Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "budget-owner-hint"), p.OwnerHint)))),
		// C590: name the scope. This picker and the page's "Budgeting method"
		// popover were both called "method", and neither said which budgets it moved.
		labeledField(uistate.T("budgets.methodLabel"),
			Fragment(
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   budgetMethodOptions(p.Draft.Methodology),
					Selected:  p.Draft.Methodology,
					OnChange:  p.Actions.Method,
					AriaLabel: uistate.T("budgets.methodLabel"),
				}),
				Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "budget-add-method-hint"),
					budgetOwnMethodHint(p.GlobalMethod, p.Draft.Methodology)))),
		// Rollover gets its own full-width row so the label never wraps.
		Label(css.Class("ba-full", "ba-check"),
			Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), rolloverOnChange}, checkedAttr(p.Draft.Rollover)...)...),
			Span(Title(uistate.T("budgets.rolloverTitle")), uistate.T("budgets.rollover")),
		),
		MapKeyed(p.Defs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: p.Draft.Custom[d.Key], OnChange: p.Actions.Custom})
		}),
	)
}

func budgetAddFormula(p budgetAddViewProps) ui.Node {
	varNameHandler := ui.UseEvent(p.Actions.VarName)
	return Fragment(
		// --- For your own calculations ---
		// LAST, and explained: this is a formula handle, not a budget setting, and
		// leading the advanced section with it asked a first-time user to name an
		// engine variable before they had finished naming their budget.
		Div(css.Class("ba-full", "ba-group"), Attr("data-testid", "budget-add-group-formula"),
			Span(css.Class("ba-group-head"), uistate.T("budgets.groupFormula")),
			Span(css.Class("ba-group-hint", tw.TextFaint), uistate.T("budgets.groupFormulaHint"))),
		Div(css.Class("ba-full"),
			labeledField(uistate.T("budgets.varNameLabel"),
				entityVarField(budgetVarKind, budgetVarEntities(p.Budgets), "", "budget-add-varname", "budget-add-varname-warn", p.Draft.VarName, p.Draft.Name, varNameHandler))),
	)
}

func budgetAddCategory(p budgetAddViewProps) ui.Node {
	toggleCreateOnChange := OnChange(ui.UseEvent(p.Actions.Create))
	newCategoryOnInput := OnInput(ui.UseEvent(p.Actions.NewCategory))
	var catFateNode ui.Node = Fragment()
	if p.Fate != "" {
		catFateNode = Span(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-cat-fate"), p.Fate)
	}
	return Fragment(
		Div(css.Class("ba-full", "budget-cat-row"),
			Label(css.Class("budget-cat-optin", tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("data-testid", "budget-create-cat-label"),
				Input(append([]any{Type("checkbox"), Attr("data-testid", "budget-create-cat"),
					Attr("style", "flex-shrink:0"), toggleCreateOnChange},
					append(checkedAttr(p.Preview.WantNew), checkboxDisabled(p.Preview.MustCreate)...)...)...),
				Span(uistate.T("budgets.createCatOptIn"))),
			If(p.Preview.MustCreate, Span(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-create-cat-forced"),
				uistate.T("budgets.createCatForced"))),
			If(!p.Preview.WantNew, uiw.Field(uiw.FieldProps{
				ID: "budget-add-category", Label: uistate.T("budgets.categoryLabel"), Required: true,
				TestID: "budget-add-category-field",
				Control: func(a uiw.FieldA11y) ui.Node {
					return uiw.Picker(uiw.PickerProps{
						ID: a.ID, Label: a.Label, Placeholder: uistate.T("budgets.chooseCategory"),
						SelectedLabel: p.Preview.SelectedName, Searchable: true, TestID: "budget-existing-cat",
						Options: p.CategoryOptions, OnCommit: p.Actions.Category,
					})
				},
			})),
			If(p.Preview.WantNew && !p.Preview.MustCreate, labeledField(uistate.T("budgets.newCategoryName"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "budget-new-cat-name"),
					Placeholder(uistate.T("budgets.newCategoryPlaceholder")), newCategoryOnInput, uiw.FieldValue(p.Draft.NewCategoryName)))),
			catFateNode,
		),
	)
}

func budgetAddFooter(p budgetAddViewProps) ui.Node {
	cancelOnClick := OnClick(ui.UseEvent(Prevent(p.Actions.Cancel)))
	return Fragment(
		Div(css.Class("modal-foot"),
			// C666: a disabled button is only honest if it says what it is waiting
			// for. This line sits with the action, so the answer is where the
			// question gets asked.
			If(p.Blocker != "", Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "budget-add-blockers"), p.Blocker)),
			Button(css.Class("btn"), Type("button"), cancelOnClick, uistate.T("action.cancel")),
			Button(append([]any{css.Class("btn btn-primary", "ba-submit"), Type("submit"),
				Attr("data-testid", "budget-add-submit"), uistate.T("budgets.add")},
				disabledAttr(p.Preview.Blocked)...)...),
		),
	)
}

func budgetAddTemplateReview(p budgetAddViewProps) ui.Node {
	back := OnClick(ui.UseEvent(Prevent(p.Actions.CloseTemplate)))
	apply := OnSubmit(ui.UseEvent(Prevent(p.Actions.Apply)))
	checked, total, err := budgetform.Review(p.Template.Proposals, p.Draft.Excluded)
	if err != nil {
		slog.Error("calculate template review total", "error", err)
	}
	rows := MapKeyed(p.Template.Proposals, func(proposal budgeting.BudgetProposal) any { return proposal.Category.ID }, func(proposal budgeting.BudgetProposal) ui.Node {
		return ui.CreateElement(tmplReviewRow, tmplReviewRowProps{ID: proposal.Category.ID, Label: proposal.Category.Name, AmountStr: fmtMoney(money.New(proposal.LimitMinor, p.Base)), Checked: !p.Draft.Excluded[proposal.Category.ID], Bucket: proposal.Bucket, OnToggle: p.Actions.Exclude, OnBucketChange: p.Actions.Bucket})
	})
	var emptyNote ui.Node = Fragment()
	if len(p.Template.Proposals) == 0 {
		emptyNote = P(css.Class("empty"), uistate.T("budgets.tmplNothingToAdd"))
	}
	return Form(css.Class("budget-add-shell"), Attr("data-testid", "budget-tmpl-review"), apply,
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}), uistate.T("bc6n7.reviewHint")),
			If(p.Template.Err != nil, P(css.Class("form-err"), Attr("role", "status"), uistate.T("budgets.incomplete"))),
			Div(css.Class("budget-tmpl-rows"), rows),
			emptyNote,
			If(checked > 0, P(css.Class("budget-tmpl-total"), Attr("data-testid", "budget-tmpl-total"),
				uistate.T("budgets.tmplReviewTotal", fmtMoney(money.New(total, p.Base)), uistate.T(budgetCountKey("bc4b4.budgetCount", checked), checked)))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-tmpl-back"), back, uistate.T("budgets.tmplBack")),
			Button(css.Class("btn btn-primary", "ba-submit"), Type("submit"), Attr("data-testid", "budget-tmpl-apply"),
				attrIf(checked == 0, "disabled", "disabled"),
				uistate.T("budgets.tmplCreateN", uistate.T(budgetCountKey("bc4b4.budgetCount", checked), checked))),
		),
	)
}

func applyBudgetTemplate(app *appstate.App, proposals []budgeting.BudgetProposal, excluded map[string]bool, base string, now time.Time) (int, error) {
	writes := budgetform.TemplateBudgets(proposals, excluded, base, id.New)
	if len(writes) == 0 {
		return 0, nil
	}
	if err := app.PutBudgetsAtomicAt(writes, now); err != nil {
		return 0, err
	}
	return len(writes), nil
}

type budgetAddTemplateMemo struct {
	Proposals []budgeting.BudgetProposal
	Err       error
}

// attrIf returns the attribute option when cond holds, else a no-op fragment arg.
func attrIf(cond bool, k, v string) any {
	if cond {
		return Attr(k, v)
	}
	return Fragment()
}

// tmplReviewRowProps drives one proposal line in the 50/30/20 review list.
type tmplReviewRowProps struct {
	ID, Label, AmountStr string
	Checked              bool
	Bucket               budgeting.Bucket
	OnToggle             func(string) // plain func Ã¢â‚¬â€ never an On* hook (no-On*-in-loop rule)
	OnBucketChange       func(string, string)
}

// tmplReviewRow is one checkbox row of the template review: include/exclude the
// proposed budget. Its own component so the change hook sits at a stable call-site.
func tmplReviewRow(props tmplReviewRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	return Div(css.Class("budget-tmpl-row"),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "tmpl-pick-"+props.ID), OnChange(toggle)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("budget-tmpl-name"), props.Label),
		uiw.SelectInput(uiw.SelectInputProps{
			Options: []uiw.SelectOption{
				{Value: strconv.Itoa(int(budgeting.BucketNeeds)), Label: uistate.T("bc6n7.bucketNeeds")},
				{Value: strconv.Itoa(int(budgeting.BucketWants)), Label: uistate.T("bc6n7.bucketWants")},
				{Value: strconv.Itoa(int(budgeting.BucketSavings)), Label: uistate.T("bc6n7.bucketSavings")},
			},
			Selected: strconv.Itoa(int(props.Bucket)), OnChange: func(value string) { props.OnBucketChange(props.ID, value) },
			AriaLabel: uistate.T("bc6n7.bucketFor", props.Label), Class: "budget-tmpl-bucket", TestID: "tmpl-bucket-" + props.ID,
		}),
		Span(css.Class("budget-tmpl-amt"), props.AmountStr),
	)
}

// checkboxDisabled returns the disabled attribute when the control has no real
// choice to offer Ã¢â‚¬â€ a household with no categories must create one, and a
// checkbox that looks clickable but is not is worse than one that says so.
func checkboxDisabled(disabled bool) []any {
	if disabled {
		return []any{Disabled(true)}
	}
	return nil
}
