// lane2_verify62.mjs — e2e for #62 "Continue where you left off" resume card.
// Usage: node e2e/lane2_verify62.mjs [port]
import { chromium } from "playwright";
const PORT = process.argv[2] || "8112";
const BASE = `http://127.0.0.1:${PORT}`;
let allOK = true;
const check = (name, ok, detail = "") => { allOK = allOK && !!ok; console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`); };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" })).newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1500); };

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(5000);

// Card renders with the sample data's real unfinished work.
const card = page.locator('[data-testid="dash-resume-card"]');
check("#62 resume card renders", await card.count() === 1);
check("#62 review row present", await page.locator('[data-testid="resume-review"]').count() === 1);
check("#62 over-assignment row present", await page.locator('[data-testid="resume-overassign"]').count() === 1);

// Review jump-back: opens the Review inbox modal in place.
await page.locator('[data-testid="resume-review"] button').click();
await page.waitForTimeout(1200);
const inboxOpen = await page.evaluate(() => !!document.querySelector(".flip-panel, .flip-wrap"));
check("#62 review row opens the Review inbox", inboxOpen);
await page.keyboard.press("Escape");
await page.waitForTimeout(900);

// Over-assignment jump-back: lands on /budgets with the month-close flow open.
await page.locator('[data-testid="resume-overassign"] button').click();
await page.waitForTimeout(1800);
const budgets = await page.evaluate(() => ({ path: location.pathname, modal: !!document.querySelector(".flip-panel, .flip-wrap") }));
check("#62 over-assignment resolves into /budgets month-close", budgets.path.endsWith("/budgets") && budgets.modal, JSON.stringify(budgets));
await page.keyboard.press("Escape");
await page.waitForTimeout(600);

// Reconcile draft: save one via the #48 flow, then the dashboard offers Resume.
await nav("/accounts");
const startBtn = page.locator('[data-testid^="reconcile-start-btn-"]').first();
const acctID = (await startBtn.getAttribute("data-testid")).replace("reconcile-start-btn-", "");
const row = page.locator(`[data-testid="acct-row-${acctID}"]`);
await row.locator('button[aria-haspopup="menu"]').first().click();
await page.waitForTimeout(400);
await page.locator(`[data-testid="reconcile-start-btn-${acctID}"]`).click();
await page.waitForTimeout(900);
await page.locator('[data-testid="reconcile-statement-input"]').fill("999999.99");
await page.waitForTimeout(600);
await page.locator('[data-testid="reconcile-save-draft"]').click();
await page.waitForTimeout(900);
await nav("/");
await page.waitForTimeout(3500);
const reconRow = page.locator(`[data-testid="resume-reconcile-${acctID}"]`);
check("#62 saved reconcile draft surfaces a Resume row", await reconRow.count() === 1);
await reconRow.locator("button").click();
await page.waitForTimeout(1800);
check("#62 reconcile Resume lands on /accounts", await page.evaluate(() => location.pathname.endsWith("/accounts")));

// Dismiss is session-only and polite.
await nav("/");
await page.waitForTimeout(3500);
await page.locator('[data-testid="resume-dismiss"]').click();
await page.waitForTimeout(600);
check("#62 dismiss hides the card for the session", await card.count() === 0);

check("#62 zero page errors", errors.length === 0, errors.slice(0, 2).join(" | "));
await browser.close();
process.exit(allOK ? 0 : 1);
