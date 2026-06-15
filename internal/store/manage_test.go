package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/validate"
)

func TestSettingsAccessor(t *testing.T) {
	s := newStore(t)

	got, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings empty: %v", err)
	}
	if got.BaseCurrency != "" {
		t.Errorf("empty settings base = %q, want empty", got.BaseCurrency)
	}

	want := Settings{BaseCurrency: "EUR", OpenAIModel: "gpt-x", FXRates: map[string]float64{"USD": 0.9}}
	if err := s.PutSettings(want); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	got, err = s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.BaseCurrency != "EUR" || got.OpenAIModel != "gpt-x" || got.FXRates["USD"] != 0.9 {
		t.Errorf("settings round trip = %+v", got)
	}

	// Upsert replaces.
	if err := s.PutSettings(Settings{BaseCurrency: "GBP"}); err != nil {
		t.Fatalf("PutSettings 2: %v", err)
	}
	got, _ = s.GetSettings()
	if got.BaseCurrency != "GBP" || got.OpenAIModel != "" {
		t.Errorf("upsert settings = %+v", got)
	}
}

func TestWipe(t *testing.T) {
	s := newStore(t)
	if err := s.Load(SampleDataset()); err != nil {
		t.Fatalf("Load sample: %v", err)
	}
	if err := s.Wipe(); err != nil {
		t.Fatalf("Wipe: %v", err)
	}
	got, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(got.Accounts) != 0 || len(got.Transactions) != 0 || len(got.Members) != 0 {
		t.Errorf("store not empty after wipe: %d accounts, %d txns, %d members",
			len(got.Accounts), len(got.Transactions), len(got.Members))
	}
	if got.Settings.BaseCurrency != "" {
		t.Errorf("settings not cleared: %q", got.Settings.BaseCurrency)
	}
}

func TestSampleDatasetIsValid(t *testing.T) {
	ds := SampleDataset()
	if len(ds.Accounts) == 0 || len(ds.Transactions) == 0 {
		t.Fatal("sample dataset should be populated")
	}
	for _, m := range ds.Members {
		if is := validate.ValidateMember(m); !is.OK() {
			t.Errorf("member %s invalid: %v", m.ID, is)
		}
	}
	for _, a := range ds.Accounts {
		if is := validate.ValidateAccount(a); !is.OK() {
			t.Errorf("account %s invalid: %v", a.ID, is)
		}
	}
	for _, c := range ds.Categories {
		if is := validate.ValidateCategory(c); !is.OK() {
			t.Errorf("category %s invalid: %v", c.ID, is)
		}
	}
	for _, tx := range ds.Transactions {
		if is := validate.ValidateTransaction(tx); !is.OK() {
			t.Errorf("transaction %s invalid: %v", tx.ID, is)
		}
	}
	for _, b := range ds.Budgets {
		if is := validate.ValidateBudget(b); !is.OK() {
			t.Errorf("budget %s invalid: %v", b.ID, is)
		}
	}
	for _, g := range ds.Goals {
		if is := validate.ValidateGoal(g); !is.OK() {
			t.Errorf("goal %s invalid: %v", g.ID, is)
		}
	}
	for _, tk := range ds.Tasks {
		if is := validate.ValidateTask(tk); !is.OK() {
			t.Errorf("task %s invalid: %v", tk.ID, is)
		}
	}
}
