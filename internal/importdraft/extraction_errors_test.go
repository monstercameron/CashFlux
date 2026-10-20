// SPDX-License-Identifier: MIT

package importdraft

import "testing"

func TestExtractionErrors(t *testing.T) {
	for _, tc := range []struct {
		issues []string
		want   string
	}{
		{nil, "imb.invalidRow"},
		{[]string{"untrusted text"}, "imb.invalidRow"},
		{[]string{"Date is missing or invalid.", "Amount is missing or invalid."}, "importux.invalidDate importux.invalidAmount"},
		{[]string{"Date is outside the supported range."}, "importux.dateRangeError"},
		{[]string{"Currency is missing."}, "importux.missingCurrency"},
		{[]string{"Currency is unknown."}, "importux.unknownCurrency"},
		{[]string{"Amount is outside the supported range."}, "importux.amountRangeError"},
		{[]string{"Account does not match an available account."}, "importux.unknownAccount"},
	} {
		if got := ExtractionErrors(tc.issues, func(key string, _ ...any) string { return key }); got != tc.want {
			t.Errorf("ExtractionErrors(%v) = %q, want %q", tc.issues, got, tc.want)
		}
	}
}
