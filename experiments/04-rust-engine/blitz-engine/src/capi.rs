//! The C interface, for the Go reader (cgo; experiments/engine/blitz). Go reads the book (readium/go-toolkit) and
//! answers file requests through a callback; this side keeps the laid-out book behind an opaque handle; Go owns the
//! page buffers. See blitz_engine.h.

use crate::{Book, Options, Source};
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
    let paths: Vec<String> = (0..n)
        .map(|i| unsafe {
            let p = *spine.add(i);
            let l = *spine_len.add(i);
            String::from_utf8_lossy(std::slice::from_raw_parts(p, l)).into_owned()
        })
        .collect();
    let opts = Options { width, height, margin, font_size, ..Options::default() };
    Box::into_raw(Box::new(Book::open(Arc::new(Callback { read, ctx }), &paths, opts)))
}

/// # Safety
/// `b` comes from be_open and is not closed.
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
