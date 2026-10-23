//! Stage 0 probe for experiment 04: Blitz (Stylo, Taffy, Parley) on one chapter from disk, with the book's own
//! stylesheet and fonts, at experiment 03's page width and text size; renders the top of the chapter, one page high,
//! with its CPU renderer (vello_cpu) to PNG and times the steps.
//!
//!     blitz-probe CHAPTER.xhtml OUT.png

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

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let (chapter, out) = (&args[1], &args[2]);
    let path = std::fs::canonicalize(chapter).unwrap();
    let base = url::Url::from_file_path(&path).unwrap().to_string();
    // the page settings experiments 01–03 give Chromium: 34 px text, 60 px side margins
    let html = std::fs::read_to_string(&path)
        .unwrap()
        .replace("</head>", "<style>html { font-size: 34px; padding: 60px 60px 0 60px; }</style></head>");

    let t = Instant::now();
    let mut doc = HtmlDocument::from_html(
        &html,
        DocumentConfig {
            base_url: Some(base),
            net_provider: Some(Arc::new(Files) as _),
            viewport: Some(Viewport::new(W, H, 1.0, ColorScheme::Light)),
            ..Default::default()
        },
    );
    let parsed = t.elapsed();
    doc.as_mut().resolve(0.0);
    doc.as_mut().resolve(0.0);
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
