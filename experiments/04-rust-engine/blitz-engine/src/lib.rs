//! A paginated layout engine on Blitz (Stylo for CSS, Taffy for boxes, Parley for text). Blitz lays each spine
//! document out as one long page; this crate cuts it into pages between line boxes, with the rules experiment 03
//! matches Chromium on, draws a page by scrolling Blitz's painter to it, and finds an element's page and boxes. It
//! reads the book through a `Source`, so the reader (Go, with readium/go-toolkit) stays in charge of the EPUB.

use anyrender::{PaintScene as _, render_to_buffer};
use anyrender_vello_cpu::VelloCpuImageRenderer;
use blitz_dom::{BaseDocument, DocumentConfig, util::Color};
use blitz_html::HtmlDocument;
use blitz_paint::paint_scene;
use blitz_traits::net::{NetHandler, NetProvider, Request};
use blitz_traits::shell::{ColorScheme, Viewport};
use peniko::{Fill, kurbo::Rect};
use std::sync::Arc;

pub mod capi;

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
}

impl Book {
    /// Lays out every spine document (paths from the EPUB's root) and paginates it.
    pub fn open(src: Arc<dyn Source>, spine: &[String], opts: Options) -> Book {
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
                    ..Default::default()
                },
            );
            doc.as_mut().resolve(0.0);
            doc.as_mut().resolve(0.0); // again, once the stylesheets and fonts are in
            let blocks = blocks(doc.as_ref());
            paginate(&blocks, i, opts.margin as f64, &opts, &mut pages);
            docs.push(doc);
        }
        Book { docs, pages, opts }
    }

    /// Draws page n into `out` (width × height bytes, 8-bit grey, black on white).
    pub fn draw(&mut self, n: usize, out: &mut [u8]) {
        let p = self.pages[n];
        let (w, h, m) = (self.opts.width, self.opts.height, self.opts.margin as f64);
        let doc = self.docs[p.doc].as_mut();
        doc.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: p.top - m });
        let rgba = render_to_buffer::<VelloCpuImageRenderer, _>(
            |scene| {
                scene.fill(Fill::NonZero, Default::default(), Color::WHITE, Default::default(), &Rect::new(0.0, 0.0, w as f64, h as f64));
                paint_scene(scene, doc, 1.0, w, h, 0, 0);
            },
            w,
            h,
        );
        // the page's slice only: blank above the top margin's end and below the page's last line
        let (from, to) = (m as usize, (m + p.bottom - p.top).ceil() as usize);
        for y in 0..h as usize {
            for x in 0..w as usize {
                let i = y * w as usize + x;
                out[i] = if y < from || y >= to {
                    255
                } else {
                    let (r, g, b) = (rgba[4 * i] as u32, rgba[4 * i + 1] as u32, rgba[4 * i + 2] as u32);
                    ((r * 299 + g * 587 + b * 114) / 1000) as u8
                };
            }
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

/// The line boxes of a laid-out document, block by block, in document order.
fn blocks(doc: &BaseDocument) -> Vec<Block> {
    let mut out = vec![];
    collect(doc, doc.root_node().id, &mut out);
    out
}

fn collect(doc: &BaseDocument, id: blitz_dom::NodeId, out: &mut Vec<Block>) {
    let Some(node) = doc.get_node(id) else { return };
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
                cur.bottom = l.bottom;
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
    if placed > 0 || !pages.iter().any(|p| p.doc == doc) {
        pages.push(cur);
    }
}
