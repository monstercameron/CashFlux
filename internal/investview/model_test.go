// SPDX-License-Identifier: MIT

package investview

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/portfolio"
)

func investmentAccount(id, name, code string, kind domain.AccountType, opening int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: kind, Currency: code, OpeningBalance: money.New(opening, code),
	}
}

func TestPreparePreservesEligibleHoldingsAllocationAndTotals(t *testing.T) {
	investment := investmentAccount("broker", "Brokerage", "USD", domain.TypeInvestment, 10000)
	traditional := investmentAccount("retirement", "Retirement", "USD", domain.TypeRetirement, 3000)
	archived := investmentAccount("archived", "Old account", "USD", domain.TypeCrypto, 9000)
	archived.Archived = true
	other := investmentAccount("checking", "Checking", "USD", domain.TypeChecking, 7000)
	accounts := []domain.Account{investment, traditional, archived, other}
	txns := []domain.Transaction{
		{ID: "broker-deposit", AccountID: "broker", Amount: money.New(500, "USD")},
		{ID: "retirement-fee", AccountID: "retirement", Amount: money.New(-500, "USD")},
		{ID: "checking-row", AccountID: "checking", Amount: money.New(999, "USD")},
	}
	holdings := []domain.Holding{
		{ID: "tech", AccountID: "broker", Ticker: "ABC", Shares: 2, CurrentPriceMinorPerShare: 1000, CostBasisMinor: 1500, AssetClass: "Stocks", Sector: "Technology", SecurityType: domain.SecurityETF, ExpenseRatioBps: 25},
		{ID: "unclassified", AccountID: "broker", Name: "Unknown bucket", Shares: 1, CurrentPriceMinorPerShare: 1000, CostBasisMinor: 1000, AssetClass: "Stocks"},
		{ID: "archived-position", AccountID: "archived", Shares: 10, CurrentPriceMinorPerShare: 1000},
		{ID: "unlinked-position", AccountID: "missing", Shares: 10, CurrentPriceMinorPerShare: 1000},
	}
	accountsBefore := append([]domain.Account(nil), accounts...)
	txnsBefore := append([]domain.Transaction(nil), txns...)
	holdingsBefore := append([]domain.Holding(nil), holdings...)

	got := Prepare(Input{Accounts: accounts, Transactions: txns, Holdings: holdings, Rates: currency.Rates{Base: "USD"}})
	if len(got.Accounts) != 2 || got.Accounts[0].ID != "broker" || got.Accounts[1].ID != "retirement" {
		t.Fatalf("active investment accounts = %+v", got.Accounts)
	}
	if len(got.Securities) != 2 || got.Securities[0].ID != "tech" || got.Securities[1].ID != "unclassified" {
		t.Fatalf("eligible securities = %+v", got.Securities)
	}
	if len(got.HoldingsByAccount["broker"]) != 2 || len(got.HoldingsByAccount["retirement"]) != 0 {
		t.Fatalf("holdings index = %#v", got.HoldingsByAccount)
	}
	if len(got.Traditional) != 1 || got.Traditional[0].ID != "retirement" {
		t.Fatalf("traditional accounts = %+v", got.Traditional)
	}
	if !got.TraditionalKnown || got.TraditionalValue.Amount != 2500 || !got.TotalKnown || got.TotalValue.Amount != 5500 {
		t.Fatalf("portfolio totals = traditional %v known=%v total %v known=%v err=%v", got.TraditionalValue, got.TraditionalKnown, got.TotalValue, got.TotalKnown, got.TotalErr)
	}
	if got.Summary.TotalValueMinor != 3000 || got.Summary.TotalCostMinor != 2500 {
		t.Fatalf("security summary = %+v", got.Summary)
	}
	if len(got.AllocationByClass) != 1 || got.AllocationByClass[0].Label != "Stocks" || got.AllocationByClass[0].ValueMinor != 3000 {
		t.Fatalf("asset-class allocation = %+v", got.AllocationByClass)
	}
	if len(got.AllocationBySector) != 2 || got.AllocationBySector[0].Label != "Technology" || got.AllocationBySector[1].Label != portfolio.Unclassified {
		t.Fatalf("sector allocation must preserve named + unclassified policy: %+v", got.AllocationBySector)
	}
	if !got.ReconciliationKnown || got.Reconciliation.AccountsTotalMinor != 13000 || got.Reconciliation.SecuritiesMinor != 3000 || got.Reconciliation.UntrackedMinor != 10000 {
		t.Fatalf("reconciliation = %+v known=%v err=%v", got.Reconciliation, got.ReconciliationKnown, got.ReconciliationErr)
	}
	if got.BalancesByAccount["broker"].Native.Amount != 10500 || got.BalanceByID["broker"].Amount != 10500 {
		t.Fatalf("broker balance fact = %+v", got.BalancesByAccount["broker"])
	}
	if !reflect.DeepEqual(accounts, accountsBefore) || !reflect.DeepEqual(txns, txnsBefore) || !reflect.DeepEqual(holdings, holdingsBefore) {
		t.Fatal("Prepare mutated an input slice")
	}
}

func TestPrepareKeepsBalanceFXAndSecurityFactsIndependent(t *testing.T) {
	foreign := investmentAccount("foreign", "Foreign brokerage", "EUR", domain.TypeInvestment, 1000)
	traditional := investmentAccount("cash", "Retirement cash", "USD", domain.TypeRetirement, 500)
	in := Input{
		Accounts:     []domain.Account{foreign, traditional},
		Transactions: []domain.Transaction{{ID: "deposit", AccountID: "foreign", Amount: money.New(200, "EUR")}},
		Holdings:     []domain.Holding{{ID: "position", AccountID: "foreign", Shares: 1, CurrentPriceMinorPerShare: 750}},
		Rates:        currency.Rates{Base: "USD"},
	}
	got := Prepare(in)
	foreignFact := got.BalancesByAccount["foreign"]
	if !foreignFact.NativeKnown() || foreignFact.Native.Amount != 1200 || foreignFact.BaseKnown() || foreignFact.BaseErr == nil {
		t.Fatalf("missing FX should preserve native balance only: %+v", foreignFact)
	}
	if !got.TraditionalKnown || !got.TotalKnown || got.TotalValue.Amount != 1250 {
		t.Fatalf("foreign securities must remain known independently of account FX: total=%v known=%v traditional=%v err=%v", got.TotalValue, got.TotalKnown, got.TraditionalKnown, got.TotalErr)
	}
	if got.ReconciliationKnown || got.ReconciliationErr == nil {
		t.Fatalf("reconciliation must disclose missing account FX: known=%v err=%v", got.ReconciliationKnown, got.ReconciliationErr)
	}

	// With no security holding, the same EUR account is balance-tracked. Its
	// failed conversion makes the traditional and combined totals unavailable,
	// while the unrelated USD account retains its own facts.
	in.Holdings = nil
	got = Prepare(in)
	if got.TraditionalKnown || got.TotalKnown || got.TraditionalErr == nil || got.TotalErr == nil {
		t.Fatalf("missing FX must not become a zero/raw-currency total: %+v", got)
	}
	if !got.BalancesByAccount["cash"].BaseKnown() || got.BalanceByID["cash"].Amount != 500 {
		t.Fatalf("healthy account lost independent base balance: %+v", got.BalancesByAccount["cash"])
	}
}

func TestPrepareRetainsHealthyAccountsWhenOneLedgerBalanceIsInvalid(t *testing.T) {
	bad := investmentAccount("bad", "Bad", "USD", domain.TypeRetirement, 100)
	good := investmentAccount("good", "Good", "USD", domain.TypeInvestment, 200)
	in := Input{
		Accounts: []domain.Account{bad, good},
		Transactions: []domain.Transaction{
			{ID: "wrong-currency", AccountID: "bad", Amount: money.New(1, "EUR")},
			{ID: "valid", AccountID: "good", Amount: money.New(50, "USD")},
		},
		Rates: currency.Rates{Base: "USD"},
	}
	got := Prepare(in)
	if got.BalancesByAccount["bad"].NativeKnown() || got.BalancesByAccount["bad"].BalanceErr == nil || got.BalancesByAccount["bad"].BaseKnown() {
		t.Fatalf("bad balance should be unavailable: %+v", got.BalancesByAccount["bad"])
	}
	if !got.BalancesByAccount["good"].BaseKnown() || got.BalancesByAccount["good"].Base.Amount != 250 {
		t.Fatalf("one invalid account poisoned a healthy balance: %+v", got.BalancesByAccount["good"])
	}
	if got.TotalKnown || got.ReconciliationKnown {
		t.Fatal("incomplete totals/reconciliation were reported as complete")
	}
}

func TestPrepareReportsOverflowInTraditionalAndReconciliationTotals(t *testing.T) {
	max := investmentAccount("max", "Maximum", "USD", domain.TypeRetirement, math.MaxInt64)
	one := investmentAccount("one", "One", "USD", domain.TypeInvestment, 1)
	got := Prepare(Input{Accounts: []domain.Account{max, one}, Rates: currency.Rates{Base: "USD"}})
	if got.TotalKnown || got.TraditionalKnown || got.TraditionalErr == nil || got.TotalErr == nil {
		t.Fatalf("overflow total should be unavailable: %+v", got)
	}
	if got.ReconciliationKnown || got.ReconciliationErr == nil {
		t.Fatalf("overflow reconciliation should be unavailable: %+v", got)
	}
}

type legacyModel struct {
	Securities         []domain.Holding
	Traditional        []domain.Account
	Balances           map[string]int64
	Summary            portfolio.Summary
	TraditionalTotal   int64
	AllocationByClass  []portfolio.Weight
	AllocationBySector []portfolio.Weight
	Reconciliation     portfolio.Reconciliation
}

// legacyPrepareReference mirrors the pre-extraction screen model on complete,
// same-currency inputs. The paired benchmark uses the same fixtures and excludes
// app-state cloning/rendering from both measurements.
func legacyPrepareReference(in Input) legacyModel {
	out := legacyModel{Balances: map[string]int64{}}
	var accounts []domain.Account
	for _, account := range in.Accounts {
		if !account.Archived && isInvestmentAccount(account.Type) {
			accounts = append(accounts, account)
		}
	}
	byAccount := map[string][]domain.Holding{}
	for _, holding := range in.Holdings {
		byAccount[holding.AccountID] = append(byAccount[holding.AccountID], holding)
	}
	accountSet := map[string]bool{}
	accountBalances := map[string]int64{}
	for _, account := range accounts {
		accountSet[account.ID] = true
		balance, _ := ledger.Balance(account, in.Transactions)
		converted, err := in.Rates.Convert(balance, in.Rates.Base)
		if err == nil {
			balance = converted
		}
		accountBalances[account.ID] = balance.Amount
		if len(byAccount[account.ID]) == 0 {
			out.Balances[account.ID] = balance.Amount
			out.Traditional = append(out.Traditional, account)
			out.TraditionalTotal += balance.Amount
		}
	}
	for _, holding := range in.Holdings {
		if accountSet[holding.AccountID] {
			out.Securities = append(out.Securities, holding)
		}
	}
	converted := portfolio.FromDomainSlice(out.Securities)
	out.Summary = portfolio.PortfolioSummary(converted)
	out.AllocationByClass = portfolio.AllocationByAssetClass(converted)
	out.AllocationBySector = portfolio.AllocationBySector(converted)
	values := make([]portfolio.AccountValue, 0, len(accounts))
	for _, account := range accounts {
		var securities int64
		for _, holding := range byAccount[account.ID] {
			securities += portfolio.HoldingValueMinor(portfolio.FromDomain(holding))
		}
		values = append(values, portfolio.AccountValue{AccountID: account.ID, Name: account.Name, BalanceMinor: accountBalances[account.ID], SecuritiesMinor: securities})
	}
	out.Reconciliation = portfolio.Reconcile(values)
	sort.SliceStable(out.Securities, func(i, j int) bool {
		return portfolio.HoldingValueMinor(portfolio.FromDomain(out.Securities[i])) > portfolio.HoldingValueMinor(portfolio.FromDomain(out.Securities[j]))
	})
	sort.SliceStable(out.Traditional, func(i, j int) bool {
		return out.Balances[out.Traditional[i].ID] > out.Balances[out.Traditional[j].ID]
	})
	return out
}

func benchmarkInput(accountCount, transactionCount int) Input {
	in := Input{Rates: currency.Rates{Base: "USD"}}
	in.Accounts = make([]domain.Account, accountCount)
	for i := range in.Accounts {
		kind := domain.TypeInvestment
		if i%2 == 1 {
			kind = domain.TypeRetirement
		}
		in.Accounts[i] = investmentAccount(fmt.Sprintf("acct-%02d", i), fmt.Sprintf("Account %02d", i), "USD", kind, 100000+int64(i*100))
	}
	in.Transactions = make([]domain.Transaction, transactionCount)
	for i := range in.Transactions {
		accountID := fmt.Sprintf("acct-%02d", i%accountCount)
		in.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("txn-%06d", i), AccountID: accountID, Amount: money.New(int64(i%7-3), "USD")}
	}
	for i := 0; i < accountCount; i += 2 {
		in.Holdings = append(in.Holdings, domain.Holding{ID: fmt.Sprintf("holding-%02d", i), AccountID: fmt.Sprintf("acct-%02d", i), Ticker: fmt.Sprintf("T%02d", i), Shares: 10, CurrentPriceMinorPerShare: 2500, CostBasisMinor: 20000, AssetClass: "Stocks", Sector: "Technology", SecurityType: domain.SecurityETF})
	}
	return in
}

var (
	benchmarkPreparedSink Model
	benchmarkLegacySink   legacyModel
)

// BenchmarkPreparePaired compares the previous screen preparation algorithm
// and the indexed native model over the same synthetic dataset. It measures
// preparation only; store snapshots and UI construction are intentionally out
// of both cases.
func BenchmarkPreparePaired(b *testing.B) {
	in := benchmarkInput(16, 12000)
	b.Run("previous-screen-reference", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkLegacySink = legacyPrepareReference(in)
		}
	})
	b.Run("indexed-native-model", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkPreparedSink = Prepare(in)
		}
	})
}

func TestPrepareMatchesLegacyReferenceOnOrdinaryFixture(t *testing.T) {
	in := benchmarkInput(8, 1200)
	got, old := Prepare(in), legacyPrepareReference(in)
	if !reflect.DeepEqual(got.Securities, old.Securities) || !reflect.DeepEqual(got.Traditional, old.Traditional) {
		t.Fatalf("eligible/sorted records differ from prior behavior: new securities=%+v old securities=%+v new traditional=%+v old traditional=%+v", got.Securities, old.Securities, got.Traditional, old.Traditional)
	}
	if got.Summary != old.Summary || got.TraditionalValue.Amount != old.TraditionalTotal || !reflect.DeepEqual(got.AllocationByClass, old.AllocationByClass) || !reflect.DeepEqual(got.AllocationBySector, old.AllocationBySector) || !reflect.DeepEqual(got.Reconciliation, old.Reconciliation) {
		t.Fatalf("prepared facts differ from prior behavior: new=%+v old=%+v", got, old)
	}
}
