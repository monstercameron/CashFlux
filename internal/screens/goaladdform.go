// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smarttext"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalAddFormProps configures the GoalAddForm component.
type GoalAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// GoalAddForm is the standalone add-a-goal form. It owns all its state and
// handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Goals() for use in the AddHost modal.
func GoalAddForm(props GoalAddFormProps) ui.Node {
	return ui.CreateElement(goalAddForm, props)
}

func goalAddForm(props GoalAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	accounts := app.Accounts()

	name := ui.UseState("")
	target := ui.UseState("")
	current := ui.UseState("0")
	owner := ui.UseState(domain.GroupOwnerID)
	dateStr := ui.UseState("")
	linkAcct := ui.UseState("")
	advOpen := ui.UseState(false)
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	// C189: sinking-fund flag; C192: optional linked spending category.
	isSinkingFund := ui.UseState(false)
	categoryID := ui.UseState("")
	// Goal kind (savings / checklist / milestone / habit) + habit-only fields.
	kindS := ui.UseState(string(domain.GoalKindFinancial))
	cadenceS := ui.UseState(string(domain.CadenceWeekly))
	habitTargetS := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onHabitTarget := ui.UseEvent(func(v string) { habitTargetS.Set(v) })
	// onOwner/onLinkAcct hooks kept for stable hook ordering; SelectInput owns the
	// change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { linkAcct.Set(e.GetValue()) })
	// Essentials-first: everything beyond Name / Type / Target / Date lives behind this
	// disclosure so the common case reads as a three-field form.
	toggleAdv := ui.UseEvent(Prevent(func() { advOpen.Set(!advOpen.Get()) }))
	// Quick-start templates: one click seeds the name (and the sinking-fund flag where
	// that's the right shape) so a blank form never stares back. Plain closure — the
	// chips are a fixed set rendered by hook-owning child components.
	applyTemplate := func(tName string, sinking bool) {
		name.Set(tName)
		isSinkingFund.Set(sinking)
		kindS.Set(string(domain.GoalKindFinancial))
		if sinking {
			advOpen.Set(true) // the seeded flag lives in More options — show it
		}
	}
	// Copy an existing goal: prefills the form from it (name gets a "copy" suffix) and
	// opens More options so every carried-over value is visible, not silently applied.
	copyFrom := func(gid string) {
		for _, gg := range app.Goals() {
			if gg.ID != gid {
				continue
			}
			name.Set(uistate.T("budgets.copySuffix", gg.Name))
			kindS.Set(string(gg.EffectiveKind()))
			if gg.TargetAmount.Amount > 0 {
				target.Set(money.FormatMinor(gg.TargetAmount.Amount, currency.Decimals(gg.TargetAmount.Currency)))
			}
			owner.Set(gg.OwnerID)
			linkAcct.Set(gg.AccountID)
			isSinkingFund.Set(gg.IsSinkingFund)
			categoryID.Set(gg.CategoryID)
			advOpen.Set(true)
			return
		}
	}
	// onSinkingFund / onCategoryID hooks kept for stable hook ordering; the
	// checkbox uses OnChange(onSinkingFund) and SelectInput owns onCategoryID.
	onSinkingFund := ui.UseEvent(func(e ui.Event) { isSinkingFund.Set(e.IsChecked()) })
	ui.UseEvent(func(e ui.Event) { categoryID.Set(e.GetValue()) })

	goalDefs := app.CustomFieldDefsFor("goal")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

	add := ui.UseEvent(Prevent(func() {
		if strings.TrimSpace(name.Get()) == "" {
			errMsg.Set(uistate.T("goals.nameRequired"))
			return
		}
		kind := domain.GoalKind(kindS.Get())
		// A target date is optional for every kind; parse it once up front.
		var targetDate time.Time
		if ds := strings.TrimSpace(dateStr.Get()); ds != "" {
			d, derr := dateutil.ParseDate(ds)
			if derr != nil {
				errMsg.Set(uistate.T("goals.invalidDate"))
				return
			}
			targetDate = d
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		// Base goal common to every kind. Money defaults to a zeroed base-currency
		// amount so non-financial goals still carry a valid currency downstream.
		g := domain.Goal{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			Kind: kind, TargetDate: targetDate,
			TargetAmount: money.New(0, base), CurrentAmount: money.New(0, base),
			Custom:         customValuesToMap(goalDefs, customVals.Get()),
			LastReviewedAt: time.Now(), // a freshly-created goal counts as just reviewed
		}
		switch kind {
		case domain.GoalKindFinancial:
			tgt, err := money.ParseMinor(strings.TrimSpace(target.Get()), currency.Decimals(base))
			if err != nil || tgt <= 0 {
				errMsg.Set(uistate.T("goals.targetRequired"))
				return
			}
			cur, err := money.ParseMinor(strings.TrimSpace(current.Get()), currency.Decimals(base))
			if err != nil {
				cur = 0
			}
			g.TargetAmount = money.New(tgt, base)
			g.CurrentAmount = money.New(cur, base)
			// C9 (one provenance regime): seed money entered at creation is RECORDED as
			// the goal's first contribution, so it's itemised and undoable exactly like
			// money added later — never an unexplained starting number.
			if cur > 0 {
				g = g.RecordContribution(domain.GoalContribution{Amount: money.New(cur, base), At: time.Now()})
			}
			g.AccountID = linkAcct.Get()
			g.IsSinkingFund = isSinkingFund.Get()
			g.CategoryID = categoryID.Get()
		case domain.GoalKindHabit:
			n, err := strconv.Atoi(strings.TrimSpace(habitTargetS.Get()))
			if err != nil || n <= 0 {
				errMsg.Set(uistate.T("goals.habitTargetRequired"))
				return
			}
			g.HabitCadence = domain.RecurringCadence(cadenceS.Get())
			g.HabitTarget = n
		}
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		name.Set("")
		target.Set("")
		current.Set("0")
		dateStr.Set("")
		linkAcct.Set("")
		isSinkingFund.Set(false)
		categoryID.Set("")
		kindS.Set(string(domain.GoalKindFinancial))
		habitTargetS.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		uistate.PostNotice(uistate.T("goals.addedToast"), false)
		// The add modal is a sibling (AddHost), so bump the shared data revision to
		// re-render the Goals list immediately rather than only after a reload
		// (C177/R2). Goals() subscribes to UseDataRevision for this.
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	// Cancel just closes the add modal (the pinned footer's quiet action).
	cancelAdd := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	ownerOptions := ownerSelectOptions(app.Members(), owner.Get())
	linkOptions := goalAccountOptions(accounts, linkAcct.Get())
	catOptions := goalCategoryOptions(app.Categories(), categoryID.Get())

	// Wish→goal assist: if the typed name parses as a free-text savings wish,
	// offer a chip that fills both the name and the target amount in one click.
	goalSmartSettings := uistate.LoadSmartSettings()
	decimals := currency.Decimals(base)
	var wishSuggestion string
	var wishMinor int64
	if wn, wamt, wok := smarttext.ParseWish(name.Get()); wok {
		// Only show when the parsed name differs from what was typed (real suggestion).
		if wn != strings.TrimSpace(name.Get()) || target.Get() == "" || target.Get() == "0" {
			wishSuggestion = wn
			wishMinor = wamt
		}
	}
	wishAssist := smartFieldAssist(goalSmartSettings, "goal-wish", wishSuggestion, func() {
		name.Set(wishSuggestion)
		if wishMinor > 0 {
			target.Set(money.FormatMinor(wishMinor, decimals))
		}
	})

	kind := domain.GoalKind(kindS.Get())
	financial := kind.IsFinancial()

	advLabel := uistate.T("budgets.advancedShow")
	if advOpen.Get() {
		advLabel = uistate.T("budgets.advancedHide")
	}
	// Quick-start templates + copy-an-existing-goal shortcuts (a blank form never
	// stares back). Copy options lead with a placeholder so the select reads as an action.
	copyOptions := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.copyExisting")}}
	for _, gg := range app.Goals() {
		if !gg.Archived {
			copyOptions = append(copyOptions, uiw.SelectOption{Value: gg.ID, Label: gg.Name})
		}
	}

	return Form(css.Class("acct-edit-form", "goal-add"), Attr("data-testid", "goal-add-form"), OnSubmit(add),
		Div(css.Class("modal-scroll"),
			// Start-from shortcuts: template chips + copy an existing goal.
			Div(css.Class("goal-add-start"), Attr("data-testid", "goal-add-start"),
				Div(css.Class("goal-add-start-chips"),
					ui.CreateElement(goalTemplateChip, goalTemplateChipProps{Label: uistate.T("goals.tmplEmergency"), Sinking: false, OnPick: applyTemplate}),
					ui.CreateElement(goalTemplateChip, goalTemplateChipProps{Label: uistate.T("goals.tmplVacation"), Sinking: false, OnPick: applyTemplate}),
					ui.CreateElement(goalTemplateChip, goalTemplateChipProps{Label: uistate.T("goals.tmplCar"), Sinking: false, OnPick: applyTemplate}),
					ui.CreateElement(goalTemplateChip, goalTemplateChipProps{Label: uistate.T("goals.tmplRepairs"), Sinking: true, OnPick: applyTemplate}),
				),
				If(len(copyOptions) > 1, uiw.SelectInput(uiw.SelectInputProps{
					Options: copyOptions, Selected: "", TestID: "goal-copy-existing",
					OnChange: func(v string) { copyFrom(v) }, AriaLabel: uistate.T("goals.copyExisting"),
				})),
			),
			Div(css.Class("form-grid"),
				// The essentials: Name, Type, then the kind's defining figures + deadline.
				Div(css.Class("fg-span"),
					labeledField(uistate.T("common.name"),
						Input(append([]any{css.Class("field"), Attr("id", "goal-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("goal-err", errMsg.Get())...)...))),
				Div(css.Class("fg-span"),
					labeledField(uistate.T("goals.kindLabel"),
						Div(
							uiw.SelectInput(uiw.SelectInputProps{
								Options: goalKindOptions(), Selected: kindS.Get(), TestID: "goal-add-kind",
								OnChange: func(v string) { kindS.Set(v) }, AriaLabel: uistate.T("goals.kindLabel"),
							}),
							Span(css.Class("budget-sub"), Attr("data-testid", "goal-add-kind-hint"), goalKindHint(kind)),
						))),
				If(financial, wishAssist),
				If(financial, labeledField(uistate.T("goals.targetLabel"),
					Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("goals.targetPlaceholder", base)), Value(target.Get()), Step("0.01"), OnInput(onTarget)))),
				// Habit essentials: check-in rhythm + how many check-ins finish it.
				If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitCadenceLabel"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options: habitCadenceOptions(), Selected: cadenceS.Get(), TestID: "goal-add-cadence",
						OnChange: func(v string) { cadenceS.Set(v) }, AriaLabel: uistate.T("goals.habitCadenceLabel"),
					}))),
				If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitTargetLabel"),
					Input(css.Class("field"), Type("number"), Attr("data-testid", "goal-add-habit-target"), Placeholder(uistate.T("goals.habitTargetPlaceholder")), Value(habitTargetS.Get()), Step("1"), OnInput(onHabitTarget)))),
				labeledField(uistate.T("goals.dateLabel"),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateStr.Get()), OnInput(onDate))),
				// More options: seed money, ownership, links, fund flags, custom fields.
				Div(css.Class("fg-span"),
					Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "goal-add-advanced"),
						Attr("aria-expanded", ariaBool(advOpen.Get())), OnClick(toggleAdv), Text(advLabel))),
				If(advOpen.Get(), Fragment(
					If(financial, labeledField(uistate.T("goals.savedSoFar"),
						Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.savedSoFar")), Value(current.Get()), Step("0.01"), OnInput(onCurrent)))),
					labeledField(uistate.T("goals.owner"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   ownerOptions,
							Selected:  owner.Get(),
							OnChange:  func(v string) { owner.Set(v) },
							AriaLabel: uistate.T("goals.owner"),
						})),
					If(financial, Fragment(
						labeledField(uistate.T("goals.linkedOptional"),
							uiw.SelectInput(uiw.SelectInputProps{
								Options:   linkOptions,
								Selected:  linkAcct.Get(),
								OnChange:  func(v string) { linkAcct.Set(v) },
								AriaLabel: uistate.T("goals.linkedOptional"),
							})),
						// C6: what linking DOES (and does not do), said where it's chosen.
						Div(css.Class("fg-span"),
							Span(css.Class("budget-sub"), Attr("data-testid", "goal-add-link-hint"), uistate.T("goals.linkExplain"))))),
					// C189: sinking-fund toggle — marks this goal as a regular-save-for-irregular-expense fund.
					If(financial, Div(css.Class("fg-span"),
						labeledField(uistate.T("goals.sinkingFund"),
							Label(css.Class("goal-check-row"),
								Input(Type("checkbox"), Attr("id", "goal-add-sinking"), OnChange(onSinkingFund), Checked(isSinkingFund.Get())),
								Span(css.Class("budget-sub"), uistate.T("goals.sinkingFundHint")),
							)))),
					// C192: optional linked spending category for the fund (meaningful mainly for sinking funds).
					If(financial, labeledField(uistate.T("goals.linkedCategory"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   catOptions,
							Selected:  categoryID.Get(),
							OnChange:  func(v string) { categoryID.Set(v) },
							AriaLabel: uistate.T("goals.linkedCategory"),
						}))),
					MapKeyed(goalDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
						return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
					}),
				)),
				errText("goal-err", errMsg.Get()),
			),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancelAdd), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.add")),
		),
	)
}

// goalTemplateChipProps drives one quick-start template chip in the add-goal form.
type goalTemplateChipProps struct {
	Label   string
	Sinking bool
	OnPick  func(name string, sinking bool) // plain closure — the chip owns its hook
}

// goalTemplateChip seeds the add form with a template's name (and sinking-fund shape
// where that fits). Its own component so the click hook sits at a stable call-site.
func goalTemplateChip(props goalTemplateChipProps) ui.Node {
	pick := ui.UseEvent(Prevent(func() { props.OnPick(props.Label, props.Sinking) }))
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(props.Label), " ", "-"))
	return Button(css.Class("goal-tmpl-chip"), Type("button"),
		Attr("data-testid", "goal-tmpl-"+slug),
		OnClick(pick), props.Label)
}
