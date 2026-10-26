//go:build blitz

// Package blitz is the Rust engine in engine/ (Blitz: Stylo, Taffy, Parley; our pagination) as an
// engine.Document, through cgo and its C interface (blitz_engine.h). Go reads the book: the engine asks for files
// through a callback. Needs the static library built first (engine/: cargo build --release, and
// for ARMv7 --target armv7-unknown-linux-musleabihf) and -tags blitz, in the eink-rust image; build-armv7.sh makes
// the 32-bit ARM binaries.
package blitz

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine
#cgo arm64 LDFLAGS: ${SRCDIR}/../../../engine/target/release/libblitz_engine.a -lgcc_s -lutil -lrt -lpthread -lm -ldl
#cgo arm LDFLAGS: ${SRCDIR}/../../../engine/target/armv7-unknown-linux-musleabihf/release/libblitz_engine.a -lunwind -lm
#include <stdlib.h>
#include "blitz_engine.h"
void goRead(uintptr_t ctx, uint8_t *path, size_t path_len, void *dst);
*/
import "C"

import (
	"image"
	"sync"
	"unsafe"

	engine "einkengine"
	"einkengine/epub"
)

func init() { engine.Register("blitz", Open) }

// Books being read, by the number C holds (C may not keep Go pointers).
var (
	booksMu sync.Mutex
	books   = map[uintptr]*epub.Book{}
	nextCtx uintptr
)

//export goRead
func goRead(ctx C.uintptr_t, path *C.uint8_t, pathLen C.size_t, dst unsafe.Pointer) {
	booksMu.Lock()
	b := books[uintptr(ctx)]
	booksMu.Unlock()
	data, err := b.Read(C.GoStringN((*C.char)(unsafe.Pointer(path)), C.int(pathLen)))
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
func start(b *epub.Book) (ctx uintptr, ptrs, lens unsafe.Pointer, n int, free func()) {
	booksMu.Lock()
	nextCtx++
	ctx = nextCtx
	books[ctx] = b
	booksMu.Unlock()
	spine := b.Spine()
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

func Open(b *epub.Book, o engine.Options) (engine.Document, error) {
	ctx, ptrs, lens, n, free := start(b)
	defer free()
	h := C.be_open(C.be_read_fn(C.goRead), C.uintptr_t(ctx), (**C.uint8_t)(ptrs), (*C.size_t)(lens),
		C.size_t(n), C.uint32_t(o.Width), C.uint32_t(o.Height), C.uint32_t(o.Margin), C.float(o.FontSize))
	return &doc{b: h, ctx: ctx, w: o.Width, h: o.Height}, nil
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

// Book is a book opened as experiment 08 does: nothing laid out at New; the reader lays out the document it opens
// at, then the rest, and keeps the cache (Save) for the next time (Restore). Pages, Draw, Find, PageLines and Close
// are as on the Document Open returns.
type Book struct{ doc }

// New makes a book with nothing laid out; fonts (or nil) are for text the book's own fonts lack.
func New(b *epub.Book, o engine.Options, fonts *Fonts) *Book {
	ctx, ptrs, lens, n, free := start(b)
	defer free()
	var f *C.Fonts
	if fonts != nil {
		f = fonts.f
	}
	h := C.be_new(C.be_read_fn(C.goRead), C.uintptr_t(ctx), (**C.uint8_t)(ptrs), (*C.size_t)(lens), C.size_t(n), f,
		C.uint32_t(o.Width), C.uint32_t(o.Height), C.uint32_t(o.Margin), C.float(o.FontSize))
	return &Book{doc{b: h, ctx: ctx, w: o.Width, h: o.Height}}
}

// Layout lays out spine document doc (paginating it the first time); its pages are [first, last). Page numbers
// after it shift the first time: final once every document is paginated.
func (b *Book) Layout(doc int) (first, last int) {
	var f, l C.size_t
	C.be_layout(b.b, C.size_t(doc), &f, &l)
	return int(f), int(l)
}

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

func (d *doc) PageLines() []engine.PageInfo {
	ps := make([]engine.PageInfo, d.Pages())
	for i := range ps {
		var docIdx, lines C.size_t
		C.be_page_info(d.b, C.size_t(i), &docIdx, &lines)
		ps[i] = engine.PageInfo{Spine: int(docIdx), Lines: int(lines)}
	}
	return ps
}

func (d *doc) Close() {
	C.be_close(d.b)
	booksMu.Lock()
	delete(books, d.ctx)
	booksMu.Unlock()
}
