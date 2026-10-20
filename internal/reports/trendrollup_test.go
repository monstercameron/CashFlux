// SPDX-License-Identifier: MIT

package reports

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRollUpCategorySeriesRejectsOverflowAndIncompatibleBuckets(t *testing.T) {
	cats := []domain.Category{{ID: "parent"}, {ID: "child", ParentID: "parent"}}
	if _, err := RollUpCategorySeries([]CategoryTrend{{CategoryID: "parent", Spend: []int64{math.MaxInt64}}, {CategoryID: "child", Spend: []int64{1}}}, cats); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("overflow = %v", err)
	}
	if _, err := RollUpCategorySeries([]CategoryTrend{{CategoryID: "parent", Spend: []int64{1}}, {CategoryID: "child", Spend: []int64{1, 2}}}, cats); err == nil {
		t.Fatal("accepted incompatible bucket counts")
	}
	if _, err := RollUpByParentChecked([]CategorySpend{{CategoryID: "parent", Amount: math.MaxInt64}, {CategoryID: "child", Amount: 1}}, cats); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("parent total overflow = %v", err)
	}
}
