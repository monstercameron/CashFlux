// SPDX-License-Identifier: MIT

package app

import "testing"

func TestCloudProgressRequiresIdentityAndEncryption(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts cloudFacts
		want  cloudStep
	}{
		{"off", cloudFacts{Disabled: true}, cloudOff},
		{"choose", cloudFacts{Disabled: true, Setup: "where"}, cloudWhere},
		{"sign in", cloudFacts{Disabled: true, Setup: "signin"}, cloudSignIn},
		{"protect", cloudFacts{Disabled: true, SignedIn: true, Setup: "signin"}, cloudProtect},
		{"ready", cloudFacts{Disabled: true, SignedIn: true, Protected: true, Setup: "protect"}, cloudFirst},
		{"first upload", cloudFacts{SignedIn: true, Protected: true, Setup: "first", State: "syncing"}, cloudFirst},
		{"synced", cloudFacts{SignedIn: true, Protected: true, State: "synced"}, cloudDone},
		{"removed protection", cloudFacts{SignedIn: true, State: "synced"}, cloudProtect},
		{"expired identity", cloudFacts{Protected: true, State: "synced"}, cloudSignIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cloudNextStep(tc.facts); got != tc.want {
				t.Fatalf("step = %q, want %q", got, tc.want)
			}
		})
	}
	for mask := 0; mask < 8; mask++ {
		f := cloudFacts{Disabled: mask&1 != 0, SignedIn: mask&2 != 0, Protected: mask&4 != 0}
		if cloudCanSync(f) != (mask == 6) {
			t.Fatalf("unsafe sync authorization: %#v", f)
		}
	}
}

func TestCloudStatusNeverInventsSuccess(t *testing.T) {
	for _, tc := range []struct {
		state   string
		pending int
		want    string
	}{
		{"synced", 0, "cloud1.synced"}, {"synced", 2, "cloud1.syncing"},
		{"syncing", 0, "cloud1.syncing"}, {"offline", 1, "cloud1.paused"},
		{"error", 0, "cloud1.attention"}, {"conflict", 0, "cloud1.attention"},
		{"rebind", 0, "cloud1.attention"}, {"future-state", 0, "cloud1.attention"},
	} {
		f := cloudFacts{SignedIn: true, Protected: true, State: tc.state, Pending: tc.pending}
		if title, _ := cloudFace(f, cloudDone); title != tc.want {
			t.Errorf("%s: title = %s, want %s", tc.state, title, tc.want)
		}
	}
}

func TestCloudSignInDoesNotEnableUnprotectedSync(t *testing.T) {
	for _, paused := range []bool{false, true} {
		for _, setup := range []bool{false, true} {
			for _, protected := range []bool{false, true} {
				if disabled := cloudAuthDisablesSync(paused, setup, protected); disabled != (paused || setup || !protected) {
					t.Fatalf("authentication bypassed protection, pause or unfinished setup: paused=%t, setup=%t, protected=%t", paused, setup, protected)
				}
			}
		}
	}
}

func TestCloudSetupFinishesOnlyAfterCommittedSuccessfulFirstSync(t *testing.T) {
	for _, tc := range []struct {
		saved, state   string
		disabled, busy bool
		pending        int
		done           bool
	}{
		{saved: "first", state: "synced", done: true},
		{saved: "first", state: "synced", busy: true},
		{saved: "first", state: "synced", disabled: true},
		{saved: "first", state: "synced", pending: 1},
		{saved: "first", state: "syncing"},
		{saved: "first", state: "error"},
		{saved: "signin", state: "synced"},
	} {
		if got := cloudSetupFinished(tc.saved, tc.disabled, tc.busy, tc.state, tc.pending); got != tc.done {
			t.Fatalf("premature setup completion: %#v, got %t", tc, got)
		}
	}
}
