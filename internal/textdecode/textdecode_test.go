// SPDX-License-Identifier: MIT

package textdecode

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestDecodeEncodings(t *testing.T) {
	utf16Text := "Date,Description,Amount\n2026-06-01,Caf\u00e9,1.00\n"
	utf16LE := encodeUTF16(utf16Text, binary.LittleEndian)
	utf16BE := encodeUTF16(utf16Text, binary.BigEndian)
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"UTF-8 BOM", append([]byte{0xEF, 0xBB, 0xBF}, []byte("plain\n")...), "plain\n"},
		{"UTF-16LE BOM", append([]byte{0xFF, 0xFE}, utf16LE...), utf16Text},
		{"UTF-16BE BOM", append([]byte{0xFE, 0xFF}, utf16BE...), utf16Text},
		{"UTF-16LE without BOM", utf16LE, utf16Text},
		{"UTF-16BE without BOM", utf16BE, utf16Text},
		{"Windows-1252", []byte{'C', 'a', 'f', 0xE9, ' ', 0x96, ' ', 0x80}, "Caf\u00e9 \u2013 \u20ac"},
		{"valid UTF-8", []byte("caf\u00e9"), "caf\u00e9"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Decode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecodeRejectsIncompleteUTF16(t *testing.T) {
	if _, err := Decode([]byte{0xFF, 0xFE, 0x41}); err == nil || !strings.Contains(err.Error(), "incomplete code unit") {
		t.Fatalf("Decode() error = %v, want incomplete UTF-16 code unit", err)
	}
}

func encodeUTF16(s string, order binary.ByteOrder) []byte {
	units := utf16.Encode([]rune(s))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		order.PutUint16(data[i*2:], unit)
	}
	return data
}
