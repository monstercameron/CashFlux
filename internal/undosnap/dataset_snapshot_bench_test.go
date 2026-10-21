// SPDX-License-Identifier: MIT

package undosnap_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

func BenchmarkSnapshot3000Transactions(b *testing.B) {
	ds := store.Dataset{Transactions: make([]domain.Transaction, 3000), Settings: store.Settings{BaseCurrency: "USD", OpenAIKey: "device-secret"}}
	for i := range ds.Transactions {
		ds.Transactions[i] = domain.Transaction{ID: fmt.Sprintf("txn-%04d", i), Amount: money.New(int64(i*17), "USD"), Desc: "benchmark row"}
	}

	redacted := ds
	redacted.Settings.OpenAIKey = ""
	exported, err := store.Export(redacted)
	if err != nil {
		b.Fatal(err)
	}
	want, err := undosnap.ToSnapshot(exported)
	if err != nil {
		b.Fatal(err)
	}
	got, err := undosnap.ToSnapshotDataset(ds)
	if err != nil {
		b.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		b.Fatal("typed snapshot differs from export snapshot")
	}

	b.Run("export-then-parse", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := store.Export(redacted)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := undosnap.ToSnapshot(data); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("typed-direct", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := undosnap.ToSnapshotDataset(ds); err != nil {
				b.Fatal(err)
			}
		}
	})
}
