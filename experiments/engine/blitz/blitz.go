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

func Open(b *epub.Book, o engine.Options) (engine.Document, error) {
	booksMu.Lock()
	nextCtx++
	ctx := nextCtx
	books[ctx] = b
	booksMu.Unlock()
	spine := b.Spine()
	ptrs := C.malloc(C.size_t(len(spine)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	lens := C.malloc(C.size_t(len(spine)) * C.size_t(unsafe.Sizeof(C.size_t(0))))
	defer C.free(ptrs)
	defer C.free(lens)
	ps := unsafe.Slice((**C.uint8_t)(ptrs), len(spine))
	ls := unsafe.Slice((*C.size_t)(lens), len(spine))
	for i, s := range spine {
		ps[i] = (*C.uint8_t)(C.CBytes([]byte(s)))
		ls[i] = C.size_t(len(s))
	}
	defer func() {
		for _, p := range ps {
			C.free(unsafe.Pointer(p))
		}
	}()
	h := C.be_open(C.be_read_fn(C.goRead), C.uintptr_t(ctx), (**C.uint8_t)(ptrs), (*C.size_t)(lens),
		C.size_t(len(spine)), C.uint32_t(o.Width), C.uint32_t(o.Height), C.uint32_t(o.Margin), C.float(o.FontSize))
	return &doc{b: h, ctx: ctx, w: o.Width, h: o.Height}, nil
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
