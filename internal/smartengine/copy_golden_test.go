// SPDX-License-Identifier: MIT

package smartengine

// copy_golden_test.go — golden/snapshot tests for insight Title+Detail copy.
//
// The fixture is fully deterministic: fixed Now (2026-06-15), fixed amounts,
// no time.Now() calls anywhere. It exercises a handful of engines whose output
// is stable on this fixture and locks down the exact copy produced.
//
// A guard sub-test also scans every produced insight for copy anti-patterns:
// symbol-less 2-decimal amounts, bare currency codes in prose, and broken
// English plurals — regardless of which engine produced them.

import (
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// ── golden fixture ─────────────────────────────────────────────────────────

// goldenNow is the fixed reference clock used throughout these tests. It must
// match `ref` defined in accounts_test.go (the shared test var) so helpers
// like `baseInput()` and `goal()` continue to work in the same package.
var goldenNow = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

// goldenInput builds the deterministic snapshot used by every golden test.
// It contains:
//   - A $5,000 checking account with trailing-month baseline ($3,000/mo income,
//     $2,000/mo expense over the 3 prior months) → $1,000/mo surplus.
//   - One active goal (Vacation, $1,200 target, $0 saved, deadline Dec 1 2026)
//     → G1 fires with a concrete contribution figure.
//   - One emergency-fund goal ($12,000 target, $2,000 current, no deadline)
//     → G11 fires against the ~$2,000/mo expense baseline.
//   - Five recent uncategorized transactions → D1 fires.
func goldenInput() Input {
	in := baseInput() // USD, Now=2026-06-15

	// Trailing baseline: 3 prior months of income + expense.
	// Income $3,000/mo (minor 300000), expense $2,000/mo (minor -200000).
	// Both carry a CategoryID so they don't pollute the D1 uncategorized count.
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for k := 1; k <= 3; k++ {
		d := monthStart.AddDate(0, -k, 9)
		in.Transactions = append(in.Transactions,
			domain.Transaction{
				ID: "gi_inc" + itoa64(int64(k)), AccountID: "chk",
				Date: d, Amount: usd(300000), Desc: "Salary",
				CategoryID: "income", // categorized → excluded from D1
			},
			domain.Transaction{
				ID: "gi_exp" + itoa64(int64(k)), AccountID: "chk",
				Date: d, Amount: usd(-200000), Desc: "Rent",
				CategoryID: "rent", // categorized → excluded from D1
			},
		)
	}

	// Checking account with $5,000 opening balance.
	in.Accounts = []domain.Account{
		acct("chk", "Checking", domain.TypeChecking, 500000, goldenNow.AddDate(0, -6, 0)),
	}

	// Goals: Vacation (deadline) + Emergency Fund (no deadline).
	vacaDue := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{
		goal("vaca", "Vacation", 120000, 0, vacaDue),               // $1,200 target, deadline Dec 2026
		goal("ef", "Emergency Fund", 1200000, 200000, time.Time{}), // $12,000 target, $2,000 saved
	}

	// 5 recent uncategorized transactions → D1 fires.
	for i := range 5 {
		in.Transactions = append(in.Transactions, domain.Transaction{
			ID:        "gi_unc" + itoa64(int64(i)),
			AccountID: "chk",
			Date:      goldenNow.AddDate(0, 0, -i),
			Amount:    usd(-1000),
			Desc:      "Misc",
			// CategoryID deliberately empty → uncategorized
		})
	}

	return in
}

// ── golden assertions ──────────────────────────────────────────────────────

// TestGoldenD1AutoTodos locks the exact D1 copy when 5 recent uncategorized
// transactions are present.
func TestGoldenD1AutoTodos(t *testing.T) {
	in := goldenInput()
	got := d1AutoTodos(in)
	if len(got) != 1 {
		t.Fatalf("D1: want 1 insight, got %d: %+v", len(got), got)
	}
	ins := got[0]

	wantTitle := "5 transactions still need a category"
	wantDetail := "You have 5 recent uncategorized transactions. Categorizing them keeps your budgets and reports accurate."

	if ins.Title != wantTitle {
		t.Errorf("D1 Title\n got:  %q\n want: %q", ins.Title, wantTitle)
	}
	if ins.Detail != wantDetail {
		t.Errorf("D1 Detail\n got:  %q\n want: %q", ins.Detail, wantDetail)
	}
}

// TestGoldenG1SuggestedContribution locks G1 copy for the Vacation goal.
// The goal needs $1,200 over ~6.5 months from the fixture's Now → ~$185/mo.
// We don't lock the exact figure (it depends on goals.MonthlyNeeded rounding)
// but we do lock the structural template and confirm money uses a "$" symbol.
func TestGoldenG1SuggestedContribution(t *testing.T) {
	in := goldenInput()
	insights := g1SuggestedContribution(in)
	if len(insights) == 0 {
		t.Fatal("G1: want at least 1 insight, got 0")
	}
	// Sort for determinism in case multiple goals fire.
	sort.Slice(insights, func(i, j int) bool { return insights[i].Key < insights[j].Key })

	var vacaIns smart.Insight
	found := false
	for _, ins := range insights {
		if strings.Contains(ins.Key, "vaca") {
			vacaIns = ins
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("G1: no insight keyed to 'vaca': %+v", insights)
	}

	// Title must be "Save $X/mo for Vacation".
	if !strings.HasPrefix(vacaIns.Title, "Save $") {
		t.Errorf("G1 Title must start with 'Save $', got: %q", vacaIns.Title)
	}
	if !strings.Contains(vacaIns.Title, "/mo for Vacation") {
		t.Errorf("G1 Title must contain '/mo for Vacation', got: %q", vacaIns.Title)
	}

	// Detail must mention the target date and slack.
	if !strings.Contains(vacaIns.Detail, "Dec 2026") {
		t.Errorf("G1 Detail must contain 'Dec 2026', got: %q", vacaIns.Detail)
	}
}

// TestGoldenG11EmergencyFund locks G11 copy for the emergency-fund goal.
// Essentials ~$2,000/mo → $2,000 saved covers ~1 month of a 6-month target.
// Gap = 6 × $2,000 - $2,000 = $10,000.
func TestGoldenG11EmergencyFund(t *testing.T) {
	in := goldenInput()
	got := g11EmergencyFund(in)
	if len(got) != 1 {
		t.Fatalf("G11: want 1 insight, got %d: %+v", len(got), got)
	}
	ins := got[0]

	// Title must reference the coverage (e.g. "1.0 months") and "essentials".
	if !strings.Contains(ins.Title, "months") {
		t.Errorf("G11 Title must contain 'months', got: %q", ins.Title)
	}
	if !strings.Contains(ins.Title, "essentials") {
		t.Errorf("G11 Title must contain 'essentials', got: %q", ins.Title)
	}

	// Detail must show "Emergency Fund" (the goal name).
	if !strings.Contains(ins.Detail, "Emergency Fund") {
		t.Errorf("G11 Detail must reference 'Emergency Fund', got: %q", ins.Detail)
	}

	// Amount must be roughly $10,000 (800,000–1,100,000 minor to allow rounding).
	if ins.Amount.Amount < 800000 || ins.Amount.Amount > 1100000 {
		t.Errorf("G11 gap amount = %d, want ~1000000 (±200000)", ins.Amount.Amount)
	}

	// The gap figure in the Detail must be money-formatted (starts with "$").
	if !strings.Contains(ins.Detail, "$") {
		t.Errorf("G11 Detail must include a '$'-prefixed amount, got: %q", ins.Detail)
	}
}

// TestGoldenG3AllocateSurplus locks G3 copy for the surplus-to-goals nudge.
// Surplus = $3,000 - $2,000 = $1,000/mo; 1 active incomplete goal.
func TestGoldenG3AllocateSurplus(t *testing.T) {
	in := goldenInput()
	got := g3AllocateSurplus(in)
	if len(got) != 1 {
		t.Fatalf("G3: want 1 insight, got %d: %+v", len(got), got)
	}
	ins := got[0]

	// Title: "You're freeing up about $1,000/mo".
	wantTitle := "You're freeing up about $1,000/mo"
	if ins.Title != wantTitle {
		t.Errorf("G3 Title\n got:  %q\n want: %q", ins.Title, wantTitle)
	}

	// Detail must mention surplus and "active goal" (1 active goal).
	if !strings.Contains(ins.Detail, "$1,000/mo") {
		t.Errorf("G3 Detail must contain '$1,000/mo', got: %q", ins.Detail)
	}
	if !strings.Contains(ins.Detail, "active goal") {
		t.Errorf("G3 Detail must contain 'active goal', got: %q", ins.Detail)
	}
}

// ── guard sub-test (anti-pattern scanner) ─────────────────────────────────

// TestCopyGuard runs every engine on the golden fixture (and a few variant
// inputs that make more engines fire) and scans every Title+Detail for the
// copy anti-patterns the style guide forbids.
func TestCopyGuard(t *testing.T) {
	all := collectAllInsights(t)

	// Sort by Key for stable output in failure messages.
	sort.Slice(all, func(i, j int) bool { return all[i].Key < all[j].Key })

	// Anti-pattern: symbol-less 2-decimal amount.
	// Matches a decimal amount like "1234.56" NOT preceded by a currency symbol
	// ($, €, £, ¥). A leading digit or space directly before the pattern is the
	// problem; "$1,234.56" is fine because it is preceded by "$".
	// We look for the pattern: digit(s) followed by "." followed by exactly 2
	// digits, where the character immediately before the digit run is NOT a
	// currency symbol or another digit (which would indicate a longer decimal).
	reSymbolless := regexp.MustCompile(`(?:^|[^$€£¥\d])(\d{1,3}(?:,\d{3})*\.\d{2})(?:[^%\d]|$)`)

	// Anti-pattern: bare ISO currency code in prose.
	reBareCode := regexp.MustCompile(`\b(USD|EUR|GBP|JPY|CAD|AUD)\b`)

	// Anti-pattern: broken English plurals our pluralizer should have caught.
	reBrokenPlural := regexp.MustCompile(`\b(entrys|categorys|daies|goales|billses)\b`)

	for _, ins := range all {
		text := ins.Title + " " + ins.Detail

		// Check for symbol-less 2-decimal amounts.
		if m := reSymbolless.FindString(text); m != "" {
			t.Errorf("copy guard [%s] — symbol-less amount near %q\n  full: %q",
				ins.Key, strings.TrimSpace(m), text)
		}

		// Check for bare currency codes.
		if m := reBareCode.FindString(text); m != "" {
			t.Errorf("copy guard [%s] — bare currency code %q in prose\n  full: %q",
				ins.Key, m, text)
		}

		// Check for broken plurals.
		if m := reBrokenPlural.FindString(text); m != "" {
			t.Errorf("copy guard [%s] — broken plural %q\n  full: %q",
				ins.Key, m, text)
		}
	}
}

// collectAllInsights runs every registered engine over a rich variant of the
// golden fixture and returns all produced insights. Engines that need specific
// conditions (bills, subscriptions, etc.) are exercised with supplemental data.
func collectAllInsights(t *testing.T) []smart.Insight {
	t.Helper()

	// Base golden fixture.
	in := goldenInput()

	// Supplement: a high-APR liability (fires BL6, BL3, BL13, AL1-debt).
	card := domain.Account{
		ID: "visa", Name: "Visa", Type: domain.TypeCreditCard,
		Class: domain.ClassLiability, Currency: "USD",
		DueDayOfMonth: 18, MinPayment: usd(2500),
		OpeningBalance: usd(-200000), InterestRateAPR: 22.0,
	}
	in.Accounts = append(in.Accounts, card)

	// Supplement: a savings account with a yield for A4 cash-positioning.
	sav := acct("sav", "HY Savings", domain.TypeSavings, 200000, goldenNow.AddDate(0, -1, 0))
	sav.ExpectedReturnAPR = 4.5
	in.Accounts = append(in.Accounts, sav)

	// Supplement: a recurring monthly bill for BL8/BL10/BL5.
	in.Recurring = []domain.Recurring{
		{
			ID: "phone", Label: "Phone", Amount: usd(-8000),
			Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "internet", Label: "Internet", Amount: usd(-6000),
			Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		},
	}

	// Supplement: income transaction so payday-based engines (BL8, BL5) fire.
	in.Transactions = append(in.Transactions, domain.Transaction{
		ID: "gi_salary_curr", AccountID: "chk",
		Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Amount: usd(300000), Desc: "Salary",
	})

	// Supplement: a recurring subscription series for SU4/SU11/SU1.
	for _, m := range []time.Month{time.March, time.April, time.May, time.June} {
		in.Transactions = append(in.Transactions, domain.Transaction{
			ID: "netflix_" + m.String(), AccountID: "chk",
			Date:   time.Date(2026, m, 8, 0, 0, 0, 0, time.UTC),
			Amount: usd(-1599), Desc: "Netflix",
		})
	}

	// Supplement: a Dining category + typical expenses for B9/B10/T6/T11.
	in.Categories = []domain.Category{
		{ID: "dining", Name: "Dining"},
		{ID: "rent", Name: "Rent"},
	}
	// Re-categorize existing rent expenses.
	var reCat []domain.Transaction
	for _, txn := range in.Transactions {
		if txn.Desc == "Rent" {
			txn.CategoryID = "rent"
		}
		reCat = append(reCat, txn)
	}
	in.Transactions = reCat

	// A dining budget that is near-busted (for B9 pacing).
	in.Budgets = []domain.Budget{
		{
			ID: "bdining", Name: "Dining", CategoryID: "dining",
			Period: domain.PeriodMonthly,
			Limit:  usd(20000), // $200 budget
			Scope:  domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		},
	}
	// Dining spend this month: $180 (90% of budget) for B9 to fire.
	in.Transactions = append(in.Transactions, domain.Transaction{
		ID: "gi_dining1", AccountID: "chk", CategoryID: "dining",
		Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-9000), Desc: "Restaurant",
	}, domain.Transaction{
		ID: "gi_dining2", AccountID: "chk", CategoryID: "dining",
		Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), Amount: usd(-9000), Desc: "Delivery",
	})

	var all []smart.Insight
	for code, fn := range engines {
		results := fn(in)
		_ = code
		all = append(all, results...)
	}
	return all
}
