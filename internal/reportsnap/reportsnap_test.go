// SPDX-License-Identifier: MIT

package reportsnap

import (
	"fmt"
	"testing"
	"time"
)

func TestAddCapsAndOrder(t *testing.T) {
	var list []Snapshot
	for i := 0; i < Max+2; i++ {
		list = Add(list, Snapshot{ID: fmt.Sprint(i), TakenAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)})
	}
	if len(list) != Max {
		t.Fatalf("len = %d, want %d", len(list), Max)
	}
	if list[0].ID != "2" || list[len(list)-1].ID != fmt.Sprint(Max+1) {
		t.Errorf("order/cap wrong: first %s last %s", list[0].ID, list[len(list)-1].ID)
	}
}

func TestNetRemoveByIDTopN(t *testing.T) {
	s := Snapshot{ID: "a", Income: 500, Expense: 300}
	if s.Net() != 200 {
		t.Errorf("Net = %d, want 200", s.Net())
	}
	list := Add(nil, s)
	list = Add(list, Snapshot{ID: "b"})
	if _, ok := ByID(list, "a"); !ok {
		t.Error("ByID(a) missing")
	}
	list = Remove(list, "a")
	if len(list) != 1 || list[0].ID != "b" {
		t.Errorf("after remove: %+v", list)
	}
	lines := []LabelAmount{{"x", 1}, {"y", 2}, {"z", 3}}
	if got := TopN(lines, 2); len(got) != 2 || got[1].Label != "y" {
		t.Errorf("TopN = %+v", got)
	}
	if got := TopN(lines, 9); len(got) != 3 {
		t.Errorf("TopN over-length = %+v", got)
	}
}
