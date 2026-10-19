// SPDX-License-Identifier: MIT

// Package textdecode decodes common text encodings used by financial statements.
package textdecode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"unicode/utf16"
	"unicode/utf8"
)

// Decode converts UTF-8, UTF-16, or Windows-1252 bytes to UTF-8 text.
// UTF-16 byte order is detected from a BOM or, for BOM-less ASCII-heavy files,
// from the position of NUL bytes. Invalid UTF-8 without a UTF-16 signature is
// interpreted as Windows-1252, as commonly exported by spreadsheet software.
func Decode(data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:]), nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeUTF16(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeUTF16(data[2:], binary.BigEndian)
	}

	if order, ok := detectUTF16ByteOrder(data); ok {
		return decodeUTF16(data, order)
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	return decodeWindows1252(data), nil
}

func detectUTF16ByteOrder(data []byte) (binary.ByteOrder, bool) {
	if len(data) < 4 || len(data)%2 != 0 {
		return nil, false
	}
	pairs := len(data) / 2
	var evenNUL, oddNUL int
	for i := 0; i+1 < len(data); i += 2 {
		if data[i] == 0 {
			evenNUL++
		}
		if data[i+1] == 0 {
			oddNUL++
		}
	}
	// Require a strong NUL-byte majority to avoid misclassifying arbitrary
	// binary or legacy single-byte text as UTF-16.
	if oddNUL*4 >= pairs*3 {
		return binary.LittleEndian, true
	}
	if evenNUL*4 >= pairs*3 {
		return binary.BigEndian, true
	}
	return nil, false
}

func decodeUTF16(data []byte, order binary.ByteOrder) (string, error) {
	if len(data)%2 != 0 {
		return "", errors.New("textdecode: UTF-16 input has an incomplete code unit")
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(units)), nil
}

func decodeWindows1252(data []byte) string {
	runes := make([]rune, len(data))
	for i, b := range data {
		r := rune(b)
		if b >= 0x80 && b <= 0x9F {
			r = windows1252High[b-0x80]
		}
		runes[i] = r
	}
	return string(runes)
}

var windows1252High = [32]rune{
	'\u20AC', '\uFFFD', '\u201A', '\u0192', '\u201E', '\u2026', '\u2020', '\u2021',
	'\u02C6', '\u2030', '\u0160', '\u2039', '\u0152', '\uFFFD', '\u017D', '\uFFFD',
	'\uFFFD', '\u2018', '\u2019', '\u201C', '\u201D', '\u2022', '\u2013', '\u2014',
	'\u02DC', '\u2122', '\u0161', '\u203A', '\u0153', '\uFFFD', '\u017E', '\u0178',
}
