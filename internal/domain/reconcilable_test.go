// SPDX-License-Identifier: MIT

package domain

import "testing"

// C685: which account types are ever reconciled against an outside statement.
// The answer decides whether a cleared-only balance means anything for them, and
// a wrong answer here puts a figure that reads as debt on an account that owes
// nothing.
func TestIsReconcilable(t *testing.T) {
	reconcilable := []AccountType{
		TypeChecking, TypeDebit, TypeSavings, TypeCash,
		TypeCreditCard, TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage,
		TypeInvestment, TypeRetirement, TypeOther,
	}
	for _, ty := range reconcilable {
		if !ty.IsReconcilable() {
			t.Errorf("%s should be reconcilable — it has a statement", ty)
		}
	}
	// A utility/HOA shell records an obligation, and nobody sends a statement of
	// it to tick rows against. The other three are estimated on a cadence.
	for _, ty := range []AccountType{TypeUtilities, TypeProperty, TypeVehicle, TypeCrypto} {
		if ty.IsReconcilable() {
			t.Errorf("%s should not be reconcilable", ty)
		}
	}
}

// Every type the app offers must have a CONSIDERED answer, so a type added later
// cannot silently inherit one.
//
// The obvious way to write this — a switch mirroring the implementation's — is
// worthless: a new type would fall through both defaults, they would agree, and
// the test would pass while nobody had thought about it. This states the expected
// answers as data instead, so a type absent from the table fails outright.
func TestEveryAccountTypeHasAConsideredAnswer(t *testing.T) {
	want := map[AccountType]bool{
		TypeChecking: true, TypeDebit: true, TypeSavings: true, TypeCash: true,
		TypeCreditCard: true, TypeLineOfCredit: true, TypeLoan: true,
		TypePersonalLoan: true, TypeMortgage: true,
		TypeInvestment: true, TypeRetirement: true, TypeOther: true,
		// No statement to reconcile against: a tracking shell, and three types
		// whose worth is estimated on a revaluation cadence.
		TypeUtilities: false, TypeProperty: false, TypeVehicle: false, TypeCrypto: false,
	}
	for _, ty := range AllAccountTypes {
		expected, considered := want[ty]
		if !considered {
			t.Errorf("%s has no entry here — decide whether it is ever reconciled against a statement, then add it", ty)
			continue
		}
		if got := ty.IsReconcilable(); got != expected {
			t.Errorf("%s: IsReconcilable = %v, want %v", ty, got, expected)
		}
	}
	for ty := range want {
		found := false
		for _, known := range AllAccountTypes {
			if known == ty {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is expected here but is no longer an account type", ty)
		}
	}
}
