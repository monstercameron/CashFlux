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

// Every type the app offers must have a considered answer, so a type added later
// cannot silently inherit one.
func TestEveryAccountTypeHasAReconcilableAnswer(t *testing.T) {
	for _, ty := range AllAccountTypes {
		got := ty.IsReconcilable()
		switch ty {
		case TypeUtilities, TypeProperty, TypeVehicle, TypeCrypto:
			if got {
				t.Errorf("%s: IsReconcilable = true, want false", ty)
			}
		default:
			if !got {
				t.Errorf("%s: IsReconcilable = false, want true", ty)
			}
		}
	}
}
