// Package pwcheck validates and scores the secret that guards a CashFlux app
// lock — a numeric PIN, a password, or a passphrase. It is pure Go (no
// syscall/js), so the rules are table-tested on native Go and reused unchanged
// by the WebAssembly UI.
//
// The design follows modern NIST SP 800-63B guidance: enforce a minimum length
// per credential type, screen against a bundled list of common/breached values,
// reject trivial numeric codes (all-same, sequences) — but otherwise favour
// urging good hygiene (a strength meter plus actionable suggestions) over
// forced-composition rules. There is a sane hard floor (OK): meet the minimum
// length, not be on the blocklist, and (for a PIN) not be trivial.
package pwcheck

import (
	"math"
	"strings"
	"unicode"
)

// Kind is the type of lock secret being validated.
type Kind string

const (
	// PIN is a digits-only numeric code, optimized for fast entry on a shared
	// device. It is the lowest-entropy option.
	PIN Kind = "pin"
	// Password is any printable/unicode secret of at least MinPasswordLen runes.
	Password Kind = "password"
	// Passphrase is several words; length is favoured over symbol composition.
	Passphrase Kind = "passphrase"
)

// Minimum lengths per credential type, per the B17.3 spec (NIST-aligned: length
// over composition).
const (
	// MinPINLen is the fewest digits a PIN may have.
	MinPINLen = 6
	// MinPasswordLen is the fewest runes a password may have.
	MinPasswordLen = 8
	// MinPassphraseLen is the fewest runes a passphrase may have.
	MinPassphraseLen = 12
	// MinPassphraseWords is the fewest whitespace-separated words a passphrase
	// must contain.
	MinPassphraseWords = 4
)

// Context supplies app- and household-specific words that should be treated as
// weak (a secret equal to or built around them is easy for someone nearby to
// guess). All matching is case-insensitive. The zero value adds no extra terms.
type Context struct {
	// Terms are words to screen against — typically the household name, member
	// names, and account names. The app name "cashflux" is always screened.
	Terms []string
}

// Result is the outcome of validating a secret. OK reports whether the value
// clears the hard floor (so the UI may allow it); Score is a 0–4 guessability
// estimate (0 = trivially guessable, 4 = strong) for the strength meter; Issues
// explains every reason it is weak or rejected; Suggestions offers concrete,
// friendly next steps.
type Result struct {
	OK          bool
	Score       int
	Issues      []string
	Suggestions []string
}

// Validate checks a secret of the given kind with no extra context. It is the
// common entry point; ValidateWithContext adds household-specific screening.
func Validate(kind Kind, value string) Result {
	return ValidateWithContext(kind, value, Context{})
}

// ValidateWithContext checks a secret of the given kind, additionally rejecting
// values that match ctx.Terms (household/member/account names). The returned
// Result is fully populated regardless of OK so the UI can always show a meter
// and feedback.
func ValidateWithContext(kind Kind, value string, ctx Context) Result {
	switch kind {
	case PIN:
		return validatePIN(value, ctx)
	case Password:
		return validateSecret(value, ctx, MinPasswordLen, "password")
	case Passphrase:
		return validatePassphrase(value, ctx)
	default:
		return Result{Issues: []string{"unknown credential type"}}
	}
}

func validatePIN(value string, ctx Context) Result {
	var r Result
	if !allDigits(value) {
		r.Issues = append(r.Issues, "a PIN must be digits only")
		r.Suggestions = append(r.Suggestions, "use only the numbers 0–9, or switch to a password")
		return r // a non-numeric PIN can't be scored as a PIN
	}
	if len(value) < MinPINLen {
		r.Issues = append(r.Issues, "a PIN needs at least 6 digits")
		r.Suggestions = append(r.Suggestions, "add more digits — longer is harder to guess")
	}
	if isTrivialDigits(value) || commonPINs[value] {
		r.Issues = append(r.Issues, "this PIN is one of the most common — it's easy to guess")
		r.Suggestions = append(r.Suggestions, "avoid repeats like 0000 and sequences like 1234")
	}
	if hit, term := matchesContext(value, ctx); hit {
		r.Issues = append(r.Issues, "this is based on "+term+", which someone nearby could guess")
	}
	r.Score = scorePIN(value)
	r.OK = len(value) >= MinPINLen && !isTrivialDigits(value) && !commonPINs[value]
	if r.OK && r.Score < 3 {
		r.Suggestions = append(r.Suggestions, "a longer PIN, or a password, would be stronger")
	}
	return r
}

func validatePassphrase(value string, ctx Context) Result {
	r := validateSecret(value, ctx, MinPassphraseLen, "passphrase")
	if words := wordCount(value); words < MinPassphraseWords {
		r.Issues = append(r.Issues, "a passphrase should be at least four words")
		r.Suggestions = append(r.Suggestions, "string together a few unrelated words")
		r.OK = false
	}
	return r
}

// validateSecret holds the rules shared by passwords and passphrases: a minimum
// rune length, blocklist screening, and context screening, plus an entropy-based
// score. label names the credential type in messages.
func validateSecret(value string, ctx Context, minLen int, label string) Result {
	var r Result
	runes := len([]rune(value))
	if runes < minLen {
		r.Issues = append(r.Issues, longMsg(label, minLen))
		r.Suggestions = append(r.Suggestions, "length matters most — make it longer")
	}
	onList := commonPasswords[normalizeForBlocklist(value)]
	if onList {
		r.Issues = append(r.Issues, "this is a commonly used "+label+" — it would be guessed quickly")
		r.Suggestions = append(r.Suggestions, "pick something unique to you")
	}
	hit, term := matchesContext(value, ctx)
	if hit {
		r.Issues = append(r.Issues, "this is based on "+term+", which someone nearby could guess")
	}
	r.Score = scoreSecret(value, onList || hit)
	r.OK = runes >= minLen && !onList && !hit
	if r.OK && r.Score < 3 {
		r.Suggestions = append(r.Suggestions, "add another word or a few more characters to strengthen it")
	}
	return r
}

func longMsg(label string, minLen int) string {
	switch label {
	case "passphrase":
		return "a passphrase needs at least 12 characters"
	default:
		return "a password needs at least 8 characters"
	}
}

// allDigits reports whether s is non-empty and every rune is an ASCII digit.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isTrivialDigits reports whether a digit string is all the same digit, a strict
// ascending run (1234), or a strict descending run (4321) — the patterns people
// reach for first.
func isTrivialDigits(s string) bool {
	if len(s) < 2 {
		return true
	}
	allSame, asc, desc := true, true, true
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			allSame = false
		}
		if s[i] != s[i-1]+1 {
			asc = false
		}
		if s[i] != s[i-1]-1 {
			desc = false
		}
	}
	return allSame || asc || desc
}

// matchesContext reports whether value contains (case-insensitively) the app
// name or any context term of three or more characters, returning the term hit.
func matchesContext(value string, ctx Context) (bool, string) {
	lower := strings.ToLower(value)
	terms := append([]string{"cashflux"}, ctx.Terms...)
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		if len(t) >= 3 && strings.Contains(lower, t) {
			return true, t
		}
	}
	return false, ""
}

// normalizeForBlocklist lowercases and trims a value for case-insensitive
// blocklist comparison.
func normalizeForBlocklist(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// wordCount returns the number of whitespace-separated words in s.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// scorePIN buckets a PIN into a 0–4 strength score from its digit-entropy,
// flooring trivial or common PINs at 0.
func scorePIN(s string) int {
	if isTrivialDigits(s) || commonPINs[s] {
		return 0
	}
	// log2(10) ≈ 3.32 bits per independent digit.
	return bucketEntropy(float64(len(s)) * 3.32)
}

// scoreSecret buckets a password/passphrase into a 0–4 strength score from an
// estimate of its entropy. A blocklist or context hit floors the score at 0.
func scoreSecret(s string, blocked bool) int {
	if blocked {
		return 0
	}
	return bucketEntropy(estimateEntropy(s))
}

// estimateEntropy approximates a secret's entropy in bits. For multi-word input
// it uses a word-count model (length is encouraged over symbols); otherwise it
// uses the character-pool model len*log2(poolSize).
func estimateEntropy(s string) float64 {
	if words := wordCount(s); words >= 3 {
		// ~11 bits per word is a conservative estimate for user-chosen words.
		return float64(words) * 11.0
	}
	pool := poolSize(s)
	if pool == 0 {
		return 0
	}
	return float64(len([]rune(s))) * math.Log2(float64(pool))
}

// poolSize estimates the size of the character set a secret draws from, which
// bounds its per-character entropy.
func poolSize(s string) int {
	var lower, upper, digit, symbol, other bool
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		case r > unicode.MaxASCII:
			other = true
		default:
			symbol = true
		}
	}
	pool := 0
	if lower {
		pool += 26
	}
	if upper {
		pool += 26
	}
	if digit {
		pool += 10
	}
	if symbol {
		pool += 32
	}
	if other {
		pool += 100 // a rough allowance for the unicode range
	}
	return pool
}

// bucketEntropy maps an entropy estimate (bits) to a 0–4 strength score using
// common guessability thresholds.
func bucketEntropy(bits float64) int {
	switch {
	case bits < 28:
		return 0
	case bits < 36:
		return 1
	case bits < 48:
		return 2
	case bits < 60:
		return 3
	default:
		return 4
	}
}
