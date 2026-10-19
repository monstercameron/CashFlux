// SPDX-License-Identifier: MIT

package unusualeval

import (
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/store"
)

//go:embed testdata/fixtures.json
var fixtureFiles embed.FS

type fixtureFile struct {
	ID              string       `json:"id"`
	Candidate       string       `json:"candidate"`
	Expected        []Signal     `json:"expected"`
	Evidence        []string     `json:"evidence"`
	HardNegativeFor []Signal     `json:"hard_negative_for,omitempty"`
	Rationale       string       `json:"rationale"`
	Merchant        string       `json:"merchant"`
	Period          string       `json:"period"`
	Split           Split        `json:"split"`
	Rows            []fixtureRow `json:"rows"`
}

type fixtureRow struct {
	ID       string         `json:"id"`
	Date     string         `json:"date"`
	Payee    string         `json:"payee"`
	Category string         `json:"category"`
	Amount   int64          `json:"amount"`
	Currency string         `json:"currency"`
	Transfer bool           `json:"transfer,omitempty"`
	Splits   []fixtureSplit `json:"splits,omitempty"`
}

type fixtureSplit struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
}

var referenceNow = time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC)

// Fixtures returns the checked-in labels paired with a fixed-date sample
// household snapshot and a fixture-specific set of synthetic rows.
func Fixtures() ([]Fixture, error) {
	data, err := fixtureFiles.ReadFile("testdata/fixtures.json")
	if err != nil {
		return nil, fmt.Errorf("unusualeval: read fixtures: %w", err)
	}
	var definitions []fixtureFile
	if err := json.Unmarshal(data, &definitions); err != nil {
		return nil, fmt.Errorf("unusualeval: decode fixtures: %w", err)
	}
	fixtures := make([]Fixture, 0, len(definitions))
	for _, definition := range definitions {
		fixture, err := buildFixture(definition)
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	return fixtures, nil
}

func buildFixture(def fixtureFile) (Fixture, error) {
	dataset := store.SampleDatasetAt(referenceNow)
	categories := make(map[string]bool, len(dataset.Categories))
	for _, category := range dataset.Categories {
		categories[category.ID] = true
	}
	for _, row := range def.Rows {
		date, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			return Fixture{}, fmt.Errorf("unusualeval: fixture %q row %q date: %w", def.ID, row.ID, err)
		}
		txn := domain.Transaction{
			ID: row.ID, AccountID: "eval-account", Date: date.UTC(), Payee: row.Payee,
			Desc: row.Payee, CategoryID: row.Category, Amount: money.New(row.Amount, row.Currency),
		}
		if row.Transfer {
			txn.TransferAccountID = "other-account"
		}
		for _, split := range row.Splits {
			txn.Splits = append(txn.Splits, domain.CategorySplit{CategoryID: split.Category, Amount: money.New(split.Amount, row.Currency)})
		}
		dataset.Transactions = append(dataset.Transactions, txn)
		if row.Category != "" && !categories[row.Category] {
			dataset.Categories = append(dataset.Categories, domain.Category{ID: row.Category, Name: row.Category})
			categories[row.Category] = true
		}
	}
	input := smartengine.Input{
		Now: referenceNow, Base: dataset.Settings.BaseCurrency,
		Rates:    currency.Rates{Base: dataset.Settings.BaseCurrency, Rates: dataset.Settings.FXRates},
		Accounts: dataset.Accounts, Transactions: dataset.Transactions, Categories: dataset.Categories,
		Budgets: dataset.Budgets, Goals: dataset.Goals, Recurring: dataset.Recurring,
		Members: dataset.Members, Tasks: dataset.Tasks, Aliases: dataset.PayeeAliases,
	}
	fixture := Fixture{
		ID: def.ID, Candidate: def.Candidate, Expected: append([]Signal(nil), def.Expected...),
		Evidence: append([]string(nil), def.Evidence...), HardNegativeFor: append([]Signal(nil), def.HardNegativeFor...),
		Rationale: def.Rationale, Merchant: def.Merchant, Period: def.Period, Split: def.Split, Input: input,
	}
	if err := ValidateFixture(fixture); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}
