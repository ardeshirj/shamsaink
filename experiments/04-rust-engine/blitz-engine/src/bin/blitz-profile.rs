//! blitz-profile: where Blitz's time and memory go, document by document: parse, the first and second resolve (style
//! and layout), whether the second moved anything (the client rects of every word span), the memory held after each
//! document, and a page drawn in three steps (scene, render, grey). The same settings as `Book::open`.
//!
//!     blitz-profile BOOK_DIR [FONTS_DIR]

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

const ROOT: &str = "file:///book/";

struct Files(std::path::PathBuf);

impl NetProvider for Files {
    fn fetch(&self, _doc_id: usize, request: Request, handler: Box<dyn NetHandler>) {
        if let Some(path) = request.url.as_str().strip_prefix(ROOT) {
            let path = path.split(['#', '?']).next().unwrap_or("").replace("%20", " ");
            if let Ok(data) = std::fs::read(self.0.join(path)) {
                handler.bytes(request.url.to_string(), bytes::Bytes::from(data));
            }
        }
    }
}

fn spine(dir: &std::path::Path) -> Vec<String> {
    let attr = |tag: &str, name: &str| -> Option<String> {
        let i = tag.find(&format!("{name}=\""))? + name.len() + 2;
        Some(tag[i..i + tag[i..].find('"')?].to_string())
    };
    let container = std::fs::read_to_string(dir.join("META-INF/container.xml")).unwrap();
    let opf_path = attr(&container[container.find("<rootfile").unwrap()..], "full-path").unwrap();
    let opf = std::fs::read_to_string(dir.join(&opf_path)).unwrap();
    let base = opf_path.rsplit_once('/').map(|(b, _)| format!("{b}/")).unwrap_or_default();
    let mut hrefs = std::collections::HashMap::new();
    for item in opf.split("<item ").skip(1) {
        let tag = &item[..item.find('>').unwrap()];
        if let (Some(id), Some(href)) = (attr(tag, "id"), attr(tag, "href")) {
            hrefs.insert(id, href);
        }
    }
    opf.split("<itemref ")
        .skip(1)
        .filter_map(|r| attr(&r[..r.find('>').unwrap()], "idref"))
        .filter_map(|id| hrefs.get(&id).map(|h| format!("{base}{h}")))
        .collect()
}

fn rss_mb() -> f64 {
    let s = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    s.lines()
        .find(|l| l.starts_with("VmRSS:"))
        .and_then(|l| l.split_whitespace().nth(1)?.parse::<f64>().ok())
        .map(|kb| kb / 1024.0)
        .unwrap_or(0.0)
}

fn ms(t: Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

/// Every word span's client rects, rounded to 0.01 px.
fn word_rects(doc: &mut HtmlDocument) -> Vec<Vec<[i64; 4]>> {
    let d = doc.as_mut();
    let mut ids = vec![];
    for n in 1.. {
        let id = format!("w{n:05}");
        match d.get_element_by_id(&id) {
            Some(node) => ids.push(node),
            None => break,
        }
    }
    ids.iter()
        .map(|&n| {
            d.node_client_rects(n)
                .iter()
                .map(|r| [(r.x * 100.0) as i64, (r.y * 100.0) as i64, (r.width * 100.0) as i64, (r.height * 100.0) as i64])
                .collect()
        })
        .collect()
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let dir = std::path::PathBuf::from(&args[1]);
    let (w, h, m, size) = (1072u32, 1448u32, 60u32, 34.0f32);
    let base_rss = rss_mb();
    let mut docs = vec![];
    let (mut tp, mut t1, mut t2, mut moved, mut words) = (0.0, 0.0, 0.0, 0usize, 0usize);
    println!("document                       parse   resolve1 resolve2  words moved-by-2nd  rss MB");
    for path in spine(&dir) {
        let html = std::fs::read_to_string(dir.join(&path)).unwrap_or_default();
        let css = format!("<style>html {{ font-size: {size}px; padding: {m}px {m}px {h}px {m}px; }}</style></head>");
        let html = html.replacen("</head>", &css, 1);
        let t = Instant::now();
        let mut doc = HtmlDocument::from_html(
            &html,
            DocumentConfig {
                base_url: Some(format!("{ROOT}{path}")),
                net_provider: Some(Arc::new(Files(dir.clone())) as _),
                viewport: Some(Viewport::new(w, h, 1.0, ColorScheme::Light)),
                ..Default::default()
            },
        );
        let parse = ms(t);
        let t = Instant::now();
        doc.as_mut().resolve(0.0);
        let r1 = ms(t);
        let before = word_rects(&mut doc);
        let t = Instant::now();
        doc.as_mut().resolve(0.0);
        let r2 = ms(t);
        let after = word_rects(&mut doc);
        let n = before.iter().zip(&after).filter(|(a, b)| a != b).count();
        println!(
            "{:30} {parse:6.1} {r1:8.1} {r2:8.1} {:6} {n:6} {:12.1}",
            path.rsplit('/').next().unwrap(),
            before.len(),
            rss_mb() - base_rss
        );
        (tp, t1, t2, moved, words) = (tp + parse, t1 + r1, t2 + r2, moved + n, words + before.len());
        docs.push(doc);
    }
    println!("total: parse {tp:.0} ms, resolve1 {t1:.0} ms, resolve2 {t2:.0} ms; {moved} of {words} words moved by the 2nd; rss {:.0} MB", rss_mb() - base_rss);

    // a page drawn in steps, from the biggest document, three times
    let doc = docs.iter_mut().max_by_key(|d| d.as_ref().tree().len()).unwrap();
    for _ in 0..3 {
        let d = doc.as_mut();
        d.set_viewport_scroll(blitz_dom::Point { x: 0.0, y: 2000.0 });
        let t = Instant::now();
        let mut scene_ms = 0.0;
        let rgba = render_to_buffer::<VelloCpuImageRenderer, _>(
            |scene| {
                let s = Instant::now();
                scene.fill(Fill::NonZero, Default::default(), Color::WHITE, Default::default(), &Rect::new(0.0, 0.0, w as f64, h as f64));
                paint_scene(scene, d, 1.0, w, h, 0, 0);
                scene_ms = ms(s);
            },
            w,
            h,
        );
        let total = ms(t);
        let s = Instant::now();
        let grey: Vec<u8> = rgba
            .chunks_exact(4)
            .map(|p| ((p[0] as u32 * 299 + p[1] as u32 * 587 + p[2] as u32 * 114) / 1000) as u8)
            .collect();
        let grey_ms = ms(s);
        println!("draw: scene {scene_ms:.1} ms, render {:.1} ms, grey {grey_ms:.1} ms ({} px)", total - scene_ms, grey.len());
    }
}
