//! blitz-demo: how one page is drawn, step by step, for experiment 07's page (`07-blitz-drawing/demo.sh`). Writes into
//! OUT_DIR, all 8-bit grey PNGs: before.png (the whole page through vello_cpu, as before the glyph layer), rest.png
//! (what still goes to vello_cpu), text.png (the glyphs copied from masks), boxes.png (text.png with each copied
//! glyph's box), after.png (the page as drawn now), masks.png (the masks this page needed, one per glyph and
//! quarter-pixel position), quarters.png (one glyph's four quarter-pixel masks) and timings.json (this page, each way).
//!
//!     blitz-demo BOOK_DIR OUT_DIR PAGE [FONTS_DIR]

use blitz_engine::{Book, Dir, Layer, Options};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Instant;

fn spine(dir: &std::path::Path) -> Vec<String> {
    let attr = |tag: &str, name: &str| -> Option<String> {
        let i = tag.find(&format!("{name}=\""))? + name.len() + 2;
        Some(tag[i..i + tag[i..].find('"')?].to_string())
    };
    let container = std::fs::read_to_string(dir.join("META-INF/container.xml")).unwrap();
    let opf_path = attr(&container[container.find("<rootfile").unwrap()..], "full-path").unwrap();
    let opf = std::fs::read_to_string(dir.join(&opf_path)).unwrap();
    let base = opf_path.rsplit_once('/').map(|(b, _)| format!("{b}/")).unwrap_or_default();
    let mut hrefs = HashMap::new();
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

fn png(path: &str, w: usize, h: usize, px: &[u8]) {
    let mut enc = png::Encoder::new(std::fs::File::create(path).unwrap(), w as u32, h as u32);
    enc.set_color(png::ColorType::Grayscale);
    enc.write_header().unwrap().write_image_data(px).unwrap();
}

/// Masks side by side in rows `width` wide, black on white, `gap` pixels apart; returns the sheet's height.
fn sheet(masks: &[(u32, u8, usize, usize, &[u8])], width: usize, gap: usize) -> (usize, Vec<u8>) {
    let (mut x, mut y, mut row) = (gap, gap, 0);
    let mut at = vec![];
    for &(_, _, w, h, _) in masks {
        if x + w + gap > width {
            (x, y, row) = (gap, y + row + gap, 0);
        }
        at.push((x, y));
        x += w + gap;
        row = row.max(h);
    }
    let height = y + row + gap;
    let mut px = vec![255u8; width * height];
    for (&(_, _, w, h, alpha), &(x0, y0)) in masks.iter().zip(&at) {
        for yy in 0..h {
            for xx in 0..w {
                px[(y0 + yy) * width + x0 + xx] = 255 - alpha[yy * w + xx];
            }
        }
    }
    (height, px)
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let (dir, out) = (std::path::PathBuf::from(&args[1]), &args[2]);
    let n: usize = args[3].parse::<usize>().unwrap() - 1;
    std::fs::create_dir_all(out).unwrap();
    let opts = Options::default();
    let fonts = args.get(4).map(|d| fallback_fonts(d)).unwrap_or_default();
    let mut book = Book::open_with_fonts(Arc::new(Dir(dir.clone())), &spine(&dir), opts, &fonts);
    let (w, h) = (opts.width as usize, opts.height as usize);
    let mut px = vec![0u8; w * h];
    let mut boxes = vec![];

    // this page first, so the masks are the ones it needs
    book.draw_layer(n, &mut px, Layer::Text, &mut boxes);
    png(&format!("{out}/text.png"), w, h, &px);
    for b in &boxes {
        let [x0, y0, x1, y1] = b.map(|v| v.max(0) as usize);
        for x in x0..x1.min(w) {
            for y in [y0, y1.saturating_sub(1)] {
                if y < h && px[y * w + x] > 150 {
                    px[y * w + x] = 150;
                }
            }
        }
        for y in y0..y1.min(h) {
            for x in [x0, x1.saturating_sub(1)] {
                if x < w && px[y * w + x] > 150 {
                    px[y * w + x] = 150;
                }
            }
        }
    }
    png(&format!("{out}/boxes.png"), w, h, &px);
    book.draw_layer(n, &mut px, Layer::Rest, &mut vec![]);
    png(&format!("{out}/rest.png"), w, h, &px);
    book.draw_layer(n, &mut px, Layer::AllVello, &mut vec![]);
    png(&format!("{out}/before.png"), w, h, &px);
    book.draw(n, &mut px);
    png(&format!("{out}/after.png"), w, h, &px);

    let mut masks = book.glyph_masks();
    masks.sort_by_key(|m| (m.0, m.1));
    let (sh, sp) = sheet(&masks, 1072, 6);
    png(&format!("{out}/masks.png"), 1072, sh, &sp);
    // the glyph with the most ink among those drawn at all four positions
    let mut by_glyph: HashMap<u32, Vec<_>> = HashMap::new();
    for m in &masks {
        by_glyph.entry(m.0).or_default().push(*m);
    }
    let quarters = by_glyph
        .into_values()
        .filter(|v| v.len() == 4)
        .max_by_key(|v| v[0].4.iter().map(|&a| a as u32).sum::<u32>())
        .unwrap_or_default();
    let (qh, qp) = sheet(&quarters, 4 * 60, 6);
    png(&format!("{out}/quarters.png"), 4 * 60, qh, &qp);
    let (n_masks, n_quarters) = (masks.len(), quarters.len());
    drop(masks);

    // this page drawn each way, warm (masks cached), median of 9
    let mut time = |f: &mut dyn FnMut(&mut Book, &mut [u8])| {
        let mut t: Vec<f64> = (0..9)
            .map(|_| {
                let s = Instant::now();
                f(&mut book, &mut px);
                s.elapsed().as_secs_f64() * 1e3
            })
            .collect();
        t.sort_by(|a, b| a.partial_cmp(b).unwrap());
        t[4]
    };
    let before = time(&mut |b, p| b.draw_layer(n, p, Layer::AllVello, &mut vec![]));
    let after = time(&mut |b, p| b.draw(n, p));
    let json = format!(
        "{{\"page\": {}, \"before_ms\": {before:.2}, \"after_ms\": {after:.2}, \"glyphs\": {}, \"masks\": {}, \"quarter_masks\": {}}}\n",
        n + 1,
        boxes.len(),
        n_masks,
        n_quarters
    );
    std::fs::write(format!("{out}/timings.json"), &json).unwrap();
    print!("{json}");
}
