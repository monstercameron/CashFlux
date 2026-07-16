// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
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
// hobbyists: ONE flex number for all day-to-day discretionary spending — read as a
// single serif figure over a horizon meter with a pace tick — plus two quiet
// ledgers for fixed commitments (expected-vs-actual) and non-monthly set-asides
// (XC3 smoothed accruals). It reads the live store and evaluates through the pure
// budgeting.EvaluateFlex read model.
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
	// The flex-sheet atom is a hook, so it must be resolved during render and only
	// its setter invoked from the event handler — never call UseAtom inside a callback.
	flexSheet := uistate.UseFlexSheetOpen()
	openSheet := ui.UseEvent(func() { flexSheet.Set(true) })

	m := newFlexMeter(view, target, now, start, end)

	return Div(css.Class("budgets-flex"), Attr("data-testid", "budget-flex"),
		flexHero(view, m, editing.Get(), draft.Get(), onEdit, onDraft, onSaveTarget, openSheet),
		Div(css.Class("bflex-ledgers"),
			flexFixedSection(view),
			flexNonMonthlySection(view),
		),
	)
}

// flexMeter is the pure, view-derived read that drives the signature meter: how
// full the flex pool is, where the calendar's pace sits, and the one-word verdict.
// Computed once in the component so the render helpers stay hook-free.
type flexMeter struct {
	unset     bool
	fillPct   float64 // spent as a % of target, clamped to [0,100]
	pacePct   float64 // fraction of the period elapsed, as a %
	showPace  bool    // hide the tick at the very edges where it reads as noise
	toneClass string  // meter fill tone: is-empty | is-ok | is-warn | is-over
	paceClass string  // pace-pill tone: is-ok | is-warn | is-over
	paceLabel string  // localized verdict: On track / Getting tight / Spending fast / Over budget
	daysLeft  string  // localized "N days left" (empty on the period's last hours)
}

// newFlexMeter derives the meter read from the evaluated view and the period clock.
func newFlexMeter(view budgeting.FlexView, target int64, now, start, end time.Time) flexMeter {
	if target <= 0 {
		return flexMeter{unset: true, toneClass: "is-empty"}
	}
	m := flexMeter{}
	spent := float64(view.Spent.Amount)
	tgt := float64(target)
	m.fillPct = clampPct(spent / tgt * 100)

	// Pace: how far through the period we are. If spending outruns the calendar, the
	// fill crosses this tick — the whole point of the marker.
	var pace float64
	if total := end.Sub(start); total > 0 {
		pace = now.Sub(start).Seconds() / total.Seconds()
	}
	if pace < 0 {
		pace = 0
	} else if pace > 1 {
		pace = 1
	}
	m.pacePct = pace * 100
	m.showPace = pace > 0.03 && pace < 0.97
	m.daysLeft = flexDaysLeft(now, end)

	spentFrac := spent / tgt
	switch {
	case view.Over:
		m.toneClass, m.paceClass, m.paceLabel = "is-over", "is-over", uistate.T("flex.paceOver")
	case view.Remaining.Amount < target/5:
		m.toneClass, m.paceClass, m.paceLabel = "is-warn", "is-warn", uistate.T("flex.paceTight")
	case spentFrac > pace+0.1:
		// Ahead of the calendar though not yet tight — the pace tick earns its keep.
		m.toneClass, m.paceClass, m.paceLabel = "is-warn", "is-warn", uistate.T("flex.paceFast")
	default:
		m.toneClass, m.paceClass, m.paceLabel = "is-ok", "is-ok", uistate.T("flex.paceOnTrack")
	}
	return m
}

// flexHero renders the top band: the eyebrow + classify action, and — depending on
// state — the invitation (no number yet), the inline editor, or the live reading.
func flexHero(view budgeting.FlexView, m flexMeter, editing bool, draft string, onEdit, onDraft, onSave, openSheet ui.Handler) ui.Node {
	eyebrow := Div(css.Class("bflex-eyebrow"),
		Div(css.Class("bflex-kicker"),
			Span(css.Class("bflex-kicker-label"), uistate.T("flex.eyebrow")),
			Span(css.Class("bflex-kicker-dot")),
			Span(css.Class("bflex-kicker-period"), uistate.T("flex.thisMonth"))),
		Button(css.Class("btn btn-tool bflex-classify"), Type("button"),
			Attr("data-testid", "flex-classify"), Title(uistate.T("flex.classifyTitle")), OnClick(openSheet),
			uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.classify"))))

	var body ui.Node
	switch {
	case editing:
		body = flexEditor(draft, onDraft, onSave)
	case m.unset:
		body = flexInvitation(onEdit)
	default:
		body = flexReading(view, m, onEdit)
	}
	return Div(css.Class("bflex-hero"), eyebrow, body)
}

// flexInvitation is the empty state: an invitation to name the flex number, not a
// dead "$0 of $0" reading.
func flexInvitation(onEdit ui.Handler) ui.Node {
	return Div(css.Class("bflex-empty"),
		Div(css.Class("bflex-empty-copy"),
			H3(css.Class("bflex-empty-title"), uistate.T("flex.emptyTitle")),
			P(css.Class("bflex-empty-body"), uistate.T("flex.emptyBody"))),
		Button(css.Class("btn btn-primary bflex-empty-cta"), Type("button"),
			Attr("data-testid", "flex-target-edit"), OnClick(onEdit),
			uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.setNumber"))))
}

// flexEditor is the staged inline editor for the flex number.
func flexEditor(draft string, onDraft, onSave ui.Handler) ui.Node {
	return Div(css.Class("bflex-editor"),
		Span(css.Class("bflex-editor-label"), uistate.T("flex.targetLabel")),
		Div(css.Class("bflex-editor-row"),
			Input(css.Class("input bflex-editor-input"), Attr("data-testid", "flex-target-input"),
				Attr("inputmode", "decimal"), Attr("aria-label", uistate.T("flex.targetLabel")),
				Value(draft), OnInput(onDraft)),
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("data-testid", "flex-target-save"), OnClick(onSave),
				uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("flex.saveTarget")))))
}

// flexReading is the live instrument: the serif hero figure, the horizon meter with
// its pace tick, and the context line (spent-of-target, pace verdict, days left).
func flexReading(view budgeting.FlexView, m flexMeter, onEdit ui.Handler) ui.Node {
	// Lead with the actionable number — what's still spendable — not what's gone.
	// Over budget flips the figure to the overspend, read in the danger tone.
	numTxt := fmtMoney(view.Remaining)
	word := uistate.T("flex.leftWord")
	figClass := "bflex-figure"
	if view.Over {
		numTxt = fmtMoney(view.Remaining.Abs())
		word = uistate.T("flex.overWord")
		figClass = "bflex-figure is-over"
	}

	return Div(css.Class("bflex-reading"),
		Div(css.Class(figClass),
			Span(css.Class("bflex-num"), numTxt),
			Span(css.Class("bflex-word"), word)),
		flexMeterBar(m),
		Div(css.Class("bflex-context"),
			Span(css.Class("bflex-spent"), Attr("data-testid", "flex-spent"),
				uistate.T("flex.spentTarget", fmtMoney(view.Spent), fmtMoney(view.Target))),
			Div(css.Class("bflex-context-right"),
				Span(css.Class("bflex-pace "+m.paceClass), Attr("role", "status"),
					Span(css.Class("bflex-pace-dot")), Span(m.paceLabel)),
				If(m.daysLeft != "", Span(css.Class("bflex-days"), m.daysLeft)),
				Button(css.Class("btn btn-tool bflex-edit"), Type("button"),
					Attr("data-testid", "flex-target-edit"), Title(uistate.T("flex.editTarget")), OnClick(onEdit),
					uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("flex.editShort"))))))
}

// flexMeterBar is the signature horizon: a wide, tactile gauge of spent-vs-target,
// toned by state, with a vertical pace tick marking where the calendar sits — so a
// fill that crosses the tick reads instantly as "spending faster than the month".
func flexMeterBar(m flexMeter) ui.Node {
	track := Div(css.Class("bflex-meter-track"),
		Div(css.Class("bflex-meter-fill "+m.toneClass),
			Style(map[string]string{"width": pctStr(m.fillPct)})))
	pace := Fragment()
	if m.showPace {
		pace = Div(css.Class("bflex-meter-pace"),
			Attr("aria-hidden", "true"), Style(map[string]string{"left": pctStr(m.pacePct)}))
	}
	return Div(css.Class("bflex-meter"), Attr("role", "meter"),
		Attr("aria-label", uistate.T("flex.eyebrow")),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"),
		Attr("aria-valuenow", fmt.Sprintf("%.0f", m.fillPct)),
		track, pace)
}

// flexFixedSection renders the fixed-commitment ledger: each fixed category as an
// expected-vs-actual checkoff, headed by a paid/total count.
func flexFixedSection(view budgeting.FlexView) ui.Node {
	paid := 0
	for _, r := range view.Fixed {
		if r.Paid {
			paid++
		}
	}
	var count ui.Node = Fragment()
	if len(view.Fixed) > 0 {
		cls := "bflex-count"
		if paid == len(view.Fixed) {
			cls = "bflex-count is-done"
		}
		count = Span(css.Class(cls), uistate.T("flex.fixedCount", paid, len(view.Fixed)))
	}
	body := Fragment(MapKeyed(view.Fixed,
		func(r budgeting.FixedRow) any { return r.CategoryID },
		func(r budgeting.FixedRow) ui.Node { return ui.CreateElement(flexFixedRow, flexFixedRowProps{Row: r}) }))
	if len(view.Fixed) == 0 {
		body = P(css.Class("bflex-ledger-empty"), uistate.T("flex.fixedEmpty"))
	}
	return Div(css.Class("bflex-ledger"),
		Div(css.Class("bflex-ledger-head"),
			H4(css.Class("bflex-ledger-title"), uistate.T("flex.fixedHeading")), count),
		Div(css.Class("bflex-list"), body))
}

// flexNonMonthlySection renders the non-monthly set-asides (XC3 smoothed accrual).
func flexNonMonthlySection(view budgeting.FlexView) ui.Node {
	body := Fragment(MapKeyed(view.NonMonthly,
		func(r budgeting.NonMonthlyRow) any { return r.CategoryID },
		func(r budgeting.NonMonthlyRow) ui.Node {
			return ui.CreateElement(flexNonMonthlyRow, flexNonMonthlyRowProps{Row: r})
		}))
	if len(view.NonMonthly) == 0 {
		body = P(css.Class("bflex-ledger-empty"), uistate.T("flex.nonMonthEmpty"))
	}
	return Div(css.Class("bflex-ledger"),
		Div(css.Class("bflex-ledger-head"),
			H4(css.Class("bflex-ledger-title"), uistate.T("flex.nonMonthHeading"))),
		Div(css.Class("bflex-list"), body))
}

type flexFixedRowProps struct{ Row budgeting.FixedRow }

// flexFixedRow is one fixed-commitment checkoff row (own component so its markup
// carries no per-row hooks).
func flexFixedRow(props flexFixedRowProps) ui.Node {
	r := props.Row
	rowClass := "bflex-row"
	tick := uiw.Icon(icon.Clock, css.Class("bflex-tick", tw.ShrinkO, tw.W4, tw.H4))
	badge := Span(css.Class("bflex-badge"), uistate.T("flex.unpaid"))
	if r.Paid {
		rowClass = "bflex-row is-paid"
		tick = uiw.Icon(icon.CheckCircle, css.Class("bflex-tick", tw.ShrinkO, tw.W4, tw.H4))
		badge = Span(css.Class("bflex-badge is-paid"), uistate.T("flex.paid"))
	}
	return Div(css.Class(rowClass),
		Div(css.Class("bflex-row-main"), tick, Span(css.Class("bflex-row-name"), r.CategoryName)),
		Div(css.Class("bflex-row-side"),
			Span(css.Class("bflex-row-fig"), uistate.T("flex.actualOf", fmtMoney(r.Actual), fmtMoney(r.Expected))),
			badge))
}

type flexNonMonthlyRowProps struct{ Row budgeting.NonMonthlyRow }

// flexNonMonthlyRow is one non-monthly set-aside row: the recommended monthly
// accrual with what's actually been spent so far riding beneath it.
func flexNonMonthlyRow(props flexNonMonthlyRowProps) ui.Node {
	r := props.Row
	return Div(css.Class("bflex-row"),
		Div(css.Class("bflex-row-main"), Span(css.Class("bflex-row-name"), r.CategoryName)),
		Div(css.Class("bflex-row-side is-stacked"),
			Span(css.Class("bflex-row-fig"), uistate.T("flex.setAside", fmtMoney(r.Accrual))),
			Span(css.Class("bflex-row-sub"), uistate.T("flex.spentThisPeriod", fmtMoney(r.Spent)))))
}

// clampPct clamps a percentage to [0,100].
func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// pctStr renders a percentage as a CSS width/offset value.
func pctStr(v float64) string { return fmt.Sprintf("%.2f%%", v) }

// flexDaysLeft returns the localized "N days left" for the period, counting the
// current day; it returns "" once the period has effectively ended.
func flexDaysLeft(now, end time.Time) string {
	hrs := end.Sub(now).Hours()
	if hrs <= 0 {
		return ""
	}
	days := int(math.Ceil(hrs / 24))
	if days <= 1 {
		return uistate.T("flex.dayLeft")
	}
	return uistate.T("flex.daysLeft", days)
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
