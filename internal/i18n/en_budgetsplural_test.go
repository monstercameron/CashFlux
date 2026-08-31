// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"strings"
	"testing"
)

// Count strings on /budgets are singular/plural pairs read through uistate.TN.
// Two things can go wrong that no other test here catches.
//
// First, the month-close pair is chosen by a helper (screens.tense) that swaps a
// live period's wording for a closed one, so TestScreensKeyCoverage — which scans
// for literal keys at call sites — cannot see four of these eight keys. A missing
// key renders as the raw key, which would put "monthclose.overIntroLiveOne" on a
// dialog where a sentence about money should be.
//
// Second, a pair whose two forms are IDENTICAL is worse than no pair at all: it
// costs a catalog entry, passes an existence check, and still prints "1 budgets".
// That is the defect this was written for, so the forms are compared, not just
// counted.
func TestBudgetsPluralPairs(t *testing.T) {
	pairs := [][2]string{
		{"budgets.savingsHintOne", "budgets.savingsHintMany"},
		{"budgets.recurring.foldHintOne", "budgets.recurring.foldHintMany"},
		{"budgets.coverMarkerOngoingTextOne", "budgets.coverMarkerOngoingTextMany"},
		{"track.summaryOne", "track.summaryMany"},
		{"monthclose.overIntroOne", "monthclose.overIntroMany"},
		{"monthclose.overIntroLiveOne", "monthclose.overIntroLiveMany"},
		{"monthclose.leftIntroOne", "monthclose.leftIntroMany"},
		{"monthclose.leftIntroLiveOne", "monthclose.leftIntroLiveMany"},
	}
	for _, pair := range pairs {
		one, many := pair[0], pair[1]
		oneText, okOne := english[one]
		manyText, okMany := english[many]
		if !okOne {
			t.Errorf("%s is missing from the English catalog", one)
		}
		if !okMany {
			t.Errorf("%s is missing from the English catalog", many)
		}
		if !okOne || !okMany {
			continue
		}
		if oneText == manyText {
			t.Errorf("%s and %s are the same string (%q) — the pair does nothing", one, many, oneText)
		}
		// TN prepends the count, so the singular form has to be able to consume an
		// int first. Verbs are counted rather than the rendering compared, because
		// several of these use explicit argument indexes to keep money first.
		if strings.Count(oneText, "%") != strings.Count(manyText, "%") {
			t.Errorf("%s and %s take different arguments: %q vs %q", one, many, oneText, manyText)
		}
	}
}

// The singular forms have to READ as singular. A pair can differ, take the same
// arguments, and still say "1 budgets are over" because only the noun was fixed
// and the verb was left plural. Rendering each singular with n=1 and looking for
// plural agreement is the only way to see that.
func TestBudgetsSingularFormsReadAsSingular(t *testing.T) {
	cases := []struct {
		key  string
		args []any
	}{
		{"budgets.savingsHintOne", []any{1, "$500.00"}},
		{"budgets.recurring.foldHintOne", []any{1, "$120.00"}},
		{"budgets.coverMarkerOngoingTextOne", []any{1, "$80.00"}},
		{"track.summaryOne", []any{1, "$300.00", "$300.00"}},
		{"monthclose.overIntroOne", []any{1, "$40.00"}},
		{"monthclose.overIntroLiveOne", []any{1, "$40.00"}},
		{"monthclose.leftIntroOne", []any{1, "$40.00"}},
		{"monthclose.leftIntroLiveOne", []any{1, "$40.00"}},
	}
	// Plural tails that would be wrong after the number 1.
	bad := []string{
		"1 budgets", "1 charges", "1 accounts", "1 categories", "1 others",
		"budgets are", "budgets went", "budgets is",
	}
	for _, c := range cases {
		text, ok := english[c.key]
		if !ok {
			t.Errorf("%s is missing", c.key)
			continue
		}
		got := fmt.Sprintf(text, c.args...)
		if strings.Contains(got, "%!") || strings.Contains(got, "EXTRA") {
			t.Errorf("%s does not accept (count, …): %q", c.key, got)
			continue
		}
		for _, b := range bad {
			if strings.Contains(got, b) {
				t.Errorf("%s reads plural at n=1: %q contains %q", c.key, got, b)
			}
		}
	}
}
