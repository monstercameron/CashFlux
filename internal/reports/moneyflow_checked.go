// SPDX-License-Identifier: MIT

package reports

import "github.com/monstercameron/CashFlux/internal/money"

// BuildMoneyFlowChecked retains the canonical diagram's layout/grouping while
// refusing unrepresentable input magnitudes, totals or combined hub flows.
func BuildMoneyFlowChecked(in MoneyFlowInputs) (MoneyFlowDiagram, error) {
	for _, rows := range [][]CategorySpend{in.Income, in.Spending} {
		var total int64
		for _, row := range rows {
			magnitude, err := money.Abs(row.Amount)
			if err != nil {
				return MoneyFlowDiagram{}, err
			}
			total, err = money.Add(total, magnitude)
			if err != nil {
				return MoneyFlowDiagram{}, err
			}
		}
	}
	if _, err := money.Abs(in.Net); err != nil {
		return MoneyFlowDiagram{}, err
	}
	result := BuildMoneyFlow(in)
	// BuildMoneyFlow cannot panic for the validated magnitudes. A split income
	// source may differ from period income; validate both hub directions rather
	// than assuming the net ribbon necessarily balances those attributed rows.
	var incoming, outgoing int64
	for _, edge := range result.Edges {
		var err error
		switch edge.Kind {
		case FlowIncomeSource, FlowOtherIncome, FlowFromSavings:
			incoming, err = money.Add(incoming, edge.Value)
		default:
			outgoing, err = money.Add(outgoing, edge.Value)
		}
		if err != nil {
			return MoneyFlowDiagram{}, err
		}
	}
	return result, nil
}
