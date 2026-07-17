// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// reportsBarSpec builds a horizontal Bar chart spec from label+amount pairs.
// amounts are in minor currency units; pass decimals (e.g. 2) to convert to major units.
// The default series color is overridden per-point by every reports call site
// (Tableau10 rank hues for the categorical charts, the live theme accent for the
// ranked payee/expense charts) so the charts track the active theme.
func reportsBarSpec(pairs []struct {
	Label  string
	Amount int64
}, decimals int) chartspec.Spec {
	divisor := math.Pow(10, float64(decimals))
	var points []chartspec.Point
	for i, p := range pairs {
		points = append(points, chartspec.Point{
			X:     float64(i),
			Y:     float64(p.Amount) / divisor,
			Label: p.Label,
		})
	}
	return chartspec.Spec{
		Kind: chartspec.Bar,
		Series: []chartspec.Series{
			{Name: "Amount", Color: "#4f8ef7", Points: points},
		},
		// "money" Y ticks → currency-aware compact axis ("$1.5k") matching the
		// rest of the app instead of bare numbers (the symbol is passed live via
		// ChartProps.CurrencySymbol so non-USD bases render the right glyph).
		Y:      chartspec.Axis{Format: "money"},
		Legend: false,
	}
}

// rptToneCls maps the "pos"/"neg" stat accents to the shared money color classes
// used inside the hero figure chips.
func rptToneCls(tone string) string {
	switch tone {
	case "pos":
		return " " + tw.ColorClass("text-up")
	case "neg":
		return " " + tw.ColorClass("text-down")
	}
	return ""
}

// rptTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement ("1 / span 4" full-width, "span 2" for a half-width pair that
// auto-flows beside its partner).
func rptTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// rptSection wraps a tile body with a serif section title + optional header
// action, reusing the debt-section chrome so /reports matches the other
// redesigned surfaces (/debt, /investments, /planning, /recurring).
func rptSection(sid, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if sid != "" {
		args = append(args, Attr("id", sid))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// rptChip renders one headline figure chip (the shared debt-stat chrome), with
// optional extra nodes (e.g. a small delta sub-line) below the value.
func rptChip(label, value, valueCls string, extra ...ui.Node) ui.Node {
	args := []any{css.Class("debt-stat"),
		Div(css.Class("debt-stat-label", tw.TextDim), label),
		Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+valueCls), value),
	}
	for _, e := range extra {
		args = append(args, e)
	}
	return Div(args...)
}

// customFieldSpendSection renders the "Spending by <field>" section body: a
// field selector, a ranked list of value→amount rows, and a CSV download
// button. It is extracted to keep the main Reports function readable and to
// isolate the per-field OnChange hook (called at a single stable render
// position, not in a loop).
func customFieldSpendSection(
	txns []domain.Transaction,
	defs []customfields.Def,
	selectedKey string,
	onKeyChange any,
	start, end time.Time,
	rates currency.Rates,
	base string,
	fmtMinor func(int64) string,
	win period.Window,
) ui.Node {
	// Resolve the active definition; fall back to the first if selectedKey is stale.
	activeDef := defs[0]
	for _, d := range defs {
		if d.Key == selectedKey {
			activeDef = d
			break
		}
	}

	cfRows, _ := reports.ByCustomField(txns, activeDef.Key, start, end, rates)

	// When nothing in the period actually carries a value for this field, the
	// grouper degenerates to a single "(no value) — 100%" bar that reads like a
	// real insight. Show an honest empty state instead (the field selector stays
	// so the user can try another field).
	allUnvalued := len(cfRows) > 0
	for _, r := range cfRows {
		if r.Value != "" {
			allUnvalued = false
			break
		}
	}

	// Field selector options — built outside of a loop hook (no On* here).
	var fieldOpts []ui.Node
	for _, d := range defs {
		fieldOpts = append(fieldOpts, Option(Value(d.Key), SelectedIf(d.Key == activeDef.Key), d.Label))
	}

	// When "(no value)" DOMINATES (a field only a handful of transactions carry), a
	// single giant "(no value)" bar drowns the real classifications and reads like a
	// false insight (e.g. "(no value) $44,738" beside "Personal $180"). Detect that,
	// drop the no-value row from the chart, and surface an honest note — the valued
	// rows then scale to each other instead of to the unclassified mass.
	var totalAmt, noValAmt int64
	for _, r := range cfRows {
		totalAmt += r.Amount
		if r.Value == "" {
			noValAmt += r.Amount
		}
	}
	dominantUnvalued := totalAmt > 0 && !allUnvalued && noValAmt*100/totalAmt >= 70

	displayRows := cfRows
	if dominantUnvalued {
		valued := make([]reports.CustomFieldSpend, 0, len(cfRows))
		for _, r := range cfRows {
			if r.Value != "" {
				valued = append(valued, r)
			}
		}
		displayRows = valued
	}

	// Value rows are plain display (no On* in the loop).
	noValueLabel := uistate.T("reports.customFieldNoValue")
	var rowNodes []ui.Node
	var maxAmt int64
	for _, r := range displayRows {
		if r.Amount > maxAmt {
			maxAmt = r.Amount
		}
	}
	for _, r := range displayRows {
		label := r.Value
		if label == "" {
			label = noValueLabel
		}
		pct := 0
		if maxAmt > 0 {
			pct = int(r.Amount * 100 / maxAmt)
		}
		if pct > 100 {
			pct = 100
		}
		bar := Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		rowNodes = append(rowNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), label), bar),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	// Honest note when most spending is unclassified for this field.
	var domNote ui.Node = Fragment()
	if dominantUnvalued {
		domNote = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cf-dominant-unvalued"),
			uistate.T("reports.customFieldMostUnvalued", int(noValAmt*100/totalAmt), activeDef.Label))
	}

	var body ui.Node
	switch {
	case allUnvalued:
		body = P(css.Class("empty"), Attr("data-testid", "cf-unvalued"), uistate.T("reports.customFieldUnvalued", activeDef.Label))
	case len(rowNodes) == 0:
		body = P(css.Class("empty"), uistate.T("reports.empty"))
	default:
		body = Fragment(domNote, Div(css.Class("rows"), rowNodes))
	}

	sectionLabel := uistate.T("reports.byCustomField", activeDef.Label)
	selectorLabel := uistate.T("reports.customFieldSelectLabel")

	return Div(Attr("data-testid", "customfield-spend-section"),
		rptSection("sec-customfield", sectionLabel, nil, Fragment(
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Py1),
				Label(Attr("for", "cf-field-select"), selectorLabel),
				Select(css.Class("field"), Attr("id", "cf-field-select"), Attr("aria-label", selectorLabel), Attr("data-testid", "cf-field-select"), onKeyChange, fieldOpts),
			),
			body,
			If(!allUnvalued && len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn", tw.Gap2), Type("button"),
					Attr("data-testid", "cf-download-csv"),
					Title(uistate.T("reports.customFieldDownloadTitle")),
					Attr("aria-label", uistate.T("reports.customFieldDownloadTitle")),
					OnClick(func() {
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						filename := reports.ExportFilename("spending-by-"+activeDef.Key, win.Res, win.From)
						downloadBytes(filename, "text/csv", reports.CustomFieldCSV(cfRows, activeDef.Label, csvAmount))
					}),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("reports.downloadCsv")),
				),
			)),
		)))
}

// rollupLabelKey is the i18n key for the by-category roll-up toggle's label,
// reflecting whether sub-categories are currently rolled up (L28).
func rollupLabelKey(on bool) string {
	if on {
		return "reports.rollupOn"
	}
	return "reports.rollupOff"
}

// investmentPerformanceSection renders the "Investment performance" section: for each
// investment / retirement / crypto account, what was put in (cost basis) vs its
// current value, the gain, and the return % — all from the account's own history, no
// live prices. Returns nil when there are no investment accounts. Includes a CSV export.
func investmentPerformanceSection(
	accounts []domain.Account,
	txns []domain.Transaction,
	rates currency.Rates,
	base string,
	fmtMinor func(int64) string,
	win period.Window,
) ui.Node {
	perf, err := reports.InvestmentPerformance(accounts, txns, rates)
	if err != nil || len(perf) == 0 {
		return nil
	}
	var totalInvested, totalCurrent, totalGain int64
	rowNodes := make([]ui.Node, 0, len(perf))
	for _, p := range perf {
		totalInvested += p.Invested
		totalCurrent += p.Current
		totalGain += p.Gain
		tone := "text-up"
		if p.Gain < 0 {
			tone = "text-down"
		}
		// A % return is only meaningful against a positive cost basis. When net
		// contributions are zero or negative (e.g. more was withdrawn than put in),
		// show just the gain — a "%" there would be nonsense.
		gainCell := fmtMinor(p.Gain)
		if p.Invested > 0 {
			gainCell += " · " + fmt.Sprintf("%+.1f%%", float64(p.ReturnBips)/100)
		}
		// QA CF-25: negative net contributions read "Put in ($1,900.00)" —
		// nonsense. Say "Took out" with the magnitude when more came out than in.
		basisLine := uistate.T("reports.invPerfBasis", fmtMinor(p.Invested), fmtMinor(p.Current))
		if p.Invested < 0 {
			basisLine = uistate.T("reports.invPerfBasisOut", fmtMinor(-p.Invested), fmtMinor(p.Current))
		}
		rowNodes = append(rowNodes, Div(css.Class("row"), Attr("data-testid", "invperf-row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), p.Name),
				Span(css.Class("row-meta", tw.TextDim), basisLine),
			),
			Span(ClassStr("budget-amount "+tw.ColorClass(tone)), gainCell),
		))
	}
	totalTone := "text-up"
	if totalGain < 0 {
		totalTone = "text-down"
	}
	totalRet := 0.0
	if totalInvested > 0 {
		totalRet = float64(totalGain) / float64(totalInvested) * 100
	}
	return Div(Attr("data-testid", "investperf-section"),
		rptSection("sec-investperf", uistate.T("reports.invPerfTitle"), nil, Fragment(
			P(css.Class("muted"), uistate.T("reports.invPerfHint")),
			Div(css.Class("rows"), rowNodes),
			P(ClassStr("muted "+tw.ColorClass(totalTone)), Attr("data-testid", "invperf-total"),
				uistate.T("reports.invPerfTotal", fmtMinor(totalCurrent), fmtMinor(totalGain), fmt.Sprintf("%+.1f%%", totalRet))),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn", tw.Gap2), Type("button"),
					Attr("data-testid", "invperf-download-csv"),
					Title(uistate.T("reports.invPerfDownloadTitle")),
					Attr("aria-label", uistate.T("reports.invPerfDownloadTitle")),
					OnClick(func() {
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						downloadBytes(reports.ExportFilename("investment-performance", win.Res, win.From), "text/csv", reports.InvestmentPerformanceCSV(perf, csvAmount))
					}),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("reports.downloadCsv")),
				),
			),
		)))
}

// deductibleRowProps drives one drillable deductible-category line.
type deductibleRowProps struct {
	CategoryID string
	Name       string
	Amount     string
	From, To   string // inclusive ledger-filter dates for the report window
	Bar        ui.Node
}

// deductibleRow renders one deductible line as a drill into its supporting
// transactions: the ledger filtered to the category's subtree within the
// report window. Own component so the hooks sit at a stable call-site.
func deductibleRow(props deductibleRowProps) ui.Node {
	nav := router.UseNavigate()
	filterAtom := uistate.UseTxFilter()
	drill := ui.UseEvent(Prevent(func() {
		f := uistate.TxFilter{From: props.From, To: props.To}
		set := drillCategorySet([]string{props.CategoryID})
		if strings.Contains(set, ",") {
			f.Categories = set
		} else {
			f.Category = props.CategoryID
		}
		f = f.Normalize()
		filterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}))
	return Button(css.Class("row", "dash-drill", tw.WFull, tw.TextLeft),
		Type("button"),
		Attr("data-testid", "deductible-row-"+props.CategoryID),
		Attr("title", uistate.T("reports.deductibleDrillTitle", props.Name)),
		OnClick(drill),
		Div(css.Class("row-main"), Span(css.Class("row-desc"), props.Name), props.Bar),
		Span(css.Class("budget-amount"), props.Amount),
	)
}

// deductibleSection renders the "Deductible totals" section body (L16/L58): a
// ranked list of deductible-flagged categories with their expense totals for
// the period, a headline total, and a CSV export. Returns nil when no
// categories are marked deductible, so the tile stays invisible until the user
// sets up at least one deductible category.
func deductibleSection(
	txns []domain.Transaction,
	cats []domain.Category,
	start, end time.Time,
	rates currency.Rates,
	base string,
	fmtMinor func(int64) string,
	win period.Window,
) ui.Node {
	// Only show the section when at least one deductible category exists.
	hasDeductible := false
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		if c.Deductible {
			hasDeductible = true
		}
	}
	if !hasDeductible {
		return nil
	}

	summary, _ := reports.DeductibleTotals(txns, cats, start, end, rates)
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}

	var rowNodes []ui.Node
	var maxAmt int64
	for _, r := range summary.Rows {
		if r.Amount > maxAmt {
			maxAmt = r.Amount
		}
	}
	for _, r := range summary.Rows {
		pct := 0
		if maxAmt > 0 {
			pct = int(r.Amount * 100 / maxAmt)
		}
		if pct > 100 {
			pct = 100
		}
		bar := Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		// Each deductible line drills to its SUPPORTING TRANSACTIONS — the
		// ledger filtered to the category subtree within the report window
		// (parity scan: a tax workflow needs the receipts behind the totals).
		rowNodes = append(rowNodes, ui.CreateElement(deductibleRow, deductibleRowProps{
			CategoryID: r.CategoryID,
			Name:       nameOf(r.CategoryID),
			Amount:     fmtMinor(r.Amount),
			From:       start.Format(dateutil.Layout),
			To:         end.AddDate(0, 0, -1).Format(dateutil.Layout),
			Bar:        bar,
		}))
	}

	var body ui.Node
	if len(rowNodes) == 0 {
		body = P(css.Class("empty"), uistate.T("reports.empty"))
	} else {
		body = Div(css.Class("rows"), rowNodes)
	}

	return Div(Attr("data-testid", "deductible-section"),
		rptSection("sec-deductible", uistate.T("reports.deductibleTitle"), nil, Fragment(
			P(css.Class("muted"), uistate.T("reports.deductibleHint")),
			If(summary.Total > 0, P(css.Class("muted"), uistate.T("reports.deductibleTotal", fmtMinor(summary.Total)))),
			body,
			If(len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn", tw.Gap2), Type("button"),
					Attr("data-testid", "deductible-download-csv"),
					Title(uistate.T("reports.deductibleDownloadTitle")),
					Attr("aria-label", uistate.T("reports.deductibleDownloadTitle")),
					OnClick(func() {
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						downloadBytes(reports.ExportFilename("deductible-totals", win.Res, win.From), "text/csv", reports.DeductibleCSV(summary, nameOf, csvAmount))
					}),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("reports.downloadCsv")),
				),
			)),
		)))
}
