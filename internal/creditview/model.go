// SPDX-License-Identifier: MIT

// Package creditview prepares immutable Credit page facts without browser state.
package creditview

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// HistoryInput distinguishes an unread history source from a known empty one.
type HistoryInput struct {
	Snapshots []domain.BalanceSnapshot
	Err       error
}

// Input contains household-wide source snapshots, explicit reference time and
// rates. Details controls advice/card preparation; histories are loaded only by
// full visual hosts. No caller slice or map is mutated.
type Input struct {
	Accounts                               []domain.Account
	Transactions                           []domain.Transaction
	AccountsErr, TransactionsErr, RatesErr error
	Rates                                  currency.Rates
	Now                                    time.Time
	Details, History                       bool
	Histories                              map[string]HistoryInput
}

// Factor carries score and normalized weight presentation facts independently.
type Factor struct {
	Score, WeightPercent, Meter int
	Weight                      float64
	OnTarget, NoData            bool
	Err                         error
}

// Card contains native monetary facts and checked presentation values.
type Card struct {
	credithealth.CardSnapshot
	Owed                           money.Money
	OwedErr                        error
	Band                           credithealth.UtilBand
	UtilKnown, NoLimit, HasPayment bool
	Meter                          int
	EditorCurrency                 string
	History                        credithealth.UtilizationHistory
}

// Model is shared by full/embedded Credit hosts and AI context. Failed facts
// retain errors; no caller should substitute their zero values for known data.
type Model struct {
	credithealth.CheckedResult
	Cards                        []Card
	Base, Scope, Disclaimer      string
	AsOf                         time.Time
	Band                         credithealth.Band
	UtilBand                     credithealth.UtilBand
	UtilMeter, UtilWeightPercent int
	UtilOnTarget                 bool
	OnTime, Age                  Factor
	Demerits                     []credithealth.CheckedDemerit
	Advice                       []credithealth.CheckedAdvice
	Incomplete, Empty, Fatal     bool
	SourceErr                    error
	BalanceFolds, HistoryBuilds  int
}

func meter(pct int64) int {
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return int(pct)
}
func band(pct int64) credithealth.UtilBand {
	if pct > 80 {
		return credithealth.BandUtilWorst
	}
	return credithealth.UtilBandFor(int(pct))
}
func factor(score int, weight float64, err error) Factor {
	f := Factor{Score: score, Weight: weight, WeightPercent: int(math.Round(weight * 100)), Meter: score, OnTarget: score >= 90, NoData: score < 0 && err == nil, Err: err}
	if f.Meter == 0 {
		f.Meter = 2
	}
	if f.Meter < 0 {
		f.Meter = 0
	}
	return f
}

// Prepare composes one checked card-balance fold, canonical credit evaluation,
// optional checked advice, and independent historical observations. Scope is
// household-wide, matching the engine credit atoms and existing Credit policy.
func Prepare(in Input) Model {
	m := Model{Base: in.Rates.Base, Scope: "household", AsOf: in.Now, Disclaimer: credithealth.Disclaimer, SourceErr: errors.Join(in.AccountsErr, in.TransactionsErr, in.RatesErr)}
	if in.AccountsErr != nil || in.Now.IsZero() {
		m.Fatal = true
		m.Incomplete = true
		if in.Now.IsZero() {
			m.SourceErr = errors.Join(m.SourceErr, fmt.Errorf("credit: reference time is required"))
		}
		return m
	}
	var accounts []domain.Account
	for _, a := range in.Accounts {
		if a.Type == domain.TypeCreditCard && !a.Archived {
			accounts = append(accounts, a)
		}
	}
	m.Empty = len(accounts) == 0
	balances := map[string]money.Money{}
	if in.TransactionsErr == nil {
		m.BalanceFolds = 1
		balances, _ = ledger.Balances(accounts, in.Transactions)
	}
	native := map[string]int64{}
	for _, a := range accounts {
		if balance, ok := balances[a.ID]; ok && balance.Currency == a.Currency {
			native[a.ID] = balance.Amount
		}
	}
	rates := in.Rates
	if in.RatesErr != nil {
		rates = currency.Rates{}
	}
	inputs := credithealth.Inputs{Accounts: accounts, Balances: native, Transactions: in.Transactions, TransactionsErr: in.TransactionsErr, Now: in.Now}
	m.CheckedResult = credithealth.EvaluateWithRates(inputs, rates)
	m.UtilBand = band(m.UtilPct)
	m.UtilMeter = meter(m.UtilPct)
	m.UtilWeightPercent = int(math.Round(m.Weights.Util * 100))
	m.UtilOnTarget = m.UtilErr == nil && m.UtilPct >= 0 && m.UtilPct < 30
	m.OnTime = factor(m.OnTimeScore, m.Weights.OnTime, m.OnTimeErr)
	m.Age = factor(m.AgeScore, m.Weights.Age, nil)
	if m.ProxyKnown {
		m.Band = credithealth.BandFor(m.ProxyScore)
	}
	m.Incomplete = m.SourceErr != nil || m.UtilErr != nil || m.OnTimeErr != nil
	if !in.Details {
		return m
	}
	details := credithealth.PrepareDetails(inputs, rates, m.CheckedResult)
	m.Demerits, m.Advice = details.Demerits, details.Advice
	m.Incomplete = m.Incomplete || details.Incomplete
	for i, snapshot := range m.CheckedResult.Cards {
		a := accounts[i]
		card := Card{CardSnapshot: snapshot, Band: band(snapshot.UtilPct), UtilKnown: snapshot.Err == nil && snapshot.UtilErr == nil, NoLimit: snapshot.Limit.Amount == 0, HasPayment: snapshot.Err == nil && snapshot.Target30.Amount > 0, Meter: meter(snapshot.UtilPct), EditorCurrency: a.CreditLimit.Currency}
		if card.EditorCurrency == "" {
			card.EditorCurrency = a.Currency
		}
		if balance, ok := balances[a.ID]; ok && balance.Currency == a.Currency && currency.Valid(a.Currency) {
			card.Owed, card.OwedErr = balance.AbsChecked()
		} else {
			card.OwedErr = credithealth.ErrMissingBalance
		}
		if in.History {
			m.HistoryBuilds++
			source := in.Histories[a.ID]
			if source.Err != nil {
				card.History.Err = source.Err
			} else if in.RatesErr != nil {
				card.History.Err = in.RatesErr
			} else {
				card.History = credithealth.BuildUtilizationHistory(a, source.Snapshots, rates)
			}
		}
		if card.OwedErr != nil || card.Err != nil || card.UtilErr != nil || card.History.Err != nil {
			m.Incomplete = true
		}
		m.Cards = append(m.Cards, card)
	}
	return m
}
