// cgo-probe: Go calling the Rust probe library (Parley) through cgo; prints the number of lines a text takes.
//
//	cgo-probe FONT.ttf TEXT_FILE
package main

/*
#cgo LDFLAGS: -L${SRCDIR}/../lib -lprobelayout -lunwind -lm
#include <stdint.h>
#include <stddef.h>
int32_t probe_line_count(const uint8_t *font, size_t font_len, const uint8_t *text, size_t text_len, float width,
                         float size);
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

func main() {
	font, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	text, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	n := C.probe_line_count((*C.uint8_t)(unsafe.Pointer(&font[0])), C.size_t(len(font)),
		(*C.uint8_t)(unsafe.Pointer(&text[0])), C.size_t(len(text)), 918, 34)
	fmt.Println("lines:", n)
}
