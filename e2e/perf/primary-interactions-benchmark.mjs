// Representative UI interaction benchmark for the nine primary CashFlux pages.
// This script does not build, start a server, submit Assistant prompts, or write
// results into the repository by default. Each sweep gets a new disposable
// Playwright context and the app's built-in sample workspace.
import { chromium } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { gzipSync } from "node:zlib";

const FIXED_NOW = "2026-07-01T12:00:00.000Z";
const ACTIONS = ["period-step", "search-and-date-filter", "account-expand", "budget-period-and-income-basis-modal", "goals-view-sort-expand", "todo-view-and-search", "notifications-triage-populated", "assistant-local-controls-and-typing", "reports-summary-full-export-scope"];

function parse(argv) {
  const o = { baseUrl: process.env.E2E_BASE_URL || "", label: "run", wasmFile: "", buildId: "", runs: 5, warmups: 1, out: "" };
  const fields = { "--base-url": "baseUrl", "--label": "label", "--wasm-file": "wasmFile", "--build-id": "buildId", "--runs": "runs", "--warmups": "warmups", "--out": "out" };
  for (let i = 0; i < argv.length; i++) {
    const [flag, inline] = argv[i].split(/=(.*)/s, 2);
    if (flag === "--help" || flag === "-h") {
      console.log("Usage: node e2e/perf/primary-interactions-benchmark.mjs --base-url http://127.0.0.1:PORT [--wasm-file PATH --build-id SHA256] [--label NAME] [--warmups 1] [--runs 5] [--out FILE]");
      process.exit(0);
    }
    if (!fields[flag]) throw new Error(`unknown option ${flag}`);
    o[fields[flag]] = inline ?? argv[++i];
    if (!o[fields[flag]]) throw new Error(`missing value for ${flag}`);
  }
  o.runs = Number(o.runs); o.warmups = Number(o.warmups);
  if (!o.baseUrl || !Number.isInteger(o.runs) || o.runs < 1 || !Number.isInteger(o.warmups) || o.warmups < 0) throw new Error("base-url, positive runs, and non-negative warmups are required");
  const u = new URL(o.baseUrl);
  if (u.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(u.hostname)) throw new Error("base-url must be loopback HTTP");
  if (Boolean(o.wasmFile) !== Boolean(o.buildId)) throw new Error("--wasm-file and --build-id must be supplied together");
  o.baseUrl = o.baseUrl.replace(/\/$/, "");
  return o;
}
const round = n => Number.isFinite(n) ? Math.round(n * 10) / 10 : null;
const median = xs => { const a = xs.filter(Number.isFinite).sort((x, y) => x - y); if (!a.length) return null; const m = Math.floor(a.length / 2); return round(a.length % 2 ? a[m] : (a[m - 1] + a[m]) / 2); };

async function main() {
  const opts = parse(process.argv.slice(2));
  let artifact = null;
  const delivery = { mode: opts.wasmFile ? "injected-gzip-fallback" : "server-negotiated", artifactPath: null, sha256: null, rawCount: 0, gzipCount: 0, brAttempts: [] };
  if (opts.wasmFile) {
    const artifactPath = path.resolve(process.cwd(), opts.wasmFile), bytes = readFileSync(artifactPath);
    if (bytes.length < 8 || !bytes.subarray(0, 4).equals(Buffer.from([0, 97, 115, 109])) || !bytes.subarray(4, 8).equals(Buffer.from([1, 0, 0, 0]))) throw new Error("invalid or unsupported WASM artifact");
    const sha = createHash("sha256").update(bytes).digest("hex");
    if (!/^[0-9a-f]{64}$/i.test(opts.buildId) || sha !== opts.buildId.toLowerCase()) throw new Error(`artifact SHA256 ${sha} does not match --build-id`);
    artifact = { gzip: gzipSync(bytes, { level: 9, mtime: 0 }) };
    Object.assign(delivery, { artifactPath, sha256: sha, byteLength: bytes.length, gzipByteLength: artifact.gzip.length });
  }
  const report = { schemaVersion: 1, label: opts.label, baseUrl: opts.baseUrl, fixedNow: FIXED_NOW, viewport: { width: 1440, height: 900 }, sampleConfirmed: false, wasmDelivery: delivery, sweeps: [], failures: [] };
  const browser = await chromium.launch({ args: ["--disable-gpu", "--enable-precise-memory-info"] });
  try {
    const total = opts.warmups + opts.runs;
    for (let sweep = 0; sweep < total; sweep++) {
      const measured = sweep >= opts.warmups;
      const context = await browser.newContext({ viewport: report.viewport, deviceScaleFactor: 1, reducedMotion: "reduce", serviceWorkers: "block", locale: "en-US", timezoneId: "America/New_York" });
      const page = await context.newPage();
      // Isolate the frozen app from dev-server reload notifications.
      await page.routeWebSocket(url => url.hostname === "127.0.0.1" || url.hostname === "localhost" || url.hostname === "[::1]", socket => socket.close());
      const origin = new URL(opts.baseUrl).origin;
      await page.route("**/*", route => {
        const u = new URL(route.request().url());
        if (!["localhost", "127.0.0.1", "[::1]"].includes(u.hostname)) return route.abort();
        if (artifact && u.origin === origin && route.request().method() === "GET") {
          if (u.pathname === "/bin/main.wasm") { delivery.rawCount++; return route.fulfill({ status: 404, headers: { "Cache-Control": "no-store" }, body: "" }); }
          if (u.pathname === "/bin/main.wasm.gz") { delivery.gzipCount++; return route.fulfill({ status: 200, headers: { "Content-Type": "application/gzip", "Cache-Control": "no-store" }, body: artifact.gzip }); }
          if (u.pathname === "/bin/main.wasm.br") { delivery.brAttempts.push(u.pathname); return route.abort(); }
        }
        return route.continue();
      });
      await page.clock.setFixedTime(new Date(FIXED_NOW));
      await page.addInitScript(() => {
        try { localStorage.setItem("cashflux:tx-filter", JSON.stringify({ periodPreset: "all-dates" })); } catch (_) {}
        window.__interactionBench = { longTasks: [], mutations: 0 };
        try { new PerformanceObserver(list => window.__interactionBench.longTasks.push(...list.getEntries().map(x => x.duration))).observe({ type: "longtask", buffered: true }); } catch (_) {}
      });
      const one = { index: sweep, measured, actions: [], setup: {} };
      try {
        await page.goto(`${opts.baseUrl}/`, { waitUntil: "commit", timeout: 60000 });
        await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60000 });
        await page.locator('[data-testid="sample-data-banner"]').waitFor({ state: "visible", timeout: 15000 });
        report.sampleConfirmed = true;
        if (artifact && (delivery.rawCount < sweep + 1 || delivery.gzipCount < sweep + 1 || delivery.rawCount !== delivery.gzipCount || delivery.brAttempts.length)) throw new Error(`WASM override delivery mismatch: ${JSON.stringify(delivery)}`);
        const run = async (name, route, fn) => {
          const rec = { name, route, status: "ok" };
          try {
            await navigate(page, opts.baseUrl, route);
            rec.contentSha256 = await page.locator("#main").evaluate(el => crypto.subtle.digest("SHA-256", new TextEncoder().encode((el.innerText || "").replace(/\s+/g, " ").trim())).then(d => [...new Uint8Array(d)].map(b => b.toString(16).padStart(2, "0")).join("")));
            rec.metrics = await measure(page, fn);
          } catch (e) { rec.status = "error"; rec.error = String(e).slice(0, 500); rec.diagnostic = await diagnose(page); }
          one.actions.push(rec);
        };
        await run("period-step", "/", async p => {
          const label = p.locator('[data-testid="period-pill"] .period-label'); await label.waitFor(); const before = (await label.innerText()).trim();
          await p.locator(".period-control .period-step").first().click(); await p.waitForFunction(v => document.querySelector('[data-testid="period-pill"] .period-label')?.innerText.trim() !== v, before); return { changed: true, direction: "previous", visiblePeriodLabelChanged: true };
        });
        await run("search-and-date-filter", "/transactions", async p => {
          const search = p.getByTestId("filter-toolbar-search"); await search.waitFor(); await armSearchCompletion(p); await search.fill("coffee");
          // Search is intentionally debounced by the app (150 ms on Transactions).
          // The observer is armed before fill, so even a fast busy true→false pair cannot be missed.
          await waitForSearchCompletion(p);
          const scope = p.getByTestId("txn-scopebar"), beforeScope = await scope.getAttribute("aria-label");
          const select = p.getByTestId("txn-period-select"); await select.selectOption("this-month");
          await p.waitForFunction(v => document.querySelector('[data-testid="txn-scopebar"]')?.getAttribute("aria-label") !== v, beforeScope, { timeout: 15000 });
          const rows = await p.locator(".txn-table tbody tr").count(); await armSearchCompletion(p); await search.fill("");
          await waitForSearchCompletion(p);
          const beforeRestore = await scope.getAttribute("aria-label"); await select.selectOption("all-dates");
          await p.waitForFunction(v => document.querySelector('[data-testid="txn-scopebar"]')?.getAttribute("aria-label") !== v, beforeRestore, { timeout: 15000 });
          return { rows, intentionalSearchDebounceMsPerTransition: 150, searchTransitions: 2, expectedDebounceWaitMs: 300, dateScopeChanged: true };
        });
        await run("account-expand", "/accounts", async p => {
          const b = p.locator('[data-testid^="acct-details-toggle-"]').first(); await b.waitFor(); const id = (await b.getAttribute("data-testid")).replace("acct-details-toggle-", "");
          await b.click(); await p.waitForFunction(x => document.querySelector(`[data-testid="acct-details-toggle-${x}"]`)?.getAttribute("aria-expanded") === "true", id); return { expanded: true };
        });
        await run("budget-period-and-income-basis-modal", "/budgets", async p => {
          const label = p.locator('[data-testid="period-pill"] .period-label'), before = (await label.innerText()).trim(); await p.locator(".period-control .period-step").first().click(); await p.waitForFunction(v => document.querySelector('[data-testid="period-pill"] .period-label')?.innerText.trim() !== v, before);
          // The basis control is inside the native More budget details disclosure.
          // Open it through its summary before clicking the real basis action.
          const details = p.getByTestId("budgets-status-details"); await details.locator("summary").click(); await p.waitForFunction(() => document.querySelector('[data-testid="budgets-status-details"]')?.open === true);
          const basis = p.getByTestId("budgets-basis-open"); await basis.waitFor({ state: "visible" }); await basis.click(); const modal = p.locator(".zbb-basis-modal"); await modal.waitFor({ state: "visible" }); const chars = await modal.innerText().then(x => x.length); await p.keyboard.press("Escape"); await modal.waitFor({ state: "hidden" }); return { modalTextChars: chars, direction: "previous", detailsDisclosureOpened: true };
        });
        await run("goals-view-sort-expand", "/goals", async p => {
          const goalsTab = p.getByTestId("goals-tab-goals"), earmarksTab = p.getByTestId("goals-tab-earmarks"); await goalsTab.waitFor(); await goalsTab.click(); await waitForSelectedControl(p, '[data-testid="goals-tab-goals"]');
          await earmarksTab.click(); const earmarksSelection = await waitForSelectedControl(p, '[data-testid="goals-tab-earmarks"]');
          await goalsTab.click(); const goalsSelection = await waitForSelectedControl(p, '[data-testid="goals-tab-goals"]'); const sort = p.getByTestId("goals-sort"); await sort.waitFor(); const sortBefore = await sort.inputValue(); const sortValues = await sort.locator("option").evaluateAll(options => options.map(option => option.value)); const sortTarget = sortValues.find(value => value !== sortBefore); if (!sortTarget) throw new Error("goals sort has no alternate option"); await sort.selectOption(sortTarget); await p.waitForFunction(value => document.querySelector('[data-testid="goals-sort"]')?.value === value, sortTarget);
          const b = p.locator('[data-testid^="goal-expand-"]').first(); await b.waitFor(); const expandTestId = await b.getAttribute("data-testid"), goalId = expandTestId.replace("goal-expand-", ""); await b.click(); const collapse = p.getByTestId(`goal-collapse-${goalId}`); await collapse.waitFor({ state: "visible" }); await p.waitForFunction(id => document.querySelector(`[data-testid="goal-collapse-${id}"]`)?.getAttribute("aria-expanded") === "true", goalId); return { expanded: true, goalId, expandControlReplacedByCollapseControl: true, earmarksSelectionState: earmarksSelection, goalsSelectionState: goalsSelection, sortChanged: sortBefore !== sortTarget };
        });
        await run("todo-view-and-search", "/todo", async p => {
          const board = p.getByTestId("todo-view-board");
          if (await board.getAttribute("aria-pressed") === "true") { await p.getByTestId("todo-view-list").click(); await p.waitForFunction(() => document.querySelector('[data-testid="todo-view-list"]')?.getAttribute("aria-pressed") === "true"); }
          await board.click(); await p.waitForFunction(() => document.querySelector('[data-testid="todo-view-board"]')?.getAttribute("aria-pressed") === "true");
          const search = p.getByTestId("filter-toolbar-search"); await search.waitFor(); await armSearchCompletion(p); await search.fill("baby"); await waitForSearchCompletion(p);
          const chars = await p.locator("#main").innerText().then(x => x.length); await armSearchCompletion(p); await search.fill(""); await waitForSearchCompletion(p);
          await p.getByTestId("todo-view-list").click(); await p.waitForFunction(() => document.querySelector('[data-testid="todo-view-list"]')?.getAttribute("aria-pressed") === "true"); return { textChars: chars, pendingStateObserved: true, viewChanged: true };
        });
        try {
        await navigate(page, opts.baseUrl, "/notifications");
        const initialNotificationCounts = {};
        for (const view of ["live", "watching", "history"]) {
          await page.getByTestId(`notif-view-${view}`).click();
          await page.waitForFunction(v => document.querySelector(`[data-testid="notif-view-${v}"]`)?.getAttribute("aria-selected") === "true", view);
          initialNotificationCounts[view] = await page.locator(view === "history" ? '[data-testid^="notif-history-row-"]' : '[role="listitem"][data-testid^="notif-"]:not([data-testid^="notif-group-"])').count();
        }
        one.setup.notificationInitialCounts = initialNotificationCounts;
        if (Object.values(initialNotificationCounts).every(n => n === 0)) {
          one.setup.notificationPrecondition = "empty across all triage views; adding a due-today task through the UI";
          await navigate(page, opts.baseUrl, "/todo");
          const before = await page.locator('[data-testid="task-card"]').count();
          await page.getByTestId("todo-add").click();
          const input = page.locator("#task-add"); await input.waitFor({ state: "visible" });
          const title = `Benchmark reminder ${sweep}`; await input.fill(title); await page.getByTestId("task-quick-today").click(); await page.getByTestId("task-add-submit").click();
          await page.waitForFunction(t => document.body.innerText.includes(t), title);
          const after = await page.locator('[data-testid="task-card"]').count();
          if (after <= before) throw new Error(`UI-created due-today task did not increase visible task cards (before=${before}, after=${after})`);
          one.setup.notificationTask = { createdThroughUI: true, taskCountBefore: before, taskCountAfter: after, delta: after - before };
          await page.reload({ waitUntil: "commit" }); await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60000 });
          await page.locator('[data-testid="sample-data-banner"]').waitFor({ state: "visible", timeout: 15000 });
          await navigate(page, opts.baseUrl, "/notifications"); await page.getByTestId("notif-view-live").click();
          await page.waitForFunction(t => [...document.querySelectorAll('[role="listitem"][data-testid^="notif-"]')].some(x => x.innerText.includes(t)), title, { timeout: 15000 });
          one.setup.generatedNotificationAsserted = true;
          one.setup.generatedLiveCount = await page.locator('[role="listitem"][data-testid^="notif-"]:not([data-testid^="notif-group-"])').count();
        }
        one.setup.notificationPrecondition ??= "existing sample notifications available";
        } catch (e) {
          one.setup.notificationPreconditionError = String(e).slice(0, 300);
          report.failures.push({ sweep, error: `notification precondition failed: ${one.setup.notificationPreconditionError}` });
        }
        await run("notifications-triage-populated", "/notifications", async p => {
          const counts = {};
          for (const view of ["live", "watching", "history"]) {
            await p.getByTestId(`notif-view-${view}`).click();
            await p.waitForFunction(v => document.querySelector(`[data-testid="notif-view-${v}"]`)?.getAttribute("aria-selected") === "true", view);
            counts[view] = await p.locator(view === "history" ? '[data-testid^="notif-history-row-"]' : '[role="listitem"][data-testid^="notif-"]:not([data-testid^="notif-group-"])').count();
          }
          if (Object.values(counts).every(n => n === 0)) throw new Error("no notification items appeared across triage views or UI-created due-today task");
          return { viewCounts: counts, generatedByUI: Boolean(one.setup.generatedNotificationAsserted) };
        });
        await run("assistant-local-controls-and-typing", "/assistant", async p => {
          const input = p.locator("#cf-chat-input"); await input.waitFor(); const before = await p.locator("[data-testid^='chat-turn-']").count(); await input.fill("Summarize my budget"); const valueChars = await input.inputValue().then(x => x.length); await input.fill(""); const cleared = await input.inputValue(); const after = await p.locator("[data-testid^='chat-turn-']").count(); if (valueChars === 0 || cleared !== "") throw new Error("Assistant input typing or clear did not take effect"); if (before !== after) throw new Error("typing unexpectedly created a chat turn");
          const hub = p.getByTestId("assistant-hub"), insightsTab = hub.getByRole("radio", { name: "Insights" }); await insightsTab.click(); await p.waitForFunction(() => [...document.querySelectorAll('[data-testid="assistant-hub"] [role="radio"]')].some(el => el.textContent.trim() === "Insights" && el.getAttribute("aria-checked") === "true"));
          const period = p.getByTestId("assistant-period"), range = p.getByTestId("ast-hero-range"); await period.waitFor(); await range.waitFor(); const originalPeriod = await period.inputValue(), originalRange = (await range.innerText()).trim();
          const alternatives = await period.locator("option").evaluateAll(options => options.map(option => option.value)); const alternatePeriod = alternatives.find(value => value !== originalPeriod); if (!alternatePeriod) throw new Error("Assistant Insights has no alternate period");
          await period.selectOption(alternatePeriod); await p.waitForFunction(({ period, range }) => document.querySelector('[data-testid="assistant-period"]')?.value === period && document.querySelector('[data-testid="ast-hero-range"]')?.innerText.trim() !== range, { period: alternatePeriod, range: originalRange });
          const alternateRange = (await range.innerText()).trim(); if (alternateRange === originalRange) throw new Error("Assistant Insights period selection did not change the briefing range");
          await period.selectOption(originalPeriod); await p.waitForFunction(({ period, range }) => document.querySelector('[data-testid="assistant-period"]')?.value === period && document.querySelector('[data-testid="ast-hero-range"]')?.innerText.trim() === range, { period: originalPeriod, range: originalRange });
          await hub.getByRole("radio", { name: "Ask" }).click(); await p.waitForFunction(() => [...document.querySelectorAll('[data-testid="assistant-hub"] [role="radio"]')].some(el => el.textContent.trim() === "Ask" && el.getAttribute("aria-checked") === "true")); await p.locator("#cf-chat-input").waitFor({ state: "visible" });
          return { valueChars, cleared, turnsUnchanged: true, noPromptSubmitted: true, insightsPeriod: { from: originalPeriod, to: alternatePeriod, rangeChanged: true, restored: true }, askReturned: true };
        });
        await run("reports-summary-full-export-scope", "/reports", async p => {
          await p.getByTestId("rpta-mode-summary").click(); await p.waitForFunction(() => document.querySelector('[data-testid="rpta-mode-summary"]')?.getAttribute("aria-pressed") === "true");
          await p.getByTestId("rpta-mode-full").click(); await p.waitForFunction(() => document.querySelector('[data-testid="rpta-mode-full"]')?.getAttribute("aria-pressed") === "true");
          await p.getByTestId("reports-export-toggle").click(); const menu = p.getByRole("menu"); await menu.waitFor(); const options = await menu.getByRole("menuitem").count(); await p.keyboard.press("Escape"); await menu.waitFor({ state: "hidden" });
          await p.getByTestId("reports-scope-toggle").click(); const selector = p.locator('[data-testid="scope-selector"]'); await selector.waitFor({ state: "visible" });
          const chip = selector.locator(".scope-chip:not(.scope-chip-clear)").first(); await chip.waitFor({ state: "visible" });
          const pressedBefore = await chip.getAttribute("aria-pressed"), label = (await chip.innerText()).trim(); await chip.click();
          await p.waitForFunction(({ label, old }) => [...document.querySelectorAll('[data-testid="scope-selector"] .scope-chip:not(.scope-chip-clear)')].some(el => el.innerText.trim() === label && el.getAttribute("aria-pressed") !== old), { label, old: pressedBefore });
          const pressedAfter = await selector.locator(".scope-chip:not(.scope-chip-clear)").filter({ hasText: label }).first().getAttribute("aria-pressed");
          await selector.locator(".scope-chip:not(.scope-chip-clear)").filter({ hasText: label }).first().click();
          await p.waitForFunction(({ label, old }) => [...document.querySelectorAll('[data-testid="scope-selector"] .scope-chip:not(.scope-chip-clear)')].some(el => el.innerText.trim() === label && el.getAttribute("aria-pressed") === old), { label, old: pressedBefore });
          await p.getByTestId("reports-scope-toggle").click(); await selector.waitFor({ state: "hidden" }); return { exportOptions: options, scopeSelection: { label, changed: pressedBefore !== pressedAfter, restored: true } };
        });
      } catch (e) { one.error = String(e).slice(0, 300); report.failures.push({ sweep, error: one.error }); }
      if (measured) report.sweeps.push(one);
      await context.close();
    }
  } finally { await browser.close(); }
  for (const action of ACTIONS) {
    const records = report.sweeps.flatMap(s => s.actions.filter(a => a.name === action && a.status === "ok").map(a => a.metrics));
    report.summary ??= {};
    report.summary[action] = { successfulRuns: records.length, settledMsMedian: median(records.map(x => x.settledMs)), longTaskTotalMsMedian: median(records.map(x => x.longTaskTotalMs)), mutationCountMedian: median(records.map(x => x.mutationCount)), elementCountMedian: median(records.map(x => x.elementCount)), rowCountMedian: median(records.map(x => x.rowCount)) };
  }
  for (const sweep of report.sweeps) {
    const names = sweep.actions.map(action => action.name);
    const missing = ACTIONS.filter(name => !names.includes(name));
    if (missing.length) report.failures.push({ sweep: sweep.index, error: `missing required interaction records: ${missing.join(", ")}` });
  }
  if (artifact && (delivery.rawCount !== delivery.gzipCount || delivery.rawCount < opts.warmups + opts.runs || delivery.brAttempts.length)) report.failures.push({ error: `WASM override counts mismatch: ${JSON.stringify(delivery)}` });
  const json = JSON.stringify(report, null, 2);
  if (opts.out) { const out = path.resolve(process.cwd(), opts.out); mkdirSync(path.dirname(out), { recursive: true }); writeFileSync(out, json); }
  else console.log(json);
  if (report.failures.length || report.sweeps.some(s => s.actions?.some(a => a.status !== "ok"))) process.exitCode = 1;
}

async function navigate(page, baseUrl, route) {
  await page.evaluate(r => { if (location.pathname !== r) { history.pushState({}, "", r); dispatchEvent(new PopStateEvent("popstate")); } }, route);
  await page.locator(`#main[data-route="${route}"]`).waitFor({ state: "visible", timeout: 15000 });
  await page.waitForFunction(r => {
    const root = document.querySelector(`#main[data-route="${r}"]`), deferred = root?.querySelector("[data-settled]");
    return !deferred || deferred.getAttribute("data-settled") === "true";
  }, route, { timeout: 30000 });
}
async function waitForSelectedControl(page, selector) {
  await page.waitForFunction(sel => {
    const el = document.querySelector(sel);
    return el && (el.getAttribute("aria-selected") === "true" || el.getAttribute("aria-pressed") === "true" || el.getAttribute("aria-checked") === "true" || el.classList.contains("is-active") || el.classList.contains("active"));
  }, selector, { timeout: 15000 });
  return page.locator(selector).evaluate(el => ({ ariaSelected: el.getAttribute("aria-selected"), ariaPressed: el.getAttribute("aria-pressed"), ariaChecked: el.getAttribute("aria-checked"), activeClass: el.classList.contains("is-active") || el.classList.contains("active") }));
}
async function diagnose(page) {
  try {
    return await page.evaluate(() => {
      const main = document.querySelector("#main");
      return {
        route: location.pathname,
        mainText: (main?.innerText || "").slice(0, 1200),
        controls: [...(main?.querySelectorAll("button,input,select,summary,[role=tab],[role=radio],[role=menuitem]") || [])].slice(0, 40).map(el => ({
          tag: el.tagName.toLowerCase(), text: (el.innerText || el.getAttribute("aria-label") || "").trim().slice(0, 80),
          testId: el.getAttribute("data-testid"), role: el.getAttribute("role"),
          expanded: el.getAttribute("aria-expanded"), selected: el.getAttribute("aria-selected"),
          pressed: el.getAttribute("aria-pressed"), checked: el.getAttribute("aria-checked"),
          open: typeof el.open === "boolean" ? el.open : undefined,
          hidden: el.offsetParent === null, disabled: Boolean(el.disabled),
        })),
      };
    });
  } catch (error) {
    return { diagnosticError: String(error).slice(0, 200) };
  }
}
async function measure(page, action) {
  const state = await page.evaluate(() => {
    window.__interactionBench.mutations = 0;
    window.__interactionBench.observer?.disconnect();
    window.__interactionBench.observer = new MutationObserver(xs => { window.__interactionBench.mutations += xs.length; });
    window.__interactionBench.observer.observe(document.querySelector("#main"), { childList: true, subtree: true, attributes: true });
    return { longStart: window.__interactionBench.longTasks.length, start: performance.now() };
  });
  const result = await action(page);
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const end = await page.evaluate(() => {
    window.__interactionBench.observer?.disconnect();
    return { end: performance.now(), longs: window.__interactionBench.longTasks.slice(), mutations: window.__interactionBench.mutations, elements: document.querySelectorAll("#main *").length, rows: document.querySelectorAll("#main [role='row'], #main tbody tr, #main [role='listitem']").length, textChars: document.querySelector("#main")?.innerText.length ?? 0 };
  });
  const newLongs = end.longs.slice(state.longStart);
  return { ...result, settledMs: round(end.end - state.start), longTaskCount: newLongs.length, longTaskTotalMs: round(newLongs.reduce((a, b) => a + Math.max(0, b - 50), 0)), longestTaskMs: round(Math.max(0, ...newLongs)), mutationCount: end.mutations, elementCount: end.elements, rowCount: end.rows, textChars: end.textChars };
}

async function armSearchCompletion(page) {
  await page.evaluate(() => {
    const selector = '[data-testid="filter-toolbar-search"]';
    if (!document.querySelector(selector)) throw new Error("search input is missing before typing");
    window.__searchCompletionObserved = false;
    let sawBusy = false;
    const observer = new MutationObserver(records => {
      // Preserve a transient true state even if the render flips it back before
      // MutationObserver callbacks run; oldValue records the intermediate edge.
      if (records.some(record => record.oldValue === "true")) sawBusy = true;
      const input = document.querySelector(selector);
      if (input?.getAttribute("aria-busy") === "true") sawBusy = true;
      else if (sawBusy) {
        window.__searchCompletionObserved = true;
        observer.disconnect();
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeOldValue: true, subtree: true, attributeFilter: ["aria-busy"] });
    window.__searchCompletionObserver = observer;
    if (document.querySelector(selector)?.getAttribute("aria-busy") === "true") sawBusy = true;
  });
}

async function waitForSearchCompletion(page) {
  await page.waitForFunction(() => window.__searchCompletionObserved === true, null, { timeout: 15000 });
}

main().catch(e => { console.error(e); process.exitCode = 1; });
