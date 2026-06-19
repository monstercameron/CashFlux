package pwcheck

// This file bundles a small, offline screening list of the most commonly used
// passwords and numeric PINs. It is intentionally compact — a complete breach
// corpus is far too large to embed — but it catches the values an attacker
// guesses first, which is where a hard-reject delivers the most protection.
//
// Lookups are case-insensitive for passwords (see normalizeForBlocklist).

// commonPasswords is a curated set of the most-guessed passwords, drawn from the
// public breach top-lists. Stored lowercase; callers normalize before lookup.
var commonPasswords = map[string]bool{
	"password":      true,
	"password1":     true,
	"password123":   true,
	"passw0rd":      true,
	"123456":        true,
	"1234567":       true,
	"12345678":      true,
	"123456789":     true,
	"1234567890":    true,
	"qwerty":        true,
	"qwertyuiop":    true,
	"qwerty123":     true,
	"abc123":        true,
	"111111":        true,
	"123123":        true,
	"000000":        true,
	"iloveyou":      true,
	"admin":         true,
	"administrator": true,
	"welcome":       true,
	"welcome1":      true,
	"monkey":        true,
	"dragon":        true,
	"letmein":       true,
	"login":         true,
	"princess":      true,
	"sunshine":      true,
	"master":        true,
	"football":      true,
	"baseball":      true,
	"superman":      true,
	"trustno1":      true,
	"whatever":      true,
	"shadow":        true,
	"michael":       true,
	"ashley":        true,
	"qazwsx":        true,
	"zxcvbn":        true,
	"asdfgh":        true,
	"cashflux":      true,
	"changeme":      true,
	"secret":        true,
}

// commonPINs is the set of numeric codes that attackers try first — the trivial
// ones (all-same, sequences) plus the empirically most-popular PINs. Sequences
// and repeats are also caught algorithmically by isTrivialDigits, so a value
// need not be listed here to be rejected.
var commonPINs = map[string]bool{
	// Classic 4-digit offenders.
	"1234": true, "1111": true, "0000": true, "1212": true, "7777": true,
	"1004": true, "2000": true, "4444": true, "2222": true, "6969": true,
	"9999": true, "3333": true, "5555": true, "6666": true, "1122": true,
	"1313": true, "8888": true, "4321": true, "2001": true, "1010": true,
	// Common 6-digit offenders.
	"123456": true, "654321": true, "111111": true, "000000": true,
	"121212": true, "112233": true, "123123": true, "159753": true,
	"147258": true, "789456": true,
}
