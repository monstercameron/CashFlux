// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(major float64) money.Money { return money.New(int64(major*100), "USD") }

func baseData(now time.Time) Data {
	return Data{
		Accounts: []domain.Account{
			{ID: "chk", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
				OpeningBalance: usd(1000), BalanceAsOf: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
		Transactions: []domain.Transaction{
			{ID: "in", AccountID: "chk", Date: now, Amount: usd(2000)},  // income
			{ID: "out", AccountID: "chk", Date: now, Amount: usd(-500)}, // expense
		},
		Rates: currency.Rates{Base: "USD", Rates: map[string]float64{}},
		Now:   now,
	}
}

func TestDerivedFinancialVars(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	v := Vars(baseData(now))

	if v["income"] != 2000 || v["expense"] != 500 {
		t.Fatalf("income/expense = %v/%v, want 2000/500", v["income"], v["expense"])
	}
	if v["cashflow_net"] != 1500 {
		t.Errorf("cashflow_net = %v, want 1500 (income - expense)", v["cashflow_net"])
	}
	// savings_rate = (2000-500)/2000*100 = 75
	if v["savings_rate"] != 75 {
		t.Errorf("savings_rate = %v, want 75", v["savings_rate"])
	}
	// Only a checking account → liquid_cash equals its balance (opening, no in-period
	// balance change before asOf; the two txns post on the 15th).
	if v["liquid_cash"] <= 0 {
		t.Errorf("liquid_cash = %v, want > 0", v["liquid_cash"])
	}
	// No recurring bills and no dated goals → bills_due and goal_needs are 0, so
	// safe_to_spend must equal liquid_cash (the composition identity).
	if v["bills_due"] != 0 || v["goal_needs"] != 0 {
		t.Errorf("bills_due/goal_needs = %v/%v, want 0/0", v["bills_due"], v["goal_needs"])
	}
	if v["safe_to_spend"] != v["liquid_cash"]-v["bills_due"]-v["goal_needs"] {
		t.Errorf("safe_to_spend (%v) != liquid_cash - bills_due - goal_needs (%v)",
			v["safe_to_spend"], v["liquid_cash"]-v["bills_due"]-v["goal_needs"])
	}
}

func TestEssentialMonthlyAtom(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := baseData(now)
	// $1,500/mo recurring rent commitment.
	d.Recurring = []domain.Recurring{
		{ID: "r", Label: "Rent", Amount: usd(-1500), Cadence: domain.CadenceMonthly, NextDue: now},
	}
	// Essential groceries $300 in each of the 3 prior whole months (Mar/Apr/May),
	// plus discretionary dining that must be excluded.
	d.Categories = []domain.Category{
		{ID: "groc", Name: "Groceries", CategoryClass: domain.ClassFixed},
		{ID: "fun", Name: "Dining", CategoryClass: domain.ClassFlex},
	}
	for m := time.March; m <= time.May; m++ {
		when := time.Date(2026, m, 10, 0, 0, 0, 0, time.UTC)
		d.Transactions = append(d.Transactions,
			domain.Transaction{ID: "g" + m.String(), AccountID: "chk", CategoryID: "groc", Date: when, Amount: usd(-300)},
			domain.Transaction{ID: "d" + m.String(), AccountID: "chk", CategoryID: "fun", Date: when, Amount: usd(-100)},
		)
	}
	v := Vars(d)
	// $1,500 fixed + $300 essential trailing avg = $1,800.
	if v["essential_monthly"] != 1800 {
		t.Fatalf("essential_monthly = %v, want 1800", v["essential_monthly"])
	}
}

func TestZeroIncomeSavingsRate(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := baseData(now)
	d.Transactions = []domain.Transaction{{ID: "out", AccountID: "chk", Date: now, Amount: usd(-500)}}
	v := Vars(d)
	if v["savings_rate"] != 0 {
		t.Errorf("zero-income savings_rate = %v, want 0 (guarded)", v["savings_rate"])
	}
}

func TestPeriodWindowOverridesMonth(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := baseData(now)
	// Add an income txn in May; a June-only window must exclude it.
	d.Transactions = append(d.Transactions,
		domain.Transaction{ID: "may", AccountID: "chk", Date: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Amount: usd(9999)})

	junOnly := Vars(d) // no window → month of Now (June) → excludes May
	if junOnly["income"] != 2000 {
		t.Errorf("month income = %v, want 2000 (May excluded)", junOnly["income"])
	}

	d.PeriodStart = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	d.PeriodEnd = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	wide := Vars(d) // May+June window includes the May income
	if wide["income"] != 2000+9999 {
		t.Errorf("windowed income = %v, want %v (May included)", wide["income"], 2000+9999)
	}
}

func TestMoleculesComposeFromAtoms(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	v := Vars(baseData(now))
	// net_worth molecule == its atoms.
	if v["net_worth"] != v["assets"]-v["liabilities"] {
		t.Errorf("net_worth (%v) != assets - liabilities (%v)", v["net_worth"], v["assets"]-v["liabilities"])
	}
}

func TestExplainAuditsDerivation(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	v := Vars(baseData(now))

	d, ok := Explain("net_worth", v, nil)
	if !ok || d.Kind != "molecule" {
		t.Fatalf("net_worth: %+v ok=%v", d, ok)
	}
	if d.Formula != "assets - liabilities" {
		t.Errorf("formula = %q, want 'assets - liabilities'", d.Formula)
	}
	if _, ok := d.Inputs["assets"]; !ok {
		t.Error("audit should list 'assets' as an input atom")
	}
	if _, ok := d.Inputs["liabilities"]; !ok {
		t.Error("audit should list 'liabilities' as an input atom")
	}
	if d.Value != v["net_worth"] {
		t.Errorf("audit value %v != %v", d.Value, v["net_worth"])
	}

	// An atom explains as a leaf with a source note.
	a, ok := Explain("assets", v, nil)
	if !ok || a.Kind != "atom" || a.Source == "" {
		t.Errorf("assets atom audit: %+v", a)
	}
}

func TestMoleculeOverride(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := baseData(now)
	// A persisted override redefines net_worth; Vars must use the stored formula.
	d.Molecules = []domain.Molecule{{Name: "net_worth", Formula: "assets * 2"}}
	v := Vars(d)
	if v["net_worth"] != v["assets"]*2 {
		t.Errorf("override not applied: net_worth=%v, assets=%v", v["net_worth"], v["assets"])
	}
}

func TestCustomFieldVars(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := baseData(now)
	// Two in-period txns carry a numeric "tip"; one out-of-period must be excluded.
	d.Transactions = []domain.Transaction{
		{ID: "t1", AccountID: "chk", Date: now, Amount: usd(100), Custom: map[string]any{"tip": 5.0}},
		{ID: "t2", AccountID: "chk", Date: now, Amount: usd(100), Custom: map[string]any{"tip": 3.0}},
		{ID: "t3", AccountID: "chk", Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Amount: usd(100), Custom: map[string]any{"tip": 99.0}},
	}
	// An account numeric field too.
	d.Accounts[0].Custom = map[string]any{"risk": 7.0}
	d.CustomDefs = []customfields.Def{
		{EntityType: "transaction", Key: "tip", Type: customfields.TypeNumber},
		{EntityType: "account", Key: "risk", Type: customfields.TypeNumber},
		{EntityType: "transaction", Key: "note", Type: customfields.TypeText}, // ignored (not numeric)
	}
	v := Vars(d)
	if v["cf_txn_tip"] != 8 { // 5 + 3, May's 99 excluded by period
		t.Errorf("cf_txn_tip = %v, want 8 (in-period sum)", v["cf_txn_tip"])
	}
	if v["cf_acct_risk"] != 7 {
		t.Errorf("cf_acct_risk = %v, want 7", v["cf_acct_risk"])
	}
	if _, ok := v["cf_txn_note"]; ok {
		t.Error("non-numeric custom field should not be exposed")
	}
	// CustomFieldNames lists the numeric ones.
	names := CustomFieldNames(d.CustomDefs)
	if len(names) != 2 {
		t.Errorf("CustomFieldNames = %v, want 2 numeric", names)
	}
}
