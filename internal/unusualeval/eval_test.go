// SPDX-License-Identifier: MIT

package unusualeval

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestFixtureSetIsCompleteAndWellLabelled(t *testing.T) {
	fixtures, err := Fixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 48 {
		t.Fatalf("fixture count = %d, want at least 48", len(fixtures))
	}
	ids := make(map[string]bool, len(fixtures))
	candidates := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		if err := ValidateFixture(fixture); err != nil {
			t.Error(err)
		}
		if ids[fixture.ID] {
			t.Errorf("duplicate fixture id %q", fixture.ID)
		}
		ids[fixture.ID] = true
		if candidates[fixture.Candidate] {
			t.Errorf("candidate %q appears more than once", fixture.Candidate)
		}
		candidates[fixture.Candidate] = true
		if !containsTransaction(fixture, fixture.Candidate) {
			t.Errorf("fixture %q candidate %q is missing from its input", fixture.ID, fixture.Candidate)
		}
	}
	invalid := Fixture{ID: "invalid", Candidate: "candidate", Merchant: "merchant", Period: "period", Rationale: "reason", Expected: []Signal{"bad"}}
	if err := ValidateFixture(invalid); err == nil {
		t.Fatal("invalid signal label unexpectedly passed validation")
	}
	for _, split := range []Split{Tuning, Holdout} {
		count := 0
		for _, fixture := range fixtures {
			if FixtureSplit(fixture) == split {
				count++
			}
		}
		if count == 0 {
			t.Errorf("no fixtures assigned to %s", split)
		}
	}
	for _, split := range []Split{Tuning, Holdout} {
		for _, signal := range signalOrder[:len(signalOrder)-1] {
			positives, hardNegatives := 0, 0
			for _, fixture := range fixtures {
				if FixtureSplit(fixture) != split {
					continue
				}
				if containsSignal(fixture.Expected, signal) {
					positives++
				}
				if containsSignal(fixture.HardNegativeFor, signal) {
					hardNegatives++
				}
			}
			if positives < 4 || hardNegatives < 4 {
				t.Errorf("%s %s support = %d positives, %d hard negatives; want at least 4 each", split, signal, positives, hardNegatives)
			}
		}
	}
}

func TestSignalLabels(t *testing.T) {
	tests := []struct {
		label Signal
		valid bool
	}{
		{Spike, true},
		{PriceIncrease, true},
		{PossibleDoubleCharge, true},
		{FirstTimeMerchantLarge, true},
		{None, true},
		{"Unknown", false},
		{"", false},
	}
	for _, test := range tests {
		t.Run(string(test.label), func(t *testing.T) {
			if got := ValidSignal(test.label); got != test.valid {
				t.Errorf("ValidSignal(%q) = %t, want %t", test.label, got, test.valid)
			}
		})
	}
}

func TestExpectationEvidenceAndSampleGoalLink(t *testing.T) {
	fixtures, err := Fixtures()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Fixture, len(fixtures))
	for _, fixture := range fixtures {
		byID[fixture.ID] = fixture
	}
	if got := byID["babylist-june-unlinked"]; !containsSignal(got.Expected, Spike) || len(got.Evidence) != 0 {
		t.Errorf("unlinked June Babylist fixture = expected %v evidence %v; want Spike with no expectation evidence", got.Expected, got.Evidence)
	}
	crib := byID["sample-goal-funded-crib"]
	if !containsSignal(crib.Expected, None) || !containsSignalString(crib.Evidence, "goal:goal-baby") {
		t.Errorf("goal-funded crib fixture = expected %v evidence %v; want None with baby goal evidence", crib.Expected, crib.Evidence)
	}
	var linked bool
	for _, txn := range crib.Input.Transactions {
		if txn.ID == crib.Candidate {
			linked = txn.GoalID == "goal-baby"
		}
	}
	if !linked {
		t.Fatal("sample crib candidate no longer carries its goal-baby link")
	}
	for id, evidence := range map[string]string{
		"planned-large-purchase": "explicit_expected:fixture-planned-large",
		"seasonal-holiday":       "seasonal_cycles:2",
	} {
		fixture := byID[id]
		if !containsSignal(fixture.Expected, None) || !containsSignalString(fixture.Evidence, evidence) {
			t.Errorf("fixture %q = expected %v evidence %v; want None with %q", id, fixture.Expected, fixture.Evidence, evidence)
		}
	}
}

func TestMetricsAreDeterministicUnderPermutedInput(t *testing.T) {
	fixtures, err := Fixtures()
	if err != nil {
		t.Fatal(err)
	}
	forward := Evaluate(fixtures, Holdout, CurrentT6)
	forwardUNU2 := EvaluateUNU2(fixtures, Holdout)
	reverse := append([]Fixture(nil), fixtures...)
	for left, right := 0, len(reverse)-1; left < right; left, right = left+1, right-1 {
		reverse[left], reverse[right] = reverse[right], reverse[left]
	}
	for i := range reverse {
		transactions := reverse[i].Input.Transactions
		for left, right := 0, len(transactions)-1; left < right; left, right = left+1, right-1 {
			transactions[left], transactions[right] = transactions[right], transactions[left]
		}
	}
	backward := Evaluate(reverse, Holdout, CurrentT6)
	backwardUNU2 := EvaluateUNU2(reverse, Holdout)
	if !reflect.DeepEqual(forward, backward) {
		t.Fatalf("metrics changed when fixture input was permuted\nforward:  %+v\nbackward: %+v", forward, backward)
	}
	if !reflect.DeepEqual(forwardUNU2, backwardUNU2) {
		t.Fatalf("UNU2 metrics changed when fixture input was permuted\nforward:  %+v\nbackward: %+v", forwardUNU2, backwardUNU2)
	}
}

func TestT6BaselineAndLatencyReport(t *testing.T) {
	fixtures, err := Fixtures()
	if err != nil {
		t.Fatal(err)
	}
	printFixtureTable(t, fixtures)
	for _, split := range []Split{Tuning, Holdout} {
		report := Evaluate(fixtures, split, CurrentT6)
		printReport(t, report)
	}
	for _, latency := range MeasureLatency(fixtures[0].Input, 3292, 10000) {
		t.Logf("latency rows=%d median=%s (%d runs across 5 batches; report only)", latency.Rows, latency.Elapsed, latency.Samples)
	}
}

func TestUNU2EvaluationAndLatencyReport(t *testing.T) {
	fixtures, err := Fixtures()
	if err != nil {
		t.Fatal(err)
	}
	for _, split := range []Split{Tuning, Holdout} {
		old := Evaluate(fixtures, split, CurrentT6)
		newReport := EvaluateUNU2(fixtures, split)
		printComparison(t, old, newReport)
		if split == Holdout {
			if newReport.Overall.Precision < .70 || newReport.Overall.Recall < .60 {
				t.Errorf("UNU2 holdout overall P/R = %.3f/%.3f; want >= .70/.60", newReport.Overall.Precision, newReport.Overall.Recall)
			}
			for _, metric := range newReport.Signals[:len(newReport.Signals)-1] {
				if metric.Signal == PriceIncrease && (metric.Recall <= 0 || metric.Precision < .70) {
					t.Errorf("UNU2 holdout PriceIncrease P/R = %.3f/%.3f; want precision >= .70 and recall > 0", metric.Precision, metric.Recall)
				}
				if metric.Recall <= 0 && metric.Signal != PriceIncrease {
					t.Errorf("UNU2 holdout %s recall = %.3f; want > 0", metric.Signal, metric.Recall)
				}
			}
		}
	}
	for _, fixture := range fixtures[:1] {
		for _, size := range []int{3292, 10000} {
			input := latencyInput(fixture.Input, size)
			const runs = 50
			start := time.Now()
			for range runs {
				_ = UNU2(Fixture{Input: input, Candidate: "none"})
			}
			t.Logf("UNU2 LATENCY rows=%d elapsed=%s runs=%d", len(input.Transactions), time.Since(start)/runs, runs)
		}
	}
}

func printFixtureTable(t *testing.T, fixtures []Fixture) {
	t.Helper()
	ordered := append([]Fixture(nil), fixtures...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	t.Log("FIXTURES: id | expected | split | evidence | rationale")
	for _, fixture := range ordered {
		t.Logf("%s | %v | %s | %v | %s", fixture.ID, fixture.Expected, FixtureSplit(fixture), fixture.Evidence, fixture.Rationale)
	}
	for _, split := range []Split{Tuning, Holdout} {
		for _, signal := range signalOrder[:len(signalOrder)-1] {
			positive, hardNegative := 0, 0
			for _, fixture := range fixtures {
				if FixtureSplit(fixture) == split {
					if containsSignal(fixture.Expected, signal) {
						positive++
					}
					if containsSignal(fixture.HardNegativeFor, signal) {
						hardNegative++
					}
				}
			}
			t.Logf("SUPPORT split=%s signal=%s positive=%d hard-negative=%d", split, signal, positive, hardNegative)
		}
	}
}

func containsSignal(signals []Signal, want Signal) bool {
	for _, signal := range signals {
		if signal == want {
			return true
		}
	}
	return false
}

func containsSignalString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func printReport(t *testing.T, report Report) {
	t.Helper()
	t.Logf("BASELINE split=%s overall P=%.3f R=%.3f counts=%+v FP=%s FN=%s", report.Split, report.Overall.Precision, report.Overall.Recall, report.Overall.Counts, examples(report.Overall.FP), examples(report.Overall.FN))
	for _, metric := range report.Signals {
		t.Logf("SIGNAL split=%s name=%s P=%.3f R=%.3f counts=%+v FP=%s FN=%s", report.Split, metric.Signal, metric.Precision, metric.Recall, metric.Counts, examples(metric.FP), examples(metric.FN))
	}
	for _, result := range report.Results {
		t.Logf("RESULT split=%s fixture=%s txn=%s expected=%v predicted=%v rationale=%s", report.Split, result.FixtureID, result.TxnID, result.Expected, result.Predicted, result.Rationale)
	}
}

func printComparison(t *testing.T, t6, unu2 Report) {
	t.Helper()
	t.Logf("COMPARISON split=%s | T6 overall P=%.3f R=%.3f TP=%d FP=%d FN=%d | UNU2 overall P=%.3f R=%.3f TP=%d FP=%d FN=%d", t6.Split, t6.Overall.Precision, t6.Overall.Recall, t6.Overall.Counts.TP, t6.Overall.Counts.FP, t6.Overall.Counts.FN, unu2.Overall.Precision, unu2.Overall.Recall, unu2.Overall.Counts.TP, unu2.Overall.Counts.FP, unu2.Overall.Counts.FN)
	for i, metric := range unu2.Signals {
		old := t6.Signals[i]
		t.Logf("COMPARISON split=%s signal=%s | T6 P=%.3f R=%.3f TP=%d FP=%d FN=%d | UNU2 P=%.3f R=%.3f TP=%d FP=%d FN=%d", t6.Split, metric.Signal, old.Precision, old.Recall, old.Counts.TP, old.Counts.FP, old.Counts.FN, metric.Precision, metric.Recall, metric.Counts.TP, metric.Counts.FP, metric.Counts.FN)
		for _, namedMetric := range []struct {
			name   string
			metric Metrics
		}{{"T6", old}, {"UNU2", metric}} {
			for _, ex := range namedMetric.metric.FP {
				t.Logf("SIGNAL ERROR detector=%s split=%s signal=%s type=FP fixture=%s txn=%s why=%s", namedMetric.name, t6.Split, namedMetric.metric.Signal, ex.FixtureID, ex.TxnID, ex.Rationale)
			}
			for _, ex := range namedMetric.metric.FN {
				t.Logf("SIGNAL ERROR detector=%s split=%s signal=%s type=FN fixture=%s txn=%s why=%s", namedMetric.name, t6.Split, namedMetric.metric.Signal, ex.FixtureID, ex.TxnID, ex.Rationale)
			}
		}
	}
	for _, named := range []struct {
		name   string
		report Report
	}{{"T6", t6}, {"UNU2", unu2}} {
		report := named.report
		for _, ex := range report.Overall.FP {
			t.Logf("ERROR detector=%s split=%s type=FP fixture=%s txn=%s why=%s", named.name, report.Split, ex.FixtureID, ex.TxnID, ex.Rationale)
		}
		for _, ex := range report.Overall.FN {
			t.Logf("ERROR detector=%s split=%s type=FN fixture=%s txn=%s why=%s", named.name, report.Split, ex.FixtureID, ex.TxnID, ex.Rationale)
		}
	}
}

func examples(in []Example) string {
	ids := make([]string, 0, len(in))
	for _, example := range in {
		ids = append(ids, fmt.Sprintf("%s(%s)", example.FixtureID, example.TxnID))
	}
	return fmt.Sprint(ids)
}

func containsTransaction(fixture Fixture, id string) bool {
	for _, txn := range fixture.Input.Transactions {
		if txn.ID == id {
			return true
		}
	}
	return false
}
