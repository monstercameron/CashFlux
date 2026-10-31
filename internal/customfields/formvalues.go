// SPDX-License-Identifier: MIT

package customfields

import (
	"fmt"
	"strconv"
)

// FormValuesToStrings converts a typed custom-value map to the string values
// edited by custom-field forms. Booleans use "true"/"false", float64 values
// use a compact decimal representation, strings pass through, and other values
// use their default formatted representation.
func FormValuesToStrings(values map[string]any) map[string]string {
	strings := make(map[string]string, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case bool:
			if typed {
				strings[key] = "true"
			} else {
				strings[key] = "false"
			}
		case float64:
			strings[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		case string:
			strings[key] = typed
		default:
			strings[key] = fmt.Sprintf("%v", typed)
		}
	}
	return strings
}
