package dashlayout

// ClampSpan keeps a grid span within [1, max]: values below 1 collapse to 1 and
// above max collapse to max. Used by the keyboard resize path to bound a span
// after an arrow-key adjustment.
func ClampSpan(v, max int) int {
	if v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
}

// CycleSpan advances a grid span on a resize-handle click. When shrink is set it
// subtracts one (clamped at 1) for a direct shrink; otherwise it grows by one and
// wraps back to 1 once past max — so a plain click grows (the wrap being the
// no-modifier way to shrink), and Shift+click shrinks one step directly.
func CycleSpan(cur, max int, shrink bool) int {
	if shrink {
		if cur <= 1 {
			return 1
		}
		return cur - 1
	}
	if cur+1 > max {
		return 1
	}
	return cur + 1
}
