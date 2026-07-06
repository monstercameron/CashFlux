// L26 E2E check — "task entity link". Adds a task linked to a Goal, asserts
// the persisted data carries relatedType/relatedId, and checks the row renders
// a deep-link button. Bonus: clicking it should change the route to /goals.
// Exits non-zero on any assertion failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(
  path.join(__dirname, "..", ".tools", "package.json")
);
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TITLE = "ZZLINK-" + Math.random().toString(36).slice(2, 7).toUpperCase();

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

/** Return the task record from localStorage by title (walks the whole dataset). */
const taskByTitle = (page, title) =>
  page.evaluate((t) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    let found = null;
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) return o.forEach(walk);
      if (o.title === t && o.status) found = o;
      Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  }, title);

/** Poll until pred(task) is true or timeout. */
async function waitForTask(page, title, pred, timeoutMs = 8000) {
  let t = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    t = await taskByTitle(page, title);
    if (pred(t)) return t;
    await page.waitForTimeout(400);
  }
  return t;
}

/** Return the first Goal from localStorage, or null. */
const firstGoal = (page) =>
  page.evaluate(() => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    // goals live at data.goals (array) in the CashFlux dataset shape.
    const goals = data.goals || [];
    return goals[0] || null;
  });

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60_000 });

  // Ensure there is at least one goal in the dataset; seed one if needed.
  let goal = await firstGoal(page);
  if (!goal) {
    // Navigate to goals screen and add one, then return to /todo.
    await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(800);
    const nameInput = page
      .locator('input[type="text"]')
      .first();
    if ((await nameInput.count()) > 0) {
      await nameInput.fill("Test Goal ZZLINK");
      await page.locator('button[type="submit"]').first().click();
      await page.waitForTimeout(600);
    }
    await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#task-add", { timeout: 60_000 });
    goal = await firstGoal(page);
  }
  if (!goal) {
    fail("no goal found in the dataset — cannot test entity link");
    await browser.close();
    process.exit(process.exitCode || 1);
  }

  // -- Add form: fill title, choose Goal in the "Link to" type selector. --
  await page.locator("#task-add").fill(TITLE);

  // Select "goal" from the "Link to" type dropdown (aria-label="Link to").
  const linkToSelect = page.locator('select[aria-label="Link to"]').first();
  if ((await linkToSelect.count()) === 0) {
    fail('could not find select[aria-label="Link to"] in the add form');
  } else {
    await linkToSelect.selectOption("goal");
    await page.waitForTimeout(300); // let entity select render

    // The entity sub-select should now be visible (aria-label="— Choose —").
    const entitySelect = page
      .locator('select[aria-label="— Choose —"]')
      .first();
    if ((await entitySelect.count()) === 0) {
      fail("entity sub-select did not appear after choosing Goal type");
    } else {
      // Pick the first goal by its id.
      await entitySelect.selectOption(goal.id);
      await page.waitForTimeout(200);
    }
  }

  // Submit.
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  // -- Assert the row appeared. --
  const row = page.locator(".row", { hasText: TITLE });
  if ((await row.count()) === 0) fail("task row did not appear after adding");

  // -- Assert persisted relatedType and relatedId. --
  const saved = await waitForTask(page, TITLE, (t) => !!t && !!t.relatedType);
  if (!saved) {
    fail("task not found in localStorage after submit");
  } else {
    if (saved.relatedType !== "goal") {
      fail(`relatedType = "${saved.relatedType}", want "goal"`);
    }
    if (!saved.relatedId || saved.relatedId !== goal.id) {
      fail(
        `relatedId = "${saved.relatedId}", want "${goal.id}"`
      );
    }
  }

  // -- Assert the row shows a deep-link button (aria-label contains "Go to linked item:"). --
  const linkBtn = row
    .locator('button[aria-label*="Go to linked item"]')
    .first();
  if ((await linkBtn.count()) === 0) {
    fail(
      'link button with aria-label "Go to linked item: …" not found in the task row'
    );
  }

  // -- Bonus: clicking the link button should navigate to /goals. --
  if ((await linkBtn.count()) > 0) {
    await linkBtn.click();
    await page.waitForTimeout(500);
    const url = page.url();
    if (!url.endsWith("/goals")) {
      fail(`after clicking link button, URL = "${url}", expected it to end with "/goals"`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      `PASS: added task "${TITLE}" linked to goal "${goal.id}"; ` +
        `persisted relatedType=goal + relatedId; ` +
        `link button rendered and navigated to /goals.`
    );
} finally {
  await browser.close();
}
