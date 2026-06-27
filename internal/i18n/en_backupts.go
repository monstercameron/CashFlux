// SPDX-License-Identifier: MIT

package i18n

// backupTSKeys holds the English strings for the C299 "last backed up"
// timestamp line in Settings → Data. Kept in its own file so it does not
// land in the concurrent-WIP en.go.
var backupTSKeys = Catalog{
	// Settings → Data: last-backup timestamp line (C299).
	"settings.lastBackup":      "Last backed up %s.",
	"settings.lastBackupNever": "Not backed up yet — export a backup to keep your data safe.",
}

func init() {
	for k, v := range backupTSKeys {
		english[k] = v
	}
}
