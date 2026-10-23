// Package cre drives crengine (KOReader's layout engine) from Go through wrap.cpp.
package cre

/*
#cgo CXXFLAGS: -std=c++17 -I/opt/crengine/include -Wno-unused
#cgo pkg-config: freetype2 harfbuzz fribidi libunibreak libutf8proc libzstd libpng libjpeg zlib libxxhash
#cgo LDFLAGS: /opt/crengine/lib/libcrengine.a -lstdc++
#include <stdlib.h>
#include "wrap.h"
*/
import "C"

import (
	"image"
	"unsafe"
)

// Init sets up the font manager, the cache of laid-out books (cacheDir "" for none) and hyphenation, once.
func Init(cacheDir string, cacheMB int, hyphDir string) {
	var cd *C.char
	if cacheDir != "" {
		cd = C.CString(cacheDir)
		defer C.free(unsafe.Pointer(cd))
	}
	hd := C.CString(hyphDir)
	defer C.free(unsafe.Pointer(hd))
	C.cre_init(cd, C.int(cacheMB), hd)
}

func RegisterFont(path string) bool {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.cre_register_font(p) != 0
}

// Doc is one book laid out at a fixed page size.
type Doc struct {
	d    *C.CreDoc
	w, h int
}

func New(width, height int) *Doc {
	return &Doc{d: C.cre_new(C.int(width), C.int(height)), w: width, h: height}
}

func (d *Doc) Close() { C.cre_close(d.d) }

// SetInt and SetString set crengine properties (lvdocviewprops.h), as KOReader's setIntProperty does.
func (d *Doc) SetInt(name string, v int) {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	C.cre_set_int(d.d, n, C.int(v))
}

func (d *Doc) SetString(name, v string) {
	n, s := C.CString(name), C.CString(v)
	defer C.free(unsafe.Pointer(n))
	defer C.free(unsafe.Pointer(s))
	C.cre_set_str(d.d, n, s)
}

func (d *Doc) SetFontSize(px int)        { C.cre_set_font_size(d.d, C.int(px)) }
func (d *Doc) SetInterlineSpace(pct int) { C.cre_set_interline(d.d, C.int(pct)) }
func (d *Doc) Render()                   { C.cre_render(d.d) }
func (d *Doc) PageCount() int            { return int(C.cre_page_count(d.d)) }

func (d *Doc) SetStyleSheet(path string) bool {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.cre_set_stylesheet(d.d, p) != 0
}

func (d *Doc) Load(path string) bool {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.cre_load(d.d, p) != 0
}

// DrawPage draws page (from 0) into img, which must be the page size; that page becomes the one shown.
func (d *Doc) DrawPage(page int, img *image.Gray) {
	C.cre_draw_page(d.d, C.int(page), (*C.uchar)(unsafe.Pointer(&img.Pix[0])))
}

// IDPage is the page (from 0) where the element with this id in spine item spine starts, -1 if not found.
func (d *Doc) IDPage(spine int, id string) int {
	s := C.CString(id)
	defer C.free(unsafe.Pointer(s))
	return int(C.cre_id_page(d.d, C.int(spine), s))
}

// IDRects are the boxes of the element's text on the page shown, one per line.
func (d *Doc) IDRects(spine int, id string) []image.Rectangle {
	s := C.CString(id)
	defer C.free(unsafe.Pointer(s))
	var buf [16]C.CreRect
	n := int(C.cre_id_rects(d.d, C.int(spine), s, &buf[0], C.int(len(buf))))
	rs := make([]image.Rectangle, n)
	for i := range rs {
		rs[i] = image.Rect(int(buf[i].left), int(buf[i].top), int(buf[i].right), int(buf[i].bottom))
	}
	return rs
}
