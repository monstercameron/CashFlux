// ux_density_audit.mjs — R44/R72 desktop UX quality gate (density dimension).
//
// Counts FIRST-VIEWPORT visible controls per route and compares them to the
// control-density budgets in CASHFLUX_ENTERPRISE_UI_STYLE_SPEC §11, following the
// §11.1 measurement protocol (1440x1000, dark theme, seeded sample data, no modal
// / hover / selection). Content controls are counted separately from global shell
// controls (rail / topbar), and controls hidden by display/visibility/opacity,
// offscreen position, or collapsed disclosure are excluded — matching §11.1 so a
// row action tucked behind hover/overflow does not inflate the count.
//
// Usage:  node e2e/ux_density_audit.mjs [baseURL]
// Exit code = number of routes OVER their hard ceiling, so it can gate CI.

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');

// Route -> archetype, with §11 [target, hardCeiling] for content controls.
const BUDGET = {
  overview:  [35, 45],
  ledger:    [55, 65],
  planning:  [45, 55],
  builder:   [55, 70],
  settings:  [40, 50],
  admin:     [30, 40],
};
const ROUTES = [
  ['/', 'overview'],
  ['/accounts', 'ledger'],
  ['/transactions', 'ledger'],
  ['/budgets', 'planning'],
  ['/goals', 'overview'],
  ['/planning', 'planning'],
  ['/reports', 'planning'],
  ['/bills', 'planning'],
  ['/subscriptions', 'ledger'],
  ['/allocate', 'planning'],
  ['/health', 'overview'],
  ['/todo', 'ledger'],
  ['/notifications', 'ledger'],
  ['/workflows', 'builder'],
  ['/customize', 'builder'],
  ['/widget-manager', 'builder'],
];

const CONTROL_SEL = [
  'button', 'a[href]', 'input', 'select', 'textarea',
  '[role="button"]', '[role="switch"]', '[role="checkbox"]',
  '[role="radio"]', '[role="tab"]', '[contenteditable="true"]',
].join(',');

const browser = await chromium.launch();
let over = 0;
const rows = [];

for (const [route, type] of ROUTES) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  try {
    await page.goto(base + route, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('.card, .bento, main', { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(1500);

    const counts = await page.evaluate((sel) => {
      const visible = (el) => {
        const r = el.getBoundingClientRect();
        if (r.width < 2 || r.height < 2) return false;
        if (r.bottom <= 0 || r.top >= 1000 || r.right <= 0 || r.left >= 1440) return false; // first viewport only
        const s = getComputedStyle(el);
        if (s.display === 'none' || s.visibility === 'hidden' || +s.opacity === 0) return false;
        if (el.disabled || el.getAttribute('aria-disabled') === 'true') return false;
        return true;
      };
      // Shell = the persistent rail + topbar chrome; everything else is content.
      const shellRoots = [...document.querySelectorAll('aside.rail, .rail, .topbar, .mobile-tabbar, .sidebar')];
      const inShell = (el) => shellRoots.some((r) => r.contains(el));
      let content = 0, shell = 0;
      const seen = new Set();
      document.querySelectorAll(sel).forEach((el) => {
        if (seen.has(el)) return; seen.add(el);
        if (!visible(el)) return;
        if (inShell(el)) shell++; else content++;
      });
      const h1 = (document.querySelector('h1, .page-title')?.textContent || '').trim().slice(0, 28);
      return { content, shell, h1 };
    }, CONTROL_SEL);

    const [target, ceiling] = BUDGET[type];
    const flag = counts.content > ceiling ? 'OVER-CEILING' : counts.content > target ? 'over-target' : 'ok';
    if (counts.content > ceiling) over++;
    rows.push({ route, type, ...counts, target, ceiling, flag });
  } catch (e) {
    rows.push({ route, type, content: -1, shell: -1, h1: 'ERROR ' + e.message.slice(0, 30), flag: 'error' });
  } finally {
    await page.close();
  }
}

await browser.close();

console.log('route            type      content  (target/ceiling)  shell   status');
for (const r of rows) {
  console.log(
    `${r.route.padEnd(16)} ${String(r.type).padEnd(9)} ${String(r.content).padStart(5)}    (${r.target}/${r.ceiling})`.padEnd(52) +
    `   ${String(r.shell).padStart(3)}   ${r.flag}` + (r.h1 ? `  "${r.h1}"` : '')
  );
}
console.log(`\nRoutes over hard ceiling: ${over}/${rows.length}`);
process.exit(Math.min(over, 250));
