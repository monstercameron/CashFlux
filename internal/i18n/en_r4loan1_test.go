// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestR4LOAN1EnglishCopy(t *testing.T) {
	tests := map[string]string{
		"r4loan1.groupTitle":          "Possible loan payments",
		"r4loan1.suggestionMessage":   "Looks like a payment to %s — record as a transfer?",
		"r4loan1.suggestionOnlyTitle": "A few payments may be loans",
		"r4loan1.suggestionOnlySub":   "Choose whether to record each one as a transfer.",
		"r4loan1.record":              "Record as transfer",
		"r4loan1.dismiss":             "Not a loan payment",
		"r4loan1.transferCategory":    "Transfer → %s",
	}
	for key, want := range tests {
		if got := english[key]; got != want {
			t.Errorf("english[%q] = %q, want %q", key, got, want)
		}
	}
}
