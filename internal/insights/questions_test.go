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
