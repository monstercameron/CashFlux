// SPDX-License-Identifier: MIT

package undosnap

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

var snapshotBenchmarkSink history.Snapshot

type customMarshaledRow struct {
	ID string `json:"id"`
}

func (row customMarshaledRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID string `json:"id"`
	}{ID: "encoded-" + row.ID})
}

func TestCollectionKeyMetadataFallsBackForUnverifiedCustomMarshalers(t *testing.T) {
	fields := resolveCollectionKeyFields("accounts", reflect.TypeOf(customMarshaledRow{}))
	if fields.safe {
		t.Fatal("custom JSON row encoder must use the raw JSON key fallback")
	}

	row := customMarshaledRow{ID: "source"}
	raw, err := marshalIndentedValue(row, "    ")
	if err != nil {
		t.Fatal(err)
	}
	key, err := extractCollectionKeyTyped("accounts", reflect.ValueOf(row), raw, fields)
	if err != nil {
		t.Fatal(err)
	}
	if key != "encoded-source" {
		t.Fatalf("fallback key = %q, want marshaled key", key)
	}
}

func BenchmarkSnapshot3000Transactions(b *testing.B) {
	ds := store.Dataset{Transactions: make([]domain.Transaction, 3000), Settings: store.Settings{BaseCurrency: "USD", OpenAIKey: "device-secret"}}
	for i := range ds.Transactions {
		ds.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("txn-%04d", i), Amount: money.New(int64(i*17), "USD"), Desc: "benchmark row"}
	}

	redacted := ds
	redacted.Settings.OpenAIKey = ""
	exported, err := store.Export(redacted)
	if err != nil {
		b.Fatal(err)
	}
	want, err := ToSnapshot(exported)
	if err != nil {
		b.Fatal(err)
	}
	got, err := ToSnapshotDataset(ds)
	if err != nil {
		b.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		b.Fatal("typed snapshot differs from export snapshot")
	}

	b.Run("legacy-export-parse", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := store.Export(redacted)
			if err != nil {
				b.Fatal(err)
			}
			snapshotBenchmarkSink, err = ToSnapshot(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("typed-baseline", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			snapshotBenchmarkSink, err = baselineTypedSnapshot(ds)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("typed-optimized", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			snapshotBenchmarkSink, err = ToSnapshotDataset(ds)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// baselineTypedSnapshot reproduces the pre-key-metadata typed path: each row
// is marshaled, then unmarshaled again by extractCollectionKey.
func baselineTypedSnapshot(ds store.Dataset) (history.Snapshot, error) {
	ds.SchemaVersion = store.SchemaVersion
	ds.Settings.OpenAIKey = ""
	ds.TransferOperations = liveTransferReceipts(ds.TransferOperations, ds.Transactions)
	value := reflect.ValueOf(ds)
	typeOf := value.Type()
	snapshot := make(history.Snapshot, typeOf.NumField())
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		name, options, ok := jsonField(field)
		if !ok {
			continue
		}
		fieldValue := value.Field(i)
		if options["omitempty"] && emptyJSONValue(fieldValue) {
			continue
		}
		if fieldValue.Kind() == reflect.Slice {
			if fieldValue.IsNil() {
				snapshot["_meta:"+name] = map[string]json.RawMessage{name: json.RawMessage("null")}
				continue
			}
			rows := make(map[string]json.RawMessage, fieldValue.Len())
			for rowIndex := 0; rowIndex < fieldValue.Len(); rowIndex++ {
				raw, err := marshalIndentedValue(fieldValue.Index(rowIndex).Interface(), "    ")
				if err != nil {
					return nil, fmt.Errorf("undosnap: marshal %s row %d: %w", name, rowIndex, err)
				}
				key, err := extractCollectionKey(name, raw)
				if err != nil {
					return nil, fmt.Errorf("undosnap: element %d of %q: %w", rowIndex, name, err)
				}
				if _, exists := rows[key]; exists {
					return nil, fmt.Errorf("undosnap: duplicate row key %q in %q", key, name)
				}
				rows[key] = raw
			}
			snapshot[name] = rows
			continue
		}
		raw, err := marshalIndentedValue(fieldValue.Interface(), "  ")
		if err != nil {
			return nil, fmt.Errorf("undosnap: marshal field %s: %w", name, err)
		}
		snapshot["_meta:"+name] = map[string]json.RawMessage{name: raw}
	}
	return snapshot, nil
}
