// SPDX-License-Identifier: MIT

package importdraft

import "strings"

// Transport chooses an available schema-capable route. The current backend
// supports structured vision; other sources need the personal-key transport.
func Transport(source, personalKey string, backend bool) string {
	if source == "receipt" && backend {
		return "proxy"
	}
	if strings.TrimSpace(personalKey) != "" {
		return "direct"
	}
	return "unavailable"
}
