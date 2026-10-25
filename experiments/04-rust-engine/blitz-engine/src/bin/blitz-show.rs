//! blitz-show: our page on the Kindle's screen (experiment 09, step 2). Opens BOOK at document DOC, draws the page
//! holding the element ID at the screen's size, writes it into the framebuffer (/dev/fb0, the buffer shown) and has
//! FBInk refresh the whole screen (GC16, flashing); then marks that word and the next WORDS-1 ones in turn, DELAY_MS
//! apart, refreshing only the box around the word let go and the word marked (WAVEFORM, DU by default), and last lets
//! the last word go. MARK is how a word is marked: "invert" (its boxes inverted, the default) or "underline" (a black
//! bar under each box). Prints each step's time. Without a framebuffer (the Mac), writes PNGs instead: the page, the
//! page with its first word marked, and the page with every word from ID to the page's end marked. FBINK is FBInk's
//! command-line tool (KOReader's by default); the page is the screen's size, 1236×1648 (WIDTH, HEIGHT).
//!
//!     blitz-show BOOK_DIR FONTS_DIR|- DOC ID [WORDS [WAVEFORM [DELAY_MS [MARK]]]]

use blitz_engine::{Book, Dir, Options};
use std::collections::HashMap;
use std::os::unix::fs::FileExt;
use std::sync::Arc;
use std::time::{Duration, Instant};

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

fn ms(t: Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

/// The id after this one: its trailing number plus one, as wide ("w00041" → "w00042").
fn next_id(id: &str) -> String {
    let digits = id.len() - id.trim_end_matches(|c: char| c.is_ascii_digit()).len();
    let (stem, n) = id.split_at(id.len() - digits);
    format!("{stem}{:0digits$}", n.parse::<u64>().unwrap_or(0) + 1)
}

/// A rectangle around all these boxes, a few pixels wider, within the page: [left, top, right, bottom].
fn around(boxes: &[[i32; 4]], w: usize, h: usize) -> [usize; 4] {
    let pad = 4;
    let l = boxes.iter().map(|b| b[0]).min().unwrap() - pad;
    let t = boxes.iter().map(|b| b[1]).min().unwrap() - pad;
    let r = boxes.iter().map(|b| b[2]).max().unwrap() + pad;
    let b = boxes.iter().map(|b| b[3]).max().unwrap() + pad;
    [l.max(0) as usize, t.max(0) as usize, (r.max(0) as usize).min(w), (b.max(0) as usize).min(h)]
}

fn union(a: [usize; 4], b: [usize; 4]) -> [usize; 4] {
    [a[0].min(b[0]), a[1].min(b[1]), a[2].max(b[2]), a[3].max(b[3])]
}

/// The bar under a word's box: [left, top, right, bottom], just under the lowest ink of the box's line (the deepest
/// descender across the page's width, so every bar on a line is level; a box is the line's whole height).
fn bar(page: &[u8], w: usize, b: [i32; 4]) -> [i32; 4] {
    let (gap, thick) = (3, 4);
    let h = page.len() / w;
    let (t, bt) = (b[1].max(0) as usize, (b[3].max(0) as usize).min(h));
    let ink = (t..bt).rev().find(|&y| page[y * w..(y + 1) * w].iter().any(|&p| p < 128)).unwrap_or(bt) as i32;
    [b[0], ink + 1 + gap, b[2], ink + 1 + gap + thick]
}

/// The pixels a mark changes for these boxes: the boxes themselves when inverted, the bars under them when underlined.
fn marked_area(page: &[u8], w: usize, boxes: &[[i32; 4]], underline: bool) -> Vec<[i32; 4]> {
    if underline { boxes.iter().map(|b| bar(page, w, *b)).collect() } else { boxes.to_vec() }
}

/// The page with these words marked: each word's boxes inverted, or a black bar under each.
fn marked(page: &[u8], w: usize, words: &[Vec<[i32; 4]>], underline: bool) -> Vec<u8> {
    let mut out = page.to_vec();
    let h = page.len() / w;
    for b in words.iter().flat_map(|boxes| marked_area(page, w, boxes, underline)) {
        if underline {
            let (l, r) = (b[0].max(0) as usize, (b[2].max(0) as usize).min(w));
            for y in b[1].max(0) as usize..(b[3].max(0) as usize).min(h) {
                out[y * w + l..y * w + r].fill(0);
            }
            continue;
        }
        let [l, t, r, bt] = around(&[b], w, h);
        for y in t..bt {
            for p in &mut out[y * w + l..y * w + r] {
                *p = 255 - *p;
            }
        }
    }
    out
}

/// The framebuffer: the screen's memory, its row length and the first row of the buffer shown.
struct Fb {
    file: std::fs::File,
    stride: usize,
    top: usize,
}

impl Fb {
    fn open() -> Option<Fb> {
        let file = std::fs::OpenOptions::new().read(true).write(true).open("/dev/fb0").ok()?;
        let sys = |f: &str| std::fs::read_to_string(format!("/sys/class/graphics/fb0/{f}")).unwrap_or_default();
        let stride = sys("stride").trim().parse().ok()?;
        let top = sys("pan").trim().split(',').nth(1).and_then(|y| y.parse().ok()).unwrap_or(0);
        Some(Fb { file, stride, top })
    }

    /// The part `r` of a page (w pixels wide) written into the buffer shown.
    fn write(&self, page: &[u8], w: usize, r: [usize; 4]) {
        for y in r[1]..r[3] {
            let at = ((self.top + y) * self.stride + r[0]) as u64;
            self.file.write_all_at(&page[y * w + r[0]..y * w + r[2]], at).unwrap();
        }
    }
}

/// FBInk refreshes the rectangle r of the screen (nothing drawn), waiting until the screen is done; its time in ms.
fn refresh(fbink: &str, r: [usize; 4], waveform: &str, flash: bool) -> f64 {
    let t = Instant::now();
    let region = format!("top={},left={},width={},height={}", r[1], r[0], r[2] - r[0], r[3] - r[1]);
    let mut cmd = std::process::Command::new(fbink);
    cmd.args(["-q", "-w", "-W", waveform, "-s", &region]);
    if flash {
        cmd.arg("-f");
    }
    let ok = cmd.stdout(std::process::Stdio::null()).status().map(|s| s.success()).unwrap_or(false);
    assert!(ok, "{fbink} failed");
    ms(t)
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let dir = std::path::PathBuf::from(&args[1]);
    let fonts = args.get(2).filter(|d| *d != "-").map(|d| fallback_fonts(d)).unwrap_or_default();
    let doc: usize = args[3].parse().unwrap();
    let mut id = args[4].clone();
    let words: usize = args.get(5).map(|n| n.parse().unwrap()).unwrap_or(10);
    let waveform = args.get(6).map(String::as_str).unwrap_or("DU");
    let delay = Duration::from_millis(args.get(7).map(|n| n.parse().unwrap()).unwrap_or(500));
    let underline = match args.get(8).map(String::as_str).unwrap_or("invert") {
        "invert" => false,
        "underline" => true,
        m => panic!("MARK is invert or underline, not {m}"),
    };
    let fbink = std::env::var("FBINK").unwrap_or("/mnt/us/koreader/fbink".into());
    let env = |k: &str, d: u32| std::env::var(k).ok().and_then(|v| v.parse().ok()).unwrap_or(d);
    let opts = Options { width: env("WIDTH", 1236), height: env("HEIGHT", 1648), ..Options::default() };
    let (w, h) = (opts.width as usize, opts.height as usize);

    let t = Instant::now();
    let mut book = Book::new(Arc::new(Dir(dir.clone())), &spine(&dir), opts, &fonts);
    book.layout(doc);
    let open = ms(t);
    let t = Instant::now();
    let (n, mut boxes) = book.find(doc, &id).expect("no such id in that document");
    let find = ms(t);
    let mut page = vec![0u8; w * h];
    let t = Instant::now();
    book.draw(n, &mut page);
    let draw = ms(t);
    println!(
        "{}: document {doc} laid out in {open:.1} ms, {id} found on page {n} in {find:.1} ms, page drawn in {draw:.1} ms",
        dir.display()
    );

    let Some(fb) = Fb::open() else {
        let mut all = vec![boxes.clone()];
        let mut next = next_id(&id);
        while let Some((m, b)) = book.find(doc, &next) {
            if m != n {
                break;
            }
            all.push(b);
            next = next_id(&next);
        }
        png("show-page.png", w, h, &page);
        png("show-word.png", w, h, &marked(&page, w, &all[..1], underline));
        png("show-all.png", w, h, &marked(&page, w, &all, underline));
        println!("no framebuffer: show-page.png, show-word.png, show-all.png written ({} words), {id} at {boxes:?}", all.len());
        return;
    };
    let whole = [0, 0, w, h];
    let t = Instant::now();
    fb.write(&page, w, whole);
    let write = ms(t);
    let shown = refresh(&fbink, whole, "GC16", true);
    println!("page on the screen: written in {write:.1} ms, refreshed (GC16, flashing) in {shown:.0} ms");

    std::thread::sleep(delay);
    let mut last: Option<[usize; 4]> = None;
    for k in 0..words {
        let r = around(&marked_area(&page, w, &boxes, underline), w, h);
        let t = Instant::now();
        let lit = marked(&page, w, std::slice::from_ref(&boxes), underline);
        let area = last.map(|l| union(l, r)).unwrap_or(r);
        fb.write(&lit, w, area);
        let write = ms(t);
        let took = refresh(&fbink, area, waveform, false);
        println!(
            "word {k} ({id}, {} box{}): {}×{} px written in {write:.2} ms, refreshed ({waveform}) in {took:.0} ms",
            boxes.len(),
            if boxes.len() == 1 { "" } else { "es" },
            area[2] - area[0],
            area[3] - area[1]
        );
        last = Some(r);
        std::thread::sleep(delay);
        id = next_id(&id);
        match book.find(doc, &id) {
            Some((m, b)) if m == n && !b.is_empty() => boxes = b,
            _ => break,
        }
    }
    if let Some(l) = last {
        fb.write(&page, w, l);
        let took = refresh(&fbink, l, waveform, false);
        println!("last word let go: refreshed ({waveform}) in {took:.0} ms");
    }
}
