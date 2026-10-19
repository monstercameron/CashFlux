// SPDX-License-Identifier: MIT

package txntable

import "reflect"

// EqualRowValues compares a strongly typed row snapshot while ignoring top-level
// callback fields. Nested data and portal targets retain DeepEqual semantics.
func EqualRowValues[T any](a, b T) bool {
	left, right := reflect.ValueOf(&a).Elem(), reflect.ValueOf(&b).Elem()
	if left.Kind() == reflect.Struct {
		for i := 0; i < left.NumField(); i++ {
			if left.Field(i).Kind() == reflect.Func && left.Field(i).CanSet() {
				left.Field(i).SetZero()
				right.Field(i).SetZero()
			}
		}
	}
	return reflect.DeepEqual(a, b)
}

// RetainRow updates a bounded insertion-order cache, retaining equal row identity.
// The caller owns the cache map and order; neither is shared across scopes.
func RetainRow[T any](rows map[string]*T, order []string, id string, next T, limit int) (*T, map[string]*T, []string) {
	if rows == nil {
		rows = make(map[string]*T)
	}
	if previous, ok := rows[id]; ok && previous != nil && EqualRowValues(*previous, next) {
		return previous, rows, order
	}
	if _, ok := rows[id]; !ok {
		order = append(order, id)
	}
	current := new(T)
	*current = next
	rows[id] = current
	for len(order) > max(0, limit) {
		delete(rows, order[0])
		order = order[1:]
	}
	return current, rows, order
}
