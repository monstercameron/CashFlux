package store

import (
	"testing"
	"time"
)

func TestPayoffBaselineRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	s := Settings{BaseCurrency: "USD", PayoffBaseline: &PayoffBaseline{
		TotalOwed: 1234500, Currency: "USD", StartedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}}
	if err := st.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}

	ds, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	b := imported.Settings.PayoffBaseline
	if b == nil {
		t.Fatal("payoff baseline lost in round-trip")
	}
	if b.TotalOwed != 1234500 || b.Currency != "USD" || !b.StartedAt.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("baseline after round-trip = %+v", b)
	}
}
