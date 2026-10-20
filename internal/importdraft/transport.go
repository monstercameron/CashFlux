// SPDX-License-Identifier: MIT

package importdraft

import "strings"

// Transport chooses an available extraction route. The backend supports text
// chat and structured vision; PDF files and category suggestions need a personal
// key. Text replies are strictly decoded before becoming draft rows.
func Transport(source, personalKey string, backend bool) string {
	if (source == "receipt" || source == "stmt") && backend {
		return "proxy"
	}
	if strings.TrimSpace(personalKey) != "" {
		return "direct"
	}
	return "unavailable"
}
