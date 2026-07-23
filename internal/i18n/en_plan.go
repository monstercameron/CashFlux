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
	"plan.hero.intro":         "New to this? Money can feel overwhelming. This page keeps it simple: it gives you ONE thing to do next. Do that, come back, get the next thing.",
	"plan.hero.next":          "Your next move",
	"plan.hero.allDone":       "You've cleared every step",
	"plan.hero.allDoneDetail": "Every step your data can measure reads done. Keep investing, keep the buffer full, and revisit if your income or debts change.",
	// Shown when the top step is one we can't judge from your data yet — we ask
	// instead of pretending it's advice.
	"plan.hero.confirmLabel":  "Let's find your next move",
	"plan.hero.confirmValue":  "Answer 2 quick questions",
	"plan.hero.confirmDetail": "There are a couple of things I can't see in your data. Answer the two quick questions just below and your real next step shows up right here.",
	"plan.hero.confirmCTA":    "Answer the questions",
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
	"plan.fw.hint":       "Not sure? Leave it on Order of Operations — it's the recommended pick for most people.",

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

	// FOO — Financial Order of Operations (9 steps). `plain` is the beginner one-liner
	// (no jargon) shown as the main text; `detail` is the "why" underneath.
	"plan.foo.1.title":  "Cover your deductibles",
	"plan.foo.1.plain":  "Keep a little cash set aside for an insurance surprise — a fender-bender, a broken tooth — so you don't reach for a credit card.",
	"plan.foo.1.detail": "A deductible is the part of a bill your insurance makes you pay yourself. Having it ready is the floor that stops one accident from becoming debt.",
	"plan.foo.2.title":  "Get the full employer match",
	"plan.foo.2.plain":  "If your job adds money to your retirement when you do, put in enough to grab all of it. It's free money.",
	"plan.foo.2.detail": "An employer match is money your workplace adds to your 401(k) to match yours. Capturing all of it is an instant, guaranteed return you can't beat anywhere else.",
	"plan.foo.3.title":  "Pay off high-interest debt",
	"plan.foo.3.plain":  "Knock out your expensive debt — most credit cards, anything around 6% interest or higher.",
	"plan.foo.3.detail": "Paying off a debt is a guaranteed return equal to its interest rate. At ~6%+ that beats almost any investment, so it comes first.",
	"plan.foo.4.title":  "Build 3–6 months of reserves",
	"plan.foo.4.plain":  "Save up enough cash to cover 3 to 6 months of your bills, in case your income stops.",
	"plan.foo.4.detail": "This is your safety net. Three to six months of expenses in cash means a job loss or big surprise doesn't derail everything you've built.",
	"plan.foo.5.title":  "Fund a Roth IRA & HSA",
	"plan.foo.5.plain":  "Start investing for retirement in a Roth IRA, and use a health savings account (HSA) if you have one. Both grow tax-free.",
	"plan.foo.5.detail": "A Roth IRA is a retirement account you fund with after-tax money and never pay tax on again. An HSA (paired with a high-deductible health plan) is triple-tax-advantaged.",
	"plan.foo.6.title":  "Max out retirement accounts",
	"plan.foo.6.plain":  "Put more into your retirement accounts, up to the yearly limit the government allows.",
	"plan.foo.6.detail": "Fill your 401(k) and IRA to the annual limits to keep compounding on tax-advantaged money.",
	"plan.foo.7.title":  "Hyper-accumulate (25% of income)",
	"plan.foo.7.plain":  "Aim to save about a quarter of what you earn for your future.",
	"plan.foo.7.detail": "Pushing total retirement saving toward 25% of your gross income is the step that buys back your future time.",
	"plan.foo.8.title":  "Prepay future expenses",
	"plan.foo.8.plain":  "Set money aside now for the big things you know are coming — a kid's college, a car.",
	"plan.foo.8.detail": "Fund known upcoming costs early (for example a child's college through a 529 plan) so they don't blindside your budget.",
	"plan.foo.9.title":  "Pay off low-interest debt",
	"plan.foo.9.plain":  "Last, pay off your cheap debt — like your mortgage — whenever you're ready.",
	"plan.foo.9.detail": "With everything else handled, retire the last low-interest debt on your own timeline.",

	// Ramsey — 7 Baby Steps.
	"plan.ramsey.1.title":  "$1,000 starter emergency fund",
	"plan.ramsey.1.plain":  "Save your first $1,000 as a safety cushion, before anything else.",
	"plan.ramsey.1.detail": "This starter fund keeps a small surprise from sending you back to the credit card while you work on the rest.",
	"plan.ramsey.2.title":  "Pay off all debt (except the house)",
	"plan.ramsey.2.plain":  "Pay off every debt except your mortgage — smallest balance first, for quick wins.",
	"plan.ramsey.2.detail": "This is the debt snowball: paying smallest-to-largest gives you momentum that keeps you going.",
	"plan.ramsey.3.title":  "3–6 months of expenses saved",
	"plan.ramsey.3.plain":  "Build up 3 to 6 months of your bills in savings.",
	"plan.ramsey.3.detail": "A full emergency fund in cash is your safety net against a job loss or big surprise.",
	"plan.ramsey.4.title":  "Invest 15% for retirement",
	"plan.ramsey.4.plain":  "Put 15% of your pay into retirement accounts.",
	"plan.ramsey.4.detail": "Fifteen percent of household income, invested steadily, lets compounding do the heavy lifting.",
	"plan.ramsey.5.title":  "Save for kids' college",
	"plan.ramsey.5.plain":  "Start saving for your kids' school so they don't start life in debt.",
	"plan.ramsey.5.detail": "Once you're investing for yourself, begin funding your children's education.",
	"plan.ramsey.6.title":  "Pay off your home early",
	"plan.ramsey.6.plain":  "Throw extra money at the mortgage to own your home sooner.",
	"plan.ramsey.6.detail": "Every extra dollar toward the mortgage brings paid-off, rent-free living closer.",
	"plan.ramsey.7.title":  "Build wealth and give",
	"plan.ramsey.7.plain":  "Keep investing, and give some away — that's the whole point.",
	"plan.ramsey.7.detail": "With the house paid off and no debt, you're free to build wealth and be generous.",
}

func init() {
	for k, v := range planKeys {
		english[k] = v
	}
}
