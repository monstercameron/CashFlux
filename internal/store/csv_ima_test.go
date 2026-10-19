// SPDX-License-Identifier: MIT

package store

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestIMACSVEncodingsAndSourceRecordErrors(t *testing.T) {
	text := "date,desc,amount\n2026-06-01,Caf\u00e9,-4.50\n"
	utf16Bytes := func(order binary.ByteOrder, bom []byte) []byte {
		out := append([]byte(nil), bom...)
		for _, unit := range utf16.Encode([]rune(text)) {
			var encoded [2]byte
			order.PutUint16(encoded[:], unit)
			out = append(out, encoded[:]...)
		}
		return out
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"UTF8", []byte(text)}, {"UTF8 BOM", append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...)},
		{"UTF16 LE", utf16Bytes(binary.LittleEndian, []byte{0xff, 0xfe})},
		{"UTF16 BE", utf16Bytes(binary.BigEndian, []byte{0xfe, 0xff})},
		{"Windows1252", []byte(strings.ReplaceAll(text, "\u00e9", string([]byte{0xe9})))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, failed, err := TransactionsFromCSVResilientForAccountList(tc.data, "USD", "chk", []CSVAccount{{ID: "chk", Currency: "USD"}})
			if err != nil || len(failed) != 0 || len(rows) != 1 || rows[0].Desc != "Caf\u00e9" || rows[0].Amount.Amount != -450 {
				t.Fatalf("rows=%+v failed=%+v err=%v", rows, failed, err)
			}
		})
	}
	_, _, err := TransactionsFromCSVResilient([]byte{0xff, 0xfe, 0x61}, "USD")
	if err == nil {
		t.Fatal("incomplete UTF16 was silently accepted")
	}
}
