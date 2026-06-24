/**
 * GX16 deep probe — scrolls deep into Reports to capture charts.
 */
import { chromium } from "playwright";
import fs from "fs";
import path from "path";

const BASE = "http://localhost:8080";
const SS_DIR = "C:\\Users\\mreca\\Desktop\\CashFlux\\e2e\\screenshots";
fs.mkdirSync(SS_DIR, { recursive: true });

const ss = async (page, name) => {
  const p = path.join(SS_DIR, name);
  await page.screenshot({ path: p, fullPage: false });
  console.log("  screenshot:", name);
};

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  await page.addInitScript(() => {
    const orig = Storage.prototype.getItem;
    Storage.prototype.getItem = function(k) {
      if (k === "cashflux:prefs") return JSON.stringify({ theme: "light" });
      return orig.call(this, k);
    };
  });

  await page.goto(BASE + "/");
  await page.waitForTimeout(2500);

  // Navigate to reports
  await page.evaluate(() => {
    window.history.pushState({}, "", "/reports");
    window.dispatchEvent(new PopStateEvent("popstate", {}));
  });
  await page.waitForTimeout(1200);

  // Scroll down in stages to trigger chart renders
  for (let i = 1; i <= 8; i++) {
    await page.evaluate((ypos) => window.scrollTo(0, ypos), i * 600);
    await page.waitForTimeout(500);
    await ss(page, `gx16_light_reports_scroll${i * 600}.png`);
  }

  // Now measure ALL the chart elements
  const deepAudit = await page.evaluate(() => {
    // Bar chart rects in D3
    const barRects = Array.from(document.querySelectorAll(".cf-chart svg rect")).slice(0, 12).map(el => ({
      attrFill: el.getAttribute("fill"),
      computedFill: getComputedStyle(el).fill,
      width: el.getAttribute("width"),
      height: el.getAttribute("height"),
    }));

    // All SVG text elements (axis labels, legends)
    const svgTexts = Array.from(document.querySelectorAll(".cf-chart svg text")).slice(0, 16).map(el => ({
      attrFill: el.getAttribute("fill"),
      computedColor: getComputedStyle(el).color,
      text: el.textContent.trim().slice(0, 30),
    }));

    // All path elements in D3 charts
    const d3Paths = Array.from(document.querySelectorAll(".cf-chart svg path")).slice(0, 12).map(el => ({
      attrFill: el.getAttribute("fill"),
      attrStroke: el.getAttribute("stroke"),
      computedFill: getComputedStyle(el).fill,
      class: el.getAttribute("class"),
    }));

    // Mermaid SVG deep probe
    const mermaidSvgs = Array.from(document.querySelectorAll("svg[id^='cf-mmd']")).map(svg => {
      const nodes = Array.from(svg.querySelectorAll("rect")).slice(0, 4).map(r => ({
        attrFill: r.getAttribute("fill"),
        computedFill: getComputedStyle(r).fill,
        width: r.getAttribute("width"),
      }));
      const texts = Array.from(svg.querySelectorAll("text")).slice(0, 6).map(t => ({
        attrFill: t.getAttribute("fill"),
        computedColor: getComputedStyle(t).color,
        text: t.textContent.trim().slice(0, 30),
      }));
      const paths = Array.from(svg.querySelectorAll("path")).slice(0, 4).map(p => ({
        attrFill: p.getAttribute("fill"),
        attrStroke: p.getAttribute("stroke"),
        computedFill: getComputedStyle(p).fill,
      }));
      return { id: svg.id, nodes, texts, paths, outerBg: getComputedStyle(svg).backgroundColor };
    });

    // Donut chart paths (in D3)
    const donutPaths = Array.from(document.querySelectorAll(".cf-chart svg path[d]")).filter(p => {
      const f = p.getAttribute("fill");
      return f && f !== "none" && !f.startsWith("url");
    }).slice(0, 10).map(p => ({
      attrFill: p.getAttribute("fill"),
      computedFill: getComputedStyle(p).fill,
    }));

    return { barRects, svgTexts, d3Paths, mermaidSvgs, donutPaths };
  });

  console.log("\n=== Deep audit of Reports page (light) ===");
  console.log("\nD3 bar chart rects:", JSON.stringify(deepAudit.barRects, null, 2));
  console.log("\nD3 SVG texts:", JSON.stringify(deepAudit.svgTexts, null, 2));
  console.log("\nD3 paths:", JSON.stringify(deepAudit.d3Paths, null, 2));
  console.log("\nMermaid SVGs:", JSON.stringify(deepAudit.mermaidSvgs, null, 2));
  console.log("\nDonut paths:", JSON.stringify(deepAudit.donutPaths, null, 2));

  // Also check the hero numbers' CSS
  const heroStats = await page.evaluate(() => {
    const heroParts = ["savings-rate", "cash-runway", "no-spend-days"].map(cls => {
      const el = document.querySelector(`.hero-stat-label, .hero-sub, [class*="hero"]`);
      return el ? { class: el.className, color: getComputedStyle(el).color } : null;
    }).filter(Boolean);

    // The secondary stats "SAVINGS RATE / CASH RUNWAY / NO-SPEND DAYS" labels
    const smallLabels = Array.from(document.querySelectorAll(".hero-stat-label, .hero-flanker-label")).slice(0,4).map(el => ({
      text: el.textContent.trim().slice(0, 30),
      color: getComputedStyle(el).color,
    }));
    const smallValues = Array.from(document.querySelectorAll(".hero-stat-value, .hero-flanker-value")).slice(0,4).map(el => ({
      text: el.textContent.trim().slice(0, 30),
      color: getComputedStyle(el).color,
    }));
    return { heroParts, smallLabels, smallValues };
  });
  console.log("\nHero stats:", JSON.stringify(heroStats, null, 2));

  await ctx.close();
  await browser.close();
  process.exit(0);
})();
