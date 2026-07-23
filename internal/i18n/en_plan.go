// SPDX-License-Identifier: MIT

package i18n

// planKeys is the copy for the /plan surface ("Fix My Finances") — an opinionated,
// research-backed roadmap that places the household on one step of a well-known
// order-of-operations framework (the Financial Order of Operations, or Dave
// Ramsey's Baby Steps) from their own data, and names the single next move.
//
// Kept in its own file (init()-merge, like en_a11y.go) so the surface's copy
// lands here rather than in the user's working en.go tree. All step titles and
// details live here — the finplan engine stays a pure logic package and the
// screen renders every string through T, so the i18n ratchet holds.
var planKeys = Catalog{
	// Nav + route chrome.
	"nav.plan":       "Fix My Finances",
	"screen.planSub": "One clear next move, backed by proven money frameworks.",

	// Hero.
	"plan.hero.eyebrow":       "Your roadmap",
	"plan.hero.next":          "Your next move",
	"plan.hero.allDone":       "You've cleared every step",
	"plan.hero.allDoneDetail": "Every step your data can measure reads done. Keep investing, keep the buffer full, and revisit if your income or debts change.",
	"plan.chip.framework":     "Framework",
	"plan.chip.progress":      "Progress",
	"plan.chip.method":        "Method",
	"plan.chip.progressVal":   "%d of %d done",
	"plan.method.foo":         "Math-first",
	"plan.method.ramsey":      "Debt-first",

	// Framework switch (segmented control).
	"plan.fw.title":      "Which playbook?",
	"plan.fw.foo":        "Order of Operations",
	"plan.fw.ramsey":     "Baby Steps",
	"plan.fw.fooDesc":    "The Financial Order of Operations (The Money Guy) — nine steps that put free employer match and tax-advantaged investing ahead of low-interest debt. The math-optimal path; our default.",
	"plan.fw.ramseyDesc": "Dave Ramsey's 7 Baby Steps — kill every non-mortgage debt before you invest. Slower on paper, but the momentum keeps a lot of people going.",
	"plan.fw.pick":       "Use this playbook",

	// Step list.
	"plan.steps.title": "The full ladder",
	"plan.steps.note":  "We assess each rung from your accounts and spending. Rungs we can't measure from your data are marked “Confirm” — answer the two questions below and they resolve.",

	// Status pills.
	"plan.status.done": "Done",
	"plan.status.now":  "Do this now",
	"plan.status.todo": "To do",
	"plan.status.ask":  "Confirm",

	// Questionnaire (the "on start" questions that resolve the steps data can't see).
	"plan.q.title":          "Two quick questions",
	"plan.q.note":           "Your answers stay on this device and sharpen the roadmap above.",
	"plan.q.match":          "Do you get your employer's full retirement match?",
	"plan.q.matchHelp":      "If your job matches 401(k) contributions and you're contributing enough to capture all of it.",
	"plan.q.deductible":     "Could you cover your insurance deductibles in cash today?",
	"plan.q.deductibleHelp": "Enough set aside to pay the deductible on your health, auto, or home policy if something broke tomorrow.",
	"plan.q.yes":            "Yes",
	"plan.q.no":             "No",
	"plan.q.unsure":         "Not sure",

	// Free credit-score links.
	"plan.credit.title":        "Know your credit — for free",
	"plan.credit.note":         "You never have to pay to see your credit. These are the genuinely free, reputable sources.",
	"plan.credit.annualHref":   "https://www.annualcreditreport.com",
	"plan.credit.annualName":   "AnnualCreditReport.com",
	"plan.credit.annualDesc":   "The only federally authorized site for your free reports from all three bureaus. No score, but the full record — check it for errors.",
	"plan.credit.karmaHref":    "https://www.creditkarma.com",
	"plan.credit.karmaName":    "Credit Karma",
	"plan.credit.karmaDesc":    "Free VantageScore from TransUnion and Equifax, updated weekly, with monitoring alerts.",
	"plan.credit.experianHref": "https://www.experian.com/consumer-products/free-credit-report.html",
	"plan.credit.experianName": "Experian",
	"plan.credit.experianDesc": "Free FICO Score 8 and Experian credit report straight from the bureau.",
	"plan.credit.chaseHref":    "https://www.chase.com/personal/credit-cards/journey/credit-journey",
	"plan.credit.chaseName":    "Chase Credit Journey",
	"plan.credit.chaseDesc":    "Free VantageScore and monitoring — no Chase account required.",
	"plan.credit.disclaimer":   "CashFlux isn't affiliated with these services and earns nothing from them. Links open in a new tab.",

	// FOO — Financial Order of Operations (9 steps).
	"plan.foo.1.title":  "Cover your deductibles",
	"plan.foo.1.detail": "Keep enough cash on hand to pay your insurance deductibles if something breaks. This is the floor that stops one accident from becoming debt.",
	"plan.foo.2.title":  "Get the full employer match",
	"plan.foo.2.detail": "Contribute at least enough to your workplace retirement plan to capture the entire employer match. It's an instant, guaranteed return — free money you can't beat anywhere else.",
	"plan.foo.3.title":  "Pay off high-interest debt",
	"plan.foo.3.detail": "Attack any debt at roughly 6% APR or higher. Clearing it is a guaranteed return that beats almost any investment.",
	"plan.foo.4.title":  "Build 3–6 months of reserves",
	"plan.foo.4.detail": "Save three to six months of expenses in cash so a job loss or a big surprise doesn't derail everything you've built.",
	"plan.foo.5.title":  "Fund a Roth IRA & HSA",
	"plan.foo.5.detail": "Max these tax-advantaged accounts. The HSA is triple-tax-advantaged; the Roth grows tax-free for decades.",
	"plan.foo.6.title":  "Max out retirement accounts",
	"plan.foo.6.detail": "Fill your 401(k) and IRA up to the annual limits to keep compounding on tax-advantaged money.",
	"plan.foo.7.title":  "Hyper-accumulate (25% of income)",
	"plan.foo.7.detail": "Push total retirement saving toward 25% of your gross income. This is the step that buys back your future time.",
	"plan.foo.8.title":  "Prepay future expenses",
	"plan.foo.8.detail": "Fund known upcoming costs — a child's college through a 529, a planned major purchase — before they arrive.",
	"plan.foo.9.title":  "Pay off low-interest debt",
	"plan.foo.9.detail": "With everything else handled, retire the last low-interest debt, like your mortgage, on your own timeline.",

	// Ramsey — 7 Baby Steps.
	"plan.ramsey.1.title":  "$1,000 starter emergency fund",
	"plan.ramsey.1.detail": "Save a $1,000 starter fund before anything else. It's the buffer that keeps a small surprise from sending you back to the credit card.",
	"plan.ramsey.2.title":  "Pay off all debt (except the house)",
	"plan.ramsey.2.detail": "Clear every non-mortgage debt using the snowball — smallest balance first, for the wins that keep you going.",
	"plan.ramsey.3.title":  "3–6 months of expenses saved",
	"plan.ramsey.3.detail": "Build a full emergency fund of three to six months of expenses in cash.",
	"plan.ramsey.4.title":  "Invest 15% for retirement",
	"plan.ramsey.4.detail": "Put 15% of household income into retirement accounts and let compounding do the work.",
	"plan.ramsey.5.title":  "Save for kids' college",
	"plan.ramsey.5.detail": "Start funding your children's education so they don't start life in debt.",
	"plan.ramsey.6.title":  "Pay off your home early",
	"plan.ramsey.6.detail": "Throw everything extra at the mortgage and own your home outright.",
	"plan.ramsey.7.title":  "Build wealth and give",
	"plan.ramsey.7.detail": "Keep investing, and give generously — the whole point of the plan.",
}

func init() {
	for k, v := range planKeys {
		english[k] = v
	}
}
