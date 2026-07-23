// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"testing"
	"time"
)

// TestSubscriptionEventIsStale covers the ordering guard itself (TODOS.md C430).
func TestSubscriptionEventIsStale(t *testing.T) {
	base := time.Unix(1000, 0).UTC()
	cases := []struct {
		name        string
		hadPrevious bool
		lastEventAt time.Time
		eventTime   time.Time
		wantStale   bool
	}{
		{"no previous row: never stale", false, time.Time{}, base, false},
		{"previous row with no ordering signal: never stale", true, time.Time{}, base, false},
		{"event has no timestamp: never stale (fallback to apply)", true, base, time.Time{}, false},
		{"newer event: not stale", true, base, base.Add(time.Second), false},
		{"older event: stale", true, base, base.Add(-time.Second), true},
		{"same-instant event: stale (not strictly newer)", true, base, base, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previous := Subscription{LastEventAt: tc.lastEventAt}
			if got := subscriptionEventIsStale(previous, tc.hadPrevious, tc.eventTime); got != tc.wantStale {
				t.Errorf("subscriptionEventIsStale() = %v, want %v", got, tc.wantStale)
			}
		})
	}
}

// TestApplyStripeEventIgnoresOutOfOrderRetry covers TODOS.md C430's
// correctness note directly: a delayed/reordered "past_due" webhook retry
// arriving AFTER a newer "canceled" event must not un-cancel the row.
func TestApplyStripeEventIgnoresOutOfOrderRetry(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: "user-1", Provider: "github", Subject: "user-1", Email: "u@example.com"}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	newerCanceled := stripeEvent{
		ID:      "evt_2",
		Type:    "customer.subscription.deleted",
		Created: 2000,
	}
	newerCanceled.Data.Object = stripeSubscriptionJSON(t, stripeSubscriptionObject{
		ID: "sub_1", Customer: "cus_1", Status: "canceled",
		Metadata: map[string]string{"user_id": "user-1", "plan": "personal_monthly"},
	})
	if err := applyStripeEvent(store, newerCanceled, now, nil); err != nil {
		t.Fatalf("apply newer canceled event: %v", err)
	}

	sub, ok, err := store.GetSubscriptionByProviderID("stripe", "sub_1")
	if err != nil || !ok {
		t.Fatalf("GetSubscriptionByProviderID after canceled: ok=%v err=%v", ok, err)
	}
	if sub.Status != "canceled" {
		t.Fatalf("Status = %q, want canceled", sub.Status)
	}

	// A delayed retry of an OLDER "past_due" event (lower Created timestamp)
	// arrives after the cancellation. It must be dropped, not applied.
	olderPastDue := stripeEvent{
		ID:      "evt_1_retry",
		Type:    "customer.subscription.updated",
		Created: 1000,
	}
	olderPastDue.Data.Object = stripeSubscriptionJSON(t, stripeSubscriptionObject{
		ID: "sub_1", Customer: "cus_1", Status: "past_due",
		Metadata: map[string]string{"user_id": "user-1", "plan": "personal_monthly"},
	})
	if err := applyStripeEvent(store, olderPastDue, now, nil); err != nil {
		t.Fatalf("apply stale past_due retry: %v", err)
	}

	sub, ok, err = store.GetSubscriptionByProviderID("stripe", "sub_1")
	if err != nil || !ok {
		t.Fatalf("GetSubscriptionByProviderID after stale retry: ok=%v err=%v", ok, err)
	}
	if sub.Status != "canceled" {
		t.Fatalf("a stale past_due retry un-canceled the subscription: Status = %q, want canceled", sub.Status)
	}
}

func stripeSubscriptionJSON(t *testing.T, sub stripeSubscriptionObject) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(sub)
	if err != nil {
		t.Fatalf("marshal stripe subscription object: %v", err)
	}
	return data
}
