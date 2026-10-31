// SPDX-License-Identifier: MIT

package artifactview

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

// BenchmarkVaultBaseline records the current route's data-dependent work before
// the prepared model exists: reference maps, all-artifact counts, and a redacted
// exact-size store export. Fixture size approximates a receipt-heavy household.
func BenchmarkVaultBaseline(b *testing.B) {
	ds := vaultFixture()
	const blobUsage int64 = 18 << 20
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		refs := map[string]int{}
		for _, txn := range ds.Transactions {
			for _, att := range txn.Attachments {
				refs[att.ArtifactID]++
			}
		}
		pageRefs := map[string]int{}
		for _, page := range ds.CustomPages {
			for _, widget := range page.Widgets {
				if widget.Binding.ArtifactID != "" {
					pageRefs[widget.Binding.ArtifactID]++
				}
			}
		}
		img, pdf, other := 0, 0, 0
		for _, artifact := range ds.Artifacts {
			switch artifact.Kind {
			case "image":
				img++
			case "pdf":
				pdf++
			default:
				other++
			}
		}
		attached, pageBound := 0, 0
		for id, n := range refs {
			if n > 0 && id != "" {
				attached++
			}
		}
		for _, n := range pageRefs {
			if n > 0 {
				pageBound++
			}
		}
		copy := ds
		copy.Settings.OpenAIKey = ""
		data, err := store.Export(copy)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSink = len(data) + attached + pageBound + img + pdf + other + int(blobUsage)
	}
}

var benchmarkSink int

func vaultFixture() store.Dataset {
	const artifactN, txnN, pageN = 240, 5000, 100
	ds := store.Dataset{SchemaVersion: store.SchemaVersion, Settings: store.Settings{BaseCurrency: "USD", OpenAIKey: "must be redacted"}}
	ds.Artifacts = make([]domain.Artifact, artifactN)
	for i := range ds.Artifacts {
		kind := "csv"
		if i%7 == 0 {
			kind = "image"
		} else if i%11 == 0 {
			kind = "pdf"
		}
		ds.Artifacts[i] = domain.Artifact{ID: fmt.Sprintf("artifact-%04d", i), Name: fmt.Sprintf("receipt-%04d", i), Kind: kind, Size: 64 << 10}
	}
	ds.Transactions = make([]domain.Transaction, txnN)
	for i := range ds.Transactions {
		id := fmt.Sprintf("artifact-%04d", i%artifactN)
		ds.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("txn-%06d", i), Attachments: []domain.AttachmentRef{{ArtifactID: id}, {ArtifactID: id}, {ArtifactID: ""}}}
	}
	ds.CustomPages = make([]domain.CustomPage, pageN)
	for i := range ds.CustomPages {
		widgets := make([]domain.PageWidget, 4)
		for j := range widgets {
			id := fmt.Sprintf("artifact-%04d", (i*4+j)%artifactN)
			if (i+j)%9 == 0 {
				id = ""
			}
			widgets[j] = domain.PageWidget{ID: fmt.Sprintf("widget-%d-%d", i, j), Binding: domain.WidgetBinding{ArtifactID: id}}
		}
		ds.CustomPages[i] = domain.CustomPage{ID: fmt.Sprintf("page-%03d", i), Widgets: widgets}
	}
	return ds
}
