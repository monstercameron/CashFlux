// SPDX-License-Identifier: MIT

package modules

import "testing"

func TestIsLocked(t *testing.T) {
	if !IsLocked("/") {
		t.Error("home (dashboard) must be locked")
	}
	if IsLocked("/goals") || IsLocked("/settings") {
		t.Error("only the dashboard is locked now")
	}
}

func TestIsHidden(t *testing.T) {
	h := Hidden{"/goals": true, "/": true}
	if !h.IsHidden("/goals") {
		t.Error("goals should be hidden")
	}
	if h.IsHidden("/") {
		t.Error("the dashboard is locked and must never report hidden")
	}
	if h.IsHidden("/accounts") {
		t.Error("accounts not in set should be visible")
	}
}

func TestToggle(t *testing.T) {
	h := Hidden{}
	h = h.Toggle("/goals")
	if !h.IsHidden("/goals") {
		t.Error("toggle should hide goals")
	}
	h = h.Toggle("/goals")
	if h.IsHidden("/goals") {
		t.Error("toggle again should show goals")
	}
	if len(h) != 0 {
		t.Errorf("re-shown path should be removed from set, got %v", h)
	}
}

func TestToggleLockedIsNoOp(t *testing.T) {
	h := Hidden{}.Toggle("/")
	if len(h) != 0 || h.IsHidden("/") {
		t.Errorf("toggling a locked path must not hide it, got %v", h)
	}
}

func TestToggleDoesNotMutateOriginal(t *testing.T) {
	h := Hidden{}
	_ = h.Toggle("/goals")
	if len(h) != 0 {
		t.Errorf("Toggle must return a new set, original was mutated: %v", h)
	}
}

func TestNormalize(t *testing.T) {
	h := Hidden{"/goals": true, "/budgets": false, "/": true}
	got := h.Normalize()
	if !got.IsHidden("/goals") {
		t.Error("goals should survive normalize")
	}
	if _, ok := got["/budgets"]; ok {
		t.Error("false entry should be dropped")
	}
	if _, ok := got["/"]; ok {
		t.Error("locked entry should be dropped")
	}
}
