// SPDX-License-Identifier: MIT

package i18n

// dupPluralKeys holds singular variants for the Duplicates screen (the plural
// strings read "1 possible duplicate entries across 1 groups"). Merged via init
// so this file does not touch en.go.
var dupPluralKeys = Catalog{
	"duplicates.headlineOne":   "1 possible duplicate entry",
	"duplicates.groupCountOne": "1 entry",
	"artifacts.deleteConfirm":  "Delete the file \"%s\"? This can't be undone.",
	"documents.deleteConfirm":  "Remove this import from your history? The transactions it created stay; only the record is removed.",
}

func init() {
	for k, v := range dupPluralKeys {
		english[k] = v
	}
}
