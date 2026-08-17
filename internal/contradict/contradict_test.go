// SPDX-License-Identifier: MIT

package contradict

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var day = time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)

func acct(id string, opening int64) domain.Account {
	return domain.Account{ID: id, Name: id, Currency: "USD", OpeningBalance: money.New(opening, "USD")}
}

// leg builds one side of a transfer.
func leg(id, from, to string, minor int64, at time.Time) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: from, TransferAccountID: to,
		Desc: "transfer", Date: at, Amount: money.New(minor, "USD")}
}

func kindsOf(fs []Finding) map[Kind]int {
	out := map[Kind]int{}
	for _, f := range fs {
		out[f.Kind]++
	}
	return out
}

// A transfer is two rows. One leg alone means money left an account and arrived
// nowhere, and every total spanning the two accounts is wrong by that amount.
func TestOneSidedTransfer(t *testing.T) {
	d := Data{
		Accounts:     []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{leg("t1", "a", "b", -50000, day)},
		Now:          day,
	}
	got := Check(d)
	if kindsOf(got)[KindOneSidedTransfer] != 1 {
		t.Fatalf("findings = %+v, want one one-sided transfer", got)
	}
	f := got[0]
	if f.Severity != SeverityCritical {
		t.Errorf("severity = %v, want critical", f.Severity)
	}
	if !f.HasDelta || f.DeltaMinor != 50000 {
		t.Errorf("delta = %d,%v want 50000,true", f.DeltaMinor, f.HasDelta)
	}
	// Both sides, always. A finding with one side is a complaint, and a user
	// cannot act on a complaint because they do not know which side to fix.
	if f.Left == "" || f.Right == "" {
		t.Errorf("finding shows only one side: %+v", f)
	}
}

// A matched pair must produce nothing, and must not be reported twice under two
// keys — judged from the outgoing side only.
func TestMatchedTransferIsClean(t *testing.T) {
	d := Data{
		Accounts: []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, day),
			leg("t2", "b", "a", 50000, day),
		},
		Now: day,
	}
	if got := Check(d); len(got) != 0 {
		t.Errorf("a matched transfer produced %+v", got)
	}
}

// The legs must sum to zero. If they do not, the household's net moved although
// no money entered or left it.
func TestTransferLegsThatDisagree(t *testing.T) {
	d := Data{
		Accounts: []domain.Account{acct("a", 0), acct("b", 0)},
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, day),
			leg("t2", "b", "a", 47000, day),
		},
		Now: day,
	}
	got := Check(d)
	if kindsOf(got)[KindTransferLegsDisagree] != 1 {
		t.Fatalf("findings = %+v, want one legs-disagree", got)
	}
	if got[0].DeltaMinor != -3000 {
		t.Errorf("delta = %d, want -3000", got[0].DeltaMinor)
	}
}

// Two surfaces read these numbers — the investments page totals holdings, the
// accounts page reads the balance — so a mismatch is the exact defect shape:
// both screens individually correct, disagreeing with each other.
func TestHoldingsThatDoNotMatchTheBalance(t *testing.T) {
	a := acct("inv", 1000000) // $10,000
	d := Data{
		Accounts: []domain.Account{a},
		Holdings: []domain.Holding{
			{ID: "h1", AccountID: "inv", Ticker: "VTI", Shares: 10, CurrentPriceMinorPerShare: 20000},
		},
		Now: day,
	}
	got := Check(d)
	if kindsOf(got)[KindHoldingsVsBalance] != 1 {
		t.Fatalf("findings = %+v, want a holdings-vs-balance", got)
	}
	// 10 × $200 = $2,000 of positions against a $10,000 balance.
	if got[0].DeltaMinor != 200000-1000000 {
		t.Errorf("delta = %d", got[0].DeltaMinor)
	}
}

// Prices are entered by hand and go stale; a few dollars is a stale price, not a
// disagreement, and reporting it would bury the real ones.
func TestHoldingsWithinToleranceAreClean(t *testing.T) {
	a := acct("inv", 200000+HoldingsTolerance-1)
	d := Data{
		Accounts: []domain.Account{a},
		Holdings: []domain.Holding{
			{ID: "h1", AccountID: "inv", Shares: 10, CurrentPriceMinorPerShare: 20000},
		},
		Now: day,
	}
	if got := Check(d); len(got) != 0 {
		t.Errorf("a within-tolerance difference produced %+v", got)
	}
}

// Not having entered positions yet is a blank, not a contradiction. Reporting it
// would bury the real findings under every unfilled account.
func TestAnAccountWithNoHoldingsIsNotAContradiction(t *testing.T) {
	d := Data{Accounts: []domain.Account{acct("inv", 5000000)}, Now: day}
	if got := Check(d); len(got) != 0 {
		t.Errorf("an account with no holdings produced %+v", got)
	}
}

// Work that is already done, still sitting in the list, teaches people the list
// is stale and to stop reading it.
func TestTaskOpenAfterItsGoalWasReached(t *testing.T) {
	d := Data{
		Goals: []domain.Goal{{ID: "g1", Name: "Fund", TargetAmount: money.New(100000, "USD"),
			CurrentAmount: money.New(100000, "USD")}},
		Tasks: []domain.Task{
			{ID: "t1", Title: "Top up the fund", Status: domain.StatusOpen,
				RelatedType: domain.RelatedGoal, RelatedID: "g1"},
			// A task on an unreached goal is fine.
			{ID: "t2", Title: "Keep saving", Status: domain.StatusOpen,
				RelatedType: domain.RelatedGoal, RelatedID: "g2"},
			// A done task on a reached goal is fine.
			{ID: "t3", Title: "Done", Status: domain.StatusDone,
				RelatedType: domain.RelatedGoal, RelatedID: "g1"},
		},
		Now: day,
	}
	got := Check(d)
	if kindsOf(got)[KindTaskOpenAfterAction] != 1 {
		t.Fatalf("findings = %+v, want exactly one stale task", got)
	}
	if got[0].EntityID != "t1" {
		t.Errorf("flagged %q, want t1", got[0].EntityID)
	}
}

// An orphaned link reads as a working one until followed, which is worse than no
// link: the reader believes there is context there.
func TestOrphanedLinks(t *testing.T) {
	d := Data{
		Accounts:   []domain.Account{acct("a", 0)},
		Categories: []domain.Category{{ID: "c-real", Name: "Dining"}},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "a", CategoryID: "c-gone", Desc: "x",
				Date: day, Amount: money.New(-100, "USD")},
			{ID: "t2", AccountID: "acct-gone", CategoryID: "c-real", Desc: "y",
				Date: day, Amount: money.New(-100, "USD")},
			{ID: "t3", AccountID: "a", CategoryID: "c-real", Desc: "z",
				Date: day, Amount: money.New(-100, "USD")},
		},
		Now: day,
	}
	got := Check(d)
	if n := kindsOf(got)[KindOrphanedLink]; n != 2 {
		t.Fatalf("got %d orphaned links, want 2: %+v", n, got)
	}
}

// One category budgeted twice means its spending counts against two limits, so
// neither "spent" figure is the truth and the two disagree by construction.
func TestACategoryBudgetedTwice(t *testing.T) {
	d := Data{
		Budgets: []domain.Budget{
			{ID: "b1", Name: "Food", CategoryID: "c-dining"},
			{ID: "b2", Name: "Eating out", CategoryID: "c-dining"},
			{ID: "b3", Name: "Travel", CategoryID: "c-travel"},
		},
		Now: day,
	}
	got := Check(d)
	if kindsOf(got)[KindDuplicateBudgetCategory] != 1 {
		t.Fatalf("findings = %+v, want one duplicate", got)
	}
	if got[0].EntityID != "b2" {
		t.Errorf("flagged %q, want the second budget", got[0].EntityID)
	}
}

// A budget can carry its categories in either field; a check that reads only one
// of them misses half the data.
func TestDuplicateDetectionCoversMultiCategoryBudgets(t *testing.T) {
	d := Data{
		Budgets: []domain.Budget{
			{ID: "b1", Name: "Food", CategoryIDs: []string{"c-dining", "c-groceries"}},
			{ID: "b2", Name: "Groceries", CategoryID: "c-groceries"},
		},
		Now: day,
	}
	if kindsOf(Check(d))[KindDuplicateBudgetCategory] != 1 {
		t.Errorf("a multi-category budget's overlap was missed: %+v", Check(d))
	}
}

// Findings sort most-severe-first, then stably by key, so the list does not
// reshuffle between renders.
func TestFindingsAreOrderedAndStable(t *testing.T) {
	d := Data{
		Accounts:   []domain.Account{acct("a", 0), acct("b", 0)},
		Categories: nil,
		Transactions: []domain.Transaction{
			leg("t1", "a", "b", -50000, day),
			{ID: "t2", AccountID: "a", CategoryID: "c-gone", Desc: "x",
				Date: day, Amount: money.New(-100, "USD")},
		},
		Now: day,
	}
	got := Check(d)
	if len(got) < 2 {
		t.Fatalf("expected at least two findings, got %+v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Severity < got[i].Severity {
			t.Errorf("findings are not most-severe-first: %v then %v", got[i-1].Severity, got[i].Severity)
		}
	}
	// Same input, same order.
	again := Check(d)
	for i := range got {
		if got[i].Key != again[i].Key {
			t.Fatalf("order changed between identical runs at %d", i)
		}
	}
}

// A key must be stable across recomputes so a dismissal sticks — which means it
// cannot be built from values that change as the user works.
func TestKeysDoNotDependOnValues(t *testing.T) {
	mk := func(minor int64) []Finding {
		return Check(Data{
			Accounts:     []domain.Account{acct("a", 0), acct("b", 0)},
			Transactions: []domain.Transaction{leg("t1", "a", "b", minor, day)},
			Now:          day,
		})
	}
	a, b := mk(-50000), mk(-90000)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("got %d and %d findings", len(a), len(b))
	}
	if a[0].Key != b[0].Key {
		t.Errorf("key changed with the amount: %q vs %q — a dismissal would not stick", a[0].Key, b[0].Key)
	}
}

func TestSummarizeAndClean(t *testing.T) {
	if s := Summarize(nil); !s.Clean() || s.Total() != 0 {
		t.Errorf("empty summary = %+v", s)
	}
	s := Summarize([]Finding{
		{Severity: SeverityCritical}, {Severity: SeverityWarning},
		{Severity: SeverityWarning}, {Severity: SeverityNotice},
	})
	if s.Critical != 1 || s.Warning != 2 || s.Notice != 1 || s.Total() != 4 || s.Clean() {
		t.Errorf("Summarize = %+v", s)
	}
}

func TestDescribeStatesBothSides(t *testing.T) {
	got := Describe(Finding{Left: "the bill is unpaid", Right: "a payment for it exists"})
	if got != "the bill is unpaid, but a payment for it exists" {
		t.Errorf("Describe = %q", got)
	}
}

// Clean data must produce nothing at all — a detector that always finds
// something is one people learn to ignore.
func TestCleanDataProducesNothing(t *testing.T) {
	d := Data{
		Accounts:   []domain.Account{acct("a", 100000)},
		Categories: []domain.Category{{ID: "c1", Name: "Dining"}},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "a", CategoryID: "c1", Desc: "x",
				Date: day, Amount: money.New(-2500, "USD")},
		},
		Budgets: []domain.Budget{{ID: "b1", Name: "Food", CategoryID: "c1"}},
		Goals:   []domain.Goal{{ID: "g1", TargetAmount: money.New(100000, "USD")}},
		Tasks: []domain.Task{{ID: "t-open", Status: domain.StatusOpen,
			RelatedType: domain.RelatedGoal, RelatedID: "g1"}},
		Now: day,
	}
	if got := Check(d); len(got) != 0 {
		t.Errorf("clean data produced %+v", got)
	}
}
