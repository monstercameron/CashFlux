package validate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestValidateAccountMoreProblems covers the account checks the existing
// problems test doesn't: an invalid type, a 3-letter but non-uppercase currency
// code (validCode's character-range branch), a negative stability score, and a
// negative APR.
func TestValidateAccountMoreProblems(t *testing.T) {
	a := domain.Account{
		Name: "Acct", OwnerID: "m1", Scope: domain.ScopeShared,
		Type: "bogus", Class: domain.ClassAsset, // type is invalid
		Currency:        "usd", // 3 letters but lowercase → invalid code
		StabilityScore:  -5,
		InterestRateAPR: -1,
	}
	is := ValidateAccount(a)
	for _, f := range []string{"type", "currency", "stabilityScore", "interestRateApr"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q, got %v", f, is)
		}
	}
}

// TestValidateTaskInvalidRelatedType covers the non-empty-but-invalid RelatedType
// branch.
func TestValidateTaskInvalidRelatedType(t *testing.T) {
	is := ValidateTask(domain.Task{
		Title: "x", Status: domain.StatusOpen, Priority: domain.PriorityLow,
		RelatedType: "bogus",
	})
	if !hasField(is, "relatedType") {
		t.Errorf("expected a relatedType issue, got %v", is)
	}
}

// TestIssuesErrorEmpty covers the empty-issues branch of Error().
func TestIssuesErrorEmpty(t *testing.T) {
	if (Issues{}).Error() != "" {
		t.Error("empty issues should produce an empty error string")
	}
}
