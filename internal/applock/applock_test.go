package applock

import "testing"

func TestHashPasscodeDeterministicAndSalted(t *testing.T) {
	h1 := HashPasscode("1234", "saltA")
	if h1 != HashPasscode("1234", "saltA") {
		t.Error("hash should be deterministic for the same passcode+salt")
	}
	if h1 == HashPasscode("1234", "saltB") {
		t.Error("different salts must produce different hashes")
	}
	if h1 == HashPasscode("9999", "saltA") {
		t.Error("different passcodes must produce different hashes")
	}
	if len(h1) != 64 { // hex SHA-256
		t.Errorf("want 64 hex chars, got %d", len(h1))
	}
}

func TestWithPasscodeAndVerify(t *testing.T) {
	c := Config{}.WithPasscode("2468", "s1", 5, "")
	if !c.Enabled || c.Hash == "" || c.Salt != "s1" || c.AutoLockMinutes != 5 {
		t.Fatalf("WithPasscode did not configure the lock: %+v", c)
	}
	if !c.Verify("2468") {
		t.Error("correct passcode should verify")
	}
	if c.Verify("0000") {
		t.Error("wrong passcode must not verify")
	}
	if c.Verify("") {
		t.Error("empty passcode must not verify")
	}

	// Empty passcode or salt is rejected (lock stays as-is / disabled).
	if got := (Config{}).WithPasscode("", "s1", 5, ""); got.Enabled {
		t.Error("empty passcode should not enable the lock")
	}
	if got := (Config{}).WithPasscode("1234", "", 5, ""); got.Enabled {
		t.Error("empty salt should not enable the lock")
	}
	// Negative window clamps to 0.
	if got := (Config{}).WithPasscode("1234", "s", -3, ""); got.AutoLockMinutes != 0 {
		t.Errorf("negative auto-lock should clamp to 0, got %d", got.AutoLockMinutes)
	}
}

func TestValidHintAndStorage(t *testing.T) {
	// Empty hint is always fine; a hint containing the passcode is rejected.
	if !ValidHint("", "1234") {
		t.Error("empty hint should be valid")
	}
	if !ValidHint("my birth year", "1234") {
		t.Error("an unrelated hint should be valid")
	}
	if ValidHint("it's 1234", "1234") {
		t.Error("a hint containing the passcode must be rejected")
	}
	if ValidHint("PIN is 1234!", "1234") {
		t.Error("substring match should reject")
	}
	if ValidHint("contains ABC", "abc") { // case-insensitive
		t.Error("case-insensitive containment must be rejected")
	}

	// A safe hint is stored; a leaky one is dropped (lock still enables).
	good := Config{}.WithPasscode("2468", "s", 0, "year we met")
	if good.Hint != "year we met" {
		t.Errorf("safe hint should be stored, got %q", good.Hint)
	}
	leaky := Config{}.WithPasscode("2468", "s", 0, "the code is 2468")
	if !leaky.Enabled || leaky.Hint != "" {
		t.Errorf("leaky hint should be dropped (lock still enabled), got enabled=%v hint=%q", leaky.Enabled, leaky.Hint)
	}
}

func TestVerifyDisabled(t *testing.T) {
	if (Config{}).Verify("anything") {
		t.Error("a disabled/unconfigured lock must never verify")
	}
	// Enabled flag but no hash/salt: still can't verify.
	if (Config{Enabled: true}).Verify("") {
		t.Error("enabled but unconfigured lock must not verify")
	}
}

func TestCleared(t *testing.T) {
	c := Config{}.WithPasscode("1234", "s", 10, "hi").Cleared()
	if c.Enabled || c.Hash != "" || c.Salt != "" || c.AutoLockMinutes != 0 || c.Hint != "" {
		t.Errorf("Cleared should fully disable the lock, got %+v", c)
	}
	if c.Verify("1234") {
		t.Error("a cleared lock must not verify the old passcode")
	}
}

func TestShouldAutoLock(t *testing.T) {
	c := Config{}.WithPasscode("1234", "s", 5, "")
	cases := []struct {
		idle int
		want bool
	}{
		{0, false}, {4, false}, {5, true}, {10, true},
	}
	for _, tc := range cases {
		if got := c.ShouldAutoLock(tc.idle); got != tc.want {
			t.Errorf("ShouldAutoLock(%d) = %v, want %v", tc.idle, got, tc.want)
		}
	}
	// Window of 0 (manual/reload only) never auto-locks.
	if (Config{}).WithPasscode("1234", "s", 0, "").ShouldAutoLock(9999) {
		t.Error("auto-lock window 0 should never fire")
	}
	// Disabled lock never auto-locks.
	if (Config{}).ShouldAutoLock(9999) {
		t.Error("disabled lock should never auto-lock")
	}
}
