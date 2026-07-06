// C280 verification: /members shows "Spending this period" section with per-member rows.
'use strict';
const { chromium } = require('C:\\Users\\mreca\\Desktop\\CashFlux\\.tools\\node_modules\\playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' && !/already exited/i.test(msg.text())) {
      errors.push(msg.text());
    }
  });

  await page.goto('http://127.0.0.1:8099/', { waitUntil: 'networkidle' });

  // Load sample data so members exist.
  const startFresh = await page.$('[data-testid="sample-start-fresh"]');
  if (startFresh) {
    await startFresh.click();
    await page.waitForTimeout(800);
  }

  // Navigate to /members.
  await page.goto('http://127.0.0.1:8099/members', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);

  // Screenshot.
  await page.screenshot({ path: 'e2e/screenshots/c280_members.png', fullPage: true });

  // Check "Spending this period" section heading.
  const bodyText = await page.evaluate(() => document.body.innerText);
  const hasSpendTitle = /spending this period/i.test(bodyText);
  console.log('Body text includes "Spending this period":', hasSpendTitle);

  // Check net worth section still present.
  const hasNetWorth = /net worth by member/i.test(bodyText);
  console.log('Body text includes "Net worth by member":', hasNetWorth);

  // Count spending rows (rows under the spending section).
  const allRows = await page.$$eval('.rows .row, [class*="row"]', rows => {
    return rows.filter(r => r.closest('[data-testid]') == null).length;
  });
  console.log('Total row elements on page:', allRows);

  console.log('JS errors:', errors.length, errors);

  if (!hasSpendTitle) {
    console.error('FAIL: "Spending this period" section not found on /members');
    process.exit(1);
  }
  if (!hasNetWorth) {
    console.error('FAIL: "Net worth by member" section missing (regression)');
    process.exit(1);
  }
  if (errors.length > 0) {
    console.error('FAIL: JS errors detected');
    process.exit(1);
  }

  console.log('PASS: C280 — per-member spending section present on /members');
  await browser.close();
})();
