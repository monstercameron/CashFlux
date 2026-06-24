// SPDX-License-Identifier: MIT

// Package aitools defines the Insights agent's tools on the C82 agent.Registry
// (C89 phase 2). It starts with the read tools — the safe, run-freely queries that
// let the model pull detail beyond the injected summary (the MCP "tools" half): how
// much was spent on X, account balances, and whether something is affordable.
//
// The tools bind to the data through a small DataSource interface, NOT appstate
// directly, so this package stays pure and table-tests with a fake — appstate
// provides the production implementation (read-first; the write tools and their
// audit/undo come in phase 3). Reuses txnfilter (C83) and afford (L8).
package aitools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/agent"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// DataSource is the read view the tools query. Money is in minor units; Format/Parse
// keep currency formatting at the implementation (appstate) so the tools stay
// currency-agnostic and pure.
type DataSource interface {
	Transactions() []domain.Transaction
	Accounts() []domain.Account
	Balance(a domain.Account) int64 // current balance, minor units
	LiquidBalance() int64           // cash on hand for affordability, minor units
	MonthlyNet() int64              // this month's income minus expense, minor units
	FormatMoney(minor int64) string
	ParseMoney(s string) (int64, error)
}

// RegisterRead adds the read tools to a registry, bound to src. Safe to call once
// per agent run; tools are name-keyed so a re-register replaces.
func RegisterRead(reg *agent.Registry, src DataSource) {
	reg.Register(queryTransactions(src))
	reg.Register(accountBalances(src))
	reg.Register(affordability(src))
}

func queryTransactions(src DataSource) agent.Tool {
	return agent.Tool{
		Name:        "query_transactions",
		Description: "Count and net-total the user's transactions matching a filter (categories, accounts, members, tags). Use to answer questions like 'how much did I spend on groceries?'.",
		Params:      json.RawMessage(`{"type":"object","properties":{"categories":{"type":"array","items":{"type":"string"}},"accounts":{"type":"array","items":{"type":"string"}},"members":{"type":"array","items":{"type":"string"}},"tags":{"type":"array","items":{"type":"string"}}}}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				Categories []string `json:"categories"`
				Accounts   []string `json:"accounts"`
				Members    []string `json:"members"`
				Tags       []string `json:"tags"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &a); err != nil {
					return "", err
				}
			}
			mc := txnfilter.MultiCriteria{Accounts: a.Accounts, Categories: a.Categories, Members: a.Members, Tags: a.Tags}
			rows := mc.Filter(src.Transactions())
			var net int64
			for _, t := range rows {
				net += t.Amount.Amount
			}
			return fmt.Sprintf("%d transactions, net %s.", len(rows), src.FormatMoney(net)), nil
		},
	}
}

func accountBalances(src DataSource) agent.Tool {
	return agent.Tool{
		Name:        "account_balances",
		Description: "List every account with its current balance.",
		Params:      json.RawMessage(`{"type":"object","properties":{}}`),
		Handler: func(json.RawMessage) (string, error) {
			accts := src.Accounts()
			if len(accts) == 0 {
				return "No accounts.", nil
			}
			var b strings.Builder
			for _, a := range accts {
				fmt.Fprintf(&b, "%s: %s\n", a.Name, src.FormatMoney(src.Balance(a)))
			}
			return strings.TrimRight(b.String(), "\n"), nil
		},
	}
}

func affordability(src DataSource) agent.Tool {
	return agent.Tool{
		Name:        "affordability",
		Description: "Check whether the user can afford a purchase, projected from their liquid balance and monthly net cash flow. amount is a decimal string; months is how many months out (0 = now).",
		Params:      json.RawMessage(`{"type":"object","properties":{"amount":{"type":"string"},"months":{"type":"integer"}},"required":["amount"]}`),
		Handler: func(raw json.RawMessage) (string, error) {
			var a struct {
				Amount string `json:"amount"`
				Months int    `json:"months"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", err
			}
			amt, err := src.ParseMoney(a.Amount)
			if err != nil {
				return "", err
			}
			res := afford.CanAfford(amt, src.LiquidBalance(), src.MonthlyNet(), a.Months, 0)
			if res.Affordable {
				return fmt.Sprintf("Yes — projected balance %s covers %s (%s free to spend).",
					src.FormatMoney(res.ProjectedBalance), src.FormatMoney(amt), src.FormatMoney(res.Available)), nil
			}
			if res.MonthsNeeded > 0 {
				return fmt.Sprintf("Not yet — short %s now; affordable in about %d months at this pace.",
					src.FormatMoney(res.Shortfall), res.MonthsNeeded), nil
			}
			return fmt.Sprintf("Not affordable — short %s, and the current cash flow won't cover it.", src.FormatMoney(res.Shortfall)), nil
		},
	}
}
