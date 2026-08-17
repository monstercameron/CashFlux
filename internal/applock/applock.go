// SPDX-License-Identifier: MIT

// Package applock models CashFlux's optional passcode lock: a soft gate that
// keeps the app's screens behind a passcode and can auto-lock after a period of
// inactivity. It is a deterrent, not encryption — the data still lives in the
// browser's local storage — so the passcode is never stored in the clear; only a
// salted hash is kept.
//
// Since R30-gatekdf the gate hash uses PBKDF2-SHA256 at [PBKDF2Iterations]
// iterations rather than plain SHA-256. Legacy bare-SHA-256 hashes are still
// verified during a transparent migration window: on a successful unlock the
// caller should re-hash and re-store using [HashPasscodePBKDF2] (see
// [VerifyPasscode] needsMigration return value).
//
// Pure Go, no platform dependencies (the random salt and the wall clock are the
// caller's job, so this stays deterministic and unit-testable). The wasm/UI layer
// generates the salt (crypto/rand), measures idle time, and renders the gate.
package applock

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Config is the persisted app-lock configuration. The zero value is a valid,
// disabled lock (no passcode, no auto-lock).
//
// The Hash field stores either a legacy bare hex SHA-256 (pre-R30-gatekdf) or
// the new self-describing "pbkdf2$<iters>$<hex>" format. Use [VerifyPasscode]
// to verify — it handles both formats and flags when migration is due.
type Config struct {
	Enabled         bool   `json:"enabled"`
	Salt            string `json:"salt"`            // random per-install, set with the passcode
	Hash            string `json:"hash"`            // pbkdf2$<iters>$<hex> (or legacy bare SHA-256)
	AutoLockMinutes int    `json:"autoLockMinutes"` // 0 = lock only on reload / manual lock
	Hint            string `json:"hint,omitempty"`  // optional reminder, revealed only after failed tries
	// Lock-screen content toggles. Stored as "hide" flags so the default (zero
	// value / older configs) is "shown" — both default ON per the B17.1 spec.
	HideQuotes bool `json:"hideQuotes,omitempty"`
	HideMeta   bool `json:"hideMeta,omitempty"`
	// Suspended pauses the gate without dropping the passcode: the credentials are
	// kept, but the lock screen doesn't appear. Resuming needs no new passcode.
	Suspended bool `json:"suspended,omitempty"`
}

// Active reports whether the gate should actually guard the app: a passcode is set
// and the lock isn't paused.
func (c Config) Active() bool { return c.Enabled && !c.Suspended }

// ValidHint reports whether hint is safe to store with the given passcode. An
// empty hint (no hint) is always fine; a non-empty hint must not contain the
// passcode (case-insensitive), so it can never leak the secret.
func ValidHint(hint, passcode string) bool {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return true
	}
	if passcode == "" {
		return false
	}
	return !strings.Contains(strings.ToLower(hint), strings.ToLower(passcode))
}

// PBKDF2Iterations is the iteration count used by [HashPasscodePBKDF2].
// 210,000 meets the 2023 OWASP recommendation for PBKDF2-HMAC-SHA256 and
// matches the NIST SP 800-132 guidance tier for password storage.
const PBKDF2Iterations = 210_000

// pbkdf2Prefix is prepended to every hash produced by [HashPasscodePBKDF2] so
// that stored values are self-describing and can survive future algorithm
// changes. Format: "pbkdf2$<iterations>$<hex-derived-key>".
const pbkdf2Prefix = "pbkdf2"

// pbkdf2SHA256 is a stdlib-only PBKDF2 implementation using HMAC-SHA256 as the
// pseudo-random function. It follows RFC 2898 §5.2 with dkLen == 32 (one block).
// No external dependency is required because Go's standard library provides all
// the primitives.
func pbkdf2SHA256(password, salt []byte, iterations int) []byte {
	// PRF = HMAC-SHA256; one block (T_1) is sufficient for a 32-byte key.
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	// Block index 1 encoded as a big-endian uint32, per RFC 2898.
	var block [4]byte
	binary.BigEndian.PutUint32(block[:], 1)
	mac.Write(block[:])
	u := mac.Sum(nil) // U_1

	out := make([]byte, len(u))
	copy(out, u)

	for i := 1; i < iterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}

// ─── argon2id (C284) ─────────────────────────────────────────────────────────
//
// PBKDF2 is a large number of cheap hashes. That is exactly the shape a GPU is
// good at, so its cost to an attacker scales far more slowly than its cost to
// the user. argon2id is MEMORY-hard: cracking it in parallel needs memory per
// guess, which is the resource attackers cannot buy their way out of cheaply.
// For a four-digit passcode gate — a tiny keyspace — that difference is most of
// the security this gate can offer.
//
// The parameters below are the OWASP minimum configuration for argon2id, and
// they are the minimum ON PURPOSE. This runs in WebAssembly, inside a browser
// tab, on whatever device the household owns. A 64 MiB allocation per unlock is
// a real risk of a failed allocation or a visible stall on a phone, and a gate
// that will not open is worse security than a gate that opens with a weaker
// hash — people turn off the lock they cannot get past.
const (
	// Argon2Time is the number of passes.
	Argon2Time = 2
	// Argon2MemoryKiB is the memory cost. 19 MiB is OWASP's floor for t=2.
	Argon2MemoryKiB = 19 * 1024
	// Argon2Threads is the parallelism. One, because wasm is single-threaded
	// here: asking for more would cost memory and buy nothing.
	Argon2Threads = 1
	// maxArgon2KeyLen bounds the derived key read back out of a stored hash. The
	// bound is generous next to Argon2KeyLen (32) because its job is to reject a
	// malformed hash, not to pin the length a future cost bump might choose.
	maxArgon2KeyLen = 1024
	// Argon2KeyLen is the derived-key length in bytes.
	Argon2KeyLen = 32
)

// argon2Prefix tags hashes produced by [HashPasscodeArgon2id] so a stored value
// is self-describing. Format: "argon2id$<time>$<memKiB>$<threads>$<hex>".
//
// The parameters are stored WITH the hash rather than read from the constants
// at verify time. If they were not, raising the cost later would silently break
// every existing passcode — the derived key depends on them, so a hash made at
// 19 MiB cannot be verified at 64 MiB.
const argon2Prefix = "argon2id"

// HashPasscodeArgon2id derives an argon2id hash of passcode using salt and
// returns a self-describing string. It is the current scheme; [VerifyPasscode]
// still accepts the two older ones and reports that they should be migrated.
func HashPasscodeArgon2id(passcode, salt string) string {
	dk := argon2.IDKey([]byte(passcode), []byte(salt),
		Argon2Time, Argon2MemoryKiB, Argon2Threads, Argon2KeyLen)
	return fmt.Sprintf("%s$%d$%d$%d$%s", argon2Prefix,
		Argon2Time, Argon2MemoryKiB, Argon2Threads, hex.EncodeToString(dk))
}

// verifyArgon2id checks passcode against an "argon2id$t$m$p$hex" hash, using the
// parameters recorded in the hash itself rather than today's constants.
func verifyArgon2id(passcode, salt, storedHash string) (bool, error) {
	parts := strings.SplitN(storedHash, "$", 5)
	if len(parts) != 5 {
		return false, errors.New("applock: malformed argon2id hash: expected 5 '$'-separated parts")
	}
	// Parsed at the WIDTH argon2 will use them at, not as ints that are narrowed
	// afterwards. These three come from the stored string, and argon2.IDKey takes
	// uint32/uint32/uint8 — so parsing into int and converting let a parameter
	// larger than its destination type wrap on the way in rather than be rejected:
	// "p=256" became 0 threads and "t=4294967297" became a single pass, so a hash
	// that should have failed as malformed was instead verified against parameters
	// nobody chose (gosec G115). ParseUint with an explicit bit size rejects those
	// at the point of reading, which is also where the error message belongs.
	//
	// Reading the parameters out of the hash at all is deliberate — it is what lets
	// an existing hash keep verifying after the cost constants are raised — and that
	// is exactly why they have to be range-checked rather than trusted.
	t, errT := strconv.ParseUint(parts[1], 10, 32)
	m, errM := strconv.ParseUint(parts[2], 10, 32)
	p, errP := strconv.ParseUint(parts[3], 10, 8)
	if errT != nil || errM != nil || errP != nil || t == 0 || m == 0 || p == 0 {
		return false, fmt.Errorf("applock: invalid argon2id parameters %q/%q/%q", parts[1], parts[2], parts[3])
	}
	storedBytes, decErr := hex.DecodeString(parts[4])
	if decErr != nil {
		return false, fmt.Errorf("applock: malformed argon2id hash hex: %w", decErr)
	}
	// A derived key longer than this is not a key, it is a malformed hash — and the
	// bound is what makes the length's narrowing to uint32 safe.
	if len(storedBytes) == 0 || len(storedBytes) > maxArgon2KeyLen {
		return false, errors.New("applock: argon2id hash has no usable derived key")
	}
	// #nosec G115 -- len(storedBytes) is bounded to [1, maxArgon2KeyLen] by the check
	// immediately above, so this cannot overflow uint32. The narrowings that COULD
	// overflow were the parameters read out of the hash, and those are now parsed at
	// their target width by ParseUint rather than range-checked after the fact.
	keyLen := uint32(len(storedBytes))
	dk := argon2.IDKey([]byte(passcode), []byte(salt), uint32(t), uint32(m), uint8(p), keyLen)
	return subtle.ConstantTimeCompare(dk, storedBytes) == 1, nil
}

// HashPasscodePBKDF2 derives a PBKDF2-SHA256 hash of passcode using salt and
// returns a self-describing string of the form "pbkdf2$<iters>$<hex>". The
// salt must be non-empty and comes from the caller (crypto/rand in the UI
// layer). This is the preferred hash for all new gate credentials.
func HashPasscodePBKDF2(passcode, salt string) string {
	dk := pbkdf2SHA256([]byte(passcode), []byte(salt), PBKDF2Iterations)
	return fmt.Sprintf("%s$%d$%s", pbkdf2Prefix, PBKDF2Iterations, hex.EncodeToString(dk))
}

// VerifyPasscode checks passcode against storedHash using the scheme indicated
// by storedHash's format:
//
//   - New format ("pbkdf2$<iters>$<hex>"): verified with PBKDF2-SHA256.
//     needsMigration is false.
//   - Legacy format (bare 64-char hex, no "$" prefix): verified with the old
//     plain SHA-256 path. On success needsMigration is true — the caller should
//     re-hash with [HashPasscodePBKDF2] and persist the new value so the account
//     graduates to the stronger scheme on next unlock.
//
// Always uses a constant-time comparison to prevent timing side-channels.
// Returns an error if storedHash is not in a recognised format.
func VerifyPasscode(passcode, salt, storedHash string) (ok bool, needsMigration bool, err error) {
	if strings.HasPrefix(storedHash, argon2Prefix+"$") {
		// Current scheme. Nothing to migrate.
		match, vErr := verifyArgon2id(passcode, salt, storedHash)
		return match, false, vErr
	}
	if strings.HasPrefix(storedHash, pbkdf2Prefix+"$") {
		// New PBKDF2 scheme: pbkdf2$<iters>$<hex>
		parts := strings.SplitN(storedHash, "$", 3)
		if len(parts) != 3 {
			return false, false, errors.New("applock: malformed pbkdf2 hash: expected 3 '$'-separated parts")
		}
		iters, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil || iters <= 0 {
			return false, false, fmt.Errorf("applock: invalid pbkdf2 iteration count %q", parts[1])
		}
		storedBytes, decErr := hex.DecodeString(parts[2])
		if decErr != nil {
			return false, false, fmt.Errorf("applock: malformed pbkdf2 hash hex: %w", decErr)
		}
		dk := pbkdf2SHA256([]byte(passcode), []byte(salt), iters)
		match := subtle.ConstantTimeCompare(dk, storedBytes) == 1
		// PBKDF2 is no longer the current scheme (C284). A successful verify is
		// the one moment the plaintext passcode is in hand, so it is the only
		// moment a re-hash is possible — hence migrating on verify rather than
		// in a background pass that has nothing to hash.
		return match, match, nil
	}

	// Legacy bare SHA-256 (64 hex chars, no "$").
	if len(storedHash) == 64 && !strings.Contains(storedHash, "$") {
		got := HashPasscode(passcode, salt)
		match := subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
		if match {
			return true, true, nil // signal caller to re-hash
		}
		return false, false, nil
	}

	return false, false, fmt.Errorf("applock: unrecognised storedHash format (len=%d)", len(storedHash))
}

// HashPasscode returns the hex-encoded SHA-256 of salt+passcode. Deterministic
// given the same inputs, so the salt must come from the caller.
//
// Deprecated: new gate credentials should use [HashPasscodePBKDF2]. This
// function is retained for legacy verification and hash migration via
// [VerifyPasscode].
func HashPasscode(passcode, salt string) string {
	sum := sha256.Sum256([]byte(salt + passcode))
	return hex.EncodeToString(sum[:])
}

// WithPasscode returns a copy of the config with the lock enabled for the given
// passcode (hashed with salt), auto-lock window, and optional hint. An empty
// passcode or salt is rejected (returns the config unchanged) so the lock can't
// be enabled without a real secret. A negative auto-lock window is clamped to 0
// (manual/reload only). A hint that would leak the passcode is dropped. The
// lock-screen display preferences (HideQuotes/HideMeta) are carried over from the
// receiver — they're unrelated to the credential, so changing the passcode must
// not silently reset them. The lock is left active (un-suspended), since setting
// a passcode is an explicit re-enable.
func (c Config) WithPasscode(passcode, salt string, autoLockMinutes int, hint string) Config {
	if passcode == "" || salt == "" {
		return c
	}
	if autoLockMinutes < 0 {
		autoLockMinutes = 0
	}
	if !ValidHint(hint, passcode) {
		hint = ""
	}
	return Config{
		Enabled:         true,
		Salt:            salt,
		Hash:            HashPasscodeArgon2id(passcode, salt),
		AutoLockMinutes: autoLockMinutes,
		Hint:            strings.TrimSpace(hint),
		HideQuotes:      c.HideQuotes,
		HideMeta:        c.HideMeta,
	}
}

// Cleared returns a disabled lock (no passcode), for turning the lock off.
func (c Config) Cleared() Config { return Config{} }

// Verify reports whether passcode matches the configured hash. Always false when
// the lock is disabled or unconfigured. Handles both the legacy bare-SHA-256
// format and the current PBKDF2 format via [VerifyPasscode].
func (c Config) Verify(passcode string) bool {
	if !c.Enabled || c.Hash == "" || c.Salt == "" {
		return false
	}
	ok, _, _ := VerifyPasscode(passcode, c.Salt, c.Hash)
	return ok
}

// ShouldAutoLock reports whether the app should auto-lock given how many whole
// minutes the user has been idle. Only fires when the lock is enabled with a
// positive auto-lock window.
func (c Config) ShouldAutoLock(idleMinutes int) bool {
	return c.Active() && c.AutoLockMinutes > 0 && idleMinutes >= c.AutoLockMinutes
}

// MinPasscodeLength is the shortest passcode the lock should accept on set. Below
// this, PasscodeStrength returns StrengthTooShort so the UI can reject it.
const MinPasscodeLength = 4

// Strength ranks a passcode for the strength meter shown when the user sets one
// (R30). It is a UX guide — the lock is a deterrent, not encryption (see the
// package doc) — so callers use it to label/encourage, and reject only
// StrengthTooShort. Higher is stronger.
type Strength int

const (
	// StrengthTooShort is below MinPasscodeLength — reject it.
	StrengthTooShort Strength = iota
	// StrengthWeak meets the minimum but is trivial or low-variety.
	StrengthWeak
	// StrengthFair is a reasonable everyday passcode.
	StrengthFair
	// StrengthStrong is long and/or varied.
	StrengthStrong
)

// String returns the stable lowercase token for the strength level.
func (s Strength) String() string {
	switch s {
	case StrengthTooShort:
		return "too-short"
	case StrengthWeak:
		return "weak"
	case StrengthFair:
		return "fair"
	case StrengthStrong:
		return "strong"
	default:
		return "weak"
	}
}

// PasscodeStrength scores a passcode by length and character variety, demoting
// trivial patterns (all-same character or a simple ascending/descending run like
// "1234"/"4321"). Pure and deterministic — no randomness, no clock.
func PasscodeStrength(passcode string) Strength {
	r := []rune(passcode)
	n := len(r)
	if n < MinPasscodeLength {
		return StrengthTooShort
	}
	if isTrivialPasscode(r) {
		return StrengthWeak
	}
	classes := charClasses(r)
	switch {
	case n >= 12 && classes >= 3:
		return StrengthStrong
	case n >= 8 && classes >= 2:
		return StrengthStrong
	case n >= 6 && classes >= 2:
		return StrengthFair
	case n >= 8:
		return StrengthFair
	default:
		return StrengthWeak
	}
}

// charClasses counts how many of {lowercase, uppercase, digit, other} appear.
func charClasses(r []rune) int {
	var lower, upper, digit, other bool
	for _, c := range r {
		switch {
		case c >= 'a' && c <= 'z':
			lower = true
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= '0' && c <= '9':
			digit = true
		default:
			other = true
		}
	}
	n := 0
	for _, present := range []bool{lower, upper, digit, other} {
		if present {
			n++
		}
	}
	return n
}

// isTrivialPasscode reports whether the passcode is all the same character or a
// strictly ascending/descending run of consecutive code points (e.g. "1111",
// "1234", "4321", "abcd") — patterns a brute-forcer tries first.
func isTrivialPasscode(r []rune) bool {
	if len(r) < 2 {
		return true
	}
	allSame, asc, desc := true, true, true
	for i := 1; i < len(r); i++ {
		if r[i] != r[0] {
			allSame = false
		}
		if r[i] != r[i-1]+1 {
			asc = false
		}
		if r[i] != r[i-1]-1 {
			desc = false
		}
	}
	return allSame || asc || desc
}

// VerifyAndUpgrade verifies a passcode and, when the stored hash uses a
// superseded scheme, returns a config carrying a freshly-derived current one
// (C284).
//
// The lazy migration only works if someone calls this. `Verify` discards the
// needsMigration flag, so every unlock through it left the old hash in place
// forever — the machinery existed and nothing drove it. Callers that can persist
// the config should use this instead.
//
// changed is true only when BOTH the passcode was correct AND the hash moved.
// A failed verify never upgrades: re-hashing on a wrong passcode would let
// anyone who can reach the lock screen overwrite the stored credential.
func (c Config) VerifyAndUpgrade(passcode string) (ok bool, upgraded Config, changed bool) {
	if !c.Enabled || c.Hash == "" || c.Salt == "" {
		return false, c, false
	}
	match, needsMigration, err := VerifyPasscode(passcode, c.Salt, c.Hash)
	if err != nil || !match {
		return false, c, false
	}
	if !needsMigration {
		return true, c, false
	}
	next := c
	next.Hash = HashPasscodeArgon2id(passcode, c.Salt)
	return true, next, true
}
