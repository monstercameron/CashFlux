// SPDX-License-Identifier: MIT

package pricecreep

import "testing"

func TestPreview(t *testing.T) {
	// Entertainment: $200 limit, $196 spent, price up $10/mo → 98% then 103%.
	imp := Preview("Entertainment", "USD", 200_00, 196_00, 10_00, true)
	if !imp.HasBudget {
		t.Fatalf("should have budget")
	}
	if imp.BeforePct != 98 {
		t.Fatalf("before = %d, want 98", imp.BeforePct)
	}
	if imp.AfterPct != 103 {
		t.Fatalf("after = %d, want 103", imp.AfterPct)
	}
	if imp.SuggestedLimitMinor != 210_00 {
		t.Fatalf("suggested limit = %d, want 21000", imp.SuggestedLimitMinor)
	}
}

func TestPreviewNoBudget(t *testing.T) {
	imp := Preview("", "USD", 0, 0, 10_00, false)
	if imp.HasBudget || imp.BeforePct != 0 || imp.AfterPct != 0 {
		t.Fatalf("no-budget preview should be zeroed: %+v", imp)
	}
}

func TestPreviewZeroLimit(t *testing.T) {
	imp := Preview("X", "USD", 0, 500, 100, true)
	if imp.HasBudget {
		t.Fatalf("zero limit means no usable budget")
	}
}
