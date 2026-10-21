// SPDX-License-Identifier: MIT

package undosnap

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
)

// ToSnapshotDataset converts the typed dataset directly to an undo snapshot.
// It mirrors store.Export's schema stamping and transfer-receipt pruning and
// clears the device credential on its value copy, then encodes each field/row
// with Export's indentation so RawMessage bytes match the export conversion.
func ToSnapshotDataset(ds store.Dataset) (history.Snapshot, error) {
	ds.SchemaVersion = store.SchemaVersion
	ds.Settings.OpenAIKey = ""
	ds.TransferOperations = liveTransferReceipts(ds.TransferOperations, ds.Transactions)

	value := reflect.ValueOf(ds)
	typeOf := value.Type()
	snap := make(history.Snapshot, typeOf.NumField())
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
				// A nil slice without omitempty exports as JSON null, which the
				// wire converter treats as scalar metadata rather than an array.
				snap["_meta:"+name] = map[string]json.RawMessage{name: json.RawMessage("null")}
				continue
			}
			rows := make(map[string]json.RawMessage, fieldValue.Len())
			for rowIndex := 0; rowIndex < fieldValue.Len(); rowIndex++ {
				row := fieldValue.Index(rowIndex).Interface()
				raw, err := marshalIndentedValue(row, "    ")
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
			snap[name] = rows
			continue
		}

		raw, err := marshalIndentedValue(fieldValue.Interface(), "  ")
		if err != nil {
			return nil, fmt.Errorf("undosnap: marshal field %s: %w", name, err)
		}
		snap["_meta:"+name] = map[string]json.RawMessage{name: raw}
	}
	return snap, nil
}

func jsonField(field reflect.StructField) (string, map[string]bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", nil, false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	options := make(map[string]bool, len(parts)-1)
	for _, option := range parts[1:] {
		options[option] = true
	}
	return name, options, true
}

func emptyJSONValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	}
	return false
}

func marshalIndentedValue(value any, prefix string) (json.RawMessage, error) {
	encoded, err := json.MarshalIndent(value, prefix, "  ")
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func liveTransferReceipts(receipts []store.TransferOperationReceipt, transactions []domain.Transaction) []store.TransferOperationReceipt {
	transactionIDs := make(map[string]struct{}, len(transactions))
	for _, transaction := range transactions {
		transactionIDs[transaction.ID] = struct{}{}
	}
	live := make([]store.TransferOperationReceipt, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt.ID == "" || len(receipt.Transactions) == 0 {
			continue
		}
		keep := true
		for _, transaction := range receipt.Transactions {
			if transaction.ID == "" {
				keep = false
				break
			}
			if _, ok := transactionIDs[transaction.ID]; !ok {
				keep = false
				break
			}
		}
		if keep {
			live = append(live, receipt)
		}
	}
	return live
}
