import { test, expect } from "./fixtures.mjs";

// TODOS.md C701: the operator console's user rows.
//
// The row used to BE the affordance — `tr[role=button][tabindex=0]` with a click
// handler and no key handler. role=button on a non-button element does not bring
// keyboard activation with it, so Enter and Space did nothing; the only way into
// a user's detail view was a mouse click on a row with no visible indication
// that it was clickable. This covers both activation paths against the real
// console, which is the only way to catch a regression back to a fake button.

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const BACKEND = `http://127.0.0.1:${RPC_PORT}`;
const OWNER_USERNAME = "cashflux-owner";
const OWNER_PASSWORD = "CashFlux-Owner-6291";
const ROW_USERNAME = "row-e2e-user";
const ROW_PASSWORD = "CashFlux-Row-5518";

async function signInToConsole(page) {
  await page.goto(`${BACKEND}/console/`);
  const setup = page.getByRole("heading", { name: "Create owner account" });
  const signIn = page.getByTestId("admin-credential-signin");
  await expect(setup.or(signIn)).toBeVisible();
  if (await setup.isVisible()) {
    await page.getByTestId("admin-setup-key").fill("e2e-worker-token");
    await page.getByTestId("admin-setup-username").fill(OWNER_USERNAME);
    await page.getByTestId("admin-setup-password").fill(OWNER_PASSWORD);
    await page.getByTestId("admin-setup-password-confirm").fill(OWNER_PASSWORD);
    await page.getByTestId("admin-setup-submit").click();
    await page.getByTestId("admin-setup-continue").click();
  } else {
    await page.getByTestId("admin-username").fill(OWNER_USERNAME);
    await page.getByTestId("admin-password").fill(OWNER_PASSWORD);
    await signIn.click();
  }
  await expect(page.getByRole("heading", { name: "Operator Console" })).toBeVisible({ timeout: 20_000 });
}

async function ensureUser(page) {
  const existing = page.getByRole("button", { name: `Manage ${ROW_USERNAME}` });
  if (await existing.isVisible().catch(() => false)) return;
  await page.getByRole("button", { name: "Create a user account" }).click();
  await page.getByLabel("Username").fill(ROW_USERNAME);
  await page.getByLabel("Temporary password").fill(ROW_PASSWORD);
  await page.getByRole("button", { name: "Create account" }).click();
  // Back to the console list.
  const back = page.getByRole("button", { name: "← Back" });
  if (await back.isVisible().catch(() => false)) await back.click();
  await expect(existing).toBeVisible({ timeout: 20_000 });
}

test.describe("operator console user rows", () => {
  test("a user row opens by mouse and by keyboard, and shows a visible action", async ({ page }) => {
    await signInToConsole(page);
    await ensureUser(page);

    const manage = page.getByRole("button", { name: `Manage ${ROW_USERNAME}` });

    // The affordance is VISIBLE. Before C701 the row carried an aria-label and
    // nothing a sighted operator could see, so "rows are clickable" was
    // something you had to already know.
    await expect(manage).toBeVisible();
    await expect(manage).toHaveText("Manage");

    // Keyboard activation. This is the path that did not work at all: focus the
    // control and press Enter.
    await manage.focus();
    await expect(manage).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "Manage user" })).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId("admin-access-panel").or(page.getByTestId("admin-access-loading")))
      .toBeVisible({ timeout: 20_000 });

    await page.getByRole("button", { name: "← Back" }).click();
    await expect(page.getByRole("heading", { name: "Operator Console" })).toBeVisible();

    // Mouse activation still works.
    await page.getByRole("button", { name: `Manage ${ROW_USERNAME}` }).click();
    await expect(page.getByRole("heading", { name: "Manage user" })).toBeVisible({ timeout: 20_000 });
  });

  test("the detail view reports a failed load instead of loading for ever", async ({ page }) => {
    await signInToConsole(page);
    await ensureUser(page);
    // Make the detail request fail. The old view left "Loading…" on screen with
    // the real reason in a banner elsewhere, which reads as a slow request.
    await page.route("**/v1/admin/users/*", (route) => {
      if (route.request().method() === "GET") return route.fulfill({ status: 500, body: "{}" });
      return route.continue();
    });
    await page.getByRole("button", { name: `Manage ${ROW_USERNAME}` }).click();
    await expect(page.getByTestId("admin-user-detail-error")).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId("admin-user-detail-retry")).toBeVisible();
  });
});
