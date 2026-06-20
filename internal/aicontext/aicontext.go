// Package aicontext is the pure, MCP-shaped context builder for the Insights agent
// (C89 phase 1): it assembles a bounded, structured snapshot of the user's finances
// — including every enabled Formula evaluated to its current value — and serializes
// it into a compact block for the model's system prompt. It replaces the
// 4-aggregate ai.FinancialContext and, on its own, fixes the C59 "Q&A context is too
// thin" gap.
//
// The design rule that keeps "inject everything" tractable: inject a SUMMARY, never
// the raw ledger — bounded by top-N / recent-N and a privacy tier — and let the
// agent pull detail via tools (C82). Inputs are pre-formatted strings, so this
// package stays pure (no appstate/money/syscall/js) and unit-tests natively.
package aicontext

import (
	"strconv"
	"strings"
)

// Tier is how much context the user has opted to share, from least to most
// revealing (the privacy shift from B17/C45 is deliberate and opt-in). Each higher
// tier adds a section; the default is conservative.
type Tier int

const (
	TierAggregates   Tier = iota // net worth, period income/expense, account count
	TierFormulas                 // + the user's evaluated KPIs (cheap, high-signal)
	TierBreakdowns               // + accounts, budgets, goals, top categories/payees
	TierTransactions             // + recent transactions
)

// Account is one account's headline (name, type, formatted balance).
type Account struct{ Name, Type, Balance string }

// Budget is one budget's state for the period (formatted spent/limit + a state word).
type Budget struct{ Name, Spent, Limit, State string }

// Goal is one goal's progress (percent 0..100, formatted target, optional date).
type Goal struct {
	Name       string
	Percent    int
	Target     string
	TargetDate string
}

// Bucket is a labeled amount for a breakdown row (a category or payee total).
type Bucket struct{ Label, Amount string }

// Formula is one of the user's enabled KPIs, already evaluated to a display value.
type Formula struct{ Name, Value string }

// Txn is one recent transaction, summarized.
type Txn struct{ Date, Desc, Amount, Category string }

// Inputs is everything the caller (the Insights/appstate seam) gathers; aicontext
// only structures, bounds, and serializes it. Money and dates arrive pre-formatted.
type Inputs struct {
	PeriodLabel   string
	NetWorth      string
	Assets        string
	Liabilities   string
	Income        string
	Expense       string
	Accounts      []Account
	Budgets       []Budget
	Goals         []Goal
	TopCategories []Bucket
	TopPayees     []Bucket
	Formulas      []Formula
	RecentTxns    []Txn
}

// Opts bound and tier the build. TopN caps each breakdown list (default 5); RecentN
// caps recent transactions (default 10); Tier gates which sections are included.
type Opts struct {
	Tier    Tier
	TopN    int
	RecentN int
}

// Context is the assembled, bounded snapshot. Lists are already capped and tier-
// gated; Prompt renders it for the system prompt.
type Context struct {
	Period        string
	NetWorth      string
	Assets        string
	Liabilities   string
	Income        string
	Expense       string
	AccountCount  int
	Accounts      []Account
	Budgets       []Budget
	Goals         []Goal
	TopCategories []Bucket
	TopPayees     []Bucket
	Formulas      []Formula
	RecentTxns    []Txn
}

// Build assembles a bounded Context from inputs at the given tier, capping lists by
// TopN/RecentN. The account count is always included; richer sections appear only
// at their tier and above.
func Build(in Inputs, opts Opts) Context {
	if opts.TopN <= 0 {
		opts.TopN = 5
	}
	if opts.RecentN <= 0 {
		opts.RecentN = 10
	}
	c := Context{
		Period:       in.PeriodLabel,
		NetWorth:     in.NetWorth,
		Assets:       in.Assets,
		Liabilities:  in.Liabilities,
		Income:       in.Income,
		Expense:      in.Expense,
		AccountCount: len(in.Accounts),
	}
	if opts.Tier >= TierFormulas {
		c.Formulas = in.Formulas
	}
	if opts.Tier >= TierBreakdowns {
		c.Accounts = in.Accounts
		c.Budgets = in.Budgets
		c.Goals = in.Goals
		c.TopCategories = capBuckets(in.TopCategories, opts.TopN)
		c.TopPayees = capBuckets(in.TopPayees, opts.TopN)
	}
	if opts.Tier >= TierTransactions {
		c.RecentTxns = capTxns(in.RecentTxns, opts.RecentN)
	}
	return c
}

// Prompt renders the context as a compact Markdown block for the system prompt.
// Empty sections are omitted so the prompt stays small.
func (c Context) Prompt() string {
	var b strings.Builder
	head := "## Financial context"
	if c.Period != "" {
		head += " (" + c.Period + ")"
	}
	b.WriteString(head + "\n")
	b.WriteString("Net worth " + dash(c.NetWorth))
	if c.Assets != "" || c.Liabilities != "" {
		b.WriteString(" (assets " + dash(c.Assets) + ", liabilities " + dash(c.Liabilities) + ")")
	}
	b.WriteString(". This period: income " + dash(c.Income) + ", spending " + dash(c.Expense) +
		", across " + strconv.Itoa(c.AccountCount) + " accounts.\n")

	if len(c.Formulas) > 0 {
		b.WriteString("\n### Your KPIs\n")
		for _, f := range c.Formulas {
			b.WriteString("- " + f.Name + ": " + dash(f.Value) + "\n")
		}
	}
	if len(c.Accounts) > 0 {
		b.WriteString("\n### Accounts\n")
		for _, a := range c.Accounts {
			b.WriteString("- " + a.Name + " (" + a.Type + "): " + dash(a.Balance) + "\n")
		}
	}
	if len(c.Budgets) > 0 {
		b.WriteString("\n### Budgets\n")
		for _, bd := range c.Budgets {
			b.WriteString("- " + bd.Name + ": " + dash(bd.Spent) + " of " + dash(bd.Limit) + " (" + bd.State + ")\n")
		}
	}
	if len(c.Goals) > 0 {
		b.WriteString("\n### Goals\n")
		for _, g := range c.Goals {
			line := "- " + g.Name + ": " + strconv.Itoa(g.Percent) + "% of " + dash(g.Target)
			if g.TargetDate != "" {
				line += " (by " + g.TargetDate + ")"
			}
			b.WriteString(line + "\n")
		}
	}
	writeBuckets(&b, "Top spending categories", c.TopCategories)
	writeBuckets(&b, "Top payees", c.TopPayees)
	if len(c.RecentTxns) > 0 {
		b.WriteString("\n### Recent transactions\n")
		for _, t := range c.RecentTxns {
			line := "- " + t.Date + " " + t.Desc + " " + t.Amount
			if t.Category != "" {
				line += " (" + t.Category + ")"
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func writeBuckets(b *strings.Builder, title string, rows []Bucket) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("\n### " + title + "\n")
	for _, r := range rows {
		b.WriteString("- " + r.Label + " " + dash(r.Amount) + "\n")
	}
}

func capBuckets(in []Bucket, n int) []Bucket {
	if n > 0 && len(in) > n {
		return in[:n]
	}
	return in
}

func capTxns(in []Txn, n int) []Txn {
	if n > 0 && len(in) > n {
		return in[:n]
	}
	return in
}

// dash renders an empty value as an en-dash so a missing figure is explicit rather
// than blank in the prompt.
func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
