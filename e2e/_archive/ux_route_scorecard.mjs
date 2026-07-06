// ux_route_scorecard.mjs — R44 desktop per-route UX score report.
//
// The dimension audits (contrast/density/overflow/parity) each answer ONE question
// across all routes and exit on aggregate failures — good for CI gating, but they
// don't tell you "how is route X doing overall?". This script is the other half of
// R44's acceptance ("a script produces a score report for all routes"): it visits
// every registered route once and computes a single 0-100 desktop-UX score per
// route from measurable signals, then prints a ranked scorecard so a reviewer can
// see at a glance which pages are strongest/weakest and verify an R36-R43 fix moves
// the right route's number. Scoring follows CASHFLUX_ENTERPRISE_UI_STYLE_SPEC:
//
//   signal              spec        penalty
//   density over hard ceiling §11    -30   (over soft target only: -10)
//   horizontal overflow @1440 §5.5.11 -25
//   no single clear headline  §6.3    -12   (0 or >1 visible h1/.page-title)
//   no hero figure on a money page §6.3 -8  (overview/planning archetypes only)
//
// Note: shell (rail/topbar) controls are measured and reported for context but are
// NOT penalized — the persistent nav rail lives in its own column with a fixed
// ~35-link set, so "shell >= content" mis-fires on exactly the calmest, best pages
// (a 2-control /health is excellent, not chrome-dominated). Content density is the
// real signal and is scored directly via the §11 budget above.
//
// Score is advisory (the hard gates live in ux_quality_gate.mjs); this exits 0
// unless a route falls below FLOOR (60), which would indicate a real regression.
//
// Usage:  node e2e/ux_route_scorecard.mjs [baseURL]

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
const FLOOR = 60;

// Route -> archetype, with §11 [softTarget, hardCeiling] content-control budgets.
// (Same archetype budgets as ux_density_audit.mjs §11, kept in sync.)
// NOTE on the builder ceiling: §11 allows builder pages 70 only "in explicit edit
// mode"; at rest the ceiling is 55. This audit measures at rest but uses 70 to match
// ux_density_audit.mjs (both share this known leniency) — builder routes scoring 100
// here should be spot-checked against the 55 at-rest target until edit-mode detection
// is added.
const BUDGET = {
  overview: [35, 45], ledger: [55, 65], planning: [45, 55],
  builder: [55, 70], settings: [40, 50], admin: [30, 40],
};
// Superset of the density audit's route list: it adds /settings and /admin (the two
// gated/config archetypes) for fuller scorecard coverage. The hard density gate
// (ux_density_audit.mjs) deliberately omits those two; passing this scorecard is
// therefore broader than — not a substitute for — the gate.
const ROUTES = [
  ['/', 'overview'], ['/accounts', 'ledger'], ['/transactions', 'ledger'],
  ['/budgets', 'planning'], ['/goals', 'overview'], ['/planning', 'planning'],
  ['/reports', 'planning'], ['/bills', 'planning'], ['/subscriptions', 'ledger'],
  ['/allocate', 'planning'], ['/health', 'overview'], ['/todo', 'ledger'],
  ['/notifications', 'ledger'], ['/workflows', 'builder'], ['/customize', 'builder'],
  ['/widget-manager', 'builder'], ['/settings', 'settings'], ['/admin', 'admin'],
];

const CONTROL_SEL = [
  'button', 'a[href]', 'input', 'select', 'textarea',
  '[role="button"]', '[role="switch"]', '[role="checkbox"]',
  '[role="radio"]', '[role="tab"]', '[contenteditable="true"]',
  '[tabindex]:not([tabindex="-1"])', // §11.1: focusable custom controls (canvas nodes, custom widgets)
].join(',');

function measure(page) {
  return page.evaluate((sel) => {
    const visible = (el) => {
      const r = el.getBoundingClientRect();
      if (r.width < 2 || r.height < 2) return false;
      if (r.bottom <= 0 || r.top >= 1000 || r.right <= 0 || r.left >= 1440) return false;
      if (el.disabled || el.getAttribute('aria-disabled') === 'true') return false;
      for (let e = el; e && e !== document.body; e = e.parentElement) {
        const s = getComputedStyle(e);
        if (s.display === 'none' || s.visibility === 'hidden' || +s.opacity === 0) return false;
      }
      return true;
    };
    const shellRoots = [...document.querySelectorAll('aside.rail, .rail, .topbar, .mobile-tabbar, .sidebar')];
    const inShell = (el) => shellRoots.some((r) => r.contains(el));
    let content = 0, shell = 0;
    const seen = new Set();
    document.querySelectorAll(sel).forEach((el) => {
      if (seen.has(el) || !visible(el)) return; seen.add(el);
      if (inShell(el)) shell++; else content++;
    });
    // Headline: a single dominant page title in the first viewport. Dedup via a
    // Set so an <h1 class="page-title"> (matched by both selectors) counts ONCE —
    // otherwise a perfectly-structured page would falsely read headlines=2.
    const seenH = new Set();
    document.querySelectorAll('h1, .page-title').forEach((el) => { if (visible(el)) seenH.add(el); });
    const headlines = seenH.size;
    // Hero figure: a prominent money/figure element near the top. §6.3 wants ONE
    // dominant figure — too few (0) is weak, but a wall of same-weight figures
    // (>=5) is the opposite §6.3 failure ("never 4-6 same-weight money figures").
    const seenF = new Set();
    document.querySelectorAll('.hero-net, .t-figure-lg, .stat-value, .kpi-value, .t-figure').forEach((el) => {
      if (visible(el)) seenF.add(el);
    });
    const hero = seenF.size;
    // Horizontal overflow at this width.
    const overflow = document.documentElement.scrollWidth - document.documentElement.clientWidth;
    const h1 = (document.querySelector('h1, .page-title')?.textContent || '').trim().slice(0, 26);
    return { content, shell, headlines, hero, overflow, h1 };
  }, CONTROL_SEL);
}

const browser = await chromium.launch();
const rows = [];

for (const [route, type] of ROUTES) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  try {
    await page.goto(base + route, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(1500);
    const m = await measure(page);
    const [target, ceiling] = BUDGET[type];
    const reasons = [];
    let score = 100;
    if (m.content > ceiling) { score -= 30; reasons.push(`density ${m.content}>${ceiling}`); }
    else if (m.content > target) { score -= 10; reasons.push(`density ${m.content}>${target}`); }
    if (m.overflow > 1) { score -= 25; reasons.push(`overflow ${m.overflow}px`); }
    if (m.headlines !== 1) { score -= 12; reasons.push(`headlines=${m.headlines}`); }
    if ((type === 'overview' || type === 'planning') && m.hero === 0) { score -= 8; reasons.push('no hero figure'); }
    if (m.hero >= 5) { score -= 8; reasons.push(`figure-wall hero=${m.hero}`); } // §6.3 same-weight figure wall
    rows.push({ route, type, score: Math.max(0, score), ...m, reasons });
  } catch (e) {
    rows.push({ route, type, score: 0, content: -1, shell: -1, headlines: -1, hero: -1, overflow: -1, h1: 'ERROR', reasons: [e.message.slice(0, 30)] });
  } finally {
    await page.close();
  }
}

await browser.close();
rows.sort((a, b) => a.score - b.score);

console.log(`CashFlux per-route desktop UX scorecard — ${base}`);
console.log(`measured: dark theme, 1440x1000, first viewport, no modal/hover/selection (§11.1)`);
console.log(`scored signals: density (§11), overflow (§5.5.11), headline (§6.3), hero figure (§6.3).`);
console.log(`NOT scored here (covered by ux_quality_gate.mjs): contrast (§12), theme parity (§12.1), light-theme density.\n`);
console.log('score  route            type      ctrl  shell  hero  notes');
console.log('────────────────────────────────────────────────────────────────────');
for (const r of rows) {
  const mark = r.score >= 85 ? '✅' : r.score >= FLOOR ? '· ' : '❌';
  console.log(
    `${mark}${String(r.score).padStart(3)}  ${r.route.padEnd(16)} ${String(r.type).padEnd(9)} ` +
    `${String(r.content).padStart(4)}  ${String(r.shell).padStart(4)}  ${String(r.hero).padStart(4)}  ${r.reasons.join('; ')}`
  );
}
const avg = Math.round(rows.reduce((s, r) => s + r.score, 0) / rows.length);
const below = rows.filter((r) => r.score < FLOOR).length;
console.log('────────────────────────────────────────────────────────────────────');
console.log(`\nMean score ${avg}/100 across ${rows.length} routes — ${below} below floor (${FLOOR}).`);
process.exit(Math.min(below, 250));
