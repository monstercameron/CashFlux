// SPDX-License-Identifier: MIT

package undosnap

import (
	"encoding"
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
			keyFields := resolveCollectionKeyFields(name, field.Type.Elem())
			for rowIndex := 0; rowIndex < fieldValue.Len(); rowIndex++ {
				rowValue := fieldValue.Index(rowIndex)
				row := rowValue.Interface()
				raw, err := marshalIndentedValue(row, "    ")
				if err != nil {
					return nil, fmt.Errorf("undosnap: marshal %s row %d: %w", name, rowIndex, err)
				}
				key, err := extractCollectionKeyTyped(name, rowValue, raw, keyFields)
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

type collectionKeyFields struct {
	primaryIndex []int
	surfaceIndex []int
	safe         bool
}

// resolveCollectionKeyFields examines a row type once per collection. It only
// enables typed extraction for direct, unique, exported string fields with the
// same effective JSON names used by the wire key extractor. Unknown shapes and
// custom JSON marshalers use the raw-JSON fallback.
func resolveCollectionKeyFields(collection string, rowType reflect.Type) collectionKeyFields {
	if rowType.Kind() == reflect.Pointer {
		rowType = rowType.Elem()
	}
	if rowType.Kind() != reflect.Struct || (hasCustomRowEncoding(rowType) && !customJSONPreservesCollectionKey(collection, rowType)) {
		return collectionKeyFields{}
	}
	primaryName := "id"
	if collection == "molecules" {
		primaryName = "name"
	}
	primaryIndex, primaryOK := directJSONFieldIndex(rowType, primaryName)
	if !primaryOK {
		return collectionKeyFields{}
	}
	fields := collectionKeyFields{primaryIndex: primaryIndex, safe: true}
	if collection == "placements" {
		fields.surfaceIndex, fields.safe = directJSONFieldIndex(rowType, "surface")
		if !fields.safe {
			return collectionKeyFields{}
		}
	}
	return fields
}

// Transaction.MarshalJSON normalizes the owner fields on a value copy, then
// marshals an alias of the full struct. Its explicit id field is not changed,
// so direct key extraction remains equivalent while the conservative fallback
// stays enabled for every other custom row encoder.
func customJSONPreservesCollectionKey(collection string, rowType reflect.Type) bool {
	return collection == "transactions" && rowType == reflect.TypeOf(domain.Transaction{})
}

func hasCustomRowEncoding(rowType reflect.Type) bool {
	jsonMarshaler := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshaler := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	for _, typ := range []reflect.Type{rowType, reflect.PointerTo(rowType)} {
		if typ.Implements(jsonMarshaler) || typ.Implements(textMarshaler) {
			return true
		}
	}
	return false
}

func directJSONFieldIndex(rowType reflect.Type, wanted string) ([]int, bool) {
	var index []int
	for i := 0; i < rowType.NumField(); i++ {
		field := rowType.Field(i)
		if field.Anonymous {
			// Embedded fields can promote JSON names in ways that are not
			// captured by this direct-field scan. Keep these row types on the
			// wire-key path, even when the embedded field looks unrelated.
			return nil, false
		}
		if field.PkgPath != "" {
			continue
		}
		name, _, ok := jsonField(field)
		if !ok || !strings.EqualFold(name, wanted) {
			continue
		}
		// encoding/json accepts case-insensitive field matches, while the
		// collection key extractor reads the exact canonical JSON property.
		// Any case variant or same-name field of another type is ambiguous for
		// typed extraction, even if an exact string field is also present.
		if name != wanted || field.Type.Kind() != reflect.String || index != nil {
			return nil, false
		}
		index = field.Index
	}
	return index, index != nil
}

func extractCollectionKeyTyped(collection string, row reflect.Value, raw json.RawMessage, fields collectionKeyFields) (string, error) {
	if !fields.safe || row.Kind() == reflect.Pointer || row.Kind() != reflect.Struct {
		return extractCollectionKey(collection, raw)
	}
	primary := row.FieldByIndex(fields.primaryIndex).String()
	if primary == "" {
		if collection == "molecules" {
			return "", fmt.Errorf("element has no non-empty \"name\" field")
		}
		return "", fmt.Errorf("element has no non-empty \"id\" field")
	}
	if collection == "placements" {
		surface := row.FieldByIndex(fields.surfaceIndex).String()
		if surface == "" {
			return "", fmt.Errorf("placement has no non-empty \"surface\" or \"id\" field")
		}
		return surface + "/" + primary, nil
	}
	return primary, nil
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
