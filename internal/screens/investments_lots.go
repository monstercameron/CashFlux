// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/investincome"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	"github.com/monstercameron/CashFlux/internal/taxlot"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T1d: purchase history, and what a sale actually realized ─────────────
//
// Selling used to mean deleting the position. The gain went with it, so the one
// figure a household needs in April — and the only one the app was ever in a
// position to compute — was the one it threw away.
//
// Two surfaces, in the order they have to be used: record what the shares cost
// and when, then record the sale. The second refuses to invent the first.

// holdingLotsProps is the purchase-history panel for one position.
type holdingLotsProps struct {
	H    domain.Holding
	Sym  string
	Dec  int
	Base string
	// OnSave persists the holding with its lots changed.
	OnSave func(domain.Holding)
}

// holdingLotsPanel lists a position's tax lots and adds new ones.
//
// Its own component so its input hooks sit at stable positions, and its open
// state lives in a shared atom rather than in the row: a background re-render
// remounts the list, and a form that vanishes mid-typing for invisible reasons
// is worse than no form.
func holdingLotsPanel(props holdingLotsProps) ui.Node {
	h := props.H
	openAtom := uistate.UseHoldingLotsOpen()
	dateS := ui.UseState("")
	sharesS := ui.UseState("")
	costS := ui.UseState("")
	errS := ui.UseState("")

	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onShares := ui.UseEvent(func(v string) { sharesS.Set(v) })
	onCost := ui.UseEvent(func(v string) { costS.Set(v) })
	toggle := ui.UseEvent(Prevent(func() {
		if openAtom.Get() == h.ID {
			openAtom.Set("")
			return
		}
		dateS.Set(time.Now().Format("2006-01-02"))
		errS.Set("")
		openAtom.Set(h.ID)
	}))
	add := ui.UseEvent(Prevent(func() {
		when, derr := time.Parse("2006-01-02", strings.TrimSpace(dateS.Get()))
		if derr != nil {
			// A lot with no date has no holding period, and a holding period the
			// app cannot support is the difference between two tax rates.
			errS.Set(uistate.T("lots.errDate"))
			return
		}
		shares, serr := strconv.ParseFloat(strings.TrimSpace(sharesS.Get()), 64)
		if serr != nil || shares <= 0 {
			errS.Set(uistate.T("lots.errShares"))
			return
		}
		cost, cerr := strconv.ParseFloat(strings.TrimSpace(costS.Get()), 64)
		if cerr != nil || cost < 0 {
			errS.Set(uistate.T("lots.errCost"))
			return
		}
		next := h
		next.Lots = append(append([]domain.Lot{}, h.Lots...), domain.Lot{
			ID: id.NewWithPrefix("lot"), Date: when, Shares: shares,
			CostBasisMinor: currency.MinorFromMajor(cost, props.Base),
		})
		if props.OnSave != nil {
			props.OnSave(next)
		}
		sharesS.Set("")
		costS.Set("")
		errS.Set("")
	}))
	remove := func(lotID string) {
		next := h
		kept := make([]domain.Lot, 0, len(h.Lots))
		for _, l := range h.Lots {
			if l.ID != lotID {
				kept = append(kept, l)
			}
		}
		next.Lots = kept
		if props.OnSave != nil {
			props.OnSave(next)
		}
	}

	covered := taxlot.Covers(portfolio.TaxLots(h), h.Shares)
	label := uistate.T("lots.show", len(h.Lots))
	if openAtom.Get() == h.ID {
		label = uistate.T("lots.hide")
	}
	// The coverage line is the point of the panel: a position whose lots account
	// for 300 of its 500 shares cannot report a realized gain, and the reader must
	// learn that here rather than at the moment they try to record a sale.
	var coverage ui.Node = Fragment()
	if len(h.Lots) > 0 && !covered {
		coverage = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
			Attr("data-testid", "holding-lots-gap-"+h.ID),
			uistate.T("lots.partial", fmtShares(taxlot.TotalShares(portfolio.TaxLots(h))), fmtShares(h.Shares)))
	}

	var body ui.Node = Fragment()
	if openAtom.Get() == h.ID {
		var rows ui.Node = P(css.Class("t-caption", tw.TextDim),
			Attr("data-testid", "holding-lots-empty-"+h.ID), uistate.T("lots.none"))
		if len(h.Lots) > 0 {
			rows = Div(css.Class("lot-list"), MapKeyed(h.Lots,
				func(l domain.Lot) any { return l.ID },
				func(l domain.Lot) ui.Node {
					return ui.CreateElement(lotRow, lotRowProps{
						L: l, Sym: props.Sym, Dec: props.Dec, OnDelete: remove,
					})
				}))
		}
		var errNode ui.Node = Fragment()
		if errS.Get() != "" {
			errNode = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
				Attr("data-testid", "holding-lots-err-"+h.ID), errS.Get())
		}
		body = Div(css.Class("lot-panel"), Attr("data-testid", "holding-lots-"+h.ID),
			P(css.Class("t-caption", tw.TextDim), uistate.T("lots.why")),
			coverage,
			rows,
			Form(css.Class("lot-add"), OnSubmit(add),
				labeledField(uistate.T("lots.date"),
					Input(css.Class("field"), Type("date"), Attr("data-testid", "lot-date-"+h.ID),
						uiw.FieldValue(dateS.Get()), OnInput(onDate))),
				labeledField(uistate.T("lots.shares"),
					Input(css.Class("field"), Type("number"), Step("any"), Attr("min", "0"),
						Attr("data-testid", "lot-shares-"+h.ID),
						uiw.FieldValue(sharesS.Get()), OnInput(onShares))),
				labeledField(uistate.T("lots.cost", props.Sym),
					Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
						Attr("data-testid", "lot-cost-"+h.ID),
						uiw.FieldValue(costS.Get()), OnInput(onCost))),
				Button(css.Class("btn"), Type("submit"), Attr("data-testid", "lot-add-"+h.ID),
					uistate.T("lots.add")),
			),
			errNode,
		)
	}
	return Div(css.Class("lot-wrap"),
		Button(css.Class("btn", "btn-quiet"), Type("button"),
			Attr("data-testid", "holding-lots-toggle-"+h.ID),
			Attr("aria-expanded", boolAttr(openAtom.Get() == h.ID)),
			OnClick(toggle), label),
		body)
}

// lotRowProps is one purchase-history row.
type lotRowProps struct {
	L        domain.Lot
	Sym      string
	Dec      int
	OnDelete func(string)
}

// lotRow renders one lot. Its own component so the delete handler's hook sits at
// a stable position rather than inside the list loop.
func lotRow(props lotRowProps) ui.Node {
	l := props.L
	del := ui.UseEvent(Prevent(func() {
		if props.OnDelete != nil {
			props.OnDelete(l.ID)
		}
	}))
	long := taxlot.IsLongTerm(l.Date, time.Now())
	term := uistate.T("lots.short")
	if long {
		term = uistate.T("lots.long")
	}
	return Div(css.Class("lot-row"), Attr("data-testid", "lot-"+l.ID),
		Span(css.Class("lot-date"), l.Date.Format("2 Jan 2006")),
		Span(css.Class("lot-shares"), fmtShares(l.Shares)),
		Span(css.Class("lot-cost"), fmtSignedMoney(l.CostBasisMinor, props.Sym, props.Dec)),
		// Held-long-enough is stated per lot, because it is the fact that changes
		// the rate and it changes on a specific day the reader cannot see coming.
		Span(css.Class("lot-term", tw.TextDim), Attr("data-testid", "lot-term-"+l.ID), term),
		Button(css.Class("btn", "btn-quiet"), Type("button"),
			Attr("data-testid", "lot-del-"+l.ID),
			Attr("aria-label", uistate.T("lots.removeAria", l.Date.Format("2 Jan 2006"))),
			OnClick(del), "×"),
	)
}

// --- recording a sale ------------------------------------------------------------

// holdingSellProps is the record-a-sale form for one position.
type holdingSellProps struct {
	H    domain.Holding
	Sym  string
	Dec  int
	Base string
	// OnSell persists the disposal and the position it left behind.
	OnSell func(h domain.Holding, sale taxlot.Sale, remaining []taxlot.Lot, when time.Time, method taxlot.Method)
}

// holdingSellForm records a disposal and shows what it realized BEFORE it is
// saved.
//
// The preview is the feature. A realized gain is not reversible by editing a
// number afterwards — it is a tax fact about a year — and the figure changes
// sharply with the basis method, so the reader has to see the consequence while
// the choice is still a choice.
func holdingSellForm(props holdingSellProps) ui.Node {
	h := props.H
	openAtom := uistate.UseHoldingSellOpen()
	sharesS := ui.UseState("")
	proceedsS := ui.UseState("")
	dateS := ui.UseState("")
	methodS := ui.UseState(string(taxlot.FIFO))

	onShares := ui.UseEvent(func(v string) { sharesS.Set(v) })
	onProceeds := ui.UseEvent(func(v string) { proceedsS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	cancel := ui.UseEvent(Prevent(func() { openAtom.Set("") }))

	// The form seeds itself for the common case — selling the whole position on
	// the day you are recording it — so the only figure that always has to be
	// typed is the one the app cannot know.
	if openAtom.Get() == h.ID && sharesS.Get() == "" {
		sharesS.Set(strconv.FormatFloat(h.Shares, 'f', -1, 64))
		dateS.Set(time.Now().Format("2006-01-02"))
	}

	shares, _ := strconv.ParseFloat(strings.TrimSpace(sharesS.Get()), 64)
	proceedsF, _ := strconv.ParseFloat(strings.TrimSpace(proceedsS.Get()), 64)
	proceeds := currency.MinorFromMajor(proceedsF, props.Base)
	when, dateErr := time.Parse("2006-01-02", strings.TrimSpace(dateS.Get()))
	method := taxlot.Method(methodS.Get()).Normalized()

	// The lots must cover the WHOLE position, not merely the shares being sold.
	// Recording a sale re-derives the position from what its lots say is left, so
	// a history covering 20 of 412 shares would shrink the position to the lots —
	// 392 shares quietly gone. Found by the browser probe, and it is the kind of
	// loss nothing on screen would have explained.
	lots := portfolio.TaxLots(h)
	covered := taxlot.Covers(lots, h.Shares)
	sale, remaining, ok := taxlot.Sale{}, []taxlot.Lot(nil), false
	if covered && dateErr == nil && shares > 0 {
		sale, remaining, ok = taxlot.Relieve(lots, shares, proceeds, when, method)
	}

	confirm := ui.UseEvent(Prevent(func() {
		if !ok || props.OnSell == nil {
			return
		}
		props.OnSell(h, sale, remaining, when, method)
		sharesS.Set("")
		proceedsS.Set("")
		openAtom.Set("")
	}))

	if openAtom.Get() != h.ID {
		return Fragment()
	}

	// What the app CANNOT do is stated as plainly as what it can. A position with
	// no purchase history has no basis to relieve, and the honest answer is to say
	// so and point at the panel that fixes it — not to assume the shares were free
	// and report the entire proceeds as profit.
	var preview ui.Node
	switch {
	case len(h.Lots) == 0:
		preview = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
			Attr("data-testid", "sell-nobasis-"+h.ID), uistate.T("sell.noLots"))
	case !covered:
		preview = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
			Attr("data-testid", "sell-uncovered-"+h.ID),
			uistate.T("sell.uncovered", fmtShares(taxlot.TotalShares(lots)), fmtShares(h.Shares)))
	case !ok:
		preview = P(css.Class("t-caption", tw.TextDim),
			Attr("data-testid", "sell-incomplete-"+h.ID), uistate.T("sell.incomplete"))
	default:
		tone := gainToneClass(sale.GainMinor)
		preview = Div(css.Class("sell-preview"), Attr("data-testid", "sell-preview-"+h.ID),
			P(ClassStr("sell-gain "+tw.ColorClass(tone)), Attr("data-testid", "sell-gain-"+h.ID),
				uistate.T("sell.gain", fmtSignedMoney(sale.GainMinor, props.Sym, props.Dec),
					fmtSignedMoney(sale.BasisMinor, props.Sym, props.Dec))),
			// The split is shown even when one side is zero: "all of it is
			// long-term" is information, and an absent line reads as an oversight.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "sell-split-"+h.ID),
				uistate.T("sell.split",
					fmtSignedMoney(sale.ShortTermGainMinor, props.Sym, props.Dec),
					fmtSignedMoney(sale.LongTermGainMinor, props.Sym, props.Dec))),
			P(css.Class("t-caption", tw.TextDim), uistate.T("sell.method")),
		)
	}

	return Form(css.Class("sell-form"), Attr("data-testid", "sell-form-"+h.ID), OnSubmit(confirm),
		labeledField(uistate.T("sell.shares"),
			Input(css.Class("field"), Type("number"), Step("any"), Attr("min", "0"),
				Attr("data-testid", "sell-shares-"+h.ID),
				uiw.FieldValue(sharesS.Get()), OnInput(onShares))),
		labeledField(uistate.T("sell.proceeds", props.Sym),
			Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
				Attr("data-testid", "sell-proceeds-"+h.ID),
				uiw.FieldValue(proceedsS.Get()), OnInput(onProceeds))),
		labeledField(uistate.T("sell.date"),
			Input(css.Class("field"), Type("date"), Attr("data-testid", "sell-date-"+h.ID),
				uiw.FieldValue(dateS.Get()), OnInput(onDate))),
		labeledField(uistate.T("sell.basisMethod"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: []uiw.SelectOption{
					{Value: string(taxlot.FIFO), Label: uistate.T("sell.methodFifo")},
					{Value: string(taxlot.HighestCost), Label: uistate.T("sell.methodHifo")},
					{Value: string(taxlot.LIFO), Label: uistate.T("sell.methodLifo")},
				},
				Selected:  methodS.Get(),
				OnChange:  func(v string) { methodS.Set(v) },
				AriaLabel: uistate.T("sell.basisMethod"),
				TestID:    "sell-method-" + h.ID,
			})),
		preview,
		Div(css.Class("sell-actions"),
			Button(css.Class("btn", "btn-primary"), Type("submit"),
				Attr("data-testid", "sell-confirm-"+h.ID),
				Attr("aria-disabled", boolAttr(!ok)), uistate.T("sell.record")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "sell-cancel-"+h.ID),
				OnClick(cancel), uistate.T("action.cancel"))),
	)
}

// --- recording income a position paid out (FP-T1f) --------------------------------

// holdingIncomeProps is the dividend/distribution recorder for one position.
type holdingIncomeProps struct {
	H    domain.Holding
	Sym  string
	Dec  int
	Base string
	// Income is what this position has already paid out, over the window the
	// surface chose.
	Income investincome.HoldingIncome
	// OnRecord posts the payment as income tagged to this holding.
	OnRecord func(h domain.Holding, amountMinor int64, when time.Time, note string)
}

// holdingIncomePanel shows what a position has paid out and records a new
// payment.
//
// A dividend and a price rise move the account balance identically, so the
// growth chart cannot tell them apart — but one is cash the household received
// and was taxed on, and the other is a number on a screen. This is where the
// difference gets recorded.
func holdingIncomePanel(props holdingIncomeProps) ui.Node {
	h := props.H
	openAtom := uistate.UseHoldingIncomeOpen()
	amountS := ui.UseState("")
	dateS := ui.UseState("")
	noteS := ui.UseState("")

	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })
	toggle := ui.UseEvent(Prevent(func() {
		if openAtom.Get() == h.ID {
			openAtom.Set("")
			return
		}
		dateS.Set(time.Now().Format("2006-01-02"))
		openAtom.Set(h.ID)
	}))
	record := ui.UseEvent(Prevent(func() {
		amt, aerr := strconv.ParseFloat(strings.TrimSpace(amountS.Get()), 64)
		if aerr != nil || amt <= 0 {
			uistate.PostNotice(uistate.T("income.errAmount"), true)
			return
		}
		when, derr := time.Parse("2006-01-02", strings.TrimSpace(dateS.Get()))
		if derr != nil {
			// The date is the whole point of recording it — income is taxed in the
			// year it arrived, whatever was done with it afterwards.
			uistate.PostNotice(uistate.T("income.errDate"), true)
			return
		}
		if props.OnRecord != nil {
			props.OnRecord(h, currency.MinorFromMajor(amt, props.Base), when, strings.TrimSpace(noteS.Get()))
		}
		amountS.Set("")
		noteS.Set("")
		openAtom.Set("")
	}))

	// The reading comes first, and it is stated against COST. A position bought at
	// $10 now worth $100 and paying $1 yields 10% on what was actually spent and
	// 1% on what it is worth; the first answers "was this a good buy", the second
	// reprices the past.
	var reading ui.Node = Fragment()
	if props.Income.TotalMinor > 0 {
		// One payment gets its own wording. "1 payments" is the kind of seam that
		// makes a reader trust the number less than the number deserves.
		amount := fmtSignedMoney(props.Income.TotalMinor, props.Sym, props.Dec)
		one := props.Income.Count == 1
		line := uistate.T("income.paid", amount, props.Income.Count)
		if one {
			line = uistate.T("income.paidOne", amount)
		}
		if y, ok := investincome.YieldOnCostPct(props.Income.TotalMinor, h.CostBasisMinor); ok {
			line = uistate.T("income.paidYield", amount, props.Income.Count, y)
			if one {
				line = uistate.T("income.paidYieldOne", amount, y)
			}
		}
		reading = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "holding-income-"+h.ID), line)
		// Annualizing is offered only when the payments cover enough time to
		// support it — otherwise one quarterly dividend becomes a yearly yield.
		if span := investincome.Span(props.Income); span > 0 {
			if ay, ok := investincome.AnnualizedYieldPct(props.Income.TotalMinor, h.CostBasisMinor, span); ok {
				reading = Fragment(reading,
					P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "holding-income-annual-"+h.ID),
						uistate.T("income.annual", ay)))
			}
		}
	}

	var form ui.Node = Fragment()
	if openAtom.Get() == h.ID {
		form = Form(css.Class("income-form"), Attr("data-testid", "income-form-"+h.ID), OnSubmit(record),
			labeledField(uistate.T("income.amount", props.Sym),
				Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
					Attr("data-testid", "income-amount-"+h.ID),
					uiw.FieldValue(amountS.Get()), OnInput(onAmount))),
			labeledField(uistate.T("income.date"),
				Input(css.Class("field"), Type("date"), Attr("data-testid", "income-date-"+h.ID),
					uiw.FieldValue(dateS.Get()), OnInput(onDate))),
			labeledField(uistate.T("income.note"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "income-note-"+h.ID),
					Placeholder(uistate.T("income.notePlaceholder")),
					uiw.FieldValue(noteS.Get()), OnInput(onNote))),
			Button(css.Class("btn", "btn-primary"), Type("submit"),
				Attr("data-testid", "income-save-"+h.ID), uistate.T("income.record")),
		)
	}
	return Div(css.Class("income-wrap"),
		reading,
		Button(css.Class("btn", "btn-quiet"), Type("button"),
			Attr("data-testid", "holding-income-toggle-"+h.ID),
			Attr("aria-expanded", boolAttr(openAtom.Get() == h.ID)),
			OnClick(toggle), uistate.T("income.add")),
		form)
}
