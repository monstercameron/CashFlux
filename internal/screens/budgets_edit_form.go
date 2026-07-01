// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverformula"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// BudgetEditFormProps drives the budget editor form rendered inside the shell-root
// flip modal (see internal/app BudgetEditHost). Mode selects which editor to show.
type BudgetEditFormProps struct {
	BudgetID string
	Mode     string // one of uistate.BudgetEditMode*
	OnDone   func() // clears the atom (closes the modal); called after a save/cancel
}

// BudgetEditForm renders the budget editor (full edit, or top-up) as the body of the
// flip modal. It owns all its form state and its own Save/Cancel buttons; the host's
// FlipPanel is NoFooter. Because the host only renders this when the atom is set, the
// component mounts fresh on each open, so the useState initializers seed correctly
// from the budget. It lives at the shell root, outside the transformed bento/tile
// ancestors, so the modal centers on the viewport.
func BudgetEditForm(props BudgetEditFormProps) ui.Node {
	// Re-render on data mutations so a stale figure can't linger while open.
	_ = uistate.UseDataRevision().Get()
	// Resolved for the cover editor's source list (hooks stay unconditional).
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	var coverPayAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, e := time.Parse("2006-01-02", pr.PayCycleAnchor); e == nil {
			coverPayAnchor = t
		}
	}

	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	// Resolve the budget (before the hooks so useState seeds from it).
	var b domain.Budget
	found := false
	if app != nil {
		for _, bb := range app.Budgets() {
			if bb.ID == props.BudgetID {
				b, found = bb, true
				break
			}
		}
	}
	cur := b.Limit.Currency
	if cur == "" {
		if app != nil {
			cur = app.Settings().BaseCurrency
		}
		if cur == "" {
			cur = "USD"
		}
	}
	dec := currency.Decimals(cur)
	limitMajor := ""
	if found {
		limitMajor = money.FormatMinor(b.Limit.Amount, dec)
	}

	// Cover-editor data: the over-budget's shortfall (prefills the amount) and every
	// other budget that has limit to give, so the user can spread the cover across
	// several. Computed for every mode so the useState seeds below can read it (cheap;
	// keeps hooks stable).
	var coverSrcs []budgetCoverSource
	coverDefaultStr, coverShortfallStr := "", ""
	if app != nil {
		cv := computeBudgetView(app, activeMemberID, vw, pr)
		for _, s := range cv.Statuses {
			if s.Budget.ID == props.BudgetID {
				sf := budgeting.CoverAmount(s)
				coverShortfallStr = fmtMoney(sf)
				if sf.IsPositive() {
					coverDefaultStr = money.FormatMinor(sf.Amount, currency.Decimals(sf.Currency))
				}
				continue
			}
			if s.Budget.Limit.Amount <= 0 {
				continue // nothing to give
			}
			coverSrcs = append(coverSrcs, budgetCoverSource{
				ID:          s.Budget.ID,
				Label:       budgetTitle(s.Budget.Name, cv.CatName[s.Budget.CategoryID]),
				RemainMinor: s.Remaining.Amount,
				LimitMinor:  s.Budget.Limit.Amount,
			})
		}
	}

	// All hooks unconditionally at stable positions (before any branch/return).
	nameS := ui.UseState(b.Name)
	varNameS := ui.UseState(b.VarName)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(b.Period))
	ownerS := ui.UseState(b.OwnerID)
	rolloverS := ui.UseState(b.Rollover)
	methodologyS := ui.UseState(b.Methodology)
	customEditVals := ui.UseState(customMapToStrings(b.Custom))
	topupAmt := ui.UseState("")
	coverAmtS := ui.UseState(coverDefaultStr)
	coverSelS := ui.UseState(map[string]bool{})            // sourceID → checked
	coverWtS := ui.UseState(map[string]int{})              // sourceID → ratio weight (default 1)
	coverMaxS := ui.UseState(map[string]bool{})            // sourceID → "use all remaining"
	amtFxS := ui.UseState(recurringAmountFormula(b) != "") // amount is a formula, not a number
	amtFormulaS := ui.UseState(recurringAmountFormula(b))
	wtFxS := ui.UseState(recurringWeightFormula(b) != "") // weights come from a formula, not numbers
	wtFormulaS := ui.UseState(recurringWeightFormula(b))
	recurringS := ui.UseState(b.RecurringCover != nil)
	coverCustomVals := ui.UseState(customMapToStrings(recurringCoverCustom(b))) // "cover" custom fields
	errS := ui.UseState("")

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onVarName := ui.UseEvent(func(v string) { varNameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	onCoverAmt := ui.UseEvent(func(v string) { coverAmtS.Set(v) })
	fullCover := ui.UseEvent(Prevent(func() { coverAmtS.Set(coverDefaultStr) }))
	toggleAmtFx := ui.UseEvent(func() { amtFxS.Set(!amtFxS.Get()) })
	onAmtFormula := ui.UseEvent(func(v string) { amtFormulaS.Set(v) })
	toggleWtFx := ui.UseEvent(func() { wtFxS.Set(!wtFxS.Get()) })
	onWtFormula := ui.UseEvent(func(v string) { wtFormulaS.Set(v) })
	onToggleRecurring := ui.UseEvent(func() { recurringS.Set(!recurringS.Get()) })
	// Plain funcs (not hooks) passed down to each per-source row component, so the
	// checkbox/weight On* handlers live in the row, not in this loop.
	onToggleSrc := func(sid string) {
		sel := coverSelS.Get()
		ns := make(map[string]bool, len(sel)+1)
		for k, v := range sel {
			ns[k] = v
		}
		ns[sid] = !ns[sid]
		coverSelS.Set(ns)
		if ns[sid] { // default a newly-checked source to weight 1 (equal split)
			w := coverWtS.Get()
			if w[sid] == 0 {
				nw := make(map[string]int, len(w)+1)
				for k, v := range w {
					nw[k] = v
				}
				nw[sid] = 1
				coverWtS.Set(nw)
			}
		}
	}
	onWeightSrc := func(sid string, val int) {
		if val < 0 {
			val = 0
		}
		w := coverWtS.Get()
		nw := make(map[string]int, len(w)+1)
		for k, v := range w {
			nw[k] = v
		}
		nw[sid] = val
		coverWtS.Set(nw)
	}
	onToggleMax := func(sid string) {
		m := coverMaxS.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[sid] = !nm[sid]
		coverMaxS.Set(nm)
	}
	submitCover := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		// Resolve the amount: a formula evaluated in this budget's context, else a number.
		var amt int64
		if amtFxS.Get() {
			m, ferr := buildCoverContext(app, pr.WeekStartWeekday(), coverPayAnchor).AmountMinor(amtFormulaS.Get(), b)
			if ferr != nil {
				errS.Set(uistate.T("budgets.coverFormulaErr", ferr.Error()))
				return
			}
			amt = m
		} else if m, perr := money.ParseMinor(strings.TrimSpace(coverAmtS.Get()), dec); perr == nil {
			amt = m
		}
		if amt < 0 {
			amt = 0
		}
		// Sources are required (that's where the money comes from), whether covering now
		// or saving a recurring rule.
		selectedAny := false
		for _, sc := range coverSrcs {
			if coverSelS.Get()[sc.ID] {
				selectedAny = true
				break
			}
		}
		if !selectedAny {
			errS.Set(uistate.T("budgets.coverPickSource"))
			return
		}
		// A one-time cover needs a positive amount; a recurring rule may evaluate to 0
		// this period (nothing to cover yet) and still be saved for future periods.
		if amt == 0 && !recurringS.Get() {
			errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		// Resolve the ratio weights once (formula-driven or fixed) so the applied split
		// and the saved recurring shares agree.
		effWeights := coverWtS.Get()
		if wtFxS.Get() {
			effWeights, _ = resolveCoverWeights(app, buildCoverContext(app, pr.WeekStartWeekday(), coverPayAnchor), coverSrcs, coverSelS.Get(), coverWtS.Get(), wtFormulaS.Get())
		}
		if amt > 0 {
			shares := splitCoverAmount(amt, coverSrcs, coverSelS.Get(), effWeights, coverMaxS.Get())
			// Pre-validate so a partial failure can't leave a half-applied cover: each
			// source must keep a positive limit after giving its share.
			for _, sc := range coverSrcs {
				if share, ok := shares[sc.ID]; ok && share >= sc.LimitMinor {
					errS.Set(uistate.T("budgets.coverSourceShort", sc.Label))
					return
				}
			}
			for _, sc := range coverSrcs {
				share, ok := shares[sc.ID]
				if !ok || share <= 0 {
					continue
				}
				if err := app.CoverBudget(sc.ID, props.BudgetID, money.New(share, cur)); err != nil {
					errS.Set(err.Error())
					return
				}
			}
		}
		// Save / clear the standing recurring arrangement per the toggle (re-fetch the
		// destination since CoverBudget just mutated the budget set).
		start, _ := budgeting.PeriodRange(b.Period, time.Now(), pr.WeekStartWeekday())
		for _, nb := range app.Budgets() {
			if nb.ID != props.BudgetID {
				continue
			}
			if recurringS.Get() {
				wtFormula := ""
				if wtFxS.Get() {
					wtFormula = strings.TrimSpace(wtFormulaS.Get())
				}
				var srcs []domain.CoverShare
				for _, sc := range coverSrcs {
					if coverSelS.Get()[sc.ID] {
						w := effWeights[sc.ID]
						if w <= 0 {
							w = 1
						}
						// Store the shared per-source formula (re-evaluated each period in
						// that source's context) alongside the last-resolved weight as a record.
						srcs = append(srcs, domain.CoverShare{BudgetID: sc.ID, Weight: w, WeightFormula: wtFormula})
					}
				}
				rc := &domain.RecurringCover{Sources: srcs, LastAppliedPeriod: start.Format("2006-01-02")}
				if amtFxS.Get() {
					rc.AmountFormula = strings.TrimSpace(amtFormulaS.Get())
				}
				rc.AmountMinor = amt // fixed amount, or the last-evaluated value as a record
				if defs := app.CustomFieldDefsFor("cover"); len(defs) > 0 {
					rc.Custom = customValuesToMap(defs, coverCustomVals.Get())
				}
				nb.RecurringCover = rc
			} else {
				nb.RecurringCover = nil
			}
			_ = app.PutBudget(nb)
			break
		}
		uistate.BumpDataRevision()
		done()
	}))
	cancel := ui.UseEvent(Prevent(func() { done() }))
	onCustom := func(key, value string) {
		m := customEditVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[key] = value
		customEditVals.Set(nm)
	}
	onCoverCustom := func(key, value string) {
		m := coverCustomVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[key] = value
		coverCustomVals.Set(nm)
	}

	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		for _, bb := range app.Budgets() {
			if bb.ID != props.BudgetID {
				continue
			}
			if n := strings.TrimSpace(nameS.Get()); n != "" {
				bb.Name = n
			}
			// Refuse a variable name that collides with another budget's handle.
			if warn := budgetVarCollision(app, props.BudgetID, varNameS.Get(), nameS.Get()); warn != "" {
				errS.Set(warn)
				return
			}
			bb.VarName = strings.TrimSpace(varNameS.Get())
			amt, err := money.ParseMinor(strings.TrimSpace(limitS.Get()), dec)
			if err != nil || amt <= 0 {
				errS.Set(uistate.T("budgets.limitRequired"))
				return
			}
			bb.Limit = money.New(amt, cur)
			if p := domain.Period(periodS.Get()); p.Valid() {
				bb.Period = p
			}
			bb.OwnerID = ownerS.Get()
			if ownerS.Get() == domain.GroupOwnerID {
				bb.Scope = domain.ScopeShared
			} else {
				bb.Scope = domain.ScopeIndividual
			}
			bb.Rollover = rolloverS.Get()
			if m := budgeting.Methodology(methodologyS.Get()); m.Valid() {
				bb.Methodology = methodologyS.Get()
			} else {
				bb.Methodology = ""
			}
			if defs := app.CustomFieldDefsFor("budget"); len(defs) > 0 {
				bb.Custom = customValuesToMap(defs, customEditVals.Get())
			}
			if err := app.PutBudget(bb); err != nil {
				errS.Set(err.Error())
				return
			}
			break
		}
		uistate.BumpDataRevision()
		done()
	}))

	submitTopup := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(topupAmt.Get()), dec)
		if err != nil || amt <= 0 {
			errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		for _, bb := range app.Budgets() {
			if bb.ID != props.BudgetID {
				continue
			}
			bb.Limit = money.New(bb.Limit.Amount+amt, cur)
			if err := app.PutBudget(bb); err != nil {
				errS.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("budgets.toppedUpToast", fmtMoney(money.New(amt, cur))), false)
			done()
			return
		}
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	// --- Cover: spread the overspend across one or more source budgets. ---
	if props.Mode == uistate.BudgetEditModeCover {
		if len(coverSrcs) == 0 {
			return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("budgets.coverNoSources")))
		}
		// Effective amount + live preview: in formula mode evaluate the amount formula in
		// this budget's context; otherwise parse the number. The shares below use it.
		coverCtx := buildCoverContext(app, pr.WeekStartWeekday(), coverPayAnchor)
		var amtMinor int64
		amtPreview, amtFxErr := "—", ""
		if amtFxS.Get() {
			if strings.TrimSpace(amtFormulaS.Get()) != "" {
				if m, ferr := coverCtx.AmountMinor(amtFormulaS.Get(), b); ferr != nil {
					amtFxErr = ferr.Error()
				} else {
					amtMinor, amtPreview = m, fmtMoney(money.New(m, cur))
				}
			}
		} else {
			amtMinor, _ = money.ParseMinor(strings.TrimSpace(coverAmtS.Get()), dec)
		}
		// Effective ratio weights: a per-source formula (evaluated in each source's own
		// context) or the fixed ratio inputs. The shares + the displayed ratios use these.
		effWeights := coverWtS.Get()
		wtFxErr := ""
		if wtFxS.Get() {
			effWeights, wtFxErr = resolveCoverWeights(app, coverCtx, coverSrcs, coverSelS.Get(), coverWtS.Get(), wtFormulaS.Get())
		}
		shares := splitCoverAmount(amtMinor, coverSrcs, coverSelS.Get(), effWeights, coverMaxS.Get())
		spreadSub := uistate.T("budgets.coverSpreadSub")
		if wtFxS.Get() {
			spreadSub = uistate.T("budgets.coverWeightFxSub")
		}
		// Keyed single list sorted checked-first, so the picked budgets cluster at the top
		// (the CSS tints them into one visible group and rules a divider before the rest).
		// A single keyed list — rather than two — keeps each row's DOM node across a toggle,
		// so a click never detaches the element being interacted with.
		selNow := coverSelS.Get()
		var selCount int
		var selTotal int64
		for _, sc := range coverSrcs {
			if selNow[sc.ID] {
				selCount++
				selTotal += shares[sc.ID]
			}
		}
		sortedCoverSrcs := make([]budgetCoverSource, len(coverSrcs))
		copy(sortedCoverSrcs, coverSrcs)
		sort.SliceStable(sortedCoverSrcs, func(i, j int) bool {
			return selNow[sortedCoverSrcs[i].ID] && !selNow[sortedCoverSrcs[j].ID]
		})
		renderSrc := func(sc budgetCoverSource) ui.Node {
			shareStr, over := "", false
			if sh, ok := shares[sc.ID]; ok && sh > 0 {
				shareStr = fmtMoney(money.New(sh, cur))
				over = sh >= sc.LimitMinor // a share this source can't fully give
			}
			return ui.CreateElement(coverSourceRow, coverSourceRowProps{
				ID: sc.ID, Label: sc.Label, RemainStr: fmtMoney(money.New(sc.RemainMinor, cur)),
				Selected: selNow[sc.ID], Weight: effWeights[sc.ID], ShareStr: shareStr,
				Over: over, AvailStr: fmtMoney(money.New(sc.LimitMinor, cur)), Max: coverMaxS.Get()[sc.ID],
				WeightLocked: wtFxS.Get(), OnToggle: onToggleSrc, OnWeight: onWeightSrc, OnToggleMax: onToggleMax,
			})
		}
		srcKey := func(sc budgetCoverSource) any { return sc.ID }
		return Form(css.Class("acct-edit-form"), OnSubmit(submitCover),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.coverHint", coverShortfallStr)),
			// Amount: a number or a formula (ƒx toggle). A formula is evaluated live in
			// this budget's context and re-evaluated each period when recurring.
			labeledField(uistate.T("budgets.amountToMove"),
				Div(css.Class("cover-amount-block"),
					Div(css.Class("cover-amount-row"),
						IfElse(amtFxS.Get(),
							Input(css.Class("field"), Attr("id", "budget-cover-formula"), Type("text"),
								Placeholder("overspend"), Value(amtFormulaS.Get()), OnInput(onAmtFormula)),
							Input(css.Class("field"), Attr("id", "budget-cover-amt"), Attr("autofocus", ""), Type("number"),
								Attr("aria-label", uistate.T("budgets.amountToMove")), Placeholder(uistate.T("budgets.amountToMove")),
								Value(coverAmtS.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onCoverAmt))),
						If(!amtFxS.Get() && coverDefaultStr != "", Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.fullOverspendTitle")),
							OnClick(fullCover), uistate.T("budgets.coverFull", coverShortfallStr))),
						Button(css.Class("btn", "cover-fx-toggle"), Type("button"), Attr("aria-pressed", ariaBool(amtFxS.Get())),
							Attr("data-testid", "cover-fx-toggle"), Title(uistate.T("budgets.coverFormulaTitle")), OnClick(toggleAmtFx), "ƒx"),
					),
					If(amtFxS.Get() && amtFxErr == "", Span(css.Class("cover-fx-preview"), Attr("data-testid", "cover-fx-preview"), uistate.T("budgets.coverFormulaPreview", amtPreview))),
					If(amtFxErr != "", Span(css.Class("cover-fx-err"), uistate.T("budgets.coverFormulaErr", amtFxErr))),
					If(amtFxS.Get(), Span(css.Class("cover-fx-hint"), uistate.T("budgets.coverFormulaHint"))),
				)),
			// Multi-source: check the budgets to pull from (split equally by default);
			// the ratio input per source lets the split be unequal — or drive every
			// source's ratio from a single formula (ƒx) evaluated in each source's context.
			Div(css.Class("cover-spread-head"),
				Div(css.Class("cover-spread-titles"),
					Span(css.Class("cover-spread-label"), uistate.T("budgets.coverSpreadLabel")),
					Span(css.Class("cover-spread-sub"), spreadSub),
				),
				Button(css.Class("btn", "cover-fx-toggle"), Type("button"), Attr("aria-pressed", ariaBool(wtFxS.Get())),
					Attr("data-testid", "cover-wt-fx-toggle"), Title(uistate.T("budgets.coverWeightFxTitle")), OnClick(toggleWtFx), "ƒx"),
			),
			If(wtFxS.Get(), Div(css.Class("cover-weight-fx"),
				Input(css.Class("field"), Attr("id", "budget-cover-wt-formula"), Type("text"),
					Placeholder("cf_budget_priority"), Value(wtFormulaS.Get()), OnInput(onWtFormula)),
				If(wtFxErr != "", Span(css.Class("cover-fx-err"), uistate.T("budgets.coverFormulaErr", wtFxErr))),
				Span(css.Class("cover-fx-hint"), uistate.T("budgets.coverWeightFxHint")),
			)),
			// A running "Selected N · splitting $X" caption pinned above the list, so the
			// split total reads at a glance even as rows scroll. Kept always-present (hidden
			// when empty) so toggling it never restructures the siblings and forces the keyed
			// list below to remount — which would detach a row mid-click.
			Span(ClassStr(coverCapClass(selCount)), coverCapText(selCount, selTotal, cur)),
			// One keyed list, sorted so the picked budgets cluster at the top; the CSS tints
			// the checked group and rules a divider before the remaining sources.
			Div(css.Class("cover-sources"),
				MapKeyed(sortedCoverSrcs, srcKey, renderSrc),
			),
			// Repeat this cover automatically at the start of each new period.
			Label(css.Class("cover-recurring-toggle"),
				Input(append([]any{Type("checkbox"), Attr("data-testid", "cover-recurring"), OnChange(onToggleRecurring)}, checkedAttr(recurringS.Get())...)...),
				Span(uistate.T("budgets.coverRecurring")),
			),
			// Custom fields on the standing rule (metadata like a reason / review-by), shown
			// only when recurring is on and the household has defined "cover" fields.
			If(recurringS.Get() && len(app.CustomFieldDefsFor("cover")) > 0,
				Div(css.Class("cover-custom-fields"),
					P(css.Class("cover-fx-hint"), uistate.T("budgets.coverCustomHint")),
					MapKeyed(app.CustomFieldDefsFor("cover"), func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
						return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: coverCustomVals.Get()[d.Key], OnChange: onCoverCustom}))
					}),
				)),
			errLine,
			Div(css.Class("acct-edit-actions"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("budgets.coverAction")),
			),
		)
	}

	// --- Top-up: a single amount that raises the budget's limit. ---
	if props.Mode == uistate.BudgetEditModeTopup {
		return Form(css.Class("acct-edit-form"), OnSubmit(submitTopup),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.topupHint", budgetTitle(b.Name, budgetCategoryName(app, b.CategoryID)), fmtMoney(b.Limit))),
			labeledField(uistate.T("budgets.amountToAdd"),
				Input(css.Class("field"), Attr("id", "budget-topup-amt"), Attr("autofocus", ""), Type("number"),
					Attr("aria-label", uistate.T("budgets.amountToAdd")), Placeholder(uistate.T("budgets.amountToAdd")),
					Value(topupAmt.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onTopupAmt))),
			errLine,
			Div(css.Class("acct-edit-actions"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("budgets.addFunds")),
			),
		)
	}

	// --- Full edit. Single-column form (.acct-edit-form) — a clean vertical stack so
	// the rollover explanation and Method sit full-width beneath their controls, and the
	// action row's margin-top:auto pins Save/Cancel to the modal's bottom. ---
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "budget-edit-name"), Attr("autofocus", ""), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
		// Variable name: an optional explicit handle for this budget in formulas/widgets.
		// Empty falls back to the display name; the resolved variable is previewed live.
		labeledField(uistate.T("budgets.varNameLabel"),
			Div(css.Class("cover-amount-block"),
				Input(css.Class("field"), Attr("id", "budget-edit-varname"), Type("text"),
					Placeholder(budgetVarPlaceholder(nameS.Get())), Value(varNameS.Get()), OnInput(onVarName)),
				Span(css.Class("cover-fx-hint"), uistate.T("budgets.varNameHint", budgetVarPreview(varNameS.Get(), nameS.Get()))),
				If(budgetVarCollision(app, props.BudgetID, varNameS.Get(), nameS.Get()) != "",
					Span(css.Class("cover-fx-err"), Attr("data-testid", "budget-varname-warn"),
						budgetVarCollision(app, props.BudgetID, varNameS.Get(), nameS.Get()))),
			)),
		labeledField(uistate.T("budgets.limitLabel"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("budgets.limitLabel")), Value(limitS.Get()), Step("0.01"), OnInput(onLimit))),
		labeledField(uistate.T("budgets.period"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: periodOptions(periodS.Get()), Selected: periodS.Get(),
				OnChange: func(v string) { periodS.Set(v) }, AriaLabel: uistate.T("budgets.period"),
			})),
		labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: ownerSelectOptions(app.Members(), ownerS.Get()), Selected: ownerS.Get(),
				OnChange: func(v string) { ownerS.Set(v) }, AriaLabel: uistate.T("common.owner"),
			})),
		Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("style", "flex-wrap:nowrap"),
			Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), OnChange(onRollover)}, checkedAttr(rolloverS.Get())...)...),
			Span(uistate.T("budgets.rollover")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.rolloverHint")),
		labeledField(uistate.T("budgets.methodLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: budgetMethodOptions(methodologyS.Get()), Selected: methodologyS.Get(),
				OnChange: func(v string) { methodologyS.Set(v) }, AriaLabel: uistate.T("budgets.methodLabel"),
			})),
		// Custom fields: one input per user-defined "budget" field (renders nothing
		// when there are no defs).
		MapKeyed(app.CustomFieldDefsFor("budget"), func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customEditVals.Get()[d.Key], OnChange: onCustom}))
		}),
		errLine,
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

// budgetCoverSource is one budget offered as a funding source in the cover editor:
// its id, a display label, its remaining room, and its base limit (used to validate a
// share can't zero it out).
type budgetCoverSource struct {
	ID          string
	Label       string
	RemainMinor int64
	LimitMinor  int64
}

// coverMaxGive is the most a source can contribute — its remaining room, capped one
// cent below its limit so the cover leaves it a positive limit.
func coverMaxGive(sc budgetCoverSource) int64 {
	g := sc.RemainMinor
	if capMax := sc.LimitMinor - 1; g > capMax {
		g = capMax
	}
	if g < 0 {
		g = 0
	}
	return g
}

// splitCoverAmount divides totalMinor across the checked sources. Sources in `maxed`
// are pinned to their full remaining (coverMaxGive) and the rest of the amount is
// split among the remaining (non-maxed) checked sources by weight — so checking "use
// all remaining" on one source back-fills the ratios of the others. When the pinned
// sources already meet or exceed the amount, they're scaled down to sum exactly to it
// and the non-maxed sources get nothing. Any rounding remainder lands on the last
// recipient so the shares sum exactly to totalMinor. Returns nil when nothing is
// selected or the total ≤ 0.
func splitCoverAmount(totalMinor int64, srcs []budgetCoverSource, sel map[string]bool, weights map[string]int, maxed map[string]bool) map[string]int64 {
	if totalMinor <= 0 {
		return nil
	}
	byID := map[string]budgetCoverSource{}
	var maxedIDs, freeIDs []string
	for _, sc := range srcs {
		if !sel[sc.ID] {
			continue
		}
		byID[sc.ID] = sc
		if maxed[sc.ID] {
			maxedIDs = append(maxedIDs, sc.ID)
		} else {
			freeIDs = append(freeIDs, sc.ID)
		}
	}
	if len(maxedIDs)+len(freeIDs) == 0 {
		return nil
	}
	out := make(map[string]int64, len(maxedIDs)+len(freeIDs))

	// Pinned "use all remaining" sources give their full remaining.
	var pinned int64
	for _, id := range maxedIDs {
		g := coverMaxGive(byID[id])
		out[id] = g
		pinned += g
	}

	// Pinned sources already cover the amount → scale them to sum exactly to it.
	if pinned >= totalMinor {
		var assigned int64
		for i, id := range maxedIDs {
			var share int64
			if i == len(maxedIDs)-1 {
				share = totalMinor - assigned
			} else if pinned > 0 {
				share = totalMinor * out[id] / pinned
			}
			assigned += share
			out[id] = share
		}
		for _, id := range freeIDs {
			out[id] = 0
		}
		return out
	}

	// Split the remainder across the non-maxed sources by weight.
	rest := totalMinor - pinned
	totalWeight := 0
	for _, id := range freeIDs {
		w := weights[id]
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	if len(freeIDs) == 0 || totalWeight == 0 {
		// Nowhere to put the remainder — hand it to a pinned source (best effort).
		if len(maxedIDs) > 0 {
			out[maxedIDs[len(maxedIDs)-1]] += rest
		}
		return out
	}
	var assigned int64
	for i, id := range freeIDs {
		w := weights[id]
		if w <= 0 {
			w = 1
		}
		var share int64
		if i == len(freeIDs)-1 {
			share = rest - assigned
		} else {
			share = rest * int64(w) / int64(totalWeight)
		}
		assigned += share
		out[id] = share
	}
	return out
}

// coverSourceRowProps drives one row of the cover editor's source list. On* handlers
// live here (not in the parent's map loop) per the framework's no-hooks-in-loops rule.
type coverSourceRowProps struct {
	ID           string
	Label        string
	RemainStr    string
	Selected     bool
	Weight       int
	ShareStr     string // formatted amount this source contributes (when selected)
	Over         bool   // this source's share exceeds what it can give
	AvailStr     string // formatted amount this source can give (its limit)
	Max          bool   // "use all remaining" — pin this source to its full remaining
	WeightLocked bool   // ratio is driven by a formula → the number input is read-only
	OnToggle     func(string)
	OnWeight     func(string, int)
	OnToggleMax  func(string)
}

// coverSourceRow renders a styled checkbox to include a source budget, and — when
// checked — a ratio input, a "use all remaining" toggle, and the live amount it yields.
// Checking "use all remaining" pins the source to its full remaining (the ratio input
// is then disabled) and back-fills the other selected sources' ratios; an over-limit
// share turns amber with an "only $X available" note.
func coverSourceRow(props coverSourceRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	onWeight := ui.UseEvent(func(v string) { n, _ := strconv.Atoi(strings.TrimSpace(v)); props.OnWeight(props.ID, n) })
	onMax := ui.UseEvent(func() { props.OnToggleMax(props.ID) })
	w := props.Weight
	if props.Selected && w <= 0 {
		w = 1
	}
	rowCls := "cover-src-row"
	if props.Selected {
		rowCls += " is-checked"
	}
	shareCls := "cover-src-share"
	if props.Over {
		shareCls += " is-over"
	}
	weightAttrs := []any{css.Class("field", "cover-src-weight"), Type("number"), Attr("min", "0"),
		Attr("aria-label", uistate.T("budgets.coverRatio")), Value(strconv.Itoa(w)), Step("1"), OnInput(onWeight)}
	if props.Max || props.WeightLocked {
		// Pinned to full remaining, or driven by the shared ratio formula — either way the
		// number isn't hand-edited here.
		weightAttrs = append(weightAttrs, Attr("disabled", "disabled"))
	}
	return Div(css.Class(rowCls),
		Label(css.Class("cover-src-main"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "cover-src-"+props.ID), OnChange(onToggle)}, checkedAttr(props.Selected)...)...),
			Span(css.Class("cover-src-name"), props.Label),
			Span(css.Class("cover-src-remain"), props.RemainStr+" left"),
		),
		If(props.Selected, Div(css.Class("cover-src-ratio"),
			Span(css.Class("cover-src-ratio-label"), uistate.T("budgets.coverRatio")),
			Input(weightAttrs...),
			Label(css.Class("cover-src-maxlabel"), Title(uistate.T("budgets.coverUseAllTitle")),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "cover-max-"+props.ID), OnChange(onMax)}, checkedAttr(props.Max)...)...),
				Span(uistate.T("budgets.coverUseAll")),
			),
			Div(css.Class("cover-src-shares"),
				Span(ClassStr(shareCls), If(props.Over, Span(css.Class("cover-src-warn"), "⚠ ")), props.ShareStr),
				If(props.Over, Span(css.Class("cover-src-avail"), uistate.T("budgets.coverOnlyAvail", props.AvailStr))),
			),
		)),
	)
}

// buildCoverContext assembles the coverformula evaluation context (the global engine
// surface + the live data) for the cover modal's amount/weight formula previews. Same
// shape the recurring boot apply builds.
func buildCoverContext(app *appstate.App, weekStart time.Weekday, payCycleAnchor time.Time) coverformula.Context {
	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	ms, me := dateutil.MonthRange(now)
	surface := engineenv.Vars(engineenv.Data{
		Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
		Rates: rates, Now: now, PeriodStart: ms, PeriodEnd: me, WeekStart: weekStart,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
	})
	return coverformula.Context{
		Base: surface, Txns: app.Transactions(), Rates: rates, Now: now,
		WeekStart: weekStart, PayCycleAnchor: payCycleAnchor, Defs: app.CustomFieldDefs(),
	}
}

// recurringAmountFormula returns the budget's stored recurring cover amount formula
// (empty when none).
func recurringAmountFormula(b domain.Budget) string {
	if b.RecurringCover != nil {
		return b.RecurringCover.AmountFormula
	}
	return ""
}

// coverCapClass hides the "Selected …" caption when nothing is picked (it stays in the
// DOM so toggling it never restructures the source list's siblings).
func coverCapClass(selCount int) string {
	if selCount == 0 {
		return "cover-selected-cap is-empty"
	}
	return "cover-selected-cap"
}

// coverCapText is the running "Selected N · splitting $X" caption (empty when nothing
// is picked, since the span is then hidden).
func coverCapText(selCount int, totalMinor int64, cur string) string {
	if selCount == 0 {
		return ""
	}
	return uistate.T("budgets.coverSelectedCap", selCount, fmtMoney(money.New(totalMinor, cur)))
}

// budgetVarPlaceholder is the auto-derived variable slug for a name, shown as the
// var-name field's placeholder so the user sees what they'd get by leaving it blank.
func budgetVarPlaceholder(name string) string {
	if s := engineenv.BudgetVarSlug(name); s != "" {
		return s
	}
	return uistate.T("budgets.varNamePlaceholder")
}

// budgetVarPreview renders an example of the resolved variable a budget will expose,
// e.g. "budget_rent_remaining" — using the explicit var name when set, else the name.
func budgetVarPreview(varName, name string) string {
	src := varName
	if src == "" {
		src = name
	}
	slug := engineenv.BudgetVarSlug(src)
	if slug == "" {
		slug = "…"
	}
	return "budget_" + slug + "_remaining"
}

// budgetVarCollision returns a warning when the resolved variable slug for (varName else
// name) clashes with another budget's variable — so two budgets can't silently produce
// the same handle. Empty when there's no clash.
func budgetVarCollision(app *appstate.App, selfID, varName, name string) string {
	if app == nil {
		return ""
	}
	src := varName
	if src == "" {
		src = name
	}
	slug := engineenv.BudgetVarSlug(src)
	if slug == "" {
		return ""
	}
	for _, bb := range app.Budgets() {
		if bb.ID == selfID {
			continue
		}
		other := bb.VarName
		if other == "" {
			other = bb.Name
		}
		if engineenv.BudgetVarSlug(other) == slug {
			return uistate.T("budgets.varNameTaken", bb.Name)
		}
	}
	return ""
}

// recurringCoverCustom returns the custom-field values stored on a budget's recurring
// cover rule (nil when there's no rule).
func recurringCoverCustom(b domain.Budget) map[string]any {
	if b.RecurringCover == nil {
		return nil
	}
	return b.RecurringCover.Custom
}

// recurringWeightFormula returns the shared per-source weight formula stored on a
// recurring cover (empty when none). All sources carry the same formula, so the first
// non-empty one is the modal-level formula to restore.
func recurringWeightFormula(b domain.Budget) string {
	if b.RecurringCover == nil {
		return ""
	}
	for _, s := range b.RecurringCover.Sources {
		if s.WeightFormula != "" {
			return s.WeightFormula
		}
	}
	return ""
}

// resolveCoverWeights returns the effective per-source ratio weight for each selected
// source. When wtFormula is set, each source's weight is that formula evaluated in the
// source budget's own context (e.g. "cf_budget_priority" weights sources by a custom
// priority field); otherwise the fixed ratio inputs are used. The second return is the
// first formula error encountered (for the live preview), if any.
func resolveCoverWeights(app *appstate.App, ctx coverformula.Context, srcs []budgetCoverSource, sel map[string]bool, fixed map[string]int, wtFormula string) (map[string]int, string) {
	out := make(map[string]int, len(srcs))
	formula := strings.TrimSpace(wtFormula)
	var byID map[string]domain.Budget
	if formula != "" {
		byID = make(map[string]domain.Budget, len(app.Budgets()))
		for _, bb := range app.Budgets() {
			byID[bb.ID] = bb
		}
	}
	errStr := ""
	for _, sc := range srcs {
		if !sel[sc.ID] {
			continue
		}
		if formula != "" {
			w, err := ctx.Weight(formula, byID[sc.ID])
			if err != nil {
				if errStr == "" {
					errStr = err.Error()
				}
				out[sc.ID] = 0
				continue
			}
			out[sc.ID] = w
			continue
		}
		w := fixed[sc.ID]
		if w <= 0 {
			w = 1
		}
		out[sc.ID] = w
	}
	return out, errStr
}

// budgetCategoryName resolves a category's display name (for the top-up hint), or ""
// when the budget has no category or it can't be found.
func budgetCategoryName(app *appstate.App, categoryID string) string {
	if app == nil || categoryID == "" {
		return ""
	}
	for _, c := range app.Categories() {
		if c.ID == categoryID {
			return c.Name
		}
	}
	return ""
}
