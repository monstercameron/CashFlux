package insights

import (
	"reflect"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name   string
		series []CategorySeries
		opts   Options
		want   []Anomaly
	}{
		{
			name: "spike up flagged",
			// baseline = mean(100,100,100) = 100, current 200 → +100%
			series: []CategorySeries{{Category: "Dining", Spend: []int64{10000, 10000, 10000, 20000}}},
			opts:   DefaultOptions(),
			want: []Anomaly{{
				Category: "Dining", Current: 20000, Baseline: 10000, Delta: 10000, PctChange: 100, Direction: Up,
			}},
		},
		{
			name:   "drop down flagged",
			series: []CategorySeries{{Category: "Gas", Spend: []int64{10000, 10000, 2000}}},
			opts:   DefaultOptions(),
			want: []Anomaly{{
				Category: "Gas", Current: 2000, Baseline: 10000, Delta: -8000, PctChange: -80, Direction: Down,
			}},
		},
		{
			name:   "within threshold not flagged",
			series: []CategorySeries{{Category: "Rent", Spend: []int64{10000, 10000, 11000}}}, // +10%
			opts:   DefaultOptions(),
			want:   nil,
		},
		{
			name:   "below noise floor skipped",
			series: []CategorySeries{{Category: "Candy", Spend: []int64{100, 100, 900}}}, // baseline 100 < 1000
			opts:   DefaultOptions(),
			want:   nil,
		},
		{
			name:   "too few periods skipped",
			series: []CategorySeries{{Category: "New", Spend: []int64{10000, 50000}}}, // only 1 baseline period, MinPeriods 2
			opts:   DefaultOptions(),
			want:   nil,
		},
		{
			name:   "zero baseline skipped (no meaningful percent)",
			series: []CategorySeries{{Category: "Fresh", Spend: []int64{0, 0, 5000}}},
			opts:   Options{MinPeriods: 2, MinBaseline: 0, ThresholdPct: 50},
			want:   nil,
		},
		{
			name: "sorted by absolute delta, then category",
			series: []CategorySeries{
				{Category: "Small", Spend: []int64{10000, 10000, 16000}},  // +6000, +60%
				{Category: "Big", Spend: []int64{10000, 10000, 30000}},    // +20000, +200%
				{Category: "AlsoBig", Spend: []int64{40000, 0, 0, 60000}}, // baseline mean(40000,0,0)=13333, +46667
			},
			opts: DefaultOptions(),
			want: []Anomaly{
				{Category: "AlsoBig", Current: 60000, Baseline: 13333, Delta: 46667, PctChange: 350, Direction: Up},
				{Category: "Big", Current: 30000, Baseline: 10000, Delta: 20000, PctChange: 200, Direction: Up},
				{Category: "Small", Current: 16000, Baseline: 10000, Delta: 6000, PctChange: 60, Direction: Up},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.series, tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Detect() = %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestOptionsNormalize(t *testing.T) {
	o := Options{MinPeriods: 0, ThresholdPct: 0, MinBaseline: -5}.normalize()
	if o.MinPeriods != 1 || o.ThresholdPct != 1 || o.MinBaseline != 0 {
		t.Errorf("normalize() = %+v, want MinPeriods 1, ThresholdPct 1, MinBaseline 0", o)
	}
}

func TestDirectionString(t *testing.T) {
	if Up.String() != "up" || Down.String() != "down" {
		t.Errorf("Direction.String() = %q/%q", Up.String(), Down.String())
	}
}

func TestMean(t *testing.T) {
	if got := mean(nil); got != 0 {
		t.Errorf("mean(nil) = %d, want 0", got)
	}
	if got := mean([]int64{10, 20, 31}); got != 20 { // 61/3 = 20 (truncated)
		t.Errorf("mean = %d, want 20", got)
	}
}
