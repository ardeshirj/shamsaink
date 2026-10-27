//go:build blitz

package blitz

/*
#cgo CFLAGS: -I${SRCDIR}/..
#include <stdlib.h>
#include <stdint.h>
#include "blitz_engine.h"
*/
import "C"

import "unsafe"

// Audio is an MP3 decoded as it is played (the narration of read-along; src/audio.rs). Its samples are interleaved.
type Audio struct{ a *C.Audio }

// OpenAudio opens an MP3 (its bytes are copied); nil if it cannot be decoded.
func OpenAudio(data []byte) *Audio {
	p := C.CBytes(data)
	defer C.free(p)
	a := C.be_audio_open((*C.uint8_t)(p), C.size_t(len(data)))
	if a == nil {
		return nil
	}
	return &Audio{a}
}

// Rate is the samples per second, per channel.
func (a *Audio) Rate() int { return int(C.be_audio_rate(a.a)) }

// Channels is the number of channels.
func (a *Audio) Channels() int { return int(C.be_audio_channels(a.a)) }

// Seek goes to secs and returns the time reached, or -1 if the seek failed.
func (a *Audio) Seek(secs float64) float64 { return float64(C.be_audio_seek(a.a, C.double(secs))) }

// Read fills buf with samples and returns how many were written: 0 only at the end of the stream.
func (a *Audio) Read(buf []int16) int {
	if len(buf) == 0 {
		return 0
	}
	return int(C.be_audio_read(a.a, (*C.int16_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))))
}

// Close releases the audio; it is not used afterwards.
func (a *Audio) Close() {
	C.be_audio_close(a.a)
	a.a = nil
}
