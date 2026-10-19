// SPDX-License-Identifier: MIT

// Package copytext carries user-facing copy as a catalog KEY plus its
// ARGUMENTS, rather than as a finished English sentence.
//
// It exists because the app's logic packages are pure — they have no language
// context and must not acquire one — yet they are where a lot of the app's
// sentences are actually written. `internal/smartengine` composes the copy for
// ~84 insight features, `internal/widgetcatalog` names every widget, column and
// chart. They built those strings by concatenation at detection time, which had
// two consequences (V-sweep C362):
//
//   - the language setting could not reach any of it, and
//   - the notification feed PERSISTED the finished English, so a household that
//     switched language kept an archive in the old one forever. That half is not
//     a translation gap but a data bug: the information needed to re-render the
//     sentence had already been thrown away when it was written.
//
// A Text keeps all three parts — the key, the arguments, and the English the
// producer would otherwise have baked in. The fallback means a caller with no
// translator (a test, a CSV export, a log line) still gets a real sentence, and
// a key the catalog does not know yet degrades to correct English instead of
// showing a raw key name. Resolve is what a UI calls.
//
// Pure Go, no syscall/js; unit-tested on native Go.
package copytext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

var formattingWarning sync.Once

// Translator renders a catalog key with arguments. It is the shape of
// uistate.T, injected so pure packages never import the UI layer.
type Translator func(key string, args ...any) string

// Text is one piece of user-facing copy.
//
// The JSON form is what a persisted notification stores, so history can be
// re-rendered in a language chosen later. Integral arguments are restored as
// int64 values so catalog verbs such as %d keep working after a round trip.
type Text struct {
	Key      string `json:"key,omitempty"`
	Args     []any  `json:"args,omitempty"`
	Fallback string `json:"text,omitempty"`
}

// Of builds a Text from a catalog key, the English format the producer would
// have written, and the arguments that fill both.
func Of(key, format string, args ...any) Text {
	return Text{Key: key, Args: args, Fallback: fmt.Sprintf(format, args...)}
}

// OfKey builds a Text when the translated string is already rendered and its
// format is not a compile-time constant. It keeps the key and arguments for
// future re-translation while storing rendered as the English fallback.
func OfKey(key, rendered string, args ...any) Text {
	return Text{Key: key, Args: args, Fallback: rendered}
}

// MarshalJSON preserves the existing persisted field names for a Text.
func (t Text) MarshalJSON() ([]byte, error) {
	type textAlias Text
	data, err := json.Marshal(textAlias(t))
	if err != nil {
		return nil, fmt.Errorf("encode copy text: %w", err)
	}
	return data, nil
}

// UnmarshalJSON restores numeric argument types from the persisted JSON form.
// JSON has one number syntax, so integral values become int64 and fractional
// values become float64. Other JSON types retain their usual Go types.
func (t *Text) UnmarshalJSON(data []byte) error {
	type textFields struct {
		Key      string          `json:"key,omitempty"`
		Args     json.RawMessage `json:"args,omitempty"`
		Fallback string          `json:"text,omitempty"`
	}
	var fields textFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode copy text: %w", err)
	}
	t.Key = fields.Key
	t.Fallback = fields.Fallback
	t.Args = nil
	if len(fields.Args) == 0 || bytes.Equal(bytes.TrimSpace(fields.Args), []byte("null")) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(fields.Args))
	decoder.UseNumber()
	if err := decoder.Decode(&t.Args); err != nil {
		return fmt.Errorf("decode copy text arguments: %w", err)
	}
	for i, arg := range t.Args {
		number, ok := arg.(json.Number)
		if !ok {
			continue
		}
		if integer, err := number.Int64(); err == nil {
			t.Args[i] = integer
			continue
		}
		floating, err := number.Float64()
		if err != nil {
			return fmt.Errorf("decode copy text argument %d: %w", i, err)
		}
		t.Args[i] = floating
	}
	return nil
}

// Plain wraps a finished sentence that has no catalog key yet. It is the
// honest intermediate state for copy mid-conversion: it renders correctly and
// it is visibly untranslatable, rather than pretending to be either.
func Plain(s string) Text { return Text{Fallback: s} }

// Empty reports whether there is nothing to render.
func (t Text) Empty() bool { return t.Fallback == "" && t.Key == "" }

// String renders the producer's English. It satisfies fmt.Stringer so a Text
// drops into logs, exports and tests unchanged.
func (t Text) String() string { return t.Fallback }

// Resolve renders through a translator, falling back to the producer's English
// when there is no key, no translator, or the translator does not know the key.
//
// The last case is the important one: a catalog that has not caught up with a
// new detector should show that detector's English, not the string
// "smart.a4.title".
func (t Text) Resolve(tr Translator) string {
	if t.Key == "" || tr == nil {
		return t.Fallback
	}
	got := tr(t.Key, t.Args...)
	if got == "" || got == t.Key {
		return t.Fallback
	}
	if strings.Contains(got, "%!") {
		formattingWarning.Do(func() {
			slog.Warn("translated copy contains a formatting error; using stored fallback", "key", t.Key)
		})
		return t.Fallback
	}
	return got
}
