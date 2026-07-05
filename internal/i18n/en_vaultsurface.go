// SPDX-License-Identifier: MIT

package i18n

// vaultSurfaceKeys holds the English strings for the redesigned /artifacts
// file-vault surface: the hero (storage footprint + figure chips + the
// plain-English takeaway) and the vault sections. Merged via init so this
// file does not touch en.go.
var vaultSurfaceKeys = Catalog{
	"artifacts.heroTitle":   "Your files",
	"artifacts.heroLabel":   "On this device",
	"artifacts.fileWordOne": "1 file",
	"artifacts.fileWordN":   "%d files",
	"artifacts.eyebrowTail": "receipts and datasets, stored locally",
	"artifacts.chipImages":  "Images",
	"artifacts.chipCSV":     "Datasets",
	"artifacts.chipAttach":  "Attached to transactions",
	"artifacts.chipPages":   "Used by pages",
	"art.takeEmpty":         "Nothing stored yet — upload a receipt image or import a CSV dataset below.",
	"art.takeStored":        "%s and %s take %s of this device's storage.",
	"art.takeNearLimit":     "You're close to the practical limit — consider exporting a backup and clearing old receipts.",
	"art.imageWordOne":      "1 image",
	"art.imageWordN":        "%d images",
	"art.csvWordOne":        "1 dataset",
	"art.csvWordN":          "%d datasets",
	"artifacts.addTitle":    "Add to the vault",
	"artifacts.vaultTitle":  "In the vault",
	"artifacts.menuAria":    "File actions",
}

func init() {
	for k, v := range vaultSurfaceKeys {
		english[k] = v
	}
}
