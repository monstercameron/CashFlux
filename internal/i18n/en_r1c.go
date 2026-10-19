// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r1c.category":       "Category",
		"r1c.person":         "Person",
		"r1c.moreActions":    "More bulk actions",
		"r1c.markReviewed":   "Mark reviewed",
		"r1c.addTagEllipsis": "Add tag…",
		"r1c.addTag":         "Add tag",
		"r1c.tagInput":       "Tag name",
		"r1c.clearSelection": "Clear selection",
		"r1c.reviewCallout":  "%d to review →",
		"r1c.markedReviewed": "Marked %d transactions reviewed.",
		"r1c.tagged":         "Added “%s” to %d transactions.",
	} {
		english[key] = value
	}
}
