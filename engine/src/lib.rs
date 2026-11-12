//! A paginated layout engine on Blitz (Stylo for CSS, Taffy for boxes, Parley for text). Blitz lays each spine
//! document out as one long page; this crate cuts it into pages between line boxes, with the rules experiment 03
//! matches Chromium on, draws a page by scrolling Blitz's painter to it, and finds an element's page and boxes. It
//! reads the book through a `Source`, so the reader (Go, with readium/go-toolkit) stays in charge of the EPUB.

use anyrender::{ImageRenderer as _, PaintScene as _};
use anyrender_vello_cpu::VelloCpuImageRenderer;
use blitz_dom::{BaseDocument, DocumentConfig};
use blitz_html::HtmlDocument;
use blitz_paint::paint_scene;
use blitz_traits::net::{NetHandler, NetProvider, Request};
use blitz_traits::shell::{ColorScheme, Viewport};
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Instant;

pub mod audio;
pub mod capi;
mod glyphs;
mod pictures;
mod saved;

/// The book's files, by path from the EPUB's root.
pub trait Source: Send + Sync + 'static {
    fn read(&self, path: &str) -> Option<Vec<u8>>;
}

/// An unzipped EPUB in a folder.
pub struct Dir(pub std::path::PathBuf);

impl Source for Dir {
    fn read(&self, path: &str) -> Option<Vec<u8>> {
        std::fs::read(self.0.join(path)).ok()
    }
}

mod epub;
pub use epub::Epub;

/// Documents get URLs under this root, so that relative links resolve to paths in the book.
const ROOT: &str = "file:///book/";

/// Gives Blitz the book's files (stylesheets, fonts, images) from the Source, at once; adds the time spent, Blitz's
/// handling of each file included (parsing a stylesheet, decoding a font), to the counter in microseconds.
struct Files(Arc<dyn Source>, Arc<AtomicU64>);

impl NetProvider for Files {
    fn fetch(&self, _doc_id: usize, request: Request, handler: Box<dyn NetHandler>) {
        let url = request.url.as_str();
        if let Some(path) = url.strip_prefix(ROOT) {
            let path = percent_decode(path.split(['#', '?']).next().unwrap_or(""));
            let t = Instant::now();
            if let Some(data) = self.0.read(&path) {
                handler.bytes(request.url.to_string(), bytes::Bytes::from(data));
            }
            self.1.fetch_add(t.elapsed().as_micros() as u64, Ordering::Relaxed);
        }
    }
}

fn percent_decode(s: &str) -> String {
    let b = s.as_bytes();
    let mut out = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        if b[i] == b'%' && i + 2 < b.len() {
            if let Ok(v) = u8::from_str_radix(&s[i + 1..i + 3], 16) {
                out.push(v);
                i += 3;
                continue;
            }
        }
        out.push(b[i]);
        i += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

/// What `Book::draw_layer` draws.
#[derive(Clone, Copy, Debug)]
pub enum Layer {
    /// the glyphs our layer copies from its masks
    Text,
    /// what goes to vello_cpu: all but plain text
    Rest,
    /// the whole page through vello_cpu, as before the glyph layer
    AllVello,
}

#[derive(Clone, Copy, Debug)]
pub struct Options {
    pub width: u32,
    pub height: u32,
    pub margin: u32,
    pub font_size: f32,
    /// line spacing, as CSS's unitless line-height, over every element's own (0: the book's)
    pub line_height: f32,
    pub orphans: usize,
    pub widows: usize,
    /// documents kept laid out: the ones the reader used last (others are laid out again when reached)
    pub keep: usize,
    /// the book's script, from its language: which fallback font comes first (`font_context`)
    pub script: BookScript,
}

/// What a book's language says about its fonts: Hebrew and the languages written in the Arabic script other than Farsi
/// have their own fallback font first; everything else (Farsi, Latin, ...) the reader's usual order.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum BookScript {
    #[default]
    Other,
    Hebrew,
    Arabic,
}

impl BookScript {
    /// From a language code in BCP 47 ("ur", "he-IL"): its primary subtag.
    pub fn from_lang(code: &str) -> BookScript {
        match code.split(['-', '_']).next().unwrap_or("").to_ascii_lowercase().as_str() {
            "he" | "iw" | "yi" => BookScript::Hebrew,
            "ar" | "ur" | "ps" | "sd" | "ckb" | "ug" => BookScript::Arabic,
            _ => BookScript::Other,
        }
    }
}

impl Default for Options {
    fn default() -> Self {
        Options { width: 1072, height: 1448, margin: 60, font_size: 34.0, line_height: 0.0, orphans: 2, widows: 2, keep: 2, script: BookScript::Other }
    }
}

/// One line box, in its document's coordinates (CSS px from the top).
#[derive(Clone, Copy, Debug)]
struct LineBox {
    top: f64,
    bottom: f64,
    text: bool, // holds glyphs (not only an image)
}

/// The lines of one block of inline content (an inline root), and how it may break.
struct Block {
    lines: Vec<LineBox>,
    avoid_after: bool,  // a heading: keep it with what follows
    avoid_inside: bool, // a heading: never split
}

/// Where the time of the last document laid out went, in ms (experiment 08's `blitz-open`).
#[derive(Clone, Copy, Debug, Default)]
pub struct LoadTimes {
    /// the document's file read
    pub read: f64,
    /// parsing the HTML into Blitz's document, less `files`
    pub parse: f64,
    /// its stylesheets, fonts and pictures: read and handed to Blitz (which parses or decodes them)
    pub files: f64,
    /// Blitz's first style and layout, less `files`
    pub resolve: f64,
    /// the second, once the stylesheets and fonts are in, less `files`
    pub resolve2: f64,
    /// cutting it into pages
    pub paginate: f64,
}

/// A page: a slice [top, bottom) of one document.
#[derive(Clone, Copy, Debug)]
pub struct Page {
    pub doc: usize,
    pub top: f64,
    pub bottom: f64,
    pub lines: usize, // lines of text on it
}

pub struct Book {
    src: Arc<dyn Source>,
    spine: Vec<String>,
    fctx: Option<parley::FontContext>,
    docs: Vec<Option<HtmlDocument>>, // None: not laid out, or let go
    paged: Vec<bool>,                // its pages are in `pages` (kept when the document is let go)
    used: Vec<usize>,                // the documents the reader used, the last first
    fetched: Arc<AtomicU64>,         // time in Files, microseconds
    /// where the time of the last document laid out went
    pub last_load: LoadTimes,
    /// the pages of the documents paginated so far, in spine order
    pub pages: Vec<Page>,
    pub(crate) opts: Options,
    // kept between pages: the glyph masks, the pictures decoded, vello_cpu's renderer (its caches) and the colour page
    // for what is not text
    glyphs: glyphs::Cache,
    pictures: pictures::Cache,
    renderer: VelloCpuImageRenderer,
    rgba: Vec<u8>,
}

impl Book {
    /// Lays out every spine document (paths from the EPUB's root) and paginates it.
    pub fn open(src: Arc<dyn Source>, spine: &[String], opts: Options) -> Book {
        Book::open_with_fonts(src, spine, opts, &[])
    }

    /// The same, with fonts for text the book's own fonts do not cover: each file is (generic family, bytes), the
    /// family being "serif", "sans-serif" or "monospace" (and "sans-serif" also serves system-ui).
    pub fn open_with_fonts(src: Arc<dyn Source>, spine: &[String], opts: Options, fonts: &[(&str, Vec<u8>)]) -> Book {
        let mut book = Book::new(src, spine, opts, fonts);
        book.layout_rest();
        book
    }

    /// A book with nothing laid out yet: `layout` lays out the document to show first, `layout_rest` the others.
    pub fn new(src: Arc<dyn Source>, spine: &[String], opts: Options, fonts: &[(&str, Vec<u8>)]) -> Book {
        Book {
            src,
            spine: spine.to_vec(),
            fctx: font_context(fonts, opts.script),
            docs: spine.iter().map(|_| None).collect(),
            paged: vec![false; spine.len()],
            used: vec![],
            fetched: Default::default(),
            last_load: Default::default(),
            pages: vec![],
            opts,
            glyphs: Default::default(),
            pictures: Default::default(),
            renderer: VelloCpuImageRenderer::new(opts.width, opts.height),
            rgba: vec![],
        }
    }

    /// Lays out spine document `doc` for the reader (paginating it the first time); its pages are `pages[range]`.
    /// Page numbers after it shift by its page count the first time: they are final once every document is
    /// paginated. Only the `keep` documents used last stay laid out.
    pub fn layout(&mut self, doc: usize) -> std::ops::Range<usize> {
        self.load(doc);
        self.used.retain(|&d| d != doc);
        self.used.insert(0, doc);
        for d in self.used.split_off(self.opts.keep.max(1).min(self.used.len())) {
            self.docs[d] = None;
        }
        let from = self.pages.partition_point(|p| p.doc < doc);
        from..self.pages.partition_point(|p| p.doc <= doc)
    }

    /// The book's cache: every document's pages (once all are paginated: None before) and the glyph masks drawn so far.
    /// The reader keeps it by the book's file (its name, size, date) and gives it to `restore` when the book opens again.
    pub fn save(&self) -> Option<Vec<u8>> {
        if self.paged.contains(&false) {
            return None;
        }
        let mut w = saved::W(saved::MAGIC.to_vec());
        w.u32(saved::VERSION);
        let o = self.opts;
        for v in [o.width, o.height, o.margin, o.font_size.to_bits(), o.line_height.to_bits(), o.orphans as u32, o.widows as u32, o.script as u32] {
            w.u32(v);
        }
        w.u32(self.spine.len() as u32);
        self.spine.iter().for_each(|p| w.bytes(p.as_bytes()));
        w.u32(self.pages.len() as u32);
        for p in &self.pages {
            w.u32(p.doc as u32);
            w.f64(p.top);
            w.f64(p.bottom);
            w.u32(p.lines as u32);
        }
        self.glyphs.save(&mut w);
        Some(w.0)
    }

    /// Takes the pages and glyph masks of a cache from `save`, on a book just made (`new`): no document is
    /// paginated after, only laid out when shown. False, and the book unchanged, if the cache is for other page
    /// settings, another spine or another version, or damaged.
    pub fn restore(&mut self, data: &[u8]) -> bool {
        if self.paged.contains(&true) {
            return false;
        }
        let mut r = saved::R(data);
        let pages = (|| {
            if r.take(4)? != saved::MAGIC || r.u32()? != saved::VERSION {
                return None;
            }
            let o = self.opts;
            for v in [o.width, o.height, o.margin, o.font_size.to_bits(), o.line_height.to_bits(), o.orphans as u32, o.widows as u32, o.script as u32] {
                if r.u32()? != v {
                    return None;
                }
            }
            if r.u32()? as usize != self.spine.len() || !self.spine.iter().all(|p| r.bytes() == Some(p.as_bytes())) {
                return None;
            }
            let n = r.u32()?;
            let page = |r: &mut saved::R| Some(Page { doc: r.u32()? as usize, top: r.f64()?, bottom: r.f64()?, lines: r.u32()? as usize });
            let pages: Vec<Page> = (0..n).map(|_| page(&mut r)).collect::<Option<_>>()?;
            pages.iter().all(|p| p.doc < self.spine.len()).then_some(pages)
        })();
        let Some(pages) = pages else { return false };
        let mut glyphs = glyphs::Cache::default();
        if glyphs.load(&mut r).is_none() {
            return false;
        }
        self.pages = pages;
        self.paged.fill(true);
        self.glyphs = glyphs;
        true
    }

    /// Paginates every document not paginated yet, letting each go after unless the reader is using it.
    pub fn layout_rest(&mut self) {
        while self.layout_next() {}
    }

    /// Paginates the next document not paginated yet (in spine order), letting it go after unless the reader is using
    /// it. False if every document already was.
    pub fn layout_next(&mut self) -> bool {
        let Some(doc) = self.paged.iter().position(|&p| !p) else { return false };
        self.load(doc);
        if !self.used.contains(&doc) {
            self.docs[doc] = None;
        }
        true
    }

    /// Lays out `doc` if it is not, and paginates it the first time.
    fn load(&mut self, doc: usize) {
        if self.docs[doc].is_some() {
            return;
        }
        let (opts, path) = (self.opts, &self.spine[doc]);
        let ms = |t: Instant| t.elapsed().as_secs_f64() * 1e3;
        let files = |f: &AtomicU64| f.swap(0, Ordering::Relaxed) as f64 / 1e3;
        files(&self.fetched);
        let mut lt = LoadTimes::default();
        let t = Instant::now();
        let html = self.src.read(path).map(|b| String::from_utf8_lossy(&b).into_owned()).unwrap_or_default();
        lt.read = ms(t);
        // the shared page settings: text size and margins (the top one as the first page's), padding below so the
        // painter can scroll the last page to the top, and the line spacing if one is set
        let lh = if opts.line_height > 0.0 { format!(" * {{ line-height: {} !important; }}", opts.line_height) } else { String::new() };
        let css = format!(
            "<style>html {{ font-size: {}px; padding: {m}px {m}px {}px {m}px; }}{lh}</style></head>",
            opts.font_size,
            opts.height,
            m = opts.margin
        );
        let html = html.replacen("</head>", &css, 1);
        let t = Instant::now();
        let mut d = HtmlDocument::from_html(
            &html,
            DocumentConfig {
                base_url: Some(format!("{ROOT}{path}")),
                net_provider: Some(Arc::new(Files(Arc::clone(&self.src), Arc::clone(&self.fetched))) as _),
                viewport: Some(Viewport::new(opts.width, opts.height, 1.0, ColorScheme::Light)),
                font_ctx: self.fctx.clone(),
                ..Default::default()
            },
        );
        let f = files(&self.fetched);
        (lt.parse, lt.files) = (ms(t) - f, f);
        let t = Instant::now();
        d.as_mut().resolve(0.0);
        let f = files(&self.fetched);
        (lt.resolve, lt.files) = (ms(t) - f, lt.files + f);
        let t = Instant::now();
        d.as_mut().resolve(0.0); // again, once the stylesheets and fonts are in
        let f = files(&self.fetched);
        (lt.resolve2, lt.files) = (ms(t) - f, lt.files + f);
        let t = Instant::now();
        if !self.paged[doc] {
            let mut pages = vec![];
            paginate(&blocks(d.as_ref()), doc, opts.margin as f64, &opts, &mut pages);
            let at = self.pages.partition_point(|p| p.doc < doc);
            self.pages.splice(at..at, pages);
            self.paged[doc] = true;
        }
        lt.paginate = ms(t);
        self.last_load = lt;
        self.docs[doc] = Some(d);
    }

    /// Draws page n into `out` (width × height bytes, 8-bit grey, black on white).
    pub fn draw(&mut self, n: usize, out: &mut [u8]) {
        self.draw_parts(n, out, true, true, None);
    }

    /// For showing how a page is drawn (07's `blitz-demo`): one layer of page n, or the page the way it was drawn
    /// before the glyph layer (everything through vello_cpu on a white page); the glyph layer's boxes go to `boxes`.
    pub fn draw_layer(&mut self, n: usize, out: &mut [u8], layer: Layer, boxes: &mut Vec<[i32; 4]>) {
        match layer {
            Layer::Text => self.draw_parts(n, out, false, true, Some(boxes)),
            Layer::Rest => self.draw_parts(n, out, true, false, Some(boxes)),
            Layer::AllVello => {
                let p = self.pages[n];
                self.layout(p.doc);
                let (w, h, m) = (self.opts.width, self.opts.height, self.opts.margin as f64);
                let doc = self.docs[p.doc].as_mut().expect("laid out just now").as_mut();
                doc.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: p.top - m });
                let mut split = glyphs::Split::new(&mut self.glyphs, &mut self.pictures);
                split.text_to_rest = true;
                paint_scene(&mut split, doc, 1.0, w, h, 0, 0);
                let rest = split.rest;
                self.renderer.reset();
                self.renderer.render_to_vec(
                    |scene| {
                        let page = peniko::kurbo::Rect::new(0.0, 0.0, w as f64, h as f64);
                        scene.fill(peniko::Fill::NonZero, Default::default(), peniko::Color::WHITE, None, &page);
                        scene.append_scene(rest, Default::default());
                    },
                    &mut self.rgba,
                );
                let (from, to) = (m as usize, ((m + p.bottom - p.top).ceil() as usize).min(h as usize));
                let w = w as usize;
                out.fill(255);
                for (o, px) in out[from * w..to * w].iter_mut().zip(self.rgba[4 * from * w..].chunks_exact(4)) {
                    *o = px[1];
                }
            }
        }
    }

    /// The glyph masks drawn so far: glyph id, quarter-pixel position, width, height, coverage.
    /// The glyph masks drawn so far, and the time they took (ms).
    pub fn glyph_work(&self) -> (usize, f64) {
        (self.glyphs.made, self.glyphs.made_ms)
    }

    pub fn glyph_masks(&self) -> Vec<(u32, u8, usize, usize, &[u8])> {
        self.glyphs.masks()
    }

    fn draw_parts(&mut self, n: usize, out: &mut [u8], rest: bool, text: bool, boxes: Option<&mut Vec<[i32; 4]>>) {
        let p = self.pages[n];
        self.layout(p.doc);
        let (w, h, m) = (self.opts.width, self.opts.height, self.opts.margin as f64);
        let doc = self.docs[p.doc].as_mut().expect("laid out just now").as_mut();
        doc.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: p.top - m });
        let mut split = glyphs::Split::new(&mut self.glyphs, &mut self.pictures);
        paint_scene(&mut split, doc, 1.0, w, h, 0, 0);
        split.finish();
        // the page's slice only: blank above the top margin's end and below the page's last line
        let (from, to) = (m as usize, ((m + p.bottom - p.top).ceil() as usize).min(h as usize));
        let w = w as usize;
        out[..from * w].fill(255);
        out[to * w..].fill(255);
        if rest && split.rest_draws {
            // what is not plain text (backgrounds, rules, images) through vello_cpu, on transparent; inside the slice
            // the grey is one channel where all three are the same, weighted only for colour
            let rest = std::mem::take(&mut split.rest);
            self.renderer.reset();
            self.renderer.render_to_vec(|scene| scene.append_scene(rest, Default::default()), &mut self.rgba);
            for (o, px) in out[from * w..to * w].iter_mut().zip(self.rgba[4 * from * w..].chunks_exact(4)) {
                let (r, g, b, a) = (px[0], px[1], px[2], px[3]);
                // premultiplied: on white a pixel is its colour plus what it lets through
                let c = if r == g && g == b { g as u32 } else { (r as u32 * 299 + g as u32 * 587 + b as u32 * 114) / 1000 };
                *o = (c + 255 - a as u32).min(255) as u8;
            }
        } else {
            out[from * w..to * w].fill(255);
        }
        if text {
            split.blit(out, w, from, to);
        }
        if let Some(b) = boxes {
            *b = split.boxes();
        }
    }

    /// Where the element with this id in spine document `doc` is: its page (None if not found) and its boxes on that
    /// page, one per line, in page pixels. Lays the document out if it is not. For a block element (a panel's button, a
    /// list's row) the box is its own: what a tap is tested against.
    pub fn find(&mut self, doc: usize, id: &str) -> Option<(usize, Vec<[i32; 4]>)> {
        self.find_boxes(doc, id, false)
    }

    /// As `find`, but a paragraph (or a div of paragraphs) gives the line boxes of its text, one a line, where `find` gives
    /// its one block box: what read-along marks, a bar under each box, so a narrated paragraph is marked in every line
    /// and not under the last only (IDPF's Moby-Dick narrates by paragraph). An inline element (a word, a sentence in a
    /// span) is the same as in `find`: one client rect a line. Only the narration asks for it: the panels find their
    /// buttons by `find`, and a button's text lines are not its box (found 2026-11-05: taps missed).
    pub fn find_lines(&mut self, doc: usize, id: &str) -> Option<(usize, Vec<[i32; 4]>)> {
        self.find_boxes(doc, id, true)
    }

    /// As `find_lines`, but every line of the element, on whichever page it is: (page, box in that page's pixels) each,
    /// in order. For a narrated paragraph or sentence that runs onto the next page, whose page read-along turns when the
    /// voice reaches its first line there (2026-11-08; until then the page turned with the next element only).
    pub fn find_lines_all(&mut self, doc: usize, id: &str) -> Option<Vec<(usize, [i32; 4])>> {
        let rects = self.element_rects(doc, id, true)?;
        let m = self.opts.margin as f64;
        let mut out = vec![];
        for &(x, ry, w, h) in &rects {
            if w <= 0.0 {
                continue;
            }
            let Some(page) = self.page_at(doc, ry + h / 2.0) else { continue };
            let y = ry - self.pages[page].top + m;
            out.push((page, [x.floor() as i32, y.floor() as i32, (x + w).ceil() as i32, (y + h).ceil() as i32]));
        }
        Some(out)
    }

    /// The element's rects in its document's coordinates (x, y, width, height): with `lines`, a paragraph's line boxes
    /// (`find_lines`), else, or when it has none, its client rects. Lays the document out if it is not.
    fn element_rects(&mut self, doc: usize, id: &str, lines: bool) -> Option<Vec<(f64, f64, f64, f64)>> {
        if doc >= self.docs.len() {
            return None;
        }
        self.layout(doc);
        let d = self.docs[doc].as_mut()?.as_mut();
        d.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: 0.0 });
        let node = d.get_element_by_id(id)?;
        let mut rects: Vec<(f64, f64, f64, f64)> = vec![];
        if lines {
            line_rects(d, node, &mut rects);
        }
        if rects.is_empty() {
            rects = d.node_client_rects(node).iter().map(|r| (r.x, r.y, r.width, r.height)).collect();
        }
        Some(rects)
    }

    fn find_boxes(&mut self, doc: usize, id: &str, lines: bool) -> Option<(usize, Vec<[i32; 4]>)> {
        let rects = self.element_rects(doc, id, lines)?;
        let first = rects.first()?;
        let page = self.page_at(doc, first.1 + first.3 / 2.0)?;
        let p = self.pages[page];
        let m = self.opts.margin as f64;
        let boxes = rects
            .iter()
            .filter(|r| self.page_at(doc, r.1 + r.3 / 2.0) == Some(page) && r.2 > 0.0)
            .map(|&(x, ry, w, h)| {
                let y = ry - p.top + m;
                [x.floor() as i32, y.floor() as i32, (x + w).ceil() as i32, (y + h).ceil() as i32]
            })
            .collect();
        Some((page, boxes))
    }

    /// The words on page n, line by line, each in visual order: where each is in its document's text (which stays
    /// when the book is laid out again) and its box on the page, the height of its line. A word is the text between
    /// spaces within one run of glyphs (a word in two styles is two).
    pub fn words(&mut self, n: usize) -> Vec<Word> {
        let Some(&p) = self.pages.get(n) else { return vec![] };
        self.layout(p.doc);
        let m = self.opts.margin as f64;
        let mut out = vec![];
        let Some(d) = self.docs[p.doc].as_mut() else { return out };
        let d = d.as_mut();
        d.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: 0.0 });
        inline_roots(d, &mut |block, node, tl| {
            let pos = node.absolute_position(0.0, 0.0);
            let l = node.final_layout();
            let (x0, y0) = ((pos.x + l.padding.left + l.border.left) as f64, (pos.y + l.padding.top + l.border.top + tl.block_offset) as f64);
            for line in tl.layout.lines() {
                let lm = line.metrics();
                let (top, bottom) = (y0 + lm.block_min_coord as f64, y0 + lm.block_max_coord as f64);
                if (top + bottom) / 2.0 < p.top || (top + bottom) / 2.0 >= p.bottom {
                    continue;
                }
                let (y, yb) = ((top - p.top + m).floor() as i32, (bottom - p.top + m).ceil() as i32);
                // a run cut into glyph runs by its styles (a span a word, in a narrated book) walked once, from where its
                // first part starts: each part's clusters are the whole run's
                let mut runs = vec![];
                for item in line.items() {
                    let parley::PositionedLayoutItem::GlyphRun(gr) = item else { continue };
                    if runs.contains(&gr.run().index()) {
                        continue;
                    }
                    runs.push(gr.run().index());
                    let mut x = x0 + gr.offset() as f64;
                    let mut word: Option<Word> = None;
                    for c in gr.run().visual_clusters() {
                        let adv = c.advance() as f64;
                        if c.is_space_or_nbsp() || c.is_hard_line_break() {
                            out.extend(word.take());
                        } else {
                            let r = c.text_range();
                            let w = word.get_or_insert(Word { block: block as u32, start: r.start as u32, end: r.end as u32, rect: [x.floor() as i32, y, 0, yb] });
                            (w.start, w.end) = (w.start.min(r.start as u32), w.end.max(r.end as u32));
                            w.rect[2] = (x + adv).ceil() as i32;
                        }
                        x += adv;
                    }
                    out.extend(word);
                }
            }
        });
        out
    }

    /// The text of spine document `doc` from (block, byte) `from` to `to`, as `words` gives them (blocks joined by a
    /// line break).
    pub fn text(&mut self, doc: usize, from: (u32, u32), to: (u32, u32)) -> String {
        if doc >= self.docs.len() {
            return String::new();
        }
        self.layout(doc);
        let mut out = String::new();
        let Some(d) = self.docs[doc].as_ref() else { return out };
        inline_roots(d.as_ref(), &mut |block, _, tl| {
            let b = block as u32;
            if b < from.0 || b > to.0 {
                return;
            }
            let s = if b == from.0 { from.1 as usize } else { 0 };
            let e = if b == to.0 { to.1 as usize } else { tl.text.len() };
            if let Some(t) = tl.text.get(s.min(e)..e.min(tl.text.len())) {
                if b > from.0 {
                    out.push('\n');
                }
                out.push_str(t);
            }
        });
        out
    }

    /// The line spacing of spine document `doc` as laid out (with the book's own, or the one set): its most common
    /// height of a line of text, over the text size (0 if it has no text). Lays the document out if it is not.
    pub fn line_spacing(&mut self, doc: usize) -> f32 {
        if doc >= self.docs.len() {
            return 0.0;
        }
        self.layout(doc);
        let Some(d) = self.docs[doc].as_ref() else { return 0.0 };
        let mut counts = std::collections::HashMap::<i64, usize>::new();
        for b in blocks(d.as_ref()) {
            for l in b.lines.iter().filter(|l| l.text) {
                *counts.entry(((l.bottom - l.top) * 2.0).round() as i64).or_default() += 1; // to half a pixel
            }
        }
        counts.into_iter().max_by_key(|&(h, n)| (n, h)).map_or(0.0, |(h, _)| h as f32 / 2.0 / self.opts.font_size)
    }

    fn page_at(&self, doc: usize, y: f64) -> Option<usize> {
        self.pages.iter().position(|p| p.doc == doc && y >= p.top && y < p.bottom)
    }
}

/// The fonts in a folder, each as its generic family by its file name ("Mono", "Serif" or "Sans" in it; others are
/// skipped), for `Book::new`; in the order of their names, so a generic family's first font is the same on every
/// system (a folder's own order is the file system's: the Kobo's lists by name, Docker's not). Two fonts are for one
/// script each, Hebrew (Frank Ruhl Libre) and the Arabic script's languages other than Farsi (Noto Naskh Arabic):
/// after the others in both serif and sans, first in a book in their script (`BookScript`).
pub fn fallback_fonts(dir: &str) -> Vec<(&'static str, Vec<u8>)> {
    let mut out = vec![];
    let mut entries: Vec<_> = std::fs::read_dir(dir).unwrap().flatten().collect();
    entries.sort_by_key(|e| e.file_name());
    for e in entries {
        let name = e.file_name().to_string_lossy().into_owned();
        let generic = if name.starts_with("Parastoo") {
            "serif+sans" // one family (no separate sans and serif designs): both generics, so it is one file, not two copies
        } else if name.starts_with("FrankRuhlLibre") {
            "hebrew"
        } else if name.starts_with("NotoNaskhArabic") {
            "arabic"
        } else if name.contains("Mono") {
            "monospace"
        } else if name.contains("Serif") {
            "serif"
        } else if name.contains("Sans") {
            "sans-serif"
        } else if name.starts_with("Vazirmatn") {
            "named" // the interface's font: found by its name (ui.css), in no generic family, so books never fall back to it
        } else {
            continue;
        };
        out.push((generic, std::fs::read(e.path()).unwrap()));
    }
    out
}

/// A font context holding the given fallback fonts as the generic families (None: Blitz's own). A script's own font
/// ("hebrew", "arabic") comes after the others, or first in a book in that script, so that its words are in one font
/// (Parastoo has most of Urdu's letters but not all: after it, Noto Naskh would draw only the letters it lacks).
fn font_context(fonts: &[(&str, Vec<u8>)], script: BookScript) -> Option<parley::FontContext> {
    use parley::fontique::{Blob, GenericFamily as G};
    if fonts.is_empty() {
        return None;
    }
    let first = match script {
        BookScript::Hebrew => "hebrew",
        BookScript::Arabic => "arabic",
        BookScript::Other => "",
    };
    let rank = |generic: &str| match generic {
        g if g == first => 0,
        "hebrew" | "arabic" => 2,
        _ => 1,
    };
    let mut order: Vec<&(&str, Vec<u8>)> = fonts.iter().collect();
    order.sort_by_key(|(g, _)| rank(g)); // stable: the files' own order within each rank
    let mut ctx = parley::FontContext::default();
    let mut families: Vec<(G, parley::fontique::FamilyId)> = vec![];
    for (generic, data) in order {
        let gs: &[G] = match *generic {
            "serif" => &[G::Serif],
            "serif+sans" | "hebrew" | "arabic" => &[G::Serif, G::SansSerif, G::SystemUi, G::UiSansSerif],
            "sans-serif" => &[G::SansSerif, G::SystemUi, G::UiSansSerif],
            "monospace" => &[G::Monospace, G::UiMonospace],
            _ => &[],
        };
        for (id, _) in ctx.collection.register_fonts(Blob::new(Arc::new(data.clone())), None) {
            for &g in gs {
                if !families.contains(&(g, id)) {
                    families.push((g, id));
                }
            }
        }
    }
    for g in [G::Serif, G::SansSerif, G::SystemUi, G::UiSansSerif, G::Monospace, G::UiMonospace] {
        ctx.collection.set_generic_families(g, families.iter().filter(|(h, _)| *h == g).map(|(_, id)| *id));
    }
    Some(ctx)
}

/// The line boxes of a laid-out document, block by block, in document order.
/// A word on a page (`Book::words`): its document's inline block (counted in document order) and its bytes in that
/// block's text, and its box (left, top, right, bottom).
#[derive(Clone, Copy, Debug)]
pub struct Word {
    pub block: u32,
    pub start: u32,
    pub end: u32,
    pub rect: [i32; 4],
}

/// The lines of the text layouts at or under node `id` (its own, when it is an inline root, a paragraph; those of the
/// paragraphs in it, when it holds blocks), each as (x, y, width, height) in document pixels. Empty for an inline element
/// inside a paragraph (a span): it has no layout of its own.
fn line_rects(doc: &BaseDocument, id: blitz_dom::NodeId, out: &mut Vec<(f64, f64, f64, f64)>) {
    let Some(node) = doc.get_node(id) else { return };
    if node.flags.is_inline_root() {
        if let Some(tl) = node.element_data().and_then(|e| e.inline_layout_data.as_ref()) {
            let pos = node.absolute_position(0.0, 0.0);
            let l = node.final_layout();
            let x0 = (pos.x + l.padding.left + l.border.left) as f64;
            let y0 = (pos.y + l.padding.top + l.border.top + tl.block_offset) as f64;
            for line in tl.layout.lines() {
                let m = line.metrics();
                let (top, bottom) = (y0 + m.block_min_coord as f64, y0 + m.block_max_coord as f64);
                out.push((x0 + m.inline_min_coord as f64, top, (m.inline_max_coord - m.inline_min_coord) as f64, bottom - top));
            }
        }
        return;
    }
    for &c in node.children.iter() {
        line_rects(doc, c, out);
    }
}

/// Calls f with each inline root that has a text layout, in document order, and its number in that order.
fn inline_roots(doc: &BaseDocument, f: &mut dyn FnMut(usize, &blitz_dom::Node, &blitz_dom::node::TextLayout)) {
    fn walk(doc: &BaseDocument, id: blitz_dom::NodeId, n: &mut usize, f: &mut dyn FnMut(usize, &blitz_dom::Node, &blitz_dom::node::TextLayout)) {
        let Some(node) = doc.get_node(id) else { return };
        if node.flags.is_inline_root() {
            if let Some(tl) = node.element_data().and_then(|e| e.inline_layout_data.as_ref()) {
                f(*n, node, tl);
                *n += 1;
                return;
            }
        }
        for &c in node.children.iter() {
            walk(doc, c, n, f);
        }
    }
    walk(doc, doc.root_node().id, &mut 0, f);
}

fn blocks(doc: &BaseDocument) -> Vec<Block> {
    let mut out = vec![];
    collect(doc, doc.root_node().id, &mut out);
    out
}

fn collect(doc: &BaseDocument, id: blitz_dom::NodeId, out: &mut Vec<Block>) {
    let Some(node) = doc.get_node(id) else { return };
    // a picture outside any line (an <img> straight in the body, an <svg>): a block of one line without text, its box
    if node.element_data().is_some_and(|e| matches!(&*e.name.local, "img" | "svg" | "video" | "canvas")) {
        let (pos, l) = (node.absolute_position(0.0, 0.0), node.final_layout());
        if l.size.height > 0.0 {
            let line = LineBox { top: pos.y as f64, bottom: (pos.y + l.size.height) as f64, text: false };
            out.push(Block { lines: vec![line], avoid_after: false, avoid_inside: false });
        }
        return;
    }
    if node.flags.is_inline_root() {
        if let Some(tl) = node.element_data().and_then(|e| e.inline_layout_data.as_ref()) {
            let pos = node.absolute_position(0.0, 0.0);
            let l = node.final_layout();
            let top = (pos.y + l.padding.top + l.border.top + tl.block_offset) as f64;
            let lines = tl
                .layout
                .lines()
                .map(|line| {
                    let m = line.metrics();
                    let text = line.items().any(|it| matches!(it, parley::PositionedLayoutItem::GlyphRun(_)));
                    LineBox { top: top + m.block_min_coord as f64, bottom: top + m.block_max_coord as f64, text }
                })
                .collect::<Vec<_>>();
            let heading = node
                .element_data()
                .map(|e| matches!(e.name.local.as_ref(), "h1" | "h2" | "h3" | "h4" | "h5" | "h6"))
                .unwrap_or(false);
            if !lines.is_empty() {
                out.push(Block { lines, avoid_after: heading, avoid_inside: heading });
            }
            return; // its inline content is in these lines
        }
    }
    for &c in node.children.iter() {
        collect(doc, c, out);
    }
}

/// Cuts one document's lines into pages, as experiment 03's Paginate does. `start` is where the document's content
/// begins (the first page keeps the first block's margin; later pages start at their first line).
fn paginate(blocks: &[Block], doc: usize, start: f64, o: &Options, pages: &mut Vec<Page>) {
    let h = (o.height - 2 * o.margin) as f64;
    const EPS: f64 = 0.01;
    let mut cur = Page { doc, top: start, bottom: start, lines: 0 };
    let mut placed = 0; // line boxes on the current page
    let new_page = |cur: &mut Page, placed: &mut usize, top: f64, pages: &mut Vec<Page>| {
        pages.push(*cur);
        *cur = Page { doc, top, bottom: top, lines: 0 };
        *placed = 0;
    };
    for (i, b) in blocks.iter().enumerate() {
        let n = b.lines.len();
        let fits = |from: usize, top: f64| b.lines[from..].iter().take_while(|l| l.bottom - top <= h + EPS).count();
        // keep a heading with the start of the next block
        if b.avoid_after && placed > 0 {
            if let Some(next) = blocks.get(i + 1) {
                let k = next.lines.len().min(o.orphans);
                if next.lines[k - 1].bottom - cur.top > h + EPS {
                    let top = b.lines[0].top;
                    new_page(&mut cur, &mut placed, top, pages);
                }
            }
        }
        let (orphans, widows) = (o.orphans as i64, o.widows as i64);
        let mut k = 0;
        while k < n {
            // signed, as in experiment 03: "rest - widows" below zero means "not here"
            let rest = (n - k) as i64;
            let mut room = fits(k, cur.top) as i64;
            if room >= rest {
                room = rest;
            } else if placed == 0 {
                // a page of its own is not enough: split anyway, keeping widows if possible
                if rest - room < widows && rest > widows {
                    room = rest - widows;
                }
            } else if b.avoid_inside && k == 0 {
                room = 0;
            } else {
                // widows give way to orphans (as in Chromium)
                if rest - room < widows && (k > 0 || rest - widows >= orphans) {
                    room = rest - widows;
                }
                if k == 0 && room < orphans {
                    room = 0;
                }
            }
            let mut room = room.max(0) as usize;
            if room == 0 {
                if placed == 0 {
                    room = 1;
                } else {
                    let top = b.lines[k].top;
                    new_page(&mut cur, &mut placed, top, pages);
                    continue;
                }
            }
            for l in &b.lines[k..k + room] {
                // blocks can overlap (Holmes's title page: a heading under its picture), so a page never ends higher
                cur.bottom = cur.bottom.max(l.bottom);
                if l.text {
                    cur.lines += 1;
                }
            }
            placed += room;
            k += room;
            if k < n {
                let top = b.lines[k].top;
                new_page(&mut cur, &mut placed, top, pages);
            }
        }
    }
    // a document with nothing to show (Haji Agha's navigation document) gets no page
    if placed > 0 {
        pages.push(cur);
    }
}
