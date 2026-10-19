// SPDX-License-Identifier: MIT

package unusualeval

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/unusual"
)

// UNU2 adapts the detector's evidence-rich findings to the evaluation harness.
func UNU2(fixture Fixture) []Finding {
	in := unusual.Input{
		Transactions: fixture.Input.Transactions,
		Categories:   fixture.Input.Categories,
		Accounts:     fixture.Input.Accounts,
		Recurring:    fixture.Input.Recurring,
		Goals:        fixture.Input.Goals,
		Budgets:      fixture.Input.Budgets,
		Now:          fixture.Input.Now,
		Rates:        fixture.Input.Rates,
	}
	for _, evidence := range fixture.Evidence {
		if strings.HasPrefix(evidence, "explicit_expected:") {
			in.ExpectedTxnIDs = map[string][]string{fixture.Candidate: {evidence}}
		}
	}
	findings := unusual.Detect(in)
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
