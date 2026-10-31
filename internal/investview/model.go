// SPDX-License-Identifier: MIT

// Package investview prepares native investment-screen facts from immutable
// domain inputs. It keeps balance and base-currency availability per account.
package investview

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/portfolio"
)

// Input supplies the data needed to prepare the investment page. The slices are
// read-only; Prepare copies every returned slice and never mutates its inputs.
type Input struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Holdings     []domain.Holding
	Rates        currency.Rates
}

// AccountBalance holds a ledger balance and its independent base-currency
// conversion. A valid native balance remains known when FX is unavailable.
type AccountBalance struct {
	Native     money.Money
	Base       money.Money
	BalanceErr error
	BaseErr    error
}

// NativeKnown reports whether the account's ledger balance is valid.
func (b AccountBalance) NativeKnown() bool { return b.BalanceErr == nil }

// BaseKnown reports whether the ledger balance and its conversion are valid.
func (b AccountBalance) BaseKnown() bool { return b.BalanceErr == nil && b.BaseErr == nil }

// Model is the prepared investments screen data. Holdings and account lists
// contain only active investment accounts, matching the screen's prior scope.
type Model struct {
	Accounts            []domain.Account
	Securities          []domain.Holding
	Traditional         []domain.Account
	HoldingsByAccount   map[string][]domain.Holding
	BalancesByAccount   map[string]AccountBalance
	BalanceByID         map[string]money.Money // known base-currency balances only
	SecuritiesByID      map[string]int64
	SecuritiesErrByID   map[string]error
	Summary             portfolio.Summary
	AllocationByClass   []portfolio.Weight
	AllocationByType    []portfolio.Weight
	AllocationBySector  []portfolio.Weight
	AllocationByRegion  []portfolio.Weight
	Fees                portfolio.FeeDrag
	TraditionalValue    money.Money
	TraditionalKnown    bool
	TraditionalErr      error
	TotalValue          money.Money
	TotalKnown          bool
	TotalErr            error
	Reconciliation      portfolio.Reconciliation
	ReconciliationKnown bool
	ReconciliationErr   error
}

// HasAccounts reports whether any active investment account is present.
func (m Model) HasAccounts() bool { return len(m.Accounts) != 0 }

// Prepare classifies active investment accounts and holdings, builds one
// holding index, and composes the existing portfolio calculations.
// It preserves existing eligibility and allocation buckets: accounts qualify
// by investment type, and only holdings on those accounts are included.
func Prepare(in Input) Model {
	m := Model{
		Accounts:          make([]domain.Account, 0, len(in.Accounts)),
		Traditional:       make([]domain.Account, 0, len(in.Accounts)),
		Securities:        make([]domain.Holding, 0, len(in.Holdings)),
		HoldingsByAccount: make(map[string][]domain.Holding),
		BalancesByAccount: make(map[string]AccountBalance),
		BalanceByID:       make(map[string]money.Money),
		SecuritiesByID:    make(map[string]int64),
		SecuritiesErrByID: make(map[string]error),
		TraditionalValue:  money.Zero(in.Rates.Base),
		TotalValue:        money.Zero(in.Rates.Base),
	}
	accountIDs := make(map[string]bool, len(in.Accounts))
	for _, account := range in.Accounts {
		if account.Archived || !account.Type.IsInvestment() {
			continue
		}
		m.Accounts = append(m.Accounts, account)
		accountIDs[account.ID] = true
	}

	for _, holding := range in.Holdings {
		if accountIDs[holding.AccountID] {
			m.HoldingsByAccount[holding.AccountID] = append(m.HoldingsByAccount[holding.AccountID], holding)
			m.Securities = append(m.Securities, holding)
		}
	}

	for _, account := range m.Accounts {
		// Reuse ledger's checked, source-ordered calculation. It filters by account
		// itself, avoiding transaction-slice copies while retaining overflow behavior.
		balance, err := ledger.Balance(account, in.Transactions)
		fact := AccountBalance{BalanceErr: err}
		if err == nil {
			fact.Native = balance
			converted, convertErr := in.Rates.Convert(balance, in.Rates.Base)
			fact.BaseErr = convertErr
			if convertErr == nil {
				fact.Base = converted
				m.BalanceByID[account.ID] = converted
			}
		} else {
			fact.BaseErr = fmt.Errorf("investview: base balance unavailable because native balance failed: %w", err)
		}
		m.BalancesByAccount[account.ID] = fact

		if len(m.HoldingsByAccount[account.ID]) == 0 {
			m.Traditional = append(m.Traditional, account)
			if !fact.BaseKnown() {
				if m.TraditionalErr == nil {
					m.TraditionalErr = fmt.Errorf("investview: traditional account %s unavailable: %w", account.ID, fact.BaseErr)
				}
				continue
			}
			next, addErr := m.TraditionalValue.Add(fact.Base)
			if addErr != nil {
				if m.TraditionalErr == nil {
					m.TraditionalErr = fmt.Errorf("investview: traditional account total: %w", addErr)
				}
				continue
			}
			m.TraditionalValue = next
		}
	}
	m.TraditionalKnown = m.TraditionalErr == nil

	portfolioHoldings := portfolio.FromDomainSlice(m.Securities)
	m.Summary = portfolio.PortfolioSummary(portfolioHoldings)
	m.AllocationByClass = portfolio.AllocationByAssetClass(portfolioHoldings)
	m.AllocationByType = portfolio.AllocationBySecurityType(portfolioHoldings)
	m.AllocationBySector = portfolio.AllocationBySector(portfolioHoldings)
	m.AllocationByRegion = portfolio.AllocationByRegion(portfolioHoldings)
	m.Fees = portfolio.Fees(portfolioHoldings)

	if m.TraditionalKnown {
		securitiesValue := money.New(m.Summary.TotalValueMinor, in.Rates.Base)
		total, err := securitiesValue.Add(m.TraditionalValue)
		if err != nil {
			m.TotalErr = fmt.Errorf("investview: total portfolio value: %w", err)
		} else {
			m.TotalValue, m.TotalKnown = total, true
		}
	} else {
		m.TotalErr = m.TraditionalErr
	}

	// Reconciliation is an all-account fact. Keep it unavailable if any balance,
	// holding subtotal, or checked aggregate is unavailable; never pass a partial
	// subset to the legacy unchecked renderer as though it were complete.
	accountValues := make([]portfolio.AccountValue, 0, len(m.Accounts))
	for _, account := range m.Accounts {
		fact := m.BalancesByAccount[account.ID]
		if !fact.BaseKnown() {
			m.ReconciliationErr = fmt.Errorf("investview: reconciliation account %s: %w", account.ID, fact.BaseErr)
			break
		}
		var securities int64
		for _, holding := range m.HoldingsByAccount[account.ID] {
			value := portfolio.HoldingValueMinor(portfolio.FromDomain(holding))
			securities, m.SecuritiesErrByID[account.ID] = money.Add(securities, value)
			if m.SecuritiesErrByID[account.ID] != nil {
				break
			}
		}
		if err := m.SecuritiesErrByID[account.ID]; err != nil {
			m.ReconciliationErr = fmt.Errorf("investview: reconciliation securities for %s: %w", account.ID, err)
			break
		}
		m.SecuritiesByID[account.ID] = securities
		accountValues = append(accountValues, portfolio.AccountValue{
			AccountID: account.ID, Name: account.Name,
			BalanceMinor: fact.Base.Amount, SecuritiesMinor: securities,
		})
	}
	if m.ReconciliationErr == nil {
		m.Reconciliation, m.ReconciliationErr = portfolio.ReconcileChecked(accountValues)
		m.ReconciliationKnown = m.ReconciliationErr == nil
	}

	sort.SliceStable(m.Securities, func(i, j int) bool {
		return portfolio.HoldingValueMinor(portfolio.FromDomain(m.Securities[i])) >
			portfolio.HoldingValueMinor(portfolio.FromDomain(m.Securities[j]))
	})
	sort.SliceStable(m.Traditional, func(i, j int) bool {
		a, aok := m.BalanceByID[m.Traditional[i].ID]
		b, bok := m.BalanceByID[m.Traditional[j].ID]
		if aok != bok {
			return aok
		}
		if !aok {
			return false
		}
		return a.Amount > b.Amount
	})
	return m
}
