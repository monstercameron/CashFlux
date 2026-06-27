// SPDX-License-Identifier: MIT

package i18n

// smartStripKeys are the inline Smart-strip disclosure strings (R38), kept
// separate from en.go (concurrent WIP) like the other feature key files.
var smartStripKeys = Catalog{
	"smart.stripMore": "Show %d more",
	"smart.stripLess": "Show less",
}

func init() {
	for k, v := range smartStripKeys {
		english[k] = v
	}
}
