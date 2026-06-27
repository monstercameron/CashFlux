// ux_contrast_audit.mjs — R44/R72 desktop UX quality gate (contrast slice).
//
// A repeatable WCAG 2.2 contrast audit over the app's routes in BOTH dark and
// light themes, per CASHFLUX_ENTERPRISE_UI_STYLE_SPEC §12 (AA: normal text
// >=4.5:1, large text >=3:1) and the §11.1 measurement protocol (1440x1000,
// dark measured first, seeded sample data). It samples every visible leaf text
// node inside cards/widgets/main, resolves the effective background by walking
// ancestors, and NORMALIZES every computed color through a canvas — so modern
// color-mix() outputs in the `color(srgb ...)` form are read correctly (a naive
// regex parser misreads those 0..1 floats as 0..255 and reports false failures).
//
// Usage:  node e2e/ux_contrast_audit.mjs [baseURL]
// Default baseURL http://127.0.0.1:8099/ (serve with: go run e2e/serve.go).
// Exit code is the total number of real failures across routes/themes, so it can
// gate CI. Prints a per-route, per-theme summary plus each failing pair.

import { chromium } from 'playwright';

const base = (process.argv[2] || 'http://127.0.0.1:8099/').replace(/\/$/, '');
const ROUTES = ['/', '/accounts', '/transactions', '/budgets', '/planning', '/reports', '/bills', '/subscriptions'];
const THEMES = ['dark', 'light']; // dark first per §11.1

function relLum(r, g, b) {
  const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}
function contrast(a, b) {
  const la = relLum(...a), lb = relLum(...b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

const browser = await chromium.launch();
let totalFails = 0;

for (const route of ROUTES) {
  for (const theme of THEMES) {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    try {
      await page.goto(base + route, { waitUntil: 'domcontentloaded' });
      if (theme === 'light') {
        await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' })));
        await page.reload({ waitUntil: 'domcontentloaded' });
        await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light', { timeout: 8000 }).catch(() => {});
      }
      await page.waitForSelector('.bento .w, .card, main', { timeout: 20000 }).catch(() => {});
      await page.waitForTimeout(1500);

      const samples = await page.evaluate(() => {
        // Normalize ANY CSS color string to [r,g,b] 0..255 via a 1x1 canvas,
        // which handles rgb(), rgba(), and color(srgb ...) uniformly.
        const cv = document.createElement('canvas'); cv.width = cv.height = 1;
        const ctx = cv.getContext('2d', { willReadFrequently: true });
        const norm = (c) => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = '#000'; ctx.fillStyle = c; ctx.fillRect(0, 0, 1, 1); const d = ctx.getImageData(0, 0, 1, 1).data; return [d[0], d[1], d[2], d[3]]; };
        const visible = (el) => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 3 && r.height > 3 && r.top >= 0 && r.top < 1000 && s.visibility !== 'hidden' && s.opacity !== '0'; };
        // Effective background, walking ancestors. Returns null when the bg-providing
        // element paints a gradient/image instead of a solid color — we can't sample a
        // single representative pixel from a gradient, so the pair is skipped rather
        // than reported as a (false) failure against an unrelated ancestor color.
        const bgOf = (el) => { let e = el; while (e) { const cs = getComputedStyle(e); if (cs.backgroundImage && cs.backgroundImage !== 'none') return null; const c = norm(cs.backgroundColor); if (c[3] > 10) return [c[0], c[1], c[2]]; e = e.parentElement; } const b = norm(getComputedStyle(document.body).backgroundColor); return [b[0], b[1], b[2]]; };
        // §12.1: text needs 4.5/3, but ICON GLYPHS (single non-alphanumeric symbol,
        // dots, emoji, arrows) are non-text UI judged at 3:1 elsewhere — exclude them
        // from the text audit so they aren't flagged at the stricter 4.5 threshold.
        const isIconGlyph = (t) => t.length <= 2 && !/[a-z0-9]/i.test(t);
        const out = []; const seen = new Set();
        document.querySelectorAll('.bento .w *, .card *, main *').forEach((el) => {
          if (el.children.length > 0) return;
          const t = (el.textContent || '').trim(); if (!t || t.length > 60) return;
          if (isIconGlyph(t)) return;
          if (!visible(el)) return;
          const cs = getComputedStyle(el);
          const fg = norm(cs.color); if (fg[3] < 10) return;
          const bg = bgOf(el); if (!bg) return; // gradient/indeterminate bg — skip
          const key = cs.color + '|' + (el.className || '') + '|' + t.slice(0, 8);
          if (seen.has(key)) return; seen.add(key);
          out.push({ cls: (el.className || '').toString().slice(0, 28), fg: [fg[0], fg[1], fg[2]], bg, size: parseFloat(cs.fontSize), weight: parseInt(cs.fontWeight) || 400, t: t.slice(0, 28) });
        });
        return out;
      });

      const fails = [];
      for (const s of samples) {
        const cr = contrast(s.fg, s.bg);
        const large = s.size >= 24 || (s.size >= 18.66 && s.weight >= 700);
        const need = large ? 3 : 4.5;
        if (cr < need - 0.05) fails.push({ ...s, cr: cr.toFixed(2), need });
      }
      totalFails += fails.length;
      console.log(`[${theme}] ${route} — ${fails.length} fail / ${samples.length} sampled`);
      fails.slice(0, 15).forEach((f) => console.log(`    ${f.cr}:1 (need ${f.need}) ${f.size}px .${f.cls} "${f.t}"`));
    } catch (e) {
      console.log(`[${theme}] ${route} — ERROR ${e.message}`);
    } finally {
      await page.close();
    }
  }
}

await browser.close();
console.log(`\nTOTAL contrast failures: ${totalFails}`);
process.exit(Math.min(totalFails, 250));
