package main

import (
	"encoding/json"
	"os"
	"strings"
)

// writeViewer writes OUT_DIR/index.html: the read-along played in a browser from the page images and words.json
// (inlined, as browsers refuse to fetch files next to a page opened from disk).
func writeViewer(outDir string, cues []cue) error {
	j, err := json.Marshal(cues)
	if err != nil {
		return err
	}
	html := strings.Replace(viewerHTML, "/*WORDS*/[]", string(j), 1)
	return os.WriteFile(outDir+"/index.html", []byte(html), 0o644)
}

const viewerHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Read-along trial</title>
<style>
:root{--bg:#fff;--fg:#1d1d1f;--muted:#666;--line:#ddd}
@media (prefers-color-scheme:dark){:root{--bg:#161616;--fg:#eee;--muted:#aaa;--line:#333}}
body{background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,sans-serif;margin:0;padding:16px}
h1{font-size:20px;margin:0 0 4px} p{color:var(--muted);margin:0 0 12px;max-width:70ch}
.bar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin:0 0 12px}
button,select{font:inherit;padding:4px 12px}
input[type=range]{flex:1;min-width:160px;max-width:536px}
.page{position:relative;width:min(536px,100%);aspect-ratio:1072/1448;border:1px solid var(--line)}
.page img{width:100%;height:100%;display:block}
.hl{position:absolute;background:#000;mix-blend-mode:multiply;opacity:.33}
</style></head><body>
<h1>Read-along: our own pages, Readium timings</h1>
<p>Pages laid out and drawn by our own Go engine (go-text); the word to highlight comes from the book's SMIL media overlay
read by readium/go-toolkit. The timings are made up (a steady pace), there is no sound. Boxes are drawn here over the
page image the same way the Go program greys them on the e-ink frame.</p>
<div class="bar"><button id="play">Play</button>
<select id="speed"><option value="1">1×</option><option value="2" selected>2×</option><option value="4">4×</option><option value="10">10×</option></select>
<input id="seek" type="range" min="0" value="0"><span id="info"></span></div>
<div class="page" id="page"><img id="img" alt="Book page"></div>
<script>
const W = /*WORDS*/[];
const img = document.getElementById("img"), page = document.getElementById("page");
const seek = document.getElementById("seek"), info = document.getElementById("info");
const playBtn = document.getElementById("play"), speed = document.getElementById("speed");
seek.max = W.length - 1;
let i = 0, playing = false, t0 = 0, start = 0, timer = null;
function show(k) {
  i = k; const w = W[k]; seek.value = k;
  const src = "page-" + String(w.page + 1).padStart(3, "0") + ".png";
  if (!img.src.endsWith(src)) img.src = src;
  page.querySelectorAll(".hl").forEach(e => e.remove());
  for (const r of w.rects || []) {
    const d = document.createElement("div"); d.className = "hl";
    d.style.left = (r.Min.X / 10.72) + "%"; d.style.top = (r.Min.Y / 14.48) + "%";
    d.style.width = ((r.Max.X - r.Min.X) / 10.72) + "%"; d.style.height = ((r.Max.Y - r.Min.Y) / 14.48) + "%";
    page.appendChild(d);
  }
  info.textContent = "word " + (k + 1) + " of " + W.length + ", " + w.begin.toFixed(1) + " s, page " + (w.page + 1);
}
function tick() {
  const now = start + (performance.now() - t0) / 1000 * Number(speed.value);
  let k = i;
  while (k + 1 < W.length && W[k + 1].begin <= now) k++;
  if (k !== i) show(k);
  if (k + 1 >= W.length) { stop(); return; }
  timer = requestAnimationFrame(tick);
}
function play() { playing = true; playBtn.textContent = "Pause"; start = W[i].begin; t0 = performance.now(); tick(); }
function stop() { playing = false; playBtn.textContent = "Play"; cancelAnimationFrame(timer); }
playBtn.onclick = () => playing ? stop() : play();
speed.onchange = () => { if (playing) { stop(); play(); } };
seek.oninput = () => { const p = playing; stop(); show(Number(seek.value)); if (p) play(); };
show(0);
</script></body></html>
`
