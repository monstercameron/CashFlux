// category_merge.spec.mjs — folding one category into another (C523).
//
// The reason this exists at all: enforcing unique sibling names (C537) is only
// reasonable if there is a way to resolve the duplicates a household already
// has. Merge is that way, and its correctness claim is total — after a merge,
// NOTHING anywhere still points at the retired category.
import { test, expect, nav } from "./fixtures.mjs";

// Category names contain characters that are regex-significant ("Baby & Childcare",
// "Dining / takeout"), so any name used in a pattern has to be escaped.
const escapeRe = (s) => s.replace(/[-[\]{}()*+?.,\\^$|#\s]/g, "\\$&");

// pickTarget chooses the first real destination and confirms the choice stuck.
//
// Reading an option value and then selecting it in two steps is a race here: the
// destination list is filtered by the SOURCE category's kind, so choosing a
// source re-renders the options and the value read a moment earlier may no
// longer exist ("did not find some options"). Read and select together, then
// verify, and retry if the list moved underneath.
async function pickTarget(app) {
  const target = app.getByTestId("cats-move-target");
  await expect
    .poll(
      async () => {
        const values = (await target.locator("option").evaluateAll((os) => os.map((o) => o.value))).filter(
          (v) => v && v !== "__pick__",
        );
        if (!values.length) return "";
        try {
          await target.selectOption(values[0], { timeout: 3000 });
        } catch {
          return "";
        }
        return await target.inputValue();
      },
      { timeout: 25000 },
    )
    .not.toBe("");
  return await target.inputValue();
}

async function openMergePanel(app) {
  await nav(app, "/categories");
  // Wait for the ledger itself before clicking: the page mounts its chrome
  // before the category rows arrive, so a click landing in that gap sets state
  // on a component that is about to be re-rendered and the panel never appears.
  await expect(app.locator('[data-testid^="cat-menu-btn-"]').first()).toBeVisible();
  // ...and wait for the list to STOP changing. The first visit is still settling
  // its dataset, and a re-render mid-flight discards the panel state the click
  // just set — which is why the first attempt failed and a retry passed.
  await expect
    .poll(
      async () => {
        const a = await app.locator('[data-testid^="cat-menu-btn-"]').count();
        await app.waitForTimeout(250);
        const b = await app.locator('[data-testid^="cat-menu-btn-"]').count();
        return a === b && a > 1;
      },
      { timeout: 20000 },
    )
    .toBe(true);
  const panel = app.getByTestId("cats-move-panel");
  await expect
    .poll(
      async () => {
        if (await panel.count()) return true;
        await app.getByTestId("categories-merge").click();
        await panel.waitFor({ state: "visible", timeout: 2000 }).catch(() => {});
        return (await panel.count()) > 0;
      },
      { timeout: 20000 },
    )
    .toBe(true);
  await expect(panel).toHaveAttribute("data-mode", "merge");
  return panel;
}

test.describe("merging categories", () => {
  test("the panel asks for both categories and states what will move", async ({ app }) => {
    await openMergePanel(app);

    // Opened from the page, so neither category is chosen yet.
    const source = app.getByTestId("cats-merge-source");
    await expect(source).toBeVisible();
    const names = await source.locator("option").allInnerTexts();
    expect(names.length).toBeGreaterThan(2);

    const sourceValue = await source.locator("option").nth(1).getAttribute("value");
    await source.selectOption(sourceValue);

    await expect(app.getByTestId("cats-move-target")).toBeVisible();
    await pickTarget(app);

    // The preview names WHAT moves, not just how much: "moves 12 things" is not a
    // number anyone can check.
    const preview = app.getByTestId("cats-merge-preview");
    await expect(preview).toBeVisible();
    await expect(preview).toContainText(/Moves \d+ things into/i);
  });

  test("merging retires the source and reports what moved", async ({ app }) => {
    await openMergePanel(app);
    const before = await app.locator('[data-testid^="cat-menu-btn-"]').count();
    expect(before).toBeGreaterThan(1);

    const source = app.getByTestId("cats-merge-source");
    const sourceValue = await source.locator("option").nth(1).getAttribute("value");
    const sourceName = (await source.locator("option").nth(1).innerText()).trim();
    await source.selectOption(sourceValue);

    await pickTarget(app);
    // The preview only renders once BOTH categories are chosen, so it is the
    // readiness signal for the confirm. Clicking straight after selectOption
    // races the selection across the wasm boundary and confirms an empty target.
    await expect(app.getByTestId("cats-merge-preview")).toBeVisible();
    await app.getByTestId("cats-move-confirm").click();

    // The merged category is gone. Asserting "one row fewer" would be wrong:
    // merging a PARENT re-homes its children onto the survivor, and a collapsed
    // child becoming visible changes the row count by something other than one.
    // What must be true is that this name is no longer in the list.
    await expect
      .poll(async () => await app.locator(".row-desc").filter({ hasText: new RegExp(`^${escapeRe(sourceName)}$`) }).count(), {
        timeout: 15000,
      })
      .toBe(0);
    expect(before).toBeGreaterThan(1);
    // The confirmation names the category by NAME. It stops existing as part of
    // the merge, so a lookup made afterwards yields a raw id — which is exactly
    // what the first cut of this did.
    await expect(app.locator("body")).toContainText(new RegExp(`Merged ${escapeRe(sourceName)} into`, "i"));
    await expect(app.locator("body")).toContainText(/references moved/i);
  });

  test("the merge is durable, not just a repaint", async ({ app }) => {
    await openMergePanel(app);
    const source = app.getByTestId("cats-merge-source");
    const sourceValue = await source.locator("option").nth(1).getAttribute("value");
    const sourceName = (await source.locator("option").nth(1).innerText()).trim();
    await source.selectOption(sourceValue);
    await pickTarget(app);
    // The preview only renders once BOTH categories are chosen, so it is the
    // readiness signal for the confirm. Clicking straight after selectOption
    // races the selection across the wasm boundary and confirms an empty target.
    await expect(app.getByTestId("cats-merge-preview")).toBeVisible();
    await app.getByTestId("cats-move-confirm").click();
    await expect(app.getByTestId("cats-move-panel")).toHaveCount(0);

    // Reload from the root: a merge that only repainted the list would come back
    // on the next boot, and a dangling reference does not fail loudly — it
    // produces totals that quietly stop adding up.
    await app.evaluate(() => history.pushState({}, "", "/"));
    await app.reload();
    await app.waitForFunction(
      () => document.documentElement.getAttribute("data-app-ready") === "true",
      null,
      { timeout: 45000 },
    );
    await nav(app, "/categories");
    await expect(app.locator('[data-testid^="cat-menu-btn-"]').first()).toBeVisible();
    await expect(app.locator(".row-desc").filter({ hasText: new RegExp(`^${escapeRe(sourceName)}$`) })).toHaveCount(0);
  });

  test("a merge cannot be confirmed without both categories", async ({ app }) => {
    await openMergePanel(app);
    // No source, no target: pressing Merge must explain rather than do nothing.
    await app.getByTestId("cats-move-confirm").click();
    await expect(app.locator(".notice-danger")).toBeVisible();
  });
});
