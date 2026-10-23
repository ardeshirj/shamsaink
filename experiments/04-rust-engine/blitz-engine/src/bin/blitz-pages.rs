//! blitz-pages: lays out a whole unzipped EPUB with the Blitz engine and draws every page. Writes page-NNN.png,
//! timings.json, pages.json (each page's spine item and lines of text) and, with WORDS.json (from experiment 03's
//! spanwords), boxes.json: the same files as experiment 03's cmd/pages, for its tools/linecmp.
//!
//! FONTS_DIR, if given, holds fallback fonts for the generic families: files named *Serif*, *Sans*, *Mono*.
//!
//!     blitz-pages BOOK_DIR OUT_DIR [WORDS.json|-] [FONTS_DIR]

use blitz_engine::{Book, Dir, Options};
use std::sync::Arc;
use std::time::Instant;

/// The spine's documents, from the OPF the container names (paths from the EPUB's root).
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

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let (dir, out) = (std::path::PathBuf::from(&args[1]), &args[2]);
    std::fs::create_dir_all(out).unwrap();
    let opts = Options::default();
    let t = Instant::now();
    let files = spine(&dir);
    let fonts = args.get(4).map(|d| fallback_fonts(d)).unwrap_or_default();
    let mut book = Book::open_with_fonts(Arc::new(Dir(dir.clone())), &files, opts, &fonts);
    let layout_ms = t.elapsed().as_secs_f64() * 1e3;

    let (w, h) = (opts.width as usize, opts.height as usize);
    let mut px = vec![0u8; w * h];
    let mut draws = vec![];
    for n in 0..book.pages.len() {
        let s = Instant::now();
        book.draw(n, &mut px);
        draws.push(s.elapsed().as_secs_f64() * 1e3);
        let f = std::fs::File::create(format!("{out}/page-{:03}.png", n + 1)).unwrap();
        let mut enc = png::Encoder::new(f, w as u32, h as u32);
        enc.set_color(png::ColorType::Grayscale);
        enc.write_header().unwrap().write_image_data(&px).unwrap();
    }
    let first = draws[0];
    draws.sort_by(|a, b| a.partial_cmp(b).unwrap());

    let pages: Vec<String> =
        book.pages.iter().map(|p| format!("{{\"spine\":{},\"lines\":{}}}", p.doc, p.lines)).collect();
    std::fs::write(format!("{out}/pages.json"), format!("[{}]", pages.join(","))).unwrap();

    if let Some(words) = args.get(3).filter(|w| *w != "-") {
        // [{"spine":N,"file":"...","id":"wNNNNN"}, ...]
        let raw = std::fs::read_to_string(words).unwrap();
        let mut boxes = vec![];
        for w in raw.split('{').skip(1) {
            let spine: usize = w[w.find("\"spine\":").unwrap() + 8..].split([',', '}']).next().unwrap().trim().parse().unwrap();
            let i = w.find("\"id\":\"").unwrap() + 6;
            let id = &w[i..i + w[i..].find('"').unwrap()];
            let (page, rects) = book.find(spine, id).map(|(p, r)| (p as i64, r)).unwrap_or((-1, vec![]));
            let rs: Vec<String> = rects
                .iter()
                .map(|r| format!("{{\"Min\":{{\"X\":{},\"Y\":{}}},\"Max\":{{\"X\":{},\"Y\":{}}}}}", r[0], r[1], r[2], r[3]))
                .collect();
            boxes.push(format!("{{\"spine\":{spine},\"id\":\"{id}\",\"page\":{page},\"rects\":[{}]}}", rs.join(",")));
        }
        std::fs::write(format!("{out}/boxes.json"), format!("[{}]", boxes.join(","))).unwrap();
    }
    let t = format!(
        "{{\n  \"engine\": \"blitz\",\n  \"layout_ms\": {layout_ms:.1},\n  \"pages\": {},\n  \"draw_first_ms\": {first:.2},\n  \"draw_page_median_ms\": {:.2},\n  \"rss_end_mb\": {:.1}\n}}",
        book.pages.len(),
        draws[draws.len() / 2],
        max_rss_mb()
    );
    std::fs::write(format!("{out}/timings.json"), &t).unwrap();
    println!("{t}");
}

/// The fonts in a folder, each as its generic family by its file name.
fn fallback_fonts(dir: &str) -> Vec<(&'static str, Vec<u8>)> {
    let mut out = vec![];
    for e in std::fs::read_dir(dir).unwrap().flatten() {
        let name = e.file_name().to_string_lossy().into_owned();
        let generic = if name.contains("Mono") {
            "monospace"
        } else if name.contains("Serif") {
            "serif"
        } else if name.contains("Sans") {
            "sans-serif"
        } else {
            continue;
        };
        out.push((generic, std::fs::read(e.path()).unwrap()));
    }
    out
}

/// Peak resident memory, from /proc (Linux).
fn max_rss_mb() -> f64 {
    let s = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    s.lines()
        .find(|l| l.starts_with("VmHWM:"))
        .and_then(|l| l.split_whitespace().nth(1)?.parse::<f64>().ok())
        .map(|kb| kb / 1024.0)
        .unwrap_or(0.0)
}
