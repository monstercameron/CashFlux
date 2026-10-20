// Recurring bills on Budgets: verify the sample household's accounting and
// the path from an unbudgeted recurring category to a saved budget.
import { test, expect, nav } from "./fixtures.mjs";

// The generation marker lives in protected IndexedDB storage, not localStorage.
// Observe its encrypted record as a write-complete signal without reading data.
async function savedGenerationRecord(page) {
  return page.evaluate(async () => {
    const names = (await indexedDB.databases())
      .map(({ name }) => name)
      .filter((name) => name?.startsWith("cashflux-kv"))
      .sort((a, b) => Number(b.includes("-v2:")) - Number(a.includes("-v2:")));
    for (const name of names) {
      const value = await new Promise((resolve) => {
        const open = indexedDB.open(name);
        open.onerror = () => resolve(null);
        open.onsuccess = () => {
          const db = open.result;
          if (!db.objectStoreNames.contains("kv")) { db.close(); resolve(null); return; }
          const get = db.transaction("kv", "readonly").objectStore("kv").get("cashflux:dataset:gen");
          get.onsuccess = () => { db.close(); resolve(get.result ?? null); };
          get.onerror = () => { db.close(); resolve(null); };
        };
      });
      if (value != null) return JSON.stringify(value);
    }
    return null;
  });
}

async function openRecurring(page) {
  const section = page.getByTestId("budgets-recurring");
  const toggle = page.getByTestId("budgets-recurring-toggle");
  await section.scrollIntoViewIfNeeded();
  // The wasm view may still be settling after route navigation; retry the
  // disclosure click only while it remains closed.
  await expect.poll(async () => {
    if (await toggle.getAttribute("aria-expanded") === "true") return true;
    await toggle.click();
    return await toggle.getAttribute("aria-expanded") === "true";
  }).toBe(true);
  return section;
}

test("recurring totals reconcile and Budget this moves a category into coverage", async ({ app }) => {
  await nav(app, "/budgets");
  const section = await openRecurring(app);

  await expect(section).toContainText("12 bills");
  await expect(section).toContainText("$3,926.99");
  await expect(app.getByTestId("budgets-recurring-covered-total")).toHaveText("$1,138.00");
  await expect(app.getByTestId("budgets-recurring-unbudgeted-total")).toHaveText("($2,788.99/mo)");
  await expect(app.getByTestId("budgets-recurring-row")).toHaveCount(3);
  await expect(app.getByTestId("budgets-recurring-unbudgeted-row")).toHaveCount(8);

  const savedBefore = await savedGenerationRecord(app);
  await app.getByTestId("budget-this-recurring-cat-business-expense").click();
  const form = app.getByTestId("budget-add-form");
  await expect(form).toBeVisible();
  await expect(form.locator("#budget-add")).toHaveValue("Business expenses");
  await expect(form.getByTestId("budget-add-limit")).toHaveValue("39.00");
  await expect(form.getByTestId("budget-existing-cat")).toContainText("Business expenses");
  await form.getByTestId("budget-add-submit").click();
  await expect(form).toHaveCount(0);

  await expect(app.getByTestId("budgets-recurring-covered-total")).toHaveText("$1,177.00");
  await expect(app.getByTestId("budgets-recurring-unbudgeted-total")).toHaveText("($2,749.99/mo)");
  await expect(app.getByTestId("budget-this-recurring-cat-business-expense")).toHaveCount(0);
  await expect.poll(() => savedGenerationRecord(app), {
    timeout: 15_000,
  }).not.toBe(savedBefore);

  await app.reload();
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await openRecurring(app);
  await expect(app.getByTestId("budgets-recurring-covered-total")).toHaveText("$1,177.00");
  await expect(app.getByTestId("budgets-recurring-unbudgeted-total")).toHaveText("($2,749.99/mo)");
});
