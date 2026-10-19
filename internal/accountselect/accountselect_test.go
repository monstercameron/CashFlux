// SPDX-License-Identifier: MIT

package accountselect_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// helpers

func acct(id string, typ domain.AccountType, class domain.AccountClass, archived bool) domain.Account {
	return domain.Account{
		ID:       id,
		Type:     typ,
		Class:    class,
		Archived: archived,
	}
}

func txn(id, accountID string, date time.Time) domain.Transaction {
	return domain.Transaction{
		ID:        id,
		AccountID: accountID,
		Date:      date,
	}
}

var (
	day0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	day1 = day0.AddDate(0, 0, 1)
	day2 = day0.AddDate(0, 0, 2)
	day3 = day0.AddDate(0, 0, 3)
)

func TestDefaultID(t *testing.T) {
	checking := acct("chk1", domain.TypeChecking, domain.ClassAsset, false)
	savings := acct("sav1", domain.TypeSavings, domain.ClassAsset, false)
	invest := acct("inv1", domain.TypeInvestment, domain.ClassAsset, false)
	credit := acct("cc1", domain.TypeCreditCard, domain.ClassLiability, false)
	archivedChk := acct("archChk", domain.TypeChecking, domain.ClassAsset, true)
	cash := acct("cash1", domain.TypeCash, domain.ClassAsset, false)

	tests := []struct {
		name            string
		accounts        []domain.Account
		txns            []domain.Transaction
		memberDefaultID string
		want            string
	}{
		// --- Tier 1 tests ---
		{
			name:            "tier1: valid member default returns that account",
			accounts:        []domain.Account{checking, savings},
			txns:            nil,
			memberDefaultID: "sav1",
			want:            "sav1",
		},
		{
			name:            "tier1: archived member default falls through to next tier",
			accounts:        []domain.Account{archivedChk, checking},
			txns:            nil,
			memberDefaultID: "archChk",
			want:            "chk1", // falls to tier 3
		},
		{
			name:            "tier1: unknown member default falls through",
			accounts:        []domain.Account{checking},
			txns:            nil,
			memberDefaultID: "does-not-exist",
			want:            "chk1",
		},
		{
			name:            "tier1: empty memberDefaultID skips tier1",
			accounts:        []domain.Account{checking},
			txns:            nil,
			memberDefaultID: "",
			want:            "chk1",
		},
		// --- Tier 2 tests ---
		{
			name:     "tier2: most-used account in 90-day window wins",
			accounts: []domain.Account{checking, savings},
			txns: []domain.Transaction{
				txn("t1", "sav1", day0),
				txn("t2", "sav1", day1),
				txn("t3", "chk1", day2),
			},
			memberDefaultID: "",
			// sav1 has 2 txns, chk1 has 1 — sav1 wins
			want: "sav1",
		},
		{
			name:     "tier2: transactions outside 90-day window are not counted",
			accounts: []domain.Account{checking, savings},
			txns: []domain.Transaction{
				// max date = day3; window starts 90 days before day3
				// day0 is within 90 days of day3 in this test (just 3 days back)
				txn("t1", "chk1", day3),                       // chk1: 1 in window
				txn("t2", "sav1", day3.Add(-91*24*time.Hour)), // sav1: outside window
			},
			memberDefaultID: "",
			want:            "chk1",
		},
		{
			name:     "tier2: investment accounts excluded from frequency tier",
			accounts: []domain.Account{invest, checking},
			txns: []domain.Transaction{
				txn("t1", "inv1", day0),
				txn("t2", "inv1", day1),
				txn("t3", "chk1", day2),
			},
			memberDefaultID: "",
			// inv1 is excluded; chk1 has 1
			want: "chk1",
		},
		{
			name:     "tier2: liability accounts excluded from frequency tier",
			accounts: []domain.Account{credit, checking},
			txns: []domain.Transaction{
				txn("t1", "cc1", day0),
				txn("t2", "cc1", day1),
				txn("t3", "chk1", day2),
			},
			memberDefaultID: "",
			want:            "chk1",
		},
		{
			name:     "tier2: ties broken by lexicographic ascending account ID",
			accounts: []domain.Account{checking, savings},
			txns: []domain.Transaction{
				txn("t1", "chk1", day0),
				txn("t2", "sav1", day1),
			},
			memberDefaultID: "",
			// both have 1 txn; "chk1" < "sav1" lexicographically
			want: "chk1",
		},
		{
			name:     "tier2: zero count falls through to tier3",
			accounts: []domain.Account{checking, savings},
			txns:     []domain.Transaction{},
			// no txns → freq tier skipped → tier3
			memberDefaultID: "",
			want:            "chk1",
		},
		// --- Tier 3 tests ---
		{
			name:            "tier3: first checking/debit/savings asset when no txns",
			accounts:        []domain.Account{invest, cash, savings, checking},
			txns:            nil,
			memberDefaultID: "",
			// invest excluded; cash skipped in tier3; savings is first checking-like
			want: "sav1",
		},
		{
			name:            "tier3: debit account qualifies",
			accounts:        []domain.Account{acct("deb1", domain.TypeDebit, domain.ClassAsset, false)},
			txns:            nil,
			memberDefaultID: "",
			want:            "deb1",
		},
		// --- Tier 4 tests ---
		{
			name:            "tier4: cash account returned when no checking-like",
			accounts:        []domain.Account{invest, cash},
			txns:            nil,
			memberDefaultID: "",
			// invest excluded; cash is non-investment asset → tier4
			want: "cash1",
		},
		{
			name:            "tier4: investment excluded even in tier4",
			accounts:        []domain.Account{invest},
			txns:            nil,
			memberDefaultID: "",
			want:            "",
		},
		// --- Tier 5 tests ---
		{
			name:            "tier5: empty accounts returns empty string",
			accounts:        nil,
			txns:            nil,
			memberDefaultID: "",
			want:            "",
		},
		{
			name:            "tier5: only archived accounts returns empty string",
			accounts:        []domain.Account{archivedChk},
			txns:            nil,
			memberDefaultID: "",
			want:            "",
		},
		{
			name:            "tier5: only liability and investment returns empty string",
			accounts:        []domain.Account{credit, invest},
			txns:            nil,
			memberDefaultID: "",
			want:            "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := accountselect.DefaultID(tc.accounts, tc.txns, tc.memberDefaultID)
			if got != tc.want {
				t.Errorf("DefaultID(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	checking := acct("checking", domain.TypeChecking, domain.ClassAsset, false)
	savings := acct("savings", domain.TypeSavings, domain.ClassAsset, false)
	cash := acct("cash", domain.TypeCash, domain.ClassAsset, false)
	archived := acct("archived", domain.TypeChecking, domain.ClassAsset, true)
	investment := acct("investment", domain.TypeInvestment, domain.ClassAsset, false)
	member := domain.Member{ID: "member-1", Prefs: domain.MemberPrefs{DefaultAccountID: "savings"}}
	otherMember := domain.Member{ID: "member-2", Prefs: domain.MemberPrefs{DefaultAccountID: "cash"}}

	tests := []struct {
		name               string
		accounts           []domain.Account
		txns               []domain.Transaction
		explicitID         string
		activeMemberID     string
		members            []domain.Member
		workspaceDefaultID string
		wantID             string
		wantTier           accountselect.Tier
	}{
		{
			name:               "valid explicit choice wins over both saved defaults",
			accounts:           []domain.Account{checking, savings},
			explicitID:         "checking",
			activeMemberID:     "member-1",
			members:            []domain.Member{member},
			workspaceDefaultID: "savings",
			wantID:             "checking",
			wantTier:           accountselect.TierExplicit,
		},
		{
			name:               "member lens uses that member default",
			accounts:           []domain.Account{checking, savings, cash},
			activeMemberID:     "member-1",
			members:            []domain.Member{member, otherMember},
			workspaceDefaultID: "checking",
			wantID:             "savings",
			wantTier:           accountselect.TierMember,
		},
		{
			name:               "member without a default falls through to workspace default",
			accounts:           []domain.Account{checking, savings},
			activeMemberID:     "member-2",
			members:            []domain.Member{{ID: "member-2"}},
			workspaceDefaultID: "savings",
			wantID:             "savings",
			wantTier:           accountselect.TierWorkspace,
		},
		{
			name:               "member without a lens uses workspace default",
			accounts:           []domain.Account{checking, savings},
			workspaceDefaultID: "savings",
			wantID:             "savings",
			wantTier:           accountselect.TierWorkspace,
		},
		{
			name:               "missing member default is skipped without blocking workspace default",
			accounts:           []domain.Account{checking, savings},
			activeMemberID:     "member-1",
			members:            []domain.Member{{ID: "member-1", Prefs: domain.MemberPrefs{DefaultAccountID: "deleted"}}},
			workspaceDefaultID: "savings",
			wantID:             "savings",
			wantTier:           accountselect.TierWorkspace,
		},
		{
			name:               "archived member and workspace defaults fall through to heuristic",
			accounts:           []domain.Account{archived, checking},
			activeMemberID:     "member-1",
			members:            []domain.Member{{ID: "member-1", Prefs: domain.MemberPrefs{DefaultAccountID: "archived"}}},
			workspaceDefaultID: "archived",
			wantID:             "checking",
			wantTier:           accountselect.TierHeuristic,
		},
		{
			name:       "invalid explicit choice is skipped",
			accounts:   []domain.Account{archived, savings},
			explicitID: "archived",
			wantID:     "savings",
			wantTier:   accountselect.TierHeuristic,
		},
		{
			name:               "workspace default wins before transaction heuristic",
			accounts:           []domain.Account{checking, savings},
			txns:               []domain.Transaction{txn("t1", "checking", day0), txn("t2", "checking", day1)},
			workspaceDefaultID: "savings",
			wantID:             "savings",
			wantTier:           accountselect.TierWorkspace,
		},
		{
			name:     "heuristic fallback is deterministic by account ID on transaction tie",
			accounts: []domain.Account{savings, checking},
			txns:     []domain.Transaction{txn("t1", "checking", day0), txn("t2", "savings", day1)},
			wantID:   "checking",
			wantTier: accountselect.TierHeuristic,
		},
		{
			name:     "first non-archived account is final fallback when heuristic has no match",
			accounts: []domain.Account{archived, investment, acct("loan", domain.TypeOther, domain.ClassLiability, false)},
			wantID:   "investment",
			wantTier: accountselect.TierFirstAccount,
		},
		{
			name:     "no accounts returns no selection",
			accounts: nil,
			wantID:   "",
			wantTier: accountselect.TierNone,
		},
		{
			name:     "only archived or unidentified accounts return no selection",
			accounts: []domain.Account{archived, acct("", domain.TypeCash, domain.ClassAsset, false)},
			wantID:   "",
			wantTier: accountselect.TierNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotTier := accountselect.Resolve(accountselect.ResolveInput{
				Accounts:           tc.accounts,
				Transactions:       tc.txns,
				ExplicitID:         tc.explicitID,
				ActiveMemberID:     tc.activeMemberID,
				Members:            tc.members,
				WorkspaceDefaultID: tc.workspaceDefaultID,
			})
			if gotID != tc.wantID || gotTier != tc.wantTier {
				t.Errorf("Resolve() = (%q, %q), want (%q, %q)", gotID, gotTier, tc.wantID, tc.wantTier)
			}
		})
	}
}
