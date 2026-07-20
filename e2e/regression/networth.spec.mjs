// networth.spec.mjs — the /networth balance-sheet surface.
//
// The page ships two readings of ONE computation (Glance | Detail), so the
// assertions that matter most are agreement assertions: the two views must
// quote the same headline figures, and THE BRIDGE must sum from its start value
// to its end value INCLUDING the residual it refuses to hide. Everything else
// here guards the honesty rules the redesign was built around: no silent caps,
// no bare ratios, a persisted view choice, and no horizontal overflow at any
// pane width.
import { test, expect, nav, mainText, settle, setTheme } from "./fixtures.mjs";

// money parses a formatted figure ("$304,000.00", "−$1,234.56", "+$12.00") into
// signed cents, so a test can do arithmetic on what the page actually rendered
// rather than on what it was supposed to render.
function money(text) {
  const t = String(text).trim();
  const neg = /^[−-]/.test(t) || /^\(.*\)$/.test(t);
  const digits = t.replace(/[^0-9.]/g, "");
  if (!digits) return null;
  const cents = Math.round(parseFloat(digits) * 100);
  return neg ? -cents : cents;
}

async function glance(page) {
  await page.locator('[data-testid="nws-view-glance"]').click();
  await expect(page.locator('[data-testid="nws-view-glance"]')).toHaveAttribute("aria-pressed", "true");
}

async function detail(page) {
  await page.locator('[data-testid="nws-view-detail"]').click();
  await expect(page.locator('[data-testid="nws-view-detail"]')).toHaveAttribute("aria-pressed", "true");
  // Detail's lower sections are deferred past first paint.
  await page.waitForFunction(
    () => document.querySelector('.nws[data-settled="true"]') !== null,
    null,
    { timeout: 30_000 },
  );
}

test("both views render and the toggle persists across a reload", async ({ app }) => {
  await nav(app, "/networth");
  // Glance is the default.
  await expect(app.locator('[data-testid="nws-view-glance"]')).toHaveAttribute("aria-pressed", "true");
  await expect(app.locator('[data-testid="nws-bridge-labels"]')).toBeVisible();

  await detail(app);
  await expect(app.locator('[data-testid="nws-index"]')).toBeVisible();
  await expect(app.locator("#nws-00")).toBeVisible();

  // The choice lives in the preserved settings bucket, so it survives a reload.
  await app.reload();
  await app.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 45_000 },
  );
  await nav(app, "/networth");
  await expect(app.locator('[data-testid="nws-view-detail"]')).toHaveAttribute("aria-pressed", "true");

  // And back, so the test leaves no sticky state behind for the reader.
  await glance(app);
});

test("Glance and Detail agree on the headline figures", async ({ app }) => {
  await nav(app, "/networth");
  const hero = money(await app.locator('[data-testid="nw-hero-value"]').innerText());
  const assets = money(await app.locator('[data-testid="nws-assets"]').innerText());
  const liabilities = money(await app.locator('[data-testid="nws-liabilities"]').innerText());
  expect(hero).not.toBeNull();
  // The hero's own three figures must be internally consistent.
  expect(hero).toBe(assets - liabilities);

  await detail(app);
  const detailNet = money(await app.locator('[data-testid="nws-detail-net"]').innerText());
  expect(detailNet).toBe(hero);

  // §00 restates both sides; they must match the hero's, to the cent.
  const standRows = app.locator('[data-testid="nws-stand-table"] tbody tr');
  expect(money(await standRows.nth(0).locator("td").nth(1).innerText())).toBe(assets);
  expect(money(await standRows.nth(1).locator("td").nth(1).innerText())).toBe(liabilities);

  // The bridge's end value is the same net worth again.
  expect(money(await app.locator('[data-testid="nws-detail-bridge-end"]').innerText())).toBe(hero);
});

test("the bridge sums from start to end INCLUDING the residual", async ({ app }) => {
  await nav(app, "/networth");
  await detail(app);

  const rows = app.locator('[data-testid="nws-leg-row"]');
  const count = await rows.count();
  expect(count).toBeGreaterThanOrEqual(3); // start + at least one leg + end

  let start = null;
  let end = null;
  let legSum = 0;
  let sawResidual = false;
  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const leg = await row.getAttribute("data-leg");
    const amount = money(await row.locator("td").last().innerText());
    if (leg === "start") { start = amount; continue; }
    if (leg === "end") { end = amount; continue; }
    if (leg === "residual") sawResidual = true;
    legSum += amount;
  }
  expect(start).not.toBeNull();
  expect(end).not.toBeNull();
  // The residual is ALWAYS listed, even at zero — that is the honesty rule.
  expect(sawResidual, "the residual leg must be shown, never absorbed").toBe(true);
  expect(start + legSum, "start + every leg must land exactly on end").toBe(end);
});

test("the Glance bridge shows the same legs and discloses its axis floor", async ({ app }) => {
  await nav(app, "/networth");
  const amounts = app.locator('[data-testid="nws-bridge-labels"] [data-testid="nws-bridge-amount"]');
  expect(await amounts.count()).toBeGreaterThanOrEqual(3);
  // The residual column is present in the graphic too.
  await expect(
    app.locator('[data-testid="nws-bridge-labels"] [data-leg="residual"]'),
  ).toHaveCount(1);
  // A truncated vertical axis is disclosed rather than smuggled.
  await expect(app.locator('[data-testid="nws-bridge-floor"]')).toBeVisible();
});

test("no silent caps: every account is listed with an honest total", async ({ app }) => {
  await nav(app, "/networth");
  const text = await mainText(app);
  expect(text, "the old '+N more accounts' cap is banned").not.toMatch(/\+\s*\d+\s*more account/i);

  await detail(app);
  const rows = await app.locator('[data-testid="nw-acct-row"]').count();
  expect(rows).toBeGreaterThan(0);
  // §02 and §03 each state how many accounts they list; the two claims must add
  // up to the number of rows actually rendered.
  const claims = await app.locator("#nws-02, #nws-03").allInnerTexts();
  let claimed = 0;
  for (const c of claims) {
    const m = c.match(/All\s+(\d+)\s+accounts/i);
    if (m) claimed += Number(m[1]);
  }
  expect(claimed).toBe(rows);

  // The movers table likewise claims a count and then lists exactly that many.
  const moversText = await app.locator("#nws-01").innerText();
  const mm = moversText.match(/All\s+(\d+)\s+accounts that moved/i);
  if (mm) {
    expect(await app.locator('[data-testid="nws-mover-row"]').count()).toBe(Number(mm[1]));
  }
});

test("every ratio carries an interpretation, not a bare percentage", async ({ app }) => {
  await nav(app, "/networth");
  await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });
  for (const id of ["nws-ratio-liquid", "nws-ratio-runway", "nws-ratio-debt"]) {
    const card = app.locator(`[data-testid="${id}"]`);
    await expect(card).toBeVisible();
    const read = await card.locator(".nws-ratio-read").innerText();
    // A reading is a sentence about the number, not the number again.
    expect(read.trim().length, `${id} must explain its figure`).toBeGreaterThan(20);
  }
  // Debt is structural: the liabilities side must not be painted in the alarm
  // colour just for existing.
  const debtCard = app.locator('[data-testid="nws-ratio-debt"]');
  const cls = await debtCard.getAttribute("class");
  const pct = Number((await debtCard.locator(".nws-ratio-value").innerText()).replace(/[^0-9]/g, ""));
  if (pct <= 80) expect(cls).not.toContain("is-alarm");
});

test("the Detail section chips jump to their sections", async ({ app }) => {
  await nav(app, "/networth");
  await detail(app);
  for (const num of ["00", "01", "02", "03", "04", "05"]) {
    await expect(app.locator(`[data-testid="nws-idx-${num}"]`)).toBeVisible();
    await expect(app.locator(`#nws-${num}`)).toBeAttached();
  }
  // Clicking a chip brings its section into the viewport.
  await app.locator('[data-testid="nws-idx-04"]').click();
  await expect(app.locator("#nws-04")).toBeInViewport({ ratio: 0.1, timeout: 10_000 });
});

test("the preserved testids are present on their equivalent controls", async ({ app }) => {
  await nav(app, "/networth");
  // Glance carries the page-level drills and the metrics toggle.
  for (const id of ["nw-hero-value", "nw-delta", "nw-toggle-formulas"]) {
    await expect(app.locator(`[data-testid="${id}"]`)).toBeVisible();
  }
  await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });
  for (const id of ["nw-takeaway", "nw-accounts-link", "nw-debt-link"]) {
    await expect(app.locator(`[data-testid="${id}"]`)).toBeVisible();
  }
  // Detail carries the section-level drills and the per-account rows.
  await detail(app);
  for (const id of ["networth-drill", "nw-owe-drill", "nw-acct-row"]) {
    await expect(app.locator(`[data-testid="${id}"]`).first()).toBeVisible();
  }
});

test("both signature graphics render in both themes with no horizontal overflow", async ({ app }) => {
  for (const width of [1442, 1202, 950]) {
    await app.setViewportSize({ width, height: 900 });
    for (const mode of ["dark", "light"]) {
      await setTheme(app, mode);
      await nav(app, "/networth");
      await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });
      await settle(app);
      await expect(app.locator('[data-testid="nws-sides-svg"]')).toBeVisible();
      // At the narrow pane the waterfall changes FORM (a stacked list of the same
      // legs) rather than clipping; at wide panes the bars are the graphic.
      const bars = app.locator('[data-testid="nws-bridge-svg"]');
      const stack = app.locator('[data-testid="nws-bridge-stack"]');
      expect(
        (await bars.isVisible()) || (await stack.isVisible()),
        `the bridge must render in some form at ${width}px`,
      ).toBe(true);
      const overflow = await app.evaluate(() => {
        const el = document.querySelector("#main");
        return el.scrollWidth - el.clientWidth;
      });
      expect(overflow, `horizontal overflow at ${width}px in ${mode}`).toBeLessThanOrEqual(1);
    }
  }
  await setTheme(app, "dark");
  await app.setViewportSize({ width: 1440, height: 900 });
});

test("Two sides measures the gap, and its endpoints match the history", async ({ app }) => {
  await nav(app, "/networth");
  await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });

  // The graphic's subject is the GAP, so it states the gap at both ends rather
  // than leaving the reader to eyeball a wedge.
  const ends = app.locator('[data-testid="nws-gap-ends"] .nws-gap-value');
  await expect(ends).toHaveCount(2);
  const gapWas = money(await ends.nth(0).innerText());
  const gapNow = money(await ends.nth(1).innerText());
  expect(gapWas).not.toBeNull();
  expect(gapNow).not.toBeNull();

  // A truncated axis is disclosed in words, the same rule the bridge follows.
  await expect(app.locator('[data-testid="nws-sides-floor"]')).toBeVisible();

  // Composition is carried by the strips, at exact figures, normalized within
  // each side — so a $304k holding cannot flatten the other side's bars.
  await expect(app.locator('[data-testid="nws-strip"]')).toHaveCount(2);

  // The endpoints must equal the first and last net worth in the history table:
  // the chart and the figures behind it are the same numbers.
  await detail(app);
  const rows = app.locator('[data-testid="nws-history-row"]');
  const n = await rows.count();
  expect(n).toBeGreaterThanOrEqual(2);
  expect(money(await rows.nth(0).locator("td").nth(3).innerText())).toBe(gapWas);
  expect(money(await rows.nth(n - 1).locator("td").nth(3).innerText())).toBe(gapNow);

  // And the gap's growth must equal the bridge's total movement, or the two
  // signature graphics would be telling different stories about one window.
  const start = money(await app.locator('[data-testid="nws-leg-row"][data-leg="start"] td').last().innerText());
  const end = money(await app.locator('[data-testid="nws-detail-bridge-end"]').innerText());
  expect(gapNow - gapWas).toBe(end - start);
});

test("Two sides is readable without prior knowledge: axes, region names, values", async ({ app }) => {
  await nav(app, "/networth");
  await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });

  // A dollar axis with real values, including the floor it was scaled to — a
  // chart that starts somewhere other than zero must say where, ON the scale.
  const yticks = app.locator('[data-testid="nws-yaxis"] .nws-ytick');
  expect(await yticks.count()).toBeGreaterThanOrEqual(3);
  for (const t of await yticks.allInnerTexts()) {
    expect(t.trim(), "every y tick carries a currency value").toMatch(/[$€£¥]/);
  }
  await expect(app.locator('[data-testid="nws-yaxis"] .nws-ytick.is-floor')).toHaveCount(1);

  // A dated x axis with more than just its two ends.
  const xticks = app.locator('[data-testid="nws-xaxis"] .nws-xtick');
  expect(await xticks.count()).toBeGreaterThanOrEqual(3);

  // The two halves are NAMED where they sit, each with its current figure, and
  // the net worth is called out between them.
  const annos = app.locator('[data-testid="nws-annos"] .nws-anno');
  await expect(annos).toHaveCount(3);
  const text = (await annos.allInnerTexts()).join(" | ");
  expect(text).toMatch(/own/i);
  expect(text).toMatch(/owe/i);
  expect(text).toMatch(/net worth/i);

  // The in-chart figures must be the same numbers the hero prints, or the
  // labels would be decoration rather than a reading of the chart.
  const heroAssets = money(await app.locator('[data-testid="nws-assets"]').innerText());
  const heroLiab = money(await app.locator('[data-testid="nws-liabilities"]').innerText());
  const heroNet = money(await app.locator('[data-testid="nw-hero-value"]').innerText());
  const annoVals = await annos.locator(".nws-anno-value").allInnerTexts();
  expect(annoVals.map(money)).toEqual([heroAssets, heroNet, heroLiab]);

  // The labels must not collide even when the two boundaries run close.
  const tops = await annos.evaluateAll((els) => els.map((e) => e.getBoundingClientRect().top));
  for (let i = 1; i < tops.length; i++) {
    expect(tops[i] - tops[i - 1], "in-chart labels must stay legibly apart").toBeGreaterThan(20);
  }
});

test("both signature graphics carry a keyboard-reachable ? explainer in plain language", async ({ app }) => {
  await nav(app, "/networth");
  await app.waitForFunction(() => document.querySelector('.nws[data-settled="true"]') !== null, null, { timeout: 30_000 });

  for (const id of ["nws-explain-bridge", "nws-explain-sides"]) {
    const btn = app.locator(`[data-testid="${id}-btn"]`);
    const pop = app.locator(`[data-testid="${id}-pop"]`);
    await expect(btn).toBeVisible();
    // Labelled for assistive tech, and announced as opening a dialog.
    await expect(btn).toHaveAttribute("aria-haspopup", "dialog");
    await expect(btn).toHaveAttribute("aria-expanded", "false");
    expect((await btn.getAttribute("aria-label"))?.length ?? 0).toBeGreaterThan(5);

    // Reachable and operable from the keyboard alone.
    await btn.focus();
    await expect(btn).toBeFocused();
    await app.keyboard.press("Enter");
    await expect(btn).toHaveAttribute("aria-expanded", "true");
    await expect(pop).toBeVisible();

    // Plain language, and enough of it to actually teach the picture.
    const body = await pop.innerText();
    expect(body.length, `${id} must explain the picture`).toBeGreaterThan(120);
    // It explains what you SEE, not how it is computed.
    expect(body).not.toMatch(/residual|minor units|attribution|engine|algorithm/i);

    // Escape closes it, per the app's popover convention.
    await app.keyboard.press("Escape");
    await expect(btn).toHaveAttribute("aria-expanded", "false");
  }

  // The Two sides explainer must actually say the thing a layman needs.
  await app.locator('[data-testid="nws-explain-sides-btn"]').click();
  const sides = (await app.locator('[data-testid="nws-explain-sides-pop"]').innerText()).toLowerCase();
  expect(sides).toContain("own");
  expect(sides).toContain("owe");
  expect(sides).toContain("net worth");
});
