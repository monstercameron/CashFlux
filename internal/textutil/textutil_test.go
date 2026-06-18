package textutil

import (
	"reflect"
	"testing"
)

func TestCommaFields(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", nil},
		{"only separators/spaces", " , , ", nil},
		{"trims and drops empties", " a, ,b ,, c ", []string{"a", "b", "c"}},
		{"single value", "solo", []string{"solo"}},
		{"preserves order and inner spaces", "two words, second", []string{"two words", "second"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommaFields(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("CommaFields(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
