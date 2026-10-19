// SPDX-License-Identifier: MIT

package i18n

// imcKeys contains plain-English copy for structured Smart+ import failures.
var imcKeys = Catalog{
	"imc.aiInvalidReply":            "The AI response could not be checked safely. Please try again or review the rows manually.",
	"imc.aiNoValidRows":             "No receipt items had a valid amount in this transaction's currency. You can split it manually.",
	"imc.smartPlusOpenAIDisclosure": "Smart+ sends the receipt image to OpenAI to read its line items. Nothing is added until you review and save the split.",
	"imc.smartPlusServerDisclosure": "Smart+ sends the receipt image through your connected server for AI analysis. Nothing is added until you review and save the split.",
}

func init() {
	for key, value := range imcKeys {
		english[key] = value
	}
}
