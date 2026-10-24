//! blitz-open: how long until the first page shows when only the document being opened is laid out (experiment 08).
//! For each spine document in turn, a fresh book: lay out that document, draw its first page; then, opened at the
//! first document with pages, the rest of the book. Memory is the peak so far (Linux).
//!
//! With DOC, only one book, opened at that document: its first page, then the rest, with the peak memory after
//! each; then every page drawn in order, as a reader going through the book (the slowest page includes laying a
//! document out again), and the memory held at the end. KEEP is how many documents stay laid out (2).
//!
//!     blitz-open BOOK_DIR [FONTS_DIR|-] [DOC [KEEP]]

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

fn ms(t: Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let dir = std::path::PathBuf::from(&args[1]);
    let fonts = args.get(2).filter(|d| *d != "-").map(|d| fallback_fonts(d)).unwrap_or_default();
    let files = spine(&dir);
    let mut opts = Options::default();
    if let Some(k) = args.get(4) {
        opts.keep = k.parse().unwrap();
    }
    let mut px = vec![0u8; (opts.width * opts.height) as usize];
    let src: Arc<Dir> = Arc::new(Dir(dir.clone()));

    if let Some(i) = args.get(3).map(|d| d.parse::<usize>().unwrap()) {
        let t = Instant::now();
        let mut book = Book::new(src, &files, opts, &fonts);
        let r = book.layout(i);
        book.draw(r.start, &mut px);
        let (first, mem) = (ms(t), max_rss_mb());
        let s = Instant::now();
        book.layout_rest();
        let rest = ms(s);
        let mem_rest = max_rss_mb();
        let mut turns = vec![];
        for n in 0..book.pages.len() {
            let s = Instant::now();
            book.draw(n, &mut px);
            turns.push(ms(s));
        }
        turns.sort_by(|a, b| a.partial_cmp(b).unwrap());
        println!(
            "at document {i}: first page {first:.1} ms, {mem:.0} MB; then the rest {rest:.0} ms, {mem_rest:.0} MB; \
             read through: median page {:.1} ms, slowest {:.1} ms, peak {:.0} MB, held at the end {:.0} MB",
            turns[turns.len() / 2],
            turns[turns.len() - 1],
            max_rss_mb(),
            rss_mb()
        );
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

/// Resident memory now, from /proc (Linux).
fn rss_mb() -> f64 {
    let s = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    s.lines()
        .find(|l| l.starts_with("VmRSS:"))
        .and_then(|l| l.split_whitespace().nth(1)?.parse::<f64>().ok())
        .map(|kb| kb / 1024.0)
        .unwrap_or(0.0)
}
