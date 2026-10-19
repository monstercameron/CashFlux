// SPDX-License-Identifier: MIT

package i18n

var r10wKeys = Catalog{
	"r10w.choose":           "Choose…",
	"r10w.chooseCategory":   "Choose a category",
	"r10w.categoryFiled":    "Filed under %s",
	"r10w.suggestionInline": "%s?",
}

func init() {
	for key, value := range r10wKeys {
		english[key] = value
	}
}
