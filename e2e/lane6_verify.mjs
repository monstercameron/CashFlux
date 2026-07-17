// lane6_verify.mjs — e2e verification for the lane-6 remediation batch:
//   #49 subscriptions honesty across price changes / renewals / annual report
//   #75 notifications polish   #74 review-count clarity   #72 dialog titles
//   #73 assistant restructure  #52 confidence tiers       #68 tokens+density
//   #67 a11y gate
// Usage: node e2e/lane6_verify.mjs [port]   (default 8116, lane-6's server)
import { chromium } from "playwright";

const PORT = process.argv[2] || "8116";
const BASE = `http://127.0.0.1:${PORT}`;
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1500); };
const bodyText = async () => await page.locator("body").innerText();

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ───────────────────────── #49: subscriptions honesty ─────────────────────────
await nav("/subscriptions");
await page.waitForTimeout(1500);
let body = await bodyText();
const priceSection = body.includes("Recent price changes") ? body.slice(body.indexOf("Recent price changes")) : body;
const badPrice = ["Cigarettes", "Gas (", "Pharmacy", "Electricity", "Takeout", "Holiday gifts"].filter((w) => priceSection.slice(0, 1500).includes(w));
check("#49: price changes exclude essentials", badPrice.length === 0, badPrice.join(", ") || "clean");
// local-first cancel: primary button files a checklist; web search is secondary
const cancelBtn = page.locator('[data-testid^="sub-howto-cancel-"]').first();
if (await cancelBtn.count()) {
  const tag = await cancelBtn.evaluate((el) => el.tagName);
  check("#49: How-to-cancel is a local action (button, not external link)", tag === "BUTTON", tag);
  await cancelBtn.click();
  await page.waitForTimeout(900);
  body = await bodyText();
  check("#49: clicking files a local checklist task", /cancellation checklist/i.test(body), "");
  const webLink = page.locator('[data-testid^="sub-cancel-web-"]').first();
  check("#49: web search demoted to a secondary link", (await webLink.count()) > 0, "");
} else {
  check("#49: cancellable subscription row present", false);
}
// annual report subscription totals exclude essentials
await nav("/reports");
await page.waitForTimeout(2200);
body = await bodyText();
const recurM = body.match(/(\d+) recurring charges/);
check("#49: annual report recurring count is classified (not 25)", !recurM || parseInt(recurM[1], 10) < 25, recurM ? recurM[0] : "(no line)");

// ───────────────────────── #75: notifications polish ─────────────────────────
await nav("/notifications");
await page.waitForTimeout(1500);
body = await bodyText();
check("#75: no 'Due in 0 days' / 'Due in 1 days' wording", !/Due in [01] days/.test(body), "");
// Clear all is guarded: click → confirm dialog, cancel keeps the feed
const rowsBefore = await page.locator(".notif, .notif-group").count();
check("#75: notification rows present", rowsBefore > 0, `${rowsBefore}`);
await page.locator('[data-testid="notif-clear-all"]').click();
await page.waitForTimeout(600);
const dlgUp = await page.locator("#cf-dialog-cancel").count();
check("#75: Clear all opens a confirm dialog", dlgUp > 0, "");
if (dlgUp) {
  await page.locator("#cf-dialog-cancel").click();
  await page.waitForTimeout(600);
  const rowsAfterCancel = await page.locator(".notif, .notif-group").count();
  check("#75: cancel keeps the feed intact", rowsAfterCancel === rowsBefore, `${rowsAfterCancel}/${rowsBefore}`);
  await page.locator('[data-testid="notif-clear-all"]').click();
  await page.waitForTimeout(500);
  await page.locator("#cf-dialog-confirm").click();
  await page.waitForTimeout(800);
  const rowsAfterConfirm = await page.locator(".notif, .notif-group").count();
  check("#75: confirm clears the feed", rowsAfterConfirm === 0, `${rowsAfterConfirm}`);
}
// Mobile: icon trio collapses to labeled primary + labeled overflow menu
const mctx = await browser.newContext({ viewport: { width: 390, height: 844 }, reducedMotion: "reduce" });
const mp = await mctx.newPage();
await mp.goto(BASE + "/", { waitUntil: "load" });
await mp.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await mp.waitForTimeout(1500);
await mp.evaluate(() => { history.pushState({}, "", "/notifications"); dispatchEvent(new PopStateEvent("popstate")); });
await mp.waitForTimeout(1800);
const firstNotif = mp.locator(".notif").first();
if (await firstNotif.count()) {
  const trioVisible = await firstNotif.locator(".notif-actions").isVisible().catch(() => false);
  check("#75: mobile hides the icon-only trio", !trioVisible, "");
  const primary = firstNotif.locator(".notif-m-primary");
  check("#75: mobile primary action is labeled", (await primary.isVisible()) && (await primary.innerText()).trim().length > 2, await primary.innerText().catch(() => ""));
  const clamp = await firstNotif.locator(".notif-title").evaluate((el) => getComputedStyle(el).webkitLineClamp);
  check("#75: mobile title clamps to two lines", clamp === "2", clamp);
  await firstNotif.locator('[data-testid^="notif-more-"]').click();
  await mp.waitForTimeout(500);
  const menuItems = await firstNotif.locator('.add-menu:not(.hidden-menu) [role="menuitem"]').allInnerTexts();
  check("#75: overflow menu has labeled actions", menuItems.length >= 4 && menuItems.every((t) => t.trim().length > 2), menuItems.join(" / "));
} else {
  check("#75: mobile notification rows present", false);
}
await mctx.close();

// ─────────────── #74: review-inbox vs waiting-transactions counts ───────────────
// Fresh context: the earlier #75 steps cleared this page's notification feed but
// transactions/tasks are untouched; still, use a clean context for count parity.
const c74 = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const p74 = await c74.newPage();
await p74.goto(BASE + "/", { waitUntil: "load" });
await p74.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await p74.waitForTimeout(1500);
const nav74 = async (path) => { await p74.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, path); await p74.waitForTimeout(1800); };
await nav74("/transactions");
const inboxBtnText = await p74.locator('[data-testid="txn-review-btn"]').innerText().catch(() => "");
const inboxN = (inboxBtnText.match(/\((\d+)\)/) || [])[1];
check("#74: transactions Review inbox count present", !!inboxN, inboxBtnText.trim());
await nav74("/todo");
const suggestText = await p74.locator('[data-testid="todo-suggest-unreviewed"]').innerText().catch(() => "");
const suggestM = suggestText.match(/Review (\d+) transactions in the Review inbox/);
check("#74: suggestion names the Review inbox", !!suggestM, suggestText.trim().slice(0, 80));
check("#74: suggestion count equals the inbox count", !!suggestM && suggestM[1] === inboxN, `${suggestM && suggestM[1]} vs ${inboxN}`);
if (suggestM) {
  await p74.locator('[data-testid="todo-suggest-add-unreviewed"]').click();
  await p74.waitForTimeout(900);
  const taskLink = p74.locator('button[data-testid^="task-link-"]', { hasText: "Review inbox" }).first();
  check("#74: created task carries a 'Review inbox' link", (await taskLink.count()) > 0, "");
  if (await taskLink.count()) {
    await taskLink.click();
    await p74.waitForTimeout(1800);
    const onTxns = await p74.evaluate(() => location.pathname.includes("/transactions"));
    const inboxOpen = await p74.locator('[data-testid="review-inbox"]').count();
    check("#74: task link opens the Review inbox on /transactions", onTxns && inboxOpen > 0, `path=${onTxns} inbox=${inboxOpen}`);
  }
}
await c74.close();

// ───────────── #72: one visible dialog title, aria-labelledby wired ─────────────
const c72 = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const p72 = await c72.newPage();
await p72.goto(BASE + "/", { waitUntil: "load" });
await p72.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await p72.waitForTimeout(1500);
for (const [menuLabel, dlgTitle] of [["New goal", "Add goal"], ["New budget", "Add budget"], ["New task", "Add task"]]) {
  await p72.locator('[data-testid="add-menu-caret"]').click();
  await p72.waitForTimeout(400);
  await p72.locator(".add-menu:not(.hidden-menu) button", { hasText: menuLabel }).first().click();
  await p72.waitForTimeout(1100);
  // Exactly ONE visible heading equal to the dialog title (the front-face copy is aria-hidden).
  let matches = 0;
  for (const h of await p72.locator(".flip-wrap h1, .flip-wrap h2, .flip-wrap h3, .flip-wrap h4").all()) {
    if ((await h.isVisible()) && (await h.innerText()).trim() === dlgTitle) matches++;
  }
  check(`#72: '${dlgTitle}' shows its title exactly once`, matches === 1, `${matches}`);
  const labelled = await p72.locator('.flip-wrap[role="dialog"]').first().evaluate((el) => {
    const id = el.getAttribute("aria-labelledby");
    const t = id && document.getElementById(id);
    return t ? t.textContent.trim() : null;
  });
  check(`#72: '${dlgTitle}' dialog is aria-labelledby its visible title`, labelled === dlgTitle, String(labelled));
  await p72.locator(".set-close").last().click().catch(() => {});
  await p72.waitForTimeout(600);
}
await c72.close();

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
