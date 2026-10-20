import { test, expect, nav } from "./fixtures.mjs";

async function openIncomeBasis(page) {
  await nav(page, "/budgets");
  const details = page.getByTestId("budgets-status-details");
  if (!(await details.getAttribute("open"))) {
    await details.locator("summary").click();
  }
  await page.getByTestId("budgets-basis-open").click();
  await expect(page.getByTestId("budgets-zbb-basis")).toBeVisible();
}

test("income basis choices survive a page reload", async ({ app }) => {
  await openIncomeBasis(app);
  await app.getByTestId("budgets-zbb-income-mode").selectOption("paychecks");
  await app.getByTestId("budgets-zbb-paycheck-min").fill("1000");
  await app.getByTestId("budgets-zbb-average").check();
  await app.getByTestId("budgets-zbb-rollover").check();

  await app.getByTestId("flip-save").click();
  await expect(app.getByTestId("budgets-zbb-basis")).toHaveCount(0);
  await app.reload();
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true");

  await openIncomeBasis(app);
  await expect(app.getByTestId("budgets-zbb-income-mode")).toHaveValue("paychecks");
  await expect(app.getByTestId("budgets-zbb-paycheck-min")).toHaveValue("1000");
  await expect(app.getByTestId("budgets-zbb-average")).toBeChecked();
  await expect(app.getByTestId("budgets-zbb-rollover")).toBeChecked();
});
