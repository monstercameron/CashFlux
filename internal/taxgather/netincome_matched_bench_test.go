// SPDX-License-Identifier: MIT

package taxgather

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkNetIncomeMatched alternates operation order over the same fixture
// within each sample. Custom metrics report each preparation's elapsed time;
// standard ns/op and allocation metrics describe the whole pair. This diagnostic
// complements, and does not replace, the separately captured before/after runs.
func BenchmarkNetIncomeMatched(b *testing.B) {
	for _, count := range []int{10000, 100000} {
		b.Run(fmt.Sprintf("rows_%d", count), func(b *testing.B) {
			in := netIncomeBenchmarkInput(count)
			var referenceTime, selectedTime time.Duration
			var reference, selected NetIncome
			referenceCall := func() {
				start := time.Now()
				var err error
				reference, err = netIncomeGatherReference(in)
				referenceTime += time.Since(start)
				if err != nil {
					b.Fatal(err)
				}
			}
			selectedCall := func() {
				start := time.Now()
				var err error
				selected, err = NetIncomeWithLinks(in)
				selectedTime += time.Since(start)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if i%2 == 0 {
					referenceCall()
					selectedCall()
				} else {
					selectedCall()
					referenceCall()
				}
				if reference != selected {
					b.Fatalf("fixture result changed: %+v vs %+v", reference, selected)
				}
			}
			b.ReportMetric(float64(referenceTime.Nanoseconds())/float64(b.N), "gather-ns/op")
			b.ReportMetric(float64(selectedTime.Nanoseconds())/float64(b.N), "selected-ns/op")
		})
	}
}
