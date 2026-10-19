// SPDX-License-Identifier: MIT

package app

import "fmt"

type cloudPreferenceStore interface {
	GetSettingKV(string) (string, bool)
	SetSettingKV(string, string) error
	ExportJSONRedacted() ([]byte, error)
}

// cloudPreferenceSnapshot prepares a snapshot without exposing staged settings
// while encryption/IndexedDB are asynchronous. The caller holds the existing
// autosave exclusion throughout preparation and the durable write.
func cloudPreferenceSnapshot(s cloudPreferenceStore, next string) ([]byte, error) {
	old, _ := s.GetSettingKV("cashflux:prefs")
	if err := s.SetSettingKV("cashflux:prefs", next); err != nil {
		return nil, fmt.Errorf("stage cloud preferences: %w", err)
	}
	data, exportErr := s.ExportJSONRedacted()
	if err := s.SetSettingKV("cashflux:prefs", old); err != nil {
		return nil, fmt.Errorf("restore current cloud preferences: %w", err)
	}
	if exportErr != nil {
		return nil, fmt.Errorf("export cloud preferences: %w", exportErr)
	}
	return data, nil
}

// cloudWritePreferenceSnapshot adopts settings only after the durable writer
// succeeds. It deliberately does not publish a notice or start any connection.
func cloudWritePreferenceSnapshot(s cloudPreferenceStore, next string, data []byte, write func([]byte, func(error)), done func(error)) {
	write(data, func(err error) {
		if err != nil {
			done(fmt.Errorf("persist cloud preferences: %w", err))
			return
		}
		if err := s.SetSettingKV("cashflux:prefs", next); err != nil {
			done(fmt.Errorf("adopt cloud preferences: %w", err))
			return
		}
		done(nil)
	})
}
