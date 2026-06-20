package aicontext

import (
	"strings"
	"testing"
)

func sampleInputs() Inputs {
	return Inputs{
		PeriodLabel: "June 2026",
		NetWorth:    "$12,000.00", Assets: "$15,000.00", Liabilities: "$3,000.00",
		Income: "$4,200.00", Expense: "$2,600.00",
		Accounts:      []Account{{"Checking", "bank", "$2,000.00"}, {"Visa", "credit", "-$500.00"}},
		Budgets:       []Budget{{"Groceries", "$300.00", "$400.00", "on track"}},
		Goals:         []Goal{{Name: "Emergency fund", Percent: 60, Target: "$10,000.00", TargetDate: "Dec 2026"}},
		TopCategories: []Bucket{{"Groceries", "$300"}, {"Dining", "$150"}, {"Gas", "$90"}, {"Fun", "$60"}, {"Misc", "$30"}, {"Sixth", "$10"}},
		TopPayees:     []Bucket{{"Whole Foods", "$220"}},
		Formulas:      []Formula{{"Savings rate", "21%"}, {"Runway", "5.2 months"}},
		RecentTxns:    []Txn{{"2026-06-05", "Coffee", "-$4.50", "Dining"}, {"2026-06-04", "Paycheck", "+$4,200.00", "Income"}},
	}
}

func TestBuildTierGating(t *testing.T) {
	in := sampleInputs()

	agg := Build(in, Opts{Tier: TierAggregates})
	if len(agg.Formulas) != 0 || len(agg.Budgets) != 0 || len(agg.RecentTxns) != 0 {
		t.Errorf("aggregates tier should carry no formulas/budgets/txns: %+v", agg)
	}
	if agg.AccountCount != 2 || agg.NetWorth != "$12,000.00" {
		t.Errorf("aggregates should still have count + net worth: %+v", agg)
	}

	f := Build(in, Opts{Tier: TierFormulas})
	if len(f.Formulas) != 2 || len(f.Budgets) != 0 {
		t.Errorf("formulas tier adds KPIs but not breakdowns: %+v", f)
	}

	bd := Build(in, Opts{Tier: TierBreakdowns})
	if len(bd.Budgets) != 1 || len(bd.Goals) != 1 || len(bd.Accounts) != 2 || len(bd.RecentTxns) != 0 {
		t.Errorf("breakdowns tier adds budgets/goals/accounts but not txns: %+v", bd)
	}

	tx := Build(in, Opts{Tier: TierTransactions})
	if len(tx.RecentTxns) != 2 {
		t.Errorf("transactions tier adds recent txns: %+v", tx)
	}
}

func TestBuildCaps(t *testing.T) {
	in := sampleInputs()
	c := Build(in, Opts{Tier: TierTransactions, TopN: 3, RecentN: 1})
	if len(c.TopCategories) != 3 {
		t.Errorf("TopN=3 should cap categories to 3, got %d", len(c.TopCategories))
	}
	if len(c.RecentTxns) != 1 {
		t.Errorf("RecentN=1 should cap txns to 1, got %d", len(c.RecentTxns))
	}
	// Defaults apply when zero.
	d := Build(in, Opts{Tier: TierTransactions})
	if len(d.TopCategories) != 5 {
		t.Errorf("default TopN should cap the 6-item list to 5, got %d", len(d.TopCategories))
	}
}

func TestPromptRendersSectionsByTier(t *testing.T) {
	in := sampleInputs()
	full := Build(in, Opts{Tier: TierTransactions}).Prompt()
	for _, want := range []string{
		"## Financial context (June 2026)",
		"Net worth $12,000.00 (assets $15,000.00, liabilities $3,000.00)",
		"across 2 accounts",
		"### Your KPIs", "Savings rate: 21%",
		"### Budgets", "Groceries: $300.00 of $400.00 (on track)",
		"### Goals", "Emergency fund: 60% of $10,000.00 (by Dec 2026)",
		"### Top spending categories", "### Recent transactions", "Coffee",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("full prompt missing %q\n---\n%s", want, full)
		}
	}

	// Aggregates tier omits the richer sections.
	agg := Build(in, Opts{Tier: TierAggregates}).Prompt()
	for _, absent := range []string{"### Your KPIs", "### Budgets", "### Recent transactions"} {
		if strings.Contains(agg, absent) {
			t.Errorf("aggregates prompt should not contain %q\n---\n%s", absent, agg)
		}
	}
	if !strings.Contains(agg, "Net worth $12,000.00") {
		t.Errorf("aggregates prompt should still have the headline: %s", agg)
	}
}

func TestPromptMissingValuesShowDash(t *testing.T) {
	c := Build(Inputs{}, Opts{Tier: TierAggregates})
	p := c.Prompt()
	if !strings.Contains(p, "Net worth —") {
		t.Errorf("empty net worth should render an en-dash: %s", p)
	}
	if !strings.Contains(p, "across 0 accounts") {
		t.Errorf("empty inputs should report 0 accounts: %s", p)
	}
}
