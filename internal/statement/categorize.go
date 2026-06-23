package statement

import "strings"

// Categorizer is a function that assigns a category string to a Row.
// It may return an empty string if it cannot categorize.
type Categorizer func(r Row) string

// Categorize fills the Category field of each row using fn.
// Rows that already have a non-empty Category are left unchanged.
func Categorize(rows []Row, fn Categorizer) []Row {
	for i := range rows {
		if rows[i].Category == "" {
			rows[i].Category = fn(rows[i])
		}
	}
	return rows
}

type kw struct {
	words    []string
	category string
}

var keywordTable = []kw{
	{[]string{"coffee", "starbucks", "dunkin"}, "Food & Drink"},
	{[]string{"amazon", "walmart", "target", "costco", "shop"}, "Shopping"},
	{[]string{"uber", "lyft", "gas", "shell", "bp", "chevron"}, "Transport"},
	{[]string{"netflix", "spotify", "hulu", "disney"}, "Entertainment"},
	{[]string{"salary", "payroll", "direct dep"}, "Income"},
	{[]string{"rent", "mortgage", "lease"}, "Housing"},
	{[]string{"insurance"}, "Insurance"},
	{[]string{"electric", "water", "internet", "utility", "utilities"}, "Utilities"},
	{[]string{"atm", "withdrawal"}, "Cash"},
}

// DefaultCategorizer is a simple keyword-based Categorizer using common
// finance keywords. It returns a best-guess category or empty string.
var DefaultCategorizer Categorizer = func(r Row) string {
	lower := strings.ToLower(r.Description)
	for _, entry := range keywordTable {
		for _, word := range entry.words {
			if strings.Contains(lower, word) {
				return entry.category
			}
		}
	}
	return ""
}
