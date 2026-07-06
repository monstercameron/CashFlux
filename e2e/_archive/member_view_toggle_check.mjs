// L21 gate — "Yours, Mine, and Ours" member-view toggle.
//
// Seeded members (from internal/store/sample.go SampleDataset):
//   Daniel Carter  → member ID "m-daniel"  (default; owns all attributed transactions)
//   Jordan Lee     → member ID "m-jordan"  (roommate; no attributed transactions)
//
// Selectors:
//   Top-bar switcher : [data-testid="member-switcher"]
//   Transactions page : /transactions  (ledger rows match "member-switcher" scope)
//
// This spec:
//   1. Verifies the member-switcher <select> is present in the top bar on the
//      Transactions page with options for Everyone / Daniel Carter / Jordan Lee.
//   2. Selects "Daniel Carter" (m-daniel) and confirms the row count equals
//      the number of transactions attributed to Daniel (≥1, not zero).
//   3. Selects "Jordan Lee" (m-jordan) and confirms the row count drops to 0
//      (Jordan has no attributed transactions in the sample data).
//   4. Selects "Everyone" (empty value) and confirms the row count is restored
//      to the full set (≥ what Daniel had).
//
// Run with: node e2e/member_view_toggle_check.mjs
// Exits non-zero on any assertion failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// countTxnRows returns the number of visible transaction rows inside the
// .txn-table once the ledger has stabilised. Returns 0 when the ledger shows
// the empty-state or no-match state (the .txn-table is absent or has no .row
// children in those cases).
async function countTxnRows(page) {
  // Allow a brief settle after a switcher change.
  await page.waitForTimeout(400);
  const rows = page.locator(".txn-table .row");
  return await rows.count();
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  // Wait for the shell to mount (the switcher lives in the top bar).
  await page.waitForSelector('[data-testid="member-switcher"]', { timeout: 60_000 });

  // ── 1. Switcher is present with the expected options ──────────────────────
  const switcher = page.locator('[data-testid="member-switcher"]');
  const optionTexts = await switcher.locator("option").allInnerTexts();
  const optionValues = await switcher.locator("option").evaluateAll((els) =>
    els.map((el) => el.value)
  );

  if (!optionValues.includes("")) fail('switcher is missing the "Everyone" option (empty value)');
  if (!optionValues.includes("m-daniel")) fail('switcher is missing the "m-daniel" (Daniel Carter) option');
  if (!optionValues.includes("m-jordan")) fail('switcher is missing the "m-jordan" (Jordan Lee) option');

  if (!optionTexts.some((t) => t.includes("Everyone")))
    fail(`switcher "Everyone" option text not found (saw: ${optionTexts.join(", ")})`);
  if (!optionTexts.some((t) => t.includes("Daniel")))
    fail(`switcher Daniel option text not found (saw: ${optionTexts.join(", ")})`);
  if (!optionTexts.some((t) => t.includes("Jordan")))
    fail(`switcher Jordan option text not found (saw: ${optionTexts.join(", ")})`);

  console.log(`  ✓ switcher present with options: ${optionTexts.map((t) => t.trim()).join(" | ")}`);

  // ── 2. Select Daniel Carter → ledger scoped to Daniel's rows ─────────────
  await switcher.selectOption("m-daniel");
  const danielCount = await countTxnRows(page);
  if (danielCount < 1)
    fail(`expected ≥1 transaction row for Daniel Carter (m-daniel), got ${danielCount}`);
  console.log(`  ✓ Daniel Carter selected → ${danielCount} row(s) shown`);

  // ── 3. Select Jordan Lee → ledger is empty (no attributed transactions) ───
  await switcher.selectOption("m-jordan");
  const jordanCount = await countTxnRows(page);
  if (jordanCount !== 0)
    fail(`expected 0 transaction rows for Jordan Lee (m-jordan) — she has no attributed txns in the sample data — got ${jordanCount}`);
  console.log(`  ✓ Jordan Lee selected → 0 rows shown (no attributed transactions)`);

  // ── 4. Switch back to Everyone → full ledger restored ─────────────────────
  await switcher.selectOption("");
  const everyoneCount = await countTxnRows(page);
  if (everyoneCount < danielCount)
    fail(`expected ≥${danielCount} rows after switching back to Everyone, got ${everyoneCount}`);
  console.log(`  ✓ Everyone selected → ${everyoneCount} row(s) restored`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: member-switcher renders in top bar; selecting Daniel scopes ledger; " +
        "Jordan returns 0 rows; Everyone restores full set."
    );
} finally {
  await browser.close();
}
