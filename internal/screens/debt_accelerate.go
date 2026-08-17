// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/debtplan"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T3c: paying fortnightly, and combining debts ─────────────────────────
//
// Two pieces of advice people are given constantly and almost never given the
// arithmetic for. The app held every number needed for both and computed
// neither.
//
// Each panel is built to be able to say NO. A comparison that can only report
// good news is not a comparison, it is an advertisement — and both of these
// ideas are genuinely bad for some households.

// debtAccelerateWidget hosts the biweekly and consolidation comparisons.
func debtAccelerateWidget(props debtPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	if app == nil {
		return Fragment()
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	fmtM := func(v int64) string { return fmtMoney(money.New(v, base)) }

	debts := accelerateDebts(app)

	body := debtSection("sec-accelerate", uistate.T("accel.title"), Fragment(),
		Fragment(
			P(css.Class("muted"), uistate.T("accel.sub")),
			ui.CreateElement(biweeklyPanel, acceleratePanelProps{Debts: debts, Base: base, FmtM: fmtM}),
			ui.CreateElement(consolidatePanel, acceleratePanelProps{Debts: debts, Base: base, FmtM: fmtM}),
		))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-accelerate", Title: "", GridColumn: "1 / span 4",
		Draggable: false, Resizable: false, Preview: true, Body: body,
	})
}

// acceleratePanelProps carries the household's debts to each comparison.
type acceleratePanelProps struct {
	Debts []debtplan.Debt
	Base  string
	FmtM  func(int64) string
}

// accelerateDebts reads the household's liabilities into the engine's shape.
func accelerateDebts(app *appstate.App) []debtplan.Debt {
	txns := app.Transactions()
	out := make([]debtplan.Debt, 0, 4)
	for _, a := range app.Accounts() {
		if a.Archived || !a.Type.IsLiability() {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		owed := bal.Amount
		if owed < 0 {
			// Liabilities carry negative balances in the ledger; the models take a
			// positive amount owed.
			owed = -owed
		}
		if owed == 0 {
			continue
		}
		out = append(out, debtplan.Debt{
			ID: a.ID, Name: a.Name, BalanceMinor: owed,
			APRPct: a.InterestRateAPR, MinPaymentMinor: a.MinPayment.Amount,
		})
	}
	return out
}

// biweeklyPanel compares monthly against fortnightly payments on one debt.
func biweeklyPanel(props acceleratePanelProps) ui.Node {
	selected := ui.UseState("")
	termS := ui.UseState("")
	onSelect := func(v string) { selected.Set(v) }
	onTerm := ui.UseEvent(func(v string) { termS.Set(v) })

	if len(props.Debts) == 0 {
		return P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-biweekly-empty"),
			uistate.T("accel.noDebts"))
	}
	pick := selected.Get()
	if pick == "" {
		pick = props.Debts[0].ID
	}
	var chosen debtplan.Debt
	for _, d := range props.Debts {
		if d.ID == pick {
			chosen = d
		}
	}
	term := 60
	if n, err := strconv.Atoi(strings.TrimSpace(termS.Get())); err == nil && n > 0 && n <= 600 {
		term = n
	}

	opts := make([]uiw.SelectOption, 0, len(props.Debts))
	for _, d := range props.Debts {
		opts = append(opts, uiw.SelectOption{Value: d.ID, Label: d.Name})
	}

	var reading ui.Node
	b, ok := debtplan.SimulateBiweekly(chosen.BalanceMinor, chosen.APRPct, term, time.Now())
	switch {
	case !ok:
		reading = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-biweekly-none"),
			uistate.T("accel.cannotModel"))
	case b.MonthsSaved == 0 && b.InterestSavedMinor == 0:
		reading = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-biweekly-nogain"),
			uistate.T("accel.noGain"))
	default:
		reading = Fragment(
			P(css.Class("t-figure", tw.FontDisplay), Attr("data-testid", "accel-biweekly-saving"),
				uistate.T("accel.biweeklySaving", props.FmtM(b.InterestSavedMinor), b.MonthsSaved)),
			// The mechanism, stated as prominently as the saving. Somebody who cannot
			// afford a thirteenth payment cannot afford this plan, and should find
			// that out here rather than three months in.
			P(css.Class("t-body"), Attr("data-testid", "accel-biweekly-cost"),
				uistate.T("accel.biweeklyCost", props.FmtM(b.HalfPaymentMinor),
					props.FmtM(b.ExtraPerYearMinor))),
			P(css.Class("t-caption", tw.TextDim), uistate.T("accel.biweeklyWhy")),
		)
	}

	return Div(css.Class("accel-block"),
		Div(css.Class("accel-head"), Attr("data-testid", "accel-biweekly"),
			Span(css.Class("t-body", tw.FontMedium), uistate.T("accel.biweeklyTitle"))),
		Div(css.Class("accel-inputs"),
			labeledField(uistate.T("accel.whichDebt"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: opts, Selected: pick, OnChange: onSelect,
					AriaLabel: uistate.T("accel.whichDebt"), TestID: "accel-debt",
				})),
			labeledField(uistate.T("accel.termLabel"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "600"),
					Attr("data-testid", "accel-term"),
					uiw.FieldValue(termOrDefault(termS.Get(), term)), OnInput(onTerm))),
		),
		reading,
	)
}

// termOrDefault shows what the user typed, falling back to the default in use so
// the field is never blank while the figures below it assume a term.
func termOrDefault(typed string, inUse int) string {
	if strings.TrimSpace(typed) != "" {
		return typed
	}
	return strconv.Itoa(inUse)
}

// consolidatePanel compares keeping debts separate against one new loan.
func consolidatePanel(props acceleratePanelProps) ui.Node {
	aprS := ui.UseState("")
	termS := ui.UseState("")
	feeS := ui.UseState("")
	onAPR := ui.UseEvent(func(v string) { aprS.Set(v) })
	onTerm := ui.UseEvent(func(v string) { termS.Set(v) })
	onFee := ui.UseEvent(func(v string) { feeS.Set(v) })

	if len(props.Debts) < 2 {
		return P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-consol-empty"),
			uistate.T("accel.needTwo"))
	}
	apr := parsePctOr(aprS.Get(), 9)
	term := parseIntOr(termS.Get(), 48, 1, 600)
	fee := parsePctOr(feeS.Get(), 0)

	// The number an offer has to beat, and the one people compare against the
	// wrong thing — against their worst rate, which almost anything beats.
	var blended ui.Node = Fragment()
	if w, ok := debtplan.WeightedAPRPct(props.Debts); ok {
		blended = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-blended"),
			uistate.T("accel.blended", w))
	}

	var reading ui.Node
	c, ok := debtplan.Consolidate(props.Debts, apr, term, fee, time.Now())
	if !ok {
		reading = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-consol-none"),
			uistate.T("accel.cannotModel"))
	} else {
		key, tone := "accel.consolCosts", "text-down"
		amount := c.InterestDeltaMinor
		if c.Saves() {
			key, tone, amount = "accel.consolSaves", "text-up", -c.InterestDeltaMinor
		}
		var payLine ui.Node = Fragment()
		if c.PaymentDeltaMinor != 0 {
			// Consolidating usually SAVES interest and RAISES the monthly payment.
			// Someone deciding this month needs the second fact as much as the first.
			pk := "accel.payMore"
			d := c.PaymentDeltaMinor
			if d < 0 {
				pk, d = "accel.payLess", -d
			}
			payLine = P(css.Class("t-body"), Attr("data-testid", "accel-consol-payment"),
				uistate.T(pk, props.FmtM(d), props.FmtM(c.NewPaymentMinor)))
		}
		var unmodelled ui.Node = Fragment()
		if len(c.Unmodelled) > 0 {
			// Named, so a reader can see the comparison is incomplete rather than
			// trusting a total that quietly excluded something.
			unmodelled = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
				Attr("data-testid", "accel-consol-unmodelled"),
				uistate.T("accel.unmodelled", strings.Join(c.Unmodelled, ", ")))
		}
		var feeLine ui.Node = Fragment()
		if c.FeeMinor > 0 {
			feeLine = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-consol-fee"),
				uistate.T("accel.feeFinanced", props.FmtM(c.FeeMinor)))
		}
		// If the saving is really about a shorter term, say so. Rolling a long debt
		// into a short loan "saves" enormous interest at almost any rate, and
		// letting that read as the benefit of consolidating would tell someone a
		// 12% loan beat their 6% mortgage.
		var termNote ui.Node = Fragment()
		if c.TermDriven() {
			termNote = P(ClassStr("t-caption "+tw.ColorClass("text-warn")),
				Attr("data-testid", "accel-consol-termdriven"),
				uistate.T("accel.termDriven", c.NewMonths, c.KeepMonths))
		}
		reading = Fragment(
			P(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(tone)),
				Attr("data-testid", "accel-consol-verdict"),
				uistate.T(key, props.FmtM(amount))),
			termNote,
			payLine,
			feeLine,
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "accel-consol-basis"),
				uistate.T("accel.consolBasis", c.KeepMonths, c.NewMonths)),
			unmodelled,
		)
	}

	return Div(css.Class("accel-block"),
		Div(css.Class("accel-head"), Attr("data-testid", "accel-consol"),
			Span(css.Class("t-body", tw.FontMedium), uistate.T("accel.consolTitle"))),
		blended,
		Div(css.Class("accel-inputs"),
			labeledField(uistate.T("accel.newApr"),
				Input(css.Class("field"), Type("number"), Step("0.1"), Attr("min", "0"),
					Attr("data-testid", "accel-apr"),
					uiw.FieldValue(pctOrDefault(aprS.Get(), apr)), OnInput(onAPR))),
			labeledField(uistate.T("accel.newTerm"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "600"),
					Attr("data-testid", "accel-consol-term"),
					uiw.FieldValue(termOrDefault(termS.Get(), term)), OnInput(onTerm))),
			labeledField(uistate.T("accel.feeLabel"),
				Input(css.Class("field"), Type("number"), Step("0.1"), Attr("min", "0"),
					Attr("data-testid", "accel-fee"),
					uiw.FieldValue(pctOrDefault(feeS.Get(), fee)), OnInput(onFee))),
		),
		reading,
	)
}

// parsePctOr reads a percent, falling back when the field is mid-typing.
func parsePctOr(v string, fallback float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 || f > 100 {
		return fallback
	}
	return f
}

// parseIntOr reads a bounded whole number, falling back when unusable.
func parseIntOr(v string, fallback, lo, hi int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < lo || n > hi {
		return fallback
	}
	return n
}

// pctOrDefault mirrors termOrDefault for percent fields.
func pctOrDefault(typed string, inUse float64) string {
	if strings.TrimSpace(typed) != "" {
		return typed
	}
	return strconv.FormatFloat(inUse, 'f', -1, 64)
}
