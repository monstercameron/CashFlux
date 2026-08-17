// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEnumValidAndString(t *testing.T) {
	check := func(name string, valid bool, s, want string) {
		if !valid {
			t.Errorf("%s: Valid()=false, want true", name)
		}
		if s != want {
			t.Errorf("%s: String()=%q, want %q", name, s, want)
		}
	}
	check("ClassAsset", ClassAsset.Valid(), ClassAsset.String(), "asset")
	check("TypeCreditCard", TypeCreditCard.Valid(), TypeCreditCard.String(), "credit_card")
	check("KindIncome", KindIncome.Valid(), KindIncome.String(), "income")
	check("ScopeShared", ScopeShared.Valid(), ScopeShared.String(), "shared")
	check("PeriodMonthly", PeriodMonthly.Valid(), PeriodMonthly.String(), "monthly")
	check("StatusDone", StatusDone.Valid(), StatusDone.String(), "done")
	check("PriorityHigh", PriorityHigh.Valid(), PriorityHigh.String(), "high")
	check("RelatedGoal", RelatedGoal.Valid(), RelatedGoal.String(), "goal")
	check("SourceNudge", SourceNudge.Valid(), SourceNudge.String(), "nudge")
}

func TestEnumInvalid(t *testing.T) {
	if AccountClass("nope").Valid() {
		t.Error("invalid AccountClass should be invalid")
	}
	if AccountType("nope").Valid() {
		t.Error("invalid AccountType should be invalid")
	}
	if CategoryKind("nope").Valid() {
		t.Error("invalid CategoryKind should be invalid")
	}
	if TaskPriority("urgent").Valid() {
		t.Error("invalid TaskPriority should be invalid")
	}
}

func TestAllSlicesAreValid(t *testing.T) {
	for _, c := range AllAccountClasses {
		if !c.Valid() {
			t.Errorf("AllAccountClasses has invalid %q", c)
		}
	}
	for _, ty := range AllAccountTypes {
		if !ty.Valid() {
			t.Errorf("AllAccountTypes has invalid %q", ty)
		}
	}
	for _, r := range AllRelatedTypes {
		if !r.Valid() {
			t.Errorf("AllRelatedTypes has invalid %q", r)
		}
	}
	if len(AllAccountTypes) != 16 {
		t.Errorf("AllAccountTypes len = %d, want 16", len(AllAccountTypes))
	}
}

func TestAccountTypeClass(t *testing.T) {
	liabilities := []AccountType{TypeCreditCard, TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage, TypeUtilities}
	for _, ty := range liabilities {
		if ty.Class() != ClassLiability || !ty.IsLiability() {
			t.Errorf("%s should be a liability", ty)
		}
	}
	assets := []AccountType{TypeChecking, TypeDebit, TypeSavings, TypeCash, TypeInvestment, TypeRetirement, TypeCrypto, TypeOther}
	for _, ty := range assets {
		if ty.Class() != ClassAsset || ty.IsLiability() {
			t.Errorf("%s should be an asset", ty)
		}
	}
}

func TestTransactionClassification(t *testing.T) {
	income := Transaction{Amount: money.New(100, "USD")}
	expense := Transaction{Amount: money.New(-100, "USD")}
	transfer := Transaction{Amount: money.New(-100, "USD"), TransferAccountID: "acc2"}

	if !income.IsIncome() || income.IsExpense() || income.IsTransfer() {
		t.Error("income misclassified")
	}
	if !expense.IsExpense() || expense.IsIncome() || expense.IsTransfer() {
		t.Error("expense misclassified")
	}
	if !transfer.IsTransfer() || transfer.IsIncome() || transfer.IsExpense() {
		t.Error("transfer should not count as income or expense")
	}
}

func TestRecurringCadenceNext(t *testing.T) {
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	cases := map[RecurringCadence]time.Time{
		CadenceWeekly:      time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC),
		CadenceBiweekly:    time.Date(2026, 1, 29, 0, 0, 0, 0, time.UTC), // +14d
		CadenceMonthly:     time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		CadenceSemimonthly: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), // day 15 → 1st of next month
		CadenceQuarterly:   time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		CadenceYearly:      time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	for cad, want := range cases {
		if got := cad.Next(base); !got.Equal(want) {
			t.Errorf("%s.Next = %s, want %s", cad, got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
	// Unknown cadence falls back to monthly.
	if got := RecurringCadence("nope").Next(base); !got.Equal(cases[CadenceMonthly]) {
		t.Errorf("unknown cadence Next = %s, want monthly", got.Format("2006-01-02"))
	}
	// Semimonthly before the 15th advances to the 15th of the same month.
	early := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	if got := CadenceSemimonthly.Next(early); !got.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("semimonthly before-15 Next = %s, want 2026-03-15", got.Format("2006-01-02"))
	}
}

func TestRecurringAdvance(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cases := map[RecurringCadence]time.Time{
		CadenceWeekly:    time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		CadenceMonthly:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		CadenceQuarterly: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		CadenceYearly:    time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	for cadence, want := range cases {
		r := Recurring{Cadence: cadence, NextDue: base}
		next := r.Advance()
		if !next.NextDue.Equal(want) {
			t.Errorf("%s Advance NextDue = %s, want %s", cadence, next.NextDue.Format("2006-01-02"), want.Format("2006-01-02"))
		}
		if !r.NextDue.Equal(base) {
			t.Errorf("%s Advance mutated the original to %s", cadence, r.NextDue.Format("2006-01-02"))
		}
	}
}

func TestRecurringMonthlyEquivalent(t *testing.T) {
	mk := func(amount int64, c RecurringCadence) Recurring {
		return Recurring{Amount: money.New(amount, "USD"), Cadence: c}
	}
	cases := []struct {
		r    Recurring
		want int64
	}{
		{mk(10000, CadenceMonthly), 10000},
		{mk(12000, CadenceQuarterly), 4000},    // /3
		{mk(120000, CadenceYearly), 10000},     // /12
		{mk(12000, CadenceWeekly), 52000},      // *52/12 = 4.333× → 52000
		{mk(12000, CadenceBiweekly), 26000},    // *26/12
		{mk(12000, CadenceSemimonthly), 24000}, // *2
		{mk(-150000, CadenceMonthly), -150000},
	}
	for _, tc := range cases {
		if got := tc.r.MonthlyEquivalent(); got != tc.want {
			t.Errorf("%s %d → MonthlyEquivalent %d, want %d", tc.r.Cadence, tc.r.Amount.Amount, got, tc.want)
		}
	}
}

func TestEntitiesCarryCustomFields(t *testing.T) {
	// Smoke check that entities compile with the shared shapes we rely on.
	a := Account{ID: "a1", Type: TypeSavings, Class: ClassAsset, Currency: "USD", BalanceAsOf: time.Now(), Custom: map[string]any{"nickname": "rainy day"}}
	if a.Custom["nickname"] != "rainy day" {
		t.Error("custom field not stored")
	}
}

// TestRecurringCadencePrev locks Prev as the exact inverse of Next. The calendar
// view renders months that have already happened, and a schedule stored as its
// NEXT due date can only answer "what was due last month" by being wound back —
// so any drift between the two directions shows up as a bill on the wrong day.
func TestRecurringCadencePrev(t *testing.T) {
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	cases := map[RecurringCadence]time.Time{
		CadenceDaily:       time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC),
		CadenceWeekly:      time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
		CadenceBiweekly:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CadenceMonthly:     time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
		CadenceSemimonthly: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), // day 15 → 1st of the same month
		CadenceQuarterly:   time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC),
		CadenceYearly:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	for cad, want := range cases {
		if got := cad.Prev(base); !got.Equal(want) {
			t.Errorf("%s.Prev = %s, want %s", cad, got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
	if got := RecurringCadence("nope").Prev(base); !got.Equal(cases[CadenceMonthly]) {
		t.Errorf("unknown cadence Prev = %s, want monthly", got.Format("2006-01-02"))
	}
	// Semimonthly before the 15th steps back to the 15th of the previous month.
	early := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	if got := CadenceSemimonthly.Prev(early); !got.Equal(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("semimonthly before-15 Prev = %s, want 2026-02-15", got.Format("2006-01-02"))
	}
	// Round-trip: stepping back then forward returns the same date, across a year
	// boundary and a short month. Semimonthly is exercised only on the days its
	// schedule can actually land on (the 1st and the 15th) - it is the one cadence
	// with a fixed anchor rather than a free-running step, so an arbitrary date is
	// not a point on its schedule at all.
	for _, cad := range []RecurringCadence{CadenceDaily, CadenceWeekly, CadenceBiweekly,
		CadenceSemimonthly, CadenceMonthly, CadenceQuarterly, CadenceYearly} {
		dates := []time.Time{
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		}
		if cad != CadenceSemimonthly {
			dates = append(dates, time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC))
		}
		for _, d := range dates {
			if got := cad.Next(cad.Prev(d)); !got.Equal(d) {
				t.Errorf("%s: Next(Prev(%s)) = %s, want the original date",
					cad, d.Format("2006-01-02"), got.Format("2006-01-02"))
			}
		}
	}
}

// ─── C204: installment loans vs revolving credit ─────────────────────────────

// Only an installment loan HAS an amortization schedule. A credit card's balance
// is paid down at whatever rate the holder chooses, so offering it a table of
// payments nobody agreed to is the failure this gate prevents.
func TestIsInstallmentSeparatesLoansFromRevolvingCredit(t *testing.T) {
	installment := []AccountType{TypeLoan, TypePersonalLoan, TypeMortgage}
	revolving := []AccountType{TypeCreditCard, TypeLineOfCredit}

	for _, ty := range installment {
		if !ty.IsInstallment() {
			t.Errorf("%s is not installment", ty)
		}
		if ty.IsRevolving() {
			t.Errorf("%s is both installment and revolving", ty)
		}
	}
	for _, ty := range revolving {
		if ty.IsInstallment() {
			t.Errorf("%s is installment", ty)
		}
		if !ty.IsRevolving() {
			t.Errorf("%s is not revolving", ty)
		}
	}
	// Utilities are a liability but neither: a recurring obligation, not
	// borrowed principal.
	if TypeUtilities.IsInstallment() || TypeUtilities.IsRevolving() {
		t.Error("utilities classed as installment or revolving")
	}
	// An asset is neither, and must not be swept in by a negation somewhere.
	if TypeChecking.IsInstallment() || TypeChecking.IsRevolving() {
		t.Error("an asset type classed as a liability shape")
	}
}

// "This is a mortgage" and "we know its term" are different facts. A surface
// that conflates them shows an empty schedule instead of asking for the number.
func TestHasLoanTermsIsSeparateFromIsInstallment(t *testing.T) {
	loan := Account{Class: ClassLiability, Type: TypeMortgage}
	if !loan.IsInstallment() {
		t.Fatal("a mortgage is not installment")
	}
	if loan.HasLoanTerms() {
		t.Error("a mortgage with no term claimed to have loan terms")
	}
	loan.TermMonths = 360
	if !loan.HasLoanTerms() {
		t.Error("a mortgage with a term does not have loan terms")
	}
	// A credit card with a term set by mistake is still not amortizable.
	card := Account{Class: ClassLiability, Type: TypeCreditCard, TermMonths: 24}
	if card.IsInstallment() || card.HasLoanTerms() {
		t.Error("a credit card with a stray term was treated as amortizable")
	}
	// And an ASSET with a term is not a loan.
	asset := Account{Class: ClassAsset, Type: TypeSavings, TermMonths: 12}
	if asset.IsInstallment() || asset.HasLoanTerms() {
		t.Error("an asset was treated as an installment loan")
	}
}

// Additive fields must round-trip so existing accounts load unchanged and new
// ones persist — the store keeps accounts as JSON.
func TestLoanTermFieldsRoundTripThroughJSON(t *testing.T) {
	orig := time.Date(2021, time.July, 15, 0, 0, 0, 0, time.UTC)
	in := Account{ID: "a", Name: "Mortgage", Class: ClassLiability, Type: TypeMortgage,
		TermMonths: 360, OriginationDate: orig}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Account
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.TermMonths != 360 {
		t.Errorf("TermMonths = %d", out.TermMonths)
	}
	if !out.OriginationDate.Equal(orig) {
		t.Errorf("OriginationDate = %v", out.OriginationDate)
	}
	// An account without a term must not gain the field.
	plain, _ := json.Marshal(Account{ID: "b", Name: "Checking"})
	if strings.Contains(string(plain), "termMonths") {
		t.Errorf("an account with no term serialized one: %s", plain)
	}
	// OriginationDate DOES serialize as a zero time even with omitempty, because
	// encoding/json only omits zero values of basic kinds — a struct is never
	// "empty" to it. That is not a defect introduced here: BalanceAsOf, LockUntil
	// and FreshnessSnoozeUntil have always behaved this way, and matching the four
	// siblings is worth more than a pointer or a custom marshaller that would make
	// this one field's absence mean something different from the others'.
	// Asserted so the shared behaviour is stated rather than rediscovered.
	if !strings.Contains(string(plain), "originationDate") {
		t.Error("originationDate stopped serializing — if that was deliberate, the four " +
			"sibling time fields should change with it, not just this one")
	}
}
