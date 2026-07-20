// SPDX-License-Identifier: MIT

package subscriptions

import "testing"

// TestIsSubscriptionCommitment locks the lens's central claim: Subscriptions is
// NOT the complement of Bills. The commitments that are neither — HOA dues,
// property tax, insurance — must fall out of BOTH, appearing only under All.
func TestIsSubscriptionCommitment(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		category string
		detected map[string]bool
		want     bool
	}{
		{name: "streaming bundle", label: "Streaming & apps", category: "Subscriptions", want: true},
		{name: "business software", label: "Shop software", category: "Business expenses", want: true},
		{name: "gym", label: "Gym membership", category: "Health", want: true},
		{name: "category carries the signal", label: "YouTube", category: "Subscriptions", want: true},
		{name: "detected by the engine", label: "Wobble", detected: map[string]bool{"wobble": true}, want: true},

		{name: "HOA dues are not a subscription", label: "HOA dues", category: "Housing"},
		{name: "property tax is not a subscription", label: "Property tax (fall installment)", category: "Property tax"},
		{name: "home insurance is not a subscription", label: "Home insurance (annual)", category: "Insurance"},
		{name: "car insurance is not a subscription", label: "Car insurance", category: "Insurance"},
		{name: "a loan payment is not a subscription", label: "Student loan payment", category: "Education"},
		{name: "the mortgage is not a subscription", label: "Mortgage payment", category: "Mortgage"},
		{name: "a coffee habit is not a subscription", label: "Coffee club", category: "Dining out"},
		{name: "an empty name is nothing", label: "  "},
		{name: "an unlabelled outflow is not claimed", label: "Wobble", category: "Other"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSubscriptionCommitment(tc.label, tc.category, tc.detected); got != tc.want {
				t.Errorf("IsSubscriptionCommitment(%q, %q) = %v, want %v", tc.label, tc.category, got, tc.want)
			}
		})
	}
}

// TestIsSubscriptionLikeName checks the positive phrase judgment on its own,
// since callers that hold only a payee use it directly.
func TestIsSubscriptionLikeName(t *testing.T) {
	for _, s := range []string{"Netflix Premium", "Adobe license", "Cloud storage", "Fitness club", "VPN"} {
		if !IsSubscriptionLikeName(s) {
			t.Errorf("IsSubscriptionLikeName(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"HOA dues", "Water bill", "Payroll", "Groceries"} {
		if IsSubscriptionLikeName(s) {
			t.Errorf("IsSubscriptionLikeName(%q) = true, want false", s)
		}
	}
}
