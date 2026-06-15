package store

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SampleDataset returns a small, valid starter dataset for first run or the
// "load sample data" action. All ids are stable so re-loading is idempotent.
func SampleDataset() Dataset {
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC) }
	usd := func(n int64) money.Money { return money.New(n, "USD") }

	return Dataset{
		Members: []domain.Member{
			{ID: "sample-m1", Name: "You", IsDefault: true, Color: "#4ade80"},
		},
		Accounts: []domain.Account{
			{ID: "sample-checking", Name: "Checking", OwnerID: "sample-m1", Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(420000), BalanceAsOf: asOf},
			{ID: "sample-savings", Name: "Savings", OwnerID: "sample-m1", Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(1500000), BalanceAsOf: asOf},
			{ID: "sample-card", Name: "Credit Card", OwnerID: "sample-m1", Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-85000), BalanceAsOf: asOf, CreditLimit: usd(500000), InterestRateAPR: 19.99, DueDayOfMonth: 15, Lender: "Example Bank"},
		},
		Categories: []domain.Category{
			{ID: "cat-income", Name: "Income", Kind: domain.KindIncome, Color: "#22c55e"},
			{ID: "cat-housing", Name: "Housing", Kind: domain.KindExpense, Color: "#60a5fa"},
			{ID: "cat-food", Name: "Food", Kind: domain.KindExpense, Color: "#f59e0b"},
			{ID: "cat-transport", Name: "Transport", Kind: domain.KindExpense, Color: "#a78bfa"},
		},
		Transactions: []domain.Transaction{
			{ID: "tx-1", AccountID: "sample-checking", Date: day(1), Payee: "Employer", Desc: "Salary", CategoryID: "cat-income", Amount: usd(420000), MemberID: "sample-m1", Cleared: true},
			{ID: "tx-2", AccountID: "sample-checking", Date: day(2), Payee: "Landlord", Desc: "Rent", CategoryID: "cat-housing", Amount: usd(-150000), MemberID: "sample-m1", Cleared: true},
			{ID: "tx-3", AccountID: "sample-checking", Date: day(3), Payee: "Grocer", Desc: "Groceries", CategoryID: "cat-food", Amount: usd(-24055), MemberID: "sample-m1"},
			{ID: "tx-4", AccountID: "sample-checking", Date: day(5), Payee: "Fuel Co", Desc: "Fuel", CategoryID: "cat-transport", Amount: usd(-6020), MemberID: "sample-m1"},
		},
		Budgets: []domain.Budget{
			{ID: "bud-food", Name: "Food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: "cat-food", Period: domain.PeriodMonthly, Limit: usd(60000)},
		},
		Goals: []domain.Goal{
			{ID: "goal-trip", Name: "Vacation", Scope: domain.ScopeIndividual, OwnerID: "sample-m1", TargetAmount: usd(300000), CurrentAmount: usd(50000), TargetDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), AccountID: "sample-savings"},
		},
		Tasks: []domain.Task{
			{ID: "task-1", Title: "Pay credit card by the 15th", Status: domain.StatusOpen, Priority: domain.PriorityHigh, RelatedType: domain.RelatedAccount, RelatedID: "sample-card", MemberID: "sample-m1", Source: domain.SourceManual},
		},
		Settings: Settings{BaseCurrency: "USD"},
	}
}
