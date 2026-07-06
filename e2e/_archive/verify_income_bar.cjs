// Verify income-by-source bar chart renders on /reports
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();

  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') {
      const text = msg.text();
      if (/already exited/i.test(text)) return;
      errors.push(text);
    }
  });
  page.on('pageerror', err => {
    if (/already exited/i.test(err.message)) return;
    errors.push(err.message);
  });

  await page.goto('http://127.0.0.1:8099/reports', { waitUntil: 'networkidle' });
  await page.waitForTimeout(4000);

  // Count all SVG elements (charts render as SVG via uiw.Chart)
  const svgCount = await page.evaluate(() => document.querySelectorAll('svg').length);

  // Find charts with aria-label matching income bar
  const incomeBarLabel = await page.evaluate(() => {
    const charts = Array.from(document.querySelectorAll('[aria-label]'));
    const match = charts.find(el => el.getAttribute('aria-label') === 'Top income sources ranked by amount');
    return match ? match.getAttribute('aria-label') : null;
  });

  // Count how many chart wrappers reference "income" in their aria-label
  const incomeChartCount = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('[aria-label]'))
      .filter(el => el.getAttribute('aria-label').toLowerCase().includes('income'))
      .length;
  });

  // Check that the income section card exists (EntityListSection with "Income by source" title)
  const incomeSourceSectionExists = await page.evaluate(() => {
    const headings = Array.from(document.querySelectorAll('h2, h3, [class*="card-title"]'));
    return headings.some(h => h.textContent.trim().includes('Income by source'));
  });

  const screenshotDir = path.join(__dirname, 'screenshots');
  if (!fs.existsSync(screenshotDir)) fs.mkdirSync(screenshotDir, { recursive: true });
  await page.screenshot({ path: path.join(screenshotDir, 'reports_income_bar.png'), fullPage: true });

  console.log('=== MEASURED RESULTS ===');
  console.log(`SVG count: ${svgCount}`);
  console.log(`Income bar chart aria-label found: ${incomeBarLabel}`);
  console.log(`Charts with "income" in aria-label: ${incomeChartCount}`);
  console.log(`Income by source section exists: ${incomeSourceSectionExists}`);
  console.log(`JS error count: ${errors.length}`);
  if (errors.length > 0) {
    errors.forEach(e => console.log('  ERROR:', e));
  }

  await browser.close();

  process.exit(errors.length > 0 ? 1 : 0);
})();
