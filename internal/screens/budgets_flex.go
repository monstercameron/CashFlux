// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetFlexWidget is the flex-budgeting view (BG2), shown only when the
// methodology is Flex. It is the methodology for people who aren't budget
// hobbyists: ONE flex number for all day-to-day discretionary spending (the
// signature meter), a checklist of fixed commitments (expected-vs-actual), and a
// list of non-monthly set-asides (XC3 smoothed accruals). It reads the live store
// and evaluates through the pure budgeting.EvaluateFlex read model.
func budgetFlexWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	if app == nil {
		return Fragment()
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	pr := uistate.UsePrefs().Get()
	now := time.Now()
	start, end := budgeting.PeriodRange(domain.PeriodMonthly, now, pr.WeekStartWeekday())

	target := app.Settings().FlexBudget
	view := budgeting.EvaluateFlex(app.Categories(), app.Transactions(), app.Recurring(), target, base, start, end)

	// The flex number is edited inline (staged in local state, saved on click).
	editing := ui.UseState(false)
	draft := ui.UseState(money.FormatMinor(target, dec))
	onEdit := ui.UseEvent(func() { draft.Set(money.FormatMinor(app.Settings().FlexBudget, dec)); editing.Set(true) })
	onDraft := ui.UseEvent(func(e ui.Event) { draft.Set(e.GetValue()) })
	onSaveTarget := ui.UseEvent(func() {
		amt, err := money.ParseMinor(strings.TrimSpace(draft.Get()), dec)
		if err != nil {
			return
		}
		s := app.Settings()
		s.FlexBudget = amt
		_ = app.PutSettings(s)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		editing.Set(false)
	})
	openSheet := ui.UseEvent(func() { uistate.UseFlexSheetOpen().Set(true) })

	return Div(css.Class("bento-tile budgets-flex"), Attr("data-testid", "budget-flex"),
		flexMeterHeader(view, base, dec, target, editing.Get(), draft.Get(), onEdit, onDraft, onSaveTarget, openSheet),
		flexFixedSection(view),
		flexNonMonthlySection(view),
	)
}

// flexMeterHeader renders the signature flex meter: the pooled spent-vs-target
// number with an inline editor for the flex budget and the classify action.
func flexMeterHeader(view budgeting.FlexView, base string, dec int, target int64, editing bool, draft string, onEdit, onDraft, onSave, openSheet ui.Handler) ui.Node {
	var status ui.Node
	tone := "bg-accent"
	// No flex number set yet: the meter must read NEUTRAL and EMPTY, not a full
	// green "healthy" bar — spending against an unset target isn't "on track".
	unset := target <= 0
	if unset {
		tone = "bg-dim"
	}
	switch {
	case unset:
		status = Span(css.Class(tw.TextDim), uistate.T("flex.noTarget"))
	case view.Over:
		tone = "bg-down"
		status = Span(css.Class(tw.TextDown), Attr("role", "status"),
			uistate.T("flex.over", fmtMoney(view.Remaining.Abs())))
	default:
		if view.Remaining.Amount < view.Target.Amount/5 {
			tone = "bg-warn"
		}
		status = Span(Attr("role", "status"), uistate.T("flex.left", fmtMoney(view.Remaining)))
	}

	maxV := float64(target)
	if maxV <= 0 {
		maxV = 1
	}
	// With no target set, the meter shows an empty rail (no fill) — spending
	// against a nonexistent target must not paint a bar.
	meterValue := float64(view.Spent.Amount)
	if unset {
		meterValue = 0
	}

	var editor ui.Node
	if editing {
		editor = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap15),
			Input(css.Class("input"), Attr("data-testid", "flex-target-input"),
				Attr("inputmode", "decimal"), Attr("aria-label", uistate.T("flex.targetLabel")),
				Value(draft), OnInput(onDraft)),
			Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "flex-target-save"), OnClick(onSave),
				uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.saveTarget"))))
	} else {
		editor = Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "flex-target-edit"), OnClick(onEdit),
			uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.editTarget")))
	}

	return Div(css.Class("budgets-flex-header"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap15),
			H3(css.Class("t-h3"), uistate.T("flex.title")),
			Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "flex-classify"), Title(uistate.T("flex.classifyTitle")), OnClick(openSheet),
				uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.classify")))),
		Div(css.Class("budgets-flex-figure", tw.Flex, tw.ItemsCenter, tw.Gap15),
			Span(css.Class("t-h1"), Attr("data-testid", "flex-spent"), fmtMoney(view.Spent)),
			// Until a flex number is set, show only what's been spent — "of $0.00"
			// reads as a target, which there isn't one yet.
			If(!unset, Span(css.Class(tw.TextDim), uistate.T("flex.spentOf", fmtMoney(view.Spent), fmtMoney(view.Target)))),
			If(unset, Span(css.Class(tw.TextDim), uistate.T("flex.spentSoFar")))),
		// The meter fills against the target; with no target it stays an empty rail.
		uiw.MeterBar(uiw.MeterBarProps{Value: meterValue, Max: maxV, Tone: tone,
			Label: uistate.T("flex.title"), Class: "mt-1.5"}),
		Div(css.Class(tw.Mt15, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap15), status, editor),
	)
}

// flexFixedSection renders the fixed-commitment checklist: each fixed category as
// an expected-vs-actual checkoff (paid / not yet paid).
func flexFixedSection(view budgeting.FlexView) ui.Node {
	body := Fragment(MapKeyed(view.Fixed,
		func(r budgeting.FixedRow) any { return r.CategoryID },
		func(r budgeting.FixedRow) ui.Node { return ui.CreateElement(flexFixedRow, flexFixedRowProps{Row: r}) }))
	if len(view.Fixed) == 0 {
		body = P(css.Class("t-caption", tw.TextDim), uistate.T("flex.fixedEmpty"))
	}
	return Div(css.Class("budgets-flex-fixed", tw.Mt2),
		H4(css.Class("t-h4"), uistate.T("flex.fixedHeading")),
		Div(css.Class("budgets-flex-list"), body))
}

// flexNonMonthlySection renders the non-monthly set-asides (XC3 smoothed accrual).
func flexNonMonthlySection(view budgeting.FlexView) ui.Node {
	body := Fragment(MapKeyed(view.NonMonthly,
		func(r budgeting.NonMonthlyRow) any { return r.CategoryID },
		func(r budgeting.NonMonthlyRow) ui.Node {
			return ui.CreateElement(flexNonMonthlyRow, flexNonMonthlyRowProps{Row: r})
		}))
	if len(view.NonMonthly) == 0 {
		body = P(css.Class("t-caption", tw.TextDim), uistate.T("flex.nonMonthEmpty"))
	}
	return Div(css.Class("budgets-flex-nonmonth", tw.Mt2),
		H4(css.Class("t-h4"), uistate.T("flex.nonMonthHeading")),
		Div(css.Class("budgets-flex-list"), body))
}

type flexFixedRowProps struct{ Row budgeting.FixedRow }

// flexFixedRow is one fixed-commitment checkoff row (own component so its markup
// carries no per-row hooks).
func flexFixedRow(props flexFixedRowProps) ui.Node {
	r := props.Row
	tick := uiw.Icon(icon.Clock, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim))
	badge := Span(css.Class("t-caption", tw.TextDim), uistate.T("flex.unpaid"))
	if r.Paid {
		tick = uiw.Icon(icon.CheckCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextUp))
		badge = Span(css.Class("t-caption", tw.TextUp), uistate.T("flex.paid"))
	}
	return Div(css.Class("budgets-flex-row", tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap15),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap15), tick, Span(r.CategoryName)),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class(tw.TextDim), uistate.T("flex.actualOf", fmtMoney(r.Actual), fmtMoney(r.Expected))),
			badge))
}

type flexNonMonthlyRowProps struct{ Row budgeting.NonMonthlyRow }

// flexNonMonthlyRow is one non-monthly set-aside row.
func flexNonMonthlyRow(props flexNonMonthlyRowProps) ui.Node {
	r := props.Row
	return Div(css.Class("budgets-flex-row", tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap15),
		Span(r.CategoryName),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(uistate.T("flex.setAside", fmtMoney(r.Accrual))),
			Span(css.Class(tw.TextDim), uistate.T("flex.spentThisPeriod", fmtMoney(r.Spent)))))
}

// flexAssignSheet is the one-time category-assignment modal (BG2): every expense
// category with a 3-way flex/fixed/non-monthly toggle. Edits stage in local draft
// state and persist on Save. Rendered as a shell-root sibling of the bento.
func flexAssignSheet() ui.Node {
	openAtom := uistate.UseFlexSheetOpen()
	if !openAtom.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("flex.sheetTitle"),
		Width:    uiw.FlipMediumW,
		Height:   "min(90vh, 640px)",
		NoFooter: true,
		OnClose:  func() { openAtom.Set(false) },
		Back:     ui.CreateElement(flexAssignForm, flexAssignFormProps{OnDone: func() { openAtom.Set(false) }}),
	})
}

type flexAssignFormProps struct{ OnDone func() }

// flexAssignForm is the staged assignment sheet body. Categories default to their
// heuristic seed (budgeting.DefaultCategoryClass) when never classified; Save
// writes the chosen class onto each category.
func flexAssignForm(props flexAssignFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	cats := make([]domain.Category, 0)
	for _, c := range app.Categories() {
		if c.Kind == domain.KindExpense {
			cats = append(cats, c)
		}
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
	recs := app.Recurring()

	// Seed the draft: stored class if set, else the heuristic default.
	seed := map[string]domain.CategoryClass{}
	for _, c := range cats {
		if c.CategoryClass.Valid() {
			seed[c.ID] = c.CategoryClass
		} else {
			seed[c.ID] = budgeting.DefaultCategoryClass(c, recs)
		}
	}
	chosen := ui.UseState(seed)

	setClass := func(id string, cl domain.CategoryClass) {
		m := chosen.Get()
		next := make(map[string]domain.CategoryClass, len(m))
		for k, v := range m {
			next[k] = v
		}
		next[id] = cl
		chosen.Set(next)
	}

	onSave := ui.UseEvent(func() {
		for _, c := range cats {
			cl := chosen.Get()[c.ID]
			if !cl.Valid() {
				cl = domain.ClassFlex
			}
			if c.CategoryClass == cl {
				continue
			}
			c.CategoryClass = cl
			_ = app.PutCategory(c)
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	})
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	var rows ui.Node
	if len(cats) == 0 {
		rows = P(css.Class("t-caption", tw.TextDim), uistate.T("flex.sheetEmpty"))
	} else {
		list := make([]any, 0, len(cats))
		for _, c := range cats {
			cid := c.ID
			list = append(list, ui.CreateElement(flexAssignRow, flexAssignRowProps{
				ID: cid, Name: c.Name, Class: chosen.Get()[cid],
				OnChange: func(cl domain.CategoryClass) { setClass(cid, cl) },
			}))
		}
		rows = Fragment(list...)
	}

	return Div(css.Class("modal-scroll flex-assign"),
		P(css.Class("t-caption", tw.TextDim, tw.Mb2), uistate.T("flex.sheetIntro")),
		Div(css.Class("flex-assign-list"), rows),
		Div(css.Class("modal-foot", tw.Flex, tw.ItemsCenter, tw.Gap15, tw.Mt2),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "flex-sheet-save"), OnClick(onSave), uistate.T("flex.sheetSave")),
			Button(css.Class("btn btn-ghost"), Type("button"), OnClick(onCancel), uistate.T("flex.sheetCancel"))))
}

type flexAssignRowProps struct {
	ID       string
	Name     string
	Class    domain.CategoryClass
	OnChange func(domain.CategoryClass)
}

// flexAssignRow is one category's 3-way classification control (own component so
// the select's change hook is at a stable position, never inside a loop).
func flexAssignRow(props flexAssignRowProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { props.OnChange(domain.CategoryClass(e.GetValue())) })
	cur := props.Class
	if !cur.Valid() {
		cur = domain.ClassFlex
	}
	return Div(css.Class("flex-assign-row", tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap15),
		Span(props.Name),
		Select(css.Class("fctrl-select"), Attr("data-testid", "flex-assign-"+props.ID),
			Attr("aria-label", props.Name), OnChange(onChange),
			Option(Value(string(domain.ClassFlex)), SelectedIf(cur == domain.ClassFlex), uistate.T("flex.classFlexShort")),
			Option(Value(string(domain.ClassFixed)), SelectedIf(cur == domain.ClassFixed), uistate.T("flex.classFixed")),
			Option(Value(string(domain.ClassNonMonthly)), SelectedIf(cur == domain.ClassNonMonthly), uistate.T("flex.classNonMonth"))))
}
