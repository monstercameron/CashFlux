// Package pagination is the pure window math for paged lists: total pages, page
// clamping, slice bounds, and the human "from-to of total" label. Keeping it
// platform-free means the transactions table (and any other paged view) computes
// its window here, unit-tested, rather than in view code. A size of 0 (or less)
// means "show all" — one page containing everything.
package pagination

// AllSize is the page size that means "show everything on one page".
const AllSize = 0

// TotalPages returns how many pages of the given size cover total items (always
// at least 1). Size <= 0 ("all") is a single page.
func TotalPages(total, size int) int {
	if total <= 0 {
		return 1
	}
	if size <= 0 {
		return 1
	}
	return (total + size - 1) / size
}

// Clamp constrains a 1-based page number to [1, TotalPages]. Use it after the
// filter or page size changes so a now-out-of-range page snaps back into bounds.
func Clamp(page, total, size int) int {
	if page < 1 {
		return 1
	}
	if tp := TotalPages(total, size); page > tp {
		return tp
	}
	return page
}

// Bounds returns the [start, end) slice indices for a 1-based page over total
// items. The page is clamped first. Size <= 0 ("all") returns the whole range.
func Bounds(page, total, size int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if size <= 0 {
		return 0, total
	}
	page = Clamp(page, total, size)
	start = (page - 1) * size
	end = start + size
	if end > total {
		end = total
	}
	return start, end
}

// Slice returns the items belonging to a 1-based page (a sub-slice of items).
func Slice[T any](items []T, page, size int) []T {
	start, end := Bounds(page, len(items), size)
	return items[start:end]
}

// Window returns the human 1-based position of a page: the first and last item
// numbers it shows (e.g. 1 and 50 for "1-50 of 312"). Both are 0 when empty.
func Window(page, total, size int) (from, to int) {
	start, end := Bounds(page, total, size)
	if total <= 0 || end == 0 {
		return 0, 0
	}
	return start + 1, end
}
