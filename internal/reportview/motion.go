// SPDX-License-Identifier: MIT

package reportview

import "time"

// InProgressMonth returns the monthly row containing now under the report's
// existing (start, end] label convention, or -1 outside the supplied bounds.
// Exact instants and zones are retained; no calendar rounding is performed.
func InProgressMonth(bounds []time.Time, now time.Time) int {
	for i := 0; i+1 < len(bounds); i++ {
		if bounds[i].Before(now) && !bounds[i+1].Before(now) {
			return i
		}
	}
	return -1
}
