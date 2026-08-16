// SPDX-License-Identifier: MIT

package insights

import "fmt"

// QuestionContext supplies the user's data hooks for tailoring starter questions
// in the "Ask about your money" box. Empty fields are simply skipped.
type QuestionContext struct {
	TopCategory     string // their biggest spend category recently
	NearLimitBudget string // a budget near or over its limit
	UpcomingGoal    string // a goal with a near target

	// EC-16: the state that makes a question worth asking RIGHT NOW rather than
	// in general. These outrank the standing questions above, because a chip that
	// names something true today is the difference between a suggestion and a
	// prompt.

	// OverBudgetCount is how many budgets are currently over their limit.
	OverBudgetCount int
	// FlaggedCount is how many findings are waiting in flagged activity.
	FlaggedCount int
	// LargestExpensePayee and LargestExpenseAmount describe the biggest single
	// expense of the current period; Amount is a pre-formatted money string so
	// this package stays free of currency handling.
	LargestExpensePayee  string
	LargestExpenseAmount string
	// UncategorizedCount is how many transactions have no category.
	UncategorizedCount int
}

// maxSuggestedQuestions is how many chips the Ask box offers. Four fits one line
// at most widths; more turns a prompt into a menu, which is the thing a blank box
// was already failing to avoid.
const maxSuggestedQuestions = 4

// SuggestedQuestions returns up to four tappable starter questions for the Ask box,
// seeded from what is true about the household's data right now (EC-16).
//
// The ORDER is the feature. A chip that names something currently true — "3 budgets
// are over" — is a prompt; a chip that asks a reasonable question in general is
// only a suggestion, and a box full of suggestions is the blank box it replaced.
// So the situational questions come first, most-actionable first, and the standing
// questions fill whatever room is left. Everything is derived from the passed
// context, so the same data always produces the same chips in the same order — a
// list that reshuffles between renders reads as randomness rather than as insight.
func SuggestedQuestions(ctx QuestionContext) []string {
	out := make([]string, 0, maxSuggestedQuestions)
	add := func(q string) {
		if q == "" || len(out) >= maxSuggestedQuestions {
			return
		}
		for _, e := range out {
			if e == q {
				return
			}
		}
		out = append(out, q)
	}

	// Situational: something is true today that is worth asking about.
	switch {
	case ctx.OverBudgetCount == 1:
		add("One budget is over — what changed?")
	case ctx.OverBudgetCount > 1:
		add(fmt.Sprintf("%d budgets are over — what changed?", ctx.OverBudgetCount))
	}
	switch {
	case ctx.FlaggedCount == 1:
		add("Something's flagged — is it a problem?")
	case ctx.FlaggedCount > 1:
		add(fmt.Sprintf("%d things are flagged — are any of them a problem?", ctx.FlaggedCount))
	}
	if ctx.LargestExpensePayee != "" && ctx.LargestExpenseAmount != "" {
		add(fmt.Sprintf("What was the %s at %s?", ctx.LargestExpenseAmount, ctx.LargestExpensePayee))
	}
	switch {
	case ctx.UncategorizedCount == 1:
		add("One transaction has no category — where does it belong?")
	case ctx.UncategorizedCount > 1:
		add(fmt.Sprintf("%d transactions have no category — can you sort them?", ctx.UncategorizedCount))
	}

	// Standing: reasonable questions whatever the data says, still tailored where
	// there is something to tailor with.
	if ctx.TopCategory != "" {
		add("How much did we spend on " + ctx.TopCategory + " last month?")
	}
	if ctx.NearLimitBudget != "" {
		add("How is our " + ctx.NearLimitBudget + " budget doing?")
	}
	if ctx.UpcomingGoal != "" {
		add("Are we on track for our " + ctx.UpcomingGoal + " goal?")
	}
	add("Where did our money go last month?")
	add("How does this month compare to last month?")
	add("Can we afford a $2,000 trip?")
	return out
}
