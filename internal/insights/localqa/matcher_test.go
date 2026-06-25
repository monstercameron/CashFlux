package localqa_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/insights/localqa"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantIntent  localqa.Intent
		wantMatched bool
	}{
		// --- IntentBalance ---
		{
			name:        "bare balance keyword",
			input:       "What is my balance?",
			wantIntent:  localqa.IntentBalance,
			wantMatched: true,
		},
		{
			name:        "how much do i have",
			input:       "how much do I have right now",
			wantIntent:  localqa.IntentBalance,
			wantMatched: true,
		},
		{
			name:        "what is in my account",
			input:       "What's in my account today?",
			wantIntent:  localqa.IntentBalance,
			wantMatched: true,
		},
		{
			name:        "checking balance phrase",
			input:       "Show me my checking balance",
			wantIntent:  localqa.IntentBalance,
			wantMatched: true,
		},

		// --- IntentSpendingByCategory ---
		{
			name:        "spent on phrase",
			input:       "How much have I spent on food?",
			wantIntent:  localqa.IntentSpendingByCategory,
			wantMatched: true,
		},
		{
			name:        "spending on phrase",
			input:       "My spending on entertainment this month",
			wantIntent:  localqa.IntentSpendingByCategory,
			wantMatched: true,
		},
		{
			name:        "how much did i spend on",
			input:       "How much did I spend on groceries last week?",
			wantIntent:  localqa.IntentSpendingByCategory,
			wantMatched: true,
		},
		{
			name:        "how much have i spent on",
			input:       "How much have I spent on dining this quarter?",
			wantIntent:  localqa.IntentSpendingByCategory,
			wantMatched: true,
		},

		// --- IntentSafeToSpend ---
		{
			name:        "safe to spend phrase",
			input:       "Is it safe to spend money this week?",
			wantIntent:  localqa.IntentSafeToSpend,
			wantMatched: true,
		},
		{
			name:        "can i spend phrase",
			input:       "Can I spend $200 today?",
			wantIntent:  localqa.IntentSafeToSpend,
			wantMatched: true,
		},
		{
			name:        "how much can i spend",
			input:       "How much can I spend without going over budget?",
			wantIntent:  localqa.IntentSafeToSpend,
			wantMatched: true,
		},
		{
			name:        "free to spend phrase",
			input:       "What am I free to spend?",
			wantIntent:  localqa.IntentSafeToSpend,
			wantMatched: true,
		},

		// --- IntentNetWorth ---
		{
			name:        "net worth phrase",
			input:       "What is my net worth?",
			wantIntent:  localqa.IntentNetWorth,
			wantMatched: true,
		},
		{
			name:        "total assets phrase",
			input:       "Show me my total assets",
			wantIntent:  localqa.IntentNetWorth,
			wantMatched: true,
		},
		{
			name:        "overall financial position",
			input:       "What is my overall financial position right now?",
			wantIntent:  localqa.IntentNetWorth,
			wantMatched: true,
		},

		// --- IntentUpcomingBills ---
		{
			name:        "upcoming bills phrase",
			input:       "Show me upcoming bills",
			wantIntent:  localqa.IntentUpcomingBills,
			wantMatched: true,
		},
		{
			name:        "bills this month",
			input:       "What are my bills this month?",
			wantIntent:  localqa.IntentUpcomingBills,
			wantMatched: true,
		},
		{
			name:        "what is due",
			input:       "What's due this week?",
			wantIntent:  localqa.IntentUpcomingBills,
			wantMatched: true,
		},
		{
			name:        "due this month",
			input:       "What payments are due this month?",
			wantIntent:  localqa.IntentUpcomingBills,
			wantMatched: true,
		},

		// --- IntentGoalProgress ---
		{
			name:        "goal progress phrase",
			input:       "Show me my goal progress",
			wantIntent:  localqa.IntentGoalProgress,
			wantMatched: true,
		},
		{
			name:        "savings goal phrase",
			input:       "How is my savings goal going?",
			wantIntent:  localqa.IntentGoalProgress,
			wantMatched: true,
		},
		{
			name:        "how close am i to",
			input:       "How close am I to my vacation target?",
			wantIntent:  localqa.IntentGoalProgress,
			wantMatched: true,
		},
		{
			name:        "goal status phrase",
			input:       "Give me a goal status update",
			wantIntent:  localqa.IntentGoalProgress,
			wantMatched: true,
		},

		// --- IntentHealthScore ---
		{
			name:        "financial health phrase",
			input:       "How is my financial health?",
			wantIntent:  localqa.IntentHealthScore,
			wantMatched: true,
		},
		{
			name:        "health score phrase",
			input:       "What is my health score?",
			wantIntent:  localqa.IntentHealthScore,
			wantMatched: true,
		},
		{
			name:        "how am i doing financially",
			input:       "How am I doing financially this year?",
			wantIntent:  localqa.IntentHealthScore,
			wantMatched: true,
		},

		// --- IntentNone (no match) ---
		{
			name:        "empty string",
			input:       "",
			wantIntent:  localqa.IntentNone,
			wantMatched: false,
		},
		{
			name:        "unrelated phrase",
			input:       "What is the weather like today?",
			wantIntent:  localqa.IntentNone,
			wantMatched: false,
		},
		{
			name:        "partial keyword no context",
			input:       "tell me something interesting",
			wantIntent:  localqa.IntentNone,
			wantMatched: false,
		},
		{
			name:        "numbers only",
			input:       "1234 5678",
			wantIntent:  localqa.IntentNone,
			wantMatched: false,
		},

		// --- Precedence: SpendingByCategory beats Balance ---
		// "spent on" contains "on" but also implies a category; it should NOT
		// fall through to Balance even if the phrase also contains a balance-adjacent word.
		{
			name:        "spent-on beats balance when balance word present",
			input:       "Check my balance — how much did I spend on rent?",
			wantIntent:  localqa.IntentSpendingByCategory,
			wantMatched: true,
		},

		// --- Precedence: SafeToSpend beats Balance ---
		{
			name:        "safe-to-spend beats balance",
			input:       "Based on my balance, can I spend anything?",
			wantIntent:  localqa.IntentSafeToSpend,
			wantMatched: true,
		},

		// --- Case insensitivity ---
		{
			name:        "all caps balance",
			input:       "WHAT IS MY BALANCE",
			wantIntent:  localqa.IntentBalance,
			wantMatched: true,
		},
		{
			name:        "mixed case net worth",
			input:       "NET WORTH please",
			wantIntent:  localqa.IntentNetWorth,
			wantMatched: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := localqa.Match(tc.input)
			if ok != tc.wantMatched {
				t.Errorf("Match(%q) matched=%v, want %v", tc.input, ok, tc.wantMatched)
			}
			if got != tc.wantIntent {
				t.Errorf("Match(%q) intent=%v (%d), want %v (%d)",
					tc.input, got, got, tc.wantIntent, tc.wantIntent)
			}
		})
	}
}

func TestIntent_String(t *testing.T) {
	tests := []struct {
		intent localqa.Intent
		want   string
	}{
		{localqa.IntentNone, "None"},
		{localqa.IntentBalance, "Balance"},
		{localqa.IntentSpendingByCategory, "SpendingByCategory"},
		{localqa.IntentSafeToSpend, "SafeToSpend"},
		{localqa.IntentNetWorth, "NetWorth"},
		{localqa.IntentUpcomingBills, "UpcomingBills"},
		{localqa.IntentGoalProgress, "GoalProgress"},
		{localqa.IntentHealthScore, "HealthScore"},
		// Unknown value falls through to "None".
		{localqa.Intent(999), "None"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.intent.String(); got != tc.want {
				t.Errorf("Intent(%d).String() = %q, want %q", int(tc.intent), got, tc.want)
			}
		})
	}
}

func TestExtractCategory(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic groceries",
			input: "How much did I spend on groceries",
			want:  "groceries",
		},
		{
			name:  "category with trailing whitespace",
			input: "How much did I spend on dining out  ",
			want:  "dining out",
		},
		{
			name:  "capitalized category preserved",
			input: "How much have I spent on Groceries this month",
			want:  "Groceries this month",
		},
		{
			name:  "entertainment category",
			input: "My spending on entertainment",
			want:  "entertainment",
		},
		{
			name:  "multi-word category",
			input: "spent on coffee and snacks",
			want:  "coffee and snacks",
		},
		// Last "on" wins (LastIndex).
		{
			name:  "two on phrases picks last",
			input: "focus on the report on utilities",
			want:  "utilities",
		},
		// No "on" → empty.
		{
			name:  "no on keyword",
			input: "What is my balance?",
			want:  "",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "on at end of string no trailing word",
			input: "spending on",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := localqa.ExtractCategory(tc.input)
			if got != tc.want {
				t.Errorf("ExtractCategory(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
