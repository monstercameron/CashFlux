package forecast

import "testing"

func eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMonthlyNet(t *testing.T) {
	got := MonthlyNet([]Recurring{{Monthly: 5000}, {Monthly: -2000}, {Monthly: -500}})
	if got != 2500 {
		t.Errorf("MonthlyNet = %d, want 2500", got)
	}
}

func TestProjectRecurring(t *testing.T) {
	rec := []Recurring{{Label: "salary", Monthly: 5000}, {Label: "rent", Monthly: -2000}}
	got := Project(100000, rec, nil, 3)
	want := []int64{103000, 106000, 109000}
	if !eq(got, want) {
		t.Errorf("Project = %v, want %v", got, want)
	}
}

func TestProjectOneTime(t *testing.T) {
	rec := []Recurring{{Monthly: 3000}}
	one := []OneTime{{Label: "bonus", Month: 2, Amount: 10000}}
	got := Project(100000, rec, one, 3)
	want := []int64{103000, 116000, 119000}
	if !eq(got, want) {
		t.Errorf("Project with one-time = %v, want %v", got, want)
	}
}

func TestProjectFlatAndEmpty(t *testing.T) {
	if got := Project(500, nil, nil, 2); !eq(got, []int64{500, 500}) {
		t.Errorf("flat projection = %v, want [500 500]", got)
	}
	if got := Project(500, nil, nil, 0); got != nil {
		t.Errorf("zero horizon = %v, want nil", got)
	}
}
