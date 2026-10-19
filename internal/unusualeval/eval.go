// SPDX-License-Identifier: MIT

// Package unusualeval provides a deterministic, labelled evaluation harness for
// unusual-transaction detectors. Fixtures combine the sample household with
// isolated synthetic scenarios; this package does not change production rules.
package unusualeval

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
)

// Signal is one expected unusual-activity classification.
type Signal string

// Supported signal labels used by the UNU evaluation set.
const (
	Spike                  Signal = "Spike"
	PriceIncrease          Signal = "PriceIncrease"
	PossibleDoubleCharge   Signal = "PossibleDoubleCharge"
	FirstTimeMerchantLarge Signal = "FirstTimeMerchantLarge"
	None                   Signal = "None"
)

var signalOrder = []Signal{Spike, PriceIncrease, PossibleDoubleCharge, FirstTimeMerchantLarge, None}

// Fixture is a labelled candidate transaction and its complete detector input.
type Fixture struct {
	ID              string
	Candidate       string
	Expected        []Signal
	Evidence        []string
	HardNegativeFor []Signal
	Rationale       string
	Merchant        string
	Period          string
	Split           Split
	Input           smartengine.Input
}

// Split is the deterministic evaluation partition for a fixture.
type Split string

// Tuning and Holdout are stable fixture partitions derived from merchant and period.
const (
	Tuning  Split = "tuning"
	Holdout Split = "holdout"
)

// Finding is one detector prediction for a transaction.
type Finding struct {
	TxnID  string
	Signal Signal
}

// Detector evaluates one immutable transaction snapshot and returns its findings.
type Detector func(smartengine.Input) []Finding

// Example identifies one false-positive or false-negative fixture.
type Example struct {
	FixtureID string
	TxnID     string
	Rationale string
}

// Counts holds one-vs-rest classification totals for a signal.
type Counts struct {
	TP int
	FP int
	FN int
}

// Metrics contains classification counts and example errors for a signal.
type Metrics struct {
	Signal    Signal
	Counts    Counts
	Precision float64
	Recall    float64
	FP        []Example
	FN        []Example
}

// Result is the stable classification record for one fixture.
type Result struct {
	FixtureID string
	TxnID     string
	Expected  []Signal
	Predicted []Signal
	Rationale string
}

// Report is a deterministic evaluation summary.
type Report struct {
	Split   Split
	Overall Metrics
	Signals []Metrics
	Results []Result
}

// ValidSignal reports whether s is a supported fixture label.
func ValidSignal(s Signal) bool {
	for _, valid := range signalOrder {
		if s == valid {
			return true
		}
	}
	return false
}

// Partition assigns all fixtures from a merchant and period cohort to the same
// split. Its fixed hash makes the assignment independent of input ordering.
func Partition(merchant, period string) Split {
	var hash uint32 = 2166136261
	for _, b := range []byte(period + "\x00" + merchant) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	if hash%5 == 1 {
		return Holdout
	}
	return Tuning
}

// Evaluate runs detector over fixtures and reports stable per-signal and overall
// precision, recall, false-positive examples, and false-negative examples.
func Evaluate(fixtures []Fixture, split Split, detector Detector) Report {
	return evaluate(fixtures, split, func(f Fixture) []Finding { return detector(f.Input) })
}

func evaluate(fixtures []Fixture, split Split, detector func(Fixture) []Finding) Report {
	ordered := append([]Fixture(nil), fixtures...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	report := Report{Split: split}
	for _, signal := range signalOrder {
		report.Signals = append(report.Signals, Metrics{Signal: signal})
	}
	for _, fixture := range ordered {
		if FixtureSplit(fixture) != split {
			continue
		}
		predicted := make(map[Signal]bool)
		for _, finding := range detector(fixture) {
			if finding.TxnID == fixture.Candidate && ValidSignal(finding.Signal) {
				predicted[finding.Signal] = true
			}
		}
		if len(predicted) == 0 {
			predicted[None] = true
		}
		expected := make(map[Signal]bool, len(fixture.Expected))
		for _, signal := range fixture.Expected {
			expected[signal] = true
		}
		actualAnomaly := hasAnomaly(expected)
		predictedAnomaly := hasAnomaly(predicted)
		switch {
		case actualAnomaly && predictedAnomaly:
			report.Overall.Counts.TP++
		case predictedAnomaly:
			report.Overall.Counts.FP++
			report.Overall.FP = append(report.Overall.FP, exampleFor(fixture))
		case actualAnomaly:
			report.Overall.Counts.FN++
			report.Overall.FN = append(report.Overall.FN, exampleFor(fixture))
		}
		result := Result{FixtureID: fixture.ID, TxnID: fixture.Candidate, Expected: orderedSignals(expected), Predicted: orderedSignals(predicted), Rationale: fixture.Rationale}
		report.Results = append(report.Results, result)
		for i := range report.Signals {
			metric := &report.Signals[i]
			actual, got := expected[metric.Signal], predicted[metric.Signal]
			switch {
			case actual && got:
				metric.Counts.TP++
			case got:
				metric.Counts.FP++
				metric.FP = append(metric.FP, exampleFor(fixture))
			case actual:
				metric.Counts.FN++
				metric.FN = append(metric.FN, exampleFor(fixture))
			}
		}
	}
	for i := range report.Signals {
		finishMetrics(&report.Signals[i])
	}
	report.Overall.Signal = "Overall"
	finishMetrics(&report.Overall)
	return report
}

// FixtureSplit returns a checked-in stratified assignment when present, falling
// back to the stable merchant/period cohort hash for older fixtures.
func FixtureSplit(fixture Fixture) Split {
	if fixture.Split == Tuning || fixture.Split == Holdout {
		return fixture.Split
	}
	return Partition(fixture.Merchant, fixture.Period)
}

// CurrentT6 adapts the existing public Transactions-page engine to the harness.
// All other Transactions-page features are explicitly disabled for this run.
func CurrentT6(in smartengine.Input) []Finding {
	settings := t6OnlySettings()
	insights := smartengine.RunPage(in, settings, smart.PageTransactions)
	out := make([]Finding, 0, len(insights))
	for _, insight := range insights {
		if insight.Feature != "SMART-T6" {
			continue
		}
		const prefix = "SMART-T6:"
		if len(insight.Key) > len(prefix) && insight.Key[:len(prefix)] == prefix {
			out = append(out, Finding{TxnID: insight.Key[len(prefix):], Signal: Spike})
		}
	}
	return out
}

// MeasureLatency measures one run at each requested ledger size. It is a report
// utility only; callers should not turn the returned durations into timing gates.
func MeasureLatency(input smartengine.Input, sizes ...int) []Latency {
	out := make([]Latency, 0, len(sizes))
	for _, size := range sizes {
		if size < 0 {
			continue
		}
		in := latencyInput(input, size)
		_ = CurrentT6(in) // warm caches and code paths before the measured runs.
		const samples = 5
		const runsPerSample = 10
		durations := make([]time.Duration, 0, samples)
		for range samples {
			start := time.Now()
			for range runsPerSample {
				_ = CurrentT6(in)
			}
			durations = append(durations, time.Since(start)/time.Duration(runsPerSample))
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		out = append(out, Latency{Rows: len(in.Transactions), Elapsed: durations[len(durations)/2], Samples: samples * runsPerSample})
	}
	return out
}

// Latency records a single detector duration at a given row count.
type Latency struct {
	Rows    int
	Elapsed time.Duration
	Samples int
}

// ValidateFixture checks required fields, label validity, and candidate presence.
func ValidateFixture(fixture Fixture) error {
	if fixture.ID == "" || fixture.Candidate == "" || fixture.Merchant == "" || fixture.Period == "" {
		return fmt.Errorf("fixture %q: id, candidate, merchant, and period are required", fixture.ID)
	}
	if fixture.Rationale == "" {
		return fmt.Errorf("fixture %q: rationale is required", fixture.ID)
	}
	if len(fixture.Expected) == 0 {
		return fmt.Errorf("fixture %q: expected labels are required", fixture.ID)
	}
	for _, signal := range fixture.Expected {
		if !ValidSignal(signal) {
			return fmt.Errorf("fixture %q: invalid signal %q", fixture.ID, signal)
		}
	}
	for _, signal := range fixture.HardNegativeFor {
		if !ValidSignal(signal) || signal == None {
			return fmt.Errorf("fixture %q: invalid hard-negative target %q", fixture.ID, signal)
		}
	}
	if fixture.Split != "" && fixture.Split != Tuning && fixture.Split != Holdout {
		return fmt.Errorf("fixture %q: invalid split %q", fixture.ID, fixture.Split)
	}
	return nil
}

func exampleFor(f Fixture) Example {
	return Example{FixtureID: f.ID, TxnID: f.Candidate, Rationale: f.Rationale}
}

func orderedSignals(set map[Signal]bool) []Signal {
	var out []Signal
	for _, signal := range signalOrder {
		if set[signal] {
			out = append(out, signal)
		}
	}
	return out
}

func finishMetrics(m *Metrics) {
	if n := m.Counts.TP + m.Counts.FP; n > 0 {
		m.Precision = float64(m.Counts.TP) / float64(n)
	}
	if n := m.Counts.TP + m.Counts.FN; n > 0 {
		m.Recall = float64(m.Counts.TP) / float64(n)
	}
}

func hasAnomaly(signals map[Signal]bool) bool {
	for signal, present := range signals {
		if present && signal != None {
			return true
		}
	}
	return false
}

func t6OnlySettings() smart.Settings {
	settings := smart.Settings{}
	for _, feature := range smart.Catalog() {
		settings = settings.SetEnabled(feature.Code, feature.Code == "SMART-T6")
	}
	return settings
}

func latencyInput(input smartengine.Input, size int) smartengine.Input {
	input.Transactions = append([]domain.Transaction(nil), input.Transactions...)
	if size < len(input.Transactions) {
		input.Transactions = input.Transactions[:size]
		return input
	}
	for i := len(input.Transactions); i < size; i++ {
		input.Transactions = append(input.Transactions, domain.Transaction{
			ID: fmt.Sprintf("latency-%05d", i), Date: input.Now.AddDate(-3, 0, 0),
			Amount: money.New(-100, "USD"),
		})
	}
	return input
}
