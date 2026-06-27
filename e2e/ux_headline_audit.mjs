// ux_headline_audit.mjs — R49 headline-figure standardization audit.
//
// R49: each money page should have ONE obvious headline figure and "no competing
// same-weight money figures in the first viewport." The failure mode is several
// equally-large money numbers fighting for the eye, so the user can't tell which
// number is the story. This audit measures, per page, the visual weight
// (fontSize * fontWeight) of every money/stat figure in the first viewport, finds
// the dominant one, and flags a page where 2+ figures TIE at the top weight — i.e.
// no single hero. A dominant hero with smaller supporting stats (the intended
// pattern) passes; a flat wall of same-size numbers fails.
//
// Usage:  node e2e/ux_headline_audit.mjs [baseURL]
// Exit code = number of pages with competing same-weight headline figures.

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
// R49's named pages.
const ROUTES = ['/', '/reports', '/accounts', '/budgets', '/goals', '/health', '/bills', '/subscriptions', '/allocate', '/planning'];

// Selectors for "money / headline figure" elements.
const FIG_SEL = [
  '.hero-net', '.hero-flanker-value', '.hero-stat-value', '.stat-value', '.kpi-value',
  '.t-figure', '.t-figure-lg', '.metric-value', '.figure', '.amount-income', '.amount-expense',
].join(',');

const browser = await chromium.launch();
let failed = 0;
const rows = [];

for (const route of ROUTES) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  try {
    await page.goto(base + route, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(1500);

    const r = await page.evaluate((sel) => {
      const vis = (el) => {
        const b = el.getBoundingClientRect();
        if (b.width < 2 || b.height < 2) return false;
        if (b.bottom <= 0 || b.top >= 1000) return false; // first viewport
        for (let e = el; e && e !== document.body; e = e.parentElement) {
          const s = getComputedStyle(e);
          if (s.display === 'none' || s.visibility === 'hidden' || +s.opacity === 0) return false;
        }
        return true;
      };
      const figs = [];
      document.querySelectorAll(sel).forEach((el) => {
        if (!vis(el)) return;
        const s = getComputedStyle(el);
        const size = parseFloat(s.fontSize) || 0;
        const weight = parseInt(s.fontWeight) || 400;
        // Visual weight ≈ area-ish proxy: fontSize scaled by boldness.
        const w = size * (weight / 400);
        figs.push({ w: Math.round(w * 10) / 10, size, text: (el.textContent || '').trim().slice(0, 14) });
      });
      figs.sort((a, b) => b.w - a.w);
      // Count how many figures tie (within 8%) at the top visual weight.
      const top = figs.length ? figs[0].w : 0;
      const tied = figs.filter((f) => f.w >= top * 0.92).length;
      return { count: figs.length, top, tied, sample: figs.slice(0, 4) };
    }, FIG_SEL);

    // A page with 0/1 figure trivially passes. 2+ figures tying at the top weight =
    // competing headlines (no single hero) = R49 violation.
    const bad = r.count >= 2 && r.tied >= 2;
    if (bad) failed++;
    rows.push({ route, ...r, bad });
  } catch (e) {
    rows.push({ route, count: -1, top: 0, tied: 0, sample: [], bad: false, err: e.message.slice(0, 30) });
  } finally {
    await page.close();
  }
}

await browser.close();

console.log(`R49 headline-figure audit — ${base}\n`);
console.log('page             figs  topW  tied  status');
for (const r of rows) {
  console.log(
    `${r.route.padEnd(15)} ${String(r.count).padStart(4)}  ${String(r.top).padStart(5)}  ${String(r.tied).padStart(4)}  ` +
    (r.bad ? `❌ ${r.tied} figures tie for headline` : 'ok') +
    (r.err ? `  ERR ${r.err}` : '')
  );
}
console.log(`\nPages with competing same-weight headline figures: ${failed}/${rows.length}`);
process.exit(Math.min(failed, 250));
