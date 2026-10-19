// SPDX-License-Identifier: MIT

package smartengine

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/debtpay"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/unusual"
)

const unusualCode = "SMART-U1"

func init() { register(unusualCode, u1Unusual) }

// u1Unusual adapts deterministic Unusual findings into the Smart insight
// contract. The key includes the typed signal so distinct findings on one
// transaction remain independently identifiable.
func u1Unusual(in Input) []smart.Insight {
	principalIDs := debtPrincipalTxnIDs(in)
	transactions := make([]domain.Transaction, 0, len(in.Transactions))
	for _, txn := range in.Transactions {
		if !principalIDs[txn.ID] {
			transactions = append(transactions, txn)
		}
	}
	findings := unusual.Detect(unusual.Input{
		Transactions: transactions,
		Categories:   in.Categories,
		Accounts:     in.Accounts,
		Recurring:    in.Recurring,
		Goals:        in.Goals,
		Budgets:      in.Budgets,
		Now:          in.Now,
		Rates:        in.Rates,
	})
	byID := make(map[string]domain.Transaction, len(in.Transactions))
	for _, txn := range in.Transactions {
		byID[txn.ID] = txn
	}
	out := make([]smart.Insight, 0, len(findings))
	for _, finding := range findings {
		txn, ok := byID[finding.TxnID]
		if !ok {
			continue
		}
		amount := finding.Evidence.AmountMinor
		currencyCode := finding.Evidence.CurrencyCode
		if currencyCode == "" {
			currencyCode = in.Base
		}
		ins := smart.Insight{
			Feature:  unusualCode,
			Page:     smart.PageTransactions,
			Key:      unusualCode + ":" + string(finding.Signal) + ":" + finding.TxnID,
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.u1.title", "%s charge: %s", unusualSignalName(finding.Signal), hmoneyc(amount, currencyCode)).
			WithDetail("smart.u1.detail", "%s on %s — %s", txnLabel(txn), txn.Date.Format("Jan 2, 2006"), unusualEvidence(finding)).
			WithAmount(mny(amount, currencyCode)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/transactions", RelatedType: "transaction", RelatedID: txn.ID}.
				WithLabel("smart.u1.action", "View transaction"))
		out = append(out, ins)
	}
	return out
}

func unusualSignalName(signal unusual.Signal) string {
	switch signal {
	case unusual.Spike:
		return "Unusual size"
	case unusual.PriceIncrease:
		return "Price increase"
	case unusual.PossibleDoubleCharge:
		return "Possible double charge"
	case unusual.FirstTimeMerchantLarge:
		return "First-time merchant"
	default:
		return "Unusual activity"
	}
}

func unusualEvidence(finding unusual.Finding) string {
	e := finding.Evidence
	parts := []string{"confidence " + string(finding.Confidence), unusualMethodText(e.Method)}
	if e.TypicalMinor > 0 {
		parts = append(parts, "typical amount "+hmoneyc(e.TypicalMinor, e.CurrencyCode))
	}
	if e.DeltaMinor > 0 {
		parts = append(parts, "above typical by "+hmoneyc(e.DeltaMinor, e.CurrencyCode))
	}
	if e.BaselineSource != "" {
		parts = append(parts, fmt.Sprintf("%s baseline from %d charges", e.BaselineSource, e.HistoryCount))
	}
	if e.Window != "" {
		parts = append(parts, "window "+e.Window)
	}
	if len(e.RelatedTxnIDs) > 0 {
		parts = append(parts, "related transaction IDs: "+strings.Join(e.RelatedTxnIDs, ", "))
	}
	if e.Confirmation != "" {
		parts = append(parts, e.Confirmation)
	}
	if len(e.ExpectationEvidence) > 0 {
		parts = append(parts, strings.Join(e.ExpectationEvidence, "; "))
	}
	return strings.Join(parts, "; ")
}

func unusualMethodText(method string) string {
	switch method {
	case "first_merchant_charge":
		return "no earlier charge from this merchant"
	case "median_mad":
		return "compared with the median and typical variation"
	case "same_merchant_amount_within_3_days":
		return "same merchant and amount appeared within three days"
	case "established_recurring_series":
		return "compared with an established recurring charge"
	default:
		return strings.ReplaceAll(method, "_", " ")
	}
}

// debtPrincipalTxnIDs returns cash-side debt-payment legs identified by the
// committed debtpay classifier. These payments reduce debt principal rather
// than represent ordinary purchases, so they must not enter unusual-spend
// baselines or findings.
func debtPrincipalTxnIDs(in Input) map[string]bool {
	data := debtpay.Dataset{}
	for _, account := range in.Accounts {
		data.Accounts = append(data.Accounts, debtpay.Account{
			ID: account.ID, Name: account.Name, Lender: account.Institution,
			Currency: account.Currency, Liability: account.Class == domain.ClassLiability,
		})
	}
	for _, txn := range in.Transactions {
		data.Transactions = append(data.Transactions, debtTransaction(txn))
	}
	ids := make(map[string]bool)
	for _, txn := range in.Transactions {
		if !txn.Amount.IsNegative() {
			continue
		}
		candidate := debtpay.Classify(debtTransaction(txn), data)
		if candidate.Confidence != debtpay.ConfidenceNone {
			ids[txn.ID] = true
		}
	}
	return ids
}

func debtTransaction(txn domain.Transaction) debtpay.Transaction {
	return debtpay.Transaction{
		ID: txn.ID, AccountID: txn.AccountID, Date: txn.Date,
		Payee: txn.Payee, Description: txn.Desc,
		AmountMinor: txn.Amount.Amount, Currency: txn.Amount.Currency,
		TransferGroupID: txn.TransferGroupID, TransferAccountID: txn.TransferAccountID,
		BillAccountID: txn.BillAccountID,
	}
}
