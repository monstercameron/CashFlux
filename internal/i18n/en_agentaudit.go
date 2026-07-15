// SPDX-License-Identifier: MIT

package i18n

// agentAuditKeys holds the English strings for the background auditor's findings
// card (AG6). Merged via init so this file does not touch en.go. audit.total takes
// the total-impact money string and the one-tap-fix count.
var agentAuditKeys = Catalog{
	"audit.title": "Audit findings",
	"audit.total": "%s total impact · %d one-tap fixes",
	"audit.empty": "No findings — nothing is bleeding money right now.",
}

func init() {
	for k, v := range agentAuditKeys {
		english[k] = v
	}
}
