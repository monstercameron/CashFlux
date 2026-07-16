// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
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

// SampleDataset returns a realistic FIVE-year starter dataset for first run or
// the "load sample data" action: the finances of the Hartleys — Marcus, a
// software engineer, and his wife Priya, who works part-time at a library and
// runs a small online candle/knitwear shop. They are a debt-heavy,
// thin-margined household: two financed cars (the lifestyle "bad decision"), a
// credit card they carry from eating out too often, Priya's student loan,
// frequent travel — and a baby on the way (Priya is three months pregnant as of
// "today", late June 2026; due ~December 2026). North star: a bigger family home.
//
// It carries 60 months of activity (July 2021 – June 2026, plus the first days
// of July 2026 in flight) so every trend, chart, report, and forecast has real
// history — and the history tells a STORY with year-over-year growth and real
// setbacks:
//
//   - 2021: thin margins on a junior salary; bad decision #1 — crypto bought at
//     the November top, bags capitulated in the June 2022 crash.
//   - Jan 2023: Marcus is LAID OFF from Cohere Systems. Four months out of work
//     (Feb–May 2023): severance + unemployment + COBRA, savings drawn down
//     monthly, the card at minimum payments (one late fee), dining/travel cut,
//     side projects pushed hard. Hired by Meridian Data in June 2023.
//   - Spring 2024: the LUCKY STREAK — four straight green months in the WSB
//     brokerage (plus a scratch-off win), and for once he takes profits: $8k
//     moved to savings that June.
//   - 2025: bad decision #2 (and #3) — two financed cars, on top of the dining
//     habit that runs over budget almost every month.
//   - 2026: pregnancy costs ramp in the tail; the household is growing but the
//     margins are as thin as ever.
//
// Month-to-month amounts wobble on a deterministic hash (vr) — non-periodic
// like real life, byte-stable across runs for e2e. A rotating clutch of RAW
// BANK-STATEMENT imports ("POS DEBIT 4417 AMZN MKTP US*2K3L7QW", "VENMO
// PAYMENT 1023985544", …) lands every month, a standing minority of them
// uncategorized — plus a doubled DoorDash import and a mis-filed parking
// charge — so the rules engine, Smart+ scan, /duplicates, and bulk edits
// always have genuine work in the demo. It deliberately exercises **every**
// feature surface: sub-categories, transfers, splits, tags, custom fields,
// rules, workflows + run history, budgets (all periods + rollover), goals,
// tasks, recurring schedules, plans, allocation profiles, formulas, documents,
// artifacts, holdings, balance snapshots, unified-spec custom pages, shared
// expenses + settlements, and rich settings (FX table, freshness overrides,
// payoff baseline).
//
// Liabilities (the two car loans, the student loan, the credit card) are kept as
// static balances and their payments are categorized expenses — the convention
// the rest of the app uses — while the monthly transfers to savings, the Roth
// IRA, and the 401(k) move both legs, so balances and the net-worth trend
// actually change over time. All ids are stable so re-loading is idempotent.
// sampleSourceFor assigns a plausible provenance to a demo transaction so the
// ledger's Source column is illustrative in the sample data: receipts scan, bills
// recur, transfers are manual, and the everyday rows spread deterministically (by a
// stable per-id hash) across a realistic mix of manual / imported / scanned / AI.
func sampleSourceFor(t domain.Transaction) domain.TxnSource {
	if t.SourceDocID != "" {
		return domain.TxnSourceScanned
	}
	if t.TransferAccountID != "" {
		return domain.TxnSourceManual
	}
	p := strings.ToLower(t.Payee + " " + t.Desc)
	for _, kw := range []string{
		"paycheck", "part-time", "mortgage", "hoa", "property tax", "insurance",
		"loan", "subscription", "premium", "netflix", "spotify", "gym", "utilit",
		"internet", "electric", "phone", "rent",
	} {
		if strings.Contains(p, kw) {
			return domain.TxnSourceRecurring
		}
	}
	sum := 0
	for i := 0; i < len(t.ID); i++ {
		sum += int(t.ID[i])
	}
	switch sum % 10 {
	case 0, 1, 2:
		return domain.TxnSourceImported
	case 3:
		return domain.TxnSourceScanned
	case 4:
		return domain.TxnSourceAssistant
	default:
		return domain.TxnSourceManual
	}
}

func SampleDataset() Dataset {
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	eur := func(n int64) money.Money { return money.New(n, "EUR") } // for the foreign-trip FX demo
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	// Opening balances are stated as of the eve of the modeled history (July
	// 2021); the 60 months of transactions then carry each account to "today".
	// BalanceAsOf is when a balance was last CONFIRMED (the freshness anchor,
	// not a ledger cutoff — ledger.Balance sums opening + all activity), so each
	// account carries a recent, realistic confirmation date; the 401(k)/Roth sit
	// just past their 90-day overrides to keep the stale-balance nudge honest.
	asOf := date(2026, time.June, 20)
	// Activity on/before this date is reconciled; later (current-month) activity is
	// still pending, which is what a real ledger looks like mid-month.
	clearedAsOf := date(2026, time.June, 15)

	const (
		marcus = "m-marcus"
		priya  = "m-priya"
	)
	const (
		checking   = "acct-checking"    // joint
		hysa       = "acct-hysa"        // joint emergency / house / baby savings (thin)
		k401       = "acct-401k"        // Marcus's retirement
		roth       = "acct-roth"        // Roth IRA
		bizchk     = "acct-bizchecking" // Priya's online-business checking
		wsb        = "acct-brokerage"   // Marcus's self-directed "WSB" trading account
		cash       = "acct-cash"
		home       = "acct-home"           // the condo they own — now too small for the baby
		mortgage   = "acct-mortgage"       // mortgage on the condo
		carM       = "acct-carloan-marcus" // the expensive car (financed Jan 2025)
		carP       = "acct-carloan-priya"  // second car (financed Sep 2025)
		sloan      = "acct-studentloan"    // Priya's student loan
		card       = "acct-card"           // rewards card, carried revolving balance
		travelcard = "acct-travelcard"     // EUR travel card used abroad (FX demo)
	)
	const (
		// Income
		catSalary    = "cat-salary"
		catSideProj  = "cat-sideprojects"
		catPartTime  = "cat-parttime"
		catBizInc    = "cat-business-income"
		catInvestInc = "cat-investing-income" // realized trading gains
		catOtherInc  = "cat-other-income"
		// Expense parents
		catHousing    = "cat-housing"
		catUtilities  = "cat-utilities"
		catGroceries  = "cat-groceries"
		catDining     = "cat-dining"
		catTransport  = "cat-transport"
		catInsurance  = "cat-insurance"
		catHealth     = "cat-health"
		catBaby       = "cat-baby"
		catSubs       = "cat-subscriptions"
		catShopping   = "cat-shopping"
		catEntertain  = "cat-entertainment"
		catEducation  = "cat-education"
		catGifts      = "cat-gifts"
		catTravel     = "cat-travel"
		catBizExp     = "cat-business-expense"
		catVices      = "cat-vices"          // the guilty-pleasure noise: cigarettes, cheap cosmetics
		catInvestLoss = "cat-investing-loss" // realized trading losses (WSB)
		catFees       = "cat-fees"
		// Expense sub-categories (nested, to exercise the category tree)
		catElectric      = "cat-electricity"
		catInternet      = "cat-internet"
		catMortgage      = "cat-mortgage"    // child of Housing
		catHOA           = "cat-hoa"         // child of Housing
		catPropTax       = "cat-propertytax" // child of Housing
		catGas           = "cat-gas"
		catAutoLoan      = "cat-autoloan"
		catCarInsurance  = "cat-car-insurance"  // child of Insurance
		catHomeInsurance = "cat-home-insurance" // child of Insurance
	)

	// The layoff arc: Marcus's last Cohere paycheck lands Jan 2023 (i=18); he is
	// out of work Feb–May 2023 (i 19..22) and starts at Meridian Data in June
	// 2023 (i=23). Everything money-shaped in the loop keys off these two.
	const layoffAt, rehiredAt = 19, 23

	// Marcus's net monthly paycheck: junior money at Cohere, the layoff gap,
	// then Meridian Data at a step up with a raise each July. The July-2026
	// recurring schedule (rec-salary) matches the final step.
	salaryNet := func(i int) (payee string, net int64) {
		switch {
		case i < 12:
			return "Cohere Systems", 355000
		case i < layoffAt:
			return "Cohere Systems", 380000
		case i < rehiredAt:
			return "", 0 // the gap: no paycheck (see the unemployment block)
		case i < 36:
			return "Meridian Data", 405000
		case i < 48:
			return "Meridian Data", 440000
		default:
			return "Meridian Data", 470000
		}
	}
	// Priya's part-time take-home grows slowly — and she picked up extra shifts
	// while Marcus was out of work.
	partTimeNet := func(i int) int64 {
		n := int64(90000)
		switch {
		case i >= 36:
			n = 120000
		case i >= 24:
			n = 100000
		}
		if i >= layoffAt && i < rehiredAt {
			n += 32000
		}
		return n
	}

	// vr is a tiny deterministic hash → 0..n-1: stable across runs (e2e-safe)
	// but NON-periodic month to month, so charts wobble like real life instead
	// of repeating on a fixed cycle.
	vr := func(salt string, i int, n int64) int64 {
		h := uint32(2166136261)
		for _, c := range salt {
			h = (h ^ uint32(c)) * 16777619 //#nosec G115 -- FNV-1a hash: modular uint32 overflow is the intended behavior
		}
		h = (h ^ uint32(i)) * 16777619 //#nosec G115 -- FNV-1a hash: modular uint32 overflow is intended
		h = (h ^ (h >> 13)) * 16777619
		return int64(h % uint32(n)) //#nosec G115 -- n is a small positive chart count; no overflow
	}

	var txns []domain.Transaction
	add := func(t domain.Transaction) {
		// Tag a plausible provenance so the ledger's Source column shows a realistic
		// mix in the demo data (real entries get their source at their creation path).
		if t.Source == "" {
			t.Source = sampleSourceFor(t)
		}
		txns = append(txns, t)
	}
	cleared := func(d time.Time) bool { return !d.After(clearedAsOf) }

	// --- 60 months of activity (2021-07 .. 2026-06): five years, four arcs ---
	start := date(2021, time.July, 1)
	for i := range 60 {
		ym := start.AddDate(0, i, 0)
		y, m := ym.Year(), ym.Month()
		tag := ym.Format("2006-01")
		v := vr("month", i, 6) // bounded, non-periodic wobble
		gap := i >= layoffAt && i < rehiredAt
		// Pregnancy / baby ramp: only the final three months (Apr–Jun 2026).
		babyMonth := i >= 57

		txn := func(slot string, d int, acct, payee, desc, cat string, amt int64) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s", tag, slot), AccountID: acct, Date: dt,
				Payee: payee, Desc: desc, CategoryID: cat, Amount: usd(amt), MemberID: marcus, Cleared: cleared(dt),
			})
		}
		txnBy := func(member, slot string, d int, acct, payee, desc, cat string, amt int64) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s", tag, slot), AccountID: acct, Date: dt,
				Payee: payee, Desc: desc, CategoryID: cat, Amount: usd(amt), MemberID: member, Cleared: cleared(dt),
			})
		}
		addTransferBetween := func(slot string, d int, from, to, label string, amt int64) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-out", tag, slot), AccountID: from, Date: dt,
				Payee: label, Desc: label, Amount: usd(-amt), MemberID: marcus, TransferAccountID: to, Cleared: cleared(dt),
			})
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-%s-in", tag, slot), AccountID: to, Date: dt,
				Payee: label, Desc: label, Amount: usd(amt), MemberID: marcus, TransferAccountID: from, Cleared: cleared(dt),
			})
		}
		addTransfer := func(slot, dest, label string, amt int64) {
			addTransferBetween(slot, 2, checking, dest, label, amt)
		}

		// --- Income ---
		if payee, net := salaryNet(i); net > 0 {
			txn("salary", 1, checking, payee, "Paycheck (net)", catSalary, net)
		} else {
			// The gap months (Feb–May 2023): unemployment checks, COBRA instead
			// of employer health cover, and the emergency fund doing its job —
			// a monthly drawdown from savings keeps checking above water.
			txn("unemployment", 3, checking, "State Workforce Commission", "Unemployment benefit", catOtherInc, 172500)
			txn("cobra", 4, checking, "COBRA Health Admin", "COBRA health premium (family)", catHealth, -68000)
			if i >= 21 { // the severance carried Feb–Mar; then savings had to
				addTransferBetween("xfer-drawdown", 6, hysa, checking, "Emergency fund drawdown", 90000)
			}
		}
		txnBy(priya, "parttime", 1, checking, "Lakeside Library", "Part-time pay", catPartTime, partTimeNet(i))
		// Priya's online business opens Nov 2021 and grows ~5x over five years;
		// supplies/fees come back out of the business checking (both legs in bizchk).
		if i >= 4 {
			bizRev := 5200 + int64(i-4)*1850 + vr("bizrev", i, 5)*2800
			txnBy(priya, "biz-rev", 18, bizchk, "Etsy / Shopify payout", "Online shop sales", catBizInc, bizRev)
			txnBy(priya, "biz-exp", 19, bizchk, "Supplies & shipping", "Shop supplies", catBizExp, -(4200 + int64(i-4)*160 + vr("bizexp", i, 4)*1500))
			txnBy(priya, "bizsoft", 5, bizchk, "Shopify", "Shop software", catBizExp, -3900)
			// The shop pays its owner: most of the profit sweeps into the household
			// checking each month (the rest stays as the shop's working buffer).
			addTransferBetween("bizdraw", 26, bizchk, checking, "Owner's draw (shop profit)", (bizRev*7)/10)
		}
		// Marcus's side projects pay out irregularly (an app, a freelance gig) —
		// and he pushed them hard while out of work.
		if i%4 == 1 || gap {
			amt := 22000 + v*9000
			if gap {
				amt += 45000
			}
			dt := date(y, m, 23)
			add(domain.Transaction{
				ID: "tx-" + tag + "-sideproj", AccountID: checking, Date: dt, Payee: "App Store / Gumroad",
				Desc: "Side-project revenue", CategoryID: catSideProj, Amount: usd(amt),
				MemberID: marcus, Cleared: cleared(dt), Tags: []string{"business", "side-hustle"},
				Custom: map[string]any{"reimbursable": false, "project": "Side hustle"},
			})
		}
		if i%6 == 4 { // occasional resale income (selling old gear)
			txn("resale", 8, checking, "eBay", "Sold old gear", catOtherInc, 5000+v*2000)
		}

		// --- Housing & fixed bills (they OWN the condo) ---
		// Mortgage P&I and HOA are monthly; property tax is billed semi-annually
		// (two installments, April & October); home insurance is an annual premium
		// (each September) — each with its real-world cadence.
		// C206: mortgage payment as a two-legged transfer so the mortgage liability
		// balance amortizes each month. The out-leg leaves checking; the in-leg
		// credits the mortgage account, reducing the outstanding principal.
		{
			dt := date(y, m, 1)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-mortgage-out", tag), AccountID: checking, Date: dt,
				Payee: "Beacon Bank Home Loans", Desc: "Mortgage payment",
				Amount: usd(-148000), MemberID: marcus, TransferAccountID: mortgage, Cleared: cleared(dt),
			})
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-mortgage-in", tag), AccountID: mortgage, Date: dt,
				Payee: "Beacon Bank Home Loans", Desc: "Mortgage payment",
				Amount: usd(148000), MemberID: marcus, TransferAccountID: checking, Cleared: cleared(dt),
			})
		}
		txn("hoa", 1, checking, "Birchwood Condo Association", "HOA dues", catHOA, -38000)
		if m == time.April || m == time.October {
			txn("proptax", 12, checking, "County Tax Collector", "Property tax (installment)", catPropTax, -150000)
		}
		if m == time.September {
			txn("homeins", 7, checking, "SafeHarbor Insurance", "Home insurance (annual)", catHomeInsurance, -140000)
		}
		txn("electric", 8, checking, "Metro Power", "Electricity", catElectric, -(8500 + vr("elec", i, 9)*900))
		txn("internet", 9, checking, "Fiberline", "Internet", catInternet, -7500)
		txn("phone", 9, checking, "CellOne", "Phones (two lines)", catUtilities, -9500)
		if !gap { // the gym membership was frozen while Marcus was out of work
			txn("gym", 3, checking, "Iron Works Gym", "Gym membership", catHealth, -5000)
		}
		// C164: a real recurring service, not a generic "Subscriptions" lump — the old
		// desc "Subscriptions" leaked into the detector as a subscription literally named
		// "Subscriptions" (a category name masquerading as a merchant). Same account/
		// category/amount, so totals are unchanged; it just reads as a real charge now.
		txn("subs", 5, checking, "Google", "YouTube Premium", catSubs, -3800)
		// Named subscriptions — engage the Subscriptions detector, price-change
		// detection, mixed cadence (monthly + annual), and the stale/cancelled flows.
		netflix := int64(-1549)
		if i >= 42 { // Netflix raised its price (Jan 2025) → price-change detection
			netflix = -1799
		}
		txn("netflix", 11, card, "Netflix", "Netflix", catSubs, netflix)
		txn("spotify", 13, card, "Spotify", "Spotify Premium", catSubs, -1099)
		txn("icloud", 6, card, "Apple", "iCloud storage", catSubs, -299)
		if i >= 36 { // Marcus added ChatGPT Plus for the side projects (Jul 2024)
			txn("chatgpt", 7, card, "OpenAI", "ChatGPT Plus", catSubs, -2000)
		}
		if m == time.July { // Amazon Prime annual renewal (yearly-cadence subscription)
			txn("prime", 14, card, "Amazon Prime", "Prime membership (annual)", catSubs, -13900)
		}
		if i >= 30 && i <= 41 { // MasterClass: signed up Jan 2024, cancelled Jan 2025
			txn("masterclass", 16, card, "MasterClass", "MasterClass", catSubs, -1800)
		}
		// C206: student loan payment as a proper two-legged transfer so the sloan
		// liability balance decreases each month (amortizes) instead of staying flat.
		// The out-leg leaves checking; the in-leg credits sloan, reducing the owed balance.
		{
			dt := date(y, m, 5)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-studentloan-out", tag), AccountID: checking, Date: dt,
				Payee: "EdFinance Servicing", Desc: "Student loan payment",
				Amount: usd(-32000), MemberID: priya, TransferAccountID: sloan, Cleared: cleared(dt),
			})
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-studentloan-in", tag), AccountID: sloan, Date: dt,
				Payee: "EdFinance Servicing", Desc: "Student loan payment",
				Amount: usd(32000), MemberID: priya, TransferAccountID: checking, Cleared: cleared(dt),
			})
		}

		// --- Variable living expenses ---
		txn("grocery1", 6, checking, "Greenfield Market", "Groceries", catGroceries, -(20000 + vr("g1", i, 8)*1200 + boolN(babyMonth, 4000)))
		txnBy(priya, "grocery2", 20, checking, "Greenfield Market", "Groceries", catGroceries, -(17000 + vr("g2", i, 7)*1000 + boolN(babyMonth, 3000)))
		// Dining — the standing "bad decision": several outings a month,
		// chronically over budget — except the gap months, when they cut back hard.
		txn("dining1", 12, card, "Trattoria Nove", "Dinner out", catDining, -(16000 + vr("d1", i, 6)*2200 - boolN(gap, 9500)))
		if !gap {
			txnBy(priya, "dining2", 21, card, "Sushi Hana", "Date night", catDining, -(13000 + vr("d2", i, 5)*1800))
			txn("takeout", 26, card, "DoorDash", "Takeout", catDining, -(9000 + vr("d3", i, 5)*1500))
		}
		txn("gas", 10, checking, "Shell", "Gas", catGas, -(6000 + vr("gas", i, 5)*500))
		if i >= 50 { // Priya's car adds a second tank of gas
			txnBy(priya, "gas2", 24, checking, "Chevron", "Gas (Priya's car)", catGas, -(4500 + v*400))
		}
		// Car payments appear only once each car is financed.
		// C206: modeled as two-legged transfers (checking → car-loan account) so the
		// loan liabilities amortize down each month rather than staying at their
		// opening balances. The in-leg posts a positive amount to the loan account,
		// reducing the outstanding balance (which started as a negative opening bal).
		if i >= 42 { // Marcus's expensive car — financed Jan 2025 (bad decision #2)
			addTransferBetween("carpay-m", 15, checking, carM, "Car payment (Marcus)", 62000)
		}
		if i >= 50 { // Priya's car — financed Sep 2025 (bad decision #3)
			addTransferBetween("carpay-p", 17, checking, carP, "Car payment (Priya)", 48000)
		}
		// Car insurance: quarterly, and steps up once both cars are on the policy.
		if int(m)%3 == 0 {
			ins := int64(-30000)
			if i >= 50 {
				ins = -46000
			}
			txn("insurance", 6, checking, "SafeHarbor Insurance", "Car insurance", catCarInsurance, ins)
		}
		txn("health", 16, checking, "Wellness Pharmacy", "Pharmacy", catHealth, -(3000 + vr("rx", i, 6)*700))
		txn("shopping", 22, checking, "Northside Goods", "Household & shopping", catShopping, -(9500 + vr("shop", i, 6)*2200 + boolN(babyMonth, 9000) - boolN(gap, 6000)))
		if !gap {
			txn("fun", 18, checking, "Cineplex", "Movies & fun", catEntertain, -(5000 + vr("fun", i, 6)*1300))
		}
		// Holiday gifts every December; a card annual fee every August; the odd
		// out-of-network ATM fee — small realities that keep Gifts/Fees populated.
		if m == time.December {
			txn("gifts", 18, card, "Various", "Holiday gifts", catGifts, -(26000 + vr("gift", i, 4)*9000))
		}
		if m == time.August {
			txn("cardfee", 20, card, "Beacon Bank", "Card annual fee", catFees, -9500)
		}
		if vr("atmfee", i, 5) == 0 {
			txn("atmfee", 25, checking, "NON-NETWORK ATM", "ATM fee", catFees, -350)
		}
		if i == 21 { // the gap's low point: a missed card payment cost a late fee
			txn("latefee", 24, card, "Beacon Bank", "Late payment fee", catFees, -3900)
		}
		// Coffee runs — feeds the rules engine ("coffee" → Dining) and "Apply rules".
		coffee := func(slot string, d int) {
			dt := date(y, m, d)
			add(domain.Transaction{
				ID: "tx-" + tag + "-" + slot, AccountID: checking, Date: dt,
				Payee: "Blue Bottle Coffee", Desc: "Coffee", Amount: usd(-(600 + v*45)), MemberID: marcus, Cleared: cleared(dt),
			})
		}
		coffee("coffee1", 4)
		coffee("coffee2", 17)

		// --- Raw bank-statement imports (the garble a real CSV drops in) ---
		// A rotating clutch each month. Some are categorized (the rules did their
		// job); a standing minority land UNCATEGORIZED — which is exactly why the
		// rules engine, the Smart+ scan, and bulk recategorize have real work.
		type rawRow struct {
			payee, cat  string
			amt, spread int64
		}
		rawPool := []rawRow{
			{"POS DEBIT 4417 AMZN MKTP US*2K3L7QW", catShopping, -3400, 5200},
			{"ACH WEB SQ *SQ *BLUEBOTTL 877-417", catDining, -800, 500},
			{"TST* PHO SAIGON - 0042", catDining, -2600, 1400},
			{"CHECKCARD 0612 WL *STEAM PURCHASE 425-9522985", "", -1999, 4000},
			{"PAYPAL *GKELECTRON 402-935-7733", "", -4700, 6300},
			{"VENMO PAYMENT 1023985544", "", -2500, 5500},
			{"ZELLE TO CHEN, JENNY", "", -4000, 4000},
			{"IC* INSTACART*2 HTTPSINSTACAR", catGroceries, -6400, 3800},
			{"PWP*CITYPARKING 866-330-3444", catTransport, -1200, 900},
			{"APLPAY GREENFIELD MKT #204", catGroceries, -3100, 2600},
			{"GOOGLE *TEMU 855-444-0141", "", -1800, 3300},
			{"WLMRT GROC 3959 W CHSTR", catGroceries, -5400, 3200},
			{"CKE*DD DOORDASH WINGSTOP 855-431-0459", catDining, -2800, 1900},
			{"SP MIDNIGHT CANDLE CO", "", -2200, 1800},
			{"MSFT * XBOX GAME PASS 425-6816830", "", -1699, 0},
		}
		nraw := 3 + int(vr("rawn", i, 3))
		for r := range nraw {
			row := rawPool[(i*5+r*3)%len(rawPool)]
			day := 2 + int(vr(fmt.Sprintf("rawd%d", r), i, 26))
			amt := row.amt
			if row.spread > 0 {
				amt -= vr(fmt.Sprintf("rawa%d", r), i, row.spread)
			}
			member := marcus
			if r%2 == 1 {
				member = priya
			}
			dt := date(y, m, day)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-raw%d", tag, r), AccountID: checking, Date: dt,
				Payee: row.payee, Desc: row.payee, CategoryID: row.cat,
				Amount: usd(amt), MemberID: member, Cleared: cleared(dt),
				Source: domain.TxnSourceImported,
			})
		}

		// --- "Guilty pleasure" noise (varies month to month) ---
		// Marcus's cigarettes: a few small convenience-store buys a month, paid in
		// cash, with wandering payees, days, and prices so it looks like real habit
		// spending rather than a clean recurring line.
		smokeShops := []string{"Quik Mart", "7-Eleven", "Smoke Shop", "Gas-N-Go"}
		smokes := 2 + int(vr("smoken", i, 3)) // 2..4 packs a month
		if gap {
			smokes += 2 // the layoff months were stressful
		}
		for s := range smokes {
			day := min(3+s*7+int(i%4), 28)
			dt := date(y, m, day)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-smokes-%d", tag, s), AccountID: cash, Date: dt,
				Payee: smokeShops[(i+s)%len(smokeShops)], Desc: "Cigarettes", CategoryID: catVices,
				Amount: usd(-(1050 + (int64(s)+v)%4*130)), MemberID: marcus, Cleared: cleared(dt),
				Tags: []string{"cigarettes"},
			})
		}
		// Priya's cheap cosmetics: impulse Amazon orders, small and frequent.
		cosmetics := 1 + int((i/2)%3) // 1..3 orders a month
		for c := range cosmetics {
			day := min(5+c*9+int(i%3), 27)
			dt := date(y, m, day)
			add(domain.Transaction{
				ID: fmt.Sprintf("tx-%s-cosmetics-%d", tag, c), AccountID: card, Date: dt,
				Payee: "Amazon", Desc: "Cheap cosmetics", CategoryID: catVices,
				Amount: usd(-(800 + (int64(c)*3+v)%5*460)), MemberID: priya, Cleared: cleared(dt),
				Tags: []string{"cosmetics", "amazon"},
			})
		}
		// A weekend getaway every few months (they love to travel) — on the card.
		// Not during the gap: travel was the first thing cut.
		if i%3 == 2 && !gap {
			dt := date(y, m, 14)
			add(domain.Transaction{
				ID: "tx-" + tag + "-getaway", AccountID: card, Date: dt, Payee: "Airbnb",
				Desc: "Weekend getaway", CategoryID: catTravel, Amount: usd(-(28000 + vr("trip", i, 7)*7000)),
				MemberID: marcus, Cleared: cleared(dt), Tags: []string{"vacation"},
			})
		}
		// Monthly credit-card payment as a transfer (checking → card) so the card's
		// balance actually pays down: purchases post to the card, this brings it back,
		// and they carry a revolving balance. The gap months drop to the MINIMUM
		// payment — the balance visibly balloons through mid-2023.
		cardPay := int64(87000)
		switch {
		case gap:
			cardPay = 22000 // minimum payment — the balance balloons through mid-2023
		case i < 24:
			cardPay = 55000 // lighter spending years, lighter payment
		case i >= 42:
			cardPay = 105000 // two-car era: the card runs hotter, so does the payment
		}
		addTransfer("cardpay", card, "Credit card payment", cardPay)

		// --- Marcus's r/wallstreetbets dabbling (varying degrees of "success") ---
		// Small deposits feed the account; wins and losses land with meme-stock
		// descriptions. Losses skew bigger and more frequent — as they do — EXCEPT
		// the spring-2024 lucky streak: four straight green months, each bigger
		// than the last.
		tickers := []string{"GME calls", "TSLA", "NVDA calls", "AMC", "SPY puts", "PLTR", "DOGE"}
		switch {
		case gap: // no gambling money while out of work
		case i >= 31 && i <= 34: // THE LUCKY STREAK (Feb–May 2024)
			streak := []int64{90000, 210000, 380000, 650000}
			dt := date(y, m, 25)
			add(domain.Transaction{
				ID: "tx-" + tag + "-wsb-win", AccountID: wsb, Date: dt, Payee: "Robinhood",
				Desc: "Sold " + tickers[i%len(tickers)] + " — stonks only go up", CategoryID: catInvestInc,
				Amount: usd(streak[i-31]), MemberID: marcus, Cleared: cleared(dt), Tags: []string{"wsb", "hot-streak"},
			})
		default:
			if i%3 == 0 {
				addTransfer("xfer-wsb", wsb, "Deposit to brokerage", 15000)
			}
			switch i % 4 {
			case 1: // a green day
				dt := date(y, m, 25)
				add(domain.Transaction{
					ID: "tx-" + tag + "-wsb-win", AccountID: wsb, Date: dt, Payee: "Robinhood",
					Desc: "Sold " + tickers[i%len(tickers)] + " — green day", CategoryID: catInvestInc,
					Amount: usd(8000 + v*9000), MemberID: marcus, Cleared: cleared(dt), Tags: []string{"wsb", "stonks"},
				})
			case 3: // a loss (bigger, naturally)
				dt := date(y, m, 27)
				add(domain.Transaction{
					ID: "tx-" + tag + "-wsb-loss", AccountID: wsb, Date: dt, Payee: "Robinhood",
					Desc: tickers[(i+3)%len(tickers)] + " — expired worthless", CategoryID: catInvestLoss,
					Amount: usd(-(6000 + v*11000)), MemberID: marcus, Cleared: cleared(dt), Tags: []string{"wsb", "loss-porn"},
				})
			}
		}

		// A small monthly ATM withdrawal keeps the cash wallet stocked (it's what
		// Marcus's cigarettes are paid from), so cash never drifts negative.
		addTransfer("atm", cash, "ATM withdrawal", 5000)

		// --- Transfers that (slowly) build wealth; thin, sometimes skipped, and
		// suspended entirely during the gap (the drawdown runs the other way). ---
		if !gap && i%4 != 3 { // they don't manage to save every month
			save := int64(12000)
			if i >= 36 {
				save = 20000
			}
			addTransfer("xfer-hysa", hysa, "Transfer to savings", save+boolN(babyMonth, 8000))
		}
		if i >= rehiredAt+1 { // the Roth started once the Meridian job stuck
			addTransfer("xfer-roth", roth, "Transfer to Roth IRA", 10000)
		}
		if !gap {
			k := int64(15000)
			if i >= 48 {
				k = 20000
			}
			addTransfer("xfer-401k", k401, "Transfer to 401(k)", k)
		}
	}

	// --- one-off milestones across the five years (the arcs' anchors) ---

	// Bad decision #1: crypto bought at the very top (Nov 2021) — one green sale
	// near the peak, then the bags capitulated into the June 2022 crash.
	add(domain.Transaction{ID: "tx-crypto-fund-2021-11-out", AccountID: hysa, Date: date(2021, time.November, 8), Payee: "Deposit to brokerage", Desc: "Deposit to brokerage (raided savings for crypto)", Amount: usd(-300000), MemberID: marcus, TransferAccountID: wsb, Cleared: true})
	add(domain.Transaction{ID: "tx-crypto-fund-2021-11-in", AccountID: wsb, Date: date(2021, time.November, 8), Payee: "Deposit to brokerage", Desc: "Deposit to brokerage (raided savings for crypto)", Amount: usd(300000), MemberID: marcus, TransferAccountID: hysa, Cleared: true})
	add(domain.Transaction{ID: "tx-crypto-gain-2021-12", AccountID: wsb, Date: date(2021, time.December, 3), Payee: "Coinbase", Desc: "Sold some ETH near the top — should have stopped here", CategoryID: catInvestInc, Amount: usd(45000), MemberID: marcus, Cleared: true, Tags: []string{"crypto"}})
	add(domain.Transaction{ID: "tx-crypto-loss-2022-06", AccountID: wsb, Date: date(2022, time.June, 16), Payee: "Coinbase", Desc: "Capitulated — sold the BTC/ETH bags (-70%)", CategoryID: catInvestLoss, Amount: usd(-420000), MemberID: marcus, Cleared: true, Tags: []string{"crypto", "loss-porn"}})

	// The layoff (end of Jan 2023): two months' severance up front; the monthly
	// unemployment checks, COBRA, and savings drawdowns live in the loop above.
	add(domain.Transaction{ID: "tx-severance-2023-02", AccountID: checking, Date: date(2023, time.February, 3), Payee: "Cohere Systems", Desc: "Severance (2 months)", CategoryID: catSalary, Amount: usd(550000), MemberID: marcus, Cleared: true, Tags: []string{"layoff"}})

	// The lucky streak's coda (Jun 2024): for once, he actually took profits —
	// $8k out of the brokerage into savings. Plus a scratch-off in the middle of
	// the hot spring, because luck compounds.
	add(domain.Transaction{ID: "tx-wsb-cashout-2024-06-out", AccountID: wsb, Date: date(2024, time.June, 7), Payee: "Took profits — to savings", Desc: "Took profits — to savings", Amount: usd(-800000), MemberID: marcus, TransferAccountID: hysa, Cleared: true})
	add(domain.Transaction{ID: "tx-wsb-cashout-2024-06-in", AccountID: hysa, Date: date(2024, time.June, 7), Payee: "Took profits — to savings", Desc: "Took profits — to savings", Amount: usd(800000), MemberID: marcus, TransferAccountID: wsb, Cleared: true})
	add(domain.Transaction{ID: "tx-scratch-2024-03", AccountID: cash, Date: date(2024, time.March, 16), Payee: "FL Lottery", Desc: "Scratch-off win", CategoryID: catOtherInc, Amount: usd(15000), MemberID: marcus, Cleared: true, Tags: []string{"lucky"}})

	// Errata the tools exist for: the same DoorDash charge imported TWICE (the
	// /duplicates screen has a real catch), and a parking charge mis-filed under
	// Dining (bulk-recategorize / rules bait).
	add(domain.Transaction{ID: "tx-dup-doordash-2026-06-a", AccountID: card, Date: date(2026, time.June, 21), Payee: "CKE*DD DOORDASH WINGSTOP 855-431-0459", Desc: "CKE*DD DOORDASH WINGSTOP 855-431-0459", CategoryID: catDining, Amount: usd(-3849), MemberID: marcus, Cleared: true, Source: domain.TxnSourceImported})
	add(domain.Transaction{ID: "tx-dup-doordash-2026-06-b", AccountID: card, Date: date(2026, time.June, 21), Payee: "CKE*DD DOORDASH WINGSTOP 855-431-0459", Desc: "CKE*DD DOORDASH WINGSTOP 855-431-0459", CategoryID: catDining, Amount: usd(-3849), MemberID: marcus, Cleared: true, Source: domain.TxnSourceImported})
	add(domain.Transaction{ID: "tx-misfiled-parking-2026-05", AccountID: checking, Date: date(2026, time.May, 14), Payee: "PWP*CITYPARKING 866-330-3444", Desc: "PWP*CITYPARKING 866-330-3444", CategoryID: catDining, Amount: usd(-1400), MemberID: marcus, Cleared: true, Source: domain.TxnSourceImported})

	// The current month (July 2026) is mid-flight: the first few days have
	// posted — pending, uncleared — which is what a real ledger looks like on
	// the 4th. (The matching recurring schedules' NextDue dates sit in August.)
	add(domain.Transaction{ID: "tx-2026-07-salary", AccountID: checking, Date: date(2026, time.July, 1), Payee: "Meridian Data", Desc: "Paycheck (net)", CategoryID: catSalary, Amount: usd(470000), MemberID: marcus, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-parttime", AccountID: checking, Date: date(2026, time.July, 1), Payee: "Lakeside Library", Desc: "Part-time pay", CategoryID: catPartTime, Amount: usd(120000), MemberID: priya, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-mortgage-out", AccountID: checking, Date: date(2026, time.July, 1), Payee: "Beacon Bank Home Loans", Desc: "Mortgage payment", Amount: usd(-148000), MemberID: marcus, TransferAccountID: mortgage, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-mortgage-in", AccountID: mortgage, Date: date(2026, time.July, 1), Payee: "Beacon Bank Home Loans", Desc: "Mortgage payment", Amount: usd(148000), MemberID: marcus, TransferAccountID: checking, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-hoa", AccountID: checking, Date: date(2026, time.July, 1), Payee: "Birchwood Condo Association", Desc: "HOA dues", CategoryID: catHOA, Amount: usd(-38000), MemberID: marcus, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-coffee1", AccountID: checking, Date: date(2026, time.July, 2), Payee: "Blue Bottle Coffee", Desc: "Coffee", Amount: usd(-675), MemberID: marcus, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-grocery1", AccountID: checking, Date: date(2026, time.July, 3), Payee: "Greenfield Market", Desc: "Groceries", CategoryID: catGroceries, Amount: usd(-21400), MemberID: priya, Cleared: false})
	add(domain.Transaction{ID: "tx-2026-07-raw0", AccountID: checking, Date: date(2026, time.July, 3), Payee: "APLPAY GREENFIELD MKT #204", Desc: "APLPAY GREENFIELD MKT #204", CategoryID: catGroceries, Amount: usd(-2850), MemberID: marcus, Cleared: false, Source: domain.TxnSourceImported})
	add(domain.Transaction{ID: "tx-2026-07-raw1", AccountID: checking, Date: date(2026, time.July, 4), Payee: "VENMO PAYMENT 1042778120", Desc: "VENMO PAYMENT 1042778120", Amount: usd(-3200), MemberID: marcus, Cleared: false, Source: domain.TxnSourceImported})

	// --- one-off events across the years (variety for reports/charts) ---
	add(domain.Transaction{ID: "tx-bonus-2021-12", AccountID: checking, Date: date(2021, time.December, 21), Payee: "Cohere Systems", Desc: "Year-end bonus", CategoryID: catSalary, Amount: usd(80000), MemberID: marcus, Cleared: true, Tags: []string{"bonus"}})
	add(domain.Transaction{ID: "tx-honeymoon-flight-2022-09", AccountID: card, Date: date(2022, time.September, 10), Payee: "SkyJet", Desc: "Honeymoon flights", CategoryID: catTravel, Amount: usd(-110000), MemberID: marcus, Cleared: true, Tags: []string{"vacation", "honeymoon"}})
	add(domain.Transaction{ID: "tx-honeymoon-hotel-2022-09", AccountID: card, Date: date(2022, time.September, 12), Payee: "Amalfi Resort", Desc: "Honeymoon hotel", CategoryID: catTravel, Amount: usd(-145000), MemberID: marcus, Cleared: true, Tags: []string{"vacation", "honeymoon"}})
	add(domain.Transaction{ID: "tx-bonus-2022-12", AccountID: checking, Date: date(2022, time.December, 20), Payee: "Cohere Systems", Desc: "Year-end bonus", CategoryID: catSalary, Amount: usd(120000), MemberID: marcus, Cleared: true, Tags: []string{"bonus"}})
	add(domain.Transaction{ID: "tx-refund-2023-04", AccountID: checking, Date: date(2023, time.April, 12), Payee: "IRS", Desc: "Tax refund", CategoryID: catOtherInc, Amount: usd(85000), MemberID: marcus, Cleared: true})
	add(domain.Transaction{ID: "tx-trip-2023-07", AccountID: card, Date: date(2023, time.July, 6), Payee: "Seaside Resort", Desc: "Summer trip", CategoryID: catTravel, Amount: usd(-120000), MemberID: marcus, Cleared: true, Tags: []string{"vacation"}})
	add(domain.Transaction{ID: "tx-anniv-2024-06", AccountID: card, Date: date(2024, time.June, 18), Payee: "Mountain Lodge", Desc: "Anniversary trip", CategoryID: catTravel, Amount: usd(-95000), MemberID: marcus, Cleared: true, Tags: []string{"vacation"}})
	// A trip to Rome charged in euros on a EUR travel card — exercises multi-currency
	// (FX) aggregation. The charges live on a EUR-denominated account so each account's
	// balance stays single-currency; NetWorth converts at the account level.
	add(domain.Transaction{ID: "tx-rome-hotel-2024-09", AccountID: travelcard, Date: date(2024, time.September, 10), Payee: "Hotel Roma", Desc: "Hotel (Rome)", CategoryID: catTravel, Amount: eur(-45000), MemberID: marcus, Cleared: true, Tags: []string{"vacation", "fx"}})
	add(domain.Transaction{ID: "tx-rome-dinner-2024-09", AccountID: travelcard, Date: date(2024, time.September, 12), Payee: "Trattoria Roma", Desc: "Dinner (Rome)", CategoryID: catDining, Amount: eur(-8500), MemberID: priya, Cleared: true, Tags: []string{"vacation", "fx"}})
	// A returned online purchase — a positive amount on an expense category (refund),
	// so refund/return handling and category nets have a real case to chew on.
	add(domain.Transaction{ID: "tx-return-2026-03", AccountID: card, Date: date(2026, time.March, 20), Payee: "Amazon", Desc: "Refund — returned item", CategoryID: catShopping, Amount: usd(6500), MemberID: priya, Cleared: true, Tags: []string{"refund"}})
	// One stray MasterClass charge AFTER they cancelled it (Jan 2025) — engages the
	// "charged after cancellation" alert on the Subscriptions page.
	add(domain.Transaction{ID: "tx-masterclass-late-2025-02", AccountID: card, Date: date(2025, time.February, 16), Payee: "MasterClass", Desc: "MasterClass", CategoryID: catSubs, Amount: usd(-1800), MemberID: marcus, Cleared: true})
	add(domain.Transaction{ID: "tx-bonus-2024-12", AccountID: checking, Date: date(2024, time.December, 20), Payee: "Cohere Systems", Desc: "Year-end bonus", CategoryID: catSalary, Amount: usd(140000), MemberID: marcus, Cleared: true, Tags: []string{"bonus"}})
	// Marcus's expensive car: a down payment out of savings the month it's financed.
	add(domain.Transaction{ID: "tx-cardown-2025-01", AccountID: hysa, Date: date(2025, time.January, 10), Payee: "Apex Auto Finance", Desc: "Car down payment (Marcus)", CategoryID: catTransport, Amount: usd(-300000), MemberID: marcus, Cleared: true, Tags: []string{"big-purchase"}})
	add(domain.Transaction{ID: "tx-refund-2025-04", AccountID: checking, Date: date(2025, time.April, 12), Payee: "IRS", Desc: "Tax refund", CategoryID: catOtherInc, Amount: usd(70000), MemberID: marcus, Cleared: true})
	// Priya's car down payment.
	add(domain.Transaction{ID: "tx-cardown-2025-09", AccountID: hysa, Date: date(2025, time.September, 15), Payee: "Apex Auto Finance", Desc: "Car down payment (Priya)", CategoryID: catTransport, Amount: usd(-180000), MemberID: priya, Cleared: true, Tags: []string{"big-purchase"}})
	add(domain.Transaction{ID: "tx-bonus-2025-12", AccountID: checking, Date: date(2025, time.December, 19), Payee: "Cohere Systems", Desc: "Year-end bonus", CategoryID: catSalary, Amount: usd(150000), MemberID: marcus, Cleared: true, Tags: []string{"bonus"}})
	// A Costco run split across two categories (exercises CategorySplit).
	add(domain.Transaction{ID: "tx-costco-2026-02", AccountID: checking, Date: date(2026, time.February, 15), Payee: "Costco", Desc: "Costco run", Amount: usd(-28000), MemberID: priya, Cleared: true, Splits: []domain.CategorySplit{{CategoryID: catGroceries, Amount: usd(-18000)}, {CategoryID: catShopping, Amount: usd(-10000)}}})
	// A pricey anniversary dinner (more dining excess) — linked to a receipt doc + artifact.
	add(domain.Transaction{ID: "tx-anniv-dinner-2026-02", AccountID: card, Date: date(2026, time.February, 22), Payee: "Nobu", Desc: "Anniversary dinner", CategoryID: catDining, Amount: usd(-24000), MemberID: marcus, Cleared: true, Tags: []string{"big-purchase"}, Source: domain.TxnSourceScanned, SourceDocID: "doc-receipt", Attachments: []domain.AttachmentRef{{ArtifactID: "art-receipt", Name: "nobu-receipt.png", Kind: "image", MIME: "image/png"}}})

	// --- Recent scanned receipts (document-sourced) ---------------------------------
	// These demonstrate the "Scanned" provenance end-to-end: each is tagged
	// Source=Scanned, links to a Document (so it shows in the Documents panel), and
	// carries an attached receipt artifact you can preview from the ledger. Dated in the
	// current month so they land on the first (newest-first) page of the ledger.
	add(domain.Transaction{ID: "tx-receipt-grocery-2026-06", AccountID: card, Date: date(2026, time.June, 22), Payee: "Greenfield Market", Desc: "Groceries (scanned receipt)", CategoryID: catGroceries, Amount: usd(-14280), MemberID: priya, Cleared: true, Source: domain.TxnSourceScanned, SourceDocID: "doc-grocery", Attachments: []domain.AttachmentRef{{ArtifactID: "art-grocery", Name: "greenfield-receipt.png", Kind: "image", MIME: "image/png"}}})
	add(domain.Transaction{ID: "tx-receipt-pharmacy-2026-06", AccountID: card, Date: date(2026, time.June, 18), Payee: "Riverside Pharmacy", Desc: "Prenatal vitamins (scanned receipt)", CategoryID: catHealth, Amount: usd(-4820), MemberID: priya, Cleared: true, Source: domain.TxnSourceScanned, SourceDocID: "doc-pharmacy", Attachments: []domain.AttachmentRef{{ArtifactID: "art-pharmacy", Name: "pharmacy-receipt.png", Kind: "image", MIME: "image/png"}}})
	add(domain.Transaction{ID: "tx-receipt-hardware-2026-06", AccountID: checking, Date: date(2026, time.June, 9), Payee: "Bishop's Hardware", Desc: "Nursery paint & supplies (scanned receipt)", CategoryID: catShopping, Amount: usd(-8650), MemberID: marcus, Cleared: true, Source: domain.TxnSourceScanned, SourceDocID: "doc-hardware", Attachments: []domain.AttachmentRef{{ArtifactID: "art-hardware", Name: "hardware-receipt.png", Kind: "image", MIME: "image/png"}}})
	add(domain.Transaction{ID: "tx-receipt-dining-2026-06", AccountID: card, Date: date(2026, time.June, 5), Payee: "Pho Saigon", Desc: "Team lunch (scanned receipt)", CategoryID: catDining, Amount: usd(-3140), MemberID: marcus, Cleared: true, Source: domain.TxnSourceScanned, SourceDocID: "doc-dining", Attachments: []domain.AttachmentRef{{ArtifactID: "art-dining", Name: "pho-receipt.png", Kind: "image", MIME: "image/png"}}})

	// --- Pregnancy / baby (recent tail) ---
	add(domain.Transaction{ID: "tx-ob-2026-04", AccountID: card, Date: date(2026, time.April, 9), Payee: "Riverside OB-GYN", Desc: "Prenatal visit", CategoryID: catBaby, Amount: usd(-18000), MemberID: priya, Cleared: true, Tags: []string{"reimbursable", "baby"}, Custom: map[string]any{"reimbursable": true, "project": "Personal"}})
	add(domain.Transaction{ID: "tx-ultrasound-2026-05", AccountID: card, Date: date(2026, time.May, 8), Payee: "Riverside Imaging", Desc: "Ultrasound", CategoryID: catBaby, Amount: usd(-22000), MemberID: priya, Cleared: true, Tags: []string{"baby"}})
	add(domain.Transaction{ID: "tx-nursery-2026-05", AccountID: card, Date: date(2026, time.May, 20), Payee: "Babylist", Desc: "Nursery furniture", CategoryID: catBaby, Amount: usd(-85000), MemberID: marcus, Cleared: true, Tags: []string{"baby", "big-purchase"}})
	add(domain.Transaction{ID: "tx-babyreg-2026-06", AccountID: card, Date: date(2026, time.June, 10), Payee: "Target", Desc: "Crib & registry items", CategoryID: catBaby, Amount: usd(-45000), MemberID: priya, Cleared: false, Tags: []string{"baby"}})
	add(domain.Transaction{ID: "tx-ob-2026-06", AccountID: card, Date: date(2026, time.June, 11), Payee: "Riverside OB-GYN", Desc: "Prenatal visit", CategoryID: catBaby, Amount: usd(-18000), MemberID: priya, Cleared: false, Tags: []string{"baby"}})

	// A shared dinner the couple splits (settles via the SharedExpense ledger).
	add(domain.Transaction{ID: "tx-dinner-shared-2026-05", AccountID: card, Date: date(2026, time.May, 24), Payee: "Trattoria Nove", Desc: "Dinner with friends (our half)", CategoryID: catDining, Amount: usd(-11000), MemberID: marcus, Cleared: true})

	// --- Holdings: what the investment accounts actually hold (the Investments
	// surface's data) — the retirement funds are boring on purpose; the WSB
	// positions mirror the art-wsb positions table. ---
	holdings := []domain.Holding{
		{ID: "hold-401k-tdf", AccountID: k401, Name: "Target 2055 Fund", Shares: 412.6, CostBasisMinor: 1310000, CurrentPriceMinorPerShare: 4120, AssetClass: "Stocks", SecurityType: domain.SecurityMutualFund},
		{ID: "hold-roth-vti", AccountID: roth, Ticker: "VTI", Name: "Vanguard Total Market ETF", Shares: 19.6, CostBasisMinor: 442000, CurrentPriceMinorPerShare: 29350, AssetClass: "Stocks", SecurityType: domain.SecurityETF},
		{ID: "hold-wsb-nvda", AccountID: wsb, Ticker: "NVDA", Name: "NVDA calls (Jul)", Shares: 2, CostBasisMinor: 120000, CurrentPriceMinorPerShare: 132000, AssetClass: "Stocks", SecurityType: domain.SecurityStock},
		{ID: "hold-wsb-tsla", AccountID: wsb, Ticker: "TSLA", Name: "Tesla shares", Shares: 6, CostBasisMinor: 180000, CurrentPriceMinorPerShare: 25150, AssetClass: "Stocks", SecurityType: domain.SecurityStock},
		{ID: "hold-wsb-gme", AccountID: wsb, Ticker: "GME", Name: "GameStop shares", Shares: 15, CostBasisMinor: 64000, CurrentPriceMinorPerShare: 2770, AssetClass: "Stocks", SecurityType: domain.SecurityStock},
		{ID: "hold-wsb-pltr", AccountID: wsb, Ticker: "PLTR", Name: "Palantir shares", Shares: 12, CostBasisMinor: 50000, CurrentPriceMinorPerShare: 6790, AssetClass: "Stocks", SecurityType: domain.SecurityStock},
	}

	// --- Quarterly valuation snapshots for the slow accounts (401(k), Roth,
	// the condo): the market story the transaction ledger can't tell — 2022
	// down, 2023 sideways, 2024–25 up — plus steady condo appreciation. These
	// back each account row's valuation-history sparkline. ---
	var snapshots []domain.BalanceSnapshot
	snapCurve := func(acct string, quarterly []int64) {
		for qi, bal := range quarterly {
			at := date(2021, time.September, 30).AddDate(0, qi*3, 0)
			snapshots = append(snapshots, domain.BalanceSnapshot{
				ID: fmt.Sprintf("snap-%s-%d", acct, qi), AccountID: acct,
				BalanceMinor: bal, Currency: "USD", AsOf: at,
			})
		}
	}
	// 20 quarters: 2021-Q3 .. 2026-Q2.
	snapCurve(k401, []int64{880000, 905000, 860000, 812000, 795000, 830000, 872000, 918000, 969000, 1030000, 1105000, 1190000, 1245000, 1330000, 1428000, 1500000, 1552000, 1610000, 1655000, 1700000})
	snapCurve(roth, []int64{222000, 227000, 214000, 203000, 200000, 208000, 224000, 248000, 275000, 305000, 338000, 372000, 404000, 438000, 470000, 500000, 528000, 552000, 566000, 580000})
	snapCurve(home, []int64{26600000, 26750000, 26900000, 27000000, 27050000, 27150000, 27300000, 27500000, 27750000, 28000000, 28300000, 28600000, 28900000, 29150000, 29400000, 29600000, 29800000, 30000000, 30200000, 30400000})

	tinyPNG := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}

	ds := Dataset{
		Members: []domain.Member{
			{ID: marcus, Name: "Marcus Hartley", IsDefault: true, Color: "#4ade80", Custom: map[string]any{}},
			{ID: priya, Name: "Priya Hartley", Color: "#f472b6"},
		},
		Accounts: []domain.Account{
			{ID: checking, Name: "Joint Checking", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(650000), BalanceAsOf: asOf, LiquidityScore: 100, StabilityScore: 95, ExpectedReturnAPR: 0.1, Custom: map[string]any{"last4": "4821"}},
			{ID: hysa, Name: "Joint Savings (HYSA)", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(1050000), BalanceAsOf: date(2026, time.June, 2), LiquidityScore: 90, StabilityScore: 98, ExpectedReturnAPR: 4.2},
			{ID: k401, Name: "Marcus's 401(k)", OwnerID: marcus, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(850000), BalanceAsOf: date(2026, time.March, 31), LiquidityScore: 40, StabilityScore: 55, ExpectedReturnAPR: 7.5},
			{ID: roth, Name: "Roth IRA", OwnerID: marcus, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(220000), BalanceAsOf: date(2026, time.March, 31), LiquidityScore: 45, StabilityScore: 60, ExpectedReturnAPR: 7.0},
			{ID: bizchk, Name: "Priya's Business Checking", OwnerID: priya, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(15000), BalanceAsOf: date(2026, time.June, 26), LiquidityScore: 100, StabilityScore: 80, ExpectedReturnAPR: 0.1},
			{ID: wsb, Name: "Stonks (Fun Money)", OwnerID: marcus, Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(40000), BalanceAsOf: date(2026, time.June, 25), LiquidityScore: 50, StabilityScore: 15, ExpectedReturnAPR: 4.0, Custom: map[string]any{}},
			{ID: cash, Name: "Cash Wallet", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: usd(9000), BalanceAsOf: date(2026, time.June, 2), LiquidityScore: 100, StabilityScore: 80},
			{ID: home, Name: "Condo (2 bed / 1 bath)", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeProperty, Currency: "USD", OpeningBalance: usd(30400000), BalanceAsOf: date(2026, time.June, 30), LiquidityScore: 5, StabilityScore: 80, ExpectedReturnAPR: 3.5, Custom: map[string]any{}},
			{ID: mortgage, Name: "Mortgage", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeMortgage, Currency: "USD", OpeningBalance: usd(-24400000), BalanceAsOf: date(2026, time.July, 1), InterestRateAPR: 4.1, DueDayOfMonth: 1, MinPayment: usd(148000), Lender: "Beacon Bank Home Loans"},
			{ID: carM, Name: "Marcus's Car Loan", OwnerID: marcus, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-3800000), BalanceAsOf: date(2026, time.June, 15), InterestRateAPR: 7.4, DueDayOfMonth: 15, MinPayment: usd(62000), Lender: "Apex Auto Finance"},
			{ID: carP, Name: "Priya's Car Loan", OwnerID: priya, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-2600000), BalanceAsOf: date(2026, time.June, 17), InterestRateAPR: 6.9, DueDayOfMonth: 17, MinPayment: usd(48000), Lender: "Apex Auto Finance"},
			{ID: sloan, Name: "Priya's Student Loan", OwnerID: priya, Scope: domain.ScopeIndividual, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: usd(-3800000), BalanceAsOf: date(2026, time.June, 5), InterestRateAPR: 5.5, DueDayOfMonth: 5, MinPayment: usd(32000), Lender: "EdFinance Servicing"},
			{ID: card, Name: "Rewards Credit Card", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-380000), BalanceAsOf: date(2026, time.June, 22), CreditLimit: usd(1500000), InterestRateAPR: 24.99, DueDayOfMonth: 22, MinPayment: usd(22000), Lender: "Beacon Bank"},
			{ID: travelcard, Name: "Travel Card (EUR)", OwnerID: marcus, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "EUR", OpeningBalance: eur(0), BalanceAsOf: date(2026, time.June, 20), CreditLimit: eur(300000), InterestRateAPR: 19.9, Lender: "Wise"},
		},
		Categories: []domain.Category{
			{ID: catSalary, Name: "Salary", Kind: domain.KindIncome, Color: "#22c55e"},
			{ID: catSideProj, Name: "Side projects", Kind: domain.KindIncome, Color: "#16a34a"},
			{ID: catPartTime, Name: "Part-time", Kind: domain.KindIncome, Color: "#4ade80"},
			{ID: catBizInc, Name: "Online business", Kind: domain.KindIncome, Color: "#10b981"},
			{ID: catInvestInc, Name: "Investing gains", Kind: domain.KindIncome, Color: "#22d3ee"},
			{ID: catOtherInc, Name: "Other income", Kind: domain.KindIncome, Color: "#86efac"},
			{ID: catHousing, Name: "Housing", Kind: domain.KindExpense, Color: "#60a5fa"},
			{ID: catMortgage, Name: "Mortgage", Kind: domain.KindExpense, Color: "#3b82f6", ParentID: catHousing},
			{ID: catHOA, Name: "HOA dues", Kind: domain.KindExpense, Color: "#2563eb", ParentID: catHousing},
			{ID: catPropTax, Name: "Property tax", Kind: domain.KindExpense, Color: "#1d4ed8", ParentID: catHousing},
			{ID: catUtilities, Name: "Utilities", Kind: domain.KindExpense, Color: "#38bdf8"},
			{ID: catElectric, Name: "Electricity", Kind: domain.KindExpense, Color: "#0ea5e9", ParentID: catUtilities},
			{ID: catInternet, Name: "Internet", Kind: domain.KindExpense, Color: "#0284c7", ParentID: catUtilities},
			{ID: catGroceries, Name: "Groceries", Kind: domain.KindExpense, Color: "#f59e0b"},
			{ID: catDining, Name: "Dining", Kind: domain.KindExpense, Color: "#fb923c"},
			{ID: catTransport, Name: "Transportation", Kind: domain.KindExpense, Color: "#a78bfa"},
			{ID: catGas, Name: "Gas", Kind: domain.KindExpense, Color: "#8b5cf6", ParentID: catTransport},
			{ID: catAutoLoan, Name: "Auto loans", Kind: domain.KindExpense, Color: "#7c3aed", ParentID: catTransport},
			{ID: catInsurance, Name: "Insurance", Kind: domain.KindExpense, Color: "#f472b6"},
			{ID: catCarInsurance, Name: "Car insurance", Kind: domain.KindExpense, Color: "#ec4899", ParentID: catInsurance},
			{ID: catHomeInsurance, Name: "Home insurance", Kind: domain.KindExpense, Color: "#db2777", ParentID: catInsurance},
			{ID: catHealth, Name: "Health & Fitness", Kind: domain.KindExpense, Color: "#34d399"},
			{ID: catBaby, Name: "Baby & Childcare", Kind: domain.KindExpense, Color: "#fbcfe8"},
			{ID: catSubs, Name: "Subscriptions", Kind: domain.KindExpense, Color: "#c084fc"},
			{ID: catShopping, Name: "Shopping", Kind: domain.KindExpense, Color: "#e879f9"},
			{ID: catEntertain, Name: "Entertainment", Kind: domain.KindExpense, Color: "#f87171"},
			{ID: catEducation, Name: "Education & Loans", Kind: domain.KindExpense, Color: "#fbbf24"},
			{ID: catGifts, Name: "Gifts & Charity", Kind: domain.KindExpense, Color: "#fda4af"},
			{ID: catTravel, Name: "Travel", Kind: domain.KindExpense, Color: "#2dd4bf"},
			{ID: catBizExp, Name: "Business expenses", Kind: domain.KindExpense, Color: "#a3a3a3"},
			{ID: catVices, Name: "Guilty pleasures", Kind: domain.KindExpense, Color: "#737373"},
			{ID: catInvestLoss, Name: "Investing losses", Kind: domain.KindExpense, Color: "#9f1239"},
			{ID: catFees, Name: "Fees & Charges", Kind: domain.KindExpense, Color: "#94a3b8"},
		},
		Transactions:     txns,
		Holdings:         holdings,
		BalanceSnapshots: snapshots,
		Budgets: []domain.Budget{
			{ID: "bud-dining", Name: "Dining", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catDining, Period: domain.PeriodMonthly, Limit: usd(30000)},
			{ID: "bud-groceries", Name: "Groceries", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catGroceries, Period: domain.PeriodMonthly, Limit: usd(45000), Rollover: true},
			{ID: "bud-transport", Name: "Transportation", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catTransport, Period: domain.PeriodMonthly, Limit: usd(130000)},
			{ID: "bud-baby", Name: "Baby & Childcare", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catBaby, Period: domain.PeriodMonthly, Limit: usd(40000)},
			{ID: "bud-shopping", Name: "Shopping", Scope: domain.ScopeIndividual, OwnerID: marcus, CategoryID: catShopping, Period: domain.PeriodMonthly, Limit: usd(20000)},
			{ID: "bud-subs", Name: "Subscriptions", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catSubs, Period: domain.PeriodMonthly, Limit: usd(4000)},
			{ID: "bud-vices", Name: "Guilty pleasures", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catVices, Period: domain.PeriodMonthly, Limit: usd(6000)},
			{ID: "bud-fun", Name: "Entertainment", Scope: domain.ScopeIndividual, OwnerID: marcus, CategoryID: catEntertain, Period: domain.PeriodWeekly, Limit: usd(2500)},
			{ID: "bud-travel", Name: "Travel", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: catTravel, Period: domain.PeriodQuarterly, Limit: usd(60000)},
		},
		Goals: []domain.Goal{
			{ID: "goal-house", Name: "Trade up to a bigger family home", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: usd(8000000), CurrentAmount: usd(1150000), TargetDate: date(2029, time.June, 1), AccountID: hysa},
			// Virtual allocation ("earmarks"): the Hartleys have mentally reserved part of their
			// savings + checking toward the baby and emergency funds WITHOUT moving the money —
			// so the Goals page shows partly-earmarked coverage and the Earmarks tab has real data.
			{ID: "goal-baby", Name: "Baby fund (due Dec 2026)", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: usd(1200000), CurrentAmount: usd(280000), TargetDate: date(2026, time.December, 1), AccountID: hysa,
				Allocations: []domain.GoalAllocation{{AccountID: hysa, Amount: usd(300000)}, {AccountID: checking, Amount: usd(200000)}}},
			{ID: "goal-emergency", Name: "Emergency fund (3 months)", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: usd(1500000), CurrentAmount: usd(480000), TargetDate: date(2027, time.June, 1), AccountID: hysa,
				Allocations: []domain.GoalAllocation{{AccountID: hysa, Amount: usd(250000)}}},
			{ID: "goal-studentloan", Name: "Pay off Priya's student loan", Scope: domain.ScopeIndividual, OwnerID: priya, TargetAmount: usd(3400000), CurrentAmount: usd(900000), TargetDate: date(2029, time.December, 1)},
			{ID: "goal-car", Name: "Pay off Marcus's car loan", Scope: domain.ScopeIndividual, OwnerID: marcus, TargetAmount: usd(3800000), CurrentAmount: usd(600000), TargetDate: date(2030, time.January, 1)},
		},
		Tasks: []domain.Task{
			{ID: "task-card", Title: "Pay down the credit card balance", Notes: "We're carrying a balance — pay more than the minimum this month.", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.June, 22), RelatedType: domain.RelatedAccount, RelatedID: card, MemberID: marcus, Source: domain.SourceManual},
			{ID: "task-emergency", Title: "Rebuild the emergency fund — we've been through one layoff already", Notes: "The 2023 layoff drained savings to $7,500 and checking almost hit zero. Aim for 3 months of expenses before the baby comes — the $200/mo transfers aren't enough on their own.", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.August, 1), RelatedType: domain.RelatedGoal, RelatedID: "goal-emergency", MemberID: marcus, Source: domain.SourceManual},
			{ID: "task-baby-budget", Title: "Set up the nursery and finalize baby budget", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.October, 1), RelatedType: domain.RelatedGoal, RelatedID: "goal-baby", MemberID: priya, Source: domain.SourceManual},
			{ID: "task-dining-budget", Title: "Dining is way over budget — let's cut back", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedBudget, RelatedID: "bud-dining", MemberID: marcus, Source: domain.SourceAI},
			{ID: "task-categorize", Title: "Categorize the raw bank imports (VENMO / PAYPAL / ZELLE…)", Notes: "The CSV imports keep landing uncategorized. Add rules for the regulars or run the Smart+ suggestion scan on /rules.", Status: domain.StatusOpen, Priority: domain.PriorityMedium, MemberID: marcus, Source: domain.SourceAI},
			{ID: "task-stale-401k", Title: "Update 401(k) balance (stale)", Status: domain.StatusOpen, Priority: domain.PriorityLow, RelatedType: domain.RelatedAccount, RelatedID: k401, MemberID: marcus, Source: domain.SourceNudge},
			{ID: "task-maternity-leave", Title: "Check maternity-leave pay and budget for the gap", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.September, 1), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-done-refi", Title: "Looked into refinancing the car loan", Status: domain.StatusDone, Priority: domain.PriorityMedium, RelatedType: domain.RelatedAccount, RelatedID: carM, MemberID: marcus, Source: domain.SourceManual},

			// Follow-ups pinned to a specific CHARGE (RelatedType=transaction) — the errata
			// above become actionable: each shows on the ledger as an "open/total" chip that
			// hover-previews these to-dos and links back here. A couple of charges carry more
			// than one so the chip reads "1/2" / "2/2" and the popover lists several.
			{ID: "task-fu-masterclass", Title: "Dispute the MasterClass charge — we cancelled in January", Notes: "It billed a month after we cancelled. Screenshot the cancellation email before disputing.", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.July, 10), RelatedType: domain.RelatedTransaction, RelatedID: "tx-masterclass-late-2025-02", MemberID: marcus, Source: domain.SourceAI},
			{ID: "task-fu-doordash", Title: "Get the duplicate DoorDash charge reversed", Notes: "Wingstop hit the card twice on the same day — dispute the second one.", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.July, 8), RelatedType: domain.RelatedTransaction, RelatedID: "tx-dup-doordash-2026-06-a", MemberID: marcus, Source: domain.SourceNudge},
			{ID: "task-fu-pharmacy", Title: "Submit the prenatal-vitamin receipt to the HSA", Notes: "Riverside Pharmacy — reimbursable from the HSA. Receipt is attached to the charge.", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.July, 15), RelatedType: domain.RelatedTransaction, RelatedID: "tx-receipt-pharmacy-2026-06", MemberID: priya, Source: domain.SourceManual},
			{ID: "task-fu-amazon", Title: "Confirm the Amazon refund landed on the card", Status: domain.StatusDone, Priority: domain.PriorityLow, RelatedType: domain.RelatedTransaction, RelatedID: "tx-return-2026-03", MemberID: priya, Source: domain.SourceManual},
			// Two follow-ups on the Nobu dinner → the ledger chip reads "1/2".
			{ID: "task-fu-nobu-amex", Title: "Claim the Nobu dinner for the Amex dining credit", Status: domain.StatusOpen, Priority: domain.PriorityLow, RelatedType: domain.RelatedTransaction, RelatedID: "tx-anniv-dinner-2026-02", MemberID: marcus, Source: domain.SourceManual},
			{ID: "task-fu-nobu-album", Title: "Add the anniversary photos to the shared album", Status: domain.StatusDone, Priority: domain.PriorityLow, RelatedType: domain.RelatedTransaction, RelatedID: "tx-anniv-dinner-2026-02", MemberID: priya, Source: domain.SourceManual},
			// Two open follow-ups on the mystery Venmo import → the chip reads "2/2".
			{ID: "task-fu-venmo-who", Title: "Figure out who the $32 Venmo payment was to", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedTransaction, RelatedID: "tx-2026-07-raw1", MemberID: marcus, Source: domain.SourceAI},
			{ID: "task-fu-venmo-rule", Title: "Add a payee rule so future Venmo imports auto-categorize", Status: domain.StatusOpen, Priority: domain.PriorityLow, RelatedType: domain.RelatedTransaction, RelatedID: "tx-2026-07-raw1", MemberID: marcus, Source: domain.SourceAI},

			// Priya keeps an exhaustive, slightly obsessive list of house to-dos — she's
			// deep in "nesting" mode with the baby coming. A big, mixed-status batch that
			// makes the to-do system feel lived-in (open/done, every priority, due dates).
			{ID: "task-h-nursery-paint", Title: "Repaint the guest room as the nursery (sage green)", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.August, 15), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-crib", Title: "Assemble the crib and changing table", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.September, 1), RelatedType: domain.RelatedGoal, RelatedID: "goal-baby", MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-carseat", Title: "Install the car seat and get it inspected", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.November, 1), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-babyproof", Title: "Baby-proof outlets, cabinets, and stair gate", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.October, 15), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-washclothes", Title: "Wash and fold all the 0–3 month baby clothes", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.October, 20), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-freezer", Title: "Batch-cook and freeze 3 weeks of newborn meals", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.November, 10), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-declutter", Title: "Declutter the closet and box up the donation pile", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-pantry", Title: "Reorganize and label the entire pantry", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-faucet", Title: "Fix the dripping bathroom faucet (or call the plumber)", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.July, 5), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-hvac", Title: "Schedule HVAC service before winter", Status: domain.StatusOpen, Priority: domain.PriorityLow, Due: date(2026, time.September, 30), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-filters", Title: "Replace all the air filters (set a quarterly reminder)", Status: domain.StatusOpen, Priority: domain.PriorityLow, Due: date(2026, time.July, 1), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-smoke", Title: "Test smoke + CO detectors and swap batteries", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: date(2026, time.July, 12), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-deepclean", Title: "Deep-clean the kitchen (behind the fridge too)", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-garage", Title: "Clear the garage so both cars actually fit", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-stroller", Title: "Research strollers and add the top pick to the registry", Status: domain.StatusDone, Priority: domain.PriorityMedium, RelatedType: domain.RelatedGoal, RelatedID: "goal-baby", MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-curtains", Title: "Hang blackout curtains in the nursery", Status: domain.StatusDone, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-pediatrician", Title: "Tour two pediatricians and pick one", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: date(2026, time.September, 20), MemberID: priya, Source: domain.SourceManual},
			{ID: "task-h-plants", Title: "Set a watering schedule for the houseplants", Status: domain.StatusOpen, Priority: domain.PriorityLow, MemberID: priya, Source: domain.SourceManual},
		},
		CustomFields: []customfields.Def{
			{ID: "cf-txn-reimbursable", EntityType: "transaction", Key: "reimbursable", Label: "Reimbursable", Type: customfields.TypeBool},
			{ID: "cf-txn-project", EntityType: "transaction", Key: "project", Label: "Project", Type: customfields.TypeSelect, Options: []string{"Personal", "Side hustle", "Online business"}},
			{ID: "cf-acct-last4", EntityType: "account", Key: "last4", Label: "Account number (last 4)", Type: customfields.TypeText},
		},
		Rules: []rules.Rule{
			{ID: "rule-coffee", Match: "coffee", SetCategoryID: catDining, SetTags: []string{"coffee"}},
			{ID: "rule-shell", Match: "shell", SetCategoryID: catGas},
			{ID: "rule-greenfield", Match: "greenfield", SetCategoryID: catGroceries},
			{ID: "rule-streaming", Match: "streaming", SetCategoryID: catSubs},
			{ID: "rule-doordash", Match: "doordash", SetCategoryID: catDining},
			{ID: "rule-amazon", Match: "amzn", SetCategoryID: catShopping},
			{ID: "rule-instacart", Match: "instacart", SetCategoryID: catGroceries},
		},
		Documents: []domain.Document{
			{ID: "doc-statement", Filename: "checking-2026-05.csv", Kind: domain.DocCSV, UploadedAt: date(2026, time.June, 1), AccountID: checking, MemberID: marcus, Status: domain.DocImported, Extracted: []domain.DocumentRow{
				{Date: "2026-05-06", Description: "Greenfield Market", Amount: "-214.30", Category: "Groceries"},
				{Date: "2026-05-08", Description: "Riverside Imaging", Amount: "-220.00", Category: "Baby & Childcare"},
			}},
			{ID: "doc-receipt", Filename: "nobu-receipt.png", Kind: domain.DocImage, UploadedAt: date(2026, time.February, 22), AccountID: card, MemberID: marcus, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{
				{Date: "2026-02-22", Description: "Nobu — Anniversary dinner", Amount: "-240.00", Category: "Dining"},
			}},
			{ID: "doc-pending", Filename: "ob-receipt.jpg", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 11), MemberID: priya, Status: domain.DocPending},
			// Receipt scans behind the recent document-sourced transactions above.
			{ID: "doc-grocery", Filename: "greenfield-receipt.png", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 22), AccountID: card, MemberID: priya, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{{Date: "2026-06-22", Description: "Greenfield Market", Amount: "-142.80", Category: "Groceries"}}},
			{ID: "doc-pharmacy", Filename: "pharmacy-receipt.png", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 18), AccountID: card, MemberID: priya, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{{Date: "2026-06-18", Description: "Riverside Pharmacy", Amount: "-48.20", Category: "Health & Fitness"}}},
			{ID: "doc-hardware", Filename: "hardware-receipt.png", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 9), AccountID: checking, MemberID: marcus, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{{Date: "2026-06-09", Description: "Bishop's Hardware", Amount: "-86.50", Category: "Shopping"}}},
			{ID: "doc-dining", Filename: "pho-receipt.png", Kind: domain.DocImage, UploadedAt: date(2026, time.June, 5), AccountID: card, MemberID: marcus, Status: domain.DocExtracted, Extracted: []domain.DocumentRow{{Date: "2026-06-05", Description: "Pho Saigon", Amount: "-31.40", Category: "Dining"}}},
		},
		SavedInsights: []domain.SavedInsight{
			{ID: "insight-dining", Text: "Dining is your biggest leak: it runs roughly $250–$400 over the $300 monthly budget almost every month — about $3,500/year you could redirect to the baby fund or the car loan.", CreatedAt: date(2026, time.May, 2)},
			{ID: "insight-runway", Text: "Your emergency fund only covers about 1.5 months of expenses right now. With the baby due in December, building this toward three months is the most important near-term move.", CreatedAt: date(2026, time.June, 5)},
			{ID: "insight-debt", Text: "Between the two car loans, the student loan, and the card, debt payments are over $1,600/month. Paying the card down first (25% APR) saves the most interest.", CreatedAt: date(2026, time.June, 6)},
			{ID: "insight-jobloss", Text: "You've lived this before: when Marcus was laid off in early 2023, checking bottomed out near $1,200 and you drew down savings for months. Today the fund covers about 1.5 months of expenses — with the baby due in December, growing it toward three months is the single most protective move.", CreatedAt: date(2026, time.June, 8)},
			{ID: "insight-uncategorized", Text: "There are dozens of imported transactions with raw bank names (VENMO, PAYPAL, ZELLE, STEAM…) still uncategorized. Adding three or four rules — or running the Smart+ scan — would clean most of them up and make every report more accurate.", CreatedAt: date(2026, time.June, 20)},
			{ID: "insight-streak", Text: "The spring-2024 brokerage streak (+$13,300 over four months) worked out because you took profits — the $8,000 you moved to savings is most of what later covered the car down payments. The habit worth keeping is the cash-out, not the streak.", CreatedAt: date(2026, time.June, 14)},
		},
		Recurring: []domain.Recurring{
			{ID: "rec-salary", Label: "Paycheck (net)", Amount: usd(470000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.August, 1), AccountID: checking, CategoryID: catSalary, Autopost: true},
			{ID: "rec-mortgage", Label: "Mortgage payment", Amount: usd(-148000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.August, 1), AccountID: checking, CategoryID: catMortgage, Autopost: true},
			{ID: "rec-hoa", Label: "HOA dues", Amount: usd(-38000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.August, 1), AccountID: checking, CategoryID: catHOA, Autopost: true},
			// Property tax is billed in two installments a year; the cadence enum has no
			// "semi-annual", so it's modeled as two yearly schedules (April & October).
			{ID: "rec-proptax-fall", Label: "Property tax (fall installment)", Amount: usd(-150000), Cadence: domain.CadenceYearly, NextDue: date(2026, time.October, 12), AccountID: checking, CategoryID: catPropTax},
			{ID: "rec-proptax-spring", Label: "Property tax (spring installment)", Amount: usd(-150000), Cadence: domain.CadenceYearly, NextDue: date(2027, time.April, 12), AccountID: checking, CategoryID: catPropTax},
			{ID: "rec-homeins", Label: "Home insurance (annual)", Amount: usd(-140000), Cadence: domain.CadenceYearly, NextDue: date(2026, time.September, 7), AccountID: checking, CategoryID: catHomeInsurance},
			{ID: "rec-carpay-m", Label: "Car payment (Marcus)", Amount: usd(-62000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 15), AccountID: checking, CategoryID: catAutoLoan, Autopost: true},
			{ID: "rec-carpay-p", Label: "Car payment (Priya)", Amount: usd(-48000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 17), AccountID: checking, CategoryID: catAutoLoan, Autopost: true},
			{ID: "rec-studentloan", Label: "Student loan payment", Amount: usd(-32000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 5), AccountID: checking, CategoryID: catEducation, Autopost: true},
			{ID: "rec-insurance", Label: "Car insurance", Amount: usd(-46000), Cadence: domain.CadenceQuarterly, NextDue: date(2026, time.September, 1), AccountID: checking, CategoryID: catInsurance},
			{ID: "rec-subs", Label: "Streaming & apps", Amount: usd(-3800), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 5), AccountID: checking, CategoryID: catSubs},
			{ID: "rec-bizsoft", Label: "Shop software", Amount: usd(-3900), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 5), AccountID: bizchk, CategoryID: catBizExp},
			{ID: "rec-gym", Label: "Gym membership", Amount: usd(-5000), Cadence: domain.CadenceMonthly, NextDue: date(2026, time.July, 3), AccountID: checking, CategoryID: catHealth},
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
			{ID: "plan-house", Name: "House down payment in 3 years", HorizonMonths: 36, StartBalance: 1900000, Items: []domain.PlanItem{
				{ID: "pi-save", Label: "Monthly saving", Kind: domain.PlanItemRecurring, Amount: 40000},
				{ID: "pi-bonus", Label: "Annual bonus", Kind: domain.PlanItemOneTime, Amount: 150000, Month: 6},
				{ID: "pi-down", Label: "Down payment", Kind: domain.PlanItemOneTime, Amount: -6000000, Month: 36},
			}},
			{ID: "plan-card-payoff", Name: "Extra $300/mo to the credit card", HorizonMonths: 24, StartBalance: 1900000, Items: []domain.PlanItem{
				{ID: "pi-extra", Label: "Extra card payment", Kind: domain.PlanItemRecurring, Amount: -30000},
			}},
			// Marcus's worst-case worry: his job ends and they live on Priya's part-time
			// + shop income against full expenses. Starts from their liquid savings and
			// burns down fast — the case for a bigger emergency fund, made concrete.
			{ID: "plan-jobloss", Name: "If Marcus loses his job (income gap)", HorizonMonths: 6, StartBalance: 480000, Items: []domain.PlanItem{
				{ID: "pi-gap", Label: "Expenses minus Priya's income", Kind: domain.PlanItemRecurring, Amount: -370000},
			}},
		},
		CustomPages: []domain.CustomPage{
			{
				ID: "page-side-hustle", Slug: "side-hustle", Name: "Side hustle", Icon: "briefcase", Order: 0,
				CreatedAt: date(2023, time.January, 10),
				Layout: []dashlayout.Item{
					{ID: "w-surplus", ColSpan: 1, RowSpan: 1},
					{ID: "w-sh-cashflow", ColSpan: 3, RowSpan: 2},
					{ID: "w-freelance", ColSpan: 2, RowSpan: 2},
					{ID: "w-sh-savings", ColSpan: 2, RowSpan: 2},
				},
				Widgets: []domain.PageWidget{
					{ID: "w-surplus", Type: "kpi", Title: "Monthly surplus", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-surplus", Kind: domain.KindKPI, Title: "Monthly surplus", Scalar: &domain.ScalarBind{Expr: "income - expense", Format: "currency", Sub: "whole household, this month"}}},
					{ID: "w-sh-cashflow", Type: "chart", Title: "Cash flow (12 months)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-sh-cashflow", Kind: domain.KindChart, Title: "Cash flow (12 months)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "cashflow", Months: 12}}}}},
					{ID: "w-freelance", Type: "list", Title: "Side projects & business", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{Source: "transactions", Filter: "tag:business", Columns: []string{"date", "payee", "amount"}}},
					{ID: "w-sh-savings", Type: "chart", Title: "Savings rate (formula, 12 months)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-sh-savings", Kind: domain.KindChart, Title: "Savings rate (formula, 12 months)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "formula", Months: 12, Expr: "savings_rate", Format: "percent"}}}}},
				},
			},
			// Priya's online business — a detailed dashboard for her Etsy/Shopify shop:
			// narrative, two charts (revenue + orders), the monthly detail table, a couple
			// of KPIs, and the live business-tagged activity feed.
			{
				ID: "page-priya-business", Slug: "priya-business", Name: "Priya's Business", Icon: "store", Order: 1,
				CreatedAt: date(2024, time.March, 1),
				Layout: []dashlayout.Item{
					{ID: "w-pb-note", ColSpan: 4, RowSpan: 1},
					{ID: "w-pb-rev", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-orders", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-costs-live", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-cashflow", ColSpan: 1, RowSpan: 1},
					{ID: "w-pb-savings", ColSpan: 1, RowSpan: 1},
					{ID: "w-pb-pnl", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-products", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-costs", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-channels", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-inventory", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-fulfillment", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-tax", ColSpan: 2, RowSpan: 1},
					{ID: "w-pb-sales", ColSpan: 2, RowSpan: 2},
					{ID: "w-pb-recent", ColSpan: 2, RowSpan: 2},
				},
				Widgets: []domain.PageWidget{
					{ID: "w-pb-note", Type: "text", Title: "About the shop", Config: widgetcfg.Config{"text": "### 🧵 Priya's Handmade Co.\nA small **Etsy + Shopify** shop selling hand-poured candles and knitwear, open since **November 2021**. Revenue has climbed from **~$60/mo** in the first winter to about **$1,100/mo** now — it even carried groceries through Marcus's 2023 layoff. Most of the profit sweeps to the household as a monthly **owner's draw**.\n\n**This month at a glance**\n- Net margin **~65%**, avg order value **$22.40**, sales tax accruing **$118** (Q2).\n- **4 orders to fulfill**, **3 supplies low** (soy wax, jars, boxes — reorder).\n- Channels: Etsy 59% · Shopify 33% · local market 8%.\n\nEverything below is what a one-person shop actually tracks: P&L, best-sellers, cost breakdown, stock levels, fulfillment queue, and tax set-aside."}},
					{ID: "w-pb-rev", Type: "chart", Title: "Shop revenue (live, 12 months)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-pb-rev", Kind: domain.KindChart, Title: "Shop revenue (live, 12 months)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "flow", Months: 12, Filter: "cat:cat-business-income"}}}}},
					{ID: "w-pb-orders", Type: "image", Title: "Orders by month", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-orders"}},
					{ID: "w-pb-costs-live", Type: "chart", Title: "Shop costs (live, 12 months)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-pb-costs-live", Kind: domain.KindChart, Title: "Shop costs (live, 12 months)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "flow", Months: 12, Filter: "cat:cat-business-expense", Abs: true}}}}},
					{ID: "w-pb-cashflow", Type: "kpi", Title: "Household cash flow", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-pb-cashflow", Kind: domain.KindKPI, Title: "Household cash flow", Scalar: &domain.ScalarBind{Expr: "income - expense", Format: "currency", Sub: "this month"}}},
					{ID: "w-pb-savings", Type: "kpi", Title: "Savings rate", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-pb-savings", Kind: domain.KindKPI, Title: "Savings rate", Scalar: &domain.ScalarBind{Expr: "round((income - expense) / income * 100)", Format: "percent", Sub: "this month"}}},
					{ID: "w-pb-pnl", Type: "table", Title: "Profit & loss by month", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-pnl"}},
					{ID: "w-pb-products", Type: "table", Title: "Best sellers", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-products"}},
					{ID: "w-pb-costs", Type: "table", Title: "Cost breakdown", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-costs"}},
					{ID: "w-pb-channels", Type: "table", Title: "Sales by channel", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-channels"}},
					{ID: "w-pb-inventory", Type: "table", Title: "Inventory & reorders", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-inventory"}},
					{ID: "w-pb-fulfillment", Type: "table", Title: "Orders to fulfill", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-fulfillment"}},
					{ID: "w-pb-tax", Type: "table", Title: "Sales-tax set-aside", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-tax"}},
					{ID: "w-pb-sales", Type: "table", Title: "Sales detail", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-shop-sales"}},
					{ID: "w-pb-recent", Type: "list", Title: "Recent activity", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{Source: "transactions"}},
				},
			},
			// Marcus's hobbies — side projects + his r/wallstreetbets dabbling: narrative,
			// the WSB account-value rollercoaster (line), side-project revenue (bar), the
			// positions table, net worth + the net-worth trend, and recent activity.
			{
				ID: "page-marcus-hobbies", Slug: "marcus-hobbies", Name: "Marcus's Hobbies", Icon: "rocket", Order: 2,
				CreatedAt: date(2023, time.June, 15),
				Layout: []dashlayout.Item{
					{ID: "w-mh-note", ColSpan: 4, RowSpan: 1},
					{ID: "w-mh-wsbval", ColSpan: 2, RowSpan: 2},
					{ID: "w-mh-sideproj", ColSpan: 2, RowSpan: 2},
					{ID: "w-mh-networth", ColSpan: 1, RowSpan: 1},
					{ID: "w-mh-wsb", ColSpan: 2, RowSpan: 2},
					{ID: "w-mh-trend", ColSpan: 2, RowSpan: 2},
					{ID: "w-mh-recent", ColSpan: 2, RowSpan: 2},
				},
				Widgets: []domain.PageWidget{
					{ID: "w-mh-note", Type: "text", Title: "Hobbies & side projects", Config: widgetcfg.Config{"text": "### 🚀 Marcus's playground\nWhere the side income and the *risky fun* live.\n\n**Side projects** — a couple of apps plus freelance gigs. Lumpy income, and the thing that kept him sane (and paid) during the **2023 layoff**.\n\n**The brokerage** — five years of honest history: crypto bought at the **Nov 2021 top** and capitulated in the crash (−$4,200 realized), a dead-quiet 2023 (no gambling money while unemployed), then the **spring-2024 hot streak** — four straight green months, +$13,300 — and, for once, profits actually taken: **$8k moved to savings**. Current positions below — decidedly mixed.\n\n> 🧠 Rule of thumb: only money he can afford to lose goes into the Stonks account — it stays hobby-sized on purpose. It is **not** the retirement plan."}},
					{ID: "w-mh-wsbval", Type: "image", Title: "WSB account value (the rollercoaster)", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-wsb-value"}},
					{ID: "w-mh-sideproj", Type: "chart", Title: "Side-project income (live — by custom field)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-mh-sideproj", Kind: domain.KindChart, Title: "Side-project income (live — by custom field)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "flow", Months: 24, Filter: "cf:project=Side hustle"}}}}},
					{ID: "w-mh-networth", Type: "kpi", Title: "Net worth", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-mh-networth", Kind: domain.KindKPI, Title: "Net worth", Scalar: &domain.ScalarBind{Expr: "net_worth", Format: "currency", Sub: "assets minus debts, today"}}},
					{ID: "w-mh-wsb", Type: "table", Title: "Stonks positions", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{ArtifactID: "art-wsb"}},
					{ID: "w-mh-trend", Type: "chart", Title: "Net-worth trend (24 months)", Spec: &domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "w-mh-trend", Kind: domain.KindChart, Title: "Net-worth trend (24 months)", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "networth", Months: 24}}}}},
					{ID: "w-mh-recent", Type: "list", Title: "Recent side income", Config: widgetcfg.Config{}, Binding: domain.WidgetBinding{Source: "transactions"}},
				},
			},
		},
		Artifacts: []domain.Artifact{
			{ID: "art-receipt", Name: "nobu-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2026, time.February, 22)},
			{ID: "art-grocery", Name: "greenfield-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2026, time.June, 22)},
			{ID: "art-pharmacy", Name: "pharmacy-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2026, time.June, 18)},
			{ID: "art-hardware", Name: "hardware-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2026, time.June, 9)},
			{ID: "art-dining", Name: "pho-receipt.png", Kind: "image", MIME: "image/png", Bytes: tinyPNG, Size: len(tinyPNG), CreatedAt: date(2026, time.June, 5)},
			{ID: "art-spending", Name: "spending-by-category.csv", Kind: "csv", Columns: []string{"Category", "This month", "Average"}, Rows: [][]string{
				{"Dining", "540.00", "470.00"},
				{"Groceries", "412.00", "395.00"},
				{"Transport", "1186.00", "1190.00"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Priya's shop sales by month — backs the Table widget on her business page.
			{ID: "art-shop-sales", Name: "shop-sales-by-month.csv", Kind: "csv", Columns: []string{"Month", "Revenue", "Orders", "Net"}, Rows: [][]string{
				{"Jan 2026", "$1,012", "45", "$718"},
				{"Feb 2026", "$1,031", "46", "$730"},
				{"Mar 2026", "$1,058", "47", "$752"},
				{"Apr 2026", "$1,076", "48", "$772"},
				{"May 2026", "$1,091", "50", "$766"},
				{"Jun 2026", "$1,108", "49", "$791"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Marcus's WSB positions — backs the Table widget on his hobbies page.
			{ID: "art-wsb", Name: "wsb-positions.csv", Kind: "csv", Columns: []string{"Position", "Cost basis", "Current value", "P&L"}, Rows: [][]string{
				{"NVDA calls (Jul)", "$1,200", "$2,640", "+$1,440"},
				{"TSLA shares", "$1,800", "$1,510", "-$290"},
				{"GME shares", "$640", "$415", "-$225"},
				{"SPY puts (Aug)", "$900", "$120", "-$780"},
				{"PLTR shares", "$500", "$815", "+$315"},
			}, CreatedAt: date(2026, time.June, 12)},

			// --- SVG chart artifacts (rendered as <img>) for the showcase pages ---
			// Priya: shop revenue (bar) and orders (bar).
			func() domain.Artifact {
				b := svgBarChart("Shop revenue by month", "#10b981",
					[]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					[]float64{1012, 1031, 1058, 1076, 1091, 1108}, dollarsLab)
				return domain.Artifact{ID: "art-shop-rev", Name: "shop-revenue.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 1)}
			}(),
			func() domain.Artifact {
				b := svgBarChart("Orders per month", "#22d3ee",
					[]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					[]float64{45, 46, 47, 48, 50, 49}, intLab)
				return domain.Artifact{ID: "art-shop-orders", Name: "shop-orders.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 1)}
			}(),
			// Marcus: WSB account value over time (line — the rollercoaster) and
			// side-project revenue (bar).
			func() domain.Artifact {
				b := svgLineChart("Brokerage value — five years", "#a78bfa",
					[]string{"Jul 21", "Jan 22", "Jul 22", "Jan 23", "Jul 23", "Jan 24", "May 24", "Jul 24", "Jul 25", "Jun 26"},
					[]float64{550, 4830, 80, 350, 220, 790, 14090, 6070, 6730, 8190}, dollarsLab)
				return domain.Artifact{ID: "art-wsb-value", Name: "wsb-value.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 12)}
			}(),
			func() domain.Artifact {
				b := svgBarChart("Side-project revenue", "#16a34a",
					[]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					[]float64{220, 0, 310, 90, 480}, dollarsLab)
				return domain.Artifact{ID: "art-sideproj", Name: "sideproject-revenue.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 1)}
			}(),

			// --- Priya's Business: operational artifacts a small shop tracks ---
			// Profit & loss by month.
			{ID: "art-shop-pnl", Name: "shop-pnl.csv", Kind: "csv", Columns: []string{"Month", "Revenue", "COGS", "Fees", "Net"}, Rows: [][]string{
				{"Jan 2026", "$1,012", "$253", "$101", "$658"},
				{"Feb 2026", "$1,031", "$258", "$103", "$670"},
				{"Mar 2026", "$1,058", "$264", "$106", "$688"},
				{"Apr 2026", "$1,076", "$269", "$108", "$699"},
				{"May 2026", "$1,091", "$273", "$109", "$709"},
				{"Jun 2026", "$1,108", "$277", "$111", "$720"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Best-selling products.
			{ID: "art-shop-products", Name: "top-products.csv", Kind: "csv", Columns: []string{"Product", "Units (mo)", "Revenue", "Margin"}, Rows: [][]string{
				{"Lavender soy candle", "31", "$465", "72%"},
				{"Chunky knit beanie", "12", "$300", "64%"},
				{"Eucalyptus candle", "9", "$144", "70%"},
				{"Wool scarf", "4", "$120", "61%"},
				{"Gift set (candle + card)", "5", "$175", "66%"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Cost breakdown for the month.
			{ID: "art-shop-costs", Name: "cost-breakdown.csv", Kind: "csv", Columns: []string{"Cost", "This month", "% of rev"}, Rows: [][]string{
				{"Materials (wax, wool, oils)", "$198", "18%"},
				{"Shipping & postage", "$86", "8%"},
				{"Packaging & labels", "$41", "4%"},
				{"Etsy + Shopify fees", "$110", "10%"},
				{"Software (Shopify)", "$39", "4%"},
				{"Marketing (ads)", "$30", "3%"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Inventory / stock with low-stock flags.
			{ID: "art-shop-inventory", Name: "inventory.csv", Kind: "csv", Columns: []string{"Item", "On hand", "Reorder at", "Status"}, Rows: [][]string{
				{"Soy wax (lb)", "6", "10", "LOW — reorder"},
				{"Lavender oil (oz)", "14", "8", "OK"},
				{"Wicks (ct)", "120", "100", "OK"},
				{"Jars 8oz (ct)", "22", "30", "LOW — reorder"},
				{"Wool yarn (skeins)", "31", "15", "OK"},
				{"Shipping boxes (ct)", "18", "25", "LOW — reorder"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Sales by channel.
			{ID: "art-shop-channels", Name: "sales-by-channel.csv", Kind: "csv", Columns: []string{"Channel", "Orders", "Revenue", "Share"}, Rows: [][]string{
				{"Etsy", "29", "$648", "59%"},
				{"Shopify (own site)", "14", "$361", "33%"},
				{"Local market", "6", "$87", "8%"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Open orders to fulfill.
			{ID: "art-shop-fulfillment", Name: "to-fulfill.csv", Kind: "csv", Columns: []string{"Order", "Item", "Status", "Ship by"}, Rows: [][]string{
				{"#1042", "Lavender candle ×2", "Packed", "Jun 13"},
				{"#1043", "Chunky beanie", "Making", "Jun 14"},
				{"#1044", "Gift set", "Awaiting wax", "Jun 16"},
				{"#1045", "Wool scarf", "Packed", "Jun 14"},
			}, CreatedAt: date(2026, time.June, 12)},
			// Tax set-aside (sales tax collected, owed quarterly).
			{ID: "art-shop-tax", Name: "tax-set-aside.csv", Kind: "csv", Columns: []string{"Quarter", "Sales tax collected", "Set aside", "Status"}, Rows: [][]string{
				{"Q1 2026", "$142", "$142", "Filed"},
				{"Q2 2026 (so far)", "$118", "$118", "Accruing"},
			}, CreatedAt: date(2026, time.June, 1)},
			// Average order value trend (line chart).
			func() domain.Artifact {
				b := svgLineChart("Avg order value", "#f59e0b",
					[]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					[]float64{22.8, 22.3, 22.4, 22.1, 22.4}, dollarsLab)
				return domain.Artifact{ID: "art-shop-aov", Name: "shop-aov.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 1)}
			}(),
			// Margin % trend (line chart).
			func() domain.Artifact {
				b := svgLineChart("Net margin %", "#10b981",
					[]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					[]float64{65, 65, 65, 65, 65}, intLab)
				return domain.Artifact{ID: "art-shop-margin", Name: "shop-margin.svg", Kind: "image", MIME: "image/svg+xml", Bytes: b, Size: len(b), CreatedAt: date(2026, time.June, 1)}
			}(),
		},
		Workflows: []workflow.Workflow{
			{ID: "wf-flag-big", Name: "Flag large purchases", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "txn_abs > 200", Actions: []workflow.Action{{Kind: workflow.ActionFlagReview}}},
			{ID: "wf-coffee", Name: "Categorize coffee runs", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: `contains(txn_payee, "coffee")`, Actions: []workflow.Action{{Kind: workflow.ActionSetCategory, CategoryID: catDining}, {Kind: workflow.ActionAddTag, Tag: "coffee"}}},
			{ID: "wf-baby", Name: "Tag baby expenses", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: `contains(txn_payee, "babylist")`, Actions: []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "baby"}}},
			{ID: "wf-review", Name: "Monthly budget review", Enabled: false, Trigger: workflow.Trigger{Kind: workflow.TriggerManual}, Actions: []workflow.Action{{Kind: workflow.ActionApplyRules}, {Kind: workflow.ActionCreateTask, Title: "Review last month's budgets", Notes: "Check overspent categories (dining!) and adjust."}, {Kind: workflow.ActionNotify, Message: "Monthly review complete."}}},
		},
		WorkflowRuns: []workflow.Run{
			{ID: "run-coffee-2026-06", WorkflowID: "wf-coffee", At: date(2026, time.June, 4).Format(time.RFC3339), Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionSetCategory, Summary: "Set category to Dining", CategoryID: catDining, TxnID: "tx-2026-06-coffee1"}}},
			{ID: "run-flag-2026-05", WorkflowID: "wf-flag-big", At: date(2026, time.May, 20).Format(time.RFC3339), Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionFlagReview, Summary: "Flagged for review", Tag: workflow.ReviewTag, TxnID: "tx-nursery-2026-05"}}},
			{ID: "run-review-2026-05", WorkflowID: "wf-review", At: date(2026, time.May, 31).Format(time.RFC3339), DryRun: true, Matched: true, Effects: []workflow.Effect{{Kind: workflow.ActionApplyRules, Summary: "Would categorize 4 transactions"}}},
		},
		SharedExpenses: []domain.SharedExpense{
			{ID: "se-dinner", Desc: "Dinner with friends", Date: date(2026, time.May, 24), PayerID: marcus, Shares: []domain.SharedExpenseShare{{MemberID: marcus, Amount: usd(5500)}, {MemberID: priya, Amount: usd(5500)}}},
			{ID: "se-groceries", Desc: "Shared groceries", Date: date(2026, time.June, 7), PayerID: priya, Shares: []domain.SharedExpenseShare{{MemberID: marcus, Amount: usd(3200)}, {MemberID: priya, Amount: usd(3200)}}},
		},
		Settlements: []domain.Settlement{
			{ID: "settle-1", FromID: priya, ToID: marcus, Amount: usd(5500), Date: date(2026, time.May, 26)},
		},
		// They cancelled MasterClass but got charged once more after — the Subscriptions
		// page surfaces both the cancelled state and the "charged after cancel" alert.
		SubscriptionCancellations: []domain.SubscriptionCancellation{
			{ID: "subcancel-masterclass", SubName: "MasterClass", CancelledOn: date(2025, time.January, 5)},
		},
		// Groceries get falsely flagged as a recurring charge; they marked it "not a
		// subscription", which drops it into the ignored list.
		SubscriptionIgnores: []domain.SubscriptionIgnore{
			{ID: "subignore-greenfield", SubName: "Greenfield Market", IgnoredOn: date(2026, time.April, 2)},
		},
		// Earmarks set aside money toward a debt (the card) and an account (the baby
		// fund) without yet moving it — exercises the allocation earmark surface.
		Earmarks: []domain.Earmark{
			{ID: "ear-card", DestinationID: card, DestinationKind: domain.EarmarkKindDebt, Amount: usd(50000), Currency: "USD", CreatedAt: date(2026, time.June, 2), Note: "Extra toward the card from the tax refund"},
			{ID: "ear-baby", DestinationID: hysa, DestinationKind: domain.EarmarkKindAccount, Amount: usd(30000), Currency: "USD", CreatedAt: date(2026, time.June, 2), Note: "Set aside for the baby fund"},
		},
		// A couple of saved Insights chats so the AI conversation list isn't empty.
		Conversations: []domain.Conversation{
			{ID: "conv-spending", Title: "Where's our money going?", Named: true, CreatedAt: date(2026, time.May, 28), UpdatedAt: date(2026, time.May, 28), Messages: []domain.ChatMessage{
				{ID: "cm-1", Role: "user", Text: "Where is most of our money going each month?", CreatedAt: date(2026, time.May, 28)},
				{ID: "cm-2", Role: "assistant", Text: "Your biggest controllable categories are dining (~$540/mo, well over the $300 budget), the two car payments (~$1,100/mo combined), and travel. Dining is the easiest lever — trimming it by half would free up ~$270 a month for the baby fund.", CreatedAt: date(2026, time.May, 28)},
			}},
			{ID: "conv-baby", Title: "Can we afford the baby?", Named: true, CreatedAt: date(2026, time.June, 6), UpdatedAt: date(2026, time.June, 6), Messages: []domain.ChatMessage{
				{ID: "cm-3", Role: "user", Text: "With the baby coming in December, are we going to be okay?", CreatedAt: date(2026, time.June, 6)},
				{ID: "cm-4", Role: "assistant", Text: "It's tight but workable. The biggest risks are the thin emergency fund (~1.5 months) and dining overspend. If you redirect the dining excess and a bit of the stonks deposits to the baby and emergency funds, you'll be in much steadier shape by the due date.", CreatedAt: date(2026, time.June, 6)},
			}},
		},
		// A short audit trail so the audit/undo view has real history to show.
		AuditEntries: []auditlog.Entry{
			{ID: "audit-1", At: date(2026, time.June, 10), Actor: "Marcus Hartley", Action: "create", EntityType: "transaction", EntityID: "tx-babyreg-2026-06", Summary: "Added 'Crib & registry items' ($450)"},
			{ID: "audit-2", At: date(2026, time.June, 4), Actor: "Priya Hartley", Action: "update", EntityType: "task", EntityID: "task-h-stroller", Summary: "Marked 'Research strollers' done"},
			{ID: "audit-3", At: date(2026, time.June, 2), Actor: "Marcus Hartley", Action: "create", EntityType: "earmark", EntityID: "ear-card", Summary: "Earmarked $500 toward the credit card"},
			{ID: "audit-4", At: date(2026, time.May, 26), Actor: "Marcus Hartley", Action: "create", EntityType: "settlement", EntityID: "settle-1", Summary: "Recorded settlement from Priya ($55)"},
		},
		Settings: Settings{
			BaseCurrency: "USD",
			// Rates are USD-per-foreign-unit (currency.Rates convention: Rates["EUR"]=1.08 ⇒ 1 EUR = $1.08).
			// The earlier seed used the inverse (foreign-per-USD), which mis-valued every non-USD account —
			// e.g. the €535 card showed $492 instead of ~$578, and 1 JPY read as $151 (a 22,000× error).
			FXRates:            map[string]float64{"EUR": 1.08, "GBP": 1.27, "CAD": 0.74, "JPY": 0.0066},
			FreshnessOverrides: map[string]int{checking: 7, k401: 90, roth: 90},
			// PayoffBaseline is computed from the dataset's real debt below so it
			// always reflects genuine progress (C198), not a stale hardcoded number.
		},
	}

	// C198: derive the debt-payoff baseline from the actual current debt rather
	// than hardcoding it. The old seed pinned TotalOwed at $39,500 "since Jul 1,
	// 2022" — far below the ~$100k actually owed — so progress always read 0%.
	// Here we sum the current owed across the included (non-mortgage) liabilities
	// and set the baseline a bit higher, so the card shows real, sensible progress.
	var currentOwed int64
	for _, a := range ds.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || !a.IncludedInPayoff() {
			continue
		}
		bal, err := ledger.Balance(a, ds.Transactions)
		if err != nil {
			continue
		}
		owed := bal.Amount
		if owed < 0 {
			owed = -owed
		}
		// All included sample debts are USD (the EUR travel card sits at €0), so a
		// raw minor-unit sum is correct here without FX conversion.
		currentOwed += owed
	}
	if currentOwed > 0 {
		// Treat the current balance as ~82% of what was owed when tracking began,
		// i.e. roughly 18% paid down since the start date — a believable story.
		baseline := currentOwed * 100 / 82
		ds.Settings.PayoffBaseline = &PayoffBaseline{TotalOwed: baseline, Currency: "USD", StartedAt: date(2025, time.September, 1)}
	}

	return ds
}

// boolN returns n when cond is true, else 0 — a small helper for conditionally
// adding to a transaction amount (e.g. a pregnancy-month grocery/shopping bump).
func boolN(cond bool, n int64) int64 {
	if cond {
		return n
	}
	return 0
}

// --- tiny SVG chart generators for the custom-page showcases ---
// These produce self-contained SVG image artifacts so each custom page shows a real,
// subject-relevant graph (the built-in chart widget only draws the net-worth trend).
// They render via cpImageBody as <img src="data:image/svg+xml;...">. Light text/grid
// so they read on either theme.

const (
	svgW, svgH       = 360.0, 188.0
	svgX0, svgX1     = 38.0, 348.0 // plot x range
	svgYTop, svgYBot = 42.0, 156.0 // plot y range (top..baseline)
)

func svgMaxf(vals []float64) float64 {
	m := 0.0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	if m <= 0 {
		m = 1
	}
	return m
}

func svgHeader(b *strings.Builder, title string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" font-family="ui-sans-serif,system-ui,sans-serif">`, svgW, svgH)
	// A self-contained dark chip: the chart's light-slate text/labels are baked
	// into the static artifact and can't adapt to the host card's theme, so
	// without this they wash out to pale-on-white in light mode. The rounded
	// dark background makes each chart legible on any surface.
	fmt.Fprintf(b, `<rect x="0" y="0" width="%.0f" height="%.0f" rx="10" fill="#1a1a1d"/>`, svgW, svgH)
	fmt.Fprintf(b, `<text x="14" y="24" font-size="14" font-weight="600" fill="#cbd5e1">%s</text>`, title)
	fmt.Fprintf(b, `<line x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f" stroke="#94a3b8" stroke-opacity="0.35"/>`, svgX0, svgYBot, svgX1, svgYBot)
}

// svgBarChart renders a labelled bar chart with value captions above each bar.
func svgBarChart(title, color string, labels []string, vals []float64, lab func(float64) string) []byte {
	maxV := svgMaxf(vals)
	var b strings.Builder
	svgHeader(&b, title)
	slot := (svgX1 - svgX0) / float64(len(vals))
	bw := slot * 0.54
	for i, v := range vals {
		h := (v / maxV) * (svgYBot - svgYTop)
		if h < 0 {
			h = 0
		}
		bx := svgX0 + slot*float64(i) + (slot-bw)/2
		by := svgYBot - h
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" fill="%s"/>`, bx, by, bw, h, color)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" fill="#cbd5e1" text-anchor="middle">%s</text>`, bx+bw/2, by-5, lab(v))
		if i < len(labels) {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" fill="#94a3b8" text-anchor="middle">%s</text>`, bx+bw/2, svgYBot+15, labels[i])
		}
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

// svgLineChart renders a polyline with point dots and value captions. The y-axis
// auto-ranges to the data's min..max (with padding) rather than starting at zero, so
// real variation shows instead of a near-flat line — good for both the WSB
// rollercoaster and subtler trends like average order value.
func svgLineChart(title, color string, labels []string, vals []float64, lab func(float64) string) []byte {
	var b strings.Builder
	svgHeader(&b, title)
	n := len(vals)
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	x := func(i int) float64 { return svgX0 + (svgX1-svgX0)*float64(i)/float64(n-1) }
	var y func(v float64) float64
	if hi-lo < 1e-9 { // flat series: draw a clean centered line
		mid := (svgYTop + svgYBot) / 2
		y = func(float64) float64 { return mid }
	} else {
		pad := (hi - lo) * 0.18
		lo, hi = lo-pad, hi+pad
		y = func(v float64) float64 { return svgYBot - (v-lo)/(hi-lo)*(svgYBot-svgYTop) }
	}
	var pts strings.Builder
	for i, v := range vals {
		fmt.Fprintf(&pts, "%.1f,%.1f ", x(i), y(v))
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2.5"/>`, strings.TrimSpace(pts.String()), color)
	for i, v := range vals {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3" fill="%s"/>`, x(i), y(v), color)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="9" fill="#cbd5e1" text-anchor="middle">%s</text>`, x(i), y(v)-7, lab(v))
		if i < len(labels) {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" fill="#94a3b8" text-anchor="middle">%s</text>`, x(i), svgYBot+15, labels[i])
		}
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

// dollarsLab / intLab format chart captions.
func dollarsLab(v float64) string {
	if v >= 1000 {
		return fmt.Sprintf("$%.1fk", v/1000)
	}
	return fmt.Sprintf("$%.0f", v)
}
func intLab(v float64) string { return fmt.Sprintf("%.0f", v) }
