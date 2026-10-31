package accountedit

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// applyCase is one table row: a change to a valid draft and the issues it should raise.
type applyCase struct {
	name   string
	change func(*Fields)
	want   []Issue
}

func runApplyCases(t *testing.T, tests []applyCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := valid()
			tt.change(&f)
			_, issues := Apply(domain.Account{}, f)
			issuesOf(t, issues, tt.want...)
		})
	}
}

func TestApplyChecksTheAssetFigures(t *testing.T) {
	savings := func(f *Fields) { f.Type = string(domain.TypeSavings) }
	runApplyCases(t, []applyCase{
		{"APY in range", func(f *Fields) { savings(f); f.APY = "4.25" }, nil},
		{"APY with an exponent", func(f *Fields) { savings(f); f.APY = "1e1" }, nil},
		{"APY not a number", func(f *Fields) { savings(f); f.APY = "high" }, []Issue{{Field: FieldAPY, Reason: Invalid}}},
		{"APY NaN", func(f *Fields) { savings(f); f.APY = "NaN" }, []Issue{{Field: FieldAPY, Reason: Invalid}}},
		{"APY infinity", func(f *Fields) { savings(f); f.APY = "Inf" }, []Issue{{Field: FieldAPY, Reason: Invalid}}},
		{"APY hexadecimal", func(f *Fields) { savings(f); f.APY = "0x10" }, []Issue{{Field: FieldAPY, Reason: Invalid}}},
		{"APY too large to hold", func(f *Fields) { savings(f); f.APY = "1e999" }, []Issue{{Field: FieldAPY, Reason: Invalid}}},
		{"APY negative", func(f *Fields) { savings(f); f.APY = "-1" }, []Issue{{Field: FieldAPY, Reason: Range}}},
		{"APY above the ceiling", func(f *Fields) { savings(f); f.APY = "1001" }, []Issue{{Field: FieldAPY, Reason: Range}}},
		{"expected return may be negative", func(f *Fields) { savings(f); f.ExpectedReturn = "-5" }, nil},
		{"expected return below minus 100", func(f *Fields) { savings(f); f.ExpectedReturn = "-101" }, []Issue{{Field: FieldExpectedRet, Reason: Range}}},
		{"expected return not a number", func(f *Fields) { savings(f); f.ExpectedReturn = "lots" }, []Issue{{Field: FieldExpectedRet, Reason: Invalid}}},
		{"expected return infinity", func(f *Fields) { savings(f); f.ExpectedReturn = "+Inf" }, []Issue{{Field: FieldExpectedRet, Reason: Invalid}}},
		{"lock date unreadable", func(f *Fields) { savings(f); f.LockUntil = "2030-13-45" }, []Issue{{Field: FieldLockUntil, Reason: Invalid}}},
		{"lock date in the wrong layout", func(f *Fields) { savings(f); f.LockUntil = "06/30/2031" }, []Issue{{Field: FieldLockUntil, Reason: Invalid}}},
		{"stability below 0", func(f *Fields) { savings(f); f.Stability = "-1" }, []Issue{{Field: FieldStability, Reason: Range}}},
	})
}

func TestApplyChecksTheLiabilityFigures(t *testing.T) {
	card := func(f *Fields) { f.Type = string(domain.TypeCreditCard) }
	runApplyCases(t, []applyCase{
		{"credit limit unreadable", func(f *Fields) { card(f); f.CreditLimit = "lots" }, []Issue{{Field: FieldCreditLimit, Reason: Invalid}}},
		{"credit limit negative", func(f *Fields) { card(f); f.CreditLimit = "-500" }, []Issue{{Field: FieldCreditLimit, Reason: Range}}},
		{"minimum payment unreadable", func(f *Fields) { card(f); f.MinPayment = "some" }, []Issue{{Field: FieldMinPayment, Reason: Invalid}}},
		{"minimum payment negative", func(f *Fields) { card(f); f.MinPayment = "-25" }, []Issue{{Field: FieldMinPayment, Reason: Range}}},
		{"APR negative", func(f *Fields) { card(f); f.APR = "-3" }, []Issue{{Field: FieldAPR, Reason: Range}}},
		{"APR above the ceiling", func(f *Fields) { card(f); f.APR = "1000.5" }, []Issue{{Field: FieldAPR, Reason: Range}}},
		{"APR NaN", func(f *Fields) { card(f); f.APR = "nan" }, []Issue{{Field: FieldAPR, Reason: Invalid}}},
		{"APR hexadecimal", func(f *Fields) { card(f); f.APR = "0x1p-2" }, []Issue{{Field: FieldAPR, Reason: Invalid}}},
		{"due day zero is blank", func(f *Fields) { card(f); f.DueDay = "0" }, nil},
		{"due day not a number", func(f *Fields) { card(f); f.DueDay = "first" }, []Issue{{Field: FieldDueDay, Reason: Range}}},
	})
}

func TestApplyChecksTheSharedFields(t *testing.T) {
	runApplyCases(t, []applyCase{
		{"snooze date unreadable", func(f *Fields) { f.SnoozeUntil = "next week" }, []Issue{{Field: FieldSnoozeUntil, Reason: Invalid}}},
		{"snooze date readable", func(f *Fields) { f.SnoozeUntil = "2031-02-03" }, nil},
		{"revalue days negative", func(f *Fields) { f.RevalueDays = "-1" }, []Issue{{Field: FieldRevalueDays, Reason: Range}}},
		{"revalue days past the ceiling", func(f *Fields) { f.RevalueDays = "3651" }, []Issue{{Field: FieldRevalueDays, Reason: Range}}},
		{"revalue days not a number", func(f *Fields) { f.RevalueDays = "often" }, []Issue{{Field: FieldRevalueDays, Reason: Invalid}}},
		{"revalue days at the ceiling", func(f *Fields) { f.RevalueDays = "3650" }, nil},
		{"unknown type", func(f *Fields) { f.Type = "spaceship" }, []Issue{{Field: FieldType, Reason: Invalid}}},
		{"blank type", func(f *Fields) { f.Type = "" }, []Issue{{Field: FieldType, Reason: Invalid}}},
		{"currency that is not a code", func(f *Fields) { f.Currency = "dollars" }, []Issue{{Field: FieldCurrency, Reason: Invalid}}},
		{"currency with a rate but no registry entry", func(f *Fields) { f.Currency = "brl" }, nil},
		{"blank currency", func(f *Fields) { f.Currency = "" }, []Issue{{Field: FieldCurrency, Reason: Required}}},
		{"whitespace currency", func(f *Fields) { f.Currency = "  " }, []Issue{{Field: FieldCurrency, Reason: Required}}},
		{"every problem is reported", func(f *Fields) {
			f.Name, f.Opening, f.Type = "", "x", string(domain.TypeSavings)
			f.Liquidity = "500"
		}, []Issue{
			{Field: FieldName, Reason: Required},
			{Field: FieldOpening, Reason: Invalid},
			{Field: FieldLiquidity, Reason: Range},
		}},
	})
}

func TestApplyKeepsAValueTheAccountAlreadyHad(t *testing.T) {
	base := domain.Account{Type: "legacy-type", Currency: "XYZ", OwnerID: "gone-member"}
	f := valid()
	f.Type, f.Currency, f.Owner = "legacy-type", "xyz", "gone-member"
	f.MemberIDs = []string{"m1"}
	_, issues := Apply(base, f)
	issuesOf(t, issues)
}

func TestApplyChecksTheOwnerAgainstTheMembers(t *testing.T) {
	f := valid()
	f.MemberIDs = []string{"m1", "m2"}
	f.Owner = "m3"
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldOwner, Reason: Invalid})

	for _, owner := range []string{"m2", domain.GroupOwnerID} {
		f.Owner = owner
		_, issues = Apply(domain.Account{}, f)
		issuesOf(t, issues)
	}
	f.Owner, f.MemberIDs = "anyone", nil
	_, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
}

func TestApplyPassesTheFlagsAndTextThrough(t *testing.T) {
	f := valid()
	f.InstitutionID, f.ExcludeNetWorth, f.FreshnessExempt = "inst-7", true, true
	f.Notes, f.BeneficiaryNote, f.VarName = "  a note ", " to Sam ", " acct_home "
	f.SnoozeUntil, f.RevalueDays = "2031-02-03", "45"
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.InstitutionID != "inst-7" || !a.ExcludeFromNetWorth || !a.FreshnessExempt {
		t.Fatalf("flags = %q %v %v", a.InstitutionID, a.ExcludeFromNetWorth, a.FreshnessExempt)
	}
	if a.Notes != "a note" || a.BeneficiaryNote != "to Sam" || a.VarName != "acct_home" {
		t.Fatalf("text = %q %q %q", a.Notes, a.BeneficiaryNote, a.VarName)
	}
	if !a.FreshnessSnoozeUntil.Equal(time.Date(2031, 2, 3, 0, 0, 0, 0, time.UTC)) || a.RevalueDays != 45 {
		t.Fatalf("snooze/revalue = %v %d", a.FreshnessSnoozeUntil, a.RevalueDays)
	}
}

func TestApplyKeepsWhatTheFormDoesNotOwn(t *testing.T) {
	asOf := time.Date(2030, 5, 6, 0, 0, 0, 0, time.UTC)
	base := domain.Account{
		ID: "acc-9", BalanceAsOf: asOf, Custom: map[string]any{"colour": "teal"},
		DocRefs:         []domain.AccountDocRef{{ArtifactID: "art-1"}},
		Reconciliations: []domain.Reconciliation{{DifferenceMinor: 3}},
	}
	a, issues := Apply(base, valid())
	issuesOf(t, issues)
	if a.ID != "acc-9" || !a.BalanceAsOf.Equal(asOf) {
		t.Fatalf("id/as-of = %q %v", a.ID, a.BalanceAsOf)
	}
	if a.Custom["colour"] != "teal" || len(a.DocRefs) != 1 || len(a.Reconciliations) != 1 {
		t.Fatalf("custom/docs/reconciliations were lost: %+v", a)
	}
}

func TestApplyStoresADirectoryInstitutionVerbatim(t *testing.T) {
	f := valid()
	f.Institution, f.InstitutionID = "eToro", "inst-1"
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "eToro" {
		t.Fatalf("institution = %q, want the directory name as written", a.Institution)
	}
	f.Institution, f.InstitutionID = "bunq", "inst-2"
	a, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "bunq" {
		t.Fatalf("institution = %q, want bunq as written", a.Institution)
	}
}

func TestApplyTitleCasesTypedInstitutionsButNotMixedCaseWords(t *testing.T) {
	tests := []struct{ typed, want string }{
		{"bunq", "Bunq"},
		{"first national bank", "First National Bank"},
		{"eToro", "eToro"},
		{"BNP paribas", "BNP Paribas"},
	}
	for _, tt := range tests {
		f := valid()
		f.Institution = tt.typed
		a, issues := Apply(domain.Account{}, f)
		issuesOf(t, issues)
		if a.Institution != tt.want {
			t.Errorf("institution %q = %q, want %q", tt.typed, a.Institution, tt.want)
		}
	}
}

func TestApplyFallsBackToTheLenderOnlyForADebt(t *testing.T) {
	f := valid()
	f.Type = string(domain.TypeSavings)
	f.Lender = "first bank"
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "" || a.Lender != "" {
		t.Fatalf("a hidden lender leaked: institution %q, lender %q", a.Institution, a.Lender)
	}
	f.Type, f.InstitutionID = string(domain.TypeLoan), "inst-3"
	a, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if a.Institution != "" {
		t.Fatalf("institution = %q, want none while a directory entry is picked", a.Institution)
	}
}

func TestApplyChecksEachShareAndIgnoresMembersWhoAreGone(t *testing.T) {
	f := valid()
	f.Shares = map[string]int{"m1": -10, "m2": 110}
	_, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldOwnerShares, Reason: Range})

	f.Shares = map[string]int{"m1": 60, "m2": 40, "gone": 50}
	f.MemberIDs = []string{"m1", "m2"}
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if _, kept := a.OwnershipShares["gone"]; kept || len(a.OwnershipShares) != 2 {
		t.Fatalf("shares = %v, want only the members who exist", a.OwnershipShares)
	}

	f.Shares = map[string]int{"m1": 60, "gone": 40}
	_, issues = Apply(domain.Account{}, f)
	issuesOf(t, issues, Issue{Field: FieldOwnerShares, Reason: Total, Sum: 60})
}

func TestClassOf(t *testing.T) {
	tests := []struct {
		typ         domain.AccountType
		asLiability bool
		want        domain.AccountClass
	}{
		{domain.TypeChecking, false, domain.ClassAsset},
		{domain.TypeChecking, true, domain.ClassAsset},
		{domain.TypeCreditCard, false, domain.ClassLiability},
		{domain.TypeOther, false, domain.ClassAsset},
		{domain.TypeOther, true, domain.ClassLiability},
	}
	for _, tt := range tests {
		if got := ClassOf(tt.typ, tt.asLiability); got != tt.want {
			t.Errorf("ClassOf(%q, %v) = %q, want %q", tt.typ, tt.asLiability, got, tt.want)
		}
	}
}

func TestCountsAsDebtInvertsClassOf(t *testing.T) {
	for _, typ := range append([]domain.AccountType{""}, domain.AllAccountTypes...) {
		for _, asLiability := range []bool{false, true} {
			class := ClassOf(typ, asLiability)
			want := typ == domain.TypeOther && asLiability
			if got := CountsAsDebt(typ, class); got != want {
				t.Errorf("CountsAsDebt(%q, %q) = %v, want %v", typ, class, got, want)
			}
		}
	}
}

func TestShareTotalAndSharesValid(t *testing.T) {
	tests := []struct {
		name      string
		shares    map[string]int
		members   []string
		wantTotal int
		wantValid bool
	}{
		{"exact split", map[string]int{"a": 60, "b": 40}, nil, 100, true},
		{"short", map[string]int{"a": 60, "b": 30}, nil, 90, false},
		{"empty", map[string]int{}, nil, 0, false},
		{"member gone is ignored", map[string]int{"a": 60, "b": 40, "x": 30}, []string{"a", "b"}, 100, true},
		{"member gone makes it short", map[string]int{"a": 60, "x": 40}, []string{"a", "b"}, 60, false},
		{"negative share", map[string]int{"a": 110, "b": -10}, nil, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShareTotal(tt.shares, tt.members); got != tt.wantTotal {
				t.Errorf("ShareTotal = %d, want %d", got, tt.wantTotal)
			}
			if got := SharesValid(tt.shares, tt.members); got != tt.wantValid {
				t.Errorf("SharesValid = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestWithShareKeepsTheOldFigureForBadInput(t *testing.T) {
	old := map[string]int{"a": 60}
	got := WithShare(old, "a", "40")
	if got["a"] != 40 || old["a"] != 60 {
		t.Fatalf("got %v from %v, want a copy with 40", got, old)
	}
	if got = WithShare(old, "a", "12.5"); got["a"] != 60 {
		t.Fatalf("a bad figure changed the share to %d", got["a"])
	}
	if got = WithShare(old, "b", ""); got["b"] != 0 || len(got) != 2 {
		t.Fatalf("a blank box = %v, want a zero share", got)
	}
}

func TestMoneyStep(t *testing.T) {
	tests := []struct{ cur, want string }{
		{"USD", "0.01"},
		{"JPY", "1"},
		{"ZZZ", "0.01"},
		{"", "0.01"},
	}
	for _, tt := range tests {
		if got := MoneyStep(tt.cur); got != tt.want {
			t.Errorf("MoneyStep(%q) = %q, want %q", tt.cur, got, tt.want)
		}
	}
}

func TestStepOfDecimals(t *testing.T) {
	tests := []struct {
		decimals int
		want     string
	}{{0, "1"}, {1, "0.1"}, {2, "0.01"}, {3, "0.001"}, {-1, "1"}}
	for _, tt := range tests {
		if got := stepOfDecimals(tt.decimals); got != tt.want {
			t.Errorf("stepOfDecimals(%d) = %q, want %q", tt.decimals, got, tt.want)
		}
	}
}

func TestApplyKeepsTheCurrencyAnAccountAlreadyHad(t *testing.T) {
	f := valid()
	f.Currency = "xyz"
	a, issues := Apply(domain.Account{Currency: "XYZ"}, f)
	issuesOf(t, issues)
	if a.Currency != "XYZ" {
		t.Fatalf("currency = %q, want XYZ", a.Currency)
	}
}

func TestApplyChecksTheDayOfTheMonthFields(t *testing.T) {
	card := func(f *Fields) { f.Type = string(domain.TypeCreditCard) }
	runApplyCases(t, []applyCase{
		{"due day 28", func(f *Fields) { card(f); f.DueDay = "28" }, nil},
		{"due day 29", func(f *Fields) { card(f); f.DueDay = "29" }, []Issue{{Field: FieldDueDay, Reason: Range}}},
		{"due day negative", func(f *Fields) { card(f); f.DueDay = "-1" }, []Issue{{Field: FieldDueDay, Reason: Range}}},
		{"statement day 31", func(f *Fields) { card(f); f.StatementDay = "31" }, nil},
		{"statement day 32", func(f *Fields) { card(f); f.StatementDay = "32" }, []Issue{{Field: FieldStatementDay, Reason: Range}}},
		{"statement day negative", func(f *Fields) { card(f); f.StatementDay = "-2" }, []Issue{{Field: FieldStatementDay, Reason: Range}}},
		{"statement day not a number", func(f *Fields) { card(f); f.StatementDay = "last" }, []Issue{{Field: FieldStatementDay, Reason: Range}}},
	})
}

func TestRangeOfNamesTheBoundsApplyEnforces(t *testing.T) {
	tests := []struct {
		field  Field
		lo, hi int
		ok     bool
	}{
		{FieldDueDay, 1, 28, true},
		{FieldStatementDay, 1, 31, true},
		{FieldLiquidity, 0, 100, true},
		{FieldStability, 0, 100, true},
		{FieldOwnerShares, 0, 100, true},
		{FieldRevalueDays, 0, 3650, true},
		{FieldAPR, 0, 1000, true},
		{FieldAPY, 0, 1000, true},
		{FieldExpectedRet, -100, 1000, true},
		{FieldCreditLimit, 0, 0, false},
		{FieldName, 0, 0, false},
	}
	for _, tt := range tests {
		lo, hi, ok := RangeOf(tt.field)
		if lo != tt.lo || hi != tt.hi || ok != tt.ok {
			t.Errorf("RangeOf(%q) = %d, %d, %v; want %d, %d, %v", tt.field, lo, hi, ok, tt.lo, tt.hi, tt.ok)
		}
	}
}

func TestApplyClearsTheFieldsOfTheOtherClass(t *testing.T) {
	asset := valid()
	asset.Type = string(domain.TypeSavings)
	asset.APY, asset.ExpectedReturn, asset.Liquidity, asset.Stability, asset.LockUntil = "4", "6", "70", "80", "2031-06-30"
	a, issues := Apply(domain.Account{}, asset)
	issuesOf(t, issues)
	if a.APY != 4 || a.ExpectedReturnAPR != 6 || a.LiquidityScore != 70 || a.StabilityScore != 80 || a.LockUntil.IsZero() {
		t.Fatalf("asset figures were not stored: %+v", a)
	}

	// The same account turned into a debt drops every asset-only figure.
	debt := asset
	debt.Type = string(domain.TypeCreditCard)
	debt.CreditLimit, debt.APR, debt.DueDay = "5000", "19.5", "15"
	b, issues := Apply(a, debt)
	issuesOf(t, issues)
	if b.APY != 0 || b.ExpectedReturnAPR != 0 || b.LiquidityScore != 0 || b.StabilityScore != 0 || !b.LockUntil.IsZero() {
		t.Fatalf("asset-only figures survived the change to a debt: %+v", b)
	}
	if b.CreditLimit.Amount != 500000 || b.DueDayOfMonth != 15 {
		t.Fatalf("debt figures = limit %d, due day %d", b.CreditLimit.Amount, b.DueDayOfMonth)
	}

	// And back: a debt turned into an asset drops the debt-only figures.
	c, issues := Apply(b, asset)
	issuesOf(t, issues)
	if c.CreditLimit.Amount != 0 || c.DueDayOfMonth != 0 || c.HasRateAPR() {
		t.Fatalf("debt-only figures survived the change to an asset: %+v", c)
	}
}

func TestApplyDropsTheLockDateOfAnOtherAccountCountedAsADebt(t *testing.T) {
	f := valid()
	f.Type, f.AsLiability, f.LockUntil = string(domain.TypeOther), true, "2031-06-30"
	a, issues := Apply(domain.Account{}, f)
	issuesOf(t, issues)
	if !a.LockUntil.IsZero() {
		t.Fatalf("a debt kept the lock date %v", a.LockUntil)
	}
}

func TestParseShareBounds(t *testing.T) {
	tests := []struct {
		raw  string
		want int
		ok   bool
	}{
		{"", 0, true},
		{"0", 0, true},
		{"100", 100, true},
		{" 40 ", 40, true},
		{"-1", 0, false},
		{"101", 0, false},
		{"1e1", 0, false},
		{"12.5", 0, false},
		{"half", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseShare(tt.raw)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseShare(%q) = %d, %v; want %d, %v", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTitleCaseWordsThatDoNotStartWithALetter(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1st national", "1st National"},
		{"(old) bank", "(old) Bank"},
		{"123", "123"},
		{"-bank", "-bank"},
		{"first-ever bank", "First-ever Bank"},
		{"  spaced   out  ", "Spaced Out"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := titleCase(tt.in); got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
