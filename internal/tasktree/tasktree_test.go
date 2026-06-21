package tasktree

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func task(id, parent, title string) domain.Task {
	return domain.Task{ID: id, ParentID: parent, Title: title, Status: domain.StatusOpen, Priority: domain.PriorityMedium}
}

func order(ns []Node) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Task.ID
	}
	return out
}

func TestFlattenNestsDepthFirst(t *testing.T) {
	tasks := []domain.Task{
		task("a", "", "Apple"),
		task("a1", "a", "Apple child 1"),
		task("a2", "a", "Apple child 2"),
		task("a1x", "a1", "Apple grandchild"),
		task("b", "", "Banana"),
	}
	got := Flatten(tasks)
	// Roots ordered A,B (title); A's subtree depth-first before B.
	want := []string{"a", "a1", "a1x", "a2", "b"}
	if !reflect.DeepEqual(order(got), want) {
		t.Fatalf("order = %v, want %v", order(got), want)
	}
	depth := map[string]int{}
	for _, n := range got {
		depth[n.Task.ID] = n.Depth
	}
	for id, d := range map[string]int{"a": 0, "a1": 1, "a1x": 2, "a2": 1, "b": 0} {
		if depth[id] != d {
			t.Fatalf("%s depth = %d, want %d", id, depth[id], d)
		}
	}
}

func TestFlattenOrphanBecomesRoot(t *testing.T) {
	// Parent "gone" is not in the set → its child surfaces as a root (e.g. the
	// parent was hidden by a done-filter).
	got := Flatten([]domain.Task{task("c", "gone", "Orphan")})
	if order(got)[0] != "c" || got[0].Depth != 0 {
		t.Fatalf("orphan should be a depth-0 root, got %+v", got)
	}
}

func TestDescendants(t *testing.T) {
	tasks := []domain.Task{
		task("a", "", ""), task("a1", "a", ""), task("a2", "a", ""), task("a1x", "a1", ""), task("b", "", ""),
	}
	got := Descendants(tasks, "a")
	set := map[string]bool{}
	for _, id := range got {
		set[id] = true
	}
	for _, id := range []string{"a1", "a2", "a1x"} {
		if !set[id] {
			t.Fatalf("Descendants(a) missing %s: %v", id, got)
		}
	}
	if set["a"] || set["b"] {
		t.Fatalf("Descendants(a) should not include a or b: %v", got)
	}
}

func TestFlattenCycleSafe(t *testing.T) {
	// a→b→a cycle must not infinite-loop; each emitted once.
	tasks := []domain.Task{task("a", "b", ""), task("b", "a", "")}
	got := Flatten(tasks)
	if len(got) != 2 {
		t.Fatalf("cycle should emit each task once, got %d", len(got))
	}
}
