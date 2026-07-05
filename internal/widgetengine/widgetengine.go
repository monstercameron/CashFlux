// SPDX-License-Identifier: MIT

// Package widgetengine is the platform-independent HYDRATOR for the unified widget
// system: given a declarative domain.WidgetSpec and a data context, it resolves the
// spec's binding into render-ready data — no per-widget Go code. This is what makes
// a widget "fully composable from a spec": author the spec, hand it to the engine,
// and it produces the values a thin renderer paints.
//
//   - A Kind==KPI spec (ScalarBind) hydrates to a KPIView: the formula evaluated
//     over the engine variable surface (internal/engineenv), formatted, plus an
//     optional templated sub-label.
//   - A Kind in {List,Table,Chart} spec (Pipeline) hydrates to a domain.Frame: the
//     Source resolved (via internal/widgetsource) then each Transform applied in
//     order (filter/sort/limit). The renderer reads typed columns from the Frame.
//
// Pure Go, no syscall/js — fully unit-tested. The wasm layer (internal/widgetrender
// / the dashboard) builds the DataCtx, calls Hydrate*, and renders the result.
// See docs/UNIFIED_WIDGET_API.md §3 (the spec model) and §6 (the registry split).
package widgetengine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetdata"
	"github.com/monstercameron/CashFlux/internal/widgetsource"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
)

// DataCtx is everything the hydrator needs to resolve a spec against live data. The
// surface computes it once and reuses it for every spec it hydrates.
type DataCtx struct {
	// Vars is the engine variable surface (engineenv.Vars) — scoped, period-windowed
	// figures KPI formulas and sub-label templates evaluate against.
	Vars map[string]float64
	// Base is the household base currency, used to format money figures.
	Base string

	// Source inputs for Pipeline widgets.
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Budgets      []domain.Budget
	Categories   []domain.Category
	Recurring    []domain.Recurring
	Rates        currency.Rates
	Start, End   time.Time
	Now          time.Time

	// MonthVars, when set, builds the engine variable surface for an arbitrary
	// month window — what lets a "formula" series evaluate any formula or
	// molecule once per month. Supplied by the surface host (it owns scope and
	// the engineenv inputs); hosts that leave it nil can't hydrate formula series.
	MonthVars func(start, end time.Time) map[string]float64
}

// Scope is the evaluation context a KPI/template hydrates against: the numeric
// variable surface, an optional set of non-numeric string tokens (e.g. the period
// label), and the base currency for money formatting.
type Scope struct {
	Vars map[string]float64
	Strs map[string]string
	Base string
}

// KPIView is the hydrated result of a Kind==KPI spec: the numeric value (for tone /
// sign decisions), its formatted figure, and the rendered sub-label (empty if the
// spec sets none).
type KPIView struct {
	Value float64
	Text  string
	Sub   string
}

// HydrateKPI resolves a ScalarBind: it evaluates the formula over the variable
// surface, formats the value per the binding's Format (defaulting to currency), and
// renders the optional sub-label template. The figure is therefore derived from
// fundamental sources and fully described by the spec.
func HydrateKPI(s *domain.ScalarBind, sc Scope) (KPIView, error) {
	if s == nil {
		return KPIView{}, errors.New("widgetengine: nil scalar binding")
	}
	val, err := widgetspec.EvalKPI(s.Expr, sc.Vars)
	if err != nil {
		return KPIView{}, fmt.Errorf("widgetengine: kpi %q: %w", s.Expr, err)
	}
	format := s.Format
	if format == "" {
		format = widgetspec.FormatCurrency
	}
	return KPIView{
		Value: val,
		Text:  widgetdata.KPIText(val, format, sc.Base),
		Sub:   RenderTemplate(s.Sub, sc),
	}, nil
}

// RenderTemplate expands a sub-label template against a Scope. A token of the form
// "{{ expr | verb }}" is replaced by the formula expr evaluated over the variable
// surface and formatted by the verb; a bare "{{ name }}" first resolves against the
// string tokens (Scope.Strs) and otherwise evaluates as a number. Everything outside
// tokens is literal. Verbs:
//
//	number (default), currency, percent — as widgetdata.KPIText
//	signed   — currency with an explicit +/- sign ("+$1,659.33")
//	plural:N — "<count> <noun>", pluralizing the noun for count != 1 ("3 deposits")
//	arrow    — "▲" if >0, "▼" if <0, "" if 0
//
// A token whose formula fails to evaluate renders as "—" so a bad template degrades
// gracefully. Empty template → empty string.
func RenderTemplate(tmpl string, sc Scope) string {
	if tmpl == "" || !strings.Contains(tmpl, "{{") {
		return tmpl
	}
	var b strings.Builder
	rest := tmpl
	for {
		i := strings.Index(rest, "{{")
		if i < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		rest = rest[i+2:]
		j := strings.Index(rest, "}}")
		if j < 0 {
			// Unterminated token: emit literally so nothing is silently dropped.
			b.WriteString("{{")
			b.WriteString(rest)
			break
		}
		token := strings.TrimSpace(rest[:j])
		rest = rest[j+2:]
		b.WriteString(renderToken(token, sc))
	}
	return b.String()
}

// renderToken resolves one "{{...}}" token: a bare name that matches a string token
// is emitted literally; otherwise the part before "|" is a formula and the part
// after is a format verb.
func renderToken(token string, sc Scope) string {
	if !strings.Contains(token, "|") {
		if s, ok := sc.Strs[token]; ok {
			return s
		}
	}
	expr, verb := token, widgetspec.FormatNumber
	if k := strings.LastIndex(token, "|"); k >= 0 {
		expr = strings.TrimSpace(token[:k])
		verb = strings.TrimSpace(token[k+1:])
	}
	val, err := widgetspec.EvalKPI(expr, sc.Vars)
	if err != nil {
		return "—"
	}
	return formatVerb(val, verb, sc.Base)
}

// formatVerb formats a numeric value per a template verb (see RenderTemplate).
func formatVerb(val float64, verb, base string) string {
	switch {
	case verb == "signed":
		sign := "+"
		if val < 0 {
			sign = "-"
		}
		return sign + widgetdata.KPIText(abs(val), widgetspec.FormatCurrency, base)
	case verb == "arrow":
		switch {
		case val > 0:
			return "▲"
		case val < 0:
			return "▼"
		}
		return ""
	case strings.HasPrefix(verb, "plural:"):
		noun := strings.TrimPrefix(verb, "plural:")
		n := int(val + 0.5)
		if val < 0 {
			n = int(val - 0.5)
		}
		if n != 1 && n != -1 {
			noun += "s"
		}
		return fmt.Sprintf("%d %s", n, noun)
	default:
		return widgetdata.KPIText(val, verb, base)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// HydrateBlock resolves a custom-layout content Block's display string against a
// Scope (§7.5): a Text block's template is expanded; a Figure block's Bind formula
// is evaluated and formatted (wrapped as a "{{...}}" token so "net_worth|currency"
// works). Icon / Divider / Spacer / DataView blocks carry no hydrated text and
// return "". The result is plain text — the renderer splices it into a text node
// (escaped by the DOM layer), never innerHTML.
func HydrateBlock(b domain.Block, sc Scope) string {
	switch b.Kind {
	case domain.BlockText:
		return RenderTemplate(b.Text, sc)
	case domain.BlockFigure:
		if b.Bind == "" {
			return ""
		}
		return RenderTemplate("{{"+b.Bind+"}}", sc)
	}
	return ""
}

// HydrateFrame resolves a Pipeline into a Frame: the Source is materialized, then
// each Transform is applied in order. This is the data path for List/Table/Chart
// widgets — the renderer reads typed columns off the returned Frame.
func HydrateFrame(p *domain.Pipeline, dc DataCtx) (domain.Frame, error) {
	if p == nil {
		return domain.Frame{}, errors.New("widgetengine: nil pipeline")
	}
	fr, err := resolveSource(p.Source, dc)
	if err != nil {
		return domain.Frame{}, err
	}
	for i, t := range p.Transform {
		fr, err = applyTransform(fr, t)
		if err != nil {
			return domain.Frame{}, fmt.Errorf("widgetengine: transform %d (%s): %w", i, t.Kind, err)
		}
	}
	return fr, nil
}

// resolveSource materializes a Pipeline's initial Frame from the live data via the
// widgetsource resolvers (the rich, typed data layer).
func resolveSource(s domain.Source, dc DataCtx) (domain.Frame, error) {
	switch s.Kind {
	case domain.SourceCollection:
		switch s.Collection {
		case "budgets":
			return widgetsource.BudgetStatus(dc.Budgets, dc.Categories, dc.Transactions, dc.Rates, dc.Start, dc.End, false, 0), nil
		case "accounts":
			return widgetsource.AccountBalances(dc.Accounts, dc.Transactions, s.Cleared, 0), nil
		case "transactions":
			return widgetsource.RecentTransactions(dc.Transactions), nil
		case "transactions-full":
			return widgetsource.RichTransactions(dc.Transactions, dc.Accounts, dc.Categories), nil
		case "bills":
			return widgetsource.UpcomingBills(dc.Accounts, dc.Recurring, dc.Now), nil
		case "spending-breakdown":
			return widgetsource.SpendingBreakdown(dc.Categories, dc.Transactions, dc.Rates, dc.Start, dc.End), nil
		default:
			return domain.Frame{}, fmt.Errorf("unknown collection %q", s.Collection)
		}
	case domain.SourceSeries:
		switch s.Series.Metric {
		case "networth", "":
			months := s.Series.Months
			if months <= 0 {
				months = 6
			}
			cutoffs := widgetdata.ChartWindow(dc.Now, months)
			return widgetsource.NetWorthSeries(dc.Accounts, dc.Transactions, dc.Rates, cutoffs), nil
		case "cashflow":
			months := s.Series.Months
			if months <= 0 {
				months = 4
			}
			return widgetsource.CashFlowSeries(dc.Transactions, dc.Rates, dc.Now, months), nil
		case "formula":
			// A user formula/molecule evaluated once per trailing month —
			// "income - expense", "savings_rate", a custom molecule: anything
			// the variable surface can express becomes a trend line.
			if s.Series.Expr == "" {
				return domain.Frame{}, errors.New("formula series: no expression")
			}
			if dc.MonthVars == nil {
				return domain.Frame{}, errors.New("formula series: host supplies no variable surface")
			}
			return widgetsource.FormulaSeries(dc.Now, s.Series.Months, s.Series.Format, dc.Base, func(start, end time.Time) (float64, bool) {
				v, err := widgetspec.EvalKPI(s.Series.Expr, dc.MonthVars(start, end))
				return v, err == nil
			}), nil
		case "flow":
			// Monthly sums of the user's own selection — a tag, a category, or
			// a custom-field value on the transactions.
			match, err := widgetsource.TxnFilterMatcher(s.Series.Filter, dc.Categories)
			if err != nil {
				return domain.Frame{}, err
			}
			return widgetsource.FilteredFlowSeries(dc.Transactions, dc.Rates, dc.Now, s.Series.Months, match), nil
		default:
			return domain.Frame{}, fmt.Errorf("unknown series metric %q", s.Series.Metric)
		}
	}
	return domain.Frame{}, fmt.Errorf("unknown source kind %q", s.Kind)
}

// applyTransform applies one Frame→Frame step. Filter/sort/limit are supported;
// aggregate and paginate are not yet implemented and return an explicit error
// rather than silently passing the Frame through (no faked behavior).
func applyTransform(fr domain.Frame, t domain.Transform) (domain.Frame, error) {
	switch t.Kind {
	case domain.TransformLimit:
		if t.N <= 0 || t.N >= fr.Rows {
			return fr, nil
		}
		idx := make([]int, t.N)
		for i := range idx {
			idx[i] = i
		}
		return reindex(fr, idx), nil
	case domain.TransformSort:
		return sortFrame(fr, t.Arg)
	case domain.TransformFilter:
		return filterFrame(fr, t.Arg)
	case domain.TransformAggregate:
		return domain.Frame{}, errors.New("aggregate not implemented")
	case domain.TransformPaginate:
		return domain.Frame{}, errors.New("paginate not implemented")
	}
	return domain.Frame{}, fmt.Errorf("unknown transform kind %q", t.Kind)
}

// reindex rebuilds a Frame keeping only the rows in idx, in that order, across every
// column in lockstep (columns are parallel slices).
func reindex(fr domain.Frame, idx []int) domain.Frame {
	fields := make([]domain.Field, len(fr.Fields))
	for fi, f := range fr.Fields {
		vals := make([]any, 0, len(idx))
		for _, j := range idx {
			if j >= 0 && j < len(f.Values) {
				vals = append(vals, f.Values[j])
			}
		}
		fields[fi] = domain.Field{Name: f.Name, Type: f.Type, Values: vals}
	}
	return domain.NewFrame(fields...)
}

// sortFrame orders rows by a column. Arg is a column name; a leading "-" sorts
// descending. Numeric columns (number/money/percent) sort by value, others lexically.
func sortFrame(fr domain.Frame, arg string) (domain.Frame, error) {
	desc := strings.HasPrefix(arg, "-")
	name := strings.TrimPrefix(arg, "-")
	col, ok := fr.Column(name)
	if !ok {
		return domain.Frame{}, fmt.Errorf("sort: unknown column %q", name)
	}
	idx := make([]int, fr.Rows)
	for i := range idx {
		idx[i] = i
	}
	numeric := col.Type == domain.FieldNumber || col.Type == domain.FieldMoney || col.Type == domain.FieldPercent
	sort.SliceStable(idx, func(a, b int) bool {
		if numeric {
			if desc {
				return col.Num(idx[a]) > col.Num(idx[b])
			}
			return col.Num(idx[a]) < col.Num(idx[b])
		}
		if desc {
			return col.Str(idx[a]) > col.Str(idx[b])
		}
		return col.Str(idx[a]) < col.Str(idx[b])
	})
	return reindex(fr, idx), nil
}

// filterFrame keeps rows matching Arg. Supported forms:
//
//	"atrisk"       — keep rows whose state/tone column is "near" or "over"
//	"<col>=<val>"  — keep rows whose <col> equals <val> (string compare)
//	"<col>!=<val>" — keep rows whose <col> does not equal <val>
func filterFrame(fr domain.Frame, arg string) (domain.Frame, error) {
	keep := func(i int) bool { return true }
	switch {
	case arg == "atrisk":
		col, ok := fr.Column("state")
		if !ok {
			col, ok = fr.Column("tone")
		}
		if !ok {
			return domain.Frame{}, errors.New("filter atrisk: no state/tone column")
		}
		keep = func(i int) bool { s := col.Str(i); return s == "near" || s == "over" }
	case strings.Contains(arg, "!="):
		name, val, _ := strings.Cut(arg, "!=")
		col, ok := fr.Column(strings.TrimSpace(name))
		if !ok {
			return domain.Frame{}, fmt.Errorf("filter: unknown column %q", name)
		}
		want := strings.TrimSpace(val)
		keep = func(i int) bool { return col.Str(i) != want }
	case strings.Contains(arg, "="):
		name, val, _ := strings.Cut(arg, "=")
		col, ok := fr.Column(strings.TrimSpace(name))
		if !ok {
			return domain.Frame{}, fmt.Errorf("filter: unknown column %q", name)
		}
		want := strings.TrimSpace(val)
		keep = func(i int) bool { return col.Str(i) == want }
	default:
		return domain.Frame{}, fmt.Errorf("filter: unsupported criterion %q", arg)
	}
	var idx []int
	for i := 0; i < fr.Rows; i++ {
		if keep(i) {
			idx = append(idx, i)
		}
	}
	return reindex(fr, idx), nil
}
