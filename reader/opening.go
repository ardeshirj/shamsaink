package main

import (
	"image"

	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// showOpening puts an opening screen on the whole screen: in colour on a colour screen, else grey.
func showOpening(scr *device.Screen, o *ui.Opening, waveform uint32, flash bool) {
	if o.RGBA != nil {
		scr.ShowRGBA(o.RGBA, image.Rect(0, 0, scr.W, scr.H), waveform, flash)
		return
	}
	scr.Show(o.Gray, waveform, flash)
}
