// SPDX-License-Identifier: MIT

package smartengine

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/debtpay"
	"strings"

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
	job := NewUnusualJob(in)
	for !job.Step(32) {
	}
	return job.Insights()
}

// DetectUnusualFindings returns raw unusual-spend findings after applying the
// same debt-payment exclusion used by the production Smart engine.
func DetectUnusualFindings(in Input, expectedTxnIDs ...map[string][]string) []unusual.Finding {
	job := NewUnusualJob(in)
	if len(expectedTxnIDs) > 0 {
		job.expectedTxnIDs = expectedTxnIDs[0]
	}
	for !job.Step(256) {
	}
	return append([]unusual.Finding(nil), job.findings...)
}

func adaptUnusualFinding(finding unusual.Finding, byID map[string]domain.Transaction, in Input) (smart.Insight, bool) {

	txn, ok := byID[finding.TxnID]
	if !ok {
		return smart.Insight{}, false
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
	return ins, true
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

func debtTransaction(txn domain.Transaction) debtpay.Transaction {
	return debtpay.Transaction{
		ID: txn.ID, AccountID: txn.AccountID, Date: txn.Date,
		Payee: txn.Payee, Description: txn.Desc,
		AmountMinor: txn.Amount.Amount, Currency: txn.Amount.Currency,
		TransferGroupID: txn.TransferGroupID, TransferAccountID: txn.TransferAccountID,
		BillAccountID: txn.BillAccountID,
	}
}
