// The manager's DataTable (.wm-table + horizontal scroll wrapper) became the
// bespoke .wman ledger, which WRAPS on phones instead of scrolling: order +
// name + switch on the first line, size/reorder controls beneath, controls
// visible at rest (no hover on touch). This verifies the phone layout doesn't
// clip and the controls are reachable, plus the desktop layout stays flat.
import { chromium } from 'playwright';
const BASE = process.env.E2E_URL || 'http://127.0.0.1:8099';
const log = []; const P = s => log.push('PASS ' + s); const F = s => log.push('FAIL ' + s);
async function boot(p) {
  await p.goto(BASE, { waitUntil: 'domcontentloaded' });
  await p.waitForFunction(() => document.querySelector('#app')?.textContent.length > 40, { timeout: 20000 });
  await p.evaluate(() => { const o = document.getElementById('gwc-error-overlay'); if (o) o.remove(); });
}
async function wm(p) {
  await p.evaluate(() => { history.pushState({}, '', '/widget-manager'); dispatchEvent(new PopStateEvent('popstate')); });
  await p.waitForTimeout(1100);
}
(async () => {
  const b = await chromium.launch();
  // 390: ledger wraps, page itself doesn't overflow, controls visible at rest.
  {
    const ctx = await b.newContext({ viewport: { width: 390, height: 850 } });
    const p = await ctx.newPage();
    await boot(p); await wm(p);
    const m = await p.evaluate(() => {
      const ledger = document.querySelector('.wman-ledger');
      const row = document.querySelector('.wman-ledger .wm-row');
      if (!ledger || !row) return { missing: true };
      const docW = document.documentElement.clientWidth;
      const pageClip = document.documentElement.scrollWidth > docW + 2;
      const step = row.querySelector('.wm-size');
      const stepVisible = step ? getComputedStyle(step).opacity === '1' : false;
      const rowRight = Math.round(row.getBoundingClientRect().right);
      return { pageClip, docW, rowRight, stepVisible };
    });
    if (m.missing) { F('390: .wman-ledger / rows missing'); }
    else {
      (!m.pageClip) ? P(`390: no page-level horizontal overflow (docW=${m.docW})`) : F(`390: page overflows horizontally`);
      (m.rowRight <= m.docW + 2) ? P(`390: rows stay within the viewport (right=${m.rowRight})`) : F(`390: row clips (right=${m.rowRight} docW=${m.docW})`);
      m.stepVisible ? P('390: size steppers visible at rest (no hover on touch)') : F('390: size steppers hidden on touch layout');
    }
    await p.screenshot({ path: 'e2e/screenshots/wman_390.png' });
    await ctx.close();
  }
  // 1280: desktop ledger flat, no page overflow, board map present.
  {
    const ctx = await b.newContext({ viewport: { width: 1280, height: 850 } });
    const p = await ctx.newPage();
    await boot(p); await wm(p);
    const m = await p.evaluate(() => ({
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 2,
      map: document.querySelectorAll('.wman-map-tile').length,
    }));
    (!m.overflow) ? P('1280: desktop ledger fits, no page overflow') : F('1280: page overflows');
    (m.map > 0) ? P(`1280: board map renders (${m.map} tiles)`) : F('1280: board map missing');
    await ctx.close();
  }
  await b.close();
  console.log(log.map(s => '  ' + s).join('\n'));
  const f = log.filter(s => s.startsWith('FAIL')).length;
  console.log(`\n${log.filter(s => s.startsWith('PASS')).length} PASS / ${f} FAIL`);
  process.exit(f ? 1 : 0);
})();
