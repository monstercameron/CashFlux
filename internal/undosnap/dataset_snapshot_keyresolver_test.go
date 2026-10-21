// SPDX-License-Identifier: MIT

package undosnap

import (
	"encoding/json"
	"reflect"
	"testing"
)

type exactKeyRow struct {
	ID string `json:"id"`
}

type caseVariantKeyRow struct {
	Upper string `json:"ID"`
	Exact string `json:"id"`
}

type embeddedKey struct {
	ID string `json:"id"`
}

type embeddedKeyRow struct {
	embeddedKey
	Name string `json:"name"`
}

func TestDirectJSONFieldIndexUsesOnlyUnambiguousExactStringFields(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{name: "exact string", typ: reflect.TypeOf(exactKeyRow{}), want: true},
		{name: "same name with non-string field", typ: conflictingKeyRowType()},
		{name: "case variant", typ: reflect.TypeOf(caseVariantKeyRow{})},
		{name: "embedded field", typ: reflect.TypeOf(embeddedKeyRow{})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, got := directJSONFieldIndex(tc.typ, "id")
			if got != tc.want {
				t.Fatalf("directJSONFieldIndex safe = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractCollectionKeyTypedFallsBackForAmbiguousJSONFields(t *testing.T) {
	tests := []struct {
		name    string
		row     any
		wantKey string
		wantErr bool
	}{
		{name: "same JSON name is omitted", row: conflictingKeyRow(), wantErr: true},
		{name: "exact lowercase wins wire lookup", row: caseVariantKeyRow{Upper: "upper", Exact: "exact"}, wantKey: "exact"},
		{name: "promoted embedded key", row: embeddedKeyRow{embeddedKey: embeddedKey{ID: "promoted"}, Name: "row"}, wantKey: "promoted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.MarshalIndent(tc.row, "    ", "  ")
			if err != nil {
				t.Fatalf("MarshalIndent: %v", err)
			}
			row := reflect.ValueOf(tc.row)
			fields := resolveCollectionKeyFields("accounts", row.Type())
			got, err := extractCollectionKeyTyped("accounts", row, raw, fields)
			if (err != nil) != tc.wantErr {
				t.Fatalf("extractCollectionKeyTyped error = %v, wantErr %v (JSON %s)", err, tc.wantErr, raw)
			}
			if err == nil && got != tc.wantKey {
				t.Fatalf("extractCollectionKeyTyped = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

func conflictingKeyRowType() reflect.Type {
	return reflect.StructOf([]reflect.StructField{
		{Name: "ID", Type: reflect.TypeOf(""), Tag: `json:"id"`},
		{Name: "Other", Type: reflect.TypeOf(int(0)), Tag: `json:"id"`},
	})
}

func conflictingKeyRow() any {
	value := reflect.New(conflictingKeyRowType()).Elem()
	value.Field(0).SetString("typed-would-be-wrong")
	value.Field(1).SetInt(7)
	return value.Interface()
}
