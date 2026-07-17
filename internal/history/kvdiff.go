// SPDX-License-Identifier: MIT

package history

import "encoding/json"

// ScalarMapDiffKeys compares two JSON objects (the before/after of a scalar
// map collection such as the dataset's appState KV bucket) and returns the
// top-level keys whose values were added, removed, or changed. ok is false
// when either payload is not a JSON object — callers must then treat the
// change as opaque rather than assuming an empty diff.
func ScalarMapDiffKeys(before, after json.RawMessage) (keys []string, ok bool) {
	var b, a map[string]json.RawMessage
	if err := json.Unmarshal(before, &b); err != nil {
		return nil, false
	}
	if err := json.Unmarshal(after, &a); err != nil {
		return nil, false
	}
	for k, av := range a {
		bv, had := b[k]
		if !had || string(bv) != string(av) {
			keys = append(keys, k)
		}
	}
	for k := range b {
		if _, still := a[k]; !still {
			keys = append(keys, k)
		}
	}
	return keys, true
}
