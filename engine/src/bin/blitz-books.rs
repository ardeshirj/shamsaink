//! blitz-books: memory over a reader's life, one process opening book after book (the plan's stage 2 keeps the
//! reader running). ROUNDS times, each BOOK in turn: opened as the reader would (the EPUB file, at document DOC, its
//! first page drawn), read through (the rest laid out, every page drawn), then closed (the `Book` dropped). Prints the
//! resident memory after each step and the peak so far (Linux), and after a close how much of it is allocated memory
//! and how much the program's own code and files mapped, so it shows whether closing a book gives its memory
//! back (flat), the allocator keeps it (a step that stays) or something grows with every book (a slope). Fallback
//! fonts are read once at the start, as a running reader would. TRIM set (glibc builds only): `malloc_trim` after each
//! close, so the memory left is what is still in use, not what the allocator keeps.
//!
//!     blitz-books ROUNDS BOOK.epub:FONTS_DIR|-:DOC ...

use blitz_engine::{Book, Epub, Options, Source};
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

/// A field of /proc/self/status in MB (Linux; 0 elsewhere).
fn status_mb(field: &str) -> f64 {
    let s = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    s.lines()
        .find(|l| l.starts_with(field))
        .and_then(|l| l.split_whitespace().nth(1)?.parse::<f64>().ok())
        .map(|kb| kb / 1024.0)
        .unwrap_or(0.0)
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let rounds: usize = args[1].parse().unwrap();
    let books: Vec<(String, Vec<(&str, Vec<u8>)>, usize)> = args[2..]
        .iter()
        .map(|a| {
            let p: Vec<&str> = a.split(':').collect();
            let fonts = if p[1] == "-" { vec![] } else { blitz_engine::fallback_fonts(p[1]) };
            (p[0].to_string(), fonts, p[2].parse().unwrap())
        })
        .collect();
    let opts = Options::default();
    let mut px = vec![0u8; (opts.width * opts.height) as usize];
    println!(
        "start, fonts read: {:.1} MB ({:.1} allocated, {:.1} the program's code and files)",
        status_mb("VmRSS:"),
        status_mb("RssAnon:"),
        status_mb("RssFile:")
    );
    for round in 1..=rounds {
        for (path, fonts, doc) in &books {
            let name = std::path::Path::new(path).file_stem().unwrap().to_string_lossy();
            let t = Instant::now();
            let src: Arc<dyn Source> = Arc::new(Epub::open(std::path::Path::new(path)).unwrap());
            let files = spine(src.as_ref());
            let mut book = Book::new(src, &files, opts, fonts);
            let r = book.layout(*doc);
            book.draw(r.start, &mut px);
            let opened = (t.elapsed().as_secs_f64() * 1e3, status_mb("VmRSS:"));
            book.layout_rest();
            for n in 0..book.pages.len() {
                book.draw(n, &mut px);
            }
            let read = status_mb("VmRSS:");
            drop(book);
            #[cfg(target_env = "gnu")]
            if std::env::var("TRIM").is_ok() {
                // glibc only: give every free page back, so what stays is what is still in use
                unsafe extern "C" {
                    fn malloc_trim(pad: usize) -> i32;
                }
                unsafe { malloc_trim(0) };
            }
            println!(
                "round {round} {name:13} opened {:4.0} ms, {:5.1} MB; read through {read:5.1} MB; closed {:5.1} MB \
                 ({:.1} allocated, {:.1} the program's code and files; peak {:.1})",
                opened.0,
                opened.1,
                status_mb("VmRSS:"),
                status_mb("RssAnon:"),
                status_mb("RssFile:"),
                status_mb("VmHWM:")
            );
        }
    }
}
