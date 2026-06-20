package insights

// QuestionContext supplies the user's data hooks for tailoring starter questions
// in the "Ask about your money" box. Empty fields are simply skipped.
type QuestionContext struct {
	TopCategory     string // their biggest spend category recently
	NearLimitBudget string // a budget near or over its limit
	UpcomingGoal    string // a goal with a near target
}

// SuggestedQuestions returns up to four tappable starter questions for the Ask box,
// tailored to the user's data when available (their top category, a near-limit
// budget, an upcoming goal) with generic fallbacks. The list is deterministic,
// de-duplicated, capped at four, and never empty — so a blank box never stalls the
// user.
func SuggestedQuestions(ctx QuestionContext) []string {
	out := make([]string, 0, 4)
	add := func(q string) {
		if q == "" || len(out) >= 4 {
			return
		}
		for _, e := range out {
			if e == q {
				return
			}
		}
		out = append(out, q)
	}
	if ctx.TopCategory != "" {
		add("How much did we spend on " + ctx.TopCategory + " last month?")
	}
	add("Where did our money go last month?")
	if ctx.NearLimitBudget != "" {
		add("How is our " + ctx.NearLimitBudget + " budget doing?")
	}
	if ctx.UpcomingGoal != "" {
		add("Are we on track for our " + ctx.UpcomingGoal + " goal?")
	}
	add("Can we afford a $2,000 trip in August?")
	add("How does this month compare to last month?")
	return out
}
