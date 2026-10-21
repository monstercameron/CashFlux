// SPDX-License-Identifier: MIT
package notificationview

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

func feedIDs(items []uistate.FeedItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestSelect(t *testing.T) {
	items := []uistate.FeedItem{
		{ID: "snoozed", Title: "hidden", Severity: "critical", SnoozedUntil: 101},
		{ID: "first", Title: "shared", Severity: "warning", SnoozedUntil: 100},
		{ID: "duplicate", Title: "shared", Severity: "warning"},
		{ID: "a", Title: "mine", MemberID: "a", Severity: "critical"},
		{ID: "b", Title: "theirs", MemberID: "b", Severity: "warning"},
		{ID: "legacy", Title: "info"},
	}
	before := append([]uistate.FeedItem(nil), items...)
	for _, tc := range []struct {
		name string
		f    Filter
		want []string
	}{
		{"needs", Filter{}, []string{"first", "a", "b"}},
		{"member includes shared", Filter{MemberID: "a"}, []string{"first", "a"}},
		{"severity", Filter{Severity: "critical"}, []string{"a"}},
		{"watching legacy info", Filter{View: "watching", Severity: "info"}, []string{"legacy"}},
		{"combined", Filter{MemberID: "a", Severity: "warning"}, []string{"first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Select(items, 100, tc.f)
			if !reflect.DeepEqual(feedIDs(got), tc.want) {
				t.Fatalf("got %v want %v", feedIDs(got), tc.want)
			}
			if len(got) > 0 {
				got[0].Title = "changed"
			}
			if !reflect.DeepEqual(items, before) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestDedupePrecedesLensesAndSeparatesFields(t *testing.T) {
	items := []uistate.FeedItem{{ID: "a", Title: "same", Severity: "info"}, {ID: "b", Title: "same", Severity: "warning"}}
	if got := Select(items, 0, Filter{}); len(got) != 0 {
		t.Fatalf("must dedupe before triage: %v", got)
	}
	items = []uistate.FeedItem{{ID: "a", Title: "a\x00b", Body: "c", Severity: "warning"}, {ID: "b", Title: "a", Body: "b\x00c", Severity: "warning"}}
	if got := Select(items, 0, Filter{}); len(got) != 2 {
		t.Fatal("distinct title/body pairs collided")
	}
}

func TestCount(t *testing.T) {
	items := []uistate.FeedItem{{Severity: "critical", At: 11}, {Severity: "warning", At: 10, Read: true}, {At: 9}}
	if got := Count(items, 10); got != (Counts{Critical: 1, Warning: 1, Info: 1, Unread: 2, New: 1}) {
		t.Fatalf("counts %+v", got)
	}
}

func TestGroupItems(t *testing.T) {
	items := []uistate.FeedItem{{ID: "a@1", Severity: "warning"}, {ID: "b@1", Severity: "critical"}, {ID: "a@2", Severity: "warning"}, {ID: "legacy"}, {ID: "b@2", Severity: "warning"}, {ID: "a@3", Severity: "warning"}, {ID: "b@3", Severity: "warning"}}
	before := append([]uistate.FeedItem(nil), items...)
	got := GroupItems(items, 3)
	if len(got) != 5 || got[0].Kind != "a" || !reflect.DeepEqual(feedIDs(got[0].Items), []string{"a@1", "a@2", "a@3"}) {
		t.Fatalf("groups %+v", got)
	}
	if got[0].Key() != "group:a" || got[1].Key() != "item:b@1" {
		t.Fatal("unstable group identity")
	}
	for _, g := range got[1:] {
		if g.Kind != "" {
			t.Fatal("critical kind must stay individual")
		}
	}
	got[0].Items[0].Title = "changed"
	if !reflect.DeepEqual(items, before) {
		t.Fatal("input mutated")
	}
	if len(GroupItems(items, 4)) != len(items) || len(GroupItems(items, 0)) != len(items) {
		t.Fatal("threshold/disabled grouping")
	}
}

func BenchmarkSelectAndGroup(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			items := make([]uistate.FeedItem, n)
			for i := range items {
				items[i] = uistate.FeedItem{ID: fmt.Sprintf("rule-%d@%d", i%10, i), Title: fmt.Sprint(i), Severity: "warning", At: int64(i)}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				visible := Select(items, 100, Filter{})
				uistate.SortForAttention(visible, 100)
				_ = GroupItems(visible, 3)
			}
		})
	}
}
