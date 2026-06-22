// L31 gate — "a scheduled workflow whose NextRun has passed runs on boot and
// creates its task."
// Injects a due scheduled workflow (trigger=scheduled, action=createTask) via
// addInitScript, reloads the page, and asserts that the task was created.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getDS = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitDS(page, pred, timeoutMs = 10000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

const TASK_TITLE = "Scheduled workflow boot test task";
const WF_ID = "e2e-scheduled-wf-1";

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // First load: let the app boot and save its dataset.
  await page.goto(BASE, { waitUntil: "domcontentloaded" });
  await waitDS(page, (d) => Array.isArray(d.transactions));

  // Arm a one-shot init script: inject a due scheduled workflow before the next
  // wasm boot reads localStorage. The sentinel is cleared so it only runs once.
  await page.evaluate(() => localStorage.setItem("e2e-inject-scheduled-wf", "1"));
  await page.addInitScript((args) => {
    if (!localStorage.getItem("e2e-inject-scheduled-wf")) return;
    localStorage.removeItem("e2e-inject-scheduled-wf");
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      if (!ds.workflows) ds.workflows = [];
      // Remove any previous test injection.
      ds.workflows = ds.workflows.filter((w) => w.id !== args.wfId);
      // Inject a scheduled workflow whose NextRun is in the past.
      ds.workflows.push({
        id: args.wfId,
        name: "E2E scheduled boot test",
        enabled: true,
        trigger: {
          kind: "scheduled",
          cadence: "monthly",
          nextRun: "2026-01-01T00:00:00Z",
        },
        condition: "",
        actions: [{ kind: "createTask", title: args.taskTitle }],
      });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  }, { wfId: WF_ID, taskTitle: TASK_TITLE });

  // Reload — boot scheduled-workflow runner should fire and create the task.
  await page.reload({ waitUntil: "domcontentloaded" });
  const after = await waitDS(page, (d) =>
    (d.tasks || []).some((t) => t.title === TASK_TITLE)
  );

  if (!(after.tasks || []).some((t) => t.title === TASK_TITLE)) {
    fail(`scheduled workflow did not create task "${TASK_TITLE}" on boot`);
  }

  // Also assert that NextRun was advanced past now.
  const wf = (after.workflows || []).find((w) => w.id === WF_ID);
  if (!wf) {
    fail("workflow was removed from dataset after boot run");
  } else if (wf.trigger && new Date(wf.trigger.nextRun) <= new Date()) {
    fail(`NextRun was not advanced past now (got ${wf.trigger.nextRun})`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(`PASS: boot ran scheduled workflow and created task "${TASK_TITLE}"; NextRun advanced to ${wf && wf.trigger && wf.trigger.nextRun}.`);
  }
} finally {
  await browser.close();
}
