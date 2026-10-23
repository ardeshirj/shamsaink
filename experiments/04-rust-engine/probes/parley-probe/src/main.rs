//! Stage 0 probe for experiment 04: Parley on one chapter of a book whose words are each in a span (made by
//! experiment 03's spanwords). Lays out every paragraph and heading the way experiment 03 does (34 px, line height
//! 1.9, 918 px wide, justified, right to left), says which word starts each line, compares that with Chromium's word
//! positions (experiment 03's chrome/run.sh), times the layout and draws the chapter's first page.
//!
//!     parley-probe CHAPTER.xhtml FONT_DIR CHROME.json OUT_DIR

use parley::{
    Alignment, AlignmentOptions, BaseDirection, FontContext, FontWeight, LayoutContext, LineHeight,
    PositionedLayoutItem, StyleProperty,
};
use skrifa::{
    instance::{LocationRef, Size},
    outline::{DrawSettings, OutlinePen},
    FontRef, GlyphId, MetadataProvider,
};
use std::collections::HashMap;
use std::time::Instant;
use tiny_skia::{FillRule, Paint, PathBuilder, Pixmap, Transform};

const WIDTH: f32 = 918.0;
const LEFT: f32 = 77.0;
const TOP: f32 = 60.0;
const PAGE_H: f32 = 1328.0;

/// A block of text, with where each word span starts in it (byte offsets).
struct Block {
    heading: bool,
    text: String,
    words: Vec<(usize, String)>,
}

fn decode(s: &str) -> String {
    s.replace("&lt;", "<").replace("&gt;", ">").replace("&quot;", "\"").replace("&#39;", "'").replace("&amp;", "&")
}

/// The <p> and <h1>/<h2> blocks of the chapter, tags removed, whitespace collapsed, word spans noted.
fn blocks(x: &str) -> Vec<Block> {
    let mut out = vec![];
    let mut rest = x;
    while let Some(i) = ["<p", "<h1", "<h2"].iter().filter_map(|t| rest.find(t).map(|i| (i, *t))).min() {
        let (start, tag) = i;
        let name = &tag[1..];
        let open_end = start + rest[start..].find('>').unwrap() + 1;
        let close = open_end + rest[open_end..].find(&format!("</{name}>")).unwrap();
        let inner = &rest[open_end..close];
        let mut b = Block { heading: name != "p", text: String::new(), words: vec![] };
        let mut j = 0;
        let mut space = false;
        while j < inner.len() {
            if inner[j..].starts_with('<') {
                let end = j + inner[j..].find('>').unwrap() + 1;
                let t = &inner[j..end];
                if let Some(k) = t.find("id=\"w") {
                    let id = &t[k + 4..k + 4 + t[k + 4..].find('"').unwrap()];
                    b.words.push((b.text.len(), id.to_string()));
                }
                j = end;
                continue;
            }
            let end = inner[j..].find('<').map(|k| j + k).unwrap_or(inner.len());
            for c in decode(&inner[j..end]).chars() {
                if c.is_whitespace() {
                    if !b.text.is_empty() && !space {
                        b.text.push(' ');
                        space = true;
                    }
                } else {
                    b.text.push(c);
                    space = false;
                }
            }
            j = end;
        }
        // a word span placed right after a collapsed space points at the space: move it to the word
        for w in &mut b.words {
            if b.text[w.0..].starts_with(' ') {
                w.0 += 1;
            }
        }
        out.push(b);
        rest = &rest[close..];
    }
    out
}

struct Pen<'a>(&'a mut PathBuilder, f32, f32); // builder, x, baseline

impl OutlinePen for Pen<'_> {
    fn move_to(&mut self, x: f32, y: f32) { self.0.move_to(self.1 + x, self.2 - y) }
    fn line_to(&mut self, x: f32, y: f32) { self.0.line_to(self.1 + x, self.2 - y) }
    fn quad_to(&mut self, cx: f32, cy: f32, x: f32, y: f32) {
        self.0.quad_to(self.1 + cx, self.2 - cy, self.1 + x, self.2 - y)
    }
    fn curve_to(&mut self, a: f32, b: f32, c: f32, d: f32, x: f32, y: f32) {
        self.0.cubic_to(self.1 + a, self.2 - b, self.1 + c, self.2 - d, self.1 + x, self.2 - y)
    }
    fn close(&mut self) { self.0.close() }
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let (chapter, font_dir, chrome, out) = (&args[1], &args[2], &args[3], &args[4]);
    std::fs::create_dir_all(out).unwrap();
    let x = std::fs::read_to_string(chapter).unwrap();
    let bs = blocks(&x);

    let mut fcx = FontContext::new();
    for f in ["Parastoo.ttf", "Parastoo-Bold.ttf"] {
        let data = std::fs::read(format!("{font_dir}/{f}")).unwrap();
        fcx.collection.register_fonts(data.into(), None);
    }
    let mut lcx: LayoutContext<()> = LayoutContext::new();

    // lay out every block; keep the line starts and the layouts for drawing
    let t = Instant::now();
    let mut layouts = vec![];
    for b in &bs {
        let size = if b.heading { 34.0 * 1.25 } else { 34.0 };
        let mut builder = lcx.ranged_builder(&mut fcx, &b.text, 1.0, false);
        builder.set_base_direction(BaseDirection::Rtl);
        builder.push_default(StyleProperty::FontFamily("Parastoo".into()));
        builder.push_default(StyleProperty::FontSize(size));
        builder.push_default(StyleProperty::LineHeight(LineHeight::FontSizeRelative(1.9)));
        if b.heading {
            builder.push_default(StyleProperty::FontWeight(FontWeight::BOLD));
        }
        let mut layout = builder.build(&b.text);
        layout.break_all_lines(Some(WIDTH));
        let align = if b.heading { Alignment::Center } else { Alignment::Justify };
        layout.align(align, AlignmentOptions::default());
        layouts.push(layout);
    }
    let layout_ms = t.elapsed().as_secs_f64() * 1000.0;

    // which word starts each line, ours and Chromium's
    let rows: Vec<serde_json::Value> = serde_json::from_str(&std::fs::read_to_string(chrome).unwrap()).unwrap();
    let top: HashMap<String, i64> = rows.iter().map(|r| (r[0].as_str().unwrap().to_string(), r[1].as_i64().unwrap())).collect();
    let (mut same, mut ours_only, mut theirs_only, mut lines) = (0, 0, 0, 0);
    for (b, layout) in bs.iter().zip(&layouts) {
        let starts: Vec<usize> = layout.lines().map(|l| l.text_range().start).collect();
        lines += starts.len();
        let mut prev_top = None;
        for (off, id) in &b.words {
            let ours = starts.contains(off);
            let t = top[id];
            let theirs = prev_top.map_or(true, |p: i64| (t - p).abs() > 8);
            prev_top = Some(t);
            match (ours, theirs) {
                (true, true) => same += 1,
                (true, false) => ours_only += 1,
                (false, true) => theirs_only += 1,
                _ => {}
            }
        }
    }

    // the first page: blocks stacked with experiment 03's gaps (p: 17 px after; h2: 63.75 before, 42.5 after)
    let mut px = Pixmap::new(1072, 1448).unwrap();
    px.fill(tiny_skia::Color::WHITE);
    let mut paint = Paint::default();
    paint.set_color_rgba8(0, 0, 0, 255);
    paint.anti_alias = true;
    let mut y = 63.75;
    let mut fonts: HashMap<usize, FontRef> = HashMap::new();
    'page: for (b, layout) in bs.iter().zip(&layouts) {
        let block_y = y; // Parley's glyph y is from the top of its own layout
        for line in layout.lines() {
            let m = line.metrics();
            if y + m.line_height > PAGE_H {
                break 'page;
            }
            for item in line.items() {
                let PositionedLayoutItem::GlyphRun(gr) = item else { continue };
                let run = gr.run();
                let fd = run.font();
                let key = fd.data.as_ref().as_ptr() as usize;
                let font = fonts.entry(key).or_insert_with(|| {
                    let bytes: &'static [u8] = Box::leak(fd.data.as_ref().to_vec().into_boxed_slice());
                    FontRef::from_index(bytes, fd.index).unwrap()
                });
                let outlines = font.outline_glyphs();
                let mut pb = PathBuilder::new();
                for g in gr.positioned_glyphs() {
                    if let Some(o) = outlines.get(GlyphId::new(g.id)) {
                        let settings = DrawSettings::unhinted(Size::new(run.font_size()), LocationRef::default());
                        let _ = o.draw(settings, &mut Pen(&mut pb, LEFT + g.x, TOP + block_y + g.y));
                    }
                }
                if let Some(path) = pb.finish() {
                    px.fill_path(&path, &paint, FillRule::Winding, Transform::identity(), None);
                }
            }
            y += m.line_height;
        }
        y += if b.heading { 42.5 } else { 17.0 };
    }
    px.save_png(format!("{out}/page-1.png")).unwrap();

    let total = same + ours_only + theirs_only;
    println!(
        "{{\"blocks\": {}, \"lines\": {lines}, \"layout_ms\": {layout_ms:.2}, \"line_starts_same\": {same}, \"only_parley\": {ours_only}, \"only_chromium\": {theirs_only}, \"same_pct\": {:.1}}}",
        bs.len(),
        100.0 * same as f64 / total as f64
    );
}
