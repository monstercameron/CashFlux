// SPDX-License-Identifier: MIT

package auditimpact

import (
	"reflect"
	"testing"
)

func TestRecalculated(t *testing.T) {
	cases := []struct {
		name       string
		entityType string
		action     string
		fields     []string
		want       []string
	}{
		{
			name:       "transaction add recalculates the full money chain",
			entityType: "transaction", action: "added", fields: nil,
			want: []string{"account balance", "net worth", "budget progress",
				"income & spending reports", "safe to spend", "health score",
				"category rollups", "subscription detection"},
		},
		{
			name:       "transaction category-only edit is a re-bucket, not a balance change",
			entityType: "transaction", action: "updated", fields: []string{"categoryId"},
			want: []string{"budget progress", "category rollups", "income & spending reports"},
		},
		{
			name:       "transaction amount edit moves balances and budgets",
			entityType: "transaction", action: "updated", fields: []string{"amount"},
			want: []string{"account balance", "net worth", "budget progress",
				"income & spending reports", "safe to spend", "health score"},
		},
		{
			name:       "transaction tag edit touches only reports",
			entityType: "transaction", action: "updated", fields: []string{"tags"},
			want: []string{"income & spending reports"},
		},
		{
			name:       "report exclusion re-buckets totals without moving balances",
			entityType: "transaction", action: "updated", fields: []string{"excludeFromReports"},
			want: []string{"income & spending reports", "budget progress", "safe to spend"},
		},
		{
			name:       "account credit-limit edit recomputes utilization",
			entityType: "account", action: "updated", fields: []string{"creditLimit"},
			want: []string{"net worth", "safe to spend", "health score", "credit utilization"},
		},
		{
			name:       "budget change moves adherence",
			entityType: "budget", action: "updated", fields: []string{"amount"},
			want: []string{"budget progress", "health score", "safe to spend"},
		},
		{
			name:       "rule change affects only the future",
			entityType: "rule", action: "updated", fields: []string{"setCategoryId"},
			want: []string{"future auto-categorization"},
		},
		{
			name:       "FX settings change reprices everything",
			entityType: "settings", action: "updated", fields: []string{"fxRates"},
			want: []string{"every converted total"},
		},
		{
			name:       "non-FX settings change names nothing",
			entityType: "settings", action: "updated", fields: []string{"theme"},
			want: nil,
		},
		{
			name:       "broad settings change names nothing (generic KV writes reprice nothing)",
			entityType: "settings", action: "updated", fields: nil,
			want: nil,
		},
		{
			name:       "unknown entity names nothing",
			entityType: "widget", action: "updated", fields: []string{"x"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Recalculated(tc.entityType, tc.action, tc.fields)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Recalculated(%q, %q, %v) = %v, want %v",
					tc.entityType, tc.action, tc.fields, got, tc.want)
			}
		})
	}
}

func TestRecalculatedDeduplicates(t *testing.T) {
	// amount + categoryId both imply budget progress and reports — each figure
	// must appear once.
	got := Recalculated("transaction", "updated", []string{"amount", "categoryId"})
	seen := map[string]int{}
	for _, g := range got {
		seen[g]++
		if seen[g] > 1 {
			t.Fatalf("figure %q appears twice in %v", g, got)
		}
	}
}
