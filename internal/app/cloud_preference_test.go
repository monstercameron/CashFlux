// SPDX-License-Identifier: MIT

package app

import (
	"errors"
	"testing"
)

type cloudPreferenceMemory struct {
	value             string
	exportErr, setErr error
}

func (s *cloudPreferenceMemory) GetSettingKV(string) (string, bool) { return s.value, true }
func (s *cloudPreferenceMemory) SetSettingKV(_, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.value = value
	return nil
}
func (s *cloudPreferenceMemory) ExportJSONRedacted() ([]byte, error) {
	return []byte(s.value), s.exportErr
}

func TestCloudPreferenceSnapshotRestoresCurrentSettingsBeforeAsyncWork(t *testing.T) {
	for _, fails := range []bool{false, true} {
		s := &cloudPreferenceMemory{value: "off"}
		if fails {
			s.exportErr = errors.New("export failed")
		}
		data, err := cloudPreferenceSnapshot(s, "on")
		if s.value != "off" {
			t.Fatal("staged sync enable escaped snapshot preparation")
		}
		if (err != nil) != fails {
			t.Fatalf("snapshot error=%v, want failure=%t", err, fails)
		}
		if !fails && string(data) != "on" {
			t.Fatal("snapshot lost the requested preference")
		}
	}
}

func TestCloudPreferenceAdoptsOnlyAfterDurableWrite(t *testing.T) {
	for _, fails := range []bool{false, true} {
		s := &cloudPreferenceMemory{value: "off"}
		var complete func(error)
		calls := 0
		var result error
		cloudWritePreferenceSnapshot(s, "on", []byte("on"), func(data []byte, done func(error)) {
			if string(data) != "on" {
				t.Fatal("writer received another snapshot")
			}
			complete = done
		}, func(err error) { calls++; result = err })
		if calls != 0 || s.value != "off" {
			t.Fatal("sync enabled before durability acknowledgment")
		}
		if fails {
			complete(errors.New("disk full"))
		} else {
			complete(nil)
		}
		if calls != 1 || (result != nil) != fails {
			t.Fatalf("completion = %d, %v", calls, result)
		}
		if fails && s.value != "off" {
			t.Fatal("failed durable write enabled sync")
		}
		if !fails && s.value != "on" {
			t.Fatal("durable settings were not adopted")
		}
	}
}

func TestCloudPreferenceSnapshotRefusesFailedStoreWrite(t *testing.T) {
	s := &cloudPreferenceMemory{value: "off", setErr: errors.New("read only")}
	if _, err := cloudPreferenceSnapshot(s, "on"); err == nil || s.value != "off" {
		t.Fatal("failed settings write was accepted")
	}
}
