// ux_chart_audit.mjs — R52/R64 decision-oriented chart audit.
//
// CASHFLUX_ENTERPRISE_UI_STYLE_SPEC (and R52/R64) require every chart to be
// DECISION-oriented, not decorative: it must state its takeaway and be readable,
// not a bare unlabeled mini-chart. The anti-pattern is an svg/canvas floating with
// no title, no axes/units, and no accessible description — the viewer can't tell
// what it means or what to do.
//
// This runtime audit visits the chart-bearing routes, finds every rendered chart
// (svg/canvas in the known chart containers), and checks each has the minimum
// decision scaffolding:
//   1. A TITLE/heading nearby (the card title or an explicit chart title) — what is
//      this about?  (§6.3 / R52 "a title that states the insight")
//   2. READABLE context — axis ticks/labels OR a legend/caption OR an accessible
//      text description (aria-label / role=img name / <title> / <desc>). A chart
//      with neither axes nor any text label is the "unlabeled mini-chart ambiguity"
//      R52 calls out.
// A chart failing BOTH is a hard violation. A chart with a title but no readable
// context (common for intentional sparklines) is reported as a soft note, since a
// titled sparkline beside its hero figure is an accepted pattern.
//
// Usage:  node e2e/ux_chart_audit.mjs [baseURL]
// Exit code = number of HARD violations (chart with neither title nor any label).

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
const ROUTES = ['/', '/reports', '/planning', '/health', '/goals'];

const browser = await chromium.launch();
let hard = 0, soft = 0, total = 0, noA11y = 0, noAction = 0;
const lines = [];

for (const route of ROUTES) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  try {
    await page.goto(base + route, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(1800);

    const charts = await page.evaluate(() => {
      const out = [];
      const seen = new Set();
      // A "chart" = an svg/canvas OR a CSS div-chart (segmented/bar composition the
      // Widget Builder renders without svg) of meaningful size. The div-chart classes
      // (.vb-segbar/.vb-chart/.wb-bar) would otherwise be structurally invisible —
      // a decorative unlabeled segbar must still count. Progress bars ([role=
      // "progressbar"], .bar-fill) are NOT charts and are excluded.
      const CHART_SEL = 'svg, canvas, .vb-segbar, .vb-chart, .wb-bar';
      document.querySelectorAll(CHART_SEL).forEach((el) => {
        if (el.matches('[role="progressbar"]')) return;
        if (el.parentElement && el.parentElement.closest('svg')) return; // skip svg nested in another svg
        const r = el.getBoundingClientRect();
        if (r.width < 40 || r.height < 24) return;      // icons / spacers
        // Audit EVERY chart on the page, not just the first viewport — a decorative
        // unlabeled chart below the fold is still a R52 violation.
        // Skip svgs that are clearly icons (inside a button/link, or aria-hidden glyph).
        if (el.closest('button, a, .icon, .nav-link, .chip')) return;
        if (seen.has(el)) return; seen.add(el);

        // 1) Title: nearest card/section heading or an explicit chart title.
        const card = el.closest('.card, .bento, .w, .trend-body, section, article') || document.body;
        const titleEl = card.querySelector('.card-title, .chart-title, .trend-head, .w-title, h1, h2, h3, .page-title, .stat-label, .trend-figure');
        const title = (titleEl?.textContent || '').trim().slice(0, 40);

        // 2) Readable context: axis ticks, legend/caption, or accessible name.
        const hasAxis = !!el.querySelector('.tick, .domain, .axis, [class*="axis"]');
        const cap = card.querySelector('.t-caption, .chart-caption, .legend, figcaption, .axis-label');
        const hasCaption = !!(cap && cap.textContent.trim());
        // A GENERIC fallback aria-label ("Chart"/"Trend chart") is not a real
        // accessible name — it states no insight — so it does NOT count as a label.
        const rawAria = el.getAttribute('aria-label') || el.closest('[aria-label]')?.getAttribute('aria-label') || '';
        const generic = /^(trend )?chart$/i.test(rawAria.trim());
        const aria = rawAria && !generic ? rawAria : (el.getAttribute('aria-labelledby') || '');
        const svgTitle = el.tagName.toLowerCase() === 'svg' && (el.querySelector('title, desc')?.textContent || '').trim();
        const hasLabel = hasAxis || hasCaption || !!aria || !!svgTitle;

        // R64 also grades: ACCESSIBLE description (screen-reader name) and a nearby
        // ACTION/drill-down. These are reported as graded metrics (not hard-fail) —
        // per R64, remediation lands as separate C-items.
        const hasA11y = !!aria || !!svgTitle || el.getAttribute('role') === 'img';
        const actionEl = card.querySelector('button, a[href], [role="button"], .drill, .chart-action');
        const hasAction = !!actionEl;

        out.push({ w: Math.round(r.width), h: Math.round(r.height), title, hasAxis, hasCaption, hasLabel, hasA11y, hasAction, tag: el.tagName.toLowerCase() });
      });
      return out;
    });

    for (const c of charts) {
      total++;
      if (!c.hasA11y) noA11y++;
      if (!c.hasAction) noAction++;
      if (!c.title && !c.hasLabel) {
        hard++;
        lines.push(`  ❌ ${route}  ${c.tag} ${c.w}x${c.h}  — no title AND no axis/caption/aria (unlabeled chart)`);
      } else if (c.title && !c.hasLabel) {
        soft++;
        lines.push(`  · ${route}  ${c.tag} ${c.w}x${c.h}  "${c.title}" — titled but no axis/caption (sparkline-ok)`);
      }
    }
    console.log(`${route} — ${charts.length} chart(s), ${charts.filter(c => !c.title && !c.hasLabel).length} unlabeled`);
  } catch (e) {
    console.log(`${route} — ERROR ${e.message.slice(0, 40)}`);
  } finally {
    await page.close();
  }
}

await browser.close();
if (lines.length) { console.log('\nDetail:'); lines.forEach((l) => console.log(l)); }
console.log(`\nCharts graded: ${total} total`);
console.log(`  decision-orientation: ${hard} decorative/unlabeled (HARD), ${soft} titled-sparkline (ok)`);
console.log(`  accessibility (R64 C-item): ${noA11y}/${total} lack a screen-reader name (aria-label / <title>)`);
console.log(`  drill-down/action (R64 C-item): ${noAction}/${total} have no action/link in their card`);
console.log(hard === 0
  ? '\n✅ No decorative/unlabeled charts — every chart states what it is. (a11y/action gaps tracked as C-items per R64.)'
  : `\n❌ ${hard} decorative chart(s) need a title or label.`);
process.exit(Math.min(hard, 250));
