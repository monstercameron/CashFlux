// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(b.Period))
	ownerS := ui.UseState(b.OwnerID)
	rolloverS := ui.UseState(b.Rollover)
	methodologyS := ui.UseState(b.Methodology)
	customEditVals := ui.UseState(customMapToStrings(b.Custom))
	topupAmt := ui.UseState("")
	coverAmtS := ui.UseState(coverDefaultStr)
	coverSelS := ui.UseState(map[string]bool{}) // sourceID → checked
	coverWtS := ui.UseState(map[string]int{})   // sourceID → ratio weight (default 1)
	recurringS := ui.UseState(b.RecurringCover != nil)
	errS := ui.UseState("")

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	onCoverAmt := ui.UseEvent(func(v string) { coverAmtS.Set(v) })
	fullCover := ui.UseEvent(Prevent(func() { coverAmtS.Set(coverDefaultStr) }))
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
	submitCover := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(coverAmtS.Get()), dec)
		if err != nil || amt <= 0 {
			errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		shares := splitCoverAmount(amt, coverSrcs, coverSelS.Get(), coverWtS.Get())
		if len(shares) == 0 {
			errS.Set(uistate.T("budgets.coverPickSource"))
			return
		}
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
		// Save / clear the standing recurring arrangement per the toggle (re-fetch the
		// destination since CoverBudget just mutated the budget set).
		start, _ := budgeting.PeriodRange(b.Period, time.Now(), pr.WeekStartWeekday())
		for _, nb := range app.Budgets() {
			if nb.ID != props.BudgetID {
				continue
			}
			if recurringS.Get() {
				var srcs []domain.CoverShare
				for _, sc := range coverSrcs {
					if coverSelS.Get()[sc.ID] {
						w := coverWtS.Get()[sc.ID]
						if w <= 0 {
							w = 1
						}
						srcs = append(srcs, domain.CoverShare{BudgetID: sc.ID, Weight: w})
					}
				}
				nb.RecurringCover = &domain.RecurringCover{AmountMinor: amt, Sources: srcs, LastAppliedPeriod: start.Format("2006-01-02")}
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
		// Live per-source shares from the current amount + selection + weights.
		amtMinor, _ := money.ParseMinor(strings.TrimSpace(coverAmtS.Get()), dec)
		shares := splitCoverAmount(amtMinor, coverSrcs, coverSelS.Get(), coverWtS.Get())
		return Form(css.Class("acct-edit-form"), OnSubmit(submitCover),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.coverHint", coverShortfallStr)),
			labeledField(uistate.T("budgets.amountToMove"),
				Div(css.Class("cover-amount-row"),
					Input(css.Class("field"), Attr("id", "budget-cover-amt"), Attr("autofocus", ""), Type("number"),
						Attr("aria-label", uistate.T("budgets.amountToMove")), Placeholder(uistate.T("budgets.amountToMove")),
						Value(coverAmtS.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onCoverAmt)),
					If(coverDefaultStr != "", Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.fullOverspendTitle")),
						OnClick(fullCover), uistate.T("budgets.coverFull", coverShortfallStr))),
				)),
			// Multi-source: check the budgets to pull from (split equally by default);
			// the ratio input per source lets the split be unequal.
			Div(
				Span(css.Class("cover-spread-label"), uistate.T("budgets.coverSpreadLabel")),
				Span(css.Class("cover-spread-sub"), uistate.T("budgets.coverSpreadSub")),
			),
			Div(css.Class("cover-sources"),
				MapKeyed(coverSrcs, func(sc budgetCoverSource) any { return sc.ID }, func(sc budgetCoverSource) ui.Node {
					shareStr, over := "", false
					if sh, ok := shares[sc.ID]; ok && sh > 0 {
						shareStr = fmtMoney(money.New(sh, cur))
						over = sh >= sc.LimitMinor // a share this source can't fully give
					}
					return ui.CreateElement(coverSourceRow, coverSourceRowProps{
						ID: sc.ID, Label: sc.Label, RemainStr: fmtMoney(money.New(sc.RemainMinor, cur)),
						Selected: coverSelS.Get()[sc.ID], Weight: coverWtS.Get()[sc.ID], ShareStr: shareStr,
						Over: over, AvailStr: fmtMoney(money.New(sc.LimitMinor, cur)),
						OnToggle: onToggleSrc, OnWeight: onWeightSrc,
					})
				}),
			),
			// Repeat this cover automatically at the start of each new period.
			Label(css.Class("cover-recurring-toggle"),
				Input(append([]any{Type("checkbox"), Attr("data-testid", "cover-recurring"), OnChange(onToggleRecurring)}, checkedAttr(recurringS.Get())...)...),
				Span(uistate.T("budgets.coverRecurring")),
			),
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

// splitCoverAmount divides totalMinor across the checked sources in proportion to
// their weights (an unset/≤0 weight counts as 1, so the default is an equal split),
// distributing any rounding remainder to the first selected source so the shares sum
// exactly to totalMinor. Returns nil when nothing is selected or the total ≤ 0.
func splitCoverAmount(totalMinor int64, srcs []budgetCoverSource, sel map[string]bool, weights map[string]int) map[string]int64 {
	if totalMinor <= 0 {
		return nil
	}
	var ids []string
	totalWeight := 0
	for _, sc := range srcs {
		if !sel[sc.ID] {
			continue
		}
		w := weights[sc.ID]
		if w <= 0 {
			w = 1
		}
		ids = append(ids, sc.ID)
		totalWeight += w
	}
	if len(ids) == 0 || totalWeight == 0 {
		return nil
	}
	out := make(map[string]int64, len(ids))
	var assigned int64
	for _, id := range ids {
		w := weights[id]
		if w <= 0 {
			w = 1
		}
		share := totalMinor * int64(w) / int64(totalWeight)
		out[id] = share
		assigned += share
	}
	if rem := totalMinor - assigned; rem != 0 {
		out[ids[0]] += rem
	}
	return out
}

// coverSourceRowProps drives one row of the cover editor's source list. On* handlers
// live here (not in the parent's map loop) per the framework's no-hooks-in-loops rule.
type coverSourceRowProps struct {
	ID        string
	Label     string
	RemainStr string
	Selected  bool
	Weight    int
	ShareStr  string // formatted amount this source contributes (when selected)
	Over      bool   // this source's share exceeds what it can give
	AvailStr  string // formatted amount this source can give (its limit)
	OnToggle  func(string)
	OnWeight  func(string, int)
}

// coverSourceRow renders a styled checkbox to include a source budget, and — when
// checked — a ratio input (default 1) plus the live amount that ratio yields. When the
// share exceeds the source's limit it turns amber with an "only $X available" note.
func coverSourceRow(props coverSourceRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	onWeight := ui.UseEvent(func(v string) { n, _ := strconv.Atoi(strings.TrimSpace(v)); props.OnWeight(props.ID, n) })
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
	return Div(css.Class(rowCls),
		Label(css.Class("cover-src-main"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "cover-src-"+props.ID), OnChange(onToggle)}, checkedAttr(props.Selected)...)...),
			Span(css.Class("cover-src-name"), props.Label),
			Span(css.Class("cover-src-remain"), props.RemainStr+" left"),
		),
		If(props.Selected, Div(css.Class("cover-src-ratio"),
			Span(css.Class("cover-src-ratio-label"), uistate.T("budgets.coverRatio")),
			Input(css.Class("field", "cover-src-weight"), Type("number"), Attr("min", "0"),
				Attr("aria-label", uistate.T("budgets.coverRatio")), Value(strconv.Itoa(w)), Step("1"), OnInput(onWeight)),
			Div(css.Class("cover-src-shares"),
				Span(ClassStr(shareCls), If(props.Over, Span(css.Class("cover-src-warn"), "⚠ ")), props.ShareStr),
				If(props.Over, Span(css.Class("cover-src-avail"), uistate.T("budgets.coverOnlyAvail", props.AvailStr))),
			),
		)),
	)
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
