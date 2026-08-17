// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/capgains"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/esttax"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/schedulec"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T1e: the business-and-tax section ────────────────────────────────────
//
// Three questions a household with untaxed income has to answer, and the app
// held the facts for all three without ever putting them together: which line
// does my deductible spending go on, what did my investment sales realize, and
// what should I be sending each quarter.
//
// It is its own component so its input hooks sit at stable positions, and
// because the annual report is already long enough that adding five more hooks
// to it would be asking for a hook-order bug.

// taxSectionProps carries the reporting window from the annual report, so this
// section can never describe a different period from the one above it.
type taxSectionProps struct {
	From, To time.Time
	Year     int
}

// taxDepthSection renders the Schedule C grouping, realized gains, and the
// quarterly-tax estimate.
func taxDepthSection(props taxSectionProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	cfg := uistate.LoadTaxPlan()
	rateS := ui.UseState("")
	priorTaxS := ui.UseState("")
	paidS := ui.UseState("")

	onRate := ui.UseEvent(func(v string) {
		rateS.Set(v)
		saveTaxFloat(v, func(c *uistate.TaxPlan, f float64) { c.EffectiveRatePct = f })
	})
	onPriorTax := ui.UseEvent(func(v string) {
		priorTaxS.Set(v)
		saveTaxMinor(v, func(c *uistate.TaxPlan, m int64) { c.PriorYearTaxMinor = m })
	})
	onPaid := ui.UseEvent(func(v string) {
		paidS.Set(v)
		saveTaxMinor(v, func(c *uistate.TaxPlan, m int64) { c.PaidToDateMinor = m })
	})

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	fmtM := func(v int64) string { return fmtMoney(money.New(v, base)) }

	sched := schedulec.Group(app.Transactions(), app.Categories(), props.From, props.To, rates)
	gains := capgains.Gather(app.RealizedSales(), props.From, props.To)

	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}

	exportCSV := ui.UseEvent(Prevent(func() {
		downloadBytes(
			"schedule-c-"+strconv.Itoa(props.Year)+".csv",
			"text/csv",
			schedulec.CSV(sched, nameOf, func(v int64) string {
				// Plain decimal, no symbol or grouping: this file is opened by a
				// spreadsheet or a preparer, and "$1,234.00" arrives as text.
				return money.FormatMinor(v, currency.Decimals(base))
			}))
	}))

	return rptaSection("rpta-tax", "12", uistate.T("tax.secTitle"), "neutral",
		uistate.T("tax.secSub"), "", Fragment(
			taxScheduleTable(sched, nameOf, fmtM, exportCSV),
			taxGainsBlock(gains, fmtM),
			taxEstimateBlock(app, cfg, props, rates, base, fmtM, rateS, priorTaxS, paidS, onRate, onPriorTax, onPaid),
		))
}

// taxScheduleTable renders deductible spending grouped by Schedule C line.
func taxScheduleTable(s schedulec.Summary, nameOf func(string) string,
	fmtM func(int64) string, onExport ui.Handler) ui.Node {

	if len(s.Rows) == 0 && s.UnassignedMinor == 0 {
		return Div(css.Class("tax-block"),
			rptaSubG("▤", "neutral", uistate.T("tax.schedTitle"), nil),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-sched-empty"),
				uistate.T("tax.schedEmpty")))
	}

	// The unassigned line comes LAST and looks different, because it is not a
	// figure to transcribe — it is work left to do.
	var unassigned ui.Node = Fragment()
	if s.UnassignedMinor > 0 {
		names := make([]string, 0, len(s.UnassignedIDs))
		for _, id := range s.UnassignedIDs {
			names = append(names, nameOf(id))
		}
		unassigned = P(ClassStr("t-caption "+tw.ColorClass("text-down")),
			Attr("data-testid", "tax-sched-unassigned"),
			uistate.T("tax.schedUnassigned", fmtM(s.UnassignedMinor), strings.Join(names, ", ")))
	}

	return Div(css.Class("tax-block"),
		rptaSubG("▤", "neutral", uistate.T("tax.schedTitle"), nil),
		P(css.Class("t-caption", tw.TextDim), uistate.T("tax.schedNote")),
		Table(css.Class("tax-table"), Attr("data-testid", "tax-sched-table"),
			Thead(Tr(
				Th(uistate.T("tax.colLine")),
				Th(uistate.T("tax.colDesc")),
				Th(uistate.T("tax.colAmount")),
				Th(uistate.T("tax.colFrom")),
			)),
			Tbody(MapKeyed(s.Rows,
				func(r schedulec.Row) any { return r.Line.Code },
				func(r schedulec.Row) ui.Node {
					names := make([]string, 0, len(r.CategoryIDs))
					for _, id := range r.CategoryIDs {
						names = append(names, nameOf(id))
					}
					return Tr(Attr("data-testid", "tax-sched-row-"+r.Line.Code),
						Td(r.Line.Code),
						Td(r.Line.Label,
							// The gloss sits with the line it corrects, not in a footnote:
							// the wording people get wrong is wrong at the moment they read
							// the label.
							If(r.Line.Note != "",
								Div(css.Class("t-caption", tw.TextDim), r.Line.Note))),
						Td(fmtM(r.AmountMinor)),
						Td(css.Class("t-caption", tw.TextDim), strings.Join(names, ", ")))
				})),
			Tfoot(Tr(Td(""), Td(uistate.T("tax.total")),
				Td(Attr("data-testid", "tax-sched-total"), fmtM(s.Total)), Td(""))),
		),
		unassigned,
		Button(css.Class("btn", "btn-quiet"), Type("button"),
			Attr("data-testid", "tax-sched-export"), OnClick(onExport), uistate.T("tax.export")),
	)
}

// taxGainsBlock renders the year's realized capital gains.
func taxGainsBlock(g capgains.Summary, fmtM func(int64) string) ui.Node {
	if len(g.Sales) == 0 {
		return Div(css.Class("tax-block"),
			rptaSubG("↕", "neutral", uistate.T("tax.gainsTitle"), nil),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-gains-empty"),
				uistate.T("tax.gainsEmpty")))
	}
	// Short and long stay apart at every level, including the headline: they are
	// taxed differently, and one netted figure loses the fact that decides the bill.
	var lossNode ui.Node = Fragment()
	if deduct, carry, ok := g.DeductibleLossMinor(); ok {
		key := "tax.lossAll"
		if carry > 0 {
			key = "tax.lossCapped"
		}
		lossNode = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-gains-loss"),
			uistate.T(key, fmtM(deduct), fmtM(carry)))
	}
	var mixedNode ui.Node = Fragment()
	if g.MixedMethods() {
		// Legitimate, and worth knowing: the year's figures are then not
		// reproducible from one rule.
		mixedNode = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-gains-mixed"),
			uistate.T("tax.gainsMixed", strings.Join(g.Methods, ", ")))
	}
	return Div(css.Class("tax-block"),
		rptaSubG("↕", "neutral", uistate.T("tax.gainsTitle"), nil),
		Div(css.Class("stat-grid"),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("tax.shortTerm")),
				Div(ClassStr("stat-value "+tw.ColorClass(gainToneClass(g.ShortTermMinor))),
					Attr("data-testid", "tax-gains-short"), fmtM(g.ShortTermMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("tax.longTerm")),
				Div(ClassStr("stat-value "+tw.ColorClass(gainToneClass(g.LongTermMinor))),
					Attr("data-testid", "tax-gains-long"), fmtM(g.LongTermMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("tax.proceeds")),
				Div(css.Class("stat-value"), fmtM(g.ProceedsMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("tax.basis")),
				Div(css.Class("stat-value"), fmtM(g.BasisMinor))),
		),
		lossNode,
		mixedNode,
		// The individual sales stay visible: a total nobody can decompose is a
		// total nobody can check.
		Table(css.Class("tax-table"), Attr("data-testid", "tax-gains-table"),
			Thead(Tr(Th(uistate.T("tax.colSold")), Th(uistate.T("tax.colWhat")),
				Th(uistate.T("tax.colProceeds")), Th(uistate.T("tax.colBasis")),
				Th(uistate.T("tax.colGain")))),
			Tbody(MapKeyed(g.Sales,
				func(r domain.RealizedSale) any { return r.ID },
				func(r domain.RealizedSale) ui.Node {
					return Tr(Attr("data-testid", "tax-gains-row-"+r.ID),
						Td(r.Date.Format("2 Jan 2006")),
						Td(r.Name),
						Td(fmtM(r.ProceedsMinor)),
						Td(fmtM(r.BasisMinor)),
						Td(ClassStr(tw.ColorClass(gainToneClass(r.GainMinor))), fmtM(r.GainMinor)))
				})),
		),
	)
}

// taxEstimateBlock renders the quarterly estimate and the assumptions it needs.
func taxEstimateBlock(app *appstate.App, cfg uistate.TaxPlan, props taxSectionProps,
	rates currency.Rates, base string, fmtM func(int64) string,
	rateS, priorTaxS, paidS ui.State[string],
	onRate, onPriorTax, onPaid ui.Handler) ui.Node {

	// Net business income for the window: income less deductible expense. Both
	// come from the same ledger the rest of the report reads, so the estimate
	// cannot describe a different business from the section above it.
	net := taxNetIncomeMinor(app, props.From, props.To, rates)

	in := esttax.Inputs{
		NetIncomeMinor: net, EffectiveRatePct: cfg.EffectiveRatePct,
		PriorYearTaxMinor: cfg.PriorYearTaxMinor, PriorYearIncomeMinor: cfg.PriorYearIncomeMinor,
		PaidToDateMinor: cfg.PaidToDateMinor, Now: time.Now(),
	}
	est, ok := esttax.Compute(in)

	fields := Div(css.Class("tax-inputs"),
		labeledField(uistate.T("tax.rateLabel"),
			Input(css.Class("field"), Type("number"), Step("0.1"), Attr("min", "0"),
				Attr("data-testid", "tax-rate"),
				uiw.FieldValue(taxFieldValue(rateS.Get(), floatStr(cfg.EffectiveRatePct))),
				OnInput(onRate))),
		labeledField(uistate.T("tax.priorTaxLabel", base),
			Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
				Attr("data-testid", "tax-prior"),
				uiw.FieldValue(taxFieldValue(priorTaxS.Get(), majorStr(cfg.PriorYearTaxMinor, base))),
				OnInput(onPriorTax))),
		labeledField(uistate.T("tax.paidLabel", base),
			Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
				Attr("data-testid", "tax-paid"),
				uiw.FieldValue(taxFieldValue(paidS.Get(), majorStr(cfg.PaidToDateMinor, base))),
				OnInput(onPaid))),
	)

	var body ui.Node
	if !ok {
		// Say WHICH fact is missing. "Cannot estimate" with no reason is a dead end
		// the reader has to guess their way out of.
		reason := uistate.T("tax.estNeedRate")
		if cfg.EffectiveRatePct > 0 && net <= 0 {
			reason = uistate.T("tax.estNeedIncome")
		}
		body = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-est-blocked"), reason)
	} else {
		// The safe harbour leads. A projection of this year's tax is a guess about
		// income that has not finished happening; the harbour is a rule about a
		// number that is already final, and paying it removes the penalty however
		// the year turns out.
		// ONE element whose text changes, not two that swap at the same position.
		// Swapping left the previous node's data-testid in place with no text — the
		// reconciler patches a node it considers the same, and two paragraphs
		// differing only in an attribute and their content are exactly that. A
		// single node also matches what the reader sees: one line that says either
		// what the harbour is or how to get one.
		harborText := uistate.T("tax.estNoHarbor")
		if est.SafeHarborKnown {
			harborText = uistate.T("tax.estHarbor", fmtM(est.SafeHarborMinor), est.SafeHarborPct)
		}
		harbor := P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-est-harbor"),
			harborText)
		dueKey := "tax.estDue"
		due := est.DueNowMinor
		if due < 0 {
			// Ahead is reported as ahead. Clamping it to zero would hide the fact
			// that the next payment can be smaller.
			dueKey = "tax.estAhead"
			due = -due
		}
		body = Fragment(
			P(css.Class("t-figure", tw.FontDisplay), Attr("data-testid", "tax-est-due"),
				uistate.T(dueKey, fmtM(due), est.Quarter, est.QuarterDue.Format("2 Jan 2006"))),
			harbor,
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "tax-est-basis"),
				uistate.T("tax.estBasis", fmtM(net), fmtM(est.ProjectedTaxMinor), fmtM(est.TargetMinor))),
			P(css.Class("t-caption", tw.TextDim), uistate.T("tax.estCaveat")),
		)
	}

	return Div(css.Class("tax-block"),
		rptaSubG("◷", "neutral", uistate.T("tax.estTitle"), nil),
		fields,
		body,
	)
}

// taxNetIncomeMinor is income less deductible expense over the window.
func taxNetIncomeMinor(app *appstate.App, from, to time.Time, rates currency.Rates) int64 {
	deductible := map[string]bool{}
	for _, c := range app.Categories() {
		if c.Deductible {
			deductible[c.ID] = true
		}
	}
	var net int64
	for _, t := range app.Transactions() {
		if !t.CountsInReports() || t.IsTransfer() {
			continue
		}
		if t.Date.Before(from) || !t.Date.Before(to) {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		switch {
		case t.IsIncome():
			net += amt.Amount
		case deductible[t.CategoryID]:
			net += amt.Amount // already negative
		}
	}
	return net
}

// taxFieldValue prefers what the user is typing over what was stored, so a field
// does not fight the keyboard on every re-render.
func taxFieldValue(typed, stored string) string {
	if typed != "" {
		return typed
	}
	return stored
}

// floatStr renders a percent without trailing zeroes, blank when unset.
func floatStr(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// majorStr renders minor units as a plain major-unit number, blank when zero.
func majorStr(minor int64, base string) string {
	if minor == 0 {
		return ""
	}
	return strconv.FormatFloat(currency.MajorFromMinor(minor, base), 'f', -1, 64)
}

// saveTaxFloat persists a percent field, ignoring input that does not parse.
func saveTaxFloat(v string, set func(*uistate.TaxPlan, float64)) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return
	}
	cur := uistate.LoadTaxPlan()
	set(&cur, f)
	uistate.SaveTaxPlan(cur)
}

// saveTaxMinor persists a money field in major units, ignoring unparseable input.
func saveTaxMinor(v string, set func(*uistate.TaxPlan, int64)) {
	app := appstate.Default
	base := "USD"
	if app != nil && app.Settings().BaseCurrency != "" {
		base = app.Settings().BaseCurrency
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return
	}
	cur := uistate.LoadTaxPlan()
	set(&cur, currency.MinorFromMajor(f, base))
	uistate.SaveTaxPlan(cur)
}
