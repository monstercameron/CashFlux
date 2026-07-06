import { chromium } from 'playwright';
import { mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const screenshotDir = join(__dirname, 'screenshots');
mkdirSync(screenshotDir, { recursive: true });

const PORTS = [7777, 8080, 3000, 4321];
async function findPort() {
  for (const port of PORTS) {
    try {
      const r = await fetch(`http://localhost:${port}/`);
      if (r.status < 500) return port;
    } catch {}
  }
  throw new Error('No dev server found on ports: ' + PORTS.join(', '));
}

async function setTheme(page, theme) {
  await page.evaluate((t) => localStorage.setItem('cashflux:prefs', JSON.stringify({theme: t})), theme);
  await page.reload();
  try {
    await page.waitForFunction((t) => document.documentElement.getAttribute('data-theme') === t, theme, {timeout: 5000});
  } catch(e) { console.log('theme wait timeout'); }
  await page.waitForTimeout(800);
}

async function measureIcons(page, label) {
  const svgMeasurements = await page.evaluate(() => {
    const results = [];
    document.querySelectorAll('svg').forEach((svg, i) => {
      const rect = svg.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) return;
      const sw = svg.querySelector('[stroke-width]')?.getAttribute('stroke-width') ||
                 svg.getAttribute('stroke-width') ||
                 getComputedStyle(svg).strokeWidth || 'none';
      results.push({
        index: i,
        width: Math.round(rect.width * 10) / 10,
        height: Math.round(rect.height * 10) / 10,
        parentTag: svg.parentElement?.tagName,
        parentClass: (svg.parentElement?.className || '').toString().slice(0, 80),
        strokeWidth: sw,
        viewBox: svg.getAttribute('viewBox') || 'none',
        fill: svg.getAttribute('fill') || 'none',
      });
    });
    return results;
  });

  const unicodeGlyphs = await page.evaluate(() => {
    const GLYPHS = ['☐','✓','⚠','→','←','↑','↓','×','⠿','▲','▼','◀','▶','★','✕','✗','·','•','⋯','…','▾','▿','○','●'];
    const found = [];
    const walk = (el, depth) => {
      if (depth > 8) return;
      if (['SCRIPT','STYLE','SVG','PATH'].includes(el.tagName)) return;
      Array.from(el.childNodes).forEach(node => {
        if (node.nodeType === 3) {
          const t = node.textContent.trim();
          for (const g of GLYPHS) {
            if (t === g || t.startsWith(g) || t.endsWith(g)) {
              const rect = el.getBoundingClientRect();
              if (rect.width > 0) {
                found.push({ glyph: g, tag: el.tagName, class: (el.className||'').toString().slice(0,60), text: t.slice(0,40) });
              }
            }
          }
        } else if (node.nodeType === 1) {
          walk(node, depth + 1);
        }
      });
    };
    walk(document.body, 0);
    return found;
  });

  console.log(`\n--- ${label} ---`);

  const bySizes = {};
  svgMeasurements.forEach(m => {
    const key = `${m.width}x${m.height}`;
    if (!bySizes[key]) bySizes[key] = [];
    bySizes[key].push({ parentClass: m.parentClass, strokeWidth: m.strokeWidth });
  });
  console.log(`SVG count: ${svgMeasurements.length}`);
  console.log('Sizes:', Object.entries(bySizes).map(([k,v]) => `${k}(${v.length})`).join(', '));

  const strokes = [...new Set(svgMeasurements.map(m => m.strokeWidth))];
  console.log('Stroke widths:', strokes.join(', '));

  if (unicodeGlyphs.length > 0) {
    console.log('Unicode glyphs found:');
    const seen = new Set();
    unicodeGlyphs.forEach(g => {
      const key = `${g.glyph}|${g.tag}|${g.class}`;
      if (!seen.has(key)) {
        seen.add(key);
        console.log(`  "${g.glyph}" <${g.tag} class="${g.class}"> text="${g.text}"`);
      }
    });
  } else {
    console.log('Unicode glyphs: none detected');
  }

  return { svgMeasurements, unicodeGlyphs, bySizes };
}

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.setViewportSize({ width: 1280, height: 800 });

const port = await findPort();
console.log('Dev server port:', port);
const BASE = `http://localhost:${port}`;

const routes = [
  ['dashboard', '/'],
  ['transactions', '/#/transactions'],
  ['budgets', '/#/budgets'],
  ['goals', '/#/goals'],
  ['reports', '/#/reports'],
];

for (const theme of ['light', 'dark']) {
  for (const [name, path] of routes) {
    try {
      await page.goto(BASE + path, { waitUntil: 'networkidle', timeout: 10000 });
      await setTheme(page, theme);
      const shot = `gx06_${name}_${theme}_1280.png`;
      await page.screenshot({ path: join(screenshotDir, shot) });
      console.log('Screenshot:', shot);
      await measureIcons(page, `${name} / ${theme}`);
    } catch(e) {
      console.log(`SKIP ${name}/${theme}: ${e.message.slice(0,80)}`);
    }
  }
}

// Rail nav close-up
await page.goto(BASE, { waitUntil: 'networkidle', timeout: 10000 });
await setTheme(page, 'light');
const railInfo = await page.evaluate(() => {
  const nav = document.querySelector('nav, [class*="rail"], [class*="sidebar"]');
  if (!nav) return { error: 'nav not found', html: document.body.innerHTML.slice(0, 300) };
  return {
    html: nav.outerHTML.slice(0, 1200),
    icons: Array.from(nav.querySelectorAll('svg')).map(svg => {
      const r = svg.getBoundingClientRect();
      const sw = svg.style.strokeWidth || svg.getAttribute('stroke-width') || getComputedStyle(svg).strokeWidth;
      return { w: Math.round(r.width), h: Math.round(r.height), viewBox: svg.getAttribute('viewBox'), strokeWidth: sw }
    })
  };
});
console.log('\nRail nav detail:', JSON.stringify(railInfo, null, 2));

// Transactions page for sort carets + cleared checkmarks
await page.goto(BASE + '/#/transactions', { waitUntil: 'networkidle', timeout: 10000 });
await setTheme(page, 'light');
const txnIconDetail = await page.evaluate(() => {
  const thButtons = Array.from(document.querySelectorAll('.th-sort'));
  const carets = thButtons.map(b => ({ text: b.textContent.trim(), tag: b.tagName }));

  const clearedCells = Array.from(document.querySelectorAll('.td-cleared, [class*="cleared"]'));
  const cleared = clearedCells.map(c => ({ html: c.innerHTML.slice(0, 200), class: c.className }));

  return { sortButtons: carets, clearedCells: cleared.slice(0, 5) };
});
console.log('\nTransaction sort buttons:', JSON.stringify(txnIconDetail.sortButtons));
console.log('Cleared cells (first 5):', JSON.stringify(txnIconDetail.clearedCells));

// Reports screen for Advanced toggle
await page.goto(BASE + '/#/reports', { waitUntil: 'networkidle', timeout: 10000 });
await setTheme(page, 'light');
const reportsDetail = await page.evaluate(() => {
  const btns = Array.from(document.querySelectorAll('button')).filter(b => b.textContent.includes('Advanced'));
  return btns.map(b => ({ text: b.textContent.trim(), class: b.className }));
});
console.log('\nReports Advanced buttons:', JSON.stringify(reportsDetail));

// Dashboard priority dots
await page.goto(BASE, { waitUntil: 'networkidle', timeout: 10000 });
await setTheme(page, 'light');
const dashDetail = await page.evaluate(() => {
  const GLYPHS = ['▲','●','○'];
  const found = [];
  const walk = (el, depth) => {
    if (depth > 10) return;
    if (['SCRIPT','STYLE','SVG','PATH'].includes(el.tagName)) return;
    Array.from(el.childNodes).forEach(node => {
      if (node.nodeType === 3) {
        const t = node.textContent.trim();
        for (const g of GLYPHS) {
          if (t === g) {
            found.push({ glyph: g, tag: el.tagName, class: (el.className||'').toString().slice(0,60) });
          }
        }
      } else if (node.nodeType === 1) walk(node, depth + 1);
    });
  };
  walk(document.body, 0);
  return found;
});
console.log('\nDashboard priority glyphs:', JSON.stringify(dashDetail));

await browser.close();
console.log('\n=== PROBE DONE ===');
