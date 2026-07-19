// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// TestG1AffordabilityUsesFairShareNotTotalSlack locks the fix for the July 19 trust
// defect: SMART-G1 must frame a goal's affordability against its FAIR SHARE of the
// slack (the model SMART-G18 and the Goals card use), never against the whole slack —
// otherwise the app says "$838 fits within your $2,857 slack" here while the
// feasibility note says "only $408 is realistically free", a flat contradiction.
func TestG1AffordabilityUsesFairShareNotTotalSlack(t *testing.T) {
	in := baseInput().withBaseline(400000, 300000) // ~$1000/mo surplus
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{
		goal("a", "Baby fund", 480000, 0, due), // needs ~$800/mo — more than its ~$500 fair share
		goal("b", "Small", 60000, 0, due),      // needs ~$100/mo — within its fair share
	}
	byKey := map[string]smart.Insight{}
	for _, ins := range g1SuggestedContribution(in) {
		switch {
		case strings.HasSuffix(ins.Key, ":a"):
			byKey["a"] = ins
		case strings.HasSuffix(ins.Key, ":b"):
			byKey["b"] = ins
		}
	}
	a, ok := byKey["a"]
	if !ok {
		t.Fatalf("no G1 insight for goal a")
	}
	if strings.Contains(a.Detail, "fits within") {
		t.Errorf("G1 tells an over-fair-share goal it 'fits within' the slack (contradicts G18): %q", a.Detail)
	}
	if !strings.Contains(a.Detail, "fair share") {
		t.Errorf("G1 should frame an over-fair-share goal against its fair share: %q", a.Detail)
	}
	if b, ok := byKey["b"]; ok && !strings.Contains(b.Detail, "fits within") {
		t.Errorf("G1 should say a within-share goal fits: %q", b.Detail)
	}
	// SMART-G18 must agree that goal a's deadline is tight — same fair-share model.
	var g18Tight bool
	for _, ins := range g18Feasibility(in) {
		if strings.HasSuffix(ins.Key, ":a") {
			g18Tight = true
		}
	}
	if !g18Tight {
		t.Errorf("SMART-G18 should flag goal a as tight, agreeing with G1")
	}
}

func goal(id, name string, target, current int64, due time.Time) domain.Goal {
	return domain.Goal{ID: id, Name: name, TargetAmount: usd(target), CurrentAmount: usd(current), TargetDate: due}
}

// withIncome adds trailing-month income/expense so surplus engines have a baseline.
func (in Input) withBaseline(incomePerMo, expensePerMo int64) Input {
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	var txns []domain.Transaction
	for k := 1; k <= 3; k++ {
		d := monthStart.AddDate(0, -k, 9)
		txns = append(txns,
			domain.Transaction{ID: "inc" + itoa64(int64(k)), AccountID: "x", Date: d, Amount: usd(incomePerMo), Desc: "Pay"},
			domain.Transaction{ID: "exp" + itoa64(int64(k)), AccountID: "x", Date: d, Amount: usd(-expensePerMo), Desc: "Rent"},
		)
	}
	in.Transactions = append(in.Transactions, txns...)
	return in
}

func TestG1SuggestedContribution(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000) // $2000/mo surplus
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{goal("g", "New Laptop", 120000, 0, due)} // $1200 over 6 months
	got := g1SuggestedContribution(in)
	if len(got) != 1 {
		t.Fatalf("want 1 suggestion, got %d: %+v", len(got), got)
	}
	if !got[0].HasAmount || got[0].Amount.Amount <= 0 {
		t.Errorf("expected a monthly amount, got %+v", got[0].Amount)
	}
}

func TestG1SkipsNoDeadline(t *testing.T) {
	in := baseInput()
	in.Goals = []domain.Goal{goal("g", "Someday", 120000, 0, time.Time{})}
	if got := g1SuggestedContribution(in); len(got) != 0 {
		t.Errorf("no deadline — want 0, got %d", len(got))
	}
}

func TestG5GoalConflict(t *testing.T) {
	in := baseInput().withBaseline(400000, 360000) // $400/mo surplus
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{
		goal("a", "Car", 300000, 0, due), // each needs ~$1000/mo over 3 months
		goal("b", "Trip", 300000, 0, due),
	}
	got := g5GoalConflict(in)
	if len(got) != 1 {
		t.Fatalf("want 1 conflict, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected positive shortfall, got %+v", got[0].Amount)
	}
}

func TestG5NoConflictWhenAffordable(t *testing.T) {
	in := baseInput().withBaseline(800000, 100000) // $7000/mo surplus
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{
		goal("a", "Car", 120000, 0, due),
		goal("b", "Trip", 60000, 0, due),
	}
	if got := g5GoalConflict(in); len(got) != 0 {
		t.Errorf("affordable — want 0, got %d: %+v", len(got), got)
	}
}

func TestG6Milestone(t *testing.T) {
	in := baseInput()
	in.Goals = []domain.Goal{
		goal("done", "Vacation", 100000, 100000, time.Time{}), // complete
		goal("near", "Camera", 100000, 80000, time.Time{}),    // 80%
		goal("early", "House", 100000, 10000, time.Time{}),    // 10% → nothing
	}
	got := g6MilestoneNudge(in)
	if len(got) != 2 {
		t.Fatalf("want 2 (done + near), got %d: %+v", len(got), got)
	}
	var sawDone, sawNear bool
	for _, i := range got {
		if i.Key == "SMART-G6:done:done" {
			sawDone = true
		}
		if i.Key == "SMART-G6:near:near" {
			sawNear = true
		}
	}
	if !sawDone || !sawNear {
		t.Errorf("expected done + near milestones, got %+v", got)
	}
}

func TestG11EmergencyFund(t *testing.T) {
	in := baseInput().withBaseline(0, 200000)                                            // $2000/mo essentials
	in.Goals = []domain.Goal{goal("ef", "Emergency Fund", 1200000, 400000, time.Time{})} // $4000 saved
	got := g11EmergencyFund(in)
	if len(got) != 1 {
		t.Fatalf("want 1 emergency insight, got %d: %+v", len(got), got)
	}
	// 2 months covered of a 6-month target → gap = $12000 - $4000 = $8000.
	if got[0].Amount.Amount != 800000 {
		t.Errorf("gap = %d, want 800000", got[0].Amount.Amount)
	}
	if got[0].Severity != smart.SeverityNudge {
		t.Errorf("expected nudge severity")
	}
}

func TestG11AdequateFundNoInsight(t *testing.T) {
	in := baseInput().withBaseline(0, 200000)                                             // $2000/mo essentials
	in.Goals = []domain.Goal{goal("ef", "Emergency Fund", 1200000, 1300000, time.Time{})} // > 6 months
	if got := g11EmergencyFund(in); len(got) != 0 {
		t.Errorf("adequate fund — want 0, got %d: %+v", len(got), got)
	}
}

func TestG11NoEmergencyGoal(t *testing.T) {
	in := baseInput().withBaseline(0, 200000)
	in.Goals = []domain.Goal{goal("v", "Vacation", 100000, 10000, time.Time{})}
	if got := g11EmergencyFund(in); len(got) != 0 {
		t.Errorf("no emergency goal — want 0, got %d", len(got))
	}
}
