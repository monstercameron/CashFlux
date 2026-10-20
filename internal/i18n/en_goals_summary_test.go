// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestGoalSummaryExclusionsFormatCurrenciesAndFallback(t *testing.T) {
	bundle := DefaultBundle()
	for _, lang := range []Lang{English, "untranslated"} {
		for _, currencies := range []string{"GBP", "EUR, GBP"} {
			got := bundle.T(lang, "goals.summaryExcludedCurrencies", currencies)
			want := "Some goal amounts are excluded from these totals because they could not be converted or added safely: " + currencies + "."
			if got != want {
				t.Errorf("summary exclusions in %s = %q, want %q", lang, got, want)
			}
		}
	}
}
