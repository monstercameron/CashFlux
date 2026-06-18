package domain

import "testing"

func TestAccountByID(t *testing.T) {
	accounts := []Account{
		{ID: "a1", Name: "Checking"},
		{ID: "a2", Name: "Savings"},
	}

	if a, ok := AccountByID(accounts, "a2"); !ok || a.Name != "Savings" {
		t.Errorf("AccountByID(a2) = %+v, ok=%v; want Savings/true", a, ok)
	}
	if a, ok := AccountByID(accounts, "missing"); ok || a.ID != "" {
		t.Errorf("AccountByID(missing) = %+v, ok=%v; want zero/false", a, ok)
	}
	if _, ok := AccountByID(nil, "a1"); ok {
		t.Error("AccountByID(nil, …) should not find anything")
	}
}
