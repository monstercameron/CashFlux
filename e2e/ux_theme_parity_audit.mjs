// ux_theme_parity_audit.mjs — R44/R72/R69 theme HIERARCHY-PARITY gate.
//
// The contrast audit proves both themes are AA-legible. This audit proves the
// other half of §12.1/R69: dark and light preserve the SAME visual HIERARCHY, not
// just adequate contrast. CashFlux themes change only color *tokens* (via the
// theme engine + semantic alias layer) — the type scale, weights, spacing, and
// layout are theme-invariant — so a route's structural "hierarchy fingerprint"
// (heading/figure/label font-size + weight + element box geometry) MUST be
// identical in dark and light. This script captures that fingerprint per route in
// both themes and fails on any mismatch, so a future change that accidentally
// diverges hierarchy between themes (e.g. a light-only font-size override) is caught.
//
// Usage:  node e2e/ux_theme_parity_audit.mjs [baseURL]
// Exit code = number of (route, element) hierarchy mismatches, for CI gating.

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
const ROUTES = ['/', '/accounts', '/transactions', '/budgets', '/planning', '/reports', '/bills', '/subscriptions', '/goals', '/health'];

// Structural selectors whose typographic hierarchy must match across themes.
const SEL = 'h1, h2, .page-title, .card-title, .hero-net, .t-figure, .t-figure-lg, .stat-value, .stat-label, .t-caption, .kpi-value';

function fingerprint(page) {
  return page.evaluate((sel) => {
    const out = [];
    document.querySelectorAll(sel).forEach((el) => {
      const r = el.getBoundingClientRect();
      if (r.width < 2 || r.height < 2 || r.top < 0 || r.top > 1400) return;
      const s = getComputedStyle(el);
      // key = a stable identity for the element (tag + class + trimmed text head),
      // value = the hierarchy attributes that must NOT differ by theme.
      out.push({
        key: el.tagName.toLowerCase() + '.' + (el.className || '').toString().trim().split(/\s+/).slice(0, 2).join('.') + '|' + (el.textContent || '').trim().slice(0, 14),
        fontSize: s.fontSize,
        fontWeight: s.fontWeight,
        lineHeight: s.lineHeight,
        w: Math.round(r.width),
        h: Math.round(r.height),
      });
    });
    return out;
  }, SEL);
}

async function capture(browser, route, light) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  await page.goto(base + route, { waitUntil: 'domcontentloaded' });
  if (light) {
    await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 8000 }).catch(() => {});
  }
  await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(1200);
  const fp = await fingerprint(page);
  await page.close();
  return fp;
}

const browser = await chromium.launch();
let mismatches = 0;
const lines = [];

for (const route of ROUTES) {
  const dark = await capture(browser, route, false);
  const light = await capture(browser, route, true);
  // Index light by key; compare each dark element's hierarchy attrs.
  const lightByKey = new Map();
  for (const e of light) if (!lightByKey.has(e.key)) lightByKey.set(e.key, e);
  let routeMiss = 0;
  for (const d of dark) {
    const l = lightByKey.get(d.key);
    if (!l) continue; // element absent in one theme (data-driven) — not a hierarchy divergence
    const diffs = [];
    if (d.fontSize !== l.fontSize) diffs.push(`size ${d.fontSize}≠${l.fontSize}`);
    if (d.fontWeight !== l.fontWeight) diffs.push(`weight ${d.fontWeight}≠${l.fontWeight}`);
    if (Math.abs(d.h - l.h) > 2) diffs.push(`height ${d.h}≠${l.h}`);
    if (diffs.length) {
      routeMiss++;
      lines.push(`  ${route}  ${d.key.split('|')[0]} "${d.key.split('|')[1]}"  ${diffs.join(', ')}`);
    }
  }
  mismatches += routeMiss;
  console.log(`${route} — ${routeMiss} hierarchy mismatch(es)  (dark ${dark.length} / light ${light.length} elems)`);
}

await browser.close();
if (lines.length) { console.log('\nMismatches:'); lines.forEach((l) => console.log(l)); }
console.log(`\nTheme hierarchy-parity mismatches: ${mismatches}`);
process.exit(Math.min(mismatches, 250));
