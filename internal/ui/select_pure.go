// Package ui — pure (non-wasm) helpers for the Select component.
// This file has no build tag so it compiles on all platforms and can be
// unit-tested with plain "go test ./internal/ui/..." on native Go.

package ui

// SelectOption is a single entry in a SelectInput or OptionsFrom list.
type SelectOption struct {
	// Value is the machine-readable option value submitted by the select element.
	Value string
	// Label is the human-readable text shown in the dropdown.
	Label string
}

// OptionsFrom converts a typed slice into []SelectOption by applying a value
// extractor and a label extractor to each element. The selected parameter is
// matched against Value to indicate the currently-chosen item — callers can
// pass it here or set SelectInputProps.Selected; either approach keeps the
// same semantics.
//
// Pure helper — no syscall/js, fully testable on native Go.
//
//	opts := OptionsFrom(accounts,
//	    func(a Account) string { return a.ID },
//	    func(a Account) string { return a.Name },
//	    currentID)
func OptionsFrom[T any](items []T, value func(T) string, label func(T) string, selected string) []SelectOption {
	out := make([]SelectOption, 0, len(items))
	for _, item := range items {
		out = append(out, SelectOption{Value: value(item), Label: label(item)})
	}
	return out
}
