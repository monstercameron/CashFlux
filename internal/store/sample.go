// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/workflow"
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

// SampleDataset returns a realistic two-year starter dataset for first run or the
// "load sample data" action: the finances of Daniel Carter, a single 35-year-old
// renter earning the US-average ~$60k salary with a few side incomes (freelance,
// dividends, resale). It carries 24 months of recurring activity (July 2024 –
// June 2026) so every trend, chart, report, and forecast has real history, and it
// deliberately exercises **every** feature surface: sub-categories, transfers,
// splits, tags, custom fields, rules, workflows + run history, budgets (all
// periods + rollover), goals, tasks, recurring schedules, plans, allocation
// profiles, formulas, documents, artifacts, a custom page, shared expenses +
// settlements, and rich settings (FX table, freshness overrides, payoff baseline).
//
// Liabilities (credit card, student loan) are kept as static balances and their
// payments are categorized expenses — the convention the rest of the app uses —
// while the monthly transfers to savings, the Roth IRA, and the 401(k)/brokerage
// move both legs, so balances and the net-worth trend actually change over time.
// All ids are stable so re-loading is idempotent.
func SampleDataset() Dataset {
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	// Opening balances are stated as of the eve of the modeled history; the 24
	// months of transactions then carry each account to "today" (mid-June 2026).
	asOf := date(2024, time.June, 30)
	// Activity on/before this date is reconciled; later (current-month) activity is
	// still pending, which is what a real ledger looks like mid-month.
	clearedAsOf := date(2026, time.June, 15)

	const (
		me   = "m-daniel"
		room = "m-jordan"
	)
	const (
		checking = "acct-checking"
		hysa     = "acct-hysa"
		k401     = "acct-401k"
		roth     = "acct-roth"
		cash     = "acct-cash"
		cd       = "acct-cd"
		card     = "acct-card"
		sloan    = "acct-studentloan"
		oldSav   = "acct-old-savings"
	)
	const (
		// Income
		catSalary    = "cat-salary"
		catFreelance = "cat-freelance"
		catDividends = "cat-dividends"
		catOtherInc  = "cat-other-income"
		// Expense parents
		catHousing   = "cat-housing"
		catUtilities = "cat-utilities"
		catGroceries = "cat-groceries"
		catDining    = "cat-dining"
		catTransport = "cat-transport"
		catInsurance = "cat-insurance"
		catHealth    = "cat-health"
		catSubs      = "cat-subscriptions"
		catShopping  = "cat-shopping"
		catEntertain = "cat-entertainment"
		catEducation = "cat-education"
		catGifts     = "cat-gifts"
		catTravel    = "cat-travel"
		catFees      = "cat-fees"
		// Expense sub-categories (nested, to exercise the category tree)
		catElectric = "cat-electricity"
		catInternet = "cat-internet"
		catGas      = "cat-gas"
		catTransit  = "cat-transit"
	)

	var txns []domain.Transaction
	add := func(t domain.Transaction) { txns = append(txns, t) }
	cleared := func(d time.Time) bool { return !d.After(clearedAsOf) }

	// --- 24 months of recurring activity (2024-07 .. 2026-06) ---
	start := date(2024, time.July, 1)
	for i := 0; i < 24; i++ {
		ym := start.AddDate(0, i, 0)
		y, m := ym.Year(), ym.Month()
		tag := ym.Format("2006-01")
		v := int64(i % 6) // bounded per-month variation so charts aren't flat

		txn := func(slot string, d int, acct, payee, desc, cat string, amt int64) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s", tag, slot), AccountID: acct, Date: dt,
				Payee: payee, Desc: desc, CategoryID: cat, Amount: usd(amt), MemberID: me, Cleared: cleared(dt),
			})
		}
		addTransfer := func(slot, dest, label string, amt int64) {
			dt := date(y, m, 2)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-out", tag, slot), AccountID: checking, Date: dt,
				Payee: label, Desc: label, Amount: usd(-amt), MemberID: me, TransferAccountID: dest, Cleared: cleared(dt),
			})
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-in", tag, slot), AccountID: dest, Date: dt,
				Payee: label, Desc: label, Amount: usd(amt), MemberID: me, TransferAccountID: checking, Cleared: cleared(dt),
			})
		}

		// Income: monthly net salary, plus variable side incomes.
		txn("salary", 1, checking, "Northwind Logistics", "Paycheck (net)", catSalary, 360000)
		if i%2 == 0 { // freelance web-design gigs every other month
			dt := date(y, m, 15)
			add(domain.Transaction{
				ID: "tx-" + tag + "-freelance", AccountID: checking, Date: dt, Payee: "Brightside Studio",
				Desc: "Freelance web project", CategoryID: catFreelance, Amount: usd(40000 + v*8000),
				MemberID: me, Cleared: cleared(dt), Tags: []string{"business"},
				Custom: map[string]any{"reimbursable": false, "project": "Freelance"},
			})
		}
		if int(m)%3 == 0 { // quarterly brokerage dividends
			txn("dividends", 20, k401, "Vanguard", "Quarterly dividends", catDividends, 13000+v*900)
		}
		if i%5 == 2 { // occasional resale income
			txn("resale", 8, checking, "eBay", "Sold old gear", catOtherInc, 6000+v*2000)
		}

		// Housing & fixed bills.
		txn("rent", 1, checking, "Maple Court Apartments", "Rent", catHousing, -145000)
		txn("electric", 8, checking, "Metro Power", "Electricity", catElectric, -(8500 + v*900))
		txn("internet", 9, checking, "Fiberline", "Internet", catInternet, -7000)
		txn("phone", 9, checking, "CellOne", "Phone", catUtilities, -5500)
		txn("gym", 3, checking, "Iron Works Gym", "Gym membership", catHealth, -4000)
		txn("subs", 5, checking, "Streaming & apps", "Subscriptions", catSubs, -3000)
		txn("studentloan", 5, checking, "EdFinance Servicing", "Student loan payment", catEducation, -28000)

		// Variable living expenses.
		txn("grocery1", 6, checking, "Greenfield Market", "Groceries", catGroceries, -(21000 + v*1500))
		txn("grocery2", 20, checking, "Greenfield Market", "Groceries", catGroceries, -(18500 + v*1000))
		txn("dining", 12, checking, "Trattoria Nove", "Dinner out", catDining, -(9500 + v*1200))
		txn("gas", 10, checking, "Shell", "Gas", catGas, -(5500 + v*400))
		txn("health", 16, checking, "Wellness Pharmacy", "Pharmacy", catHealth, -(3000 + v*700))
		txn("shopping", 22, checking, "Northside Goods", "Household & shopping", catShopping, -(9000 + v*2500))
		txn("fun", 18, checking, "Cineplex", "Movies & fun", catEntertain, -(4500 + v*1200))
		if int(m)%2 == 0 {
			txn("transit", 11, checking, "City Transit", "Transit pass", catTransit, -9600)
		}
		// A couple of uncategorized coffee runs each month — gives the rules engine
		// ("coffee" → Dining) and "Apply rules" something real to do.
		coffee := func(slot string, d int) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: "tx-" + tag + "-" + slot, AccountID: checking, Date: dt,
				Payee: "Blue Bottle Coffee", Desc: "Coffee", Amount: usd(-(550 + v*40)), MemberID: me, Cleared: cleared(dt),
			})
		}
		coffee("coffee1", 4)
		coffee("coffee2", 17)
		// An occasional large purchase (> $200) to trip the "flag large purchases" workflow.
		if i%4 == 0 {
			dt := date(y, m, 14)
			add(domain.Transaction{
				ID: "tx-" + tag + "-big", AccountID: card, Date: dt, Payee: "Best Buy",
				Desc: "Electronics", CategoryID: catShopping, Amount: usd(-(30000 + v*6000)),
				MemberID: me, Cleared: cleared(dt), Tags: []string{"big-purchase"},
			})
		}

		// Transfers that build wealth (both legs move).
		addTransfer("xfer-hysa", hysa, "Transfer to savings", 30000)
		addTransfer("xfer-roth", roth, "Transfer to Roth IRA", 30000)
		addTransfer("xfer-401k", k401, "Transfer to 401(k)", 20000)
	}

	// --- one-off events across the two years (variety for reports/charts) ---
	add(domain.Transaction{ID: "tx-bonus-2024-12", AccountID: checking, Date: date(2024, time.December, 20), Payee: "Northwind Logistics", Desc: "Year-end bonus", CategoryID: catSalary, Amount: usd(250000), MemberID: me, Cleared: true, Tags: []string{"bonus"}})
	add(domain.Transaction{ID: "tx-refund-2025-04", AccountID: checking, Date: date(2025, time.April, 12), Payee: "IRS", Desc: "Tax refund", CategoryID: catOtherInc, Amount: usd(90000), MemberID: me, Cleared: true})
	add(domain.Transaction{ID: "tx-medical-2025-09", AccountID: card, Date: date(2025, time.September, 9), Payee: "City Medical Group", Desc: "Doctor visit", CategoryID: catHealth, Amount: usd(-60000), MemberID: me, Cleared: true, Tags: []string{"reimbursable"}, Custom: map[string]any{"reimbursable": true, "project": "Personal"}})
	add(domain.Transaction{ID: "tx-gift-2025-12", AccountID: checking, Date: date(2025, time.December, 18), Payee: "Various", Desc: "Holiday gifts", CategoryID: catGifts, Amount: usd(-35000), MemberID: me, Cleared: true})
	// A trip (Travel), tagged, with two legs.
	add(domain.Transaction{ID: "tx-trip-flight-2025-07", AccountID: card, Date: date(2025, time.July, 5), Payee: "SkyJet", Desc: "Flights", CategoryID: catTravel, Amount: usd(-62000), MemberID: me, Cleared: true, Tags: []string{"vacation"}})
	add(domain.Transaction{ID: "tx-trip-hotel-2025-07", AccountID: card, Date: date(2025, time.July, 6), Payee: "Seaside Resort", Desc: "Hotel", CategoryID: catTravel, Amount: usd(-88000), MemberID: me, Cleared: true, Tags: []string{"vacation"}})
	// A laptop purchase that links to an imported receipt document + a stored artifact.
	add(domain.Transaction{ID: "tx-laptop-2025-11", AccountID: card, Date: date(2025, time.November, 14), Payee: "Best Buy", Desc: "Laptop (work)", CategoryID: catShopping, Amount: usd(-140000), MemberID: me, Cleared: true, Tags: []string{"business", "big-purchase"}, SourceDocID: "doc-receipt", Attachments: []domain.AttachmentRef{{ArtifactID: "art-receipt", Name: "bestbuy-receipt.png", Kind: "image", MIME: "image/png"}}, Custom: map[string]any{"reimbursable": true, "project": "Freelance"}})
	// A Costco run split across two categories (exercises CategorySplit).
	add(domain.Transaction{ID: "tx-costco-2026-02", AccountID: checking, Date: date(2026, time.February, 15), Payee: "Costco", Desc: "Costco run", Amount: usd(-24000), MemberID: me, Cleared: true, Splits: []domain.CategorySplit{{CategoryID: catGroceries, Amount: usd(-16000)}, {CategoryID: catShopping, Amount: usd(-8000)}}})
	// A shared dinner with the roommate (settles via the SharedExpense ledger).
	add(domain.Transaction{ID: "tx-dinner-shared-2026-05", AccountID: card, Date: date(2026, time.May, 24), Payee: "Nobu", Desc: "Dinner with Jordan", CategoryID: catDining, Amount: usd(-9000), MemberID: me, Cleared: true})

	// --- Jordan Lee's attributed transactions (L16/L2) ---
	// A handful of transactions charged to the shared card and attributed to the
	// roommate — enough that per-member Reports and the Split/Settle-up screens
	// demonstrate multi-member behaviour out of the box.
	add(domain.Transaction{ID: "tx-jordan-rent-2026-04", AccountID: checking, Date: date(2026, time.April, 1), Payee: "Maple Court Apartments", Desc: "Jordan's half of rent", CategoryID: catHousing, Amount: usd(-72500), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-grocery-2026-04", AccountID: checking, Date: date(2026, time.April, 8), Payee: "Greenfield Market", Desc: "Jordan's groceries", CategoryID: catGroceries, Amount: usd(-18000), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-dining-2026-04", AccountID: card, Date: date(2026, time.April, 14), Payee: "Trattoria Nove", Desc: "Jordan's dinner out", CategoryID: catDining, Amount: usd(-7500), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-rent-2026-05", AccountID: checking, Date: date(2026, time.May, 1), Payee: "Maple Court Apartments", Desc: "Jordan's half of rent", CategoryID: catHousing, Amount: usd(-72500), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-grocery-2026-05", AccountID: checking, Date: date(2026, time.May, 9), Payee: "Greenfield Market", Desc: "Jordan's groceries", CategoryID: catGroceries, Amount: usd(-19500), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-subs-2026-05", AccountID: checking, Date: date(2026, time.May, 5), Payee: "Streaming & apps", Desc: "Jordan's share of subscriptions", CategoryID: catSubs, Amount: usd(-1500), MemberID: room, Cleared: true})
	add(domain.Transaction{ID: "tx-jordan-rent-2026-06", AccountID: checking, Date: date(2026, time.June, 1), Payee: "Maple Court Apartments", Desc: "Jordan's half of rent", CategoryID: catHousing, Amount: usd(-72500), MemberID: room, Cleared: false})
	add(domain.Transaction{ID: "tx-jordan-pharmacy-2026-06", AccountID: card, Date: date(2026, time.June, 10), Payee: "Wellness Pharmacy", Desc: "Jordan's pharmacy run", CategoryID: catHealth, Amount: usd(-4200), MemberID: room, Cleared: false})

	tinyPNG := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}

	return Dataset{
		Members: []domain.Member{
			{ID: me, Name: "Daniel Carter", IsDefault: true, Color: "#4ade80", Custom: map[string]any{}},
			{ID: room, Name: "Jordan Lee (roommate)", Color: "#60a5fa"},
		},
		Accounts: []domain.Account{
			{ID: checking, Name: "Everyday Checking", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(280000), BalanceAsOf: asOf, LiquidityScore: 100, StabilityScore: 95, ExpectedReturnAPR: 0.1, Custom: map[string]any{"last4": "4821"}},
			{ID: hysa, Name: "Emergency Savings (HYSA)", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(500000), BalanceAsOf: asOf, LiquidityScore: 90, StabilityScore: 98, ExpectedReturnAPR: 4.3},
			{ID: k401, Name: "401(k) / Brokerage", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(3900000), BalanceAsOf: asOf, LiquidityScore: 40, StabilityScore: 55, ExpectedReturnAPR: 7.5},
			{ID: roth, Name: "Roth IRA", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(1100000), BalanceAsOf: asOf, LiquidityScore: 45, StabilityScore: 60, ExpectedReturnAPR: 7.0},
			{ID: cash, Name: "Cash Wallet", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: usd(20000), BalanceAsOf: asOf, LiquidityScore: 100, StabilityScore: 80},
			{ID: cd, Name: "12-month CD", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(500000), BalanceAsOf: asOf, LiquidityScore: 20, StabilityScore: 99, ExpectedReturnAPR: 5.0, LockUntil: date(2026, time.December, 31)},
			{ID: card, Name: "Rewards Credit Card", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-120000), BalanceAsOf: asOf, CreditLimit: usd(800000), InterestRateAPR: 23.99, DueDayOfMonth: 22, MinPayment: usd(3500), Lender: "Beacon Bank"},
			{ID: sloan, Name: "Student Loan", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-1800000), BalanceAsOf: asOf, InterestRateAPR: 5.5, DueDayOfMonth: 5, MinPayment: usd(28000), Lender: "EdFinance Servicing"},
			{ID: oldSav, Name: "Old Savings (closed)", OwnerID: me, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(0), BalanceAsOf: asOf, Archived: true},
		},
		Categories: []domain.Category{
			{ID: catSalary, Name: "Salary", Kind: domain.KindIncome, Color: "#22c55e"},
			{ID: catFreelance, Name: "Freelance", Kind: domain.KindIncome, Color: "#16a34a"},
			{ID: catDividends, Name: "Dividends", Kind: domain.KindIncome, Color: "#4ade80"},
			{ID: catOtherInc, Name: "Other income", Kind: domain.KindIncome, Color: "#86efac"},
			{ID: catHousing, Name: "Housing", Kind: domain.KindExpense, Color: "#60a5fa"},
			{ID: catUtilities, Name: "Utilities", Kind: domain.KindExpense, Color: "#38bdf8"},
			{ID: catElectric, Name: "Electricity", Kind: domain.KindExpense, Color: "#0ea5e9", ParentID: catUtilities},
			{ID: catInternet, Name: "Internet", Kind: domain.KindExpense, Color: "#0284c7", ParentID: catUtilities},
			{ID: catGroceries, Name: "Groceries", Kind: domain.KindExpense, Color: "#f59e0b"},
			{ID: catDining, Name: "Dining", Kind: domain.KindExpense, Color: "#fb923c"},
			{ID: catTransport, Name: "Transportation", Kind: domain.KindExpense, Color: "#a78bfa"},
			{ID: catGas, Name: "Gas", Kind: domain.KindExpense, Color: "#8b5cf6", ParentID: catTransport},
			{ID: catTransit, Name: "Transit", Kind: domain.KindExpense, Color: "#7c3aed", ParentID: catTransport},
			{ID: catInsurance, Name: "Insurance", Kind: domain.KindExpense, Color: "#f472b6"},
			{ID: catHealth, Name: "Health & Fitness", Kind: domain.KindExpense, Color: "#34d399"},
			{ID: catSubs, Name: "Subscriptions", Kind: domain.KindExpense, Color: "#c084fc"},
			{ID: catShopping, Name: "Shopping", Kind: domain.KindExpense, Color: "#e879f9"},
			{ID: catEntertain, Name: "Entertainment", Kind: domain.KindExpense, Color: "#f87171"},
			{ID: catEducation, Name: "Education & Loans", Kind: domain.KindExpense, Color: "#fbbf24"},
			{ID: catGifts, Name: "Gifts & Charity", Kind: domain.KindExpense, Color: "#fda4af"},
			{ID: catTravel, Name: "Travel", Kind: domain.KindExpense, Color: "#2dd4bf"},
			{ID: catFees, Name: "Fees & Charges", Kind: domain.KindExpense, Color: "#94a3b8"},
		},
		Transactions: txns,
		Budgets: []domain.Budget{
			{ID: "bud-groceries", Name: "Groceries", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catGroceries, Period: domain.PeriodMonthly, Limit: usd(45000), Rollover: true},
			{ID: "bud-dining", Name: "Dining", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catDining, Period: domain.PeriodMonthly, Limit: usd(25000)},
			{ID: "bud-transport", Name: "Transportation", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catTransport, Period: domain.PeriodMonthly, Limit: usd(22000)},
			{ID: "bud-shopping", Name: "Shopping", Scope: domain.ScopeIndividual, OwnerID: me, CategoryID: catShopping, Period: domain.PeriodMonthly, Limit: usd(20000)},
			{ID: "bud-subs", Name: "Subscriptions", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catSubs, Period: domain.PeriodMonthly, Limit: usd(4000)},
			{ID: "bud-fun", Name: "Entertainment", Scope: domain.ScopeIndividual, OwnerID: me, CategoryID: catEntertain, Period: domain.PeriodWeekly, Limit: usd(2500)},
			{ID: "bud-gifts", Name: "Gifts & Charity", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catGifts, Period: domain.PeriodQuarterly, Limit: usd(40000)},
		},
		Goals: []domain.Goal{
			{ID: "goal-emergency", Name: "6-month emergency fund", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(2000000), CurrentAmount: usd(1220000), TargetDate: date(2027, time.March, 1), AccountID: hysa},
			{ID: "goal-roth", Name: "Max out Roth IRA", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(2000000), CurrentAmount: usd(1820000), TargetDate: date(2026, time.December, 31), AccountID: roth},
			{ID: "goal-vacation", Name: "Japan trip", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(500000), CurrentAmount: usd(180000), TargetDate: date(2027, time.April, 1)},
			{ID: "goal-car", Name: "Car down payment", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(600000), CurrentAmount: usd(120000), TargetDate: date(2027, time.September, 1)},
			{ID: "goal-debtfree", Name: "Pay off student loan", Scope: domain.ScopeIndividual, OwnerID: me, TargetAmount: usd(1800000), CurrentAmount: usd(400000), TargetDate: date(2029, time.June, 1)},
		},
		Tasks: []domain.Task{
			{ID: "task-card", Title: "Pay credit card before the 22nd", Notes: "Autopay covers the minimum; pay the statement balance.", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.June, 22), RelatedType: domain.RelatedAccount, RelatedID: card, MemberID: me, Source: domain.SourceManual},
			{ID: "task-roth", Title: "Top up Roth IRA to hit the annual max", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.December, 15), RelatedType: domain.RelatedGoal, RelatedID: "goal-roth", MemberID: me, Source: domain.SourceManual},
			{ID: "task-grocery-budget", Title: "Groceries trending over budget — review", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedBudget, RelatedID: "bud-groceries", MemberID: me, Source: domain.SourceAI},
			{ID: "task-stale-401k", Title: "Update 401(k) balance (stale)", Status: domain.StatusOpen, Priority: domain.PriorityLow, RelatedType: domain.RelatedAccount, RelatedID: k401, MemberID: me, Source: domain.SourceNudge},
			{ID: "task-overdue-physical", Title: "Schedule annual physical", Status: domain.StatusOpen, Priority: domain.PriorityLow, Due: date(2026, time.May, 1), MemberID: me, Source: domain.SourceManual},
			{ID: "task-done-refi", Title: "Refinanced student loan", Status: domain.StatusDone, Priority: domain.PriorityMedium, RelatedType: domain.RelatedAccount, RelatedID: sloan, MemberID: me, Source: domain.SourceManual},
		},
		CustomFields: []customfields.Def{
			{ID: "cf-txn-reimbursable", EntityType: "transaction", Key: "reimbursable", Label: "Reimbursable", Type: customfields.TypeBool},
			{ID: "cf-txn-project", EntityType: "transaction", Key: "project", Label: "Project", Type: customfields.TypeSelect, Options: []string{"Personal", "Freelance", "Side hustle"}},
			{ID: "cf-acct-last4", EntityType: "account", Key: "last4", Label: "Account number (last 4)", Type: customfields.TypeText},
		},
		Rules: []rules.Rule{
			{ID: "rule-coffee", Match: "coffee", SetCategoryID: catDining, SetTags: []string{"coffee"}},
			{ID: "rule-shell", Match: "shell", SetCategoryID: catGas},
			{ID: "rule-greenfield", Match: "greenfield", SetCategoryID: catGroceries},
			{ID: "rule-streaming", Match: "streaming", SetCategoryID: catSubs},
			{ID: "rule-transit", Match: "city transit", SetCategoryID: catTransit},
		},
		Documents: []domain.Document{
			{ID: "doc-statement", Filename: "checking-2026-05.csv", Kind: domain.DocCSV, UploadedAt: date(2026, time.June, 1), AccountID: checking, MemberID: me, Status: domain.DocImported, Extracted: []domain.DocumentRow{
				{Date: "2026-05-06", Description: "Greenfield Market", Amount: "-214.30", Category: "Groceries"},
				{Date: "2026-05-12", Description: "Trattoria Nove", Amount: "-95.00", Category: "Dining"},
			}},
			{ID: "doc-receipt", Filename: "bestbuy-receipt.png", Kind: domain.DocImage, UploadedAt: date(2025, time.November, 14), AccountID: card, MemberID: me, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{
				{Date: "2025-11-14", Description: "Best Buy — Laptop", Amount: "-1400.00", Category: "Shopping"},
			}},
			{ID: "doc-pending", Filename: "fuel-receipt.jpg", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 10), MemberID: me, Status: domain.DocPending},
		},
		SavedInsights: []domain.SavedInsight{
			{ID: "insight-savings-rate", Text: "You're saving about 20% of take-home pay across the last year — above the 15% rule of thumb. The biggest lever to push it higher is dining, which trends ~$50/mo over budget.", CreatedAt: date(2026, time.May, 2)},
			{ID: "insight-runway", Text: "Your emergency fund covers roughly 3.5 months of expenses. Reaching the 6-month goal by March 2027 needs about $215/mo — your current $300/mo transfer is on track.", CreatedAt: date(2026, time.June, 5)},
		},
		Recurring: []domain.Recurring{
			{ID: "rec-salary", Label: "Paycheck (net)", Amount: usd(360000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 1), AccountID: checking, CategoryID: catSalary, Autopost: true},
			{ID: "rec-rent", Label: "Rent", Amount: usd(-145000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 1), AccountID: checking, CategoryID: catHousing, Autopost: true},
			{ID: "rec-subs", Label: "Streaming & apps", Amount: usd(-3000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 5), AccountID: checking, CategoryID: catSubs},
			{ID: "rec-insurance", Label: "Car insurance", Amount: usd(-36000), Cadence: domain.CadenceQuarterly, NextDue: date(2026, time.September, 1), AccountID: checking, CategoryID: catInsurance},
			{ID: "rec-domain", Label: "Domain & hosting", Amount: usd(-9000), Cadence: domain.CadenceYearly, NextDue: date(2027, time.January, 15), AccountID: card, CategoryID: catSubs},
			{ID: "rec-gym", Label: "Gym membership", Amount: usd(-4000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 3), AccountID: checking, CategoryID: catHealth},
		},
		AllocProfiles: []domain.AllocationProfile{
			{ID: "alloc-growth", Name: "Aggressive growth", Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 1, GoalProgress: 2},
			{ID: "alloc-balanced", Name: "Balanced", Returns: 2, Stability: 2, Liquidity: 2, DebtReduction: 2, GoalProgress: 2},
			{ID: "alloc-debt", Name: "Crush debt first", Returns: 1, Stability: 1, Liquidity: 1, DebtReduction: 4, GoalProgress: 1},
		},
		Formulas: []domain.Formula{
			{ID: "formula-savings-rate", Name: "Savings rate %", Expr: "(income - expense) / income * 100", Enabled: true},
			{ID: "formula-debt-ratio", Name: "Debt-to-assets %", Expr: "liabilities / assets * 100", Enabled: true},
			{ID: "formula-runway", Name: "Runway (months)", Expr: "assets / expense", Enabled: true},
		},
		Plans: []domain.Plan{
			{ID: "plan-house", Name: "House down payment in 3 years", HorizonMonths: 36, StartBalance: 6300000, Items: []domain.PlanItem{
				{ID: "pi-save", Label: "Monthly saving", Kind: domain.PlanItemRecurring, Amount: 80000},
				{ID: "pi-bonus", Label: "Annual bonus", Kind: domain.PlanItemOneTime, Amount: 250000, Month: 12},
				{ID: "pi-down", Label: "Down payment", Kind: domain.PlanItemOneTime, Amount: -4000000, Month: 36},
			}},
			{ID: "plan-payoff", Name: "Extra $200/mo to student loan", HorizonMonths: 24, StartBalance: 6300000, Items: []domain.PlanItem{
				{ID: "pi-extra", Label: "Extra loan payment", Kind: domain.PlanItemRecurring, Amount: -20000},
			}},
		},
		CustomPages: []domain.CustomPage{
			{
				ID: "page-side-hustle", Slug: "side-hustle", Name: "Side hustle", Icon: "briefcase", Order: 0,
				CreatedAt: date(2025, time.January, 10),
				Layout: []dashlayout.Item{
					{ID: "w-surplus", ColSpan: 1, RowSpan: 1},
					{ID: "w-freelance", ColSpan: 3, RowSpan: 2},
				},
				Widgets: []domain.PageWidget{
					{ID: "w-surplus", Type: "kpi", Title: "Monthly surplus", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{Expr: "income - expense"}},
					{ID: "w-freelance", Type: "list", Title: "Freelance & business", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{Source: "transactions", Filter: "tag:business", Columns: []string{"date", "payee", "amount"}}},
				},
			},
		},
		Artifacts: []domain.Artifact{
			{ID: "art-receipt", Name: "bestbuy-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2025, time.November, 14)},
			{ID: "art-spending", Name: "spending-by-category.csv", Kind: "csv", Columns: []string{"Category", "This month", "Average"}, Rows: [][]string{
				{"Groceries", "412.00", "395.00"},
				{"Dining", "260.00", "215.00"},
				{"Transport", "186.00", "190.00"},
			}, CreatedAt: date(2026, time.June, 1)},
		},
		Workflows: []workflow.Workflow{
			{ID: "wf-flag-big", Name: "Flag large purchases", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "txn_abs > 200", Actions: []workflow.Action{{Kind: workflow.ActionFlagReview}}},
			{ID: "wf-coffee", Name: "Categorize coffee runs", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: `contains(txn_payee, "coffee")`, Actions: []workflow.Action{{Kind: workflow.ActionSetCategory, CategoryID: catDining}, {Kind: workflow.ActionAddTag, Tag: "coffee"}}},
			{ID: "wf-business", Name: "Tag freelance income", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: `contains(txn_payee, "studio")`, Actions: []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "business"}}},
			{ID: "wf-review", Name: "Monthly budget review", Enabled: false, Trigger: workflow.Trigger{Kind: workflow.TriggerManual}, Actions: []workflow.Action{{Kind: workflow.ActionApplyRules}, {Kind: workflow.ActionCreateTask, Title: "Review last month's budgets", Notes: "Check overspent categories and adjust."}, {Kind: workflow.ActionNotify, Message: "Monthly review complete."}}},
		},
		WorkflowRuns: []workflow.Run{
			{ID: "run-coffee-2026-06", WorkflowID: "wf-coffee", At: date(2026, time.June, 4).Format(time.RFC3339), Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionSetCategory, Summary: "Set category to Dining", CategoryID: catDining, TxnID: "tx-2026-06-coffee1"}}},
			{ID: "run-flag-2026-06", WorkflowID: "wf-flag-big", At: date(2026, time.June, 14).Format(time.RFC3339), Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionFlagReview, Summary: "Flagged for review", Tag: workflow.ReviewTag, TxnID: "tx-2026-06-big"}}},
			{ID: "run-review-2026-05", WorkflowID: "wf-review", At: date(2026, time.May, 31).Format(time.RFC3339), DryRun: true, Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionApplyRules, Summary: "Would categorize 4 transactions"}}},
		},
		SharedExpenses: []domain.SharedExpense{
			{ID: "se-dinner", Desc: "Dinner at Nobu", Date: date(2026, time.May, 24), PayerID: me, Shares: []domain.SharedExpenseShare{{MemberID: me, Amount: usd(4500)}, {MemberID: room, Amount: usd(4500)}}},
			{ID: "se-groceries", Desc: "Shared groceries", Date: date(2026, time.June, 7), PayerID: room, Shares: []domain.SharedExpenseShare{{MemberID: me, Amount: usd(3200)}, {MemberID: room, Amount: usd(3200)}}},
		},
		Settlements: []domain.Settlement{
			{ID: "settle-1", FromID: room, ToID: me, Amount: usd(4500), Date: date(2026, time.May, 26)},
		},
		Settings: Settings{
			BaseCurrency:       "USD",
			FXRates:            map[string]float64{"EUR": 0.92, "GBP": 0.79, "CAD": 1.36, "JPY": 151.0},
			FreshnessOverrides: map[string]int{checking: 7, k401: 90, roth: 90},
			PayoffBaseline:     &PayoffBaseline{TotalOwed: 2200000, Currency: "USD", StartedAt: date(2024, time.July, 1)},
		},
	}
}
