// SPDX-License-Identifier: MIT

package changes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Marshal encodes a batch in its canonical form: deterministic field order,
// ops sorted by (table, record), removed-field lists sorted, JSON object
// keys sorted at every depth, and no insignificant whitespace. The same
// batch always produces the same bytes, which is what lets the ciphertext of
// a batch be compared and deduplicated byte-for-byte. It validates the batch
// first and returns the validation error instead of encoding a bad batch.
func Marshal(b Batch) ([]byte, error) { return b.Marshal() }

// Marshal is the method form of the package-level Marshal.
func (b Batch) Marshal() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	norm, err := canonicalBatch(b)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(batchAlias(norm))
	if err != nil {
		return nil, fmt.Errorf("changes: marshal batch %s: %w", b.ChangeID, err)
	}
	if len(data) > MaxBatchBytes {
		return nil, fmt.Errorf("changes: batch %s encodes to %d bytes, limit is %d: %w",
			b.ChangeID, len(data), MaxBatchBytes, ErrInvalidBatch)
	}
	return data, nil
}

// MarshalJSON implements json.Marshaler with the canonical encoding, so a
// Batch embedded in any larger JSON document is still deterministic.
func (b Batch) MarshalJSON() ([]byte, error) { return b.Marshal() }

// UnmarshalJSON implements json.Unmarshaler with strict validation.
func (b *Batch) UnmarshalJSON(data []byte) error {
	decoded, err := Unmarshal(data)
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}

// UnmarshalBatch is an alias for Unmarshal for callers that prefer the
// explicit name.
func UnmarshalBatch(data []byte) (Batch, error) { return Unmarshal(data) }

// MarshalBatch is an alias for Marshal.
func MarshalBatch(b Batch) ([]byte, error) { return Marshal(b) }

// batchAlias mirrors Batch without its MarshalJSON/UnmarshalJSON methods, so
// the codec can use the standard decoder without recursing.
type batchAlias Batch

// Unmarshal decodes a canonical batch with strict validation: unknown JSON
// fields, trailing data, oversized payloads and every Validate rule are
// rejected.
//
// A batch whose SchemaVersion exceeds the supported range is the one
// exception to "no batch on error": the decoded batch is returned together
// with an error wrapping ErrNewerSchema, so the caller can quarantine it by
// ChangeID instead of dropping a newer client's write on the floor. Field
// values from such a batch are decoded leniently (unknown fields are dropped
// by the standard decoder) and must not be applied.
func Unmarshal(data []byte) (Batch, error) {
	if len(data) == 0 {
		return Batch{}, fmt.Errorf("changes: empty batch encoding: %w", ErrInvalidBatch)
	}
	if len(data) > MaxBatchBytes {
		return Batch{}, fmt.Errorf("changes: batch encoding is %d bytes, limit is %d: %w",
			len(data), MaxBatchBytes, ErrInvalidBatch)
	}
	if err := preflightBatch(data); err != nil {
		return Batch{}, fmt.Errorf("changes: batch preflight: %w: %v", ErrInvalidBatch, err)
	}
	// Peek at the schema version first so a newer batch is reported as
	// ErrNewerSchema even if it carries fields this build does not know.
	var peek struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &peek); err == nil && peek.SchemaVersion > MaxSupportedSchemaVersion {
		var alias batchAlias
		if derr := json.Unmarshal(data, &alias); derr != nil {
			return Batch{}, fmt.Errorf("changes: decode newer-schema batch: %w: %v", ErrNewerSchema, derr)
		}
		b := Batch(alias)
		return b, fmt.Errorf("changes: batch %s has schemaVersion %d, newest supported is %d: %w",
			b.ChangeID, b.SchemaVersion, MaxSupportedSchemaVersion, ErrNewerSchema)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var alias batchAlias
	if err := dec.Decode(&alias); err != nil {
		return Batch{}, fmt.Errorf("changes: decode batch: %w: %v", ErrInvalidBatch, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Batch{}, fmt.Errorf("changes: trailing data after batch: %w", ErrInvalidBatch)
	}
	b := Batch(alias)
	if err := b.Validate(); err != nil {
		return Batch{}, err
	}
	norm, err := canonicalBatch(b)
	if err != nil {
		return Batch{}, err
	}
	return norm, nil
}

// preflightBatch bounds structural work before decoding the batch into slices,
// maps, and raw field values. The token walk is linear in the encoded size and
// stops as soon as a limit is crossed.
func preflightBatch(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	limits := batchPreflight{}
	if err := scanBatchValue(dec, 1, "", &limits); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

type batchPreflight struct {
	ops     int
	fields  int
	removed int
}

func scanBatchValue(dec *json.Decoder, depth int, path string, limits *batchPreflight) error {
	if depth > MaxJSONDepth {
		return fmt.Errorf("JSON nesting exceeds %d", MaxJSONDepth)
	}
	if path == "ops[]" {
		limits.ops++
		limits.fields = 0
		limits.removed = 0
		if limits.ops > MaxOpsPerBatch {
			return fmt.Errorf("ops exceed %d", MaxOpsPerBatch)
		}
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if path == "ops[].fields" {
				limits.fields++
				if limits.fields > MaxFieldsPerOp {
					return fmt.Errorf("fields exceed %d", MaxFieldsPerOp)
				}
			}
			child := ""
			switch {
			case path == "" && key == "ops":
				child = "ops"
			case path == "ops[]" && key == "fields":
				child = "ops[].fields"
			case path == "ops[]" && key == "removed":
				child = "ops[].removed"
			}
			if err := scanBatchValue(dec, depth+1, child, limits); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case '[':
		for dec.More() {
			child := ""
			if path == "ops" {
				child = "ops[]"
			} else if path == "ops[].removed" {
				limits.removed++
				if limits.fields+limits.removed > MaxFieldsPerOp {
					return fmt.Errorf("fields and removals exceed %d", MaxFieldsPerOp)
				}
			}
			if err := scanBatchValue(dec, depth+1, child, limits); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected closing delimiter %q", delim)
	}
}

// canonicalBatch returns a normalized deep copy: ops in canonical order and
// every field value re-encoded canonically.
func canonicalBatch(b Batch) (Batch, error) {
	out := b
	out.Ops = normalizeOps(b.Ops)
	for i := range out.Ops {
		for name, raw := range out.Ops[i].Fields {
			canon, err := canonicalJSONValue(raw)
			if err != nil {
				return Batch{}, fmt.Errorf("changes: batch %s field %q: %w: %v",
					b.ChangeID, name, ErrInvalidBatch, err)
			}
			out.Ops[i].Fields[name] = canon
		}
	}
	return out, nil
}

// canonicalJSONValue re-encodes one JSON value with object keys sorted at
// every depth and no insignificant whitespace. Number literals are preserved
// verbatim (1e2 and 100 stay distinct spellings); this is safe for
// convergence because replicas store and copy the same literal from the same
// batch.
func canonicalJSONValue(raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	var buf bytes.Buffer
	if err := writeCanonicalValue(&buf, v); err != nil {
		return nil, err
	}
	return json.RawMessage(buf.Bytes()), nil
}

// writeCanonicalValue writes a decoded JSON value in canonical form.
func writeCanonicalValue(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		buf.WriteString(t.String())
	case string:
		s, err := marshalJSONString(t)
		if err != nil {
			return err
		}
		buf.Write(s)
	case []any:
		buf.WriteByte('[')
		for i, el := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalValue(buf, el); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			ks, err := marshalJSONString(k)
			if err != nil {
				return err
			}
			buf.Write(ks)
			buf.WriteByte(':')
			if err := writeCanonicalValue(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("unsupported JSON value type %T", v)
	}
	return nil
}

// marshalJSONString encodes s as a JSON string without HTML escaping and
// without the trailing newline json.Encoder appends.
func marshalJSONString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// canonicalObject builds a canonical JSON object from raw field values.
func canonicalObject(fields map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		ks, err := marshalJSONString(k)
		if err != nil {
			return nil, err
		}
		buf.Write(ks)
		buf.WriteByte(':')
		canon, err := canonicalJSONValue(fields[k])
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		buf.Write(canon)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
