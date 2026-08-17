// SPDX-License-Identifier: MIT

package applock

import (
	"encoding/hex"
	"strconv"

	"golang.org/x/crypto/argon2"
	"strings"
	"testing"
)

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

func TestWithPasscodePreservesDisplayPrefs(t *testing.T) {
	// A user who hid the lock-screen quotes/meta and then changes their passcode
	// must keep those display choices — they're unrelated to the credential.
	c := Config{HideQuotes: true, HideMeta: true}.WithPasscode("1234", "s", 5, "")
	if !c.HideQuotes || !c.HideMeta {
		t.Errorf("changing the passcode dropped display prefs: %+v", c)
	}
	// Defaults (shown) are preserved too.
	d := Config{}.WithPasscode("1234", "s", 5, "")
	if d.HideQuotes || d.HideMeta {
		t.Errorf("display prefs should default to shown, got %+v", d)
	}
	// Re-setting a passcode re-activates the gate (clears any prior suspension).
	susp := Config{Suspended: true}.WithPasscode("1234", "s", 5, "")
	if susp.Suspended {
		t.Error("setting a passcode should leave the lock active, not suspended")
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

func TestActiveAndSuspend(t *testing.T) {
	c := Config{}.WithPasscode("1234", "s", 5, "")
	if !c.Active() {
		t.Error("a freshly set lock should be active")
	}
	// Suspending keeps the credentials but deactivates the gate + auto-lock.
	c.Suspended = true
	if c.Active() {
		t.Error("a suspended lock must not be active")
	}
	if c.ShouldAutoLock(9999) {
		t.Error("a suspended lock must not auto-lock")
	}
	if !c.Verify("1234") {
		t.Error("a suspended lock still knows its passcode (resume needs no re-entry)")
	}
	// A disabled (no-passcode) lock is never active.
	if (Config{}).Active() {
		t.Error("an unconfigured lock is not active")
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

func TestPasscodeStrength(t *testing.T) {
	cases := []struct {
		pass string
		want Strength
	}{
		{"", StrengthTooShort},
		{"12", StrengthTooShort},
		{"abc", StrengthTooShort},        // 3 < min 4
		{"1111", StrengthWeak},           // all-same
		{"1234", StrengthWeak},           // ascending run
		{"4321", StrengthWeak},           // descending run
		{"abcd", StrengthWeak},           // ascending letters
		{"7392", StrengthWeak},           // 4 digits, one class, not trivial → weak
		{"a1b2c3", StrengthFair},         // 6 chars, 2 classes
		{"hunter2x", StrengthStrong},     // 8 chars, 2 classes (letters+digits)
		{"correcthorse", StrengthFair},   // 12 chars but 1 class → fair (len>=8)
		{"Tr0ub4dour&3", StrengthStrong}, // 12 chars, 4 classes
		{"Aa1!Bb2@", StrengthStrong},     // 8 chars, 4 classes
	}
	for _, c := range cases {
		if got := PasscodeStrength(c.pass); got != c.want {
			t.Errorf("PasscodeStrength(%q) = %v, want %v", c.pass, got, c.want)
		}
	}
}

func TestStrengthString(t *testing.T) {
	for s, want := range map[Strength]string{
		StrengthTooShort: "too-short", StrengthWeak: "weak", StrengthFair: "fair", StrengthStrong: "strong",
	} {
		if got := s.String(); got != want {
			t.Errorf("Strength(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// R30-gatekdf: PBKDF2 gate hash + VerifyPasscode migration path
// ---------------------------------------------------------------------------

func TestHashPasscodePBKDF2Format(t *testing.T) {
	h := HashPasscodePBKDF2("hunter2", "saltsaltsalt")
	// Must start with "pbkdf2$<iters>$" and contain only hex after the last $.
	if !strings.HasPrefix(h, "pbkdf2$") {
		t.Fatalf("want pbkdf2$ prefix, got %q", h)
	}
	parts := strings.SplitN(h, "$", 3)
	if len(parts) != 3 {
		t.Fatalf("want 3 parts, got %d in %q", len(parts), h)
	}
	if parts[1] != "210000" {
		t.Errorf("want iteration count 210000, got %q", parts[1])
	}
	// hex portion: 32 bytes = 64 hex chars
	if len(parts[2]) != 64 {
		t.Errorf("want 64 hex chars for DK, got %d in %q", len(parts[2]), parts[2])
	}
}

func TestHashPasscodePBKDF2Deterministic(t *testing.T) {
	h1 := HashPasscodePBKDF2("password", "mysalt")
	h2 := HashPasscodePBKDF2("password", "mysalt")
	if h1 != h2 {
		t.Error("PBKDF2 hash must be deterministic for the same inputs")
	}
	h3 := HashPasscodePBKDF2("password", "othersalt")
	if h1 == h3 {
		t.Error("different salts must produce different hashes")
	}
	h4 := HashPasscodePBKDF2("other", "mysalt")
	if h1 == h4 {
		t.Error("different passcodes must produce different hashes")
	}
}

func TestVerifyPasscodeNewFormat(t *testing.T) {
	salt := "testsalt"
	cases := []struct {
		name     string
		passcode string
		stored   string
		wantOk   bool
		wantMigr bool
		wantErr  bool
	}{
		{
			name:     "correct passcode — legacy PBKDF2 format (migrates to argon2id)",
			passcode: "correct",
			stored:   HashPasscodePBKDF2("correct", salt),
			wantOk:   true, wantMigr: true, wantErr: false,
		},
		{
			name:     "correct passcode — current argon2id format",
			passcode: "correct",
			stored:   HashPasscodeArgon2id("correct", salt),
			wantOk:   true, wantMigr: false, wantErr: false,
		},
		{
			name:     "wrong passcode — argon2id format",
			passcode: "wrong",
			stored:   HashPasscodeArgon2id("correct", salt),
			wantOk:   false, wantMigr: false, wantErr: false,
		},
		{
			name:     "wrong passcode — new PBKDF2 format",
			passcode: "wrong",
			stored:   HashPasscodePBKDF2("correct", salt),
			wantOk:   false, wantMigr: false, wantErr: false,
		},
		{
			name:     "correct passcode — legacy SHA-256 returns needsMigration",
			passcode: "correct",
			stored:   HashPasscode("correct", salt),
			wantOk:   true, wantMigr: true, wantErr: false,
		},
		{
			name:     "wrong passcode — legacy SHA-256",
			passcode: "wrong",
			stored:   HashPasscode("correct", salt),
			wantOk:   false, wantMigr: false, wantErr: false,
		},
		{
			name:     "garbage storedHash — unrecognised format",
			passcode: "anything",
			stored:   "not-a-hash",
			wantOk:   false, wantMigr: false, wantErr: true,
		},
		{
			name:     "tampered PBKDF2 header — malformed parts",
			passcode: "anything",
			stored:   "pbkdf2$notanumber$deadbeef",
			wantOk:   false, wantMigr: false, wantErr: true,
		},
		{
			name:     "tampered PBKDF2 hex — non-hex chars",
			passcode: "anything",
			stored:   "pbkdf2$210000$ZZZZ",
			wantOk:   false, wantMigr: false, wantErr: true,
		},
		{
			name:     "empty storedHash — unrecognised format",
			passcode: "anything",
			stored:   "",
			wantOk:   false, wantMigr: false, wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, migr, err := VerifyPasscode(tc.passcode, salt, tc.stored)
			if (err != nil) != tc.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if ok != tc.wantOk {
				t.Errorf("wantOk=%v, got ok=%v", tc.wantOk, ok)
			}
			if migr != tc.wantMigr {
				t.Errorf("wantMigr=%v, got needsMigration=%v", tc.wantMigr, migr)
			}
		})
	}
}

// TestVerifyPasscodeConstantTime ensures the constant-time path is exercised
// for both matching and non-matching hashes with no detectable early exit (this
// is a structural test — it just proves ConstantTimeCompare is called on
// equal-length inputs, not a timing oracle).
func TestVerifyPasscodeConstantTime(t *testing.T) {
	salt := "cttest"
	stored := HashPasscodePBKDF2("secret", salt)
	// Correct — must match. It also reports needing migration now that argon2id
	// is the current scheme (C284).
	ok, migr, err := VerifyPasscode("secret", salt, stored)
	if !ok || !migr || err != nil {
		t.Errorf("constant-time path: correct passcode should match and migrate; ok=%v migr=%v err=%v", ok, migr, err)
	}
	// Wrong — must not match.
	ok, migr, err = VerifyPasscode("wrong!", salt, stored)
	if ok || migr || err != nil {
		t.Errorf("constant-time path: wrong passcode should not match; ok=%v migr=%v err=%v", ok, migr, err)
	}
}

// TestWithPasscodeUsesArgon2id confirms Config.WithPasscode stores the current
// format and that Config.Verify (which delegates to VerifyPasscode) still works
// end-to-end.
func TestWithPasscodeUsesArgon2id(t *testing.T) {
	c := Config{}.WithPasscode("mypass", "mysalt", 10, "")
	if !strings.HasPrefix(c.Hash, "argon2id$") {
		t.Errorf("WithPasscode should store an argon2id hash, got %q", c.Hash)
	}
	if !c.Verify("mypass") {
		t.Error("Verify must succeed for the correct passcode after WithPasscode")
	}
	if c.Verify("wrong") {
		t.Error("Verify must fail for the wrong passcode")
	}
}

// ─── C284: argon2id ──────────────────────────────────────────────────────────

// The parameters are stored WITH the hash, not read from today's constants at
// verify time. Without that, raising the cost later would silently break every
// existing passcode — the derived key depends on them.
func TestArgon2idHashCarriesItsOwnParameters(t *testing.T) {
	h := HashPasscodeArgon2id("secret", "salty")
	parts := strings.Split(h, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		t.Fatalf("hash shape = %q", h)
	}
	if parts[1] != strconv.Itoa(Argon2Time) ||
		parts[2] != strconv.Itoa(Argon2MemoryKiB) ||
		parts[3] != strconv.Itoa(Argon2Threads) {
		t.Errorf("hash does not record its parameters: %q", h)
	}
	// A hash recorded at DIFFERENT parameters must still verify, which is the
	// whole point of storing them.
	old := "argon2id$1$8192$1$" + hex.EncodeToString(
		argon2.IDKey([]byte("secret"), []byte("salty"), 1, 8192, 1, 32))
	ok, migr, err := VerifyPasscode("secret", "salty", old)
	if err != nil || !ok {
		t.Errorf("a hash at older parameters failed to verify: ok=%v err=%v", ok, err)
	}
	if migr {
		t.Error("an argon2id hash asked to be migrated — the scheme is current regardless of cost")
	}
}

// The salt has to matter, or two households with the same passcode share a hash.
func TestArgon2idIsSaltDependent(t *testing.T) {
	a := HashPasscodeArgon2id("same", "salt-a")
	b := HashPasscodeArgon2id("same", "salt-b")
	if a == b {
		t.Error("the same passcode under two salts produced one hash")
	}
	if ok, _, _ := VerifyPasscode("same", "salt-b", a); ok {
		t.Error("a hash verified under the wrong salt")
	}
}

// Both older schemes must keep verifying AND report that they should be
// re-hashed — a successful verify is the only moment the plaintext is in hand,
// so it is the only moment a migration is possible.
func TestBothLegacySchemesVerifyAndRequestMigration(t *testing.T) {
	salt := "migr"
	legacy := map[string]string{
		"sha256": HashPasscode("pw", salt),
		"pbkdf2": HashPasscodePBKDF2("pw", salt),
	}
	for name, stored := range legacy {
		ok, migr, err := VerifyPasscode("pw", salt, stored)
		if err != nil || !ok {
			t.Errorf("%s: ok=%v err=%v", name, ok, err)
		}
		if !migr {
			t.Errorf("%s: did not request migration", name)
		}
		// A WRONG passcode must never request migration — that would re-hash on a
		// failed unlock and hand an attacker a way to overwrite the credential.
		if _, wrongMigr, _ := VerifyPasscode("nope", salt, stored); wrongMigr {
			t.Errorf("%s: a failed verify requested migration", name)
		}
	}
}

// A malformed argon2id hash must error rather than silently returning false,
// which would look identical to a wrong passcode and hide a corrupt credential.
func TestMalformedArgon2idHashErrors(t *testing.T) {
	for _, bad := range []string{
		"argon2id$2$19456$1",             // too few parts
		"argon2id$x$19456$1$deadbeef",    // non-numeric time
		"argon2id$2$0$1$deadbeef",        // zero memory
		"argon2id$2$19456$1$nothexatall", // bad hex
	} {
		if _, _, err := VerifyPasscode("pw", "s", bad); err == nil {
			t.Errorf("%q verified without an error", bad)
		}
	}
}

// The lazy migration only works if something drives it. Verify discards the
// flag, so an unlock through it leaves the old hash forever.
func TestVerifyAndUpgradeMovesALegacyHashForward(t *testing.T) {
	salt := "upg"
	legacy := Config{Enabled: true, Salt: salt, Hash: HashPasscode("pw", salt), AutoLockMinutes: 5}

	ok, next, changed := legacy.VerifyAndUpgrade("pw")
	if !ok || !changed {
		t.Fatalf("ok=%v changed=%v", ok, changed)
	}
	if !strings.HasPrefix(next.Hash, "argon2id$") {
		t.Errorf("upgraded hash = %q", next.Hash)
	}
	// Everything else about the config survives — an upgrade is not a reset.
	if next.AutoLockMinutes != 5 || next.Salt != salt || !next.Enabled {
		t.Errorf("the upgrade changed unrelated config: %+v", next)
	}
	// And the new hash verifies without asking to migrate again.
	if ok, _, again := next.VerifyAndUpgrade("pw"); !ok || again {
		t.Errorf("the upgraded config still asks to migrate: ok=%v changed=%v", ok, again)
	}
}

// Re-hashing on a wrong passcode would let anyone who reaches the lock screen
// overwrite the stored credential.
func TestVerifyAndUpgradeNeverUpgradesOnAFailedVerify(t *testing.T) {
	salt := "upg"
	legacy := Config{Enabled: true, Salt: salt, Hash: HashPasscode("pw", salt)}
	ok, next, changed := legacy.VerifyAndUpgrade("wrong")
	if ok || changed {
		t.Errorf("a wrong passcode reported ok=%v changed=%v", ok, changed)
	}
	if next.Hash != legacy.Hash {
		t.Error("a failed verify rewrote the stored hash")
	}
}

func TestVerifyAndUpgradeOnAnUnconfiguredLock(t *testing.T) {
	for _, c := range []Config{
		{},
		{Enabled: true},
		{Enabled: true, Salt: "s"},
	} {
		if ok, _, changed := c.VerifyAndUpgrade("anything"); ok || changed {
			t.Errorf("%+v reported ok=%v changed=%v", c, ok, changed)
		}
	}
}
