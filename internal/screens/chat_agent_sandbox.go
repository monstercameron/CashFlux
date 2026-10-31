// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsSandbox(app, base, rates)...) in buildChatTools

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

const (
	sandboxDefaultMonths = 12
	sandboxMaxMonths     = 60
)

type assistantSandboxArgs struct {
	Label         string          `json:"label"`
	MonthlyChange json.RawMessage `json:"monthly_change"`
	Months        json.RawMessage `json:"months"`
	Account       string          `json:"account,omitempty"`
}

// agToolsSandbox adds the what-if scenario tool (AG2). Each run reads the current
// settings and account rows, then makes all hypothetical writes to NewSandbox's
// isolated copy. The legacy base/rates arguments remain for registry compatibility;
// the scenario currency is derived from the account selected for this run.
func agToolsSandbox(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_, _ = base, rates
	return []chatTool{{
		spec: ai.FunctionTool("run_what_if",
			"Try a hypothetical recurring monthly change against a disposable copy of the user's data. "+
				"Use for questions like \"what if rent went up to 1800?\" or \"what if I saved 200 more a month?\". "+
				"monthly_change is an exact decimal in the selected account's currency; use a negative amount for a new or increased cost, positive for extra income or saving. "+
				"You may pass account as an exact account ID or name. If omitted, the default member's account preference is used when valid, then the household default, then the first active asset, then the first active account. "+
				"months defaults to 12 only when omitted and must be from 1 through 60. Nothing is saved. This is a hypothetical ledger simulation evaluated by current engine assumptions, not a complete future forecast. Report the selected account, currency, horizon, base currency, and any unavailable results. Ask whether they want to make a scenario real; do not apply it.",
			json.RawMessage(`{"type":"object","properties":{"label":{"type":"string","description":"what the scenario is, in the user's words"},"monthly_change":{"oneOf":[{"type":"string"},{"type":"number"}],"description":"exact plain decimal recurring monthly amount in the selected account currency; negative for a new or increased cost, positive for extra income or saving; no exponent notation"},"months":{"type":"integer","description":"horizon from 1 through 60 months; omitted defaults to 12"},"account":{"type":"string","description":"optional exact account ID or exact account name; duplicate names are ambiguous"}},"required":["label","monthly_change"],"additionalProperties":false}`)),
		run: func(raw json.RawMessage) string {
			var args assistantSandboxArgs
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return "Couldn't read the scenario: provide one object with the documented fields."
			}
			if err := dec.Decode(new(any)); err != io.EOF {
				return "Couldn't read the scenario: provide one JSON object."
			}
			label := strings.TrimSpace(args.Label)
			if label == "" {
				return "A scenario needs a nonblank label."
			}
			months := sandboxDefaultMonths
			if len(args.Months) > 0 {
				parsed, err := strconv.Atoi(string(args.Months))
				if err != nil {
					return "months must be an integer from 1 through 60; no scenario was run."
				}
				months = parsed
			}
			if months < 1 || months > sandboxMaxMonths {
				return fmt.Sprintf("months must be between 1 and %d; no scenario was run.", sandboxMaxMonths)
			}
			if app == nil || app.Store() == nil {
				return "Couldn't read the current household data."
			}
			settings, err := app.Store().GetSettings()
			if err != nil {
				return "Couldn't read the current household settings."
			}
			accounts, err := app.Store().ListAccounts()
			if err != nil {
				return "Couldn't read the current accounts."
			}
			members, err := app.Store().ListMembers()
			if err != nil {
				return "Couldn't read current account preferences."
			}
			account, err := selectSandboxAccount(args.Account, accounts, members, settings.DefaultAccountID)
			if err != nil {
				return err.Error()
			}
			if !currency.Valid(account.Currency) {
				return fmt.Sprintf("Account %q has unsupported currency %q; no scenario was run.", account.Name, account.Currency)
			}
			minor, err := parseSandboxAmount(args.MonthlyChange, currency.Decimals(account.Currency))
			if err != nil {
				return fmt.Sprintf("monthly_change must be an exact plain decimal valid for %s: %v", account.Currency, err)
			}
			if minor == 0 {
				return "That scenario does not change anything month to month, so there is nothing to compare."
			}
			sandbox, err := app.NewSandbox()
			if err != nil {
				return "Couldn't set up a scenario to try that in."
			}
			defer sandbox.App.Store().Close()
			start := app.ClockNow()
			for i := 0; i < months; i++ {
				if err := sandbox.App.PutTransaction(domain.Transaction{
					ID: id.New(), AccountID: account.ID, Payee: label, Desc: label,
					Amount: money.New(minor, account.Currency), Date: dateutil.AddMonthsClamped(start, i),
				}); err != nil {
					return "Couldn't apply the scenario to its disposable copy: " + err.Error()
				}
			}
			changes := sandbox.Diff(0)
			amountText := fmt.Sprintf("%s %s", account.Currency, money.FormatMinor(minor, currency.Decimals(account.Currency)))
			baseCurrency := strings.ToUpper(strings.TrimSpace(settings.BaseCurrency))
			if baseCurrency == "" {
				baseCurrency = "USD (default)"
			} else if !currency.Valid(baseCurrency) {
				baseCurrency += " (unsupported code)"
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Scenario: %s; %s per month for %d months.\nAccount: %s (%s, %s). Household base currency: %s.\nNothing was saved; all scenario transactions were written to a disposable copy.\nThis is a hypothetical ledger simulation evaluated by the current engine, not a complete future forecast.\n",
				label, amountText, months, account.Name, account.ID, account.Currency, baseCurrency)
			if len(changes) == 0 {
				b.WriteString("\nNo tracked engine variables changed in this simulation.")
			} else {
				b.WriteString("\nTracked changes (engine values; monetary variables are expressed in the household base currency):\n")
				for i, change := range changes {
					if i >= 8 {
						fmt.Fprintf(&b, "(and %d additional changes)\n", len(changes)-i)
						break
					}
					fmt.Fprintf(&b, "- %s: %s → %s\n", change.Name, formatSandboxVar(change.Before, change.Added, change.Removed, true), formatSandboxVar(change.After, change.Added, change.Removed, false))
				}
			}
			b.WriteString("\nExplain the movements in plain English and ask whether they want to make a scenario real; do not apply it.")
			return b.String()
		},
	}}
}

func parseSandboxAmount(raw json.RawMessage, decimals int) (int64, error) {
	var text string
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("amount is required")
	}
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, fmt.Errorf("invalid amount")
		}
	} else {
		text = string(raw)
	}
	if strings.ContainsAny(strings.ToLower(text), "e") {
		return 0, fmt.Errorf("exponent notation is not supported")
	}
	return money.ParseMinor(strings.TrimSpace(text), decimals)
}

func selectSandboxAccount(selector string, accounts []domain.Account, members []domain.Member, householdDefault string) (domain.Account, error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		for _, account := range accounts {
			if account.ID == selector && !account.Archived {
				return account, nil
			}
		}
		var matches []domain.Account
		for _, account := range accounts {
			if account.Archived {
				continue
			}
			if account.Name == selector {
				matches = append(matches, account)
			}
		}
		if len(matches) == 0 {
			return domain.Account{}, fmt.Errorf("No active account exactly matches %q; no scenario was run.", selector)
		}
		if len(matches) > 1 {
			return domain.Account{}, fmt.Errorf("Account name %q is ambiguous; select its exact account ID.", selector)
		}
		return matches[0], nil
	}
	defaultMember := ""
	if len(members) > 0 {
		defaultMember = members[0].Prefs.DefaultAccountID
	}
	for _, member := range members {
		if member.IsDefault {
			defaultMember = member.Prefs.DefaultAccountID
			break
		}
	}
	active := func(id string) (domain.Account, bool) {
		for _, account := range accounts {
			if account.ID == id && !account.Archived {
				return account, true
			}
		}
		return domain.Account{}, false
	}
	if account, ok := active(defaultMember); ok {
		return account, nil
	}
	if account, ok := active(householdDefault); ok {
		return account, nil
	}
	for _, account := range accounts {
		if !account.Archived && account.Class == domain.ClassAsset {
			return account, nil
		}
	}
	for _, account := range accounts {
		if !account.Archived {
			return account, nil
		}
	}
	return domain.Account{}, fmt.Errorf("There is no active account to run the scenario against.")
}

func formatSandboxVar(value float64, added, removed, before bool) string {
	if before && added {
		return "unavailable (not present before scenario)"
	}
	if !before && removed {
		return "unavailable (not present after scenario)"
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "unavailable (non-finite engine value)"
	}
	if value == math.Trunc(value) && math.Abs(value) <= 1<<53 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.4f", value)
}
