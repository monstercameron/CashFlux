// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestLiabilitySignMigrationAndRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name           string
		opening        int64
		explicit, want domain.LiabilitySign
	}{
		{"zero", 0, "", domain.LiabilityPositiveOwed},
		{"positive", 500, "", domain.LiabilityPositiveOwed},
		{"negative", -500, "", domain.LiabilityNegativeOwed},
		{"explicit credit", -500, domain.LiabilityPositiveOwed, domain.LiabilityPositiveOwed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ds := Dataset{Accounts: []domain.Account{{ID: "card", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(tt.opening, "USD"), LiabilitySign: tt.explicit}},
				// A live balance with the opposite sign must not affect migration.
				Transactions: []domain.Transaction{{ID: "charge", AccountID: "card", Amount: money.New(-2*tt.opening, "USD")}}}
			raw, err := json.Marshal(ds)
			if err != nil {
				t.Fatal(err)
			}
			migrated, err := Import(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := migrated.Accounts[0].LiabilitySign; got != tt.want {
				t.Fatalf("migrated = %q, want %q", got, tt.want)
			}
			st, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			if err := st.Load(ds); err != nil {
				t.Fatal(err)
			}
			if ds.Accounts[0].LiabilitySign != tt.explicit {
				t.Fatal("Load mutated caller")
			}
			stored, _, err := st.GetAccount("card")
			if err != nil {
				t.Fatal(err)
			}
			if stored.LiabilitySign != tt.want {
				t.Fatalf("stored = %q, want %q", stored.LiabilitySign, tt.want)
			}
			snapshot, err := st.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			blob, err := Export(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := Import(blob)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(snapshot.Accounts, imported.Accounts) {
				t.Fatal("export/import changed convention")
			}
			again, err := Export(imported)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(blob, again) {
				t.Fatal("migration was not idempotent")
			}
		})
	}
}

func TestAccountEditRetainsLiabilitySign(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, opening := range []int64{-500, 0, 500} {
		account := domain.Account{ID: "card", Class: domain.ClassLiability, OpeningBalance: money.New(opening, "USD")}
		if _, err := st.DeleteAccount(account.ID); err != nil {
			t.Fatal(err)
		}
		if err := st.PutAccount(account); err != nil {
			t.Fatal(err)
		}
		want := account.WithLiabilitySign().LiabilitySign
		account.OpeningBalance.Amount = -opening
		if err := st.PutAccount(account); err != nil {
			t.Fatal(err)
		}
		got, _, err := st.GetAccount(account.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.LiabilitySign != want {
			t.Fatalf("edit changed %q to %q", want, got.LiabilitySign)
		}
	}
}

func TestUnknownLiabilitySignUsesSafeDefaultOnDatasetLoad(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	account := domain.Account{ID: "card", Class: domain.ClassLiability}
	if err := st.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	account.LiabilitySign = "future_convention"
	ds := Dataset{Accounts: []domain.Account{account}}
	if err := st.Load(ds); err != nil {
		t.Fatalf("Load quarantined an unknown saved convention: %v", err)
	}
	if got, found, err := st.GetAccount("card"); err != nil || !found || got.LiabilitySign != domain.LiabilityPositiveOwed {
		t.Fatalf("Load did not use safe positive-owed default: account=%+v found=%v err=%v", got, found, err)
	}
	account.ID = "new-card"
	if err := st.PutAccount(account); err == nil {
		t.Fatal("PutAccount accepted a newly supplied unknown convention")
	}
	raw, err := json.Marshal(ds)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := Import(raw)
	if err != nil || imported.Accounts[0].LiabilitySign != domain.LiabilityPositiveOwed {
		t.Fatalf("Import did not migrate unknown convention: account=%+v err=%v", imported.Accounts, err)
	}
}

func TestLiabilitySignSQLiteRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	account := domain.Account{ID: "credit", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-5000, "USD"), LiabilitySign: domain.LiabilityPositiveOwed}
	if err := st.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	file, err := st.SerializeSQLite()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := ImportSQLiteFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Accounts) != 1 || !reflect.DeepEqual(ds.Accounts[0], account) {
		t.Fatalf("SQLite round trip = %+v", ds.Accounts)
	}
}
