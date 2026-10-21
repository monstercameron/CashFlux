// sync.spec.mjs — guards Settings -> Cloud against ever again showing a sync
// failure with no visible reason (TODOS.md C450: "why is there no fold or
// pocket where I can see the sync error"), and guards /sync's redirect
// (2026-07-24 unification: /sync used to be a second, drifted implementation
// of this exact page — it is now a plain redirect to /settings/cloud, kept
// routable only so old bookmarks/links don't 404). The hermetic e2e server has
// no real backend behind it, so pointing sync at any address deterministically
// fails to dial — exactly the real-world "backend unavailable" class of error
// this guards, with zero mocking required.
import { test, expect, nav } from "./fixtures.mjs";

// UNREACHABLE is a loopback port nothing listens on in the hermetic test
// environment, so connecting to it fails fast (connection refused) rather than
// timing out — keeping the test quick without relying on a live server.
const UNREACHABLE = "http://127.0.0.1:8199/";

test.describe("sync error visibility", () => {
  test("/sync redirects to Settings -> Cloud", async ({ app }) => {
    await app.evaluate(() => {
      history.pushState({}, "", "/sync");
      dispatchEvent(new PopStateEvent("popstate"));
    });
    await expect(app.locator('#main[data-route="/settings"]').first()).toBeVisible();
    await expect.poll(() => app.evaluate(() => location.pathname)).toBe("/settings/cloud");
    await expect(app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" })).toHaveAttribute("aria-checked", "true");
  });

  test("the guided setup blocks first sync when the connection test fails", async ({ app }) => {
    // /settings/:tab shares one route pattern (ActivePath stays the constant
    // "/settings" for Sidebar/data-route — see ShellProps.ContentKey in
    // shell.go) so nav()'s data-route wait can't target "/settings/cloud"
    // directly; land on /settings and click the tab like a real user would.
    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
    await app.getByTestId("cloud-setup-start").click();
    await expect(app.getByTestId("cloud-setup-where")).toBeVisible();
    await app.getByTestId("cloud-server-options").click();
    await app.getByRole("radio", { name: "Your own server", exact: true }).click();
    await app.locator('[data-testid="sync-server-url"]').fill(UNREACHABLE);
    await app.getByTestId("sync-test").click();
    await expect(app.getByTestId("sync-discovery-error")).toBeVisible({ timeout: 20_000 });
    await expect(app.getByTestId("sync-discovery-error")).toContainText(/Couldn't reach a backend|connection to .* failed/i);
    await expect(app.getByTestId("cloud-where-continue")).toBeDisabled();
    await expect(app.getByTestId("cloud-start-encrypted-sync")).toHaveCount(0);
  });
});
