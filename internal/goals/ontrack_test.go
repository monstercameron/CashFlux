package goals

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestOnTrack(t *testing.T) {
	from := mustDate("2026-01-01")

	tests := []struct {
		name        string
		goal        domain.Goal
		monthly     int64
		wantOnTrack bool
		wantKnown   bool
	}{
		{
			"on pace: $100/mo clears $1200 by next Jan",
			domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")},
			10000, true, true,
		},
		{
			"behind: $50/mo can't clear $1200 by next Jan",
			domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")},
			5000, false, true,
		},
		{
			"complete goal is on track",
			domain.Goal{TargetAmount: usd(100000), CurrentAmount: usd(100000), TargetDate: mustDate("2027-01-01")},
			0, true, true,
		},
		{
			"no target date → not judgeable",
			domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0)},
			10000, false, false,
		},
		{
			"no contribution on an unmet dated goal → not judgeable",
			domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")},
			0, false, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			onTrack, known, err := OnTrack(tc.goal, usd(tc.monthly), from)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if onTrack != tc.wantOnTrack || known != tc.wantKnown {
				t.Errorf("OnTrack = (%v, %v), want (%v, %v)", onTrack, known, tc.wantOnTrack, tc.wantKnown)
			}
		})
	}
}

func TestEvaluatePaceFields(t *testing.T) {
	from := mustDate("2026-01-01")
	g := domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0), TargetDate: mustDate("2027-01-01")}

	// $100/mo is on pace.
	st, err := Evaluate(g, usd(10000), from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.PaceKnown || !st.OnTrack {
		t.Errorf("expected PaceKnown && OnTrack, got %+v", st)
	}

	// $50/mo is behind.
	st, err = Evaluate(g, usd(5000), from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.PaceKnown || st.OnTrack {
		t.Errorf("expected PaceKnown && !OnTrack, got %+v", st)
	}

	// No target date → pace not known.
	st, err = Evaluate(domain.Goal{TargetAmount: usd(120000), CurrentAmount: usd(0)}, usd(10000), from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.PaceKnown {
		t.Errorf("undated goal should have PaceKnown=false, got %+v", st)
	}
}
