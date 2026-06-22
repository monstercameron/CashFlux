import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { mkdirSync } from "fs";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

mkdirSync(path.join(__dirname, "screenshots"), { recursive: true });
const BASE = "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });
page.on("pageerror", e => console.log("JSERR:", e.message));
page.on("console", m => console.log("CONSOLE:", m.type(), m.text()));

await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
console.log("URL:", page.url());
await page.waitForTimeout(5000);
const body = await page.evaluate(() => document.body.innerText.slice(0, 1000));
console.log("BODY:", body);

// check nav links
const navLinks = await page.evaluate(() =>
  Array.from(document.querySelectorAll('nav a[title]')).map(a => a.getAttribute("title"))
);
console.log("NAV LINKS:", navLinks);

// check sections
const secs = await page.evaluate(() =>
  Array.from(document.querySelectorAll("section")).map(s => s.textContent.slice(0, 60).replace(/\s+/g, " "))
);
console.log("SECTIONS:", secs);

await page.screenshot({ path: path.join(__dirname, "screenshots", "l61_probe2.png") });
await browser.close();
