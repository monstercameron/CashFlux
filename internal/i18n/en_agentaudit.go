// SPDX-License-Identifier: MIT

package i18n

// agentAuditKeys holds the English strings for the background auditor's findings
// card (AG6). Merged via init so this file does not touch en.go. audit.total takes
// the total-impact money string and the one-tap-fix count.
var agentAuditKeys = Catalog{
	"audit.title":        "Audit findings",
	"audit.total":        "%s total impact · %d one-tap fixes",
	"audit.empty":        "No active, non-dismissed detector findings. This does not establish that all financial issues are absent.",
	"audit.unavailable":  "Audit unavailable: %s",
	"audit.periodTotals": "Recurring estimates %s/year; one-time estimates %s; %d unclassified amounts; %d follow-ups. Period groups are separate and may overlap; these are not guaranteed savings.",
}

func init() {
	for k, v := range agentAuditKeys {
		english[k] = v
	}
}
