// report-html.mjs — renders a results JSON (from perf-audit.mjs) into a
// self-contained, theme-aware HTML scorecard: two headline gauges (average page
// score + cold load), the cold-load metric breakdown, and every route ranked
// worst-first with a per-page score ring. No external assets, no build step.
//
//   node perf/report-html.mjs perf/results/v1.0.14.json perf/results/v1.0.14.html
//
// The palette echoes the CashFlux app (charcoal-green ground, sage accent) with
// Lighthouse's tri-band gauge colors (green ≥90 / amber ≥50 / red <50).
import { readFileSync, writeFileSync } from "node:fs";

const IN = process.argv[2];
const OUT = process.argv[3];
if (!IN || !OUT) {
  console.error("usage: node report-html.mjs <results.json> <out.html>");
  process.exit(1);
}
const r = JSON.parse(readFileSync(IN, "utf8"));

const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
// Lighthouse tri-band by score.
const band = (s) => (s === null ? "na" : s >= 90 ? "good" : s >= 50 ? "avg" : "poor");
const fmtMs = (v) => (v === null || v === undefined ? "—" : `${Math.round(v)}<span class="u">ms</span>`);

// Circular gauge SVG. size px, stroke width, score 0-100.
function gauge(score, size, sw) {
  const rad = (size - sw) / 2;
  const c = 2 * Math.PI * rad;
  const pct = score === null ? 0 : score / 100;
  const off = c * (1 - pct);
  const b = band(score);
  return `<svg class="gauge g-${b}" viewBox="0 0 ${size} ${size}" width="${size}" height="${size}" role="img" aria-label="score ${score}">
    <circle cx="${size / 2}" cy="${size / 2}" r="${rad}" fill="none" stroke="var(--track)" stroke-width="${sw}"/>
    <circle cx="${size / 2}" cy="${size / 2}" r="${rad}" fill="none" stroke="var(--ring)" stroke-width="${sw}"
      stroke-linecap="round" stroke-dasharray="${c.toFixed(1)}" stroke-dashoffset="${off.toFixed(1)}"
      transform="rotate(-90 ${size / 2} ${size / 2})"/>
    <text x="50%" y="50%" class="gnum" dominant-baseline="central" text-anchor="middle">${score === null ? "—" : score}</text>
  </svg>`;
}

const load = r.coldLoad;
const loadRows = Object.entries(load.parts)
  .map(([k, v]) => {
    const sub = v.sub === null ? null : Math.round(v.sub * 100);
    const val = v.unit === "MB" ? `${Math.round(v.value * 10) / 10}<span class="u">MB</span>` : v.unit === "ms" ? fmtMs(v.value) : v.value;
    return `<div class="lrow">
      <div class="llabel">${esc(v.label)}</div>
      <div class="lval">${val}</div>
      <div class="lbar"><span class="lbarfill b-${band(sub)}" style="width:${sub ?? 0}%"></span></div>
      <div class="lsub b-${band(sub)}">${sub ?? "—"}</div>
    </div>`;
  })
  .join("");

const pages = [...r.pages].sort((a, b) => (a.score ?? 999) - (b.score ?? 999));
const pageRows = pages
  .map((p) => {
    const m = p.metrics || {};
    const b = band(p.score);
    const worst = p.parts
      ? Object.values(p.parts).filter((x) => x.sub !== null).sort((a, c) => a.sub * a.weight - c.sub * c.weight)[0]
      : null;
    return `<tr class="prow">
      <td class="pgauge">${gauge(p.score, 44, 5)}</td>
      <td class="proute"><code>${esc(p.route)}</code><span class="pgrade b-${b}">${p.grade.letter}</span>
        ${worst ? `<div class="pnote">weakest: ${esc(worst.label)}</div>` : ""}</td>
      <td class="pm">${fmtMs(m.mountMs)}</td>
      <td class="pm">${fmtMs(m.tbtMs)}</td>
      <td class="pm">${fmtMs(m.stableMs)}</td>
      <td class="pm">${m.cls ?? "—"}</td>
      <td class="pm">${m.domNodes ?? "—"}</td>
    </tr>`;
  })
  .join("");

const dist = { "A+": 0, A: 0, B: 0, C: 0, D: 0, F: 0 };
for (const p of r.pages) if (dist[p.grade.letter] !== undefined) dist[p.grade.letter]++;
const distChips = Object.entries(dist)
  .filter(([, n]) => n > 0)
  .map(([g, n]) => `<span class="chip b-${g === "A+" || g === "A" ? "good" : g === "F" ? "poor" : "avg"}">${g} · ${n}</span>`)
  .join("");

const captured = new Date(r.capturedAt).toISOString().slice(0, 16).replace("T", " ") + " UTC";

const html = `<style>
  :root{
    --bg:#f3f6f3; --card:#ffffff; --card2:#f7faf8; --border:#e0e6e1;
    --text:#161c19; --muted:#5f6b64; --faint:#8b968f;
    --accent:#3f7d63; --track:#e6ebe7;
    --good:#2f9f6a; --avg:#c68a1e; --poor:#d8503a;
    --shadow:0 1px 2px rgba(20,40,30,.06),0 8px 24px rgba(20,40,30,.05);
  }
  @media (prefers-color-scheme:dark){
    :root{
      --bg:#0e1311; --card:#161d19; --card2:#1a221e; --border:#28322c;
      --text:#e9efeb; --muted:#8f9a92; --faint:#5f6a62;
      --accent:#6fb392; --track:#232d27;
      --good:#43b581; --avg:#e0a340; --poor:#e5634d;
      --shadow:0 1px 2px rgba(0,0,0,.3),0 10px 30px rgba(0,0,0,.28);
    }
  }
  :root[data-theme="light"]{
    --bg:#f3f6f3; --card:#ffffff; --card2:#f7faf8; --border:#e0e6e1;
    --text:#161c19; --muted:#5f6b64; --faint:#8b968f;
    --accent:#3f7d63; --track:#e6ebe7;
    --good:#2f9f6a; --avg:#c68a1e; --poor:#d8503a;
    --shadow:0 1px 2px rgba(20,40,30,.06),0 8px 24px rgba(20,40,30,.05);
  }
  :root[data-theme="dark"]{
    --bg:#0e1311; --card:#161d19; --card2:#1a221e; --border:#28322c;
    --text:#e9efeb; --muted:#8f9a92; --faint:#5f6a62;
    --accent:#6fb392; --track:#232d27;
    --good:#43b581; --avg:#e0a340; --poor:#e5634d;
    --shadow:0 1px 2px rgba(0,0,0,.3),0 10px 30px rgba(0,0,0,.28);
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--text);
    font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;line-height:1.5;
    -webkit-font-smoothing:antialiased;}
  .mono{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-variant-numeric:tabular-nums;}
  .wrap{max-width:940px;margin:0 auto;padding:40px 24px 72px;}

  header.rep{display:flex;align-items:baseline;gap:12px;flex-wrap:wrap;margin-bottom:6px;}
  .brand{font-weight:700;font-size:15px;letter-spacing:.02em;}
  .brand b{color:var(--accent);}
  .ver{font-family:ui-monospace,monospace;font-size:12px;color:var(--muted);border:1px solid var(--border);
    border-radius:999px;padding:2px 9px;}
  h1{font-size:30px;line-height:1.15;letter-spacing:-.02em;margin:14px 0 4px;text-wrap:balance;font-weight:650;}
  .sub{color:var(--muted);font-size:13.5px;margin-bottom:28px;}
  .sub .mono{color:var(--faint);}

  .heroes{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:14px;}
  .hero{background:var(--card);border:1px solid var(--border);border-radius:16px;padding:22px 24px;
    box-shadow:var(--shadow);display:flex;align-items:center;gap:20px;}
  .hero .htext{min-width:0;}
  .hlabel{font-size:11px;text-transform:uppercase;letter-spacing:.09em;color:var(--muted);margin-bottom:3px;}
  .hval{font-size:15px;color:var(--text);font-weight:600;margin-top:2px;}
  .hval .mono{font-size:15px;}
  .hnote{font-size:12.5px;color:var(--muted);margin-top:7px;line-height:1.45;}

  .dist{display:flex;gap:7px;flex-wrap:wrap;margin:2px 0 30px;}
  .chip{font-family:ui-monospace,monospace;font-size:11.5px;padding:3px 9px;border-radius:6px;
    border:1px solid var(--border);color:var(--muted);background:var(--card2);}

  section{margin-top:34px;}
  h2{font-size:13px;text-transform:uppercase;letter-spacing:.1em;color:var(--muted);
    font-weight:600;margin:0 0 14px;display:flex;align-items:center;gap:10px;}
  h2::after{content:"";flex:1;height:1px;background:var(--border);}

  /* cold load */
  .load{background:var(--card);border:1px solid var(--border);border-radius:16px;padding:8px 20px 14px;box-shadow:var(--shadow);}
  .lrow{display:grid;grid-template-columns:170px 96px 1fr 40px;align-items:center;gap:14px;
    padding:11px 0;border-bottom:1px solid var(--border);}
  .lrow:last-child{border-bottom:none;}
  .llabel{font-size:13.5px;color:var(--text);}
  .lval{font-family:ui-monospace,monospace;font-variant-numeric:tabular-nums;font-size:13px;color:var(--muted);text-align:right;}
  .lval .u,.pm .u{font-size:10px;color:var(--faint);margin-left:1px;}
  .lbar{height:7px;background:var(--track);border-radius:99px;overflow:hidden;}
  .lbarfill{display:block;height:100%;border-radius:99px;}
  .lsub{font-family:ui-monospace,monospace;font-size:12.5px;text-align:right;font-weight:600;}

  /* pages table */
  .tablecard{background:var(--card);border:1px solid var(--border);border-radius:16px;box-shadow:var(--shadow);overflow:hidden;}
  .scroll{overflow-x:auto;}
  table{border-collapse:collapse;width:100%;min-width:620px;}
  thead th{font-size:10.5px;text-transform:uppercase;letter-spacing:.07em;color:var(--faint);
    font-weight:600;text-align:right;padding:12px 12px 10px;border-bottom:1px solid var(--border);}
  thead th.l{text-align:left;}
  .prow{border-bottom:1px solid var(--border);}
  .prow:last-child{border-bottom:none;}
  .prow:hover{background:var(--card2);}
  td{padding:9px 12px;}
  .pgauge{width:56px;padding-left:16px;}
  .proute code{font-family:ui-monospace,monospace;font-size:13px;color:var(--text);}
  .pgrade{font-family:ui-monospace,monospace;font-size:11px;font-weight:700;margin-left:8px;padding:1px 6px;border-radius:5px;}
  .pnote{font-size:11px;color:var(--faint);margin-top:2px;}
  .pm{font-family:ui-monospace,monospace;font-variant-numeric:tabular-nums;font-size:12.5px;
    color:var(--muted);text-align:right;white-space:nowrap;}

  /* gauges + semantic color */
  .gauge .gnum{font-family:ui-monospace,monospace;font-weight:700;fill:var(--text);}
  .g-good{--ring:var(--good)} .g-avg{--ring:var(--avg)} .g-poor{--ring:var(--poor)} .g-na{--ring:var(--faint)}
  .hero .gauge .gnum{font-size:22px;}
  .prow .gauge .gnum{font-size:15px;}
  .b-good{color:var(--good)} .b-avg{color:var(--avg)} .b-poor{color:var(--poor)}
  .lbarfill.b-good{background:var(--good)} .lbarfill.b-avg{background:var(--avg)} .lbarfill.b-poor{background:var(--poor)}
  .pgrade.b-good{background:color-mix(in srgb,var(--good) 16%,transparent);color:var(--good);}
  .pgrade.b-avg{background:color-mix(in srgb,var(--avg) 16%,transparent);color:var(--avg);}
  .pgrade.b-poor{background:color-mix(in srgb,var(--poor) 18%,transparent);color:var(--poor);}
  .chip.b-good{color:var(--good);border-color:color-mix(in srgb,var(--good) 40%,var(--border));}
  .chip.b-poor{color:var(--poor);border-color:color-mix(in srgb,var(--poor) 40%,var(--border));}

  .method{margin-top:30px;font-size:12.5px;color:var(--muted);line-height:1.6;
    background:var(--card2);border:1px solid var(--border);border-radius:12px;padding:16px 18px;}
  .method b{color:var(--text);font-weight:600;}
  .legend{display:flex;gap:16px;flex-wrap:wrap;margin-top:8px;font-size:12px;}
  .legend span{display:inline-flex;align-items:center;gap:6px;color:var(--muted);}
  .dot{width:9px;height:9px;border-radius:99px;display:inline-block;}
  @media (max-width:640px){.heroes{grid-template-columns:1fr;}.lrow{grid-template-columns:130px 70px 1fr 34px;gap:10px;}}
</style>

<div class="wrap">
  <header class="rep">
    <span class="brand"><b>CashFlux</b> · performance</span>
    <span class="ver">v${esc(r.version)}</span>
  </header>
  <h1>Page performance ratings</h1>
  <p class="sub">Every route scored on Lighthouse-style curves from the browser's own performance timeline · <span class="mono">${esc(captured)}</span> · median of ${r.generatedWith.passes} passes · ${r.generatedWith.viewport}</p>

  <div class="heroes">
    <div class="hero">
      ${gauge(r.summary.avgPageScore, 92, 9)}
      <div class="htext">
        <div class="hlabel">Average page</div>
        <div class="hval">${r.summary.rated} of ${r.pages.length} routes rated</div>
        <div class="hnote">Warm navigation — the cost of arriving on a page with the app already running.</div>
      </div>
    </div>
    <div class="hero">
      ${gauge(r.summary.loadScore, 92, 9)}
      <div class="htext">
        <div class="hlabel">Cold load</div>
        <div class="hval"><span class="mono">${Math.round(load.metrics.transferMB)}MB</span> to first interactive</div>
        <div class="hnote">One-time wasm boot. The payload and instantiate blocking dominate — not the UI.</div>
      </div>
    </div>
  </div>
  <div class="dist">${distChips}</div>

  <section>
    <h2>Cold load — the one-time wasm boot</h2>
    <div class="load">${loadRows}</div>
    <p class="method"><b>Transfer weight is the headline.</b> The app ships <b>${Math.round(load.metrics.wasmMB)} MB</b> of WebAssembly, served uncompressed here. First paint (${Math.round(load.metrics.fcpMs)}ms) and layout stability are excellent — the shell is instant; it's the binary download and ~${Math.round(load.metrics.tbtMs / 100) / 10}s of instantiate blocking that gate time-to-interactive. Host-side brotli typically cuts the wasm 3–4×; shrinking the binary is the real lever.</p>
  </section>

  <section>
    <h2>Every page — ranked by score, lowest first</h2>
    <div class="tablecard"><div class="scroll">
      <table>
        <thead><tr>
          <th class="l" colspan="2">Route</th>
          <th>Mount</th><th>Blocking</th><th>Settle</th><th>CLS</th><th>DOM</th>
        </tr></thead>
        <tbody>${pageRows}</tbody>
      </table>
    </div></div>
  </section>

  <div class="method">
    <b>How to read this.</b> Each page is measured as a <b>warm single-page navigation</b> from the dashboard: with the wasm runtime already booted, how long until the route's body mounts (Mount), how much the main thread is blocked by long tasks (Blocking / TBT), how much content jumps as it renders (CLS), how long until it's visually stable (Settle), and how heavy its DOM is. Every metric is scored on Lighthouse's log-normal curve and weighted into the 0–100 score. Nothing is measured from Go source — only from the browser's performance timeline.
    <div class="legend">
      <span><span class="dot" style="background:var(--good)"></span>90–100 good</span>
      <span><span class="dot" style="background:var(--avg)"></span>50–89 needs work</span>
      <span><span class="dot" style="background:var(--poor)"></span>0–49 poor</span>
    </div>
  </div>
</div>`;

writeFileSync(OUT, html);
console.log("wrote", OUT, html.length, "bytes");
