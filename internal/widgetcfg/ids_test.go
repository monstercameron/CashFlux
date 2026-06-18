package widgetcfg

import "testing"

// TestIDs covers IDs(): it returns the registered widget ids, sorted, and every
// one is a real schema (consistent with Has/SchemaFor).
func TestIDs(t *testing.T) {
	ids := IDs()
	if len(ids) == 0 {
		t.Fatal("IDs() should not be empty — widgets are registered at init")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] > ids[i] {
			t.Errorf("IDs() is not sorted: %v", ids)
			break
		}
	}
	for _, id := range ids {
		if !Has(id) {
			t.Errorf("IDs() returned %q but Has(%q) is false", id, id)
		}
		if _, ok := SchemaFor(id); !ok {
			t.Errorf("IDs() returned %q but SchemaFor has no schema for it", id)
		}
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for _, w := range []string{"savings", "todo", "accounts", "budgets", "goals"} {
		if !got[w] {
			t.Errorf("IDs() is missing known widget %q; got %v", w, ids)
		}
	}
}
