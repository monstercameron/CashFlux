package domain

// IncludedInPayoff reports whether this account participates in the debt-payoff
// plan. The user's explicit choice (IncludeInPayoff) wins; with no choice every
// liability is included except a mortgage, which is excluded by default — real
// debt-crusher plans target revolving/consumer debt, and a 30-year mortgage would
// otherwise dominate the timeline. (Callers filter to liabilities first; this just
// answers the include/exclude question.)
func (a Account) IncludedInPayoff() bool {
	if a.IncludeInPayoff != nil {
		return *a.IncludeInPayoff
	}
	return a.Type != TypeMortgage
}
