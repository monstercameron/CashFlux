//go:build ignore

package main

import (
	"fmt"
	"time"
)

func main() {
	epochMonday := time.Date(2006, time.January, 2, 0, 0, 0, 0, time.UTC)
	weekStart := time.Sunday
	offset := int(weekStart) - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	anchor := epochMonday.AddDate(0, 0, offset)
	fmt.Println("anchor for Sunday start:", anchor.Format("2006-01-02"), anchor.Weekday())

	ref := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	ancDay := anchor.In(time.UTC)
	diff := int(ref.Sub(ancDay).Hours()) / 24
	fmt.Println("diff:", diff)
	fortnight := (diff / 14) * 14
	fmt.Println("fortnight offset:", fortnight)
	start := ancDay.AddDate(0, 0, fortnight)
	end := start.AddDate(0, 0, 14)
	fmt.Printf("biweekly range: %s..%s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
}
