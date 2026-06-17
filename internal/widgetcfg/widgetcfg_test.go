package widgetcfg

import "testing"

func TestFieldStrFallback(t *testing.T) {
	f := Field{Key: "k", Type: Select, Default: "a", Options: []Option{{Value: "a"}, {Value: "b"}}}
	if got := f.Str(Config{}); got != "a" {
		t.Errorf("missing → %q, want default a", got)
	}
	if got := f.Str(Config{"k": "b"}); got != "b" {
		t.Errorf("valid → %q, want b", got)
	}
	if got := f.Str(Config{"k": "zzz"}); got != "a" {
		t.Errorf("invalid select → %q, want default a", got)
	}
}

func TestFieldBool(t *testing.T) {
	f := Field{Key: "on", Type: Toggle, Default: "true"}
	if !f.Bool(Config{}) {
		t.Error("missing should use default true")
	}
	if f.Bool(Config{"on": "false"}) {
		t.Error("explicit false should be false")
	}
	if !f.Bool(Config{"on": "true"}) {
		t.Error("explicit true should be true")
	}
}

func TestFieldIntDefaultAndClamp(t *testing.T) {
	f := Field{Key: "target", Type: Number, Default: "20", Min: 0, Max: 100}
	cases := []struct {
		in   Config
		want int
	}{
		{Config{}, 20}, // default
		{Config{"target": "55"}, 55},
		{Config{"target": "-5"}, 0},    // clamp low
		{Config{"target": "999"}, 100}, // clamp high
		{Config{"target": "abc"}, 20},  // parse error → default
	}
	for _, c := range cases {
		if got := f.Int(c.in); got != c.want {
			t.Errorf("Int(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestIntUnboundedWhenNoMax(t *testing.T) {
	f := Field{Key: "n", Type: Number, Default: "0"} // Max<=Min → unbounded
	if got := f.Int(Config{"n": "100000"}); got != 100000 {
		t.Errorf("unbounded Int = %d, want 100000", got)
	}
}

func TestSavingsSchemaRegistered(t *testing.T) {
	s, ok := SchemaFor("savings")
	if !ok {
		t.Fatal("savings schema not registered")
	}
	if _, ok := s.FieldByKey("target"); !ok {
		t.Error("savings missing target field")
	}
	if !Has("savings") {
		t.Error("Has(savings) = false")
	}
	if Has("nope") {
		t.Error("Has(nope) = true")
	}
}

func TestTodoSchemaRegistered(t *testing.T) {
	s, ok := SchemaFor("todo")
	if !ok {
		t.Fatal("todo schema not registered")
	}
	f, ok := s.FieldByKey("count")
	if !ok {
		t.Fatal("todo missing count field")
	}
	// Default 3, clamped to [1, 10].
	if got := f.Int(Config{}); got != 3 {
		t.Errorf("default count = %d, want 3", got)
	}
	if got := f.Int(Config{"count": "99"}); got != 10 {
		t.Errorf("count clamp high = %d, want 10", got)
	}
	if got := f.Int(Config{"count": "0"}); got != 1 {
		t.Errorf("count clamp low = %d, want 1", got)
	}
}

func TestAccountsSchemaRegistered(t *testing.T) {
	s, ok := SchemaFor("accounts")
	if !ok {
		t.Fatal("accounts schema not registered")
	}
	cnt, ok := s.FieldByKey("count")
	if !ok {
		t.Fatal("accounts missing count field")
	}
	if got := cnt.Int(Config{}); got != 6 {
		t.Errorf("default count = %d, want 6", got)
	}
	if got := cnt.Int(Config{"count": "99"}); got != 12 {
		t.Errorf("count clamp high = %d, want 12", got)
	}
	cl, ok := s.FieldByKey("cleared")
	if !ok {
		t.Fatal("accounts missing cleared field")
	}
	if cl.Bool(Config{}) {
		t.Error("cleared default should be false")
	}
	if !cl.Bool(Config{"cleared": "true"}) {
		t.Error("cleared explicit true should be true")
	}
}

func TestBudgetsSchemaRegistered(t *testing.T) {
	s, ok := SchemaFor("budgets")
	if !ok {
		t.Fatal("budgets schema not registered")
	}
	cnt, ok := s.FieldByKey("count")
	if !ok {
		t.Fatal("budgets missing count field")
	}
	if got := cnt.Int(Config{}); got != 6 {
		t.Errorf("default count = %d, want 6", got)
	}
	if got := cnt.Int(Config{"count": "1"}); got != 3 {
		t.Errorf("count clamp low = %d, want 3", got)
	}
	risk, ok := s.FieldByKey("atRisk")
	if !ok {
		t.Fatal("budgets missing atRisk field")
	}
	if risk.Bool(Config{}) {
		t.Error("atRisk default should be false")
	}
	if !risk.Bool(Config{"atRisk": "true"}) {
		t.Error("atRisk explicit true should be true")
	}
}

func TestSchemaFieldByKey(t *testing.T) {
	s := Schema{Fields: []Field{{Key: "a"}, {Key: "b"}}}
	if _, ok := s.FieldByKey("b"); !ok {
		t.Error("FieldByKey(b) not found")
	}
	if _, ok := s.FieldByKey("z"); ok {
		t.Error("FieldByKey(z) should be missing")
	}
}
