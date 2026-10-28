//! blitz-words: the words on a page (`Book::words`), for checking their boxes by eye and the time they take. Draws
//! page PAGE (from 1) of BOOK (an unzipped EPUB's folder, or a `.epub`) with every other word greyed, as a highlight
//! would be, into OUT.png; prints the page's word count, the time to find them, and the text of its first and last
//! word (`Book::text`).
//!
//!     blitz-words BOOK OUT.png PAGE [FONTS_DIR] [FONT_SIZE]

use blitz_engine::{Book, Dir, Epub, Options, Source};
use std::sync::Arc;
use std::time::Instant;

/// The spine's documents, from the OPF the container names (paths from the EPUB's root).
fn spine(src: &dyn Source) -> Vec<String> {
    let attr = |tag: &str, name: &str| -> Option<String> {
        let i = tag.find(&format!("{name}=\""))? + name.len() + 2;
        Some(tag[i..i + tag[i..].find('"')?].to_string())
    };
    let text = |path: &str| String::from_utf8(src.read(path).unwrap_or_else(|| panic!("no {path}"))).unwrap();
    let container = text("META-INF/container.xml");
    let opf_path = attr(&container[container.find("<rootfile").unwrap()..], "full-path").unwrap();
    let opf = text(&opf_path);
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
    let path = std::path::PathBuf::from(&args[1]);
    let n: usize = args[3].parse::<usize>().unwrap() - 1;
    let fonts = args.get(4).filter(|d| *d != "-").map(|d| blitz_engine::fallback_fonts(d)).unwrap_or_default();
    let src: Arc<dyn Source> =
        if args[1].ends_with(".epub") { Arc::new(Epub::open(&path).unwrap()) } else { Arc::new(Dir(path.clone())) };
    let mut opts = Options { font_size: 50.0, ..Options::default() };
    if let Some(s) = args.get(5) {
        opts.font_size = s.parse().unwrap();
    }
    let mut book = Book::open_with_fonts(src.clone(), &spine(src.as_ref()), opts, &fonts);
    let (w, h) = (opts.width as usize, opts.height as usize);
    let mut px = vec![0u8; w * h];
    book.draw(n, &mut px);
    let t = Instant::now();
    let words = book.words(n);
    let took = t.elapsed().as_secs_f64() * 1e3;
    for wd in words.iter().step_by(2) {
        let [x0, y0, x1, y1] = wd.rect.map(|v| v.max(0) as usize);
        for y in y0..y1.min(h) {
            for x in x0..x1.min(w) {
                px[y * w + x] = (px[y * w + x] as u32 * 13 / 16) as u8;
            }
        }
    }
    let mut enc = png::Encoder::new(std::fs::File::create(&args[2]).unwrap(), w as u32, h as u32);
    enc.set_color(png::ColorType::Grayscale);
    enc.write_header().unwrap().write_image_data(&px).unwrap();
    let doc = book.pages[n].doc;
    println!("page {} (document {doc}): {} words in {took:.2} ms", n + 1, words.len());
    if let (Some(a), Some(b)) = (words.iter().min_by_key(|w| (w.block, w.start)), words.iter().max_by_key(|w| (w.block, w.end))) {
        println!("first word {:?}, last {:?}", book.text(doc, (a.block, a.start), (a.block, a.end)), book.text(doc, (b.block, b.start), (b.block, b.end)));
        println!("the page's text from first to last word: {} bytes", book.text(doc, (a.block, a.start), (b.block, b.end)).len());
    }
}
