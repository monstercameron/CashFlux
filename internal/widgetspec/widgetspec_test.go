package widgetspec

import "testing"

func TestEvalKPI(t *testing.T) {
	vars := map[string]float64{"income": 5000, "expense": 2000}
	v, err := EvalKPI("(income - expense) / income * 100", vars)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if v != 60 {
		t.Errorf("savings rate = %v, want 60", v)
	}
	// Boolean coerces to 1/0.
	if v, err := EvalKPI("income > expense", vars); err != nil || v != 1 {
		t.Errorf("bool true = %v, %v, want 1", v, err)
	}
	if v, err := EvalKPI("income < expense", vars); err != nil || v != 0 {
		t.Errorf("bool false = %v, %v, want 0", v, err)
	}
	// Empty + parse errors are reported, not panics.
	if _, err := EvalKPI("", vars); err == nil {
		t.Error("empty expr should error")
	}
	if _, err := EvalKPI("income +", vars); err == nil {
		t.Error("bad expr should error")
	}
	// Unknown variable errors (sandbox doesn't silently zero).
	if _, err := EvalKPI("mystery * 2", vars); err == nil {
		t.Error("unknown var should error")
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		v      float64
		format string
		want   string
	}{
		{60, FormatPercent, "60%"},
		{1234.5, FormatNumber, "1234.5"},
		{42, FormatNumber, "42"},
		{42, FormatCurrency, "42"}, // currency falls back to number here
		{42, "", "42"},
	}
	for _, c := range cases {
		if got := Format(c.v, c.format); got != c.want {
			t.Errorf("Format(%v,%q) = %q, want %q", c.v, c.format, got, c.want)
		}
	}
}

func TestCatalogAndSources(t *testing.T) {
	if len(Catalog()) != 4 {
		t.Errorf("catalog should have 4 Phase-B types, got %d", len(Catalog()))
	}
	for _, d := range Catalog() {
		if !Known(d.Type) {
			t.Errorf("catalog type %q not Known", d.Type)
		}
	}
	if len(ListSources()) != 5 {
		t.Errorf("expected 5 list sources, got %d", len(ListSources()))
	}
	if Known("bogus") {
		t.Error("bogus should not be Known")
	}
	// Catalog is a copy — mutating it doesn't affect the package.
	c := Catalog()
	c[0].Label = "X"
	if Catalog()[0].Label == "X" {
		t.Error("Catalog returned a shared slice")
	}
}
