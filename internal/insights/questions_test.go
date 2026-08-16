// SPDX-License-Identifier: MIT

package insights

import (
	"strings"
	"testing"
)

// TestSuggestedQuestions documents TestSuggestedQuestions.
func TestSuggestedQuestions(t *testing.T) {
	t.Run("generic when no data", func(t *testing.T) {
		got := SuggestedQuestions(QuestionContext{})
		if len(got) < 2 || len(got) > 4 {
			t.Fatalf("want 2-4 generic questions, got %d: %v", len(got), got)
		}
		if got[0] != "Where did our money go last month?" {
			t.Errorf("first generic question = %q", got[0])
		}
	})

	t.Run("tailored questions come first", func(t *testing.T) {
		got := SuggestedQuestions(QuestionContext{TopCategory: "Groceries", NearLimitBudget: "Dining", UpcomingGoal: "Vacation"})
		if len(got) != 4 {
			t.Fatalf("want exactly 4 (capped), got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "Groceries") {
			t.Errorf("top-category question should be first, got %q", got[0])
		}
		joined := strings.Join(got, " | ")
		if !strings.Contains(joined, "Dining") || !strings.Contains(joined, "Vacation") {
			t.Errorf("near-limit budget + goal should appear: %s", joined)
		}
	})

	t.Run("deterministic, de-duplicated, never empty", func(t *testing.T) {
		ctx := QuestionContext{TopCategory: "Food"}
		a := SuggestedQuestions(ctx)
		b := SuggestedQuestions(ctx)
		if strings.Join(a, "|") != strings.Join(b, "|") {
			t.Errorf("not deterministic: %v vs %v", a, b)
		}
		seen := map[string]bool{}
		for _, q := range a {
			if seen[q] {
				t.Errorf("duplicate question: %q", q)
			}
			seen[q] = true
		}
		if len(a) == 0 {
			t.Error("must never be empty")
		}
	})
}

func TestSituationalQuestionsLeadWhenSomethingIsTrueToday(t *testing.T) {
	// A chip that names something currently true is a prompt; a reasonable general
	// question is only a suggestion. The order is the feature.
	got := SuggestedQuestions(QuestionContext{
		TopCategory:     "Dining",
		NearLimitBudget: "Groceries",
		OverBudgetCount: 3,
		FlaggedCount:    2,
	})
	if len(got) == 0 || got[0] != "3 budgets are over — what changed?" {
		t.Fatalf("first chip = %q, want the over-budget prompt", got[0])
	}
	if got[1] != "2 things are flagged — are any of them a problem?" {
		t.Fatalf("second chip = %q", got[1])
	}
}

func TestSituationalQuestionsReadCorrectlyInTheSingular(t *testing.T) {
	got := SuggestedQuestions(QuestionContext{OverBudgetCount: 1, FlaggedCount: 1, UncategorizedCount: 1})
	want := []string{
		"One budget is over — what changed?",
		"Something's flagged — is it a problem?",
		"One transaction has no category — where does it belong?",
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("chip %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestTheLargestExpenseBecomesAQuestionAboutItself(t *testing.T) {
	got := SuggestedQuestions(QuestionContext{
		LargestExpensePayee:  "Costco",
		LargestExpenseAmount: "$312.40",
	})
	if got[0] != "What was the $312.40 at Costco?" {
		t.Fatalf("first chip = %q", got[0])
	}
}

func TestAHalfKnownLargestExpenseIsNotOffered(t *testing.T) {
	// A chip reading "What was the  at Costco?" is worse than no chip.
	for _, ctx := range []QuestionContext{
		{LargestExpensePayee: "Costco"},
		{LargestExpenseAmount: "$312.40"},
	} {
		for _, q := range SuggestedQuestions(ctx) {
			if strings.Contains(q, "What was the") {
				t.Fatalf("offered a half-formed chip: %q", q)
			}
		}
	}
}

func TestTheListIsStillCappedAndDeterministic(t *testing.T) {
	ctx := QuestionContext{
		TopCategory: "Dining", NearLimitBudget: "Groceries", UpcomingGoal: "Holiday",
		OverBudgetCount: 2, FlaggedCount: 3, UncategorizedCount: 5,
		LargestExpensePayee: "Costco", LargestExpenseAmount: "$312.40",
	}
	first := SuggestedQuestions(ctx)
	if len(first) != maxSuggestedQuestions {
		t.Fatalf("returned %d chips, want %d", len(first), maxSuggestedQuestions)
	}
	// The same data must always produce the same chips in the same order; a list
	// that reshuffles between renders reads as randomness, not insight.
	for i := 0; i < 5; i++ {
		again := SuggestedQuestions(ctx)
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("chip %d changed between calls: %q then %q", j, first[j], again[j])
			}
		}
	}
}

func TestThereAreAlwaysQuestionsToOffer(t *testing.T) {
	// A brand-new household with no data at all must still see chips; a blank box
	// with nothing to tap is the state this whole feature exists to avoid.
	got := SuggestedQuestions(QuestionContext{})
	if len(got) == 0 {
		t.Fatal("no questions offered for an empty dataset")
	}
	for _, q := range got {
		if strings.TrimSpace(q) == "" {
			t.Fatal("an empty chip was offered")
		}
	}
}
