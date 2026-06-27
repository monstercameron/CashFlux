// ux_overflow_audit.mjs — R44/R72 desktop UX quality gate (overflow dimension).
//
// Page-level horizontal overflow is a release blocker in the style spec
// (§5.3 / §5.5.11): a desktop work page must never force the whole document to
// scroll sideways. This audit loads each route at a spread of widths (§5.5.11
// matrix: wide desktop → minimum narrow) and flags any route whose document
// scrolls horizontally, naming the widest offending element so the fix is
// actionable. Deliberate inner scrollers (a table inside `overflow-x:auto`) are
// fine and excluded — only PAGE-level overflow fails.
//
// Usage:  node e2e/ux_overflow_audit.mjs [baseURL]
// Exit code = number of (route,width) pairs with page overflow, for CI gating.

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
const ROUTES = ['/', '/accounts', '/transactions', '/budgets', '/planning', '/reports',
  '/bills', '/subscriptions', '/allocate', '/todo', '/workflows', '/widget-builder', '/widget-manager'];
const WIDTHS = [1440, 1200, 900, 768, 390, 320]; // §5.5.11 verification matrix

const browser = await chromium.launch();
let fails = 0;
const rows = [];

for (const route of ROUTES) {
  for (const width of WIDTHS) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    try {
      await page.goto(base + route, { waitUntil: 'domcontentloaded' });
      await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
      await page.waitForTimeout(900);
      const res = await page.evaluate((vw) => {
        const de = document.documentElement;
        const overflow = de.scrollWidth - de.clientWidth;
        if (overflow <= 1) return { overflow: 0 };
        // Find the widest element whose right edge exceeds the viewport — the likely
        // culprit. Skip elements that are themselves inside an x-scroll container
        // (legitimate inner scrollers like a wide ledger table).
        let worst = null, worstRight = vw;
        for (const el of document.querySelectorAll('body *')) {
          const r = el.getBoundingClientRect();
          if (r.right <= worstRight + 1) continue;
          let inScroller = false;
          for (let p = el.parentElement; p; p = p.parentElement) {
            const ox = getComputedStyle(p).overflowX;
            if (ox === 'auto' || ox === 'scroll') { inScroller = true; break; }
          }
          if (inScroller) continue;
          worstRight = r.right;
          worst = `${el.tagName.toLowerCase()}.${(el.className || '').toString().trim().split(/\s+/).slice(0, 2).join('.')}`;
        }
        return { overflow: Math.round(overflow), worst, worstRight: Math.round(worstRight) };
      }, width);
      if (res.overflow > 1) {
        fails++;
        rows.push(`OVERFLOW ${route} @${width}px  +${res.overflow}px  culprit=${res.worst || '?'} (right=${res.worstRight})`);
      }
    } catch (e) {
      rows.push(`ERROR ${route} @${width}px  ${e.message.slice(0, 40)}`);
    } finally {
      await page.close();
    }
  }
}

await browser.close();
if (rows.length) rows.forEach((r) => console.log(r));
else console.log('No page-level horizontal overflow at any route/width.');
console.log(`\nPage-overflow failures: ${fails} / ${ROUTES.length * WIDTHS.length} checked`);
process.exit(Math.min(fails, 250));
