// SPDX-License-Identifier: MIT

package unusualeval

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/smartengine"
)

// UNU2 adapts the detector's evidence-rich findings to the evaluation harness.
func UNU2(fixture Fixture) []Finding {
	in := fixture.Input
	var expected map[string][]string
	for _, evidence := range fixture.Evidence {
		if strings.HasPrefix(evidence, "explicit_expected:") {
			expected = map[string][]string{fixture.Candidate: {evidence}}
		}
	}
	findings := smartengine.DetectUnusualFindings(in, expected)
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, Finding{TxnID: finding.TxnID, Signal: Signal(finding.Signal)})
	}
	return out
}

// EvaluateUNU2 evaluates fixtures against the UNU2 detector.
func EvaluateUNU2(fixtures []Fixture, split Split) Report {
	return evaluate(fixtures, split, UNU2)
}
