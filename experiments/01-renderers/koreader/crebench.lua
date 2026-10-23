-- KOReader route: the EPUB laid out and drawn by crengine (KOReader's engine), driven headless from KOReader's own
-- LuaJIT with KOReader's default reading settings, except the ones fixed for all routes (1072×1448 px, 60 px
-- margins, 34 px text, the book's own CSS and font). Run from KOReader's directory:
--   ./luajit crebench.lua BOOK.epub OUT_DIR [CACHE_DIR]
-- Without CACHE_DIR, every run is cold (KOReader keeps a cache of laid-out books to reopen them faster).
-- LANG_MAIN is the book's language for hyphenation and line breaking (fa by default).
io.stdout:setvbuf("no")
require("setupkoenv")
local Blitbuffer = require("ffi/blitbuffer")
local ffi = require("ffi")
require("ffi/posix_h")

-- KOReader's ui/time reads a coarse clock (millisecond ticks): use the precise monotonic one, in microseconds.
local time = {}
function time.now()
    local ts = ffi.new("struct timespec")
    ffi.C.clock_gettime(ffi.C.CLOCK_MONOTONIC, ts)
    return tonumber(ts.tv_sec) * 1e6 + tonumber(ts.tv_nsec) / 1e3
end
function time.since(t) return time.now() - t end

local book, out_dir, cache_dir = arg[1], arg[2], arg[3]
local W, H = 1072, 1448

local function ms(t) return t / 1000 end -- time values are in microseconds
local function peak_rss_mb()
    for line in io.lines("/proc/self/status") do
        local kb = line:match("^VmHWM:%s+(%d+)")
        if kb then return tonumber(kb) / 1024 end
    end
end

-- Engine setup, as CreDocument:engineInit() does it (cache, hyphenation, KOReader's bundled fonts).
local t0 = time.now()
local cre = require("libs/libkoreader-cre")
local cold = cache_dir == nil
cache_dir = cache_dir or os.tmpname() .. ".cr3cache"
os.execute("mkdir -p " .. cache_dir)
cre.initCache(cache_dir, 64 * 1024 * 1024, true, 40)
cre.initHyphDict("./data/hyph/")
for f in io.popen("find ./fonts -name '*.ttf' -o -name '*.otf' | grep -v -e /urw/ -e nerdfonts"):lines() do
    pcall(cre.registerFont, f)
end
cre.regularizeRegisteredFontsWeights(false)
cre.setAsPreferredFontWithBias("Noto Serif", 1 + 128 * 5 + 256 * 5)
local t_init = time.since(t0)

-- Document view and settings, following CreDocument:setupDefaultView() and creoptions.lua's defaults.
local d = cre.newDocView(W, H, 1) -- 1: page mode
d:readDefaults()
d:setIntProperty("crengine.cache.filesize.min", 65536)
d:setStringProperty("crengine.font.fallback.faces", "Noto Naskh Arabic|Noto Sans|Noto Serif|FreeSerif")
d:setIntProperty("crengine.render.dpi", 96) -- 1 CSS px = 1 screen px, as in the other routes
d:setIntProperty("crengine.render.block.rendering.flags", 0x7FFFFFFF) -- "web" mode, KOReader's default for new books
d:setStyleSheet("./data/epub.css", "")
d:setIntProperty("crengine.doc.embedded.styles.enabled", 1)
d:setIntProperty("crengine.doc.embedded.fonts.enabled", 1)
d:setIntProperty("font.kerning.mode", 3) -- HarfBuzz shaping
d:setIntProperty("font.hinting.mode", 2)
-- KOReader's default word spacing narrows spaces to 95% and lets justification squeeze them to 75%;
-- SPACING=100,100 keeps them as the font and CSS have them (as browsers do).
local scale, condense = (os.getenv("SPACING") or "95,75"):match("(%d+),(%d+)")
d:setIntProperty("crengine.style.space.width.scale.percent", tonumber(scale))
d:setIntProperty("crengine.style.space.condensing.percent", tonumber(condense))
d:setDefaultInterlineSpace(100)
d:setStringProperty("window.status.line", "1") -- no crengine header (KOReader draws its own footer)
d:setStringProperty("crengine.textlang.main.lang", os.getenv("LANG_MAIN") or "fa")
d:setStringProperty("crengine.textlang.embedded.langs.enabled", 1)
d:setStringProperty("crengine.textlang.hyphenation.enabled", 1)
d:setFontSize(34)
d:setIntProperty("crengine.page.margin.left", 60)
d:setIntProperty("crengine.page.margin.top", 60)
d:setIntProperty("crengine.page.margin.right", 60)
d:setIntProperty("crengine.page.margin.bottom", 60)

local t1 = time.now()
assert(d:loadDocument(book), "loadDocument failed")
local t_load = time.since(t1)
local t2 = time.now()
d:renderDocument()
local t_render = time.since(t2)
local pages = d:getPages()

-- Draw every page into an 8-bit grey buffer, as KOReader does for a greyscale screen.
os.execute("mkdir -p " .. out_dir)
local bb = Blitbuffer.new(W, H, Blitbuffer.TYPE_BB8)
local draw = {}
for p = 1, pages do
    d:gotoPage(p)
    local s = time.now()
    d:drawCurrentPage(bb, false, false, false, false)
    draw[p] = time.since(s)
    bb:writePNG(string.format("%s/page-%02d.png", out_dir, p))
end
local rest = 0
for p = 2, pages do rest = rest + draw[p] end
local sorted = {}
for p = 1, pages do sorted[p] = draw[p] end
table.sort(sorted)

local f = io.open(out_dir .. "/timings.json", "w")
f:write(string.format([[{
  "renderer": "koreader",
  "cache": "%s",
  "spacing": "%s,%s",
  "pages": %d,
  "init_ms": %.1f,
  "load_ms": %.1f,
  "layout_ms": %.1f,
  "draw_first_ms": %.3f,
  "draw_avg_rest_ms": %.3f,
  "draw_page_median_ms": %.3f,
  "max_rss_mb": %.1f
}
]], cold and "cold" or "warm", scale, condense, pages, ms(t_init), ms(t_load), ms(t_render), ms(draw[1]),
    pages > 1 and ms(rest) / (pages - 1) or 0, ms(sorted[math.floor(pages / 2) + 1]), peak_rss_mb()))
f:close()
for line in io.lines(out_dir .. "/timings.json") do print(line) end
