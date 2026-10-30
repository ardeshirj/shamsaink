//! blitz-follow: a narration followed on the Kindle's or the Kobo's screen, silently (experiments 09 and 13: the page
//! sized as the screen says, unless WIDTH and HEIGHT; FBInk from that device's KOReader). Reads a chapter's SMIL (one
//! `par` a word: the word's id, its audio file and clip) and DURATIONS (each audio file's length in seconds, a line
//! "NAME.mp3 SECONDS"), and puts every word on one clock: the files' lengths before its own, plus its clip's start.
//! Opens BOOK at the chapter's document, finds every word's page and boxes, shows the first word's page (GC16,
//! flashing), prints "clock started", then underlines each word when its time comes (a bar under it, refreshed with
//! WAVEFORM, A2 by default), for SECONDS seconds from word FROM (its index in the SMIL, 0 by default). A refresh still
//! on the screen when a word's time comes is not interrupted: the word is skipped, and the latest word due is shown
//! once the screen is free. A word on the next page turns the page (drawn, GC16, flashing). Logs each word shown (its
//! time on the clock, how late its refresh was sent, words skipped before it) and a summary. Without a framebuffer
//! (the Mac) it stops after placing the words. FBINK as in blitz-show; FONT_SIZE is the text's size in px (50, about
//! 12 pt on the Kindle's 300 ppi screen); LEAD_MS (0) starts the clock that long after "clock started" is printed,
//! for a player elsewhere that takes time to start (`09-kindle/follow-audio.sh`).
//!
//!     blitz-follow BOOK_DIR FONTS_DIR|- SMIL DURATIONS [SECONDS [FROM [WAVEFORM]]]

use blitz_engine::{Book, Dir, Options};
use std::collections::HashMap;
use std::os::unix::fs::FileExt;
use std::process::{Child, Command, Stdio};
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

fn ms(t: Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

/// A path relative to a file's folder, made relative to the book's root ("OEBPS/smil" + "../text/a.xhtml").
fn resolve(base: &str, rel: &str) -> String {
    let mut parts: Vec<&str> = base.split('/').filter(|p| !p.is_empty()).collect();
    for p in rel.split('/') {
        match p {
            ".." => {
                parts.pop();
            }
            "." | "" => {}
            p => parts.push(p),
        }
    }
    parts.join("/")
}

/// One narrated word: its document's path (from the book's root), its id, its time on the clock (seconds).
struct Word {
    doc: String,
    id: String,
    at: f64,
}

/// The SMIL's words in order, each timed on one clock through the audio files' lengths.
fn words(book: &std::path::Path, smil: &str, durations: &str) -> Vec<Word> {
    let attr = |tag: &str, name: &str| -> Option<String> {
        let i = tag.find(&format!("{name}=\""))? + name.len() + 2;
        Some(tag[i..i + tag[i..].find('"')?].to_string())
    };
    let length: HashMap<String, f64> = std::fs::read_to_string(durations)
        .unwrap()
        .lines()
        .filter_map(|l| l.split_once(' '))
        .map(|(f, s)| (f.to_string(), s.trim().parse().unwrap()))
        .collect();
    let rel = std::path::Path::new(smil).strip_prefix(book).unwrap().to_string_lossy().into_owned();
    let base = rel.rsplit_once('/').map(|(b, _)| b).unwrap_or("");
    let text = std::fs::read_to_string(smil).unwrap();
    let (mut out, mut offset, mut file) = (vec![], 0.0, String::new());
    for par in text.split("<par").skip(1) {
        let t = &par[par.find("<text").unwrap()..];
        let a = &par[par.find("<audio").unwrap()..];
        let (src, audio) = (attr(t, "src").unwrap(), attr(a, "src").unwrap());
        let name = audio.rsplit('/').next().unwrap().to_string();
        if name != file {
            if !file.is_empty() {
                offset += length[&file];
            }
            file = name;
        }
        let begin: f64 = attr(a, "clipBegin").unwrap().trim_end_matches('s').parse().unwrap();
        let (doc, id) = src.split_once('#').unwrap();
        out.push(Word { doc: resolve(base, doc), id: id.to_string(), at: offset + begin });
    }
    out
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

/// The bar under a word's box, just under the lowest ink of the box's line (as blitz-show's).
fn bar(page: &[u8], w: usize, b: [i32; 4]) -> [i32; 4] {
    let (gap, thick) = (3, 4);
    let h = page.len() / w;
    let (t, bt) = (b[1].max(0) as usize, (b[3].max(0) as usize).min(h));
    let ink = (t..bt).rev().find(|&y| page[y * w..(y + 1) * w].iter().any(|&p| p < 128)).unwrap_or(bt) as i32;
    [b[0], ink + 1 + gap, b[2], ink + 1 + gap + thick]
}

/// The page with a bar under each of these boxes.
fn underlined(page: &[u8], w: usize, bars: &[[i32; 4]]) -> Vec<u8> {
    let mut out = page.to_vec();
    let h = page.len() / w;
    for b in bars {
        let (l, r) = (b[0].max(0) as usize, (b[2].max(0) as usize).min(w));
        for y in b[1].max(0) as usize..(b[3].max(0) as usize).min(h) {
            out[y * w + l..y * w + r].fill(0);
        }
    }
    out
}

/// The framebuffer: the screen's memory, its visible size, its row length, the first row of the buffer shown and its
/// bytes a pixel (the Kindle: 1236×1648, 1; the Kobo Libra Colour: 1264×1680, 4, red, green, blue and transparency:
/// experiment 13).
struct Fb {
    file: std::fs::File,
    size: (u32, u32),
    stride: usize,
    top: usize,
    bytes: usize,
}

impl Fb {
    fn open() -> Option<Fb> {
        use std::os::fd::AsRawFd;
        let file = std::fs::OpenOptions::new().read(true).write(true).open("/dev/fb0").ok()?;
        let mut var = [0u32; 40]; // fb_var_screeninfo: xres, yres, xres_virtual, yres_virtual, xoffset, yoffset, bits...
        if unsafe { libc::ioctl(file.as_raw_fd(), 0x4600 as _, var.as_mut_ptr()) } != 0 {
            return None;
        }
        let stride = std::fs::read_to_string("/sys/class/graphics/fb0/stride").ok()?.trim().parse().ok()?;
        let (size, top, bytes) = ((var[0], var[1]), var[5] as usize, var[6] as usize / 8);
        Some(Fb { file, size, stride, top, bytes })
    }

    /// The part `r` of a page (w pixels wide) written into the buffer shown, each grey widened to the screen's pixel.
    fn write(&self, page: &[u8], w: usize, r: [usize; 4]) {
        let mut row = Vec::with_capacity((r[2] - r[0]) * self.bytes);
        for y in r[1]..r[3] {
            let grey = &page[y * w + r[0]..y * w + r[2]];
            let at = ((self.top + y) * self.stride + r[0] * self.bytes) as u64;
            if self.bytes == 1 {
                self.file.write_all_at(grey, at).unwrap();
                continue;
            }
            row.clear();
            for &g in grey {
                row.extend_from_slice(&[g, g, g, 255][..self.bytes.min(4)]);
            }
            self.file.write_all_at(&row, at).unwrap();
        }
    }
}

/// FBInk refreshing the rectangle r (nothing drawn), until the screen is done: started, not waited for.
fn refresh(fbink: &str, r: [usize; 4], waveform: &str, flash: bool) -> Child {
    let region = format!("top={},left={},width={},height={}", r[1], r[0], r[2] - r[0], r[3] - r[1]);
    let mut cmd = Command::new(fbink);
    cmd.args(["-q", "-w", "-W", waveform, "-s", &region]);
    if flash {
        cmd.arg("-f");
    }
    cmd.stdout(Stdio::null()).spawn().expect("fbink")
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let dir = std::path::PathBuf::from(&args[1]);
    let fonts = args.get(2).filter(|d| *d != "-").map(|d| blitz_engine::fallback_fonts(d)).unwrap_or_default();
    let (smil, durations) = (&args[3], &args[4]);
    let seconds: f64 = args.get(5).map(|s| s.parse().unwrap()).unwrap_or(180.0);
    let from: usize = args.get(6).map(|s| s.parse().unwrap()).unwrap_or(0);
    let waveform = args.get(7).map(String::as_str).unwrap_or("A2");
    let fbink = std::env::var("FBINK").unwrap_or_else(|_| {
        let on = ["/mnt/us/koreader/fbink", "/mnt/onboard/.adds/koreader/fbink"]; // the Kindle's KOReader, the Kobo's
        on.iter().find(|p| std::path::Path::new(p).exists()).unwrap_or(&on[0]).to_string()
    });
    let font_size = std::env::var("FONT_SIZE").ok().and_then(|v| v.parse().ok()).unwrap_or(50.0);
    let fb = Fb::open();
    let (fw, fh) = fb.as_ref().map(|f| f.size).unwrap_or((1236, 1648)); // the screen's, else the Kindle's
    let size = |v: &str, or: u32| std::env::var(v).ok().and_then(|v| v.parse().ok()).unwrap_or(or);
    let opts = Options { width: size("WIDTH", fw), height: size("HEIGHT", fh), font_size, ..Options::default() };
    let (w, h) = (opts.width as usize, opts.height as usize);

    let t = Instant::now();
    let all = words(&dir, smil, durations);
    let files = spine(&dir);
    let doc = files.iter().position(|f| *f == all[from].doc).expect("the SMIL's document is not in the spine");
    let start = all[from].at;
    let run: Vec<&Word> = all[from..].iter().take_while(|x| x.at < start + seconds && x.doc == all[from].doc).collect();
    let mut book = Book::new(Arc::new(Dir(dir.clone())), &files, opts, &fonts);
    book.layout(doc);
    let placed: Vec<Option<(usize, Vec<[i32; 4]>)>> = run.iter().map(|x| book.find(doc, &x.id)).collect();
    println!(
        "{} words timed ({} in the SMIL), document {doc} laid out and {} words found in {:.0} ms ({} not found)",
        run.len(),
        all.len(),
        run.len(),
        ms(t),
        placed.iter().filter(|p| p.is_none()).count()
    );

    let Some(fb) = fb else {
        println!("no framebuffer: nothing shown");
        return;
    };
    let whole = [0, 0, w, h];
    let mut page = vec![0u8; w * h];
    let first = placed.iter().flatten().next().expect("no word found").0;
    let mut shown_page = first;
    book.draw(first, &mut page);
    fb.write(&page, w, whole);
    refresh(&fbink, whole, "GC16", true).wait().unwrap();

    let lead: u64 = std::env::var("LEAD_MS").ok().and_then(|v| v.parse().ok()).unwrap_or(0);
    let clock = Instant::now() + Duration::from_millis(lead);
    println!("clock started at {start:.3} s, running in {lead} ms");
    let (mut busy, mut last_bar): (Option<Child>, Option<[usize; 4]>) = (None, None);
    let (mut next, mut shown_k): (usize, Option<usize>) = (0, None);
    let (mut shown, mut skipped, mut turns, mut late) = (0, 0, 0, vec![]);
    loop {
        let now = Instant::now().checked_duration_since(clock).map(|d| d.as_secs_f64()).unwrap_or(-1.0);
        while next < run.len() && run[next].at - start <= now {
            next += 1;
        }
        if next == run.len() && now > seconds {
            break;
        }
        let free = busy.as_mut().map(|c| c.try_wait().unwrap().is_some()).unwrap_or(true);
        let due = next.checked_sub(1);
        if !free || due.is_none() || due == shown_k {
            std::thread::sleep(Duration::from_millis(2));
            continue;
        }
        let k = due.unwrap();
        let before = shown_k.map(|s| s + 1).unwrap_or(0);
        skipped += k - before;
        let Some((n, boxes)) = &placed[k] else {
            shown_k = Some(k);
            continue;
        };
        if *n != shown_page {
            let t = Instant::now();
            book.draw(*n, &mut page);
            let drawn = ms(t);
            fb.write(&page, w, whole);
            refresh(&fbink, whole, "GC16", true).wait().unwrap();
            println!("page {n}: drawn in {drawn:.0} ms, turned in {:.0} ms", ms(t));
            (shown_page, last_bar, turns) = (*n, None, turns + 1);
        }
        let bars: Vec<[i32; 4]> = boxes.iter().map(|b| bar(&page, w, *b)).collect();
        let r = around(&bars, w, h);
        let area = last_bar.map(|l| union(l, r)).unwrap_or(r);
        fb.write(&underlined(&page, w, &bars), w, area);
        let sent = (clock.elapsed().as_secs_f64() - (run[k].at - start)) * 1e3;
        busy = Some(refresh(&fbink, area, waveform, false));
        println!("{:8.3} s  {}  sent {sent:.0} ms late, {} skipped before", run[k].at - start, run[k].id, k - before);
        late.push(sent);
        (shown_k, last_bar, shown) = (Some(k), Some(r), shown + 1);
    }
    if let Some(mut c) = busy {
        c.wait().unwrap();
    }
    if let Some(l) = last_bar {
        fb.write(&page, w, l);
        refresh(&fbink, l, waveform, false).wait().unwrap();
    }
    late.sort_by(|a, b| a.total_cmp(b));
    let q = |p: f64| late.get(((late.len().max(1) - 1) as f64 * p) as usize).copied().unwrap_or(0.0);
    println!(
        "{seconds:.0} s followed: {shown} words shown, {skipped} skipped, {turns} page turns; refresh sent late: median \
         {:.0} ms, 90% {:.0} ms, max {:.0} ms",
        q(0.5),
        q(0.9),
        q(1.0)
    );
}
