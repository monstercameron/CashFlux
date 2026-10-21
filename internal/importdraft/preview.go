// SPDX-License-Identifier: MIT

package importdraft

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importsafe"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
)

// Preview contains the checked account impact and staged CSV diagnostics.
type Preview struct {
	Account            domain.Account
	Decimals           int
	Before, Net, After int64
	HasBalance, Jump   bool
	Failures           []store.CSVRowError
	Pairs              []importsafe.Pair
}

// ApprovedTransactions selects the exact transactions the staged batch may commit.
func ApprovedTransactions(batch staging.Batch) []domain.Transaction {
	var txns []domain.Transaction
	for _, row := range batch.Rows {
		if row.Approved() {
			txns = append(txns, row.Transaction)
		}
	}
	return txns
}

// BatchPreview calculates impact in the selected account's currency only. Missing
// accounts, currency mismatches and arithmetic failures suppress balance claims.
func BatchPreview(batch staging.Batch, accounts []domain.Account, txns []domain.Transaction, accountID string) Preview {
	approved := ApprovedTransactions(batch)
	p := Preview{Pairs: importsafe.TransferPairs(approved, txns, accountID, 3)}
	for _, row := range batch.Rows {
		if row.Verdict == staging.Unusable {
			p.Failures = append(p.Failures, store.CSVRowError{Line: row.Line, Reason: row.Reason})
		}
	}
	account, ok := domain.AccountByID(accounts, accountID)
	if !ok {
		return p
	}
	p.Account, p.Decimals = account, currency.Decimals(account.Currency)
	balance, err := ledger.Balance(account, txns)
	if err != nil {
		return p
	}
	p.Before = balance.Amount
	p.Net, p.After, err = importsafe.Impact(balance.Amount, approved, accountID, account.Currency)
	p.HasBalance = err == nil
	p.Jump = err == nil && importsafe.JumpWarning(balance.Amount, p.Net)
	return p
}

// RawPreview preserves the legacy raw-row preview's invalid-amount skipping while
// checking every addition. No cross-currency aggregation or implicit FX occurs.
func RawPreview(accounts []domain.Account, txns []domain.Transaction, accountID string, rows []extract.Row) Preview {
	var p Preview
	account, ok := domain.AccountByID(accounts, accountID)
	if !ok {
		return p
	}
	p.Account, p.Decimals = account, currency.Decimals(account.Currency)
	for _, row := range rows {
		amount, err := money.ParseMinor(strings.TrimSpace(row.Amount), p.Decimals)
		if err != nil {
			continue
		}
		p.Net, err = money.Add(p.Net, amount)
		if err != nil {
			return p
		}
	}
	balance, err := ledger.Balance(account, txns)
	if err != nil {
		return p
	}
	p.Before = balance.Amount
	p.After, err = money.Add(p.Before, p.Net)
	p.HasBalance = err == nil
	p.Jump = err == nil && importsafe.JumpWarning(p.Before, p.Net)
	return p
}

// SignedMinor formats an amount with an explicit plus only for positive values.
func SignedMinor(amount int64, decimals int) string {
	text := money.FormatMinor(amount, decimals)
	if amount > 0 {
		text = "+" + text
	}
	return text
}

// ReceiptCandidates matches a positive, readable receipt against the live ledger.
func ReceiptCandidates(rows []extract.Row, merchant, total string, decimals int, txns []domain.Transaction, fallback time.Time) []receiptmatch.Candidate {
	receipt := Receipt(rows, merchant, total)
	minor, err := receipt.TotalMinor(decimals)
	if err != nil || minor <= 0 {
		return nil
	}
	return receiptmatch.Match(minor, receipt.Merchant, ReceiptDate(rows, fallback), txns, receiptmatch.DefaultWindowDays)
}

// ReceiptMatchDate returns the effective date used by candidate matching. A
// parsed receipt date wins; when it is absent, matching uses today's calendar
// date at UTC midnight, consistent with stored transaction dates.
func ReceiptMatchDate(rows []extract.Row, now time.Time) time.Time {
	if parsed := ReceiptDate(rows, time.Time{}); !parsed.IsZero() {
		return parsed
	}
	return dateutil.CalendarDayUTC(now)
}
