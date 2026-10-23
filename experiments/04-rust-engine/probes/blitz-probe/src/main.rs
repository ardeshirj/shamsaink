//! Probe for experiment 04: Blitz (Stylo, Taffy, Parley) on books from disk, with their own stylesheets and fonts,
//! at experiment 03's page width and text size (the settings 03 gives Chromium).
//!
//!     blitz-probe CHAPTER.xhtml OUT.png      the top of the chapter, one page high, drawn with vello_cpu; timings
//!     blitz-probe measure SPANNED_DIR OUT_DIR
//!         for a book made by 03's spanwords: every word span's first line box, as OUT_DIR/boxes.json in the format
//!         of 03's cmd/pages (one long page per spine document), for 03's tools/linecmp; timings.json

use anyrender::{PaintScene as _, render_to_buffer};
use anyrender_vello_cpu::VelloCpuImageRenderer;
use blitz_dom::{DocumentConfig, util::Color};
use blitz_html::HtmlDocument;
use blitz_paint::paint_scene;
use blitz_traits::net::{NetHandler, NetProvider, Request};
use blitz_traits::shell::{ColorScheme, Viewport};
use peniko::{Fill, kurbo::Rect};
use std::sync::Arc;
use std::time::Instant;

/// Gives the document its files (stylesheet, fonts) from disk, at once.
struct Files;

impl NetProvider for Files {
    fn fetch(&self, _doc_id: usize, request: Request, handler: Box<dyn NetHandler>) {
        if let Ok(path) = request.url.to_file_path() {
            if let Ok(data) = std::fs::read(&path) {
                handler.bytes(request.url.to_string(), bytes::Bytes::from(data));
                return;
            }
        }
        eprintln!("not found: {}", request.url);
    }
}

const W: u32 = 1072;
const H: u32 = 1448;

/// Parses and lays out a document from disk with the shared settings (34 px text, 60 px side margins; `top` px of
/// top margin), its stylesheets and fonts read through `Files`.
fn load(path: &std::path::Path, top: u32) -> HtmlDocument {
    let base = url::Url::from_file_path(path).unwrap().to_string();
    let css = format!("<style>html {{ font-size: 34px; padding: {top}px 60px 0 60px; }}</style></head>");
    let html = std::fs::read_to_string(path).unwrap().replace("</head>", &css);
    let mut doc = HtmlDocument::from_html(
        &html,
        DocumentConfig {
            base_url: Some(base),
            net_provider: Some(Arc::new(Files) as _),
            viewport: Some(Viewport::new(W, H, 1.0, ColorScheme::Light)),
            ..Default::default()
        },
    );
    doc.as_mut().resolve(0.0);
    doc.as_mut().resolve(0.0);
    doc
}

/// The ids of the word spans (`id="w…"`) in a document, in order.
fn word_ids(html: &str) -> Vec<String> {
    let mut ids = vec![];
    let mut rest = html;
    while let Some(i) = rest.find("id=\"w") {
        let s = &rest[i + 4..];
        let end = s.find('"').unwrap();
        ids.push(s[..end].to_string());
        rest = &s[end..];
    }
    ids
}

fn measure(dir: &str, out: &str) {
    std::fs::create_dir_all(out).unwrap();
    let spine = std::fs::read_to_string(format!("{dir}/spine.txt")).unwrap();
    let mut boxes = vec![];
    let t = Instant::now();
    for (si, file) in spine.lines().filter(|l| !l.is_empty()).enumerate() {
        let path = std::fs::canonicalize(format!("{dir}/{file}")).unwrap();
        let doc = load(&path, 0);
        let html = std::fs::read_to_string(&path).unwrap();
        for id in word_ids(&html) {
            let Some(node) = doc.as_ref().get_element_by_id(&id) else { continue };
            let Some(r) = doc.as_ref().node_client_rects(node).into_iter().next() else { continue };
            boxes.push(format!(
                "{{\"spine\":{si},\"id\":\"{id}\",\"page\":0,\"rects\":[{{\"Min\":{{\"X\":{},\"Y\":{}}},\"Max\":{{\"X\":{},\"Y\":{}}}}}]}}",
                r.x.round(), r.y.round(), (r.x + r.width).round(), (r.y + r.height).round()
            ));
        }
    }
    let ms = t.elapsed().as_secs_f64() * 1e3;
    std::fs::write(format!("{out}/boxes.json"), format!("[{}]", boxes.join(","))).unwrap();
    std::fs::write(format!("{out}/timings.json"), format!("{{\"parse_style_layout_measure_ms\": {ms:.1}}}")).unwrap();
    println!("{} words, {ms:.1} ms", boxes.len());
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args[1] == "measure" {
        return measure(&args[2], &args[3]);
    }
    let (chapter, out) = (&args[1], &args[2]);
    let path = std::fs::canonicalize(chapter).unwrap();
    let t = Instant::now();
    let mut doc = load(&path, 60);
    let parsed = t.elapsed(); // parse and layout together here
    let laid_out = t.elapsed();
    let height = doc.as_ref().root_element().final_layout().size.height;

    let buffer = render_to_buffer::<VelloCpuImageRenderer, _>(
        |scene| {
            scene.fill(Fill::NonZero, Default::default(), Color::WHITE, Default::default(), &Rect::new(0.0, 0.0, W as f64, H as f64));
            paint_scene(scene, doc.as_mut(), 1.0, W, H, 0, 0);
        },
        W,
        H,
    );
    let painted = t.elapsed();

    let f = std::fs::File::create(out).unwrap();
    let mut enc = png::Encoder::new(f, W, H);
    enc.set_color(png::ColorType::Rgba);
    enc.set_depth(png::BitDepth::Eight);
    enc.write_header().unwrap().write_image_data(&buffer).unwrap();
    println!(
        "{{\"parse_ms\": {:.1}, \"style_and_layout_ms\": {:.1}, \"paint_one_page_ms\": {:.1}, \"chapter_height_px\": {height:.0}}}",
        parsed.as_secs_f64() * 1e3,
        (laid_out - parsed).as_secs_f64() * 1e3,
        (painted - laid_out).as_secs_f64() * 1e3
    );
}
