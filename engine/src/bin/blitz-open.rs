//! blitz-open: how long until the first page shows when only the document being opened is laid out (experiment 08).
//! For each spine document in turn, a fresh book: lay out that document, draw its first page; then, opened at the
//! first document with pages, the rest of the book. Memory is the peak so far (Linux).
//!
//! With DOC, only one book, opened at that document: its first page (split into steps: the process's start, the
//! fallback fonts read, `Book::new`, the document's steps, the draw; then the same steps for the next document, to
//! tell what is paid once from what is paid for every document), then the rest, with the peak memory after each;
//! then every page drawn in order, as a reader going through the book (the slowest page includes laying a
//! document out again), and the memory held at the end. KEEP is how many documents stay laid out (2). With CACHE,
//! the book's cache file: read before laying out if it is there (a book opened again), written after reading through.
//!
//! BOOK is an unzipped EPUB's folder or, ending in `.epub`, the EPUB file itself (read as a reader would: `Epub`).
//!
//!     blitz-open BOOK [FONTS_DIR|-] [DOC [KEEP [CACHE]]]

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

fn ms(t: Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

fn main() {
    let start = Instant::now();
    let age = process_age_ms();
    let args: Vec<String> = std::env::args().collect();
    let dir = std::path::PathBuf::from(&args[1]);
    let t = Instant::now();
    let fonts = args.get(2).filter(|d| *d != "-").map(|d| blitz_engine::fallback_fonts(d)).unwrap_or_default();
    let fonts_ms = ms(t);
    let src: Arc<dyn Source> = if args[1].ends_with(".epub") {
        Arc::new(Epub::open(&dir).unwrap())
    } else {
        Arc::new(Dir(dir.clone()))
    };
    let files = spine(src.as_ref());
    let mut opts = Options::default();
    if let Some(k) = args.get(4) {
        opts.keep = k.parse().unwrap();
    }
    let mut px = vec![0u8; (opts.width * opts.height) as usize];

    if let Some(i) = args.get(3).map(|d| d.parse::<usize>().unwrap()) {
        let t = Instant::now();
        let mut book = Book::new(src.clone(), &files, opts, &fonts);
        let new_ms = ms(t);
        let cache = args.get(5);
        let s = Instant::now();
        let restored = cache.and_then(|c| std::fs::read(c).ok()).map(|d| (d.len(), book.restore(&d)));
        let restore_ms = ms(s);
        let r = book.layout(i);
        let lt = book.last_load;
        let s = Instant::now();
        book.draw(r.start, &mut px);
        let draw = ms(s);
        let (masks, masks_ms) = book.glyph_work();
        let (first, mem) = (ms(t), max_rss_mb());
        let show = |lt: blitz_engine::LoadTimes| {
            format!(
                "read {:.1}, parse {:.1}, files {:.1}, resolve {:.1} + {:.1}, paginate {:.1}",
                lt.read, lt.parse, lt.files, lt.resolve, lt.resolve2, lt.paginate
            )
        };
        let once = format!(
            "  first page, ms: process start {age:.0} (to 10 ms), spine read {:.1}, fallback fonts read \
             {fonts_ms:.1}, Book::new {new_ms:.1}; document {i}: {}; draw {draw:.1} ({masks} glyph masks made, \
             {masks_ms:.1}){}",
            ms(start) - ms(t) - fonts_ms,
            show(lt),
            match restored {
                Some((n, true)) => format!("; from the cache ({} KB, read and restored in {restore_ms:.1})", n / 1024),
                Some((_, false)) => "; cache not used".into(),
                None => String::new(),
            }
        );
        // the next document with pages, in the same process
        let next = (i + 1..files.len()).find(|&j| {
            let r = book.layout(j);
            !r.is_empty()
        });
        let again = next.map(|j| {
            let lt = book.last_load;
            let r = book.layout(j);
            let s = Instant::now();
            book.draw(r.start, &mut px);
            let d = ms(s);
            let (m, m_ms) = book.glyph_work();
            format!("  then document {j}: {}; draw {d:.1} ({} more glyph masks, {:.1})", show(lt), m - masks, m_ms - masks_ms)
        });
        // a second book in the same process, opened at the same document: what of the first open is paid once a
        // process (statics, caches) and what again for every book
        let second = {
            let t = Instant::now();
            let mut b = Book::new(src.clone(), &files, opts, &fonts);
            let r = b.layout(i);
            let lt = b.last_load;
            let s = Instant::now();
            b.draw(r.start, &mut px);
            let (d, (m, m_ms)) = (ms(s), b.glyph_work());
            format!("  a second book at document {i}: {:.1} in all; {}; draw {d:.1} ({m} glyph masks, {m_ms:.1})", ms(t), show(lt))
        };
        let s = Instant::now();
        book.layout_rest();
        let rest = ms(s);
        let mem_rest = max_rss_mb();
        let mut turns = vec![];
        // every page's pixels hashed, to tell a book read from its cache drew the same pages
        let mut sum = std::collections::hash_map::DefaultHasher::new();
        for n in 0..book.pages.len() {
            let s = Instant::now();
            book.draw(n, &mut px);
            turns.push(ms(s));
            std::hash::Hash::hash(&px, &mut sum);
            if std::env::var_os("PAGE_HASHES").is_some() {
                println!("page {n} doc {} hash {:016x}", book.pages[n].doc, fnv1a(&px));
            }
        }
        let sum = std::hash::Hasher::finish(&sum);
        turns.sort_by(|a, b| a.partial_cmp(b).unwrap());
        let saved = cache.map(|c| {
            let s = Instant::now();
            let data = book.save().expect("every document paginated");
            std::fs::write(c, &data).unwrap();
            format!("  cache written: {} KB in {:.1} ms; pages {:016x}", data.len() / 1024, ms(s), sum)
        });
        println!(
            "at document {i}: first page {first:.1} ms, {mem:.0} MB; then the rest {rest:.0} ms, {mem_rest:.0} MB; \
             read through: median page {:.1} ms, slowest {:.1} ms, peak {:.0} MB, held at the end {:.0} MB",
            turns[turns.len() / 2],
            turns[turns.len() - 1],
            max_rss_mb(),
            rss_mb()
        );
        println!("{once}");
        if let Some(a) = again {
            println!("{a}");
        }
        println!("{second}");
        if let Some(s) = saved {
            println!("{s}");
        }
        return;
    }

    // the whole book first, as before (it also brings the files into the OS's cache)
    let t = Instant::now();
    let book = Book::open_with_fonts(src.clone(), &files, opts, &fonts);
    let whole = ms(t);
    let pages = book.pages.len();
    drop(book);

    println!("document                        pages  layout ms  draw ms  first page ms");
    let mut firsts = vec![];
    let mut start = None;
    for (i, path) in files.iter().enumerate() {
        let t = Instant::now();
        let mut book = Book::new(src.clone(), &files, opts, &fonts);
        let r = book.layout(i);
        let lay = ms(t);
        if r.is_empty() {
            continue;
        }
        let s = Instant::now();
        book.draw(r.start, &mut px);
        let draw = ms(s);
        let first = ms(t);
        println!("{:30} {:6} {lay:10.1} {draw:8.1} {first:14.1}", path.rsplit('/').next().unwrap(), r.len());
        firsts.push(first);
        start.get_or_insert(i);
    }
    firsts.sort_by(|a, b| a.partial_cmp(b).unwrap());
    let mem_docs = max_rss_mb();

    // opened at the first document with pages: its first page, then the rest
    let i = start.unwrap();
    let t = Instant::now();
    let mut book = Book::new(src.clone(), &files, opts, &fonts);
    let r = book.layout(i);
    book.draw(r.start, &mut px);
    let first = ms(t);
    let s = Instant::now();
    book.layout_rest();
    let rest = ms(s);
    assert_eq!(book.pages.len(), pages);
    println!(
        "whole book laid out: {whole:.0} ms, {pages} pages; first page opened at a document: median {:.1} ms, \
         slowest {:.1} ms; at document {i}: first page {first:.1} ms, then the rest {rest:.0} ms; peak memory {:.0} MB \
         ({mem_docs:.0} MB before the last open)",
        firsts[firsts.len() / 2],
        firsts[firsts.len() - 1],
        max_rss_mb()
    );
}

/// FNV-1a 64 of a page's pixels, for PAGE_HASHES (bench/cmd/open prints the same).
fn fnv1a(b: &[u8]) -> u64 {
    b.iter().fold(0xcbf29ce484222325, |h, &c| (h ^ c as u64).wrapping_mul(0x100000001b3))
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

/// Resident memory now, from /proc (Linux).
fn rss_mb() -> f64 {
    let s = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    s.lines()
        .find(|l| l.starts_with("VmRSS:"))
        .and_then(|l| l.split_whitespace().nth(1)?.parse::<f64>().ok())
        .map(|kb| kb / 1024.0)
        .unwrap_or(0.0)
}

/// How long ago the process started, from /proc (Linux; in clock ticks, 10 ms).
fn process_age_ms() -> f64 {
    let stat = std::fs::read_to_string("/proc/self/stat").unwrap_or_default();
    let start: f64 = stat.rsplit(')').next().and_then(|r| r.split_whitespace().nth(19)?.parse().ok()).unwrap_or(0.0);
    let up: f64 = std::fs::read_to_string("/proc/uptime").ok().and_then(|u| u.split_whitespace().next()?.parse().ok()).unwrap_or(0.0);
    (up * 1e3 - start * 10.0).max(0.0)
}
