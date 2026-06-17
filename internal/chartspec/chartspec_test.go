package chartspec

import (
	"errors"
	"testing"
)

func TestKindValid(t *testing.T) {
	for _, k := range []Kind{Line, Area, Bar, Donut} {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if Kind("pie").Valid() {
		t.Error("unknown kind reported valid")
	}
}

func lineSpec(pts ...Point) Spec {
	return Spec{Kind: Line, Series: []Series{{Name: "s", Points: pts}}}
}

func TestValidateOK(t *testing.T) {
	s := lineSpec(Point{X: 0, Y: 1}, Point{X: 1, Y: 2})
	if err := s.Validate(); err != nil {
		t.Errorf("valid spec rejected: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		spec Spec
		want error
	}{
		{"unknown kind", Spec{Kind: "pie", Series: []Series{{Points: []Point{{}}}}}, ErrUnknownKind},
		{"no series", Spec{Kind: Line}, ErrNoSeries},
		{"empty series", Spec{Kind: Line, Series: []Series{{Name: "x"}}}, ErrEmptySeries},
		{"multi-series donut", Spec{Kind: Donut, Series: []Series{
			{Points: []Point{{}}}, {Points: []Point{{}}},
		}}, ErrDonutSingle},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.spec.Validate()
			if !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestDonutSingleSeriesOK(t *testing.T) {
	s := Spec{Kind: Donut, Series: []Series{{Points: []Point{{Y: 1}, {Y: 2}}}}}
	if err := s.Validate(); err != nil {
		t.Errorf("single-series donut rejected: %v", err)
	}
}

func TestExtent(t *testing.T) {
	s := Spec{Kind: Line, Series: []Series{
		{Points: []Point{{X: -2, Y: 5}, {X: 3, Y: -1}}},
		{Points: []Point{{X: 1, Y: 9}, {X: 10, Y: 0}}},
	}}
	minX, maxX, minY, maxY, ok := s.Extent()
	if !ok {
		t.Fatal("Extent ok=false for a spec with points")
	}
	if minX != -2 || maxX != 10 || minY != -1 || maxY != 9 {
		t.Errorf("Extent = (%g,%g,%g,%g), want (-2,10,-1,9)", minX, maxX, minY, maxY)
	}
}

func TestExtentEmpty(t *testing.T) {
	_, _, _, _, ok := Spec{Kind: Line}.Extent()
	if ok {
		t.Error("Extent ok=true for an empty spec")
	}
}
