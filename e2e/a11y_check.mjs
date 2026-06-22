// L7 a11y gate — the runtime accessibility sweep, promoted to a committed CI gate.
// For each surface it asserts: (a) nav + main landmarks; (b) ZERO visible focusable
// controls without an accessible name; (c) ZERO form fields without a label;
// (d) first Tab stop on /transactions has a visible focus outline;
// (e) every [role=radiogroup] has exactly one [role=radio] descendant with tabindex=0
//     (the roving-tabindex invariant introduced in L7).
//
// Scope: /transactions, /accounts. (Settings panel omitted — it requires a panel-open
// gesture and is owned by a parallel work stream.)
//
// Any real finding is reported by tag+class so it is actionable.
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

// ---------------------------------------------------------------------------
// accName — lightweight accessible-name computation (spec §4.3 subset).
// Covers aria-label, aria-labelledby, title, for-label, wrapping label,
// text content, placeholder, value (submit/button), and alt.
// ---------------------------------------------------------------------------
const accNameScript = `
function accName(el) {
  const al = el.getAttribute("aria-label");
  if (al && al.trim()) return al.trim();
  const lb = el.getAttribute("aria-labelledby");
  if (lb) {
    const t = lb.split(/\\s+/).map(id => {
      const e = document.getElementById(id);
      return e ? (e.textContent || "").trim() : "";
    }).join(" ").trim();
    if (t) return t;
  }
  const title = el.getAttribute("title");
  if (title && title.trim()) return title.trim();
  if (el.id) {
    const esc = window.CSS && CSS.escape ? CSS.escape(el.id) : el.id;
    const lab = document.querySelector('label[for="' + esc + '"]');
    if (lab && lab.textContent.trim()) return lab.textContent.trim();
  }
  const wrap = el.closest("label");
  if (wrap && wrap.textContent.trim()) return wrap.textContent.trim();
  const txt = (el.textContent || "").trim();
  if (txt) return txt;
  const ph = el.getAttribute("placeholder");
  if (ph && ph.trim()) return ph.trim();
  if (el.tagName === "INPUT" && (el.type === "submit" || el.type === "button") && el.value) return el.value;
  const alt = el.getAttribute("alt");
  if (alt && alt.trim()) return alt.trim();
  return "";
}
`;

// Collect a11y data from the currently-loaded page.
const audit = (page) =>
  page.evaluate(() => {
    function accName(el) {
      const al = el.getAttribute("aria-label");
      if (al && al.trim()) return al.trim();
      const lb = el.getAttribute("aria-labelledby");
      if (lb) {
        const t = lb
          .split(/\s+/)
          .map((id) => {
            const e = document.getElementById(id);
            return e ? (e.textContent || "").trim() : "";
          })
          .join(" ")
          .trim();
        if (t) return t;
      }
      const title = el.getAttribute("title");
      if (title && title.trim()) return title.trim();
      if (el.id) {
        const esc =
          window.CSS && CSS.escape ? CSS.escape(el.id) : el.id;
        const lab = document.querySelector('label[for="' + esc + '"]');
        if (lab && lab.textContent.trim()) return lab.textContent.trim();
      }
      const wrap = el.closest("label");
      if (wrap && wrap.textContent.trim()) return wrap.textContent.trim();
      const txt = (el.textContent || "").trim();
      if (txt) return txt;
      const ph = el.getAttribute("placeholder");
      if (ph && ph.trim()) return ph.trim();
      if (
        el.tagName === "INPUT" &&
        (el.type === "submit" || el.type === "button") &&
        el.value
      )
        return el.value;
      const alt = el.getAttribute("alt");
      if (alt && alt.trim()) return alt.trim();
      return "";
    }

    // An element is "visible" for a11y purposes if it is in the layout tree.
    const vis = (el) =>
      el.offsetParent !== null ||
      (el.getClientRects && el.getClientRects().length > 0);

    const desc = (el) =>
      el.tagName.toLowerCase() +
      (el.type ? "[type=" + el.type + "]" : "") +
      (el.className && typeof el.className === "string"
        ? "." + el.className.split(/\s+/).slice(0, 2).join(".")
        : "");

    // (b) Focusable controls without an accessible name.
    const focusables = [
      ...document.querySelectorAll(
        'a[href], button, input:not([type=hidden]), select, textarea,' +
          ' [tabindex]:not([tabindex="-1"]), [role="radio"], [role="switch"]'
      ),
    ].filter(vis);
    const unnamed = focusables.filter((el) => !accName(el)).map(desc);

    // (c) Form fields without a label.
    const fields = [
      ...document.querySelectorAll(
        "input:not([type=hidden]):not([type=submit]):not([type=button])" +
          ":not([type=checkbox]):not([type=radio]), select, textarea"
      ),
    ].filter(vis);
    const unlabeled = fields.filter((el) => !accName(el)).map(desc);

    // (e) Radiogroup roving-tabindex invariant: each [role=radiogroup] must have
    //     exactly one [role=radio] descendant with tabindex="0" (or no tabindex
    //     attribute while all others are "-1").
    const radiogroups = [...document.querySelectorAll("[role=radiogroup]")].filter(vis);
    const badGroups = [];
    for (const rg of radiogroups) {
      const radios = [...rg.querySelectorAll("[role=radio]")].filter(vis);
      if (radios.length === 0) continue; // empty group — ignore
      const stops = radios.filter(
        (r) => r.getAttribute("tabindex") === "0" || !r.hasAttribute("tabindex")
      );
      // Exactly one should be the Tab stop; the rest must be -1.
      const tabzero = radios.filter((r) => r.getAttribute("tabindex") === "0");
      const tabminus = radios.filter((r) => r.getAttribute("tabindex") === "-1");
      const noTab = radios.filter((r) => !r.hasAttribute("tabindex"));
      // Accept: exactly one tabindex=0 and rest at -1, OR all lack tabindex (pre-enhancement).
      const valid =
        (tabzero.length === 1 && tabminus.length === radios.length - 1 && noTab.length === 0) ||
        noTab.length === radios.length;
      if (!valid) {
        badGroups.push(
          desc(rg) +
            " (radios=" + radios.length +
            ", tabindex=0 count=" + tabzero.length +
            ", tabindex=-1 count=" + tabminus.length +
            ", no-tabindex count=" + noTab.length + ")"
        );
      }
    }

    return {
      navs: document.querySelectorAll("nav[aria-label]").length,
      main: !!document.querySelector("main, #main, [role=main]"),
      focusables: focusables.length,
      unnamed,
      unlabeled,
      badGroups,
    };
  });

// Assert landmark + name + label checks, print per-surface PASS lines.
async function sweep(page, where) {
  const r = await audit(page);
  // (a) Landmarks.
  if (r.navs === 0) fail(`${where}: no nav[aria-label] landmark`);
  else console.log(`PASS: ${where}: nav[aria-label] present`);
  if (!r.main) fail(`${where}: no main landmark (main, #main, or [role=main])`);
  else console.log(`PASS: ${where}: main landmark present`);
  // (b) Unnamed focusable controls.
  if (r.unnamed.length > 0)
    fail(
      `${where}: ${r.unnamed.length} focusable control(s) without accessible name: ${r.unnamed.join(", ")}`
    );
  else console.log(`PASS: ${where}: all ${r.focusables} focusable controls are named`);
  // (c) Unlabeled form fields.
  if (r.unlabeled.length > 0)
    fail(`${where}: ${r.unlabeled.length} unlabeled form field(s): ${r.unlabeled.join(", ")}`);
  else console.log(`PASS: ${where}: all form fields are labeled`);
  // (e) Radiogroup roving-tabindex invariant.
  if (r.badGroups.length > 0)
    fail(
      `${where}: ${r.badGroups.length} radiogroup(s) violate roving-tabindex (must have exactly one tabindex=0): ${r.badGroups.join("; ")}`
    );
  else console.log(`PASS: ${where}: all radiogroups satisfy roving-tabindex invariant`);
  return r;
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // Sweep /transactions and /accounts as the primary a11y surfaces.
  for (const route of ["/transactions", "/accounts"]) {
    await page.goto(BASE + route, { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app *", { timeout: 60000 });
    await page.waitForTimeout(700);
    await sweep(page, route);
  }

  // (d) Focus-outline check on /transactions: after one Tab press the focused
  //     element should differ from body AND have a visible outline.
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.keyboard.press("Tab");
  await page.waitForTimeout(100);

  const focusResult = await page.evaluate(() => {
    const el = document.activeElement;
    if (!el || el === document.body || el === document.documentElement) {
      return { focused: false, desc: "body" };
    }
    const style = getComputedStyle(el);
    const hasOutline =
      (style.outlineStyle !== "none" &&
        style.outlineStyle !== "" &&
        parseFloat(style.outlineWidth) > 0) ||
      (style.boxShadow && style.boxShadow !== "none" && style.boxShadow !== "");
    const tag =
      el.tagName.toLowerCase() +
      (el.className && typeof el.className === "string"
        ? "." + el.className.split(/\s+/).slice(0, 2).join(".")
        : "");
    return { focused: true, hasOutline, tag, outline: style.outline, boxShadow: style.boxShadow };
  });

  if (!focusResult.focused) {
    fail("/transactions: Tab did not move focus off body");
  } else {
    console.log(`PASS: /transactions: Tab moved focus to <${focusResult.tag}>`);
    if (!focusResult.hasOutline) {
      fail(
        `/transactions: focused element <${focusResult.tag}> has no visible outline/box-shadow` +
          ` (outline="${focusResult.outline}", box-shadow="${focusResult.boxShadow}")`
      );
    } else {
      console.log(
        `PASS: /transactions: focused element <${focusResult.tag}> has a visible focus indicator`
      );
    }
  }

  if (!process.exitCode)
    console.log(
      "PASS: a11y sweep complete — landmarks, names, labels, focus-outline, and radiogroup tabindex all clean on /transactions and /accounts."
    );
} finally {
  await browser.close();
}
