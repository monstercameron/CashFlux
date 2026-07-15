package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetTargetJSONRoundTrip(t *testing.T) {
	orig := Budget{
		ID:           "b1",
		Name:         "Car insurance",
		Period:       PeriodMonthly,
		Limit:        money.New(20000, "USD"),
		TargetKind:   TargetByDate,
		TargetAmount: money.New(120000, "USD"),
		TargetDate:   time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC),
		LinkedGoalID: "g9",
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Budget
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TargetKind != orig.TargetKind {
		t.Errorf("TargetKind = %q, want %q", got.TargetKind, orig.TargetKind)
	}
	if got.TargetAmount != orig.TargetAmount {
		t.Errorf("TargetAmount = %+v, want %+v", got.TargetAmount, orig.TargetAmount)
	}
	if !got.TargetDate.Equal(orig.TargetDate) {
		t.Errorf("TargetDate = %v, want %v", got.TargetDate, orig.TargetDate)
	}
	if got.LinkedGoalID != orig.LinkedGoalID {
		t.Errorf("LinkedGoalID = %q, want %q", got.LinkedGoalID, orig.LinkedGoalID)
	}
	if !got.HasTarget() {
		t.Error("HasTarget() = false, want true")
	}
}

func TestBudgetNoTargetOmitsFields(t *testing.T) {
	data, err := json.Marshal(Budget{ID: "b1", Name: "Groceries"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"targetKind", "linkedGoalId"} {
		if contains(string(data), key) {
			t.Errorf("expected %q omitted from JSON, got %s", key, data)
		}
	}
	var got Budget
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.HasTarget() {
		t.Error("HasTarget() = true for a budget with no target")
	}
}

func TestTargetKindValid(t *testing.T) {
	for _, k := range append([]TargetKind{TargetNone}, AllTargetKinds...) {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if TargetKind("bogus").Valid() {
		t.Error("bogus kind should be invalid")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
