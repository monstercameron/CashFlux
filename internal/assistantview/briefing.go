// SPDX-License-Identifier: MIT

package assistantview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/insightsperiod"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

// BriefingInput names every data input to the Assistant Insights display.
type BriefingInput struct {
	Accounts      []domain.Account
	Transactions  []domain.Transaction
	Categories    []domain.Category
	Scope         scope.ReportScope
	Period        insightsperiod.Period
	Rates         currency.Rates
	Now           time.Time
	Uncategorized string
}

// Briefing is read-only after derivation and can be retained across UI updates.
// Flag verdicts, translated text and event callbacks remain presentation inputs.
type Briefing struct {
	Transactions   []domain.Transaction
	Window         insightsperiod.Range
	Spent          int64
	Prior          int64
	Anomalies      []insights.Anomaly
	Merchants      []reports.PayeeSummary
	MonthMerchants []reports.PayeeTotal
	Trend          Trend
	TrendError     error
}

// DeriveBriefing reuses the financial rules behind the assistant_* formula
// variables. All calendar calculations share the caller's single clock value.
func DeriveBriefing(in BriefingInput) Briefing {
	ids := scope.ResolveScope(in.Accounts, in.Scope, func(account domain.Account) string { return account.Institution })
	out := Briefing{Transactions: scope.ApplyScopeToTxns(in.Transactions, ids), Window: insightsperiod.Resolve(in.Period, in.Now)}
	w := out.Window
	out.Spent, out.Prior, _ = engineenv.AssistantSpendStoryRange(out.Transactions, in.Rates, w.Start, w.End, w.PriorStart, w.PriorEnd)
	out.Anomalies = engineenv.AssistantHighlights(out.Transactions, in.Categories, in.Rates, in.Now, in.Uncategorized)
	out.Merchants, _ = reports.TopPayeesTrailing(out.Transactions, engineenv.AssistantMerchantWindowDays, in.Now, in.Rates, 7)
	monthStart := dateutil.MonthStart(in.Now)
	out.MonthMerchants, _ = reports.TopPayees(out.Transactions, monthStart, dateutil.AddMonths(monthStart, 1), in.Rates, 1)
	out.Trend, out.TrendError = SpendTrend(out.Transactions, in.Rates, in.Now)
	return out
}
