// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

var assistantFinanceNow = time.Now

type assistantFinanceReadError struct {
	Collection string
	Err        error
}

func (e *assistantFinanceReadError) Error() string {
	return fmt.Sprintf("read finance %s: %v", e.Collection, e.Err)
}

func (e *assistantFinanceReadError) Unwrap() error { return e.Err }

// The adapter refreshes every invocation. A multi-step answer may perform edits
// between reads, and an account scope or FX preference can change while mounted.
func assistantFinanceForCall(app *appstate.App, fallbackBase string, currentScope []func() scope.ReportScope) (assistantdata.Finance, error) {
	if app == nil || app.Store() == nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "dataset", Err: fmt.Errorf("local dataset is unavailable")}
	}
	s := app.Store()
	settings, err := s.GetSettings()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "settings", Err: err}
	}
	accounts, err := s.ListAccounts()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "accounts", Err: err}
	}
	transactions, err := s.ListTransactions()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "transactions", Err: err}
	}
	categories, err := s.ListCategories()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "categories", Err: err}
	}
	budgets, err := s.ListBudgets()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "budgets", Err: err}
	}
	recurring, err := s.ListRecurring()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "recurring schedules", Err: err}
	}
	links, err := s.ListTxnLinks()
	if err != nil {
		return assistantdata.Finance{}, &assistantFinanceReadError{Collection: "transaction links", Err: err}
	}
	pr := uistate.LoadPrefs()
	base := settings.BaseCurrency
	if base == "" {
		base = fallbackBase
	}
	sc := scope.ReportScope{}
	if len(currentScope) > 0 && currentScope[0] != nil {
		sc = currentScope[0]()
	}
	anchor, _ := dateutil.ParseDate(pr.PayCycleAnchor)
	return assistantdata.BuildFinance(assistantdata.FinanceInput{Accounts: accounts, Transactions: transactions, Categories: categories,
		Budgets: budgets, Recurring: recurring, TxnLinks: links, Now: assistantFinanceNow(), Scope: sc, Base: base,
		Rates: currency.Rates{Base: base, Rates: settings.FXRates}, WeekStart: pr.WeekStartWeekday(), PayCycleAnchor: anchor, PoolRollover: pr.BudgetRolloverLeftover}), nil
}

type assistantFinanceWindowArgs struct {
	Period string `json:"period"`
	From   string `json:"from"`
	To     string `json:"to"`
}

func assistantFinanceWindow(f assistantdata.Finance, raw json.RawMessage) (reports.AskedWindow, error) {
	var args assistantFinanceWindowArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return reports.AskedWindow{}, err
		}
	}
	return f.ResolveWindow(args.Period, args.From, args.To)
}

func assistantFinanceContext(f assistantdata.Finance, w reports.AskedWindow) string {
	start := "beginning of recorded history"
	if !w.Start.IsZero() {
		start = w.Start.Format(dateutil.Layout)
	}
	return fmt.Sprintf("Activity window [%s, %s), scope: %s, base: %s; reference time %s. Linked refunds are netted to the original purchase period.", start, w.End.Format(dateutil.Layout), f.ScopeLabel(), f.Base(), f.ReferenceTime().Format(time.RFC3339))
}

func assistantFinanceUnavailable(err error) string {
	return "Calculation unavailable: " + err.Error() + ". No zero or complete total is implied."
}

func assistantFinanceAmount(raw json.RawMessage, base string, optional bool) (money.Money, error) {
	if len(raw) == 0 || string(raw) == "null" {
		if optional {
			return money.Zero(base), nil
		}
		return money.Money{}, fmt.Errorf("amount is required")
	}
	decimal := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &decimal); err != nil {
			return money.Money{}, err
		}
	}
	return assistantdata.ParseAmount(decimal, base)
}

// agToolsFinance supplies the six finance reads from checked canonical models.
// The optional provider is the live active-scope atom getter; no provider means
// an explicit whole-household account scope rather than a hidden stored filter.
func agToolsFinance(app *appstate.App, base string, rates currency.Rates, currentScope ...func() scope.ReportScope) []chatTool {
	_ = rates // Current stored FX is read for each invocation, including removals.
	newFinance := func() (assistantdata.Finance, error) { return assistantFinanceForCall(app, base, currentScope) }
	return []chatTool{
		{spec: ai.FunctionTool("financial_summary", "Checked income, spending, net and savings rate for a requested window in the active account scope, plus explicitly household-wide current net worth. Reports unavailable FX/arithmetic instead of zero.", json.RawMessage(`{"type":"object","properties":{`+agPeriodEnum+`}}`)),
			run: func(raw json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				w, err := assistantFinanceWindow(f, raw)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var b strings.Builder
				b.WriteString(assistantFinanceContext(f, w) + "\n")
				totals := f.Totals(w)
				if !totals.Known {
					b.WriteString("Activity: " + assistantFinanceUnavailable(totals.Error) + "\n")
				} else {
					fmt.Fprintf(&b, "Income %s; spending %s; net %s.", fmtMoney(totals.Income), fmtMoney(totals.Expense), fmtMoney(totals.Net))
					if totals.SavingsKnown() {
						fmt.Fprintf(&b, " Savings rate %d%%.\n", totals.SavingsPercent)
					} else {
						b.WriteString(" Savings rate unavailable.\n")
					}
				}
				nw := f.HouseholdNetWorth()
				if !nw.Known {
					b.WriteString("Household net worth: " + assistantFinanceUnavailable(nw.Err))
				} else {
					label := "Household net worth"
					if nw.Partial {
						label = "Household net-worth subtotal (incomplete; missing FX for " + strings.Join(nw.MissingCurrencies, ", ") + ")"
					}
					fmt.Fprintf(&b, "%s %s; assets %s; liabilities %s. This balance sheet is household-wide and current, independent of activity scope/window.", label, fmtMoney(nw.Net), fmtMoney(nw.Assets), fmtMoney(nw.Liabilities))
					if len(nw.ExcludedByChoice) > 0 {
						fmt.Fprintf(&b, " %d accounts excluded by household choice.", len(nw.ExcludedByChoice))
					}
				}
				return b.String()
			}},
		{spec: ai.FunctionTool("spending_by_category", "Checked refund-netted spending for one category (exact name or ID), including expense split lines and report exclusions, in the active account scope.", json.RawMessage(`{"type":"object","properties":{"category":{"type":"string"},`+agPeriodEnum+`},"required":["category"]}`)),
			run: func(raw json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				w, err := assistantFinanceWindow(f, raw)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var a struct {
					Category string `json:"category"`
				}
				if err = json.Unmarshal(raw, &a); err != nil {
					return assistantFinanceUnavailable(err)
				}
				row, err := f.Category(w, a.Category)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				return fmt.Sprintf("%s\nSpent %s on %s (%d contributing expense transactions).", assistantFinanceContext(f, w), fmtMoney(row.Amount), row.Name, row.Count)
			}},
		{spec: ai.FunctionTool("spending_breakdown", "Checked category spending for a requested window in the active account scope, including split-line attribution and linked-refund netting. An unavailable category calculation is never called no spending.", json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"},`+agPeriodEnum+`}}`)),
			run: func(raw json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				w, err := assistantFinanceWindow(f, raw)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var a struct {
					Limit int `json:"limit"`
				}
				if len(raw) > 0 {
					if err = json.Unmarshal(raw, &a); err != nil {
						return assistantFinanceUnavailable(err)
					}
				}
				if a.Limit <= 0 {
					a.Limit = 8
				}
				if a.Limit > 100 {
					a.Limit = 100
				}
				rows, err := f.Spending(w)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var b strings.Builder
				b.WriteString(assistantFinanceContext(f, w) + "\n")
				if len(rows) == 0 {
					return b.String() + "No spending in the requested scope/window."
				}
				for i, row := range rows {
					if i >= a.Limit {
						fmt.Fprintf(&b, "%d more categories omitted; increase limit.\n", len(rows)-i)
						break
					}
					fmt.Fprintf(&b, "%s: %s (%d transactions)\n", row.Name, fmtMoney(row.Amount), row.Count)
				}
				return strings.TrimSpace(b.String())
			}},
		{spec: ai.FunctionTool("account_balances", "Current balances of active accounts in the selected account scope, each in its own currency. Retains unavailable accounts explicitly rather than hiding them.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				if f.Err() != nil {
					return assistantFinanceUnavailable(f.Err())
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Current account balances; scope: %s; reference time %s. Amounts use each account's currency.\n", f.ScopeLabel(), f.ReferenceTime().Format(time.RFC3339))
				rows := f.AccountBalances()
				if len(rows) == 0 {
					return b.String() + "No active accounts in this scope."
				}
				for _, row := range rows {
					if row.Err != nil {
						fmt.Fprintf(&b, "%s (%s): unavailable (%v)\n", row.Account.Name, row.Account.Currency, row.Err)
					} else {
						fmt.Fprintf(&b, "%s: %s (%s)\n", row.Account.Name, fmtMoney(row.Balance), row.Balance.Currency)
					}
				}
				return strings.TrimSpace(b.String())
			}},
		{spec: ai.FunctionTool("list_budgets", "Budgets with standing and effective limits, checked spending/remaining/state, and their own cadence windows. Spending follows the active account scope; plan limits retain their budget ownership and rollover/boost policy.", json.RawMessage(`{"type":"object","properties":{`+agPeriodEnum+`}}`)),
			run: func(raw json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				w, err := assistantFinanceWindow(f, raw)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var b strings.Builder
				b.WriteString(assistantFinanceContext(f, w) + "\nBudgets use their own cadence at the selected window's anchor. Spending follows account scope; limits remain the budget's plan. Each budget's amounts retain its evaluation currency.\n")
				rows := f.Budgets(w)
				if len(rows) == 0 {
					return b.String() + "No visible budgets."
				}
				for _, row := range rows {
					fmt.Fprintf(&b, "%s [%s, %s), %s, owner %s (%s), currency %s; categories %s; tags %s: standing %s; ", row.Budget.Name, row.Start.Format(dateutil.Layout), row.End.Format(dateutil.Layout), row.Budget.Period, row.Budget.OwnerID, row.Budget.Scope, row.Budget.Limit.Currency, strings.Join(row.CategoryNames, ", "), strings.Join(row.Tags, ", "), fmtMoney(row.Budget.Limit))
					if !row.Valid {
						fmt.Fprintf(&b, "spending/remaining/state unavailable (%v).\n", row.Error)
						continue
					}
					fmt.Fprintf(&b, "effective %s; spent %s; remaining %s; state %s (%d%%).\n", fmtMoney(row.Status.Limit), fmtMoney(row.Status.Spent), fmtMoney(row.Status.Remaining), row.Status.State, row.Status.Percent)
				}
				return strings.TrimSpace(b.String())
			}},
		{spec: ai.FunctionTool("check_affordability", "Checked constant-cash-flow projection using available cash assets in the active scope and their calendar-month net activity. Excludes locked/restricted accounts, liabilities and investments/property. This is not safe-to-spend: only an explicit reserve is deducted, not scheduled bills or goal earmarks. Decimal amount/reserve accept exact strings or JSON decimals in base currency major units.", json.RawMessage(`{"type":"object","properties":{"amount":{"type":["string","number"]},"months":{"type":"integer","minimum":0},"reserve":{"type":["string","number"],"description":"explicit cash reserve; defaults to zero"}},"required":["amount"]}`)),
			run: func(raw json.RawMessage) string {
				f, err := newFinance()
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				var a struct {
					Amount  json.RawMessage `json:"amount"`
					Reserve json.RawMessage `json:"reserve"`
					Months  int             `json:"months"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return assistantFinanceUnavailable(err)
				}
				amount, err := assistantFinanceAmount(a.Amount, f.Base(), false)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				reserve, err := assistantFinanceAmount(a.Reserve, f.Base(), true)
				if err != nil {
					return assistantFinanceUnavailable(err)
				}
				result := f.Affordability(amount, reserve, a.Months)
				if !result.Known {
					return assistantFinanceUnavailable(result.Err)
				}
				verdict := "Projected cash does not cover the purchase"
				if result.Result.Affordable {
					verdict = "Projected cash covers the purchase under these assumptions"
				}
				return fmt.Sprintf("%s. Purchase %s (%s), liquid cash %s, available cash accounts' calendar-month net %s, horizon %d months, explicit reserve %s; projected %s and available after that reserve %s. Scope: %s; reference time %s. Available cash assets only; locked/restricted accounts, liabilities and investments/property excluded. Scheduled bills, debt commitments and goal earmarks are not deducted. This is a constant-cash-flow estimate, not a safe-to-spend amount.", verdict, fmtMoney(amount), f.Base(), fmtMoney(result.Liquid), fmtMoney(result.MonthlyNet), result.Months, fmtMoney(reserve), fmtMoney(money.New(result.Result.ProjectedBalance, f.Base())), fmtMoney(money.New(result.Result.Available, f.Base())), f.ScopeLabel(), f.ReferenceTime().Format(time.RFC3339))
			}},
	}
}
