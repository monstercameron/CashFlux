// SPDX-License-Identifier: MIT

package keyring

import (
	"fmt"
	"strings"
)

// RecoverySecretSize is the size in bytes of a recovery secret (128 bits).
const RecoverySecretSize = 16

// RecoveryDataChars is the number of Crockford base32 characters that encode
// the 128-bit recovery secret. 26 characters carry 130 bits; the top two
// bits are always zero, so the first character is one of 0–7.
const RecoveryDataChars = 26

// RecoveryCodeChars is the total number of characters in a recovery code:
// the data characters plus one checksum character. Codes are displayed in
// hyphen-separated groups of four (the final group holds the last three).
const RecoveryCodeChars = RecoveryDataChars + 1

// crockfordAlphabet is the Crockford base32 alphabet. It omits I, L, O, and
// U so codes survive handwriting and reading aloud; decoding accepts I/L as
// 1 and O as 0.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// crockfordValue maps an (uppercased) character to its 5-bit value, or -1.
func crockfordValue(c byte) int {
	switch c {
	case 'O':
		return 0
	case 'I', 'L':
		return 1
	}
	return strings.IndexByte(crockfordAlphabet, c)
}

// GenerateRecoveryCode creates a fresh 128-bit recovery secret and returns
// its human-readable code along with the raw secret. Show the code to the
// user once and zero the secret after wrapping.
func GenerateRecoveryCode() (code string, secret []byte, err error) {
	secret, err = randomBytes(RecoverySecretSize)
	if err != nil {
		return "", nil, err
	}
	code, err = EncodeRecoverySecret(secret)
	if err != nil {
		clearBytes(secret)
		return "", nil, err
	}
	return code, secret, nil
}

// EncodeRecoverySecret renders a 16-byte recovery secret as a grouped
// Crockford base32 code with a trailing checksum character.
func EncodeRecoverySecret(secret []byte) (string, error) {
	if len(secret) != RecoverySecretSize {
		return "", fmt.Errorf("%w: recovery secret is %d bytes, want %d", ErrInvalidRecoveryCode, len(secret), RecoverySecretSize)
	}
	values := secretToValues(secret)
	chars := make([]byte, 0, RecoveryCodeChars)
	for _, v := range values {
		chars = append(chars, crockfordAlphabet[v])
	}
	chars = append(chars, crockfordAlphabet[recoveryChecksum(values)])
	return groupRecoveryCode(string(chars)), nil
}

// DecodeRecoveryCode parses a recovery code as typed by the user. Hyphens
// and spaces are ignored, case does not matter, and the common look-alikes
// O/0 and I/L/1 are accepted. A single wrong character always fails: either
// it is outside the alphabet, pushes the first character past 7, or breaks
// the CRC-5 checksum, which detects every error confined to one 5-bit
// character. Errors match ErrInvalidRecoveryCode.
//
// The returned secret is the user's only copy of those bytes; zero it after
// use.
func DecodeRecoveryCode(code string) ([]byte, error) {
	values, err := parseRecoveryCode(code)
	if err != nil {
		return nil, err
	}
	data, check := values[:RecoveryDataChars], values[RecoveryDataChars]
	// The secret is 128 bits in a 130-bit field: the top two bits (held in
	// the first character) must be zero.
	if data[0] > 0b00111 {
		return nil, fmt.Errorf("%w: first character %q is out of range for a recovery code", ErrInvalidRecoveryCode, crockfordAlphabet[data[0]])
	}
	if recoveryChecksum(data) != check {
		return nil, fmt.Errorf("%w: checksum does not match — check for a typing mistake", ErrInvalidRecoveryCode)
	}
	return valuesToSecret(data), nil
}

// NormalizeRecoveryCode rewrites a user-typed code in its canonical grouped
// form, validating it in the process. Handy for echoing the code back before
// an unwrap attempt.
func NormalizeRecoveryCode(code string) (string, error) {
	secret, err := DecodeRecoveryCode(code)
	if err != nil {
		return "", err
	}
	defer clearBytes(secret)
	return EncodeRecoverySecret(secret)
}

// parseRecoveryCode strips separators and converts a code into 5-bit values.
func parseRecoveryCode(code string) ([]int, error) {
	values := make([]int, 0, RecoveryCodeChars)
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch c {
		case '-', ' ', '\t', '\n', '\r':
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		v := crockfordValue(c)
		if v < 0 {
			return nil, fmt.Errorf("%w: character %q is not part of a recovery code", ErrInvalidRecoveryCode, rune(code[i]))
		}
		values = append(values, v)
	}
	if len(values) != RecoveryCodeChars {
		return nil, fmt.Errorf("%w: code has %d characters, want %d", ErrInvalidRecoveryCode, len(values), RecoveryCodeChars)
	}
	return values, nil
}

// secretToValues splits a 16-byte secret into 26 MSB-first 5-bit values,
// left-padded with two zero bits.
func secretToValues(secret []byte) []int {
	values := make([]int, 0, RecoveryDataChars)
	acc := 0 // bit accumulator
	bits := 0
	// Two leading zero bits round 128 up to a whole number of characters.
	bits = 2
	for _, b := range secret {
		acc = (acc << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			values = append(values, (acc>>bits)&0x1f)
		}
		acc &= (1 << bits) - 1
	}
	if bits > 0 {
		values = append(values, (acc<<(5-bits))&0x1f)
	}
	return values
}

// valuesToSecret reverses secretToValues: it drops the two leading padding
// bits of the first value and regroups the remaining 128 bits into bytes.
// Callers must already have checked that those padding bits are zero.
func valuesToSecret(values []int) []byte {
	secret := make([]byte, 0, RecoverySecretSize)
	acc := 0
	bits := 0
	for i, v := range values {
		width := 5
		if i == 0 {
			// First value: keep only its low 3 bits (the top 2 are padding).
			v &= 0b00111
			width = 3
		}
		acc = (acc << width) | v
		bits += width
		for bits >= 8 {
			bits -= 8
			secret = append(secret, byte((acc>>bits)&0xff))
		}
		acc &= (1 << bits) - 1
	}
	return secret
}

// recoveryChecksum computes a CRC-5 (polynomial x^5 + x^2 + 1) over the
// 130 data bits, MSB first, with the usual 5-bit zero augmentation. A CRC
// of degree 5 detects every burst error of up to 5 bits, and one mistyped
// character changes at most a 5-bit window — so any single-character typo is
// caught. (The checksum is a typo guard only; it adds no entropy.)
func recoveryChecksum(data []int) int {
	const poly = 0b00101 // x^5 + x^2 + 1, without the implicit x^5 term
	reg := 0
	feed := func(bit int) {
		feedback := ((reg >> 4) & 1) ^ bit
		reg = (reg << 1) & 0x1f
		if feedback == 1 {
			reg ^= poly
		}
	}
	for _, v := range data {
		for shift := 4; shift >= 0; shift-- {
			feed((v >> shift) & 1)
		}
	}
	for i := 0; i < 5; i++ {
		feed(0)
	}
	return reg
}

// groupRecoveryCode inserts hyphens every four characters.
func groupRecoveryCode(chars string) string {
	var b strings.Builder
	for i := 0; i < len(chars); i += 4 {
		end := i + 4
		if end > len(chars) {
			end = len(chars)
		}
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(chars[i:end])
	}
	return b.String()
}
