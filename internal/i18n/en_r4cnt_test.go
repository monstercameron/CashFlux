// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestR4CNTEnglishCopy(t *testing.T) {
	bundle := DefaultBundle()
	for _, tc := range []struct {
		key  string
		args []any
		want string
	}{
		{key: "r4cnt.searchAllDates", args: []any{"319"}, want: "Search all dates (319)"},
		{key: "review.button", args: []any{247}, want: "Review 247"},
		{key: "fut2.futureSummary", args: []any{"November 2026", "0", "9"}, want: "November 2026 · 0 posted · 9 scheduled · "},
		{key: "fut2.dueSuffix", want: " due"},
	} {
		if got := bundle.T(English, tc.key, tc.args...); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
		}
	}
}
