// SPDX-License-Identifier: MIT

package savedreports

import (
	"fmt"
	"testing"
)

func TestAddReplacesSameName(t *testing.T) {
	list := Add(nil, Saved{ID: "1", Name: "June review", Res: "month"})
	list = Add(list, Saved{ID: "2", Name: "  june REVIEW ", Res: "quarter"})
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1 (same name replaces)", len(list))
	}
	if list[0].ID != "2" || list[0].Res != "quarter" {
		t.Errorf("kept %+v, want the newer entry", list[0])
	}
}

func TestAddCapsDroppingOldest(t *testing.T) {
	var list []Saved
	for i := 0; i < Max+3; i++ {
		list = Add(list, Saved{ID: fmt.Sprint(i), Name: fmt.Sprintf("view %d", i)})
	}
	if len(list) != Max {
		t.Fatalf("len = %d, want %d", len(list), Max)
	}
	if list[0].ID != "3" {
		t.Errorf("oldest survivor = %s, want 3", list[0].ID)
	}
}

func TestRemoveAndByID(t *testing.T) {
	list := Add(nil, Saved{ID: "a", Name: "A"})
	list = Add(list, Saved{ID: "b", Name: "B"})
	if _, ok := ByID(list, "a"); !ok {
		t.Error("ByID(a) not found")
	}
	list = Remove(list, "a")
	if len(list) != 1 || list[0].ID != "b" {
		t.Errorf("after remove: %+v", list)
	}
	if _, ok := ByID(list, "a"); ok {
		t.Error("removed entry still found")
	}
}
