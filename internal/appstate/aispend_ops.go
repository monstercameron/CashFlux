// SPDX-License-Identifier: MIT

// Package appstate — aispend_ops.go records what the AI features cost (EC-15).
//
// The recording deliberately lives on App rather than at each call site: every AI
// surface already knows its own tokens and model, but only one of them should know
// how the ledger is stored, trimmed and persisted. A surface records a call; this
// file decides everything else.
package appstate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/store"
)

// aiSpendKeepMonths is how much history the meter keeps. A year answers "what has
// this cost me?" as far back as anyone asks, and bounds a field that rides along
// with the whole dataset on every sync.
const aiSpendKeepMonths = 12

// RecordAISpend books one AI call against the running meter. feature names what
// spent it ("assistant", "categorize", "receipt"); model is used to estimate the
// cost, and a model with no known pricing records its tokens with the cost marked
// unknown rather than as zero.
//
// A call with no tokens is ignored: a request that failed before reaching the
// provider cost nothing, and recording it would make the call count disagree with
// the bill.
func (a *App) RecordAISpend(feature, model string, usage ai.Usage) {
	if a == nil || usage.TotalTokens <= 0 {
		return
	}
	cost, hasCost := ai.EstimateCostUSD(model, usage)
	// Metering is a side effect of using AI, open to every role, while PutSettings reserves the
	// spend record for editors, so the ledger entry goes straight to the store.
	if err := a.ready(); err != nil {
		a.logErr("recordAISpend", err)
		return
	}
	a.logErr("recordAISpend", writeAISpend(a.SettingsChecked, a.store.PutSettings, aispend.Entry{
		Feature: feature,
		At:      time.Now(),
		Tokens:  usage.TotalTokens,
		CostUSD: cost,
		HasCost: hasCost,
	}))
}

// writeAISpend adds one entry to the spend record held in the settings that read returns and saves
// the result with write. A settings read that fails returns its error without calling write: the
// record built from an empty Settings would replace the household's currency, rates and caps with
// blanks.
func writeAISpend(read func() (store.Settings, error), write func(store.Settings) error, entry aispend.Entry) error {
	s, err := read()
	if err != nil {
		return err
	}
	ledger := aispend.NewLedger(s.AISpend)
	ledger.Record(entry)
	ledger.Trim(entry.At, aiSpendKeepMonths)
	s.AISpend = ledger.Buckets()
	return write(s)
}

// AISpendThisMonth summarises what has been spent so far this month.
func (a *App) AISpendThisMonth() aispend.Summary {
	if a == nil {
		return aispend.Summary{}
	}
	return aispend.NewLedger(a.Settings().AISpend).Month(time.Now())
}

// AISpendMonth summarises a particular month, for a history view.
func (a *App) AISpendMonth(month time.Time) aispend.Summary {
	if a == nil {
		return aispend.Summary{}
	}
	return aispend.NewLedger(a.Settings().AISpend).Month(month)
}

// AISpendCap returns the household's monthly ceiling in dollars, or 0 for none.
func (a *App) AISpendCap() float64 {
	if a == nil {
		return 0
	}
	return a.Settings().AISpendCapUSD
}

// SetAISpendCap sets or clears the monthly ceiling. A negative cap is treated as
// no cap rather than rejected — the field is a number box, and "less than nothing"
// is most usefully read as "no limit".
func (a *App) SetAISpendCap(usd float64) error {
	if a == nil {
		return nil
	}
	if usd < 0 {
		usd = 0
	}
	s := a.Settings()
	s.AISpendCapUSD = usd
	return a.PutSettings(s)
}
