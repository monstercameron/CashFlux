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
