// SPDX-License-Identifier: MIT

package i18n

// bc6n1Keys contains English copy for the price-creep accept flow and its tasks.
var bc6n1Keys = Catalog{
	"pricecreep.taskTitle":         "Review the price of %s",
	"pricecreep.taskNotes":         "%s has gone up from its expected price. Cancel it, ask about a lower price, or accept the increase.",
	"pricecreep.taskResolved":      "Marked the price review complete.",
	"pricecreep.acceptedAndRaised": "Updated %s to %s and raised its budget.",
	"pricecreep.saveFailed":        "CashFlux couldn't save the price change. Your recurring price and budget were left unchanged.",
}

func init() {
	for key, value := range bc6n1Keys {
		english[key] = value
	}
}
