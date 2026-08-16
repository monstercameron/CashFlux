package localqa

import (
	"fmt"
	"strings"
	"testing"
)

// mockSource is a test double that satisfies the Source interface with canned
// values. Fields whose zero value represents "no data" use a separate boolean
// flag to signal unavailability.
type mockSource struct {
	liquidBalance int64

	assets      int64
	liabilities int64

	safeToSpend int64

	// category spending — keyed by lowercase category name
	categorySpend map[string]int64

	billCount int
	billTotal int64

	goalName    string
	goalCurrent int64
	goalTarget  int64
	goalOK      bool

	healthScore int
	healthBand  string
	healthOK    bool

	budgetTotal     int
	budgetOver      int
	budgetWorstName string
	budgetWorstOver int64
	budgetOK        bool

	recent []RecentTxn

	subCount   int
	subMonthly int64

	largestPayee  string
	largestAmount int64
	largestOK     bool
}

func (m *mockSource) BudgetStatus() (int, int, string, int64, bool) {
	return m.budgetTotal, m.budgetOver, m.budgetWorstName, m.budgetWorstOver, m.budgetOK
}

func (m *mockSource) RecentTransactions() []RecentTxn { return m.recent }

func (m *mockSource) Subscriptions() (int, int64) { return m.subCount, m.subMonthly }

func (m *mockSource) LargestExpense() (string, int64, bool) {
	return m.largestPayee, m.largestAmount, m.largestOK
}

func (m *mockSource) LiquidBalanceMinor() int64 { return m.liquidBalance }

func (m *mockSource) NetWorthMinor() (int64, int64) { return m.assets, m.liabilities }

func (m *mockSource) SafeToSpendMinor() int64 { return m.safeToSpend }

func (m *mockSource) SpendingOnCategoryMinor(cat string) int64 {
	return m.categorySpend[strings.ToLower(cat)]
}

func (m *mockSource) UpcomingBillsMinor() (int, int64) { return m.billCount, m.billTotal }

func (m *mockSource) TopGoal() (string, int64, int64, bool) {
	return m.goalName, m.goalCurrent, m.goalTarget, m.goalOK
}

func (m *mockSource) HealthScore() (int, string, bool) {
	return m.healthScore, m.healthBand, m.healthOK
}

// cents converts a dollar amount to minor units (cents) for readable test data.
func cents(dollars float64) int64 { return int64(dollars * 100) }

// fmtDollars is a minimal money formatter used in tests.
func fmtDollars(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	s := fmt.Sprintf("$%d.%02d", minor/100, minor%100)
	if neg {
		return "-" + s
	}
	return s
}

// baseSource returns a fully-populated mock so individual test cases can
// override only the fields they care about.
func baseSource() *mockSource {
	return &mockSource{
		liquidBalance: cents(1_250.00),
		assets:        cents(50_000.00),
		liabilities:   cents(20_000.00),
		safeToSpend:   cents(320.50),
		categorySpend: map[string]int64{
			"groceries":  cents(187.43),
			"dining out": cents(95.00),
		},
		billCount:   3,
		billTotal:   cents(480.00),
		goalName:    "Emergency Fund",
		goalCurrent: cents(2_500.00),
		goalTarget:  cents(10_000.00),
		goalOK:      true,
		healthScore: 72,
		healthBand:  "Good",
		healthOK:    true,
	}
}

func TestAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		intent      Intent
		rawText     string
		src         func() *mockSource
		wantContain []string // every string must appear in the answer
		wantOK      bool
	}{
		// ── IntentNone ────────────────────────────────────────────────────
		{
			name:    "IntentNone returns empty and false",
			intent:  IntentNone,
			rawText: "what is the weather",
			src:     baseSource,
			wantOK:  false,
		},
		// ── IntentBalance ────────────────────────────────────────────────
		{
			name:        "balance shows liquid balance",
			intent:      IntentBalance,
			rawText:     "what is my balance",
			src:         baseSource,
			wantContain: []string{"$1250.00"},
			wantOK:      true,
		},
		// ── IntentSafeToSpend ────────────────────────────────────────────
		{
			name:        "safe-to-spend shows discretionary amount",
			intent:      IntentSafeToSpend,
			rawText:     "how much can I spend",
			src:         baseSource,
			wantContain: []string{"$320.50", "available"},
			wantOK:      true,
		},
		// ── IntentNetWorth ───────────────────────────────────────────────
		{
			name:        "net worth shows assets, liabilities, and net",
			intent:      IntentNetWorth,
			rawText:     "what is my net worth",
			src:         baseSource,
			wantContain: []string{"$30000.00", "$50000.00", "$20000.00"},
			wantOK:      true,
		},
		// ── IntentSpendingByCategory ─────────────────────────────────────
		{
			name:        "spending on groceries",
			intent:      IntentSpendingByCategory,
			rawText:     "How much did I spend on groceries",
			src:         baseSource,
			wantContain: []string{"$187.43", "groceries"},
			wantOK:      true,
		},
		{
			name:        "spending on multi-word category",
			intent:      IntentSpendingByCategory,
			rawText:     "how much spent on dining out",
			src:         baseSource,
			wantContain: []string{"$95.00", "dining out"},
			wantOK:      true,
		},
		{
			name:        "spending query with no extractable category",
			intent:      IntentSpendingByCategory,
			rawText:     "how much did I spend in total",
			src:         baseSource,
			wantContain: []string{"couldn't tell"},
			wantOK:      true,
		},
		// ── IntentUpcomingBills ──────────────────────────────────────────
		{
			name:        "upcoming bills shows count and total",
			intent:      IntentUpcomingBills,
			rawText:     "what bills are due",
			src:         baseSource,
			wantContain: []string{"3", "$480.00", "bills"},
			wantOK:      true,
		},
		{
			name:    "no upcoming bills",
			intent:  IntentUpcomingBills,
			rawText: "upcoming bills",
			src: func() *mockSource {
				s := baseSource()
				s.billCount = 0
				s.billTotal = 0
				return s
			},
			wantContain: []string{"no upcoming bills"},
			wantOK:      true,
		},
		{
			name:    "single bill uses singular grammar",
			intent:  IntentUpcomingBills,
			rawText: "bills due",
			src: func() *mockSource {
				s := baseSource()
				s.billCount = 1
				s.billTotal = cents(150.00)
				return s
			},
			wantContain: []string{"1 upcoming bill", "$150.00"},
			wantOK:      true,
		},
		// ── IntentGoalProgress ───────────────────────────────────────────
		{
			name:        "goal progress shows percentage and amounts",
			intent:      IntentGoalProgress,
			rawText:     "how close am I to my goal",
			src:         baseSource,
			wantContain: []string{"25%", "Emergency Fund", "$2500.00", "$10000.00"},
			wantOK:      true,
		},
		{
			name:    "no goals configured",
			intent:  IntentGoalProgress,
			rawText: "goal status",
			src: func() *mockSource {
				s := baseSource()
				s.goalOK = false
				return s
			},
			wantContain: []string{"haven't set up"},
			wantOK:      true,
		},
		{
			name:    "goal with zero target",
			intent:  IntentGoalProgress,
			rawText: "goal status",
			src: func() *mockSource {
				s := baseSource()
				s.goalTarget = 0
				return s
			},
			wantContain: []string{"no target amount"},
			wantOK:      true,
		},
		// ── IntentHealthScore ────────────────────────────────────────────
		{
			name:        "health score shows score and band",
			intent:      IntentHealthScore,
			rawText:     "what is my financial health score",
			src:         baseSource,
			wantContain: []string{"72", "Good"},
			wantOK:      true,
		},
		{
			name:    "health score unavailable",
			intent:  IntentHealthScore,
			rawText: "how am I doing financially",
			src: func() *mockSource {
				s := baseSource()
				s.healthOK = false
				return s
			},
			wantContain: []string{"Not enough data"},
			wantOK:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src()
			got, ok := Answer(tc.intent, src, tc.rawText, fmtDollars)

			if ok != tc.wantOK {
				t.Fatalf("Answer() ok = %v, want %v (answer=%q)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				if got != "" {
					t.Fatalf("Answer() returned non-empty string %q for ok=false case", got)
				}
				return
			}
			for _, sub := range tc.wantContain {
				if !strings.Contains(got, sub) {
					t.Errorf("Answer() = %q\n  missing expected substring %q", got, sub)
				}
			}
		})
	}
}

// TestAnswerIntegration verifies that Answer cooperates correctly with Match
// and ExtractCategory end-to-end — no mocking of the classification layer.
func TestAnswerIntegration(t *testing.T) {
	t.Parallel()

	src := baseSource()
	// ExtractCategory takes everything after the last " on ", so the category
	// phrase must be the tail of the sentence. Using "spent on groceries" (no
	// trailing words) ensures the extracted category is exactly "groceries".
	input := "How much did I spend on groceries"

	intent, matched := Match(input)
	if !matched || intent != IntentSpendingByCategory {
		t.Fatalf("Match(%q) = (%v,%v); want (IntentSpendingByCategory, true)", input, intent, matched)
	}

	answer, ok := Answer(intent, src, input, fmtDollars)
	if !ok {
		t.Fatalf("Answer() ok=false; want true")
	}
	for _, want := range []string{"$187.43", "groceries"} {
		if !strings.Contains(answer, want) {
			t.Errorf("integration answer %q missing %q", answer, want)
		}
	}
}

// The four intents added for G2-C8. Each is a question the device can answer
// exactly, for free, that used to cost a paid model call.

func TestBudgetStatusAnswers(t *testing.T) {
	money := func(v int64) string { return fmtTestMoney(v) }
	for _, tc := range []struct {
		name string
		src  *mockSource
		want string
	}{
		{
			name: "no budgets",
			src:  &mockSource{},
			want: "You haven't set up any budgets yet.",
		},
		{
			name: "all within",
			src:  &mockSource{budgetTotal: 4, budgetOK: true},
			want: "All 4 budgets are within their limits.",
		},
		{
			name: "one over reads in the singular",
			src:  &mockSource{budgetTotal: 4, budgetOver: 1, budgetWorstName: "Dining", budgetWorstOver: 4210, budgetOK: true},
			want: "1 of your 4 budgets is over: Dining, by $42.10.",
		},
		{
			name: "several over names the worst",
			src:  &mockSource{budgetTotal: 6, budgetOver: 3, budgetWorstName: "Dining", budgetWorstOver: 4210, budgetOK: true},
			want: "3 of your 6 budgets are over. The furthest over is Dining, by $42.10.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Answer(IntentBudgetStatus, tc.src, "", money)
			if !ok || got != tc.want {
				t.Fatalf("got %q/%v, want %q", got, ok, tc.want)
			}
		})
	}
}

func TestRecentTransactionsAnswers(t *testing.T) {
	money := func(v int64) string { return fmtTestMoney(v) }
	src := &mockSource{recent: []RecentTxn{
		{Payee: "Trader Joe's", AmountMinor: 4210},
		{Payee: "Shell", AmountMinor: 3800},
		{Payee: "Costco", AmountMinor: 12200},
		{Payee: "Netflix", AmountMinor: 1599},
		{Payee: "Spotify", AmountMinor: 1199},
	}}
	got, ok := Answer(IntentRecentTransactions, src, "", money)
	if !ok {
		t.Fatal("no answer")
	}
	// Three named, read as a list, with the remainder counted rather than dumped.
	if got != "Your most recent: Trader Joe's $42.10, Shell $38.00 and Costco $122.00. There are 2 more." {
		t.Fatalf("got %q", got)
	}
}

func TestRecentTransactionsWithNothingRecorded(t *testing.T) {
	got, ok := Answer(IntentRecentTransactions, &mockSource{}, "", fmtTestMoney)
	if !ok || got != "You haven't recorded any transactions yet." {
		t.Fatalf("got %q/%v", got, ok)
	}
}

func TestRecentTransactionsSingularRemainder(t *testing.T) {
	src := &mockSource{recent: []RecentTxn{
		{Payee: "A", AmountMinor: 100}, {Payee: "B", AmountMinor: 200},
		{Payee: "C", AmountMinor: 300}, {Payee: "D", AmountMinor: 400},
	}}
	got, _ := Answer(IntentRecentTransactions, src, "", fmtTestMoney)
	if !strings.HasSuffix(got, "There is 1 more.") {
		t.Fatalf("singular remainder reads wrong: %q", got)
	}
}

func TestSubscriptionsAnswers(t *testing.T) {
	got, ok := Answer(IntentSubscriptions, &mockSource{subCount: 1, subMonthly: 1599}, "", fmtTestMoney)
	if !ok || got != "You have 1 subscription costing about $15.99 a month." {
		t.Fatalf("got %q/%v", got, ok)
	}
	many, _ := Answer(IntentSubscriptions, &mockSource{subCount: 7, subMonthly: 8420}, "", fmtTestMoney)
	if many != "You have 7 subscriptions costing about $84.20 a month." {
		t.Fatalf("got %q", many)
	}
	none, _ := Answer(IntentSubscriptions, &mockSource{}, "", fmtTestMoney)
	if none != "No recurring subscriptions are set up." {
		t.Fatalf("got %q", none)
	}
}

func TestLargestExpenseAnswers(t *testing.T) {
	got, ok := Answer(IntentLargestExpense, &mockSource{largestPayee: "Costco", largestAmount: 12200, largestOK: true}, "", fmtTestMoney)
	if !ok || got != "Your biggest expense this period was $122.00 at Costco." {
		t.Fatalf("got %q/%v", got, ok)
	}
	none, _ := Answer(IntentLargestExpense, &mockSource{}, "", fmtTestMoney)
	if none != "Nothing has been spent this period yet." {
		t.Fatalf("got %q", none)
	}
}

// fmtTestMoney renders minor units as dollars for the assertions above.
func fmtTestMoney(v int64) string {
	neg := ""
	if v < 0 {
		neg, v = "-", -v
	}
	return fmt.Sprintf("%s$%d.%02d", neg, v/100, v%100)
}
