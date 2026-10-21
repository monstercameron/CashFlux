// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestBC6N8IncomeAllocationReportsOverflow(t *testing.T) {
	view := budgetView{BannerIncome: math.MaxInt64, RolledOver: 1}
	if _, err := resolveAllocationChecked(view); err == nil {
		t.Fatal("expected income-pool overflow")
	}
	if got := resolveAllocation(view); got.Pool != 0 || got.PlanPct != 0 {
		t.Fatalf("unchecked fallback leaked wrapped values: %+v", got)
	}
	f := render.New(t)
	f.Render(budgetIncomeAllocation(view, nil, false, false))
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "We could not calculate this income split." {
		t.Fatalf("income allocation overflow message = %q", got)
	}
}
