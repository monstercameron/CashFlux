// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/agemoney"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
)

// budgets.go holds the shared logic behind the widgetized /budgets surface: the
// per-period status computation every tile reads (computeBudgetView), the row
// mutation handlers (buildBudgetRowCallbacks), and the small formatting helpers the
// budget rows and add form share. The surface host lives in budgets_widget.go and the
// tile bodies in budgets_tiles.go, mirroring the /accounts split.

// budgetView is the fully computed budget picture for the current view window: the
// health-sorted statuses plus every per-budget annotation (pace, rollover carry,
// prorated pace, envelope balance, effective method) and the roll-up totals the
// summary tile shows. It is a pure value — computeBudgetView takes the resolved hook
// values as plain arguments, so both the summary and list tiles can build it without
// sharing state.
type budgetView struct {
	// SummaryActual and SummaryIssues cache the union-of-scopes calendar-window
	// spend alongside the own-cadence statuses. SummaryReady allows isolated view
	// fixtures to use the uncached helper without making production renders scan.
	SummaryActual   int64
	SummaryIssues   []budgeting.ViewIssue
	SummaryReady    bool
	SummaryKnown    bool
	SummaryComplete bool
	Base            string
	Method          budgeting.Methodology
	Statuses        []budgeting.Status
	CatName         map[string]string
	PaceOver        map[string]string         // budgetID → projected overspend (in-progress only)
	PaceMark        map[string]budgetPaceMark // budgetID → even-pace tick + caption (BG3, in-progress only)
	RollCarry       map[string]string         // budgetID → previous-period carry phrase
	RollNeg         map[string]bool           // budgetID → carry is negative
	RollEffCap      map[string]string         // budgetID → effective cap (when it differs from the base limit)
	RollEffCapMath  map[string]string         // budgetID → the cap's arithmetic ("$250 limit + $50 top-up")
	ProratedRest    map[string]string         // budgetID → even-pace amount left
	EnvAvail        map[string]string         // budgetID → envelope balance (envelope method)
	EnvNeg          map[string]bool           // budgetID → envelope overdrawn
	EnvDebtStart    map[string]string         // budgetID → BG5 "starts <month> down $X" when overdrawn
	Covered         map[string]bool           // budgetID → received cover money this period
	// EarlyPeriod marks budgets whose own period is too young to score (C344).
	// Three days in, nothing has been broken yet, and a card that renders that as
	// "On track" is making a claim about the calendar, not about the household.
	EarlyPeriod map[string]bool
	EffMethod   map[string]budgeting.Methodology // budgetID → resolved method (override or global)
	// PeriodFrom / PeriodTo map each budget to its CURRENT evaluation window as
	// inclusive ISO dates, so drill-throughs (e.g. the attention strip's "View
	// spending") can scope the ledger to the same period the figures describe
	// instead of dumping the category's full history (UI/UX task #14).
	PeriodFrom map[string]string
	PeriodTo   map[string]string
	OverCount  int
	NearCount  int
	// LeakRows are the suspect transfer rows that actually contributed to the
	// figures above — each one counted by SOME budget, in THAT budget's own
	// window and category set (C672).
	//
	// It cannot be derived from the page's toolbar window, which is the mistake
	// the first attempt made: this page shows one macro window but scores N
	// budgets on N independent cadences, so a yearly budget's March charge is in
	// TotalSpent while the toolbar says August. A leak notice clipped to the
	// toolbar renders "nothing wrong" beside a total that is wrong.
	LeakRows   []domain.Transaction
	TotalSpent int64
	// Split is the month divided by where the money went, against income.
	Split             budgeting.IncomeSplit
	TotalLimit        int64
	TotalOver         int64
	TotalFundSetAside int64
	BannerIncome      int64 // monthly income (per the chosen basis) for the assign banner
	// SavingsAssigned is the total monthly income assigned to savings/investments —
	// the sum of every savings/investment account's monthly savings budget
	// (Account.MonthlySavings), FX-converted to base minor units. Counts toward
	// "assigned" in the zero-based view so To Assign spans expenses + savings.
	// SavingsAccts is the per-account breakdown the savings section renders.
	SavingsAssigned   int64
	SavingsIncomplete bool
	SavingsAccts      []savingsAcct
	// SavingsBudgeted is the total asked for by SAVING budgets this period, and
	// SavingsMoved is how much has actually been set aside toward them. Kept apart
	// from TotalLimit/TotalSpent because a contribution target is not a spending
	// cap and mixing them makes both figures lie.
	SavingsBudgeted int64
	SavingsMoved    int64
	// RolledOver is last month's unspent budget carried into this month's assignable
	// pool (zero-based view), when the roll-leftover option is on. Raises To Assign.
	RolledOver int64
	// LastMonth holds the "Last month's spend" overlay per budget (keyed by budget ID),
	// populated only when that toggle is on; empty otherwise. LastMonthMode is that
	// toggle, and LastTotalSpent / LastTotalLimit are last period's total spend and
	// total budget across all budgets (base minor units), so the summary graph can show
	// last month against last month's OWN budget — never last spend vs this budget,
	// which is a figure from two different periods.
	LastMonth      map[string]budgetLastMonth
	LastMonthMode  bool
	LastTotalSpent int64
	LastTotalLimit int64
	// Committed holds the XC4 committed-vs-free split per budget (keyed by budget ID),
	// including any XC3 smoothing set-aside folded into the committed figure. Absent
	// entries mean nothing is committed (a fully-free budget).
	Committed map[string]budgetCommitted
	// Incomplete lists budgets whose figures could not be evaluated or converted.
	Incomplete []string
	// AgeMoney is the household's Age of Money — how long, on average, a dollar sits
	// between being earned and being spent (YNAB's signature buffer metric). Computed
	// over the whole ledger (transfers excluded), so it is scope-independent; the
	// summary tile shows it plain-English when Ready, or a "need more history" nudge.
	AgeMoney agemoney.Result
	// Anchor is the date the view's period math is anchored to: today when the
	// viewed window includes today, otherwise the window's start (C40). Rows must
	// derive their period-scoped figures (metrics strip, drivers, composition)
	// from THIS — not time.Now() — or a historical month shows today's pacing
	// (QA CF-05).
	Anchor time.Time
}

// budgetBaseCurrency applies the Budgets surfaces' shared currency fallback.
func budgetBaseCurrency(base string) string {
	if base == "" {
		return "USD"
	}
	return base
}

func budgetNow() time.Time { return budgetNowAt(time.Now()) }

func budgetNowAt(now time.Time) time.Time { return dateutil.WallClockUTC(now) }

// budgetNeedsAttention is the shared definition behind the attention count and
// its list filter. Over-limit and near-limit budgets are both included.
func budgetNeedsAttention(status budgeting.Status) bool {
	return budgeting.NeedsViewAttention(status)
}

// budgetAttentionStatuses selects the exact set named by the attention count.
func budgetAttentionStatuses(statuses []budgeting.Status) []budgeting.Status {
	return budgeting.ViewAttention(statuses)
}

// budgetCommitted is the per-budget committed-vs-free breakdown the row renders as a
// second meter segment plus a quiet caption (XC4), with an optional plain-English
// set-aside explainer for smoothed annual/quarterly bills (XC3).
type budgetCommitted struct {
	CommittedStr string // formatted committed amount, e.g. "$45.00"
	FreeStr      string // formatted truly-free amount, e.g. "$55.00"
	CommittedPct int    // committed as a percent of the limit (0..100), for the segment width
	SpentPct     int    // spent as a percent of the limit (0..100), where the committed segment starts
	SetAsideNote string // XC3 explainer, e.g. "includes $50 set-aside for Insurance (Dec)"; "" hides it
}

// budgetLastMonth is the "Last month's spend" overlay for one budget: the formatted
// actual spend last period, how it lines up against this month's budget (a short
// "$X under"/"$X over" phrase + whether it exceeded it), Pct — last month's spend as a
// percent of this month's budget (uncapped, for the "%" figure) — and Fill, the same
// clamped to 0..100 for the bar width.
type budgetLastMonth struct {
	Spent string
	Delta string
	Over  bool
	Pct   int
	Fill  int
}

// incomeSource is one selectable income category in the "by source" budget basis: its
// id, display name, and how much it brought in last full month (base minor units).
type incomeSource struct {
	CategoryID string
	Name       string
	Minor      int64
}

// savingsAcct is one savings/investment account in the zero-based "Savings &
// investments" section: its identity, the per-account monthly savings budget
// (Account.MonthlySavings, in the account's own currency) and its base-currency
// equivalent, plus — when the account funds a goal — the plan-vs-reality read
// (the goal's target-date pace vs. where this monthly rate actually lands).
type savingsAcct = budgetview.SavingsAccount

// monthsUntil preserves the private adapter used by the savings display.
func monthsUntil(from, target time.Time) int {
	return budgetview.MonthsUntil(from, target)
}

// computeSavingsAccounts formats native facts for the savings rows.
func computeSavingsAccounts(app *appstate.App, member string, asOf time.Time, base string, rates currency.Rates) []savingsAcct {
	rows := budgetview.SavingsAccounts(app.Accounts(), app.Goals(), member, asOf, base, rates)
	for i := range rows {
		rows[i].Type = humanizeType(rows[i].Type)
	}
	return rows
}

func savingsMonthlyBase(monthly int64, monthlyCurrency, base string, rates currency.Rates) int64 {
	converted, _ := savingsMonthlyBaseChecked(monthly, monthlyCurrency, base, rates)
	return converted
}

func savingsMonthlyBaseChecked(monthly int64, monthlyCurrency, base string, rates currency.Rates) (int64, error) {
	if monthly == 0 {
		return 0, nil
	}
	converted, err := currency.ConvertBetween(monthly, monthlyCurrency, base, rates)
	if err != nil {
		return 0, err
	}
	return converted, nil
}

// computeBudgetView runs the full budget evaluation for the active member scope and
// the given view window. It is the single source of truth the /budgets tiles share:
// the same statuses drive the summary totals, the banners, and the rows. Pure (no
// hooks) — the caller resolves the active member, period window, and prefs and passes
// them in, so it can be called from more than one tile without hook-ordering issues.
// budgetViewKey contains only identity, revisions and scalar view inputs. Settings
// (including FX) are covered by the store revision; preferences get a local
// version when their serialized value changes. No FX map formatting is needed.
type budgetViewKey struct {
	App                *appstate.App
	Revision           uint64
	DataRevision       int
	Member             string
	Start, End, Anchor time.Time
	PrefsVersion       uint64
	Prior              bool
}

var budgetViewCache = struct {
	sync.Mutex
	values       map[budgetViewKey]budgetView
	order        []budgetViewKey
	prefs        string
	prefsVersion uint64
}{values: make(map[budgetViewKey]budgetView)}

// budgetLedgerForApp shares the single netting pass across all budget surfaces
// for an app's data revision. Entries are bounded without flushing other apps.
var budgetLedgerCache = struct {
	sync.Mutex
	values map[*appstate.App]struct {
		revision uint64
		ledger   budgeting.NettedLedger
	}
	order []*appstate.App
}{values: make(map[*appstate.App]struct {
	revision uint64
	ledger   budgeting.NettedLedger
})}

func budgetLedgerForApp(app *appstate.App) budgeting.NettedLedger {
	budgetLedgerCache.Lock()
	defer budgetLedgerCache.Unlock()
	revision := app.Rev()
	if entry, ok := budgetLedgerCache.values[app]; ok && entry.revision == revision {
		return entry.ledger
	}
	ledger := budgeting.NewNettedLedger(app.Transactions(), app.TxnLinks())
	if _, ok := budgetLedgerCache.values[app]; !ok {
		if len(budgetLedgerCache.order) == 8 {
			delete(budgetLedgerCache.values, budgetLedgerCache.order[0])
			budgetLedgerCache.order = budgetLedgerCache.order[1:]
		}
		budgetLedgerCache.order = append(budgetLedgerCache.order, app)
	}
	budgetLedgerCache.values[app] = struct {
		revision uint64
		ledger   budgeting.NettedLedger
	}{revision, ledger}
	return ledger
}

// computeBudgetView memoizes completed views with synchronized bounded eviction.
func computeBudgetView(app *appstate.App, activeMemberID string, vw period.Window, pr prefs.Prefs, showLastMonth bool) budgetView {
	if app == nil {
		return budgetView{}
	}
	budgetViewCache.Lock()
	defer budgetViewCache.Unlock()
	encoded, _ := json.Marshal(pr)
	if budgetViewCache.prefs != string(encoded) {
		budgetViewCache.prefs = string(encoded)
		budgetViewCache.prefsVersion++
	}
	start, end := vw.Range()
	key := budgetViewKey{App: app, Revision: app.Rev(), DataRevision: uistate.CurrentDataRevision(), Member: activeMemberID, Start: start, End: end,
		Anchor: dateutil.CalendarDayUTC(budgetViewAnchor(vw, time.Now())), PrefsVersion: budgetViewCache.prefsVersion, Prior: showLastMonth}
	if v, ok := budgetViewCache.values[key]; ok {
		return v
	}
	v := computeBudgetViewRaw(app, activeMemberID, vw, pr, showLastMonth)
	if len(budgetViewCache.order) == 8 {
		delete(budgetViewCache.values, budgetViewCache.order[0])
		budgetViewCache.order = budgetViewCache.order[1:]
	}
	budgetViewCache.order = append(budgetViewCache.order, key)
	budgetViewCache.values[key] = v
	return v
}

func computeBudgetViewRaw(app *appstate.App, activeMemberID string, vw period.Window, pr prefs.Prefs, showLastMonth bool) budgetView {
	base := budgetBaseCurrency(app.Settings().BaseCurrency)
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	start, end := vw.Range()
	ledger := budgetLedgerForApp(app)
	input := budgeting.ViewInput{
		Ledger:       &ledger,
		Period:       budgeting.ResolveViewPeriod(start, end, time.Now(), pr.WeekStartWeekday(), budgetPayCycleAnchor(pr.PayCycleAnchor)),
		Transactions: app.Transactions(), Categories: app.Categories(), Recurring: app.Recurring(),
		Rates: rates, Base: base, PoolRollover: pr.BudgetRolloverLeftover, ShowPrior: showLastMonth,
	}
	builder := newBudgetViewBuilder(input, showLastMonth)
	budgets := budgeting.ViewBudgets(app.Budgets(), activeMemberID)
	for _, budget := range budgets {
		builder.addBudget(budgeting.EvaluateViewBudget(budget, input))
	}
	builder.summarize()
	builder.addEnvelopes(budgets, app.Settings().BudgetMethodology)
	builder.addIncome(pr)
	builder.addSavings(app, activeMemberID)
	summary := budgetSummarySpending(ledger, app.Categories(), builder.view.Statuses, vw, showLastMonth, rates, base)
	builder.view.SummaryActual, builder.view.SummaryIssues = summary.Amount.Amount, summary.Issues
	builder.view.SummaryKnown, builder.view.SummaryComplete = summary.Known, summary.Complete
	builder.view.SummaryReady = true
	return builder.view
}

// computeAgeOfMoney builds the age-of-money input from the ledger and runs the
// pure agemoney.Compute. Every non-transfer transaction becomes a Flow in base-
// currency minor units (income positive, expense negative); transfers are skipped
// because they neither earn nor spend money. Flows are sorted oldest-first, which
// is what the FIFO metric requires, so the store's ordering doesn't matter here.
// An unconvertible amount (missing FX rate) is dropped rather than mis-aged.
func computeAgeOfMoney(txns []domain.Transaction, base string, rates currency.Rates) agemoney.Result {
	return budgetview.AgeOfMoney(txns, base, rates)
}

var budgetAgeMoneyCache = newViewCache[agemoney.Result]()

// budgetAgeOfMoney shares the scope-independent FIFO history across view windows
// and member lenses. FX changes are covered by the app's stored-data revision.
func budgetAgeOfMoney(app *appstate.App, txns []domain.Transaction, base string, rates currency.Rates) agemoney.Result {
	if app == nil {
		return computeAgeOfMoney(txns, base, rates)
	}
	return memoByRev(budgetAgeMoneyCache, revKey(app)+"|"+base, func() agemoney.Result {
		return computeAgeOfMoney(txns, base, rates)
	})
}

// incomeBasisMonth resolves the month the income basis is derived from for the
// viewed window: today's month while the window contains today, otherwise the
// window's own month.
//
// C531: this used to be answered in two places with two different answers. The
// page anchored on the view window while the basis MODAL anchored on time.Now(),
// so a user paging back to a past month approved a figure in the modal that the
// page never used — the preview and the page were averaging different months
// over different data. Both now call this.
func incomeBasisMonth(vw period.Window, weekStart time.Weekday) time.Time {
	now := time.Now()
	anchor := budgetViewAnchor(vw, now)
	ms, _ := budgeting.PeriodRange(domain.PeriodMonthly, anchor, weekStart)
	return ms
}

// resolveIncomeBasis is the ONE place the "income to budget with" figure is
// computed. The page passes the saved preferences; the basis modal passes its
// unsaved draft. Both go through here so a previewed figure and an applied
// figure can never be computed differently.
func resolveIncomeBasis(
	app *appstate.App,
	mode string, paycheckMin, fixed int64, catIDs []string, avgMonths int,
	ms time.Time, base string, rates currency.Rates,
) (int64, error) {
	if app == nil {
		return 0, nil
	}
	if mode == "" {
		mode = budgeting.IncomeModeAll
	}
	sources := budgeting.NewIncomeSources(app.Categories(), catIDs)
	return budgeting.AveragedIncomeChecked(mode, paycheckMin, fixed, sources, app.Transactions(), ms, avgMonths, base, rates)
}

// computeIncomeSources builds the income-source menu the "by source" basis presents:
// every income-kind category with how much it earned per month over the last `months`
// full months ending at monthStart (a plain average when months > 1), largest first,
// plus any uncategorized income that actually landed. months < 1 is treated as 1 (last
// month only). A category with no income in the window shows a zero amount, so the user
// can still pre-include it; the checked rows sum to the AveragedIncome figure.
func computeIncomeSources(app *appstate.App, base string, rates currency.Rates, monthStart time.Time, months int) []incomeSource {
	if months < 1 {
		months = 1
	}
	start := dateutil.AddMonths(monthStart, -months)
	amtByCat := map[string]int64{}
	if lines, err := reports.IncomeByCategory(app.Transactions(), start, monthStart, rates); err == nil {
		for _, ln := range lines {
			amtByCat[ln.CategoryID] = ln.Amount / int64(months)
		}
	}
	// Rank rows by LAST month's amount — a window independent of `months` — so toggling
	// the averaging window changes only the figures shown, never the row order (no
	// reshuffle under the user's cursor). Sources that only earned in prior months stay
	// pinned at the bottom, showing their average, instead of jumping up when averaging
	// surfaces them.
	rankByCat := map[string]int64{}
	rankStart := dateutil.AddMonths(monthStart, -1)
	if lines, err := reports.IncomeByCategory(app.Transactions(), rankStart, monthStart, rates); err == nil {
		for _, ln := range lines {
			rankByCat[ln.CategoryID] = ln.Amount
		}
	}
	var out []incomeSource
	for _, c := range app.Categories() {
		if c.Kind != domain.KindIncome {
			continue
		}
		out = append(out, incomeSource{CategoryID: c.ID, Name: c.Name, Minor: amtByCat[c.ID]})
	}
	if amt := amtByCat[""]; amt > 0 {
		out = append(out, incomeSource{CategoryID: "", Name: uistate.T("budgets.incomeSourceUncat"), Minor: amt})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rankByCat[out[i].CategoryID], rankByCat[out[j].CategoryID]; ri != rj {
			return ri > rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// budgetRowCallbacks bundles the per-row mutation handlers the list tile hands to each
// BudgetRow. The edit / top-up / cover editors now live in the shell-root flip modal
// (BudgetEditForm) which mutates the store directly, so the row only needs delete.
type budgetRowCallbacks struct {
	OnDelete          func(string)
	OnRemoveRecurring func(string)
}

// buildBudgetRowCallbacks wires the store mutations for BudgetRow, mirroring the
// /accounts buildAcctRowCallbacks convention.
func buildBudgetRowCallbacks(app *appstate.App, base string, catName map[string]string) budgetRowCallbacks {
	return budgetRowCallbacks{
		OnDelete: func(budgetID string) {
			// Guard the destructive delete with a confirm (matches the transactions delete
			// pattern) — a single misclick was previously unrecoverable.
			name := uistate.T("budgets.thisBudget")
			for _, b := range app.Budgets() {
				if b.ID == budgetID {
					if n := catName[b.CategoryID]; n != "" {
						name = n
					}
					break
				}
			}
			// C597: the old copy read "This can't be undone", which is false —
			// Ctrl+Z restores a deleted budget (verified in a browser). A
			// confirmation that overstates the risk teaches people that
			// confirmations are noise, the same defect C571 fixed for duplicates.
			// It now says what actually happens to the transactions, and the shared
			// impact line states the reach and the reversibility.
			msg := uistate.T("budgets.deleteConfirmHonest", name) + " " + fundsImpactLine(budgeting.ImpactDeleteBudget)
			uistate.ConfirmModal(msg, true, func(ok bool) {
				if !ok {
					return
				}
				if err := app.DeleteBudget(budgetID); err != nil {
					uistate.PostNotice(budgetErrorText(err), true)
					return
				}
				uistate.BumpDataRevision()
			})
		},
		OnRemoveRecurring: func(budgetID string) {
			name := uistate.T("budgets.thisBudget")
			for _, b := range app.Budgets() {
				if b.ID == budgetID {
					if n := catName[b.CategoryID]; n != "" {
						name = n
					}
					break
				}
			}
			// C597: a standing arrangement stops funding FUTURE periods; this one is
			// already paid for. The shared line says so.
			rmMsg := uistate.T("budgets.removeRecurringConfirm", name) + " " +
				fundsImpactLine(budgeting.ImpactRemoveRecurringCover)
			uistate.ConfirmModal(rmMsg, true, func(ok bool) {
				if !ok {
					return
				}
				for _, b := range app.Budgets() {
					if b.ID != budgetID {
						continue
					}
					b.RecurringCover = nil
					if err := app.PutBudget(b); err != nil {
						uistate.PostNotice(budgetErrorText(err), true)
						return
					}
					break
				}
				uistate.BumpDataRevision()
			})
		},
	}
}

// --- shared row props + formatting helpers (used by BudgetRow + the add form) ------

type budgetRowProps struct {
	Status            budgeting.Status
	Category          string
	TrackedCats       string // comma-joined category names for a multi-category budget; "" for single
	Members           []domain.Member
	BudgetDefs        []customfields.Def    // custom-field defs for the "budget" entity (display + inline edit)
	Envelope          string                // formatted envelope balance (envelope methodology); "" hides the line
	EnvelopeNeg       bool                  // envelope is overdrawn → danger tone
	EnvelopeDebtStart string                // BG5: "Starts March down $32" when overdrawn; "" hides it
	PaceOver          string                // formatted projected overspend (pace, in-progress only); "" hides the line
	PaceMarkerPct     int                   // BG3: even-pace tick position as % of the limit (0 hides the tick)
	PaceCaption       string                // BG3: "on pace" / "running $38 hot" caption; "" hides the line
	PaceHot           bool                  // BG3: spending has outrun the even-pace line → caution tone
	RolloverCarry     string                // formatted previous-period carry for per-budget rollover; "" hides the line
	RolloverNeg       bool                  // previous-period carry is negative → danger tone
	EffectiveCap      string                // C136: formatted effective cap for this period when it differs from the base limit; "" = no difference
	EffectiveCapMath  string                // the cap's arithmetic ("$250 limit + $50 top-up"); shown beside EffectiveCap
	ProratedRest      string                // C143: formatted even-pace amount left for the rest of the period; "" hides the line
	EffectiveMethod   budgeting.Methodology // C118: this budget's resolved method (own override or global fallback)
	Covered           bool                  // received one-time cover money this period
	LastMonthSpent    string                // "Last month's spend" overlay: last period's actual spend; "" hides the row
	LastMonthDelta    string                // short "$X under" / "$X over" vs this month's budget
	LastMonthOver     bool                  // last month's spend exceeded this month's budget → danger tone
	LastMonthPct      int                   // last month's spend as % of this month's budget (uncapped) — the "%" figure
	LastMonthFill     int                   // same, clamped 0..100 — the bar width
	// PeriodEarly: this budget's period has barely started, so the row must not
	// render a verdict about it (C344). It shows what the period is instead, with
	// last period's outcome as the thing actually worth reading.
	PeriodEarly bool
	// PriorSpent / PriorDelta are last period's outcome, shown as the honest
	// reading while PeriodEarly holds. Distinct from the LastMonth* fields, which
	// drive the opt-in overlay that takes the whole tile over.
	PriorSpent        string
	PriorDelta        string
	PriorOver         bool
	LinkedTodos       int             // count of to-dos linked to this budget (Task.RelatedType=budget); 0 hides the link
	Committed         budgetCommitted // XC4 committed-vs-free split (+ XC3 set-aside note); zero value hides it
	HasCommitted      bool            // whether Committed carries a figure to render
	Compact           bool            // compact density: render the one-line row instead of the full card
	Anchor            time.Time       // the view's period anchor (budgetView.Anchor); zero falls back to now (QA CF-05)
	OnDelete          func(string)
	OnRemoveRecurring func(string) // clear this budget's recurring cover (confirmed)
	// OnDrill opens Transactions filtered to this budget's whole spend SCOPE —
	// tracked categories plus their descendants plus any tracked tags, the same
	// set the bar counts (C585) — AND windowed to the budget's current period
	// (from/to inclusive ISO dates; task #14).
	OnDrill     func(scope budgeting.Scope, from, to string)
	PeriodFrom  string // this budget's current-period start (inclusive ISO date)
	PeriodTo    string // this budget's current-period end (inclusive ISO date)
	OnViewTodos func() // open the To-dos page (shown when LinkedTodos > 0)
}

// budgetLeftValue formats a budget's remaining amount for the summary "Left" stat
// (C124): a positive remaining shows plainly ("$50.00"), but an overspend reads as
// "$50.00 over" instead of the ambiguous accounting parens "($50.00)".
func budgetLeftValue(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m)
}

// budgetRemainPhrase is budgetLeftValue plus the trailing "left"/"over" word, for the
// per-row summary line ("$50.00 left" / "$50.00 over"). (C124)
func budgetRemainPhrase(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m) + " " + uistate.T("budgets.leftWord")
}

// budgetPaceMark is the BG3 even-pace read for one budget: where the meter's second
// tick sits (MarkerPct, a percent of the limit) and a plain-English caption saying
// whether spending is on pace, running hot, or has a cushion so far.
type budgetPaceMark struct {
	MarkerPct int    // ideal spend-to-date as a percent of the limit (0..100)
	Caption   string // "on pace" / "running $38 hot" / "$12 under pace so far"
	Hot       bool   // spending has outrun the even-pace line → caution tone
}

// buildBudgetPaceMark computes the BG3 even-pace tick + caption for a budget while
// its period is in progress. Committed money (XC4) is excluded from the pace race —
// pre-spoken-for recurring charges aren't "spent fast". Returns ok=false outside an
// in-progress period so the marker only shows when it's meaningful.
func buildBudgetPaceMark(st budgeting.Status, committedResult budgeting.CommittedResult, bs, be, now time.Time) (budgetPaceMark, bool) {
	cur := st.Spent.Currency
	committed := money.Zero(cur)
	// Only a positive remaining can carry a committed segment (mirrors buildBudgetCommitted).
	if committedResult.Committed.Amount > 0 {
		committed = committedResult.Committed
	}
	m := budgeting.ProjectPaceMarker(st, committed, bs, be, now)
	// Only meaningful while the period is genuinely in progress.
	if m.Elapsed <= 0 || m.Elapsed >= 1 {
		return budgetPaceMark{}, false
	}
	mark := budgetPaceMark{MarkerPct: m.MarkerPct, Hot: m.Hot}
	switch {
	case m.Hot && m.Delta.Amount > 0:
		mark.Caption = uistate.T("budgets.paceHot", fmtMoney(m.Delta))
	case m.Delta.Amount < 0:
		mark.Caption = uistate.T("budgets.paceCool", fmtMoney(money.New(-m.Delta.Amount, cur)))
	default:
		mark.Caption = uistate.T("budgets.paceOnPace")
	}
	return mark, true
}

// buildBudgetCommitted derives the XC4 committed-vs-free split for one budget and
// formats it for the row. It returns ok=false (nothing to show) when the budget is
// already met/over or nothing is committed, so a fully-free budget stays quiet. The
// committed figure folds in any XC3 smoothing set-aside, and smoothed items produce a
// plain-English "includes $50 set-aside for Insurance (Dec)" explainer.
func buildBudgetCommitted(st budgeting.Status, b domain.Budget, recurrings []domain.Recurring, cr budgeting.CommittedResult) (budgetCommitted, bool) {
	if st.Remaining.IsNegative() || st.Remaining.IsZero() {
		return budgetCommitted{}, false
	}
	if cr.Committed.Amount <= 0 {
		return budgetCommitted{}, false
	}
	spentPct, committedPct := budgeting.CommittedPercentages(st.Limit.Amount, st.Spent.Amount, cr.Committed.Amount)
	var notes []string
	for _, it := range cr.Items {
		if !it.Smoothed {
			continue
		}
		month := ""
		for _, r := range recurrings {
			if r.ID == it.RecurringID {
				month = budgetMonthName(r.NextDue, true)
				break
			}
		}
		notes = append(notes, uistate.T("budgets.setAsideNote", fmtMoney(it.Amount), it.Label, month))
	}
	return budgetCommitted{
		CommittedStr: fmtMoney(cr.Committed),
		FreeStr:      fmtMoney(cr.Free),
		CommittedPct: committedPct,
		SpentPct:     spentPct,
		SetAsideNote: strings.Join(notes, "; "),
	}, true
}

// budgetMonthName uses the shared English month catalog used by the annual grid.
func budgetMonthName(t time.Time, short bool) string {
	month := int(t.Month()) - 1
	if month < 0 || month >= len(annualGridMonthKeys) {
		return ""
	}
	if short {
		return uistate.T(annualGridMonthKeys[month])
	}
	return uistate.T(annualGridMonthFullKeys[month])
}

// budgetTitle renders a budget's display title: its name, or its category when
// unnamed, or "name · category" when both add information (never "Food · Food").
func budgetTitle(name, category string) string {
	switch {
	case name == "":
		return category
	case category != "" && !strings.EqualFold(category, name):
		return name + " · " + category
	default:
		return name
	}
}

// adjustAllScope is the set of budgets a bulk adjustment acts on: the ones
// visible under the current member perspective, which is the set the page is
// showing and therefore the only one "every budget" is a checkable claim about.
//
// Shared by the Adjust-all form and by the funding callout that quotes the count
// before opening it (C671), so the number on the button and the number in the
// preview cannot drift apart.
func adjustAllScope(app *appstate.App, activeMemberID string) []domain.Budget {
	if app == nil {
		return nil
	}
	var out []domain.Budget
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			out = append(out, b)
		}
	}
	return out
}

// budgetEffectiveCaps maps each visible budget to the cap it is actually being
// scored against this period — its base limit with rollover carry-in and any
// one-time boost already folded in.
//
// It is what a THIS-PERIOD bulk adjustment scales (C671): that write records a
// boost, and a boost lands on this figure, not on the stored limit. Taking it
// from the same Statuses the cards render means the preview cannot promise a cap
// the cards would then disagree with. Failed statuses are omitted because their
// stored fallback Limit is not an evaluated effective cap.
func budgetEffectiveCaps(v budgetView) budgetCaps {
	caps := make(budgetCaps, len(v.Statuses))
	for _, s := range v.Statuses {
		if s.Error != nil {
			continue
		}
		caps[s.Budget.ID] = s.Limit.Amount
	}
	return caps
}

// budgetCaps maps each budget to its effective cap this period. Distinct from
// budgeting.AdjustOverlays on purpose: the two differ by exactly the base limit,
// and passing one where the other belongs is a mistake that reads correctly and
// computes nonsense — so it is made a compile error rather than a review item.
type budgetCaps map[string]int64

// budgetAdjustOverlays maps each budget to how far its effective cap sits above
// (or below) its stored limit this period — rollover carry-in plus any one-off
// change — which is what a bulk adjustment needs to preview either scope
// honestly (C671).
//
// It is not simply "cap minus limit for everything in the view", because a
// budget drops out of the view entirely when its spend cannot be evaluated: one
// transaction in a currency with no exchange rate is enough. Requiring a cap
// would then exclude that budget from a PERMANENT adjustment, which rewrites a
// base limit and has no business depending on spend or FX data at all. So a
// budget without rollover — whose cap can only be offset by boosts recorded on
// the budget itself — gets its overlay computed directly. Only a rollover budget
// whose evaluation failed is genuinely unknowable, and only that one is left out.
func budgetAdjustOverlays(v budgetView, budgets []domain.Budget, periodStart func(domain.Budget) time.Time) budgeting.AdjustOverlays {
	caps := budgetEffectiveCaps(v)
	out := make(budgeting.AdjustOverlays, len(budgets))
	for _, b := range budgets {
		if cap, ok := caps[b.ID]; ok {
			delta, err := money.Sub(cap, b.Limit.Amount)
			if err == nil {
				out[b.ID] = delta
			}
			continue
		}
		if !b.Rollover && periodStart != nil {
			out[b.ID] = b.PeriodBoost(periodStart(b))
		}
	}
	return out
}

// budgetPeriodStarts resolves each budget's current period start, anchored to the
// viewed window and the household's pay cycle — the same grid the cards score
// against. Shared by every caller that needs it, so two of them cannot anchor a
// write and a preview differently.
func budgetPeriodStarts(vw period.Window, pr prefs.Prefs) func(domain.Budget) time.Time {
	anchor := budgetViewAnchor(vw, time.Now())
	payAnchor := budgetPayCycleAnchor(pr.PayCycleAnchor)
	return func(b domain.Budget) time.Time {
		start, _ := budgeting.PeriodRangeAnchored(b.Period, anchor, pr.WeekStartWeekday(), payAnchor)
		return start
	}
}

// budgetReconcileCount is how many budgets the funding callout's reconcile action
// would actually change, computed exactly as the Adjust-all form computes it —
// same scope, same overlays, same preview — so the count promised on the button
// is the count the form shows. Two call sites computing it two ways is how they
// came to disagree.
func budgetReconcileCount(app *appstate.App, activeMemberID string, v budgetView, vw period.Window, pr prefs.Prefs, pct float64) int {
	scoped := adjustAllScope(app, activeMemberID)
	overlays := budgetAdjustOverlays(v, scoped, budgetPeriodStarts(vw, pr))
	return budgeting.AdjustAllPreviewFor(scoped, pct, budgeting.AdjustThisPeriod, overlays).Count()
}

// budgetLimitError maps a parsed limit's problem to the message the editor shows
// beneath the field, or "" when the typed value may be written (C665, C666).
//
// Three surfaces take a budget limit — the card's inline editor, the full edit
// form, and the add form — and each one used to decide for itself, on submit,
// what counted as valid and what to say about it. The shared pure parser makes
// the decision; this makes them all say the same sentence about the same input,
// in the budget's own currency, while the value is still being typed.
func budgetLimitError(problem budgeting.LimitProblem, cur string) string {
	switch problem {
	case budgeting.LimitBlank:
		return uistate.T("budgets.limitBlank")
	case budgeting.LimitNotPositive:
		return uistate.T("budgets.limitPositive", fmtMoney(money.Zero(cur)))
	case budgeting.LimitMalformed:
		return uistate.T("budgets.limitMalformed")
	}
	return ""
}

// disabledAttr returns the `disabled` attribute when a commit control has nothing
// valid to commit, and nothing otherwise. It registers no hooks, so appending it
// to a button's args keeps the hook count stable in both states.
func disabledAttr(disabled bool) []any {
	if disabled {
		return []any{Disabled(true)}
	}
	return nil
}

// periodLabel localizes a budget period for the UI (the domain layer's Period.Label()
// is hardcoded English by design — domain stays i18n-free). C116.
func periodLabel(p domain.Period) string {
	switch p {
	case domain.PeriodWeekly:
		return uistate.T("budgets.periodWeekly")
	case domain.PeriodBiweekly:
		return uistate.T("budgets.periodBiweekly")
	case domain.PeriodSemimonthly:
		return uistate.T("budgets.periodSemimonthly")
	case domain.PeriodQuarterly:
		return uistate.T("budgets.periodQuarterly")
	case domain.PeriodYearly:
		return uistate.T("budgets.periodYearly")
	default:
		return uistate.T("budgets.periodMonthly")
	}
}

func periodOptions(selected string) []uiw.SelectOption {
	opts := make([]uiw.SelectOption, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, uiw.SelectOption{Value: string(p), Label: periodLabel(p)})
	}
	return opts
}

// ownerSelectOptions builds owner SelectOptions (the shared group plus each member) —
// used wherever an entity's owner can be chosen.
func ownerSelectOptions(members []domain.Member, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: domain.GroupOwnerID, Label: uistate.T("owner.group")}}
	for _, m := range members {
		opts = append(opts, uiw.SelectOption{Value: m.ID, Label: m.Name})
	}
	return opts
}

func checkedAttr(checked bool) []any {
	// Use the Checked() boolean prop (sets the DOM `checked` PROPERTY), not the
	// `checked` content attribute — an attribute only seeds defaultChecked and doesn't
	// update the live property on a keyed re-render, so the tick wouldn't show/clear.
	return []any{Checked(checked)}
}

// budgetViewAnchor is the date a period-scoped figure should be computed at:
// today when the viewed window contains today, otherwise the window's start
// (C40).
//
// It is extracted so surfaces OUTSIDE computeBudgetView — the Auto-budget
// modal's income basis, for one — resolve it the same way. Reading time.Now()
// instead meant a modal opened from a closed July compared the plan against
// July's income while the page behind it used June's: two months' figures for
// one question, on one screen.
func budgetViewAnchor(vw period.Window, now time.Time) time.Time {
	start, end := vw.Range()
	return budgeting.ResolveViewPeriod(start, end, now, time.Sunday, time.Time{}).Anchor
}

// budgetLedgerForTransactions preserves full-ledger refund resolution while
// restricting budget reads to the transaction IDs chosen by the caller's scope.
func budgetLedgerForTransactions(app *appstate.App, txns []domain.Transaction) budgeting.NettedLedger {
	ledger := budgetLedgerForApp(app)
	ids := make(map[string]bool, len(txns))
	for _, txn := range txns {
		ids[txn.ID] = true
	}
	scoped := make(budgeting.NettedLedger, 0, len(txns))
	for _, txn := range ledger {
		if ids[txn.ID] {
			scoped = append(scoped, txn)
		}
	}
	return scoped
}
