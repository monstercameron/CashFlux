// SPDX-License-Identifier: MIT

package reports

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// AccountPerformance is one investment account's return: what was put in (cost
// basis) versus what it's worth now, and the gain between — all derived from the
// account's own transactions and its updated value, with NO live market prices
// (so it works entirely on-device / offline). Amounts are base-currency minor units.
type AccountPerformance struct {
	AccountID string
	Name      string
	// Invested is the cost basis: opening balance plus net transfers in (money you
	// moved into the account from elsewhere).
	Invested int64
	// Current is the account's current value (opening balance plus every transaction —
	// contributions, dividends, and value updates alike).
	Current int64
	// Gain is Current − Invested: the growth (or loss) beyond what you put in.
	Gain int64
	// ReturnBips is Gain / Invested in basis points (700 = +7.00%); 0 when there's no
	// positive cost basis to measure against.
	ReturnBips int
}

// isInvestmentType reports whether an account's value is an investment position
// (brokerage, retirement, or crypto) — the accounts a performance return applies to.
func isInvestmentType(t domain.AccountType) bool {
	return t == domain.TypeInvestment || t == domain.TypeRetirement || t == domain.TypeCrypto
}

// InvestmentPerformance computes per-account performance for every non-archived
// investment / retirement / crypto account. For each: Invested = opening balance +
// net transfers in; Current = opening balance + all transactions; Gain = the
// difference (equivalently, the sum of the account's non-transfer transactions —
// dividends, realized gains, and value updates). Return is Gain / Invested. All
// amounts convert to the base currency. Accounts are returned in the input order.
func InvestmentPerformance(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) ([]AccountPerformance, error) {
	byAcct := map[string][]domain.Transaction{}
	for _, t := range txns {
		byAcct[t.AccountID] = append(byAcct[t.AccountID], t)
	}
	var out []AccountPerformance
	for _, a := range accounts {
		if a.Archived || !isInvestmentType(a.Type) {
			continue
		}
		opening, err := convertToBase(a.OpeningBalance, rates)
		if err != nil {
			return nil, err
		}
		var allTxns, transfersIn int64
		for _, t := range byAcct[a.ID] {
			conv, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				return nil, err
			}
			allTxns += conv.Amount
			if t.IsTransfer() {
				transfersIn += conv.Amount
			}
		}
		invested := opening + transfersIn
		current := opening + allTxns
		gain := current - invested
		bips := 0
		if invested > 0 {
			bips = int(gain * 10000 / invested)
		}
		out = append(out, AccountPerformance{
			AccountID: a.ID, Name: a.Name,
			Invested: invested, Current: current, Gain: gain, ReturnBips: bips,
		})
	}
	return out, nil
}

// InvestmentPerformanceCSV renders the performance rows as CSV (account, invested,
// current value, gain, return %) — the export the Reports section offers, so the
// figures can go into a spreadsheet or a tax worksheet. amount formats a minor-unit
// value the same way the on-screen figures are formatted.
func InvestmentPerformanceCSV(perf []AccountPerformance, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Account", "Invested", "Current value", "Gain", "Return %"})
	for _, p := range perf {
		ret := strconv.FormatFloat(float64(p.ReturnBips)/100, 'f', 2, 64)
		_ = w.Write([]string{p.Name, amount(p.Invested), amount(p.Current), amount(p.Gain), ret})
	}
	w.Flush()
	return buf.Bytes()
}

// convertToBase converts a money value to the base currency, treating an empty
// currency code as already-base (the seed's opening balances sometimes omit it).
func convertToBase(m money.Money, rates currency.Rates) (int64, error) {
	if m.Currency == "" || m.Currency == rates.Base {
		return m.Amount, nil
	}
	conv, err := rates.Convert(m, rates.Base)
	if err != nil {
		return 0, err
	}
	return conv.Amount, nil
}
