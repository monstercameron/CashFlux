package chart

import "testing"

func TestPointsEvenX(t *testing.T) {
	pts := Points([]float64{1, 2, 3}, 100, 50, 0)
	wantX := []float64{0, 50, 100}
	for i, p := range pts {
		if p.X != wantX[i] {
			t.Errorf("point %d X = %g, want %g", i, p.X, wantX[i])
		}
	}
	// Min (1) maps near the bottom (y=h), max (3) near the top (y=0) with pad 0.
	if pts[0].Y != 50 {
		t.Errorf("min Y = %g, want 50", pts[0].Y)
	}
	if pts[2].Y != 0 {
		t.Errorf("max Y = %g, want 0", pts[2].Y)
	}
	if pts[1].Y != 25 {
		t.Errorf("mid Y = %g, want 25", pts[1].Y)
	}
}

func TestPointsFlatSeriesCentered(t *testing.T) {
	pts := Points([]float64{5, 5, 5}, 100, 40, 0)
	for i, p := range pts {
		if p.Y != 20 {
			t.Errorf("flat point %d Y = %g, want 20 (centered)", i, p.Y)
		}
	}
}

func TestPointsSingle(t *testing.T) {
	pts := Points([]float64{7}, 80, 40, 0)
	if len(pts) != 1 || pts[0].X != 40 || pts[0].Y != 20 {
		t.Errorf("single point = %+v, want {40 20}", pts)
	}
}

func TestPointsEmpty(t *testing.T) {
	if Points(nil, 10, 10, 0) != nil {
		t.Error("Points(nil) should be nil")
	}
	if LinePath(nil) != "" || AreaPath(nil, 10) != "" {
		t.Error("empty paths should be empty strings")
	}
}

func TestLineAndAreaPath(t *testing.T) {
	pts := Points([]float64{1, 2, 3}, 100, 50, 0)
	gotLine := LinePath(pts)
	wantLine := "M0.00,50.00 L50.00,25.00 L100.00,0.00"
	if gotLine != wantLine {
		t.Errorf("LinePath = %q, want %q", gotLine, wantLine)
	}
	gotArea := AreaPath(pts, 50)
	wantArea := wantLine + " L100.00,50.00 L0.00,50.00 Z"
	if gotArea != wantArea {
		t.Errorf("AreaPath = %q, want %q", gotArea, wantArea)
	}
}
