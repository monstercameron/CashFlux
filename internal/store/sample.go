package store

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// EmptyDataset returns a blank starter dataset for a brand-new workspace: a
// single default member and a base currency, but no accounts, transactions,
// budgets, goals, or categories — a clean slate to build up, in contrast to
// SampleDataset's demo data. Used when the user creates a new (non-duplicated)
// workspace, so it starts empty rather than re-seeding the sample.
func EmptyDataset() Dataset {
	return Dataset{
		Members:  []domain.Member{{ID: "m-you", Name: "You", IsDefault: true, Color: "#4ade80"}},
		Settings: Settings{BaseCurrency: "USD"},
	}
}

// SampleDataset returns a realistic starter dataset for first run or the "load
// sample data" action: the finances of Michael Brooks, a 46-year-old single
// homeowner. It carries three months of recurring activity (April–June 2026) so
// trends and charts have real history. All ids are stable so re-loading is
// idempotent.
//
// The model keeps liabilities (mortgage, auto loan, card) as static balances and
// treats their payments as categorized expenses — the same convention the rest of
// the app uses — while monthly transfers to savings and the brokerage move both
// legs, so balances and the net-worth trend actually change over the months.
func SampleDataset() Dataset {
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	// Opening balances are stated as of the eve of the modeled history; the
	// April–June transactions then carry each account to "today".
	asOf := date(2026, time.March, 31)

	const me = "sample-m1"
	const (
		checking = "sample-checking"
		savings  = "sample-savings"
		broker   = "sample-brokerage"
		home     = "sample-home"
		mortgage = "sample-mortgage"
		autoLoan = "sample-autoloan"
		card     = "sample-card"
	)
	const (
		catIncome    = "cat-income"
		catHousing   = "cat-housing"
		catUtilities = "cat-utilities"
		catGroceries = "cat-groceries"
		catDining    = "cat-dining"
		catTransport = "cat-transport"
		catInsurance = "cat-insurance"
		catHealth    = "cat-health"
		catSubs      = "cat-subscriptions"
		catShopping  = "cat-shopping"
	)

	// --- three months of recurring activity ---
	var txns []domain.Transaction
	add := func(t domain.Transaction) { txns = append(txns, t) }
	months := []time.Month{time.April, time.May, time.June}
	for i, m := range months {
		y := 2026
		tag := fmt.Sprintf("2026-%02d", int(m))
		// Past months are reconciled; the current month's later activity is still
		// pending (uncleared), which is what a real ledger looks like mid-month.
		cleared := func(d int) bool { return m != time.June || d <= 12 }
		v := int64(i) // small per-month variation so charts aren't flat

		txn := func(slot string, d int, acct, payee, desc, cat string, amt int64) {
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s", tag, slot), AccountID: acct, Date: date(y, m, d),
				Payee: payee, Desc: desc, CategoryID: cat, Amount: usd(amt), MemberID: me, Cleared: cleared(d),
			})
		}
		// Income.
		add(domain.Transaction{
			ID: fmt.Sprintf("tx-%s-salary", tag), AccountID: checking, Date: date(y, m, 1),
			Payee: "Helix Software", Desc: "Salary", CategoryID: catIncome, Amount: usd(640000), MemberID: me, Cleared: true,
		})
		// Monthly bills and living expenses (paid from checking).
		txn("mortgage", 1, checking, "Summit Mortgage", "Mortgage payment", catHousing, -195000)
		txn("gym", 3, checking, "Iron Works Gym", "Gym membership", catHealth, -4500)
		txn("subs", 5, checking, "Streaming bundle", "Subscriptions", catSubs, -3200)
		txn("grocery1", 6, checking, "Greenfield Market", "Groceries", catGroceries, -(31000 + v*1500))
		txn("electric", 8, checking, "Metro Power", "Electricity", catUtilities, -(17500 + v*900))
		txn("internet", 9, checking, "Fiberline", "Internet & phone", catUtilities, -12000)
		txn("fuel", 10, checking, "QuickFuel", "Gas", catTransport, -(5500 + v*400))
		txn("auto", 10, checking, "Summit Auto Finance", "Car payment", catTransport, -42000)
		txn("dining1", 12, checking, "Trattoria Nove", "Dinner out", catDining, -(9500 + v*1200))
		txn("insurance", 14, checking, "Beacon Insurance", "Auto insurance", catInsurance, -14500)
		txn("health", 16, checking, "Wellness Pharmacy", "Pharmacy", catHealth, -(4000 + v*800))
		txn("grocery2", 20, checking, "Greenfield Market", "Groceries", catGroceries, -(28500 + v*1000))
		txn("shopping", 22, checking, "Northside Goods", "Household & shopping", catShopping, -(12000 + v*3000))
		txn("dining2", 25, checking, "The Copper Kettle", "Weekend dinner", catDining, -(14000 - v*1000))
		// Transfers: build savings and the brokerage each month (both legs move).
		addTransfer := func(slot, dest, label string, amt int64) {
			d := 2
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-out", tag, slot), AccountID: checking, Date: date(y, m, d),
				Payee: label, Desc: label, Amount: usd(-amt), MemberID: me, TransferAccountID: dest, Cleared: cleared(d),
			})
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-in", tag, slot), AccountID: dest, Date: date(y, m, d),
				Payee: label, Desc: label, Amount: usd(amt), MemberID: me, TransferAccountID: checking, Cleared: cleared(d),
			})
		}
		addTransfer("xfersav", savings, "Transfer to savings", 50000)
		addTransfer("xferinv", broker, "Transfer to brokerage", 80000)
	}

	return Dataset{
		Members: []domain.Member{
			{ID: me, Name: "Michael Brooks", IsDefault: true, Color: "#4ade80"},
		},
		Accounts: []domain.Account{
			{ID: checking, Name: "Everyday Checking", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(480000), BalanceAsOf: asOf, LiquidityScore: 100, StabilityScore: 95, ExpectedReturnAPR: 0.5},
			{ID: savings, Name: "High-Yield Savings", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(2100000), BalanceAsOf: asOf, LiquidityScore: 90, StabilityScore: 98, ExpectedReturnAPR: 4.0},
			{ID: broker, Name: "Brokerage / 401(k)", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(18200000), BalanceAsOf: asOf, LiquidityScore: 55, StabilityScore: 55, ExpectedReturnAPR: 7.5},
			{ID: home, Name: "Home", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeOther, Currency: "USD", OpeningBalance: usd(42500000), BalanceAsOf: asOf, LiquidityScore: 5, StabilityScore: 85, ExpectedReturnAPR: 3.5},
			{ID: mortgage, Name: "Mortgage", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeMortgage, Currency: "USD", OpeningBalance: usd(-26900000), BalanceAsOf: asOf, InterestRateAPR: 5.25, DueDayOfMonth: 1, MinPayment: usd(195000), Lender: "Summit Mortgage"},
			{ID: autoLoan, Name: "Auto Loan", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-1500000), BalanceAsOf: asOf, InterestRateAPR: 6.4, DueDayOfMonth: 10, MinPayment: usd(42000), Lender: "Summit Auto Finance"},
			{ID: card, Name: "Credit Card", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-190000), BalanceAsOf: asOf, CreditLimit: usd(1200000), InterestRateAPR: 22.99, DueDayOfMonth: 18, MinPayment: usd(3500), Lender: "Beacon Bank"},
		},
		Categories: []domain.Category{
			{ID: catIncome, Name: "Income", Kind: domain.KindIncome, Color: "#22c55e"},
			{ID: catHousing, Name: "Housing", Kind: domain.KindExpense, Color: "#60a5fa"},
			{ID: catUtilities, Name: "Utilities", Kind: domain.KindExpense, Color: "#38bdf8"},
			{ID: catGroceries, Name: "Groceries", Kind: domain.KindExpense, Color: "#f59e0b"},
			{ID: catDining, Name: "Dining", Kind: domain.KindExpense, Color: "#fb923c"},
			{ID: catTransport, Name: "Transportation", Kind: domain.KindExpense, Color: "#a78bfa"},
			{ID: catInsurance, Name: "Insurance", Kind: domain.KindExpense, Color: "#f472b6"},
			{ID: catHealth, Name: "Health & Fitness", Kind: domain.KindExpense, Color: "#34d399"},
			{ID: catSubs, Name: "Subscriptions", Kind: domain.KindExpense, Color: "#c084fc"},
			{ID: catShopping, Name: "Shopping", Kind: domain.KindExpense, Color: "#e879f9"},
		},
		Transactions: txns,
		Budgets: []domain.Budget{
			{ID: "bud-groceries", Name: "Groceries", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catGroceries, Period: domain.PeriodMonthly, Limit: usd(70000)},
			{ID: "bud-dining", Name: "Dining", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catDining, Period: domain.PeriodMonthly, Limit: usd(30000)},
			{ID: "bud-transport", Name: "Transportation", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catTransport, Period: domain.PeriodMonthly, Limit: usd(55000)},
			{ID: "bud-shopping", Name: "Shopping", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catShopping, Period: domain.PeriodMonthly, Limit: usd(25000)},
			{ID: "bud-subs", Name: "Subscriptions", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catSubs, Period: domain.PeriodMonthly, Limit: usd(5000)},
		},
		Goals: []domain.Goal{
			{ID: "goal-emergency", Name: "Emergency fund", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(3000000), CurrentAmount: usd(2250000), TargetDate: date(2026, time.December, 31), AccountID: savings},
			{ID: "goal-retirement", Name: "Retirement nest egg", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(25000000), CurrentAmount: usd(18440000), TargetDate: date(2031, time.January, 1), AccountID: broker},
			{ID: "goal-car", Name: "New car fund", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(2000000), CurrentAmount: usd(300000), TargetDate: date(2027, time.June, 1)},
		},
		Tasks: []domain.Task{
			{ID: "task-card", Title: "Pay credit card by the 18th", Status: domain.StatusOpen, Priority: domain.PriorityHigh, RelatedType: domain.RelatedAccount, RelatedID: card, MemberID: me, Source: domain.SourceManual},
			{ID: "task-401k", Title: "Bump 401(k) contribution to 15%", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedAccount, RelatedID: broker, MemberID: me, Source: domain.SourceManual},
			{ID: "task-physical", Title: "Schedule annual physical", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: me, Source: domain.SourceManual},
		},
		Settings: Settings{BaseCurrency: "USD"},
	}
}
