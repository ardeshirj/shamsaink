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

pub mod capi;
mod glyphs;

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

/// Documents get URLs under this root, so that relative links resolve to paths in the book.
const ROOT: &str = "file:///book/";

/// Gives Blitz the book's files (stylesheets, fonts, images) from the Source, at once.
struct Files(Arc<dyn Source>);

impl NetProvider for Files {
    fn fetch(&self, _doc_id: usize, request: Request, handler: Box<dyn NetHandler>) {
        let url = request.url.as_str();
        if let Some(path) = url.strip_prefix(ROOT) {
            let path = percent_decode(path.split(['#', '?']).next().unwrap_or(""));
            if let Some(data) = self.0.read(&path) {
                handler.bytes(request.url.to_string(), bytes::Bytes::from(data));
            }
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
    pub orphans: usize,
    pub widows: usize,
}

impl Default for Options {
    fn default() -> Self {
        Options { width: 1072, height: 1448, margin: 60, font_size: 34.0, orphans: 2, widows: 2 }
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

/// A page: a slice [top, bottom) of one document.
#[derive(Clone, Copy, Debug)]
pub struct Page {
    pub doc: usize,
    pub top: f64,
    pub bottom: f64,
    pub lines: usize, // lines of text on it
}

pub struct Book {
    docs: Vec<HtmlDocument>,
    pub pages: Vec<Page>,
    pub(crate) opts: Options,
    // kept between pages: the glyph masks, vello_cpu's renderer (its caches) and the colour page for what is not text
    glyphs: glyphs::Cache,
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
        let fctx = font_context(fonts);
        let mut docs = vec![];
        let mut pages = vec![];
        for (i, path) in spine.iter().enumerate() {
            let html = src.read(path).map(|b| String::from_utf8_lossy(&b).into_owned()).unwrap_or_default();
            // the shared page settings: text size and margins (the top one as the first page's), and padding below
            // so the painter can scroll the last page to the top
            let css = format!(
                "<style>html {{ font-size: {}px; padding: {m}px {m}px {}px {m}px; }}</style></head>",
                opts.font_size,
                opts.height,
                m = opts.margin
            );
            let html = html.replacen("</head>", &css, 1);
            let mut doc = HtmlDocument::from_html(
                &html,
                DocumentConfig {
                    base_url: Some(format!("{ROOT}{path}")),
                    net_provider: Some(Arc::new(Files(Arc::clone(&src))) as _),
                    viewport: Some(Viewport::new(opts.width, opts.height, 1.0, ColorScheme::Light)),
                    font_ctx: fctx.clone(),
                    ..Default::default()
                },
            );
            doc.as_mut().resolve(0.0);
            doc.as_mut().resolve(0.0); // again, once the stylesheets and fonts are in
            let blocks = blocks(doc.as_ref());
            paginate(&blocks, i, opts.margin as f64, &opts, &mut pages);
            docs.push(doc);
        }
        let renderer = VelloCpuImageRenderer::new(opts.width, opts.height);
        Book { docs, pages, opts, glyphs: Default::default(), renderer, rgba: vec![] }
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
                let (w, h, m) = (self.opts.width, self.opts.height, self.opts.margin as f64);
                let doc = self.docs[p.doc].as_mut();
                doc.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: p.top - m });
                self.renderer.reset();
                self.renderer.render_to_vec(
                    |scene| {
                        let page = peniko::kurbo::Rect::new(0.0, 0.0, w as f64, h as f64);
                        scene.fill(peniko::Fill::NonZero, Default::default(), peniko::Color::WHITE, None, &page);
                        paint_scene(scene, doc, 1.0, w, h, 0, 0);
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
    pub fn glyph_masks(&self) -> Vec<(u32, u8, usize, usize, &[u8])> {
        self.glyphs.masks()
    }

    fn draw_parts(&mut self, n: usize, out: &mut [u8], rest: bool, text: bool, boxes: Option<&mut Vec<[i32; 4]>>) {
        let p = self.pages[n];
        let (w, h, m) = (self.opts.width, self.opts.height, self.opts.margin as f64);
        let doc = self.docs[p.doc].as_mut();
        doc.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: p.top - m });
        let mut split = glyphs::Split::new(&mut self.glyphs);
        paint_scene(&mut split, doc, 1.0, w, h, 0, 0);
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
    /// page, one per line, in page pixels.
    pub fn find(&mut self, doc: usize, id: &str) -> Option<(usize, Vec<[i32; 4]>)> {
        let d = self.docs.get_mut(doc)?.as_mut();
        d.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: 0.0 });
        let node = d.get_element_by_id(id)?;
        let rects = d.node_client_rects(node);
        let first = rects.first()?;
        let page = self.page_at(doc, first.y + first.height / 2.0)?;
        let p = self.pages[page];
        let m = self.opts.margin as f64;
        let boxes = rects
            .iter()
            .filter(|r| self.page_at(doc, r.y + r.height / 2.0) == Some(page) && r.width > 0.0)
            .map(|r| {
                let y = r.y - p.top + m;
                [r.x.floor() as i32, y.floor() as i32, (r.x + r.width).ceil() as i32, (y + r.height).ceil() as i32]
            })
            .collect();
        Some((page, boxes))
    }

    fn page_at(&self, doc: usize, y: f64) -> Option<usize> {
        self.pages.iter().position(|p| p.doc == doc && y >= p.top && y < p.bottom)
    }
}

/// A font context holding the given fallback fonts as the generic families (None: Blitz's own).
fn font_context(fonts: &[(&str, Vec<u8>)]) -> Option<parley::FontContext> {
    use parley::fontique::{Blob, GenericFamily as G};
    if fonts.is_empty() {
        return None;
    }
    let mut ctx = parley::FontContext::default();
    let mut families: Vec<(G, parley::fontique::FamilyId)> = vec![];
    for (generic, data) in fonts {
        let gs: &[G] = match *generic {
            "serif" => &[G::Serif],
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
