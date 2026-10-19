package txntable

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"reflect"
	"testing"
	"time"
)

func TestFollowUps(t *testing.T) {
	day := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	tasks := []domain.Task{
		{ID: "done", RelatedType: domain.RelatedTransaction, RelatedID: "a", Status: domain.StatusDone, Due: day, Title: "Follow up: complete"},
		{ID: "undated", RelatedType: domain.RelatedTransaction, RelatedID: "a", Title: "Follow up:"},
		{ID: "later", RelatedType: domain.RelatedTransaction, RelatedID: "a", Due: day.AddDate(0, 0, 1), Title: " Later "},
		{ID: "earlier", RelatedType: domain.RelatedTransaction, RelatedID: "a", Due: day, Title: "Follow up: First"},
		{ID: "tie", RelatedType: domain.RelatedTransaction, RelatedID: "a", Due: day, Title: "Second"},
		{ID: "other", RelatedType: domain.RelatedTransaction, RelatedID: "b"}, {ID: "unlinked"}, {ID: "blank", RelatedType: domain.RelatedTransaction},
	}
	got := FollowUps(tasks, func(d time.Time) string { return d.Format("2006-01-02") })
	if len(got) != 2 || got["a"].Open != 4 || got["a"].Total != 5 {
		t.Fatal(got)
	}
	want := []string{"earlier", "tie", "later", "undated", "done"}
	ids := []string{}
	for _, item := range got["a"].Items {
		ids = append(ids, item.ID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatal(ids)
	}
	if got["a"].Items[0].Title != "First" || got["a"].Items[0].Due != "2026-02-28" || got["a"].Items[3].Title != "Follow up:" || got["a"].Items[3].Due != "" {
		t.Fatal(got["a"])
	}
	if tasks[0].ID != "done" || len(FollowUps(nil, nil)) != 0 {
		t.Fatal("input mutation or nil result")
	}
}
