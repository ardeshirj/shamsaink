//go:build blitz

// Package blitz is the Rust engine in engine/ (Blitz: Stylo, Taffy, Parley; our pagination) for Go, through cgo and
// its C interface (blitz_engine.h). Go reads the book: the caller gives the spine's paths and a function that reads
// a file, and the engine asks for files through a callback. Needs the static library built first (engine/: cargo
// build --release, and for ARMv7 --target armv7-unknown-linux-musleabihf) and -tags blitz, in the eink-rust image;
// bench/build-armv7.sh makes the 32-bit ARM binaries.
package blitz

/*
#cgo CFLAGS: -I${SRCDIR}/..
#cgo arm64 LDFLAGS: ${SRCDIR}/../target/release/libblitz_engine.a -lgcc_s -lutil -lrt -lpthread -lm -ldl
#cgo arm LDFLAGS: ${SRCDIR}/../target/armv7-unknown-linux-musleabihf/release/libblitz_engine.a -lunwind -lm
#include <stdlib.h>
#include "blitz_engine.h"
void goRead(uintptr_t ctx, uint8_t *path, size_t path_len, void *dst);
*/
import "C"

import (
	"image"
	"sync"
	"unsafe"
)

// Options are the page and text settings: size in pixels, margin on every side, text size in pixels, and line
// spacing over the book's own (unitless, as CSS's line-height; 0: the book's; New only).
type Options struct {
	Width, Height, Margin int
	FontSize, LineHeight  float32
}

// PageInfo says which spine document a page is from and how many lines of text it has.
type PageInfo struct{ Spine, Lines int }

// ReadFunc reads a file of the book by its path in the EPUB.
type ReadFunc func(path string) ([]byte, error)

// Books being read, by the number C holds (C may not keep Go pointers).
var (
	booksMu sync.Mutex
	books   = map[uintptr]ReadFunc{}
	nextCtx uintptr
)

//export goRead
func goRead(ctx C.uintptr_t, path *C.uint8_t, pathLen C.size_t, dst unsafe.Pointer) {
	booksMu.Lock()
	read := books[uintptr(ctx)]
	booksMu.Unlock()
	data, err := read(C.GoStringN((*C.char)(unsafe.Pointer(path)), C.int(pathLen)))
	if err != nil {
		return
	}
	p := C.CBytes(data)
	defer C.free(p)
	C.be_buf_put(dst, (*C.uint8_t)(p), C.size_t(len(data)))
}

type doc struct {
	b    *C.Book
	ctx  uintptr
	w, h int
}

// start makes the book reachable from C and the spine's paths C arrays; free releases the arrays.
func start(spine []string, read ReadFunc) (ctx uintptr, ptrs, lens unsafe.Pointer, n int, free func()) {
	booksMu.Lock()
	nextCtx++
	ctx = nextCtx
	books[ctx] = read
	booksMu.Unlock()
	n = len(spine)
	ptrs = C.malloc(C.size_t(n+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
	lens = C.malloc(C.size_t(n+1) * C.size_t(unsafe.Sizeof(C.size_t(0))))
	ps := unsafe.Slice((**C.uint8_t)(ptrs), n)
	ls := unsafe.Slice((*C.size_t)(lens), n)
	for i, s := range spine {
		ps[i] = (*C.uint8_t)(C.CBytes([]byte(s)))
		ls[i] = C.size_t(len(s))
	}
	free = func() {
		for _, p := range ps {
			C.free(unsafe.Pointer(p))
		}
		C.free(ptrs)
		C.free(lens)
	}
	return
}

// Open lays out the whole book: spine is its documents' paths in reading order.
func Open(spine []string, read ReadFunc, o Options) *Book {
	ctx, ptrs, lens, n, free := start(spine, read)
	defer free()
	h := C.be_open(C.be_read_fn(C.goRead), C.uintptr_t(ctx), (**C.uint8_t)(ptrs), (*C.size_t)(lens),
		C.size_t(n), C.uint32_t(o.Width), C.uint32_t(o.Height), C.uint32_t(o.Margin), C.float(o.FontSize))
	return &Book{doc{b: h, ctx: ctx, w: o.Width, h: o.Height}}
}

// Fonts are fallback fonts read from a folder once, for every book of a long-running reader.
type Fonts struct{ f *C.Fonts }

// LoadFonts reads the fonts in dir, each as the generic family its file name says (Mono, Serif, Sans).
func LoadFonts(dir string) *Fonts {
	c := C.CBytes([]byte(dir))
	defer C.free(c)
	return &Fonts{C.be_fonts((*C.uint8_t)(c), C.size_t(len(dir)))}
}

// Free releases the fonts; books made with them keep their own copy.
func (f *Fonts) Free() {
	C.be_fonts_free(f.f)
	f.f = nil
}

// Book is a book: from Open all laid out; from New nothing laid out (as experiment 08 does): the reader lays out the
// document it opens at, then the rest, and keeps the cache (Save) for the next time (Restore).
type Book struct{ doc }

// New makes a book with nothing laid out; fonts (or nil) are for text the book's own fonts lack.
func New(spine []string, read ReadFunc, o Options, fonts *Fonts) *Book {
	ctx, ptrs, lens, n, free := start(spine, read)
	defer free()
	var f *C.Fonts
	if fonts != nil {
		f = fonts.f
	}
	h := C.be_new(C.be_read_fn(C.goRead), C.uintptr_t(ctx), (**C.uint8_t)(ptrs), (*C.size_t)(lens), C.size_t(n), f,
		C.uint32_t(o.Width), C.uint32_t(o.Height), C.uint32_t(o.Margin), C.float(o.FontSize), C.float(o.LineHeight))
	return &Book{doc{b: h, ctx: ctx, w: o.Width, h: o.Height}}
}

// Layout lays out spine document doc (paginating it the first time); its pages are [first, last). Page numbers
// after it shift the first time: final once every document is paginated.
func (b *Book) Layout(doc int) (first, last int) {
	var f, l C.size_t
	C.be_layout(b.b, C.size_t(doc), &f, &l)
	return int(f), int(l)
}

// LineSpacing is spine document doc's line spacing as laid out: its most common line height over the text size (0:
// no text).
func (b *Book) LineSpacing(doc int) float32 { return float32(C.be_line_spacing(b.b, C.size_t(doc))) }

// LayoutNext paginates the next document not paginated yet: false if every one already was.
func (b *Book) LayoutNext() bool { return C.be_layout_next(b.b) != 0 }

// Save is the book's cache, or nil until every document is paginated.
func (b *Book) Save() []byte {
	var n C.size_t
	p := C.be_save(b.b, &n)
	if p == nil {
		return nil
	}
	defer C.be_bytes_free(p, n)
	return C.GoBytes(unsafe.Pointer(p), C.int(n))
}

// Restore takes a cache from Save on a book just made: false, and the book unchanged, if it is for other settings,
// another spine or version, or damaged.
func (b *Book) Restore(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return C.be_restore(b.b, (*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data))) != 0
}

func (d *doc) Pages() int { return int(C.be_pages(d.b)) }

func (d *doc) Draw(n int, img *image.Gray) {
	C.be_draw(d.b, C.size_t(n), (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])))
}

func (d *doc) Find(spine int, id string) (int, []image.Rectangle) {
	var buf [64]C.int32_t
	var n C.size_t
	cid := C.CBytes([]byte(id))
	defer C.free(cid)
	p := int(C.be_find(d.b, C.size_t(spine), (*C.uint8_t)(cid), C.size_t(len(id)), &buf[0], 16, &n))
	if p < 0 {
		return -1, nil
	}
	rs := make([]image.Rectangle, int(n))
	for i := range rs {
		rs[i] = image.Rect(int(buf[4*i]), int(buf[4*i+1]), int(buf[4*i+2]), int(buf[4*i+3]))
	}
	return p, rs
}

// A Word on a page: where it is in its document's text (its inline block, counted in document order, and its bytes
// in that block's text), which stays when the book is laid out again, and its box on the page, its line's height.
type Word struct {
	Block, Start, End int
	Box               image.Rectangle
}

// Words are the words on page n, line by line, each line in visual order.
func (d *doc) Words(n int) []Word {
	buf := make([]C.int32_t, 7*512)
	k := int(C.be_words(d.b, C.size_t(n), &buf[0], 512))
	if k > 512 {
		buf = make([]C.int32_t, 7*k)
		C.be_words(d.b, C.size_t(n), &buf[0], C.size_t(k))
	}
	ws := make([]Word, k)
	for i := range ws {
		v := buf[7*i:]
		ws[i] = Word{int(v[0]), int(v[1]), int(v[2]), image.Rect(int(v[3]), int(v[4]), int(v[5]), int(v[6]))}
	}
	return ws
}

// Text is spine document doc's text from (block, byte) to (block, byte), as Words gives them.
func (d *doc) Text(spine, b0, s0, b1, s1 int) string {
	var n C.size_t
	p := C.be_text(d.b, C.size_t(spine), C.uint32_t(b0), C.uint32_t(s0), C.uint32_t(b1), C.uint32_t(s1), &n)
	defer C.be_bytes_free(p, n)
	return string(C.GoBytes(unsafe.Pointer(p), C.int(n)))
}

func (d *doc) PageLines() []PageInfo {
	ps := make([]PageInfo, d.Pages())
	for i := range ps {
		var docIdx, lines C.size_t
		C.be_page_info(d.b, C.size_t(i), &docIdx, &lines)
		ps[i] = PageInfo{Spine: int(docIdx), Lines: int(lines)}
	}
	return ps
}

func (d *doc) Close() {
	C.be_close(d.b)
	booksMu.Lock()
	delete(books, d.ctx)
	booksMu.Unlock()
}
