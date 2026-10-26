//! The C interface, for the Go reader (cgo; experiments/engine/blitz). Go reads the book (readium/go-toolkit) and
//! answers file requests through a callback; this side keeps the laid-out book behind an opaque handle; Go owns the
//! page buffers. See blitz_engine.h.

use crate::{Book, Options, Source, fallback_fonts};
use std::ffi::c_void;
use std::sync::Arc;

/// Asks the reader for a file: it calls `be_buf_put(dst, ...)` with the bytes, or nothing if there is no such file.
pub type ReadFn = unsafe extern "C" fn(ctx: usize, path: *const u8, path_len: usize, dst: *mut c_void);

struct Callback {
    read: ReadFn,
    ctx: usize,
}

// The callback is only called on the thread that opened or draws the book.
unsafe impl Send for Callback {}
unsafe impl Sync for Callback {}

impl Source for Callback {
    fn read(&self, path: &str) -> Option<Vec<u8>> {
        let mut buf: Option<Vec<u8>> = None;
        unsafe { (self.read)(self.ctx, path.as_ptr(), path.len(), &mut buf as *mut _ as *mut c_void) };
        buf
    }
}

/// Copies a file's bytes into the buffer a ReadFn was given.
///
/// # Safety
/// `dst` is the pointer the ReadFn received; `data` points to `len` readable bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_buf_put(dst: *mut c_void, data: *const u8, len: usize) {
    let buf = unsafe { &mut *(dst as *mut Option<Vec<u8>>) };
    *buf = Some(if len == 0 { vec![] } else { unsafe { std::slice::from_raw_parts(data, len) }.to_vec() });
}

/// Fallback fonts read from a folder (see be_fonts).
pub struct Fonts(Vec<(&'static str, Vec<u8>)>);

unsafe fn spine_paths(spine: *const *const u8, spine_len: *const usize, n: usize) -> Vec<String> {
    (0..n)
        .map(|i| unsafe {
            let p = *spine.add(i);
            let l = *spine_len.add(i);
            String::from_utf8_lossy(std::slice::from_raw_parts(p, l)).into_owned()
        })
        .collect()
}

/// Reads the fonts in a folder (`dir`, `dir_len` bytes of UTF-8), each as the generic family its file name says
/// (Mono, Serif, Sans), once for every book of a long-running reader.
///
/// # Safety
/// `dir` has `dir_len` readable bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_fonts(dir: *const u8, dir_len: usize) -> *mut Fonts {
    let dir = String::from_utf8_lossy(unsafe { std::slice::from_raw_parts(dir, dir_len) }).into_owned();
    Box::into_raw(Box::new(Fonts(fallback_fonts(&dir))))
}

/// # Safety
/// `f` comes from be_fonts and is not used afterwards (books made with it keep their own copy).
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_fonts_free(f: *mut Fonts) {
    if !f.is_null() {
        drop(unsafe { Box::from_raw(f) });
    }
}

/// Lays out the book. `spine` holds `n` paths (from the EPUB's root), each `spine_len[i]` bytes of UTF-8.
///
/// # Safety
/// The pointers must be valid for the lengths given; `read` must stay callable until be_close.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_open(
    read: ReadFn,
    ctx: usize,
    spine: *const *const u8,
    spine_len: *const usize,
    n: usize,
    width: u32,
    height: u32,
    margin: u32,
    font_size: f32,
) -> *mut Book {
    let paths = unsafe { spine_paths(spine, spine_len, n) };
    let opts = Options { width, height, margin, font_size, ..Options::default() };
    Box::into_raw(Box::new(Book::open(Arc::new(Callback { read, ctx }), &paths, opts)))
}

/// A book with nothing laid out (see be_layout), `fonts` for text the book's fonts lack (null: none). Arguments as
/// be_open.
///
/// # Safety
/// As be_open; `fonts` is null or from be_fonts.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_new(
    read: ReadFn,
    ctx: usize,
    spine: *const *const u8,
    spine_len: *const usize,
    n: usize,
    fonts: *const Fonts,
    width: u32,
    height: u32,
    margin: u32,
    font_size: f32,
) -> *mut Book {
    let paths = unsafe { spine_paths(spine, spine_len, n) };
    let opts = Options { width, height, margin, font_size, ..Options::default() };
    let fonts: &[(&str, Vec<u8>)] = if fonts.is_null() { &[] } else { &unsafe { &*fonts }.0 };
    Box::into_raw(Box::new(Book::new(Arc::new(Callback { read, ctx }), &paths, opts, fonts)))
}

/// Lays out spine document `doc` for the reader (paginating it the first time); its pages are [first, last). Page
/// numbers after it shift the first time: final once every document is paginated.
///
/// # Safety
/// `b` comes from be_new; `first` and `last` are writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_layout(b: *mut Book, doc: usize, first: *mut usize, last: *mut usize) {
    let r = unsafe { &mut *b }.layout(doc);
    unsafe {
        *first = r.start;
        *last = r.end;
    }
}

/// Paginates one more spine document not paginated yet (the next in spine order), letting it go afterwards unless
/// the reader is using it: 1 if it did one, 0 if every document was already paginated.
///
/// # Safety
/// `b` comes from be_new.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_layout_next(b: *mut Book) -> i32 {
    unsafe { &mut *b }.layout_next() as i32
}

/// The book's cache (`len` bytes; free with be_bytes_free), or null if not every document is paginated yet. The
/// reader keeps it by the book's file.
///
/// # Safety
/// `b` comes from be_new; `len` is writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_save(b: *mut Book, len: *mut usize) -> *mut u8 {
    match unsafe { &*b }.save() {
        Some(v) => {
            let s: Box<[u8]> = v.into_boxed_slice();
            unsafe { *len = s.len() };
            Box::into_raw(s) as *mut u8
        }
        None => {
            unsafe { *len = 0 };
            std::ptr::null_mut()
        }
    }
}

/// # Safety
/// `ptr` and `len` are from be_save, not freed yet.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_bytes_free(ptr: *mut u8, len: usize) {
    if !ptr.is_null() {
        drop(unsafe { Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, len)) });
    }
}

/// Takes a cache from be_save on a book just made by be_new: 1 if restored, 0 if refused (other settings, spine or
/// version, or damaged; the book is unchanged).
///
/// # Safety
/// `b` comes from be_new; `data` has `len` readable bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_restore(b: *mut Book, data: *const u8, len: usize) -> i32 {
    let data = if len == 0 { &[][..] } else { unsafe { std::slice::from_raw_parts(data, len) } };
    unsafe { &mut *b }.restore(data) as i32
}

/// # Safety
/// `b` comes from be_open or be_new and is not closed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_pages(b: *mut Book) -> usize {
    unsafe { &*b }.pages.len()
}

/// The spine document a page belongs to, and its lines of text.
///
/// # Safety
/// `b` comes from be_open; `doc` and `lines` are writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_page_info(b: *mut Book, page: usize, doc: *mut usize, lines: *mut usize) {
    let p = unsafe { &*b }.pages[page];
    unsafe {
        *doc = p.doc;
        *lines = p.lines;
    }
}

/// Draws a page into `out`: width × height bytes of 8-bit grey.
///
/// # Safety
/// `b` comes from be_open; `out` has width × height writable bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_draw(b: *mut Book, page: usize, out: *mut u8) {
    let b = unsafe { &mut *b };
    let len = (b.opts.width * b.opts.height) as usize;
    b.draw(page, unsafe { std::slice::from_raw_parts_mut(out, len) });
}

/// The page of the element with this id in spine document `doc` (-1 if not found); its boxes on that page, as
/// left, top, right, bottom quadruples, go into `rects` (room for `max` boxes) and their number into `n`.
///
/// # Safety
/// `b` comes from be_open; `id` has `id_len` bytes; `rects` has room for 4 × `max` ints; `n` is writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_find(
    b: *mut Book,
    doc: usize,
    id: *const u8,
    id_len: usize,
    rects: *mut i32,
    max: usize,
    n: *mut usize,
) -> i64 {
    let b = unsafe { &mut *b };
    let id = String::from_utf8_lossy(unsafe { std::slice::from_raw_parts(id, id_len) }).into_owned();
    unsafe { *n = 0 };
    let Some((page, boxes)) = b.find(doc, &id) else { return -1 };
    let k = boxes.len().min(max);
    for (i, r) in boxes.iter().take(k).enumerate() {
        for j in 0..4 {
            unsafe { *rects.add(4 * i + j) = r[j] };
        }
    }
    unsafe { *n = k };
    page as i64
}

/// # Safety
/// `b` comes from be_open and is not used afterwards.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn be_close(b: *mut Book) {
    drop(unsafe { Box::from_raw(b) });
}
