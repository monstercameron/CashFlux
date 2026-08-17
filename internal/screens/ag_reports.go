// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsReports(app, base, rates, tier)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// This file gives the assistant the Reports surface. Before it, the agent could
// total one category and list balances; everything the report computes — the
// money-flow diagram, payee trends, movers, runway, the tax roll-up — was
// visible to the user and invisible to the model, so a question about a figure
// on screen was answered by re-deriving something adjacent to it.
//
// Two rules shape the design:
//
//   - One computation per figure. Every section calls the same pure reports
//     function the screen calls, over the same window, so the assistant cannot
//     quote a number that disagrees with the page.
//   - Every row traces. A report row is an aggregate, and an aggregate the user
//     cannot open is a claim they have to take on faith. Each section carries a
//     Trace that resolves one of its rows back to the exact transactions behind
//     it — with ids, so the answer can be followed by an edit.

// agRptCtx is everything a report section computes against, resolved once per
// tool call: the window, the data, and the formatting.
type agRptCtx struct {
	app       *appstate.App
	txns      []domain.Transaction
	cats      []domain.Category
	accounts  []domain.Account
	members   []domain.Member
	rates     currency.Rates
	base      string
	fmtM      func(int64) string
	start     time.Time
	end       time.Time // exclusive
	prevStart time.Time
	prevEnd   time.Time
	label     string
	now       time.Time
	nameOf    func(categoryID string) string
	memberOf  func(memberID string) string
	field     string // the custom field key, for the by-custom-field section
}

// bounds returns monthly bucket edges spanning the window, oldest first, for
// the series reports (trends, savings rate, monthly flow). A window shorter
// than a month still yields one bucket; very long windows are capped so a
// tool result stays readable.
func (c agRptCtx) bounds() []time.Time {
	const maxBuckets = 36
	start := dateutil.MonthStart(c.start)
	if c.start.IsZero() {
		start = dateutil.MonthStart(c.earliest())
	}
	var out []time.Time
	for t := start; t.Before(c.end) && len(out) <= maxBuckets; t = dateutil.AddMonths(t, 1) {
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return append(out, dateutil.AddMonths(out[len(out)-1], 1))
}

// earliest is the date of the oldest transaction, for an "all time" window.
func (c agRptCtx) earliest() time.Time {
	out := c.now
	for _, t := range c.txns {
		if t.Date.Before(out) && !t.Date.IsZero() {
			out = t.Date
		}
	}
	return out
}

// inWindow reports whether a transaction falls in the reporting window. A zero
// start means "all time", which every dated row satisfies.
func (c agRptCtx) inWindow(t domain.Transaction) bool {
	if c.start.IsZero() {
		return t.Date.Before(c.end)
	}
	return dateutil.InRange(t.Date, c.start, c.end)
}

// counted reports whether a transaction is one the reports count at all —
// the same gate the pure report functions apply, so a trace returns the rows
// that actually produced the figure rather than everything that resembles it.
func (c agRptCtx) counted(t domain.Transaction) bool {
	return t.CountsInReports() && c.inWindow(t)
}

// agReportSection is one section of the Reports surface as the assistant sees
// it: what it answers, how to compute it, and how to open one of its rows.
type agReportSection struct {
	ID string
	// About says what question the section answers, in the words the model
	// needs to pick it — this text is what list_report_sections returns.
	About string
	// Render computes the section over the context's window.
	Render func(c agRptCtx) string
	// Trace resolves ONE row of this section back to the transactions behind
	// it. Nil when the section has no row-shaped detail to open (a runway
	// estimate is a projection, not a group of records). The returned string
	// says what was matched, for the tool's header line.
	Trace func(c agRptCtx, row string) ([]domain.Transaction, string)
	// RowHint tells the model what to pass as `row` for this section.
	RowHint string
	// Detail marks a section that names individual merchants or charges rather
	// than only aggregates. Under the aggregates-only privacy tier these are
	// withheld, the same way the transaction-level read tools are — the promise
	// is enforced by filtering the DATA, not by asking the prompt nicely.
	Detail bool
}

// agReportSections is the catalog: every section of the Reports surface, in the
// order the report itself presents them. Adding a section here is what makes it
// visible to the assistant — there is no second list to keep in sync.
func agReportSections() []agReportSection {
	return []agReportSection{
		{
			ID:     "overview",
			About:  "Headline figures for the window: income, spending, net, savings rate, and net worth.",
			Render: agRptOverview,
		},
		{
			ID:      "money_flow",
			About:   "The money-flow (sankey) diagram: which income sources feed the period, where the money went, and what was kept or drawn down. This is the diagram the Annual Review draws.",
			Render:  agRptMoneyFlow,
			Trace:   agRptTraceMoneyFlow,
			RowHint: "a node or ribbon label from the diagram, e.g. \"Groceries\", \"Salary\", or \"Everything else\"",
		},
		{
			ID:      "spending_by_category",
			About:   "Every spending category with its total for the window and its change against the previous window.",
			Render:  agRptSpendingByCategory,
			Trace:   agRptTraceCategory,
			RowHint: "a category name",
		},
		{
			ID:      "income_by_category",
			About:   "Where the money came from: every income category with its total.",
			Render:  agRptIncomeByCategory,
			Trace:   agRptTraceIncomeCategory,
			RowHint: "an income category name",
		},
		{
			ID:      "top_movers",
			About:   "The categories that changed most against the previous window — what grew and what shrank.",
			Render:  agRptTopMovers,
			Trace:   agRptTraceCategory,
			RowHint: "a category name",
		},
		{
			ID:      "category_trends",
			About:   "Each category's month-by-month spend across the window, with the change from the first month to the last.",
			Render:  agRptCategoryTrends,
			Trace:   agRptTraceCategory,
			RowHint: "a category name",
		},
		{
			ID: "largest_expenses", Detail: true,
			About:   "The biggest individual purchases in the window.",
			Render:  agRptLargestExpenses,
			Trace:   agRptTracePayee,
			RowHint: "the description of one of the listed expenses",
		},
		{
			ID: "largest_income", Detail: true,
			About:   "The biggest individual deposits in the window.",
			Render:  agRptLargestIncome,
			Trace:   agRptTraceIncomePayee,
			RowHint: "the description of one of the listed deposits",
		},
		{
			ID: "top_payees", Detail: true,
			About:   "Who was paid the most in the window, largest first.",
			Render:  agRptTopPayees,
			Trace:   agRptTracePayee,
			RowHint: "a payee name",
		},
		{
			ID: "payee_trends", Detail: true,
			About:   "The top payees' month-by-month spend, for spotting a merchant that is creeping up.",
			Render:  agRptPayeeTrends,
			Trace:   agRptTracePayee,
			RowHint: "a payee name",
		},
		{
			ID:      "spending_by_tag",
			About:   "Totals per tag, with the prior window for comparison.",
			Render:  agRptSpendingByTag,
			Trace:   agRptTraceTag,
			RowHint: "a tag",
		},
		{
			ID:      "spending_by_member",
			About:   "Who in the household spent what.",
			Render:  agRptSpendingByMember,
			Trace:   agRptTraceMember,
			RowHint: "a household member's name",
		},
		{
			ID:      "spending_by_weekday",
			About:   "Which days of the week the money goes out on.",
			Render:  agRptSpendingByWeekday,
			Trace:   agRptTraceWeekday,
			RowHint: "a weekday name, e.g. \"Saturday\"",
		},
		{
			ID:      "spending_by_custom_field",
			About:   "Totals grouped by one of the user's own custom fields. Pass the field key or label as `field`; with no field the first one is used.",
			Render:  agRptSpendingByCustomField,
			Trace:   agRptTraceCustomField,
			RowHint: "one value of the custom field",
		},
		{
			ID:      "monthly_flow",
			About:   "Income, spending and net for each month in the window — the cash-flow table.",
			Render:  agRptMonthlyFlow,
			Trace:   agRptTraceMonth,
			RowHint: "a month, e.g. \"2026-03\" or \"March 2026\"",
		},
		{
			ID:     "savings_rate",
			About:  "The percent of income kept, per month across the window and for the window as a whole.",
			Render: agRptSavingsRate,
		},
		{
			ID:     "spending_stats",
			About:  "How many expenses, their total, and the average and median charge.",
			Render: agRptSpendingStats,
		},
		{
			ID:     "no_spend_days",
			About:  "How many days in the window had no spending at all.",
			Render: agRptNoSpendDays,
		},
		{
			ID:     "runway",
			About:  "How long the liquid cash covers the recent burn rate.",
			Render: agRptRunway,
		},
		{
			ID: "cost_of_money", Detail: true,
			About:   "What borrowing and banking cost in the window: interest charges and fees, with every matched charge.",
			Render:  agRptCostOfMoney,
			Trace:   agRptTraceCostOfMoney,
			RowHint: "\"interest\", \"fees\", or the description of one charge",
		},
		{
			ID:     "net_worth",
			About:  "Assets, liabilities and net worth right now, by account.",
			Render: agRptNetWorth,
		},
		{
			ID:      "investment_performance",
			About:   "For each investment account: what was put in, what it is worth now, the gain and the return.",
			Render:  agRptInvestmentPerformance,
			Trace:   agRptTraceAccount,
			RowHint: "an account name",
		},
		{
			ID:      "deductible_totals",
			About:   "Totals for the categories flagged deductible — the tax workflow's supporting figures.",
			Render:  agRptDeductible,
			Trace:   agRptTraceCategory,
			RowHint: "a deductible category name",
		},
		{
			ID:      "year_tax",
			About:   "Income and expense rolled up by category for the window, with the totals a return needs.",
			Render:  agRptYearTax,
			Trace:   agRptTraceTaxCategory,
			RowHint: "a category name",
		},
	}
}

// agFindSection looks a section up by id, tolerating the model naming it by
// title or with spaces instead of underscores.
func agFindSection(name string) (agReportSection, bool) {
	q := strings.ToLower(strings.TrimSpace(name))
	q = strings.ReplaceAll(q, " ", "_")
	q = strings.ReplaceAll(q, "-", "_")
	secs := agReportSections()
	for _, s := range secs {
		if s.ID == q {
			return s, true
		}
	}
	for _, s := range secs {
		if q != "" && strings.Contains(s.ID, q) {
			return s, true
		}
	}
	return agReportSection{}, false
}

// agSectionIDs is the comma-joined list of section ids, for error messages that
// point the model at the real names instead of leaving it to guess again.
func agSectionIDs() string {
	secs := agReportSections()
	ids := make([]string, 0, len(secs))
	for _, s := range secs {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ", ")
}

// agPeriodEnum is the shared JSON-schema fragment for a report tool's window.
const agPeriodEnum = `"period":{"type":"string","enum":["this_month","last_month","this_quarter","last_quarter","this_year","last_year","year_to_date","last_12_months","all"],"description":"named window; ignored when from/to are given"},"from":{"type":"string","description":"start date YYYY-MM-DD (inclusive)"},"to":{"type":"string","description":"end date YYYY-MM-DD (inclusive)"}`

// agBuildRptCtx resolves the shared context one report tool call runs against.
func agBuildRptCtx(app *appstate.App, base string, rates currency.Rates, preset, from, to, field string) agRptCtx {
	now := time.Now()
	w := reports.ResolveWindow(preset, from, to, now)
	cats := app.Categories()
	members := app.Members()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	memberName := make(map[string]string, len(members))
	for _, m := range members {
		memberName[m.ID] = m.Name
	}
	return agRptCtx{
		app:       app,
		txns:      app.Transactions(),
		cats:      cats,
		accounts:  app.Accounts(),
		members:   members,
		rates:     rates,
		base:      base,
		fmtM:      func(v int64) string { return fmtMoney(money.New(v, base)) },
		start:     w.Start,
		end:       w.End,
		prevStart: w.PrevStart,
		prevEnd:   w.PrevEnd,
		label:     w.Label,
		now:       now,
		field:     field,
		nameOf: func(id string) string {
			if n := catName[id]; n != "" {
				return n
			}
			return "Uncategorized"
		},
		memberOf: func(id string) string {
			if n := memberName[id]; n != "" {
				return n
			}
			return "Unassigned"
		},
	}
}

// agResolveCatID maps a category name (or "uncategorized") to its id.
func (c agRptCtx) resolveCatID(name string) (string, bool) {
	q := strings.ToLower(strings.TrimSpace(name))
	if q == "" {
		return "", false
	}
	if q == "uncategorized" || q == "(uncategorized)" || q == "no category" {
		return "", true
	}
	for _, cat := range c.cats {
		if strings.ToLower(cat.Name) == q {
			return cat.ID, true
		}
	}
	for _, cat := range c.cats {
		if strings.Contains(strings.ToLower(cat.Name), q) {
			return cat.ID, true
		}
	}
	return "", false
}

// ── Section renderers ────────────────────────────────────────────────────────

func agRptOverview(c agRptCtx) string {
	flow, _ := reports.IncomeVsExpense(c.txns, c.start, c.end, c.rates)
	prior, _ := reports.IncomeVsExpense(c.txns, c.prevStart, c.prevEnd, c.rates)
	net, assets, liab, _ := ledger.NetWorth(c.accounts, c.txns, c.rates)
	var b strings.Builder
	fmt.Fprintf(&b, "%s — income %s, spending %s, net %s, savings rate %d%%.\n",
		c.label, c.fmtM(flow.Income), c.fmtM(flow.Expense), c.fmtM(flow.Net()), flow.SavingsRate())
	if !c.prevStart.IsZero() && (prior.Income != 0 || prior.Expense != 0) {
		fmt.Fprintf(&b, "Previous window: income %s, spending %s, net %s.\n",
			c.fmtM(prior.Income), c.fmtM(prior.Expense), c.fmtM(prior.Net()))
	}
	fmt.Fprintf(&b, "Net worth now %s (assets %s, liabilities %s).",
		c.fmtM(net.Amount), c.fmtM(assets.Amount), c.fmtM(liab.Amount))
	return b.String()
}

// agMoneyFlow builds the diagram the Annual Review draws, over this window.
func agMoneyFlow(c agRptCtx) reports.MoneyFlowDiagram {
	spend, _ := reports.SpendingByCategory(c.txns, c.start, c.end, false, time.Time{}, time.Time{}, c.rates)
	income, _ := reports.IncomeByCategory(c.txns, c.start, c.end, c.rates)
	flow, _ := reports.IncomeVsExpense(c.txns, c.start, c.end, c.rates)
	return reports.BuildMoneyFlow(reports.MoneyFlowInputs{
		Income:   income,
		Spending: spend,
		Net:      flow.Net(),
		Name:     c.nameOf,
		Labels: reports.MoneyFlowLabels{
			Income:         uistate.T("rpta.nodeIncome"),
			OtherIncome:    uistate.T("rpta.nodeOtherIncome"),
			EverythingElse: uistate.T("rpta.nodeEverythingElse"),
			Savings:        uistate.T("rpta.nodeSavings"),
			FromSavings:    uistate.T("rpta.nodeFromSavings"),
			Disambiguate:   func(n string) string { return uistate.T("rpta.nodeCatDisamb", n) },
		},
		TopSources:  rptaFlowTopSources,
		TopSpending: rptaFlowTopCategories,
	})
}

func agRptMoneyFlow(c agRptCtx) string {
	d := agMoneyFlow(c)
	if len(d.Edges) == 0 {
		return "No money flowed in " + c.label + " — nothing to diagram."
	}
	pct := func(v int64) string {
		if d.IncomeTotal <= 0 {
			return ""
		}
		return fmt.Sprintf(" (%d%% of income)", v*100/d.IncomeTotal)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Money flow for %s — %s in, %s out, %s %s.\n",
		c.label, c.fmtM(d.IncomeTotal), c.fmtM(d.SpendingTotal),
		c.fmtM(agAbs(d.Net)), map[bool]string{true: "kept", false: "overspent"}[d.Net >= 0])
	b.WriteString("In:\n")
	for _, e := range d.Edges {
		if e.Kind != reports.FlowIncomeSource && e.Kind != reports.FlowOtherIncome && e.Kind != reports.FlowFromSavings {
			continue
		}
		fmt.Fprintf(&b, "  %s → %s: %s%s%s\n", e.From, e.To, c.fmtM(e.Value), pct(e.Value), agPooledNote(c, e))
	}
	b.WriteString("Out:\n")
	for _, e := range d.Edges {
		if e.Kind != reports.FlowSpendCategory && e.Kind != reports.FlowOtherSpending && e.Kind != reports.FlowSavings {
			continue
		}
		fmt.Fprintf(&b, "  %s → %s: %s%s%s\n", e.From, e.To, c.fmtM(e.Value), pct(e.Value), agPooledNote(c, e))
	}
	b.WriteString("Trace any of these with trace_report_row(section=\"money_flow\", row=<node label>).")
	return b.String()
}

// agPooledNote names the categories hiding inside a pooled ribbon, so the model
// never describes "Everything else" as if it were one thing.
func agPooledNote(c agRptCtx, e reports.MoneyFlowEdge) string {
	if !e.Kind.Pooled() || len(e.CategoryIDs) == 0 {
		return ""
	}
	names := make([]string, 0, len(e.CategoryIDs))
	for _, id := range e.CategoryIDs {
		names = append(names, c.nameOf(id))
	}
	if len(names) > 8 {
		return fmt.Sprintf(" [pools %d categories: %s, …]", len(names), strings.Join(names[:8], ", "))
	}
	return " [pools: " + strings.Join(names, ", ") + "]"
}

func agRptSpendingByCategory(c agRptCtx) string {
	compare := !c.prevStart.IsZero()
	rows, err := reports.SpendingByCategory(c.txns, c.start, c.end, compare, c.prevStart, c.prevEnd, c.rates)
	if err != nil || len(rows) == 0 {
		return "No spending in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Spending by category, %s (total %s):\n", c.label, c.fmtM(reports.Total(rows)))
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s: %s%s\n", c.nameOf(r.CategoryID), c.fmtM(r.Amount), agDelta(r))
	}
	return strings.TrimRight(b.String(), "\n")
}

// agDelta renders a category row's change against the prior window.
func agDelta(r reports.CategorySpend) string {
	switch {
	case r.PriorZero:
		return " (new)"
	case r.HasDelta:
		return fmt.Sprintf(" (%+d%% vs previous)", r.DeltaPct)
	}
	return ""
}

func agRptIncomeByCategory(c agRptCtx) string {
	rows, err := reports.IncomeByCategory(c.txns, c.start, c.end, c.rates)
	if err != nil || len(rows) == 0 {
		return "No income in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Income by category, %s (total %s):\n", c.label, c.fmtM(reports.Total(rows)))
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s: %s\n", c.nameOf(r.CategoryID), c.fmtM(r.Amount))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptTopMovers(c agRptCtx) string {
	if c.prevStart.IsZero() {
		return "Top movers needs a previous window to compare against; ask for a named period rather than \"all\"."
	}
	rows, err := reports.SpendingByCategory(c.txns, c.start, c.end, true, c.prevStart, c.prevEnd, c.rates)
	if err != nil {
		return "Couldn't compute the movers."
	}
	movers := reports.TopMovers(rows, 10)
	if len(movers) == 0 {
		return "Nothing moved much between the two windows."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Biggest changes, %s vs the previous window:\n", c.label)
	for _, r := range movers {
		fmt.Fprintf(&b, "  %s: %s (was %s)%s\n", c.nameOf(r.CategoryID), c.fmtM(r.Amount), c.fmtM(r.Prior), agDelta(r))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptCategoryTrends(c agRptCtx) string {
	bounds := c.bounds()
	trends, err := reports.CategoryTrends(c.txns, bounds, c.rates)
	if err != nil || len(trends) == 0 {
		return "Not enough history in " + c.label + " to trend categories."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Category trends by month, %s (%s):\n", c.label, agBucketLabels(bounds))
	for i, t := range trends {
		if i >= 12 {
			fmt.Fprintf(&b, "  …and %d more categories.\n", len(trends)-i)
			break
		}
		parts := make([]string, 0, len(t.Spend))
		for _, v := range t.Spend {
			parts = append(parts, c.fmtM(v))
		}
		delta := ""
		if t.HasDelta {
			delta = fmt.Sprintf(" — %+d%% first month to last", t.DeltaPct)
		}
		fmt.Fprintf(&b, "  %s (total %s): %s%s\n", c.nameOf(t.CategoryID), c.fmtM(t.Total), strings.Join(parts, ", "), delta)
	}
	return strings.TrimRight(b.String(), "\n")
}

// agBucketLabels names the monthly buckets a series report is split into.
func agBucketLabels(bounds []time.Time) string {
	if len(bounds) < 2 {
		return "one bucket"
	}
	labels := make([]string, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		labels = append(labels, bounds[i].Format("Jan 2006"))
	}
	return strings.Join(labels, ", ")
}

func agRptLargestExpenses(c agRptCtx) string {
	items, err := reports.LargestExpenses(c.txns, c.start, c.end, c.rates, 15)
	if err != nil || len(items) == 0 {
		return "No expenses in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Largest expenses, %s:\n", c.label)
	for _, it := range items {
		fmt.Fprintf(&b, "  %s  %s  %s  [%s]\n", it.Date.Format("2006-01-02"), it.Desc, c.fmtM(it.Amount), c.nameOf(it.CategoryID))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptLargestIncome(c agRptCtx) string {
	items, err := reports.LargestIncome(c.txns, c.start, c.end, c.rates, 15)
	if err != nil || len(items) == 0 {
		return "No income in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Largest income, %s:\n", c.label)
	for _, it := range items {
		fmt.Fprintf(&b, "  %s  %s  %s  [%s]\n", it.Date.Format("2006-01-02"), it.Desc, c.fmtM(it.Amount), c.nameOf(it.CategoryID))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptTopPayees(c agRptCtx) string {
	rows, err := reports.TopPayees(c.txns, c.start, c.end, c.rates, 15)
	if err != nil || len(rows) == 0 {
		return "No payees in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Top payees, %s:\n", c.label)
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s: %s\n", r.Name, c.fmtM(r.Amount))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptPayeeTrends(c agRptCtx) string {
	bounds := c.bounds()
	rows, err := reports.PayeeTrends(c.txns, bounds, c.rates, 10)
	if err != nil || len(rows) == 0 {
		return "Not enough history in " + c.label + " to trend payees."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Payee trends by month, %s (%s):\n", c.label, agBucketLabels(bounds))
	for _, r := range rows {
		parts := make([]string, 0, len(r.Spend))
		for _, v := range r.Spend {
			parts = append(parts, c.fmtM(v))
		}
		fmt.Fprintf(&b, "  %s: %s\n", r.Payee, strings.Join(parts, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptSpendingByTag(c agRptCtx) string {
	rows, err := reports.SpendingByTag(c.txns, c.start, c.end, !c.prevStart.IsZero(), c.prevStart, c.prevEnd, c.rates)
	if err != nil || len(rows) == 0 {
		return "No tagged spending in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Spending by tag, %s:\n", c.label)
	for _, r := range rows {
		prior := ""
		if r.Prior != 0 {
			prior = fmt.Sprintf(" (was %s)", c.fmtM(r.Prior))
		}
		fmt.Fprintf(&b, "  %s: %s across %s%s\n", r.Tag, c.fmtM(r.Amount), plural(r.Count, "charge"), prior)
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptSpendingByMember(c agRptCtx) string {
	rows, err := reports.SpendingByMember(c.txns, c.start, c.end, c.rates)
	if err != nil || len(rows) == 0 {
		return "No spending attributed to household members in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Spending by person, %s:\n", c.label)
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s: %s\n", c.memberOf(r.MemberID), c.fmtM(r.Amount))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptSpendingByWeekday(c agRptCtx) string {
	totals, err := reports.SpendingByWeekday(c.txns, c.start, c.end, c.rates)
	if err != nil {
		return "Couldn't compute the weekday split."
	}
	var any bool
	var b strings.Builder
	fmt.Fprintf(&b, "Spending by weekday, %s:\n", c.label)
	for d := time.Sunday; d <= time.Saturday; d++ {
		if totals[d] != 0 {
			any = true
		}
		fmt.Fprintf(&b, "  %s: %s\n", d.String(), c.fmtM(totals[d]))
	}
	if !any {
		return "No spending in " + c.label + "."
	}
	if peak, ok := reports.PeakWeekday(totals); ok {
		fmt.Fprintf(&b, "Heaviest day: %s.", peak.String())
	}
	return strings.TrimRight(b.String(), "\n")
}

// agCustomFieldDef resolves the custom field a by-field section groups on: the
// one the model named, or the first defined.
func agCustomFieldDef(c agRptCtx) (customfields.Def, bool) {
	defs := c.app.CustomFieldDefsFor("transaction")
	if len(defs) == 0 {
		return customfields.Def{}, false
	}
	q := strings.ToLower(strings.TrimSpace(c.field))
	if q == "" {
		return defs[0], true
	}
	for _, d := range defs {
		if strings.ToLower(d.Key) == q || strings.ToLower(d.Label) == q {
			return d, true
		}
	}
	for _, d := range defs {
		if strings.Contains(strings.ToLower(d.Label), q) {
			return d, true
		}
	}
	return defs[0], true
}

func agRptSpendingByCustomField(c agRptCtx) string {
	def, ok := agCustomFieldDef(c)
	if !ok {
		return "No custom fields are defined on transactions, so there is nothing to group by."
	}
	rows, err := reports.ByCustomField(c.txns, def.Key, c.start, c.end, c.rates)
	if err != nil || len(rows) == 0 {
		return fmt.Sprintf("No spending carries the %q field in %s.", def.Label, c.label)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Spending by %s, %s:\n", def.Label, c.label)
	for _, r := range rows {
		label := r.Value
		if label == "" {
			label = "(no value)"
		}
		fmt.Fprintf(&b, "  %s: %s\n", label, c.fmtM(r.Amount))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptMonthlyFlow(c agRptCtx) string {
	bounds := c.bounds()
	flows, err := reports.IncomeExpenseSeries(c.txns, bounds, c.rates)
	if err != nil || len(flows) == 0 {
		return "Not enough history in " + c.label + " for a month-by-month table."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Month by month, %s:\n", c.label)
	for i, f := range flows {
		fmt.Fprintf(&b, "  %s: income %s, spending %s, net %s (%d%% kept)\n",
			bounds[i].Format("2006-01"), c.fmtM(f.Income), c.fmtM(f.Expense), c.fmtM(f.Net()), f.SavingsRate())
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptSavingsRate(c agRptCtx) string {
	bounds := c.bounds()
	rates, err := reports.SavingsRateSeries(c.txns, bounds, c.rates)
	flow, _ := reports.IncomeVsExpense(c.txns, c.start, c.end, c.rates)
	var b strings.Builder
	fmt.Fprintf(&b, "Savings rate for %s as a whole: %d%% (kept %s of %s).\n",
		c.label, flow.SavingsRate(), c.fmtM(flow.Net()), c.fmtM(flow.Income))
	if err == nil && len(rates) > 0 {
		b.WriteString("By month: ")
		parts := make([]string, 0, len(rates))
		for i, r := range rates {
			parts = append(parts, fmt.Sprintf("%s %d%%", bounds[i].Format("2006-01"), r))
		}
		b.WriteString(strings.Join(parts, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptSpendingStats(c agRptCtx) string {
	s, err := reports.SpendingStats(c.txns, c.start, c.end, c.rates)
	if err != nil || s.Count == 0 {
		return "No expenses in " + c.label + "."
	}
	return fmt.Sprintf("%s in %s totalling %s — average %s, median %s.",
		plural(s.Count, "expense"), c.label, c.fmtM(s.Total), c.fmtM(s.Average), c.fmtM(s.Median))
}

func agRptNoSpendDays(c agRptCtx) string {
	n := reports.NoSpendDays(c.txns, c.start, c.end, c.now)
	return fmt.Sprintf("%s with no spending at all in %s.", plural(n, "day"), c.label)
}

func agRptRunway(c agRptCtx) string {
	liquid, _ := ledger.LiquidBalance(c.accounts, c.txns, c.rates)
	flows, _ := reports.IncomeExpenseSeries(c.txns, c.bounds(), c.rates)
	burn := reports.AverageMonthlyExpense(flows)
	r := reports.EstimateRunway(liquid.Amount, burn)
	if burn <= 0 {
		return fmt.Sprintf("Liquid cash %s; no spending in %s to burn against, so there is no runway to estimate.", c.fmtM(liquid.Amount), c.label)
	}
	return fmt.Sprintf("Liquid cash %s against an average monthly burn of %s (%s) — about %d months of runway.",
		c.fmtM(liquid.Amount), c.fmtM(burn), c.label, r.Months)
}

func agRptCostOfMoney(c agRptCtx) string {
	mc, err := reports.CostOfMoney(c.txns, c.cats, c.start, c.end, c.rates)
	if err != nil || (mc.FeeCount == 0 && mc.InterestCount == 0) {
		return "No interest or fees found in " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cost of money, %s: interest %s across %s, fees %s across %s (total %s).\n",
		c.label, c.fmtM(mc.InterestTotal), plural(mc.InterestCount, "charge"),
		c.fmtM(mc.FeeTotal), plural(mc.FeeCount, "charge"), c.fmtM(mc.InterestTotal+mc.FeeTotal))
	for i, it := range mc.Items {
		if i >= 15 {
			fmt.Fprintf(&b, "  …and %d more.\n", len(mc.Items)-i)
			break
		}
		kind := "fee"
		if it.Interest {
			kind = "interest"
		}
		fmt.Fprintf(&b, "  %s  %s  %s  [%s]\n", it.Date.Format("2006-01-02"), it.Desc, c.fmtM(it.Amount), kind)
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptNetWorth(c agRptCtx) string {
	net, assets, liab, err := ledger.NetWorth(c.accounts, c.txns, c.rates)
	if err != nil {
		return "Couldn't compute net worth."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Net worth %s — assets %s, liabilities %s. By account:\n",
		c.fmtM(net.Amount), c.fmtM(assets.Amount), c.fmtM(liab.Amount))
	for _, a := range c.accounts {
		if a.Archived {
			continue
		}
		bal, err := ledger.Balance(a, c.txns)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "  %s (%s): %s\n", a.Name, a.Type, fmtMoney(bal))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptInvestmentPerformance(c agRptCtx) string {
	perf, err := reports.InvestmentPerformance(c.accounts, c.txns, c.rates)
	if err != nil || len(perf) == 0 {
		return "No investment, retirement or crypto accounts to report on."
	}
	var b strings.Builder
	b.WriteString("Investment performance (from the account's own history — no live prices):\n")
	for _, p := range perf {
		line := fmt.Sprintf("  %s: put in %s, now worth %s, gain %s", p.Name, c.fmtM(p.Invested), c.fmtM(p.Current), c.fmtM(p.Gain))
		if p.Invested > 0 {
			line += fmt.Sprintf(" (%+.1f%%)", float64(p.ReturnBips)/100)
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptDeductible(c agRptCtx) string {
	s, err := reports.DeductibleTotals(c.txns, c.cats, c.start, c.end, c.rates)
	if err != nil || len(s.Rows) == 0 {
		return "No categories are flagged deductible, so there is nothing to total."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Deductible totals, %s (total %s):\n", c.label, c.fmtM(s.Total))
	for _, r := range s.Rows {
		fmt.Fprintf(&b, "  %s: %s\n", c.nameOf(r.CategoryID), c.fmtM(r.Amount))
	}
	return strings.TrimRight(b.String(), "\n")
}

func agRptYearTax(c agRptCtx) string {
	year := c.now.Year()
	if !c.start.IsZero() {
		year = c.start.Year()
	}
	s, err := reports.YearTax(c.txns, year, c.start, c.end, c.rates)
	if err != nil || len(s.Rows) == 0 {
		return "Nothing to roll up for " + c.label + "."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tax summary for %s: income %s, expense %s, net %s.\n",
		c.label, c.fmtM(s.TotalIncome), c.fmtM(s.TotalExpense), c.fmtM(s.NetIncome))
	for _, r := range s.Rows {
		fmt.Fprintf(&b, "  %s: income %s, expense %s, net %s\n",
			c.nameOf(r.CategoryID), c.fmtM(r.Income), c.fmtM(r.Expense), c.fmtM(r.Net))
	}
	return strings.TrimRight(b.String(), "\n")
}

// ── Traces: one report row back to the transactions behind it ────────────────

func agAbs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// agCollect returns the counted transactions in the window matching keep.
func agCollect(c agRptCtx, keep func(domain.Transaction) bool) []domain.Transaction {
	var out []domain.Transaction
	for _, t := range c.txns {
		if c.counted(t) && keep(t) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.After(out[j].Date)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func agRptTraceCategory(c agRptCtx, row string) ([]domain.Transaction, string) {
	id, ok := c.resolveCatID(row)
	if !ok {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsExpense() && t.CategoryID == id
	}), "spending in " + c.nameOf(id)
}

func agRptTraceIncomeCategory(c agRptCtx, row string) ([]domain.Transaction, string) {
	id, ok := c.resolveCatID(row)
	if !ok {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsIncome() && t.CategoryID == id
	}), "income in " + c.nameOf(id)
}

func agRptTraceTaxCategory(c agRptCtx, row string) ([]domain.Transaction, string) {
	id, ok := c.resolveCatID(row)
	if !ok {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return (t.IsExpense() || t.IsIncome()) && t.CategoryID == id
	}), "income and spending in " + c.nameOf(id)
}

// agRowMatchesName reports whether a transaction is one of the rows a
// payee-shaped report line stands for.
//
// The order matters and is not arbitrary: TopPayees, PayeeTrends and
// LargestExpenses all group by the DESCRIPTION, so the row label the model
// reads off a section is a description, and matching it against t.Payee first
// would fail to open the very row that was just printed. The payee field is
// still checked afterwards, because a user asking about "Publix" means the
// merchant whatever the ledger filed the line under.
func agRowMatchesName(t domain.Transaction, q string) bool {
	desc := strings.ToLower(strings.TrimSpace(t.Desc))
	payee := strings.ToLower(strings.TrimSpace(t.Payee))
	if desc == q || payee == q {
		return true
	}
	return (desc != "" && strings.Contains(desc, q)) || (payee != "" && strings.Contains(payee, q))
}

func agRptTracePayee(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	if q == "" {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsExpense() && agRowMatchesName(t, q)
	}), "spending on " + strings.TrimSpace(row)
}

func agRptTraceIncomePayee(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	if q == "" {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsIncome() && agRowMatchesName(t, q)
	}), "income from " + strings.TrimSpace(row)
}

func agRptTraceTag(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	if q == "" {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		if !t.IsExpense() {
			return false
		}
		for _, tag := range t.Tags {
			if strings.ToLower(strings.TrimSpace(tag)) == q {
				return true
			}
		}
		return false
	}), "spending tagged " + strings.TrimSpace(row)
}

func agRptTraceMember(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	var id string
	found := q == "unassigned"
	for _, m := range c.members {
		if strings.ToLower(m.Name) == q || (q != "" && strings.Contains(strings.ToLower(m.Name), q)) {
			id, found = m.ID, true
			break
		}
	}
	if !found {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsExpense() && t.MemberID == id
	}), "spending by " + c.memberOf(id)
}

func agRptTraceWeekday(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	for d := time.Sunday; d <= time.Saturday; d++ {
		name := strings.ToLower(d.String())
		if name == q || (len(q) >= 3 && strings.HasPrefix(name, q)) {
			return agCollect(c, func(t domain.Transaction) bool {
				return t.IsExpense() && t.Date.Weekday() == d
			}), "spending on " + d.String() + "s"
		}
	}
	return nil, ""
}

func agRptTraceMonth(c agRptCtx, row string) ([]domain.Transaction, string) {
	month, ok := reports.ParseMonth(row, c.now)
	if !ok {
		return nil, ""
	}
	next := dateutil.AddMonths(month, 1)
	return agCollect(c, func(t domain.Transaction) bool {
		return !t.IsTransfer() && dateutil.InRange(t.Date, month, next)
	}), "income and spending in " + month.Format("January 2006")
}

func agRptTraceAccount(c agRptCtx, row string) ([]domain.Transaction, string) {
	q := strings.ToLower(strings.TrimSpace(row))
	var id, name string
	for _, a := range c.accounts {
		if strings.ToLower(a.Name) == q || (q != "" && strings.Contains(strings.ToLower(a.Name), q)) {
			id, name = a.ID, a.Name
			break
		}
	}
	if id == "" {
		return nil, ""
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.AccountID == id || t.TransferAccountID == id
	}), "activity in " + name
}

func agRptTraceCostOfMoney(c agRptCtx, row string) ([]domain.Transaction, string) {
	mc, err := reports.CostOfMoney(c.txns, c.cats, c.start, c.end, c.rates)
	if err != nil {
		return nil, ""
	}
	q := strings.ToLower(strings.TrimSpace(row))
	wantInterest, wantFee := q == "interest", q == "fee" || q == "fees"
	keep := map[string]bool{}
	for _, it := range mc.Items {
		switch {
		case wantInterest && it.Interest, wantFee && !it.Interest:
			keep[strings.ToLower(it.Desc)] = true
		case !wantInterest && !wantFee && q != "" && strings.Contains(strings.ToLower(it.Desc), q):
			keep[strings.ToLower(it.Desc)] = true
		}
	}
	if len(keep) == 0 {
		return nil, ""
	}
	what := "the matching charges"
	switch {
	case wantInterest:
		what = "interest charges"
	case wantFee:
		what = "fees"
	}
	return agCollect(c, func(t domain.Transaction) bool {
		return t.IsExpense() && keep[strings.ToLower(strings.TrimSpace(t.Desc))]
	}), what
}

func agRptTraceCustomField(c agRptCtx, row string) ([]domain.Transaction, string) {
	def, ok := agCustomFieldDef(c)
	if !ok {
		return nil, ""
	}
	q := strings.ToLower(strings.TrimSpace(row))
	wantEmpty := q == "" || q == "(no value)" || q == "no value"
	return agCollect(c, func(t domain.Transaction) bool {
		if !t.IsExpense() {
			return false
		}
		v, has := t.Custom[def.Key]
		got := ""
		if has && v != nil {
			got = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		}
		if wantEmpty {
			return got == ""
		}
		return got == q
	}), fmt.Sprintf("spending where %s is %q", def.Label, row)
}

// agRptTraceMoneyFlow opens one node or ribbon of the money-flow diagram. A
// pooled node ("Everything else") resolves to every category inside it, which
// is the whole reason the diagram carries its category ids.
func agRptTraceMoneyFlow(c agRptCtx, row string) ([]domain.Transaction, string) {
	d := agMoneyFlow(c)
	q := strings.ToLower(strings.TrimSpace(row))
	var node reports.MoneyFlowNode
	var found bool
	for _, n := range d.Nodes {
		if strings.ToLower(n.Label) == q {
			node, found = n, true
			break
		}
	}
	if !found {
		for _, n := range d.Nodes {
			if q != "" && strings.Contains(strings.ToLower(n.Label), q) {
				node, found = n, true
				break
			}
		}
	}
	if !found {
		return nil, ""
	}
	switch node.Kind {
	case reports.FlowIncomeHub:
		return agCollect(c, func(t domain.Transaction) bool { return t.IsIncome() }), "every deposit that fed the period"
	case reports.FlowSavings, reports.FlowFromSavings:
		// Neither is a group of records — one is income minus expense, the
		// other the gap that opened. Say so rather than returning a plausible
		// but wrong set of rows.
		return nil, ""
	}
	ids := map[string]bool{}
	for _, id := range node.CategoryIDs {
		ids[id] = true
	}
	income := node.Kind == reports.FlowIncomeSource || node.Kind == reports.FlowOtherIncome
	rows := agCollect(c, func(t domain.Transaction) bool {
		if !ids[t.CategoryID] {
			return false
		}
		if income {
			return t.IsIncome()
		}
		return t.IsExpense()
	})
	what := "the " + node.Label + " ribbon"
	if node.Kind.Pooled() {
		what = fmt.Sprintf("the %s ribbon (%s pooled)", node.Label, plural(len(node.CategoryIDs), "category"))
	}
	return rows, what
}

// ── Rendering transactions for the model ─────────────────────────────────────

// agTxnShortID is the prefix of a transaction id the tools quote and accept.
// Full 32-hex ids in every row would cost more tokens than the rows themselves,
// and a 8-hex prefix is unambiguous in a household ledger — update_transaction
// re-resolves it and refuses rather than guessing if two ever collide.
const agTxnShortID = 8

func agShortID(id string) string {
	if len(id) <= agTxnShortID {
		return id
	}
	return id[:agTxnShortID]
}

// agRenderTxns lists transactions for the model with the id it needs to edit
// them, the total, and an honest note when the list was cut short.
func agRenderTxns(c agRptCtx, rows []domain.Transaction, header string, limit int) string {
	if len(rows) == 0 {
		return "No transactions behind " + header + " in " + c.label + "."
	}
	var total int64
	for _, t := range rows {
		if conv, err := c.rates.Convert(t.Amount.Abs(), c.base); err == nil {
			total += conv.Amount
		}
	}
	shown := rows
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s behind %s, %s — %s totalling %s:\n",
		plural(len(rows), "transaction"), header, c.label, plural(len(rows), "row"), c.fmtM(total))
	for _, t := range shown {
		label := strings.TrimSpace(t.Payee)
		if label == "" {
			label = strings.TrimSpace(t.Desc)
		}
		line := fmt.Sprintf("  %s  %s  %s  %s  [%s]",
			agShortID(t.ID), t.Date.Format("2006-01-02"), label, fmtMoney(t.Amount), c.nameOf(t.CategoryID))
		if t.MemberID != "" {
			line += " {" + c.memberOf(t.MemberID) + "}"
		}
		if len(t.Tags) > 0 {
			line += " #" + strings.Join(t.Tags, " #")
		}
		b.WriteString(line + "\n")
	}
	if len(shown) < len(rows) {
		fmt.Fprintf(&b, "…%d more not shown; raise `limit` or narrow the window to see them.\n", len(rows)-len(shown))
	}
	b.WriteString("The first column is the transaction id — pass it to update_transaction to edit one.")
	return b.String()
}

// ── Tools ────────────────────────────────────────────────────────────────────

// agToolsReports exposes the Reports surface: what sections exist, what each one
// says over a window, and the transactions behind any row of them.
func agToolsReports(app *appstate.App, base string, rates currency.Rates, tier aicontext.ConversationTier) []chatTool {
	// AG17: the sections that name individual merchants are withheld under the
	// aggregates-only tier, exactly as the transaction-level read tools are.
	// Filtering here means the model is never offered the section rather than
	// being told not to ask for it.
	aggregatesOnly := tier == aicontext.TierAggregatesOnly
	return append([]chatTool{
		{
			spec: ai.FunctionTool("list_report_sections",
				"List every section of the Reports surface the assistant can read, with what each one answers and whether its rows can be traced back to transactions. Call this when the user asks about a report, a chart, the money-flow diagram, or a figure they can see on a report page.",
				json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				var b strings.Builder
				b.WriteString("Report sections (use the id with report_section):\n")
				for _, s := range agReportSections() {
					if aggregatesOnly && s.Detail {
						continue
					}
					line := fmt.Sprintf("  %s — %s", s.ID, s.About)
					if s.Trace != nil {
						line += fmt.Sprintf(" Traceable: pass %s as `row` to trace_report_row.", s.RowHint)
					}
					b.WriteString(line + "\n")
				}
				b.WriteString("Every section takes a window: period (this_month, last_month, this_quarter, last_quarter, this_year, last_year, year_to_date, last_12_months, all) or explicit from/to dates.")
				return b.String()
			},
		},
		{
			spec: ai.FunctionTool("report_section",
				"Read one section of the Reports surface over a window — the same figures the report page shows, computed the same way. Use list_report_sections first if unsure which section answers the question.",
				json.RawMessage(`{"type":"object","properties":{"section":{"type":"string","description":"a section id from list_report_sections"},`+agPeriodEnum+`,"field":{"type":"string","description":"for spending_by_custom_field: which custom field to group by"}},"required":["section"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Section string `json:"section"`
					Period  string `json:"period"`
					From    string `json:"from"`
					To      string `json:"to"`
					Field   string `json:"field"`
				}
				_ = json.Unmarshal(raw, &a)
				sec, ok := agFindSection(a.Section)
				if !ok {
					return fmt.Sprintf("No report section called %q. Available: %s.", a.Section, agSectionIDs())
				}
				if aggregatesOnly && sec.Detail {
					return fmt.Sprintf("The %s section names individual merchants, and this is an aggregates-only conversation. Switch the privacy level to full detail to read it, or ask for a category-level section instead.", sec.ID)
				}
				c := agBuildRptCtx(app, base, rates, a.Period, a.From, a.To, a.Field)
				return sec.Render(c)
			},
		},
		{
			spec: ai.FunctionTool("trace_report_row",
				"Open one row of a report section and get the exact transactions behind it, each with its id. Use this whenever the user asks what a report figure is made of, why a total looks wrong, or wants to fix the transactions behind it — then edit them with update_transaction.",
				json.RawMessage(`{"type":"object","properties":{"section":{"type":"string","description":"a section id from list_report_sections"},"row":{"type":"string","description":"which row to open — a category, payee, tag, person, weekday, month or money-flow node, depending on the section"},`+agPeriodEnum+`,"field":{"type":"string"},"limit":{"type":"integer","description":"rows to show, default 25, max 100"}},"required":["section","row"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Section string `json:"section"`
					Row     string `json:"row"`
					Period  string `json:"period"`
					From    string `json:"from"`
					To      string `json:"to"`
					Field   string `json:"field"`
					Limit   int    `json:"limit"`
				}
				_ = json.Unmarshal(raw, &a)
				sec, ok := agFindSection(a.Section)
				if !ok {
					return fmt.Sprintf("No report section called %q. Available: %s.", a.Section, agSectionIDs())
				}
				if sec.Trace == nil {
					return fmt.Sprintf("The %s section is a computed figure, not a group of records, so there are no transactions to open. Read it with report_section instead.", sec.ID)
				}
				c := agBuildRptCtx(app, base, rates, a.Period, a.From, a.To, a.Field)
				rows, what := sec.Trace(c, a.Row)
				if what == "" {
					// Two different things land here and the message has to cover
					// both honestly: no row goes by that name, OR the row is a
					// computed figure rather than a group of records. Savings is
					// income minus spending — returning a plausible set of rows
					// for it would be worse than saying it has none.
					return fmt.Sprintf("%q is not a row of %s that can be opened — either nothing goes by that name, or it is a computed figure rather than a group of records (savings, for instance, is income minus spending; trace the categories on either side of it instead). Pass %s. Read the section first with report_section to see its rows.",
						a.Row, sec.ID, sec.RowHint)
				}
				limit := a.Limit
				if limit <= 0 {
					limit = 25
				}
				if limit > 100 {
					limit = 100
				}
				return agRenderTxns(c, rows, what, limit)
			},
		},
	}, agToolsLedgerEdit(app, base, rates)...)
}
