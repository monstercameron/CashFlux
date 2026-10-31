package accountedit

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// valid returns a complete, valid checking account draft.
func valid() Fields {
	return Fields{
		Name: "  Everyday Checking ", Type: string(domain.TypeChecking), Owner: domain.GroupOwnerID,
		Currency: "usd", Opening: "1250.00", RevalueDays: "", SnoozeUntil: "", LockUntil: "",
	}
}

func issuesOf(t *testing.T, got []Issue, want ...Issue) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("issues = %+v, want %+v", got, want)
	}
}

func TestApplyBuildsACheckingAccount(t *testing.T) {
	a, issues := Apply(domain.Account{ID: "acc-1"}, valid())
	issuesOf(t, issues)
	if a.ID != "acc-1" || a.Name != "Everyday Checking" || a.Currency != "USD" {
		t.Fatalf("identity fields = %q %q %q", a.ID, a.Name, a.Currency)
	}
	if a.Class != domain.ClassAsset || a.Type != domain.TypeChecking {
		t.Fatalf("class/type = %q/%q, want asset/checking", a.Class, a.Type)
	}
	if a.Scope != domain.ScopeShared {
		t.Fatalf("scope = %q, want shared for the group owner", a.Scope)
	}
	if a.OpeningBalance.Amount != 125000 {
		t.Fatalf("opening = %d minor units, want 125000", a.OpeningBalance.Amount)
	}
}

func TestApplyRequiresAName(t *testing.T) {
	f := valid()
	f.Name = "   "
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldName, Reason: Required})
}

func TestApplyRejectsAnUnreadableOpeningBalance(t *testing.T) {
	f := valid()
	f.Opening = "twelve"
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldOpening, Reason: Invalid})
}

func TestApplyTreatsAnOtherAccountAsALiabilityOnlyWhenAskedTo(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeOther)
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Class != domain.ClassAsset {
		t.Fatalf("other without the debt override = %q, want asset", a.Class)
	}
	f.AsLiability = true
	a, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Class != domain.ClassLiability {
		t.Fatalf("other with the debt override = %q, want liability", a.Class)
	}
}

func TestApplyKeepsABlankAPRAsNoRateAndZeroAsZeroPercent(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeCreditCard)
	f.APR = ""
	blank, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if _, ok := blank.RateAPR(); ok {
		t.Fatal("a blank APR recorded a rate")
	}
	f.APR = "0"
	zero, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if r, ok := zero.RateAPR(); !ok || r != 0 {
		t.Fatalf("a typed 0 APR = %v, %v, want 0 recorded", r, ok)
	}
}

func TestApplyFlagsANonNumericAPR(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeCreditCard)
	f.APR = "nine"
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldAPR, Reason: Invalid})
}

func TestApplyLiabilityClearsTheAssetOnlyFields(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeCreditCard)
	f.CreditLimit = "5000"
	f.DueDay = "15"
	f.Lender = "  first bank "
	f.LockUntil = "2030-01-01"
	base := domain.Account{LiquidityScore: 40, StabilityScore: 60, ExpectedReturnAPR: 4}
	a, issues := Apply(base, f)
	issuesOf(t, issues)
	if a.LiquidityScore != 0 || a.StabilityScore != 0 || a.ExpectedReturnAPR != 0 {
		t.Fatalf("asset figures survived a liability: %+v", a)
	}
	if a.CreditLimit.Amount != 500000 || a.DueDayOfMonth != 15 || a.Lender != "first bank" {
		t.Fatalf("liability fields = limit %d due %d lender %q", a.CreditLimit.Amount, a.DueDayOfMonth, a.Lender)
	}
}

func TestApplyAssetClearsTheDebtOnlyFields(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeSavings)
	f.Liquidity = "80"
	f.Stability = "90"
	f.LockUntil = "2031-06-30"
	base := domain.Account{CreditLimit: money.New(100, "USD"), DueDayOfMonth: 3, Lender: "old lender", StatementDay: 9}
	a, issues := Apply(base, f)
	issuesOf(t, issues)
	if a.CreditLimit.Amount != 0 || a.DueDayOfMonth != 0 || a.Lender != "" || a.StatementDay != 0 {
		t.Fatalf("debt fields survived an asset: %+v", a)
	}
	if a.LiquidityScore != 80 || a.StabilityScore != 90 {
		t.Fatalf("scores = %d/%d, want 80/90", a.LiquidityScore, a.StabilityScore)
	}
	if !a.LockUntil.Equal(time.Date(2031, 6, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("lock until = %v", a.LockUntil)
	}
}

func TestApplyRangeChecksTheDayAndScoreFields(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeCreditCard)
	f.DueDay = "29"
	f.StatementDay = "40"
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues,
		Issue{Field: FieldDueDay, Reason: Range},
		Issue{Field: FieldStatementDay, Reason: Range})

	g := valid()
	g.Liquidity = "101"
	_, issues = Apply(domain.Account{}, g)
	issuesOf(t, issues, Issue{Field: FieldLiquidity, Reason: Range})
}

func TestApplyFallsBackToTheLenderForTheInstitution(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeCreditCard)
	f.Lender = "first bank"
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "First Bank" {
		t.Fatalf("institution = %q, want the title-cased lender", a.Institution)
	}
	f.Institution = "  credit union "
	a, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "Credit Union" {
		t.Fatalf("institution = %q, want the typed name title-cased", a.Institution)
	}
}

func TestApplyChecksTheOwnershipSharesTotal(t *testing.T) {
	f := valid()
	f.Shares = map[string]int{"m1": 60, "m2": 30}
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldOwnerShares, Reason: Total, Sum: 90})
	if a.OwnershipShares == nil {
		t.Fatal("the shares were lost while the split is on")
	}
	f.Shares = map[string]int{"m1": 60, "m2": 40}
	_, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
}

func TestApplyDoesNotAliasTheFormsShares(t *testing.T) {
	f := valid()
	f.Shares = map[string]int{"m1": 100}
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	f.Shares["m1"] = 0
	if a.OwnershipShares["m1"] != 100 {
		t.Fatal("the account's shares changed when the form's map did")
	}
}

func TestApplyLeavesTheSplitOffWhenNoSharesAreGiven(t *testing.T) {
	base := domain.Account{OwnershipShares: map[string]int{"m1": 100}}
	a, issues := Apply(base, valid())
	issuesOf(t, issues)
	if a.OwnershipShares != nil {
		t.Fatalf("shares = %v, want nil when the split is off", a.OwnershipShares)
	}
}

func TestBaseCurrencyOrDefault(t *testing.T) {
	tests := []struct{ name, base, want string }{
		{"set", "EUR", "EUR"},
		{"empty", "", "USD"},
		{"blank", "  ", "USD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BaseCurrencyOrDefault(tt.base); got != tt.want {
				t.Fatalf("BaseCurrencyOrDefault(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func TestIsLockable(t *testing.T) {
	tests := []struct {
		typ  domain.AccountType
		want bool
	}{
		{domain.TypeSavings, true},
		{domain.TypeInvestment, true},
		{domain.TypeRetirement, true},
		{domain.TypeCrypto, true},
		{domain.TypeProperty, true},
		{domain.TypeVehicle, true},
		{domain.TypeOther, true},
		{domain.TypeChecking, false},
		{domain.TypeCash, false},
		{domain.TypeCreditCard, false},
		{domain.TypeLoan, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := IsLockable(tt.typ); got != tt.want {
				t.Fatalf("IsLockable(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestParseShare(t *testing.T) {
	tests := []struct {
		raw    string
		want   int
		wantOK bool
	}{
		{"", 0, true},
		{" 40 ", 40, true},
		{"100", 100, true},
		{"0", 0, true},
		{"abc", 0, false},
		{"12.5", 0, false},
		{"-5", 0, false},
		{"101", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := ParseShare(tt.raw)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("ParseShare(%q) = %d, %v, want %d, %v", tt.raw, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestApplyRejectsAVariableNameThatSlugsToNothing(t *testing.T) {
	for _, name := range []string{"!!!", "é", "  --  "} {
		f := valid()
		f.VarName = name
		_, issues := Apply(domain.Account{}, f)
		issuesOf(t, issues, Issue{Field: FieldVarName, Reason: Invalid})
	}
}

func TestApplyTreatsABlankVariableNameAsNone(t *testing.T) {
	f := valid()
	f.VarName = "   "
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.VarName != "" {
		t.Fatalf("var name = %q, want none", a.VarName)
	}
	f.VarName = " Main_Checking "
	a, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.VarName != "Main_Checking" {
		t.Fatalf("var name = %q, want it trimmed", a.VarName)
	}
}

func TestSplitAvailableNeedsTwoMembers(t *testing.T) {
	for _, tc := range []struct {
		members int
		want    bool
	}{{0, false}, {1, false}, {2, true}, {5, true}} {
		if got := SplitAvailable(tc.members); got != tc.want {
			t.Errorf("SplitAvailable(%d) = %v, want %v", tc.members, got, tc.want)
		}
	}
}
