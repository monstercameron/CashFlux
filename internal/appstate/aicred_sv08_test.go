// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestDatasetJSONExportsAlwaysOmitOpenAIKey(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ds := store.EmptyDataset()
	ds.Settings.OpenAIKey = "sk-device-secret"
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	for name, export := range map[string]func() ([]byte, error){
		"manual":              a.ExportJSON,
		"redacted":            a.ExportJSONRedacted,
		"with blobs":          a.ExportJSONWithBlobs,
		"redacted with blobs": a.ExportJSONRedactedWithBlobs,
	} {
		t.Run(name, func(t *testing.T) {
			data, err := export()
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Settings struct {
					OpenAIKey string `json:"openAiKey"`
				} `json:"settings"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Settings.OpenAIKey != "" {
				t.Fatalf("export contains key %q", got.Settings.OpenAIKey)
			}
		})
	}
}

func TestSQLiteExportOmitsDeviceOpenAIKey(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ds := store.EmptyDataset()
	ds.Settings.OpenAIKey = "sk-device-secret"
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	data, err := a.ExportSQLite()
	if err != nil {
		t.Fatal(err)
	}
	exported, err := store.ImportSQLiteFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Settings.OpenAIKey != "" {
		t.Fatal("SQLite export contains a device credential")
	}
}

func TestCompleteArtifactSnapshotOmitsLegacyDeviceKey(t *testing.T) {
	for _, withBlob := range []bool{false, true} {
		name := "no external file"
		if withBlob {
			name = "external file"
		}
		t.Run(name, func(t *testing.T) {
			a, err := New(nil, false)
			if err != nil {
				t.Fatal(err)
			}
			ds := store.EmptyDataset()
			ds.Settings.OpenAIKey = "sk-legacy-device-secret"
			if withBlob {
				blobs := newMemBlobStore()
				if err := blobs.Put("legacy-pdf", "application/pdf", []byte("PDF bytes")); err != nil {
					t.Fatal(err)
				}
				a.SetBlobStore(blobs)
				ds.Artifacts = []domain.Artifact{{ID: "legacy-pdf", Kind: artifacts.KindPDF, MIME: "application/pdf", Size: len("PDF bytes")}}
			}
			committed, err := store.Export(ds)
			if err != nil {
				t.Fatal(err)
			}
			complete, err := a.CompleteArtifactSnapshot(committed)
			if err != nil {
				t.Fatal(err)
			}
			exported, err := store.Import(complete)
			if err != nil {
				t.Fatal(err)
			}
			if exported.Settings.OpenAIKey != "" {
				t.Fatal("completed snapshot contains a device credential")
			}
			if withBlob && string(exported.Artifacts[0].Bytes) != "PDF bytes" {
				t.Fatal("completed snapshot lost file bytes")
			}
		})
	}
}

func TestImportJSONKeepsCurrentDeviceKey(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	current := store.EmptyDataset()
	current.Settings.OpenAIKey = "sk-current-device"
	if err := a.Store().Load(current); err != nil {
		t.Fatal(err)
	}
	remote := store.EmptyDataset()
	data, err := store.Export(remote)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ImportJSON(data); err != nil {
		t.Fatal(err)
	}
	if got := a.Settings().OpenAIKey; got != "sk-current-device" {
		t.Fatalf("key after keyless import = %q", got)
	}
	remote.Settings.OpenAIKey = "sk-other-device"
	data, err = store.Export(remote)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ImportJSON(data); err != nil {
		t.Fatal(err)
	}
	if got := a.Settings().OpenAIKey; got != "sk-current-device" {
		t.Fatalf("key after keyed import = %q", got)
	}
}
